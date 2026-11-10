package campus

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"
)

type aiConversation struct {
	ID         int64     `json:"id"`
	AppID      int64     `json:"app_id"`
	AppName    string    `json:"app_name"`
	AppEnabled bool      `json:"app_enabled"`
	Title      string    `json:"title"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type aiSkillHit struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
type aiMetadata struct {
	Skill     *aiSkillHit `json:"skill"`
	Citations []Citation  `json:"citations"`
}
type aiMessage struct {
	ID       int64       `json:"id"`
	Role     string      `json:"role"`
	Body     string      `json:"body"`
	Metadata *aiMetadata `json:"metadata,omitempty"`
}
type aiTurn struct {
	Tools     []toolEvent `json:"tools,omitempty"`
	ID        int64       `json:"id"`
	RequestID string      `json:"client_request_id"`
	Status    string      `json:"status"`
	Failure   *string     `json:"failure_code"`
	Messages  []aiMessage `json:"messages"`
}

const conversationFields = "c.id,c.assistant_id,a.name,a.enabled,c.title,c.updated_at"
const conversationJoin = " FROM ai_conversations c JOIN assistants a ON a.id=c.assistant_id AND a.class_id=c.class_id WHERE c.user_id=? AND c.class_id=? AND c.deleted_at IS NULL"

func scanConversation(row scanner) (aiConversation, error) {
	var c aiConversation
	err := row.Scan(&c.ID, &c.AppID, &c.AppName, &c.AppEnabled, &c.Title, &c.UpdatedAt)
	return c, err
}

func aiPageLimit(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return 20, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 50 {
		return 0, errors.New("invalid limit")
	}
	return n, nil
}

type conversationCursor struct {
	Time time.Time
	ID   int64
}

func (s *Server) listAIConversations(w http.ResponseWriter, r *http.Request) {
	if !aiAllowed(w, r, "student") {
		return
	}
	limit, err := aiPageLimit(r)
	u := currentUser(r)
	args := []any{u.ID, u.ClassID}
	query := "SELECT " + conversationFields + conversationJoin
	if raw := r.URL.Query().Get("cursor"); raw != "" && err == nil {
		var cursor conversationCursor
		data, e := base64.RawURLEncoding.DecodeString(raw)
		if e != nil || json.Unmarshal(data, &cursor) != nil || cursor.ID < 1 || cursor.Time.Year() < 1000 || cursor.Time.Year() > 9999 {
			err = errors.New("invalid cursor")
		} else {
			query += " AND (c.updated_at<? OR (c.updated_at=? AND c.id<?))"
			args = append(args, cursor.Time, cursor.Time, cursor.ID)
		}
	}
	if err != nil {
		aiError(w, 400, "invalid_cursor", "分页参数无效")
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), query+" ORDER BY c.updated_at DESC,c.id DESC LIMIT ?", append(args, limit+1)...)
	if err != nil {
		aiDBError(w, err)
		return
	}
	defer rows.Close()
	items := []aiConversation{}
	for rows.Next() {
		item, e := scanConversation(rows)
		if e != nil {
			aiDBError(w, e)
			return
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		aiDBError(w, err)
		return
	}
	var next any
	if len(items) > limit {
		items = items[:limit]
		last := items[limit-1]
		data, _ := json.Marshal(conversationCursor{last.UpdatedAt, last.ID})
		next = base64.RawURLEncoding.EncodeToString(data)
	}
	writeJSON(w, 200, map[string]any{"items": items, "next_cursor": next})
}

func (s *Server) createAIConversation(w http.ResponseWriter, r *http.Request) {
	if !aiAllowed(w, r, "student") {
		return
	}
	var in struct {
		AppID int64 `json:"app_id"`
	}
	if !decodeAIJSON(w, r, &in) {
		return
	}
	if in.AppID < 1 {
		aiError(w, 400, "invalid_input", "应用 ID 无效")
		return
	}
	ctx := r.Context()
	u := currentUser(r)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		aiDBError(w, err)
		return
	}
	defer tx.Rollback()
	var enabled bool
	if err = tx.QueryRowContext(ctx, "SELECT enabled FROM assistants WHERE id=? AND class_id=? AND deleted_at IS NULL FOR UPDATE", in.AppID, u.ClassID).Scan(&enabled); err != nil {
		aiDBError(w, err)
		return
	}
	if !enabled {
		aiDBError(w, sql.ErrNoRows)
		return
	}
	result, err := tx.ExecContext(ctx, "INSERT INTO ai_conversations(user_id,class_id,assistant_id,title) VALUES(?,?,?,'新对话')", u.ID, u.ClassID, in.AppID)
	if err != nil {
		aiDBError(w, err)
		return
	}
	id, err := result.LastInsertId()
	if err != nil {
		aiDBError(w, err)
		return
	}
	item, err := scanConversation(tx.QueryRowContext(ctx, "SELECT "+conversationFields+conversationJoin+" AND c.id=?", u.ID, u.ClassID, id))
	if err != nil {
		aiDBError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		aiDBError(w, err)
		return
	}
	writeJSON(w, 201, item)
}

func (s *Server) getAIConversation(w http.ResponseWriter, r *http.Request) {
	if !aiAllowed(w, r, "student") {
		return
	}
	id, ok := aiID(w, r, "conversation_id")
	if !ok {
		return
	}
	limit, err := aiPageLimit(r)
	var cursor int64
	if raw := r.URL.Query().Get("cursor"); raw != "" && err == nil {
		cursor, err = strconv.ParseInt(raw, 10, 64)
		if cursor < 1 {
			err = errors.New("invalid cursor")
		}
	}
	if err != nil {
		aiError(w, 400, "invalid_cursor", "分页参数无效")
		return
	}
	ctx := r.Context()
	u := currentUser(r)
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		aiDBError(w, err)
		return
	}
	defer tx.Rollback()
	item, err := scanConversation(tx.QueryRowContext(ctx, "SELECT "+conversationFields+conversationJoin+" AND c.id=?", u.ID, u.ClassID, id))
	if err != nil {
		aiDBError(w, err)
		return
	}
	query := "SELECT id,client_request_id,status,failure_code FROM ai_turns WHERE conversation_id=?"
	args := []any{id}
	if cursor != 0 {
		query += " AND id<?"
		args = append(args, cursor)
	}
	rows, err := tx.QueryContext(ctx, query+" ORDER BY id DESC LIMIT ?", append(args, limit+1)...)
	if err != nil {
		aiDBError(w, err)
		return
	}
	items := []aiTurn{}
	for rows.Next() {
		var turn aiTurn
		if err = rows.Scan(&turn.ID, &turn.RequestID, &turn.Status, &turn.Failure); err != nil {
			rows.Close()
			aiDBError(w, err)
			return
		}
		items = append(items, turn)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		aiDBError(w, err)
		return
	}
	var next any
	if len(items) > limit {
		items = items[:limit]
		next = strconv.FormatInt(items[limit-1].ID, 10)
	}
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
	for i := range items {
		items[i].Messages, err = aiTurnMessages(ctx, tx, items[i].ID, u.ClassID)
		if err != nil {
			aiDBError(w, err)
			return
		}
		items[i].Tools, err = turnToolEvents(ctx, tx, items[i].ID, u.ClassID)
		if err != nil {
			aiDBError(w, err)
			return
		}
	}
	if err = tx.Commit(); err != nil {
		aiDBError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"conversation": item, "items": items, "next_cursor": next})
}

func aiTurnMessages(ctx context.Context, tx *sql.Tx, turnID, classID int64) ([]aiMessage, error) {
	rows, err := tx.QueryContext(ctx, "SELECT id,role,body,metadata FROM ai_messages WHERE turn_id=? ORDER BY id", turnID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []aiMessage{}
	for rows.Next() {
		var m aiMessage
		var meta []byte
		if err = rows.Scan(&m.ID, &m.Role, &m.Body, &meta); err != nil {
			return nil, err
		}
		if len(meta) > 0 {
			m.Metadata = new(aiMetadata)
			if err = json.Unmarshal(meta, m.Metadata); err != nil {
				return nil, err
			}
		}
		items = append(items, m)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].Metadata == nil {
			continue
		}
		for j := range items[i].Metadata.Citations {
			citation := &items[i].Metadata.Citations[j]
			var material int64
			if citation.SourceStatus == "material_deleted" {
				continue
			}
			if err = refreshCitations(ctx, tx, classID, items[i].Metadata.Citations[j:j+1]); err != nil {
				return nil, err
			}
			if citation.SourceStatus != "available" {
				continue
			}
			err = tx.QueryRowContext(ctx, "SELECT id FROM materials WHERE id=? AND class_id=? AND deleted_at IS NULL", citation.MaterialID, classID).Scan(&material)
			if errors.Is(err, sql.ErrNoRows) {
				*citation = Citation{Number: citation.Number}
				continue
			}
			if err != nil {
				return nil, err
			}
			var chunk int64
			err = tx.QueryRowContext(ctx, "SELECT c.id"+hitJoin+" AND c.material_id=? AND c.id=?", classID, material, citation.ChunkID).Scan(&chunk)
			if errors.Is(err, sql.ErrNoRows) {
				citation.URL = "/materials?material_id=" + strconv.FormatInt(material, 10)
				citation.ChunkID = 0
				continue
			}
			if err != nil {
				return nil, err
			}
		}
	}
	return items, nil
}

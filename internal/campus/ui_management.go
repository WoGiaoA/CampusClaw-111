package campus

import (
	"net/http"
	"strings"
)

// New applications start disabled and acquire capabilities only through explicit configuration.
func (s *Server) createAIApp(w http.ResponseWriter, r *http.Request) {
	if !aiAllowed(w, r, "teacher") {
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if !decodeAIJSON(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if !aiText(in.Name, 1, 100) {
		aiError(w, 400, "invalid_input", "应用名称需要 1–100 个字符")
		return
	}
	result, err := s.DB.ExecContext(r.Context(), "INSERT INTO assistants(class_id,name,prompt_text,enabled) VALUES(?,?,'',FALSE)", currentUser(r).ClassID, in.Name)
	if err != nil {
		aiDBError(w, err)
		return
	}
	id, err := result.LastInsertId()
	if err != nil {
		aiDBError(w, err)
		return
	}
	writeJSON(w, 201, aiApp{ID: id, Name: in.Name, SkillIDs: []int64{}, MaterialIDs: []int64{}})
}

// Keep the seed key and history so restarting cannot resurrect a deleted application.
func (s *Server) deleteAIApp(w http.ResponseWriter, r *http.Request) {
	if !aiAllowed(w, r, "teacher") {
		return
	}
	id, ok := aiID(w, r, "app_id")
	if !ok {
		return
	}
	ctx, classID := r.Context(), currentUser(r).ClassID
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		aiDBError(w, err)
		return
	}
	defer tx.Rollback()
	var found int64
	if err = tx.QueryRowContext(ctx, "SELECT id FROM assistants WHERE id=? AND class_id=? AND deleted_at IS NULL FOR UPDATE", id, classID).Scan(&found); err != nil {
		aiDBError(w, err)
		return
	}
	var running int
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM ai_turns t JOIN ai_conversations c ON c.id=t.conversation_id WHERE c.assistant_id=? AND c.class_id=? AND t.status='running'", id, classID).Scan(&running); err != nil {
		aiDBError(w, err)
		return
	}
	if running > 0 {
		aiError(w, 409, "app_busy", "此应用仍有会话正在生成，请等待生成结束后再删除")
		return
	}
	if _, err = tx.ExecContext(ctx, "UPDATE assistants SET enabled=FALSE,deleted_at=CURRENT_TIMESTAMP(6) WHERE id=? AND class_id=?", id, classID); err != nil {
		aiDBError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		aiDBError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"id": id, "status": "deleted"})
}

// Keep teaching evidence and audit foreign keys intact; deleted conversations are no longer usable.
func (s *Server) deleteAIConversation(w http.ResponseWriter, r *http.Request) {
	if !aiAllowed(w, r, "student") {
		return
	}
	id, ok := aiID(w, r, "conversation_id")
	if !ok {
		return
	}
	u, ctx := currentUser(r), r.Context()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		aiDBError(w, err)
		return
	}
	defer tx.Rollback()
	var found int64
	err = tx.QueryRowContext(ctx, "SELECT id FROM ai_conversations WHERE id=? AND user_id=? AND class_id=? AND deleted_at IS NULL FOR UPDATE", id, u.ID, u.ClassID).Scan(&found)
	if err != nil {
		aiDBError(w, err)
		return
	}
	var running int
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM ai_turns WHERE conversation_id=? AND status='running'", id).Scan(&running); err != nil {
		aiDBError(w, err)
		return
	}
	if running != 0 {
		aiError(w, 409, "conversation_busy", "此会话正在生成，请取消或等待生成结束后再删除")
		return
	}
	if _, err = tx.ExecContext(ctx, "UPDATE ai_conversations SET deleted_at=CURRENT_TIMESTAMP(6) WHERE id=?", id); err != nil {
		aiDBError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		aiDBError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"id": id, "status": "deleted"})
}

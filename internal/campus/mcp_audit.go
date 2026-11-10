package campus

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"time"
)

type toolEvent struct {
	TurnID           int64  `json:"turn_id"`
	AttemptID        string `json:"attempt_id"`
	Tool             string `json:"tool"`
	Stage            int    `json:"stage"`
	Status           string `json:"status"`
	Reason           string `json:"reason_code"`
	Summary          string `json:"summary"`
	parameterSummary string
}

type auditEvent struct {
	toolEvent
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	ClassID        int64     `json:"class_id"`
	ConversationID int64     `json:"conversation_id"`
	ServerID       *int64    `json:"server_id"`
	Duration       int64     `json:"duration_ms"`
	Parameters     string    `json:"parameter_summary"`
	At             time.Time `json:"created_at"`
}

func appendToolAudit(ctx context.Context, db sqlExecutor, run toolRun, item mcpRef, event toolEvent, ms int64) error {
	_, err := db.ExecContext(ctx, `INSERT INTO tool_call_audit(attempt_id,terminal,class_id,user_id,conversation_id,turn_id,server_id,tool_ref,stage,status,reason_code,summary,duration_ms,parameter_summary)
 VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, event.AttemptID, event.Status != "running", run.user.ClassID, run.user.ID, run.conversation, run.turn.ID, item.ServerID, event.Tool, event.Stage, event.Status, event.Reason, event.Summary, ms, event.parameterSummary)
	return err
}

func (s *Server) RecoverMCP(ctx context.Context) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE mcp_servers SET status='disconnected',revision=revision+1,last_error_code='interrupted' WHERE status='connecting'`); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT IGNORE INTO tool_call_audit(attempt_id,terminal,class_id,user_id,conversation_id,turn_id,server_id,tool_ref,stage,status,reason_code,summary,parameter_summary)
 SELECT a.attempt_id,TRUE,a.class_id,a.user_id,a.conversation_id,a.turn_id,a.server_id,a.tool_ref,a.stage,'error','interrupted','工具执行中断',a.parameter_summary
 FROM tool_call_audit a LEFT JOIN tool_call_audit terminal ON terminal.attempt_id=a.attempt_id AND terminal.terminal WHERE NOT a.terminal AND terminal.id IS NULL`); err != nil {
		return err
	}
	return tx.Commit()
}

func scanToolEvent(row scanner, event *toolEvent) error {
	return row.Scan(&event.TurnID, &event.AttemptID, &event.Tool, &event.Stage, &event.Status, &event.Reason, &event.Summary)
}

const toolEventFields = "turn_id,attempt_id,tool_ref,stage,status,reason_code,summary"

func turnToolEvents(ctx context.Context, tx *sql.Tx, turnID, classID int64) ([]toolEvent, error) {
	rows, err := tx.QueryContext(ctx, "SELECT "+toolEventFields+" FROM tool_call_audit WHERE turn_id=? AND class_id=? ORDER BY id", turnID, classID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []toolEvent{}
	for rows.Next() {
		var event toolEvent
		if err = scanToolEvent(rows, &event); err != nil {
			return nil, err
		}
		items = append(items, event)
	}
	return items, rows.Err()
}

func (s *Server) listMCPAudit(w http.ResponseWriter, r *http.Request) {
	if !aiAllowed(w, r, "teacher") {
		return
	}
	limit, err := aiPageLimit(r)
	q := r.URL.Query()
	query := " WHERE class_id=? AND tool_ref<>''"
	args := []any{currentUser(r).ClassID}
	for _, filter := range []struct{ key, column string }{{"cursor", "id"}, {"conversation_id", "conversation_id"}} {
		if value := q.Get(filter.key); value != "" {
			n, e := strconv.ParseInt(value, 10, 64)
			if e != nil || n < 1 {
				aiError(w, 400, "invalid_input", "审计筛选无效")
				return
			}
			operator := "="
			if filter.key == "cursor" {
				operator = "<"
			}
			query += " AND " + filter.column + operator + "?"
			args = append(args, n)
		}
	}
	if status := q.Get("status"); status != "" {
		switch status {
		case "running", "success", "denied", "error", "cancelled":
		default:
			aiError(w, 400, "invalid_input", "审计状态无效")
			return
		}
		query += " AND status=?"
		args = append(args, status)
	}
	if err != nil {
		aiError(w, 400, "invalid_input", "审计分页无效")
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), "SELECT "+toolEventFields+",id,class_id,user_id,conversation_id,server_id,duration_ms,created_at,parameter_summary FROM tool_call_audit"+query+" ORDER BY id DESC LIMIT ?", append(args, limit+1)...)
	if err != nil {
		mcpError(w, err)
		return
	}
	defer rows.Close()
	items := []auditEvent{}
	for rows.Next() {
		var a auditEvent
		if err = rows.Scan(&a.TurnID, &a.AttemptID, &a.Tool, &a.Stage, &a.Status, &a.Reason, &a.Summary, &a.ID, &a.ClassID, &a.UserID, &a.ConversationID, &a.ServerID, &a.Duration, &a.At, &a.Parameters); err != nil {
			mcpError(w, err)
			return
		}
		items = append(items, a)
	}
	if err = rows.Err(); err != nil {
		mcpError(w, err)
		return
	}
	var next any
	if len(items) > limit {
		items = items[:limit]
		next = strconv.FormatInt(items[limit-1].ID, 10)
	}
	writeJSON(w, 200, map[string]any{"items": items, "next_cursor": next})
}

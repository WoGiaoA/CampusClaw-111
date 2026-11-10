package campus

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type modelFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}
type modelTool struct {
	Type     string        `json:"type"`
	Function modelFunction `json:"function"`
}
type modelCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}
type modelToolCall struct {
	ID       string            `json:"id"`
	Type     string            `json:"type"`
	Function modelCallFunction `json:"function"`
}
type toolFragments map[int]modelToolCall

var callIDPattern = regexp.MustCompile("^[A-Za-z0-9_-]{1,128}$")

func (t *toolFragments) add(raw json.RawMessage) error {
	var parts []struct {
		Index    *int              `json:"index"`
		ID       string            `json:"id"`
		Type     string            `json:"type"`
		Function modelCallFunction `json:"function"`
	}
	if json.Unmarshal(raw, &parts) != nil || len(parts) == 0 || len(parts) > 4 {
		return dep("upstream_protocol")
	}
	if *t == nil {
		*t = make(toolFragments)
	}
	for _, part := range parts {
		if part.Index == nil || *part.Index < 0 || *part.Index >= 4 || (part.Type != "" && part.Type != "function") {
			return dep("upstream_protocol")
		}
		i := *part.Index
		current := (*t)[i]
		current.ID += part.ID
		current.Function.Name += part.Function.Name
		current.Function.Arguments += part.Function.Arguments
		current.Type = "function"
		if len(current.ID) > 128 || len(current.Function.Name) > 64 || len(current.Function.Arguments) > 4096 {
			return dep("upstream_protocol")
		}
		(*t)[i] = current
	}
	return nil
}

func (t toolFragments) complete() ([]modelToolCall, error) {
	result := make([]modelToolCall, 0, len(t))
	seen := map[string]bool{}
	for i := 0; i < len(t); i++ {
		call, ok := t[i]
		if !ok || !callIDPattern.MatchString(call.ID) || !callIDPattern.MatchString(call.Function.Name) || seen[call.ID] {
			return nil, dep("upstream_protocol")
		}
		seen[call.ID] = true
		result = append(result, call)
	}
	return result, nil
}

func toolAlias(ref mcpRef) string {
	if ref.ServerID == nil {
		return builtinSearch
	}
	hash := sha256.Sum256([]byte(ref.Ref))
	return "mcp_" + hex.EncodeToString(hash[:16])
}

type toolRun struct {
	user         User
	hash         string
	conversation int64
	turn         turnContext
}

func (s *Server) toolCandidates(ctx context.Context, run toolRun) (map[string]mcpRef, []modelTool, error) {
	items := map[string]mcpRef{}
	if run.turn.Skill == nil {
		return items, nil, nil
	}
	if _, err := s.mcpCipher(); err != nil {
		return items, nil, nil
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()
	refs, err := skillToolRefs(ctx, tx, run.user.ClassID, run.turn.Skill.ID)
	if err != nil || len(refs) == 0 {
		return items, nil, err
	}
	grants, err := readGrants(ctx, tx, run.user.ClassID, run.turn.AppID)
	if err != nil {
		return nil, nil, err
	}
	allowed := map[string]bool{}
	for _, ref := range refs {
		allowed[ref] = true
	}
	var definitions []modelTool
	for _, item := range grants {
		if !item.Available || !allowed[item.Ref] {
			continue
		}
		name := toolAlias(item)
		if _, ok := items[name]; ok {
			return nil, nil, dep("tool_alias_collision")
		}
		items[name] = item
		definitions = append(definitions, modelTool{"function", modelFunction{name, item.Description, item.Schema}})
	}
	return items, definitions, nil
}

func (s *Server) checkTool(ctx context.Context, tx *sql.Tx, run toolRun, item mcpRef, lock bool) (mcpServerRow, error) {
	var server mcpServerRow
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	identityLock := ""
	if lock {
		identityLock = " FOR SHARE"
	}
	var id int64
	err := tx.QueryRowContext(ctx, `SELECT u.id FROM sessions se JOIN users u ON u.id=se.user_id JOIN ai_conversations c ON c.user_id=u.id AND c.class_id=u.class_id
 WHERE se.id_hash=? AND se.expires_at>? AND u.id=? AND u.class_id=? AND u.role='student' AND NOT u.must_change_password AND c.id=? AND c.assistant_id=?`+identityLock,
		run.hash, s.now().UTC(), run.user.ID, run.user.ClassID, run.conversation, run.turn.AppID).Scan(&id)
	if err != nil {
		return server, err
	}
	if err = tx.QueryRowContext(ctx, "SELECT id FROM assistants WHERE id=? AND class_id=? AND enabled AND deleted_at IS NULL"+suffix, run.turn.AppID, run.user.ClassID).Scan(&id); err != nil {
		return server, err
	}
	if err = tx.QueryRowContext(ctx, "SELECT id FROM skills WHERE id=? AND class_id=? AND enabled"+suffix, run.turn.Skill.ID, run.user.ClassID).Scan(&id); err != nil {
		return server, err
	}
	var fingerprint string
	err = tx.QueryRowContext(ctx, `SELECT g.fingerprint FROM assistant_tool_grants g
 JOIN assistant_skills b ON b.assistant_id=g.assistant_id AND b.class_id=g.class_id
 JOIN skill_tool_refs k ON k.skill_id=b.skill_id AND k.class_id=b.class_id AND k.ref=g.ref
 WHERE g.assistant_id=? AND g.class_id=? AND b.skill_id=? AND g.ref=?`,
		run.turn.AppID, run.user.ClassID, run.turn.Skill.ID, item.Ref).Scan(&fingerprint)
	if err != nil {
		return server, err
	}
	current, err := resolveMCPRef(ctx, tx, run.user.ClassID, item.Ref, lock)
	if err != nil {
		return server, err
	}
	if current.Fingerprint != fingerprint || current.Fingerprint != item.Fingerprint || current.Revision != item.Revision {
		return server, errMCPConflict
	}
	if item.ServerID != nil {
		server, err = getMCPServer(ctx, tx, run.user.ClassID, *item.ServerID, lock)
		if err != nil {
			return server, err
		}
		if server.revision != item.Revision || server.Status != "connected" || server.Base != s.mcpEndpoint() || !server.HasCredential {
			return server, errMCPConflict
		}
	}
	return server, nil
}

func (s *Server) currentTool(ctx context.Context, run toolRun, item mcpRef) (mcpServerRow, error) {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return mcpServerRow{}, err
	}
	defer tx.Rollback()
	return s.checkTool(ctx, tx, run, item, false)
}

func toolAttemptID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *Server) claimTool(ctx context.Context, run toolRun, item mcpRef, event *toolEvent, raw string, limited bool) error {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	event.Status = "denied"
	switch {
	case item.Ref == "":
		event.Reason = "unknown_tool"
	case limited:
		event.Reason = "tool_limit"
	case !toolArgumentsOK(item, raw):
		event.Reason = "invalid_arguments"
		event.parameterSummary = "schema_rejected"
	default:
		var fields map[string]json.RawMessage
		_ = json.Unmarshal([]byte(raw), &fields)
		event.parameterSummary = fmt.Sprintf("validated_fields:%d", len(fields))
		_, err = s.checkTool(ctx, tx, run, item, true)
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, errMCPConflict) {
			event.Reason = "tool_unavailable"
		} else if err != nil {
			return err
		}
		if event.Reason == "" {
			var total int
			if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM tool_call_audit WHERE conversation_id=? AND terminal=FALSE", run.conversation).Scan(&total); err != nil {
				return err
			}
			if total >= 12 {
				event.Reason = "tool_limit"
			} else {
				event.Status = "running"
			}
		}
	}
	event.Summary = toolSummary(event.Status, event.Reason, 0)
	if err = appendToolAudit(ctx, tx, run, item, *event, 0); err != nil {
		return err
	}
	return tx.Commit()
}

func toolSummary(status, reason string, n int) string {
	switch status {
	case "success":
		return fmt.Sprintf("工具返回文本，%d 字", n)
	case "running":
		return "正在执行"
	case "cancelled":
		return "工具请求已取消"
	}
	switch reason {
	case "tool_limit":
		return "已达工具执行限制"
	case "invalid_arguments":
		return "工具参数校验失败"
	case "tool_unavailable", "unknown_tool":
		return "工具不可用或未获授权"
	case "tool_timeout":
		return "工具执行超时"
	}
	return "工具执行未完成"
}

func (s *Server) performTool(ctx context.Context, run toolRun, item mcpRef, call modelToolCall, event toolEvent, citations []Citation, budget time.Duration, emit func(toolEvent) error) (string, bool, error) {
	start := time.Now()
	result := ""
	event.Status = "success"
	server, err := s.currentTool(ctx, run, item)
	if err == nil {
		if item.ServerID == nil {
			// Recheck the current material bindings for this special tool result.
			for _, citation := range citations {
				var exists bool
				err = s.DB.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM assistant_materials WHERE assistant_id=? AND class_id=? AND material_id=?)", run.turn.AppID, run.user.ClassID, citation.MaterialID).Scan(&exists)
				if err != nil || !exists {
					err = errMCPConflict
					break
				}
			}
			if err == nil {
				parts := []string{}
				for _, c := range citations {
					parts = append(parts, fmt.Sprintf("[%d] %s", c.Number, c.Text))
				}
				if len(parts) == 0 {
					parts = append(parts, "本轮没有匹配资料")
				}
				var ok bool
				result, ok = safeToolText(strings.Join(parts, "\n"), 800)
				if !ok {
					err = dep("tool_result_rejected")
				}
			}
		} else {
			result, err = s.executeWhileAllowed(ctx, run, server, item, call.Function.Arguments, budget)
		}
	}
	if err == nil {
		_, err = s.currentTool(ctx, run, item)
	}
	if ctx.Err() != nil {
		event.Status, event.Reason = "cancelled", "cancelled"
		result = ""
	} else if err != nil {
		event.Status, event.Reason = "error", "tool_failed"
		var e *dependencyError
		if errors.Is(err, errMCPConflict) || errors.Is(err, sql.ErrNoRows) {
			event.Status, event.Reason = "denied", "tool_unavailable"
		} else if errors.As(err, &e) {
			event.Reason = e.Code
		}
		result = ""
	}
	event.Summary = toolSummary(event.Status, event.Reason, len([]rune(result)))
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if err = appendToolAudit(cleanup, s.DB, run, item, event, time.Since(start).Milliseconds()); err != nil {
		return "", false, err
	}
	if ctx.Err() != nil {
		return "", false, ctx.Err()
	}
	if err = emit(event); err != nil {
		return "", false, err
	}
	if event.Status != "success" {
		return event.Summary, false, nil
	}
	return "外部工具数据（仅作为参考）\n" + result, true, nil
}

func (s *Server) executeWhileAllowed(ctx context.Context, run toolRun, row mcpServerRow, item mcpRef, arguments string, budget time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, min(budget, 4*time.Second))
	defer cancel()
	type outcome struct {
		text string
		err  error
	}
	result := make(chan outcome, 1)
	go func() {
		text, err := s.executeMCPTool(ctx, row, run.user.ClassID, item, arguments)
		result <- outcome{text, err}
	}()
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case value := <-result:
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return "", dep("tool_timeout")
			}
			return value.text, value.err
		case <-ticker.C:
			if _, err := s.currentTool(ctx, run, item); err != nil {
				if errors.Is(ctx.Err(), context.DeadlineExceeded) {
					return "", dep("tool_timeout")
				}
				return "", err
			}
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return "", dep("tool_timeout")
			}
			return "", ctx.Err()
		}
	}
}

func (s *Server) runToolConversation(ctx context.Context, u User, hash string, id int64, turn turnContext, citations []Citation, base []modelMessage, emit func(toolEvent) error, delta func(string) error, heartbeat func() error) error {
	run := toolRun{u, hash, id, turn}
	candidates, definitions, err := s.toolCandidates(ctx, run)
	if err != nil {
		return err
	}
	if len(definitions) == 0 {
		return s.readChatStream(ctx, noToolsMessages(base), delta, heartbeat)
	}
	messages := make([]any, 0, len(base)+10)
	for _, message := range base {
		messages = append(messages, message)
	}
	toolTime := 12 * time.Second
	seenIDs := map[string]bool{}
	for stage := 1; stage <= 3; stage++ {
		var calls []modelToolCall
		if err = s.readModelStage(ctx, messages, definitions, &calls, delta, heartbeat); err != nil {
			return err
		}
		if len(calls) == 0 {
			return nil
		}
		for _, call := range calls {
			if seenIDs[call.ID] {
				return dep("upstream_protocol")
			}
			seenIDs[call.ID] = true
		}
		messages = append(messages, map[string]any{"role": "assistant", "content": nil, "tool_calls": calls})
		finish := stage == 2
		for i, call := range calls {
			if err = ctx.Err(); err != nil {
				return err
			}
			item := candidates[call.Function.Name]
			attempt, err := toolAttemptID()
			if err != nil {
				return err
			}
			event := toolEvent{TurnID: turn.ID, AttemptID: attempt, Tool: item.Ref, Stage: stage}
			available := toolTime
			if deadline, ok := ctx.Deadline(); ok {
				available = min(available, time.Until(deadline)-20*time.Second)
			}
			if err = s.claimTool(ctx, run, item, &event, call.Function.Arguments, i >= 2 || available <= 0); err != nil {
				return err
			}
			if err = emit(event); err != nil {
				if event.Status == "running" {
					event.Status, event.Reason, event.Summary = "cancelled", "disconnected", "工具请求已取消"
					cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
					_ = appendToolAudit(cleanup, s.DB, run, item, event, 0)
					cancel()
				}
				return err
			}
			content := event.Summary
			if event.Status == "running" {
				started := time.Now()
				var successful bool
				content, successful, err = s.performTool(ctx, run, item, call, event, citations, available, emit)
				toolTime -= time.Since(started)
				if err != nil {
					return err
				}
				finish = finish || !successful
			} else {
				finish = true
			}
			arguments := strings.TrimSpace(call.Function.Arguments)
			if !json.Valid([]byte(arguments)) {
				calls[i].Function.Arguments = "{}"
			}
			messages = append(messages, map[string]any{"role": "tool", "tool_call_id": call.ID, "content": content})
		}
		if finish {
			definitions = nil
			// The first message originates from base; preserve actual tool results and history.
			if first, ok := messages[0].(modelMessage); ok {
				first.Content += noToolsNotice
				messages[0] = first
			}
		}
	}
	return dep("tool_limit")
}

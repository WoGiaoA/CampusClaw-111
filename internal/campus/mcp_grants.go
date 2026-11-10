package campus

import (
	"campusclaw/internal/mcpwire"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

const builtinSearch = "knowledge_search"

func legacyTool(refs []string) string {
	for _, ref := range refs {
		if ref == builtinSearch {
			return ref
		}
	}
	return ""
}

const builtinSchema = `{"type":"object","properties":{},"required":[],"additionalProperties":false}`

type mcpRef struct {
	Ref         string          `json:"ref"`
	Available   bool            `json:"available"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Schema      json.RawMessage `json:"input_schema,omitempty"`
	ServerID    *int64          `json:"-"`
	Fingerprint string          `json:"-"`
	Revision    int64           `json:"-"`
}

func parseToolRefs(field string) ([]string, error) {
	if field == "" {
		return []string{}, nil
	}
	return validateToolRefs(strings.Split(strings.ReplaceAll(field, "、", ","), ","), 8)
}

func validateToolRefs(refs []string, max int) ([]string, error) {
	if refs == nil || len(refs) > max {
		return nil, invalidSkill
	}
	result := make([]string, 0, len(refs))
	seen := map[string]bool{}
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		name, slug, ok := strings.Cut(ref, "@")
		if ref != builtinSearch && (!ok || !mcpName.MatchString(name) || !mcpSlug.MatchString(slug)) {
			return nil, invalidSkill
		}
		if !seen[ref] {
			result = append(result, ref)
			seen[ref] = true
		}
	}
	return result, nil
}

func resolveMCPRef(ctx context.Context, db aiReader, classID int64, ref string, lock bool) (mcpRef, error) {
	out := mcpRef{Ref: ref, Name: builtinSearch, Available: true, Description: "读取本轮已经授权且命中的资料片段。", Schema: json.RawMessage(builtinSchema), Fingerprint: "builtin"}
	if ref == builtinSearch {
		return out, nil
	}
	name, slug, _ := strings.Cut(ref, "@")
	var serverID int64
	var status string
	query := "SELECT id,status,revision FROM mcp_servers WHERE class_id=? AND slug=?"
	if lock {
		query += " FOR UPDATE"
	}
	if err := db.QueryRowContext(ctx, query, classID, slug).Scan(&serverID, &status, &out.Revision); err != nil {
		return out, err
	}
	out.ServerID = &serverID
	out.Name = name
	if status != "connected" {
		return out, errMCPConflict
	}
	var schema []byte
	err := db.QueryRowContext(ctx, "SELECT description,input_schema,fingerprint FROM mcp_discovered_tools WHERE server_id=? AND class_id=? AND revision=? AND tool_name=?", serverID, classID, out.Revision, name).Scan(&out.Description, &schema, &out.Fingerprint)
	if err != nil {
		return out, err
	}
	out.Schema = schema
	return out, nil
}

func replaceToolRefs(ctx context.Context, tx *sql.Tx, table, column string, ownerID, classID int64, refs []string) error {
	resolved := make([]mcpRef, 0, len(refs))
	for _, ref := range refs {
		item, err := resolveMCPRef(ctx, tx, classID, ref, true)
		if err != nil {
			return err
		}
		resolved = append(resolved, item)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE "+column+"=? AND class_id=?", ownerID, classID); err != nil {
		return err
	}
	for _, item := range resolved {
		query := "INSERT INTO " + table + "(" + column + ",class_id,ref,server_id,tool_name"
		args := []any{ownerID, classID, item.Ref, item.ServerID, item.Name}
		values := ") VALUES(?,?,?,?,?"
		if table == "assistant_tool_grants" {
			query += ",fingerprint"
			values += ",?"
			args = append(args, item.Fingerprint)
		}
		if _, err := tx.ExecContext(ctx, query+values+")", args...); err != nil {
			return err
		}
	}
	return nil
}

func skillToolRefs(ctx context.Context, db aiReader, classID, skillID int64) ([]string, error) {
	rows, err := db.QueryContext(ctx, "SELECT ref FROM skill_tool_refs WHERE class_id=? AND skill_id=? ORDER BY ref", classID, skillID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	refs := []string{}
	for rows.Next() {
		var ref string
		if err = rows.Scan(&ref); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}

func readGrants(ctx context.Context, db aiReader, classID, appID int64) ([]mcpRef, error) {
	rows, err := db.QueryContext(ctx, "SELECT ref,fingerprint FROM assistant_tool_grants WHERE class_id=? AND assistant_id=? ORDER BY ref", classID, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type grant struct{ ref, fingerprint string }
	grants := []grant{}
	for rows.Next() {
		var g grant
		if err = rows.Scan(&g.ref, &g.fingerprint); err != nil {
			return nil, err
		}
		grants = append(grants, g)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	items := []mcpRef{}
	for _, g := range grants {
		item, e := resolveMCPRef(ctx, db, classID, g.ref, false)
		if e != nil && !errors.Is(e, sql.ErrNoRows) && !errors.Is(e, errMCPConflict) {
			return nil, e
		}
		item.Available = e == nil && item.Fingerprint == g.fingerprint
		if !item.Available {
			item.Description = ""
			item.Schema = nil
			item.Name = strings.SplitN(item.Ref, "@", 2)[0]
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Server) mcpAppGrants(w http.ResponseWriter, r *http.Request) {
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
		mcpError(w, err)
		return
	}
	defer tx.Rollback()
	q := "SELECT id FROM assistants WHERE id=? AND class_id=? AND deleted_at IS NULL"
	if r.Method == "PUT" {
		q += " FOR UPDATE"
	}
	var checked int64
	if err = tx.QueryRowContext(ctx, q, id, classID).Scan(&checked); err != nil {
		mcpError(w, err)
		return
	}
	if r.Method == "PUT" {
		var in struct {
			Refs []string `json:"refs"`
		}
		if !decodeAIJSON(w, r, &in) {
			return
		}
		refs, e := validateToolRefs(in.Refs, 16)
		if e != nil {
			aiError(w, 400, "invalid_tool_refs", "工具引用格式无效")
			return
		}
		if err = replaceToolRefs(ctx, tx, "assistant_tool_grants", "assistant_id", id, classID, refs); err != nil {
			mcpError(w, err)
			return
		}
	}
	items, err := readGrants(ctx, tx, classID, id)
	if err != nil {
		mcpError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		mcpError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func toolArgumentsOK(item mcpRef, raw string) bool {
	schema, err := parseToolSchema(item.Schema)
	return err == nil && schema.accepts(json.RawMessage(raw))
}

func (s *Server) executeMCPTool(ctx context.Context, row mcpServerRow, classID int64, item mcpRef, raw string) (string, error) {
	token, err := s.mcpOpen(row.credential, classID, row.ID)
	if err != nil {
		return "", err
	}
	id := int64(3)
	var out mcpwire.Result
	if err = s.mcpRPC(ctx, row.Base, token, "tools/call", &id, map[string]any{"name": item.Name, "arguments": json.RawMessage(raw)}, &out); err != nil {
		return "", err
	}
	if out.IsError || len(out.Content) == 0 || len(out.Content) > 8 {
		return "", dep("tool_failed")
	}
	parts := make([]string, 0, len(out.Content))
	for _, part := range out.Content {
		if part.Type != "text" || strings.Contains(part.Text, token) {
			return "", dep("tool_result_rejected")
		}
		parts = append(parts, part.Text)
	}
	text, ok := safeToolText(strings.Join(parts, "\n"), 800)
	if !ok {
		return "", dep("tool_result_rejected")
	}
	return text, nil
}

package campus

import (
	"campusclaw/internal/mcpwire"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

type mcpServerRow struct {
	ID            int64   `json:"id"`
	Slug          string  `json:"slug"`
	DisplayName   string  `json:"display_name"`
	Base          string  `json:"base_url"`
	Status        string  `json:"status"`
	HasCredential bool    `json:"has_credential"`
	ToolCount     int     `json:"tool_count"`
	LastError     *string `json:"last_error_code"`
	revision      int64
	credential    []byte
}

const mcpServerFields = `id,slug,display_name,base_url,status,credential,last_error_code,revision,
 (SELECT COUNT(*) FROM mcp_discovered_tools d WHERE d.server_id=mcp_servers.id AND d.revision=mcp_servers.revision)`

func scanMCPServer(row scanner) (mcpServerRow, error) {
	var v mcpServerRow
	err := row.Scan(&v.ID, &v.Slug, &v.DisplayName, &v.Base, &v.Status, &v.credential, &v.LastError, &v.revision, &v.ToolCount)
	v.HasCredential = len(v.credential) > 0
	return v, err
}

func getMCPServer(ctx context.Context, db aiReader, classID, id int64, lock bool) (mcpServerRow, error) {
	q := "SELECT " + mcpServerFields + " FROM mcp_servers WHERE class_id=? AND id=?"
	if lock {
		q += " FOR UPDATE"
	}
	return scanMCPServer(db.QueryRowContext(ctx, q, classID, id))
}

func mcpError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errMCPConflict):
		aiError(w, 409, "mcp_conflict", "工具服务状态已改变或暂不可用")
	case errors.Is(err, sql.ErrNoRows):
		aiDBError(w, err)
	default:
		var e *dependencyError
		if errors.As(err, &e) {
			aiError(w, 503, e.Code, "MCP 服务或配置不可用")
		} else {
			aiDBError(w, err)
		}
	}
}

func (s *Server) listMCPServers(w http.ResponseWriter, r *http.Request) {
	if !aiAllowed(w, r, "teacher") {
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), "SELECT "+mcpServerFields+" FROM mcp_servers WHERE class_id=? ORDER BY id", currentUser(r).ClassID)
	if err != nil {
		mcpError(w, err)
		return
	}
	defer rows.Close()
	items := []mcpServerRow{}
	for rows.Next() {
		v, err := scanMCPServer(rows)
		if err != nil {
			mcpError(w, err)
			return
		}
		items = append(items, v)
	}
	if err = rows.Err(); err != nil {
		mcpError(w, err)
		return
	}
	_, keyErr := s.mcpCipher()
	writeJSON(w, 200, map[string]any{"items": items, "allowed_base_urls": []string{s.mcpEndpoint()}, "configured": keyErr == nil})
}

func resetMCP(ctx context.Context, tx *sql.Tx, classID, id int64, status string) error {
	if _, err := tx.ExecContext(ctx, "UPDATE mcp_servers SET status=?,revision=revision+1,last_error_code=NULL WHERE class_id=? AND id=?", status, classID, id); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "DELETE FROM mcp_discovered_tools WHERE class_id=? AND server_id=?", classID, id)
	return err
}

func (s *Server) writeMCPServer(w http.ResponseWriter, r *http.Request) {
	if !aiAllowed(w, r, "teacher") {
		return
	}
	var in struct {
		Slug    *string         `json:"slug"`
		Display *string         `json:"display_name"`
		Base    *string         `json:"base_url"`
		Token   json.RawMessage `json:"token"`
	}
	if !decodeAIJSON(w, r, &in) {
		return
	}
	create := r.Method == "POST"
	invalid := func() { aiError(w, 400, "invalid_input", "服务名称、地址或凭据格式无效") }
	if (in.Base != nil && *in.Base != s.mcpEndpoint()) || (in.Display != nil && !aiText(strings.TrimSpace(*in.Display), 1, 100)) ||
		(create && (in.Slug == nil || !mcpSlug.MatchString(*in.Slug) || in.Base == nil || in.Display == nil)) || (!create && in.Slug != nil) {
		invalid()
		return
	}
	var token *string
	if in.Token != nil {
		if json.Unmarshal(in.Token, &token) != nil || (token != nil && (!aiText(*token, 1, 512) || strings.ContainsAny(*token, " \r\n\t") || strings.IndexFunc(*token, func(r rune) bool { return r < 33 || r > 126 }) >= 0)) {
			invalid()
			return
		}
	}
	if _, err := s.mcpCipher(); err != nil {
		mcpError(w, err)
		return
	}
	ctx, classID := r.Context(), currentUser(r).ClassID
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		mcpError(w, err)
		return
	}
	defer tx.Rollback()
	var row mcpServerRow
	status := 200
	if create {
		result, err := tx.ExecContext(ctx, "INSERT IGNORE INTO mcp_servers(class_id,slug,display_name,base_url) VALUES(?,?,?,?)", classID, *in.Slug, strings.TrimSpace(*in.Display), *in.Base)
		if err != nil {
			mcpError(w, err)
			return
		}
		n, err := result.RowsAffected()
		if err != nil {
			mcpError(w, err)
			return
		}
		if n != 1 {
			mcpError(w, errMCPConflict)
			return
		}
		row.ID, err = result.LastInsertId()
		if err != nil {
			mcpError(w, err)
			return
		}
		status = 201
	} else {
		id, ok := aiID(w, r, "id")
		if !ok {
			return
		}
		row, err = getMCPServer(ctx, tx, classID, id, true)
		if err != nil {
			mcpError(w, err)
			return
		}
	}
	row, err = getMCPServer(ctx, tx, classID, row.ID, true)
	if err != nil {
		mcpError(w, err)
		return
	}
	changed := in.Token != nil
	if in.Display != nil {
		row.DisplayName = strings.TrimSpace(*in.Display)
	}
	if in.Base != nil && row.Base != *in.Base {
		row.Base = *in.Base
		changed = true
	}
	if in.Token != nil {
		row.credential = nil
		if token != nil {
			row.credential, err = s.mcpSeal(*token, classID, row.ID)
			if err != nil {
				mcpError(w, err)
				return
			}
		}
	}
	if _, err = tx.ExecContext(ctx, "UPDATE mcp_servers SET display_name=?,base_url=?,credential=? WHERE class_id=? AND id=?", row.DisplayName, row.Base, row.credential, classID, row.ID); err != nil {
		mcpError(w, err)
		return
	}
	if changed && !create {
		if err = resetMCP(ctx, tx, classID, row.ID, "disconnected"); err != nil {
			mcpError(w, err)
			return
		}
	}
	row, err = getMCPServer(ctx, tx, classID, row.ID, false)
	if err != nil {
		mcpError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		mcpError(w, err)
		return
	}
	writeJSON(w, status, row)
}

func (s *Server) changeMCPConnection(w http.ResponseWriter, r *http.Request) {
	if !aiAllowed(w, r, "teacher") {
		return
	}
	id, ok := aiID(w, r, "id")
	if !ok {
		return
	}
	ctx, classID := r.Context(), currentUser(r).ClassID
	connect := strings.HasSuffix(r.URL.Path, "/connect")
	next := "disconnected"
	if connect {
		next = "connecting"
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		mcpError(w, err)
		return
	}
	defer tx.Rollback()
	row, err := getMCPServer(ctx, tx, classID, id, true)
	if err != nil {
		mcpError(w, err)
		return
	}
	if err = resetMCP(ctx, tx, classID, id, next); err != nil {
		mcpError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		mcpError(w, err)
		return
	}
	if connect {
		child, cancel := context.WithTimeout(ctx, 8*time.Second)
		var tools []mcpwire.Tool
		token, connectErr := s.mcpOpen(row.credential, classID, id)
		if connectErr == nil {
			tools, connectErr = s.discoverMCP(child, row.Base, token)
		}
		cancel()
		cleanup, done := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer done()
		if err = s.publishMCP(cleanup, classID, id, row.revision+1, tools, connectErr); err != nil {
			mcpError(w, err)
			return
		}
		if connectErr != nil {
			mcpError(w, connectErr)
			return
		}
	}
	row, err = getMCPServer(ctx, s.DB, classID, id, false)
	if err != nil {
		mcpError(w, err)
		return
	}
	writeJSON(w, 200, row)
}

func (s *Server) publishMCP(ctx context.Context, classID, id, revision int64, tools []mcpwire.Tool, connectErr error) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	row, err := getMCPServer(ctx, tx, classID, id, true)
	if err != nil {
		return err
	}
	if row.revision != revision || row.Status != "connecting" {
		return errMCPConflict
	}
	status := "connected"
	var reason any
	if connectErr != nil {
		status = "disconnected"
		var e *dependencyError
		reason = "mcp_discovery_failed"
		if errors.As(connectErr, &e) {
			reason = e.Code
		}
	}
	for _, tool := range tools {
		if _, err = tx.ExecContext(ctx, "INSERT INTO mcp_discovered_tools(server_id,class_id,revision,tool_name,description,input_schema,fingerprint) VALUES(?,?,?,?,?,?,?)", id, classID, revision, tool.Name, tool.Description, []byte(tool.InputSchema), toolFingerprint(tool)); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, "UPDATE mcp_servers SET status=?,last_error_code=? WHERE class_id=? AND id=?", status, reason, classID, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Server) listMCPTools(w http.ResponseWriter, r *http.Request) {
	if !aiAllowed(w, r, "teacher") {
		return
	}
	id, ok := aiID(w, r, "id")
	if !ok {
		return
	}
	ctx, classID := r.Context(), currentUser(r).ClassID
	row, err := getMCPServer(ctx, s.DB, classID, id, false)
	if err != nil {
		mcpError(w, err)
		return
	}
	items := []mcpwire.Tool{}
	rows, err := s.DB.QueryContext(ctx, `SELECT d.tool_name,d.description,d.input_schema FROM mcp_discovered_tools d JOIN mcp_servers m
 ON m.id=d.server_id AND m.class_id=d.class_id AND m.revision=d.revision WHERE m.class_id=? AND m.id=? AND m.status='connected' AND m.revision=? ORDER BY d.tool_name`, classID, id, row.revision)
	if err != nil {
		mcpError(w, err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var tool mcpwire.Tool
		var schema []byte
		if err = rows.Scan(&tool.Name, &tool.Description, &schema); err != nil {
			mcpError(w, err)
			return
		}
		tool.InputSchema = schema
		items = append(items, tool)
	}
	if err = rows.Err(); err != nil {
		mcpError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items, "status": row.Status})
}

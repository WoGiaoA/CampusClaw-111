package campus

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
)

func decodeAIJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		aiError(w, 400, "invalid_input", "需要 application/json 格式")
		return false
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 16<<10))
	var limit *http.MaxBytesError
	if errors.As(err, &limit) {
		aiError(w, 413, "request_too_large", "请求超过大小限制")
		return false
	}
	trimmed := bytes.TrimSpace(raw)
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err != nil || !utf8.Valid(raw) || len(trimmed) == 0 || trimmed[0] != '{' ||
		decoder.Decode(target) != nil || decoder.Decode(new(any)) != io.EOF {
		aiError(w, 400, "invalid_input", "请求字段或 JSON 格式无效")
		return false
	}
	return true
}

func aiError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": message, "code": code})
}

func aiDBError(w http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		aiError(w, 404, "not_found", "资源不存在")
		return
	}
	aiError(w, 503, "unavailable", "服务暂不可用")
}

func aiAllowed(w http.ResponseWriter, r *http.Request, role string) bool {
	u := currentUser(r)
	if (role != "" && u.Role != role) || (r.Method != "GET" && !validCSRF(r, u)) {
		aiError(w, 403, "forbidden", "没有操作权限或 CSRF 校验失败")
		return false
	}
	return true
}

func aiID(w http.ResponseWriter, r *http.Request, key string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(key), 10, 64)
	if err != nil || id < 1 {
		aiError(w, 404, "not_found", "资源不存在")
		return 0, false
	}
	return id, true
}

func aiText(s string, min, max int) bool {
	n := utf8.RuneCountInString(s)
	return utf8.ValidString(s) && n >= min && n <= max
}

type aiApp struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Enabled     bool    `json:"enabled"`
	RolePrompt  string  `json:"role_prompt"`
	TemplateID  *int64  `json:"template_id"`
	SkillIDs    []int64 `json:"skill_ids"`
	MaterialIDs []int64 `json:"material_ids"`
}

type aiReader interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func aiIDs(ctx context.Context, db aiReader, query string, args ...any) ([]int64, error) {
	ids := []int64{}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

const appFields = "id,name,enabled,COALESCE(prompt_text,''),template_id"
const activeMaterialBindings = "SELECT b.material_id FROM assistant_materials b JOIN materials m ON m.id=b.material_id AND m.class_id=b.class_id WHERE b.assistant_id=? AND b.class_id=? AND m.deleted_at IS NULL ORDER BY b.material_id"

func scanApp(row scanner) (aiApp, error) {
	var a aiApp
	err := row.Scan(&a.ID, &a.Name, &a.Enabled, &a.RolePrompt, &a.TemplateID)
	return a, err
}

func appBindings(ctx context.Context, db aiReader, a *aiApp, classID int64) error {
	var err error
	a.SkillIDs, err = aiIDs(ctx, db, "SELECT skill_id FROM assistant_skills WHERE assistant_id=? AND class_id=? ORDER BY skill_id", a.ID, classID)
	if err != nil {
		return err
	}
	a.MaterialIDs, err = aiIDs(ctx, db, activeMaterialBindings, a.ID, classID)
	return err
}

func (s *Server) listAIApps(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	ctx := r.Context()
	filter := ""
	if u.Role == "student" {
		filter = " AND enabled"
	}
	rows, err := s.DB.QueryContext(ctx, "SELECT "+appFields+" FROM assistants WHERE class_id=? AND deleted_at IS NULL"+filter+" ORDER BY id", u.ClassID)
	if err != nil {
		aiDBError(w, err)
		return
	}
	defer rows.Close()
	apps := []aiApp{}
	for rows.Next() {
		app, err := scanApp(rows)
		if err != nil {
			aiDBError(w, err)
			return
		}
		apps = append(apps, app)
	}
	if err = rows.Err(); err != nil {
		aiDBError(w, err)
		return
	}
	rows.Close()
	if u.Role == "student" {
		type publicApp struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		}
		items := make([]publicApp, 0, len(apps))
		for _, a := range apps {
			items = append(items, publicApp{a.ID, a.Name})
		}
		writeJSON(w, 200, map[string]any{"items": items})
		return
	}
	for i := range apps {
		if err := appBindings(ctx, s.DB, &apps[i], u.ClassID); err != nil {
			aiDBError(w, err)
			return
		}
	}
	writeJSON(w, 200, map[string]any{"items": apps})
}

func bindingIDs(ids []int64, max int) bool {
	if ids == nil || len(ids) > max {
		return false
	}
	seen := map[int64]bool{}
	for _, id := range ids {
		if id < 1 || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}

func aiReferences(ctx context.Context, tx *sql.Tx, table string, classID int64, ids []int64) error {
	filter := ""
	if table == "materials" {
		filter = " AND deleted_at IS NULL"
	}
	for _, id := range ids {
		var found int64
		if err := tx.QueryRowContext(ctx, "SELECT id FROM "+table+" WHERE class_id=? AND id=?"+filter+" FOR UPDATE", classID, id).Scan(&found); err != nil {
			return err
		}
	}
	return nil
}

func replaceAIBindings(ctx context.Context, tx *sql.Tx, table, column string, appID, classID int64, ids []int64) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE assistant_id=? AND class_id=?", appID, classID); err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, "INSERT INTO "+table+"(assistant_id,class_id,"+column+") VALUES(?,?,?)", appID, classID, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) patchAIApp(w http.ResponseWriter, r *http.Request) {
	if !aiAllowed(w, r, "teacher") {
		return
	}
	id, ok := aiID(w, r, "app_id")
	if !ok {
		return
	}
	var in struct {
		Name      *string         `json:"name"`
		Role      *string         `json:"role_prompt"`
		Enabled   *bool           `json:"enabled"`
		Template  json.RawMessage `json:"template_id"`
		Skills    json.RawMessage `json:"skill_ids"`
		Materials json.RawMessage `json:"material_ids"`
	}
	if !decodeAIJSON(w, r, &in) {
		return
	}
	bad := func() { aiError(w, 400, "invalid_input", "应用配置无效") }
	var templateID *int64
	if in.Template != nil && (json.Unmarshal(in.Template, &templateID) != nil || (templateID != nil && *templateID < 1)) {
		bad()
		return
	}
	var skills, materials []int64
	if in.Skills != nil && (json.Unmarshal(in.Skills, &skills) != nil || !bindingIDs(skills, 50)) {
		bad()
		return
	}
	if in.Materials != nil && (json.Unmarshal(in.Materials, &materials) != nil || !bindingIDs(materials, 100)) {
		bad()
		return
	}
	ctx := r.Context()
	classID := currentUser(r).ClassID
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		aiDBError(w, err)
		return
	}
	defer tx.Rollback()
	app, err := scanApp(tx.QueryRowContext(ctx, "SELECT "+appFields+" FROM assistants WHERE id=? AND class_id=? AND deleted_at IS NULL FOR UPDATE", id, classID))
	if err != nil {
		aiDBError(w, err)
		return
	}
	if in.Name != nil {
		app.Name = strings.TrimSpace(*in.Name)
	}
	if in.Role != nil {
		app.RolePrompt = strings.TrimSpace(*in.Role)
	}
	if in.Enabled != nil {
		app.Enabled = *in.Enabled
	}
	if in.Template != nil {
		app.TemplateID = templateID
	}
	if !aiText(app.Name, 1, 100) || !aiText(app.RolePrompt, 0, 2000) {
		bad()
		return
	}
	if templateID != nil {
		if err = aiReferences(ctx, tx, "prompt_templates", classID, []int64{*templateID}); err != nil {
			aiDBError(w, err)
			return
		}
	}
	if err = aiReferences(ctx, tx, "skills", classID, skills); err != nil {
		aiDBError(w, err)
		return
	}
	if err = aiReferences(ctx, tx, "materials", classID, materials); err != nil {
		aiDBError(w, err)
		return
	}
	if _, err = tx.ExecContext(ctx, "UPDATE assistants SET name=?,prompt_text=?,enabled=?,template_id=? WHERE id=? AND class_id=?", app.Name, app.RolePrompt, app.Enabled, app.TemplateID, id, classID); err != nil {
		aiDBError(w, err)
		return
	}
	if skills != nil {
		if err = replaceAIBindings(ctx, tx, "assistant_skills", "skill_id", id, classID, skills); err != nil {
			aiDBError(w, err)
			return
		}
	}
	if materials != nil {
		if err = replaceAIBindings(ctx, tx, "assistant_materials", "material_id", id, classID, materials); err != nil {
			aiDBError(w, err)
			return
		}
	}
	if err = appBindings(ctx, tx, &app, classID); err != nil {
		aiDBError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		aiDBError(w, err)
		return
	}
	writeJSON(w, 200, app)
}

type aiTemplate struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Body string `json:"body"`
}

func (s *Server) listAITemplates(w http.ResponseWriter, r *http.Request) {
	if !aiAllowed(w, r, "teacher") {
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), "SELECT id,name,body FROM prompt_templates WHERE class_id=? ORDER BY id", currentUser(r).ClassID)
	if err != nil {
		aiDBError(w, err)
		return
	}
	defer rows.Close()
	items := []aiTemplate{}
	for rows.Next() {
		var item aiTemplate
		if err = rows.Scan(&item.ID, &item.Name, &item.Body); err != nil {
			aiDBError(w, err)
			return
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		aiDBError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (s *Server) writeAITemplate(w http.ResponseWriter, r *http.Request) {
	if !aiAllowed(w, r, "teacher") {
		return
	}
	var in struct {
		Name *string `json:"name"`
		Body *string `json:"body"`
	}
	if !decodeAIJSON(w, r, &in) {
		return
	}
	ctx := r.Context()
	classID := currentUser(r).ClassID
	create := r.Method == "POST"
	var item aiTemplate
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		aiDBError(w, err)
		return
	}
	defer tx.Rollback()
	if !create {
		id, ok := aiID(w, r, "template_id")
		if !ok {
			return
		}
		item.ID = id
		err = tx.QueryRowContext(ctx, "SELECT name,body FROM prompt_templates WHERE class_id=? AND id=? FOR UPDATE", classID, id).Scan(&item.Name, &item.Body)
		if err != nil {
			aiDBError(w, err)
			return
		}
	}
	if in.Name != nil {
		item.Name = strings.TrimSpace(*in.Name)
	}
	if in.Body != nil {
		item.Body = strings.TrimSpace(*in.Body)
	}
	if !aiText(item.Name, 1, 100) || !aiText(item.Body, 1, 2000) {
		aiError(w, 400, "invalid_input", "模板名称或正文无效")
		return
	}
	status := 200
	if create {
		result, e := tx.ExecContext(ctx, "INSERT INTO prompt_templates(class_id,name,body) VALUES(?,?,?)", classID, item.Name, item.Body)
		err = e
		if err == nil {
			item.ID, err = result.LastInsertId()
		}
		status = 201
	} else {
		_, err = tx.ExecContext(ctx, "UPDATE prompt_templates SET name=?,body=? WHERE class_id=? AND id=?", item.Name, item.Body, classID, item.ID)
	}
	if err != nil {
		aiDBError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		aiDBError(w, err)
		return
	}
	writeJSON(w, status, item)
}

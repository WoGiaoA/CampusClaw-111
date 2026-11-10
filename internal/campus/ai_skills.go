package campus

import (
	"database/sql"
	"errors"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

var invalidSkill = errors.New("invalid skill")
var oversizedSkill = errors.New("oversized skill")

const (
	skillMaxBytes          = 128 << 10
	skillMaxTextRunes      = 40000
	skillMaxProcedureRunes = 30000
)

type aiSkill struct {
	ID            int64    `json:"id"`
	Name          string   `json:"name"`
	Enabled       bool     `json:"enabled"`
	Conditions    string   `json:"conditions"`
	Procedure     string   `json:"procedure"`
	MaterialTerms string   `json:"material_terms"`
	Tool          string   `json:"declared_tool"`
	ToolRefs      []string `json:"tool_refs"`
	Raw           string   `json:"raw_text"`
}

func skillTerms(text string) []string {
	seen := map[string]bool{}
	var terms []string
	for _, term := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune(",，、;；|", r)
	}) {
		if !seen[term] {
			terms = append(terms, term)
			seen[term] = true
		}
	}
	return terms
}

func validSkillTerms(text string, required bool) bool {
	terms := skillTerms(text)
	if len(terms) > 32 || (required && len(terms) == 0) {
		return false
	}
	for _, term := range terms {
		if !aiText(term, 1, 50) {
			return false
		}
	}
	return true
}

func parseSkill(raw []byte) (aiSkill, error) {
	var k aiSkill
	if len(raw) > skillMaxBytes {
		return k, oversizedSkill
	}
	if !utf8.Valid(raw) {
		return k, invalidSkill
	}
	text := strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(strings.TrimPrefix(string(raw), "\ufeff"))
	if utf8.RuneCountInString(text) > skillMaxTextRunes {
		return k, oversizedSkill
	}
	lines := strings.Split(text, "\n")
	fields := map[string]string{}
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			k.Procedure = strings.TrimSpace(strings.Join(lines[i+1:], "\n"))
			break
		}
		sep := strings.IndexAny(line, ":：")
		if sep < 0 {
			return k, invalidSkill
		}
		key := strings.TrimSpace(line[:sep])
		_, size := utf8.DecodeRuneInString(line[sep:])
		if _, exists := fields[key]; exists {
			return k, invalidSkill
		}
		switch key {
		case "名称", "适用条件", "启用", "资料", "工具":
		default:
			return k, invalidSkill
		}
		fields[key] = strings.TrimSpace(line[sep+size:])
	}
	if utf8.RuneCountInString(k.Procedure) > skillMaxProcedureRunes {
		return k, oversizedSkill
	}
	k.Name, k.Conditions, k.MaterialTerms, k.Tool = fields["名称"], fields["适用条件"], fields["资料"], fields["工具"]
	refs, err := parseToolRefs(k.Tool)
	if !aiText(k.Name, 1, 100) || !aiText(k.Procedure, 1, skillMaxProcedureRunes) || !validSkillTerms(k.Conditions, true) || !validSkillTerms(k.MaterialTerms, false) || err != nil {
		return k, invalidSkill
	}
	k.ToolRefs = refs
	k.Tool = legacyTool(refs)
	switch strings.ToLower(fields["启用"]) {
	case "true", "是":
		k.Enabled = true
	case "false", "否":
	default:
		return k, invalidSkill
	}
	k.Raw = string(raw)
	return k, nil
}

func uploadSkill(w http.ResponseWriter, r *http.Request) (aiSkill, error) {
	r.Body = http.MaxBytesReader(w, r.Body, skillMaxBytes+(8<<10))
	reader, err := r.MultipartReader()
	if err != nil {
		return aiSkill{}, invalidSkill
	}
	part, err := reader.NextPart()
	if err != nil {
		return aiSkill{}, err
	}
	defer part.Close()
	ext := strings.ToLower(filepath.Ext(part.FileName()))
	if part.FormName() != "file" || (ext != ".txt" && ext != ".md") {
		return aiSkill{}, invalidSkill
	}
	raw, err := io.ReadAll(io.LimitReader(part, skillMaxBytes+1))
	if err != nil {
		return aiSkill{}, err
	}
	skill, err := parseSkill(raw)
	if err != nil {
		return aiSkill{}, err
	}
	if _, err := reader.NextPart(); err != io.EOF {
		if err == nil {
			err = invalidSkill
		}
		return aiSkill{}, err
	}
	return skill, nil
}

const skillFields = "id,name,enabled,COALESCE(conditions,''),COALESCE(procedure_text,''),COALESCE(material_terms,''),COALESCE(declared_tool,''),COALESCE(raw_text,'')"

func scanSkill(row scanner) (aiSkill, error) {
	var k aiSkill
	err := row.Scan(&k.ID, &k.Name, &k.Enabled, &k.Conditions, &k.Procedure, &k.MaterialTerms, &k.Tool, &k.Raw)
	return k, err
}

func (s *Server) listAISkills(w http.ResponseWriter, r *http.Request) {
	if !aiAllowed(w, r, "teacher") {
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), "SELECT "+skillFields+" FROM skills WHERE class_id=? ORDER BY id", currentUser(r).ClassID)
	if err != nil {
		aiDBError(w, err)
		return
	}
	defer rows.Close()
	items := []aiSkill{}
	for rows.Next() {
		skill, err := scanSkill(rows)
		if err != nil {
			aiDBError(w, err)
			return
		}
		items = append(items, skill)
	}
	if err = rows.Err(); err != nil {
		aiDBError(w, err)
		return
	}
	rows.Close()
	for i := range items {
		items[i].ToolRefs, err = skillToolRefs(r.Context(), s.DB, currentUser(r).ClassID, items[i].ID)
		if err != nil {
			aiDBError(w, err)
			return
		}
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (s *Server) writeAISkill(w http.ResponseWriter, r *http.Request) {
	if !aiAllowed(w, r, "teacher") {
		return
	}
	create := r.Method == "POST"
	var id int64
	if !create {
		var ok bool
		id, ok = aiID(w, r, "skill_id")
		if !ok {
			return
		}
	}
	media, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	replace := create || media == "multipart/form-data"
	var k aiSkill
	var input struct {
		Enabled    *bool    `json:"enabled"`
		Conditions *string  `json:"conditions"`
		ToolRefs   []string `json:"tool_refs"`
	}
	if replace {
		var err error
		k, err = uploadSkill(w, r)
		if err != nil {
			var size *http.MaxBytesError
			if errors.Is(err, oversizedSkill) || errors.As(err, &size) {
				aiError(w, 413, "skill_too_large", "Skill 文件最大 128 KB，全文最多 40000 字符，规程正文最多 30000 字符")
			} else {
				aiError(w, 400, "invalid_skill", "Skill 文件格式或字段无效")
			}
			return
		}
	} else if !decodeAIJSON(w, r, &input) {
		return
	}
	if input.ToolRefs != nil {
		refs, err := validateToolRefs(input.ToolRefs, 8)
		if err != nil {
			aiError(w, 400, "invalid_tool_refs", "工具引用格式无效")
			return
		}
		input.ToolRefs = refs
	}
	ctx := r.Context()
	classID := currentUser(r).ClassID
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		aiDBError(w, err)
		return
	}
	defer tx.Rollback()
	status := 200
	switch {
	case create:
		var result sql.Result
		result, err = tx.ExecContext(ctx, "INSERT INTO skills(class_id,name,enabled,conditions,procedure_text,material_terms,declared_tool,raw_text) VALUES(?,?,?,?,?,?,?,?)", classID, k.Name, k.Enabled, k.Conditions, k.Procedure, k.MaterialTerms, k.Tool, k.Raw)
		if err == nil {
			k.ID, err = result.LastInsertId()
		}
		status = 201
	default:
		existing, e := scanSkill(tx.QueryRowContext(ctx, "SELECT "+skillFields+" FROM skills WHERE id=? AND class_id=? FOR UPDATE", id, classID))
		if e != nil {
			aiDBError(w, e)
			return
		}
		if !replace {
			k = existing
			if input.ToolRefs != nil {
				k.ToolRefs = input.ToolRefs
				k.Tool = legacyTool(input.ToolRefs)
			}
			if input.Enabled != nil {
				k.Enabled = *input.Enabled
			}
			if input.Conditions != nil {
				k.Conditions = strings.TrimSpace(*input.Conditions)
			}
			if (input.Conditions != nil && !validSkillTerms(k.Conditions, true)) || (k.Enabled && k.Procedure == "") {
				aiError(w, 400, "invalid_skill", "需要有效条件和规程正文")
				return
			}
		}
		k.ID = id
		_, err = tx.ExecContext(ctx, "UPDATE skills SET name=?,enabled=?,conditions=?,procedure_text=?,material_terms=?,declared_tool=?,raw_text=? WHERE id=? AND class_id=?", k.Name, k.Enabled, k.Conditions, k.Procedure, k.MaterialTerms, k.Tool, k.Raw, id, classID)
	}
	if err != nil {
		aiDBError(w, err)
		return
	}
	if replace || input.ToolRefs != nil {
		if err = replaceToolRefs(ctx, tx, "skill_tool_refs", "skill_id", k.ID, classID, k.ToolRefs); err != nil {
			mcpError(w, err)
			return
		}
	}
	k.ToolRefs, err = skillToolRefs(ctx, tx, classID, k.ID)
	if err != nil {
		aiDBError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		aiDBError(w, err)
		return
	}
	writeJSON(w, status, k)
}

func routeSkill(question string, candidates []aiSkill) (*aiSkill, map[int64]int) {
	question = strings.ToLower(question)
	scores := map[int64]int{}
	var best *aiSkill
	for _, skill := range candidates {
		if !skill.Enabled || skill.Procedure == "" {
			continue
		}
		score := 0
		for _, term := range skillTerms(skill.Conditions) {
			if strings.Contains(question, term) {
				score++
			}
		}
		scores[skill.ID] = score
		if score > 0 && (best == nil || score > scores[best.ID] || (score == scores[best.ID] && skill.ID < best.ID)) {
			best = &skill
		}
	}
	return best, scores
}

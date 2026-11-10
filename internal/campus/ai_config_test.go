package campus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
)

func TestAIApplicationConfiguration(t *testing.T) {
	s, h := testServer(t)
	teacher, u := loginAs(t, s, h, "teacher_a")
	student, v := loginAs(t, s, h, "student_a1")
	other, _ := loginAs(t, s, h, "teacher_b")
	call := func(want int, method, path, body string, c *http.Cookie, csrf string) []byte {
		t.Helper()
		status, _, data := request(t, h, method, path, strings.NewReader(body), c, csrf, "application/json")
		if status != want {
			t.Fatalf("%s %s: %d %s", method, path, status, data)
		}
		return data
	}
	apps := func(c *http.Cookie) []aiApp {
		var response struct{ Items []aiApp }
		if err := json.Unmarshal(call(200, "GET", "/api/ai/apps", "", c, ""), &response); err != nil {
			t.Fatal(err)
		}
		if len(response.Items) != 2 {
			t.Fatalf("unexpected apps: %v", response.Items)
		}
		return response.Items
	}
	local, foreign := apps(teacher), apps(other)
	if bytes.Contains(call(200, "GET", "/api/ai/apps", "", student, ""), []byte("role_prompt")) {
		t.Fatal("private app configuration leaked")
	}
	appPath := fmt.Sprintf("/api/ai/apps/%d", local[0].ID)
	call(403, "PATCH", appPath, `{"enabled":false}`, student, v.CSRF)
	call(403, "PATCH", appPath, `{"enabled":false}`, teacher, "")
	call(404, "PATCH", fmt.Sprintf("/api/ai/apps/%d", foreign[0].ID), `{"enabled":false}`, teacher, u.CSRF)
	call(404, "PATCH", appPath, fmt.Sprintf(`{"enabled":false,"template_id":%d}`, *foreign[0].TemplateID), teacher, u.CSRF)
	if !apps(teacher)[0].Enabled {
		t.Fatal("invalid configuration was partially applied")
	}
	var template aiTemplate
	if err := json.Unmarshal(call(201, "POST", "/api/ai/prompt-templates", `{"name":"共享","body":"初始正文"}`, teacher, u.CSRF), &template); err != nil {
		t.Fatal(err)
	}
	for _, app := range local {
		call(200, "PATCH", fmt.Sprintf("/api/ai/apps/%d", app.ID), fmt.Sprintf(`{"template_id":%d,"skill_ids":[]}`, template.ID), teacher, u.CSRF)
	}
	call(200, "PATCH", fmt.Sprintf("/api/ai/prompt-templates/%d", template.ID), `{"body":"更新正文"}`, teacher, u.CSRF)
	call(200, "PATCH", appPath, `{"enabled":false,"template_id":null}`, teacher, u.CSRF)
	var response struct{ Items []aiApp }
	if err := json.Unmarshal(call(200, "GET", "/api/ai/apps", "", student, ""), &response); err != nil || len(response.Items) != 1 {
		t.Fatalf("student apps: %+v %v", response, err)
	}
}

func TestSkillParserAndRouting(t *testing.T) {
	raw := "\ufeff名称：演示\r\n适用条件:判别式、ABC、abc\r\n启用：是\r\n工具：knowledge_search\r\n\r\n请给出提示。"
	k, err := parseSkill([]byte(raw))
	if err != nil || !k.Enabled || k.Procedure != "请给出提示。" || k.Raw != raw {
		t.Fatalf("parse: %+v %v", k, err)
	}
	for _, text := range []string{
		strings.Replace(raw, "工具：knowledge_search", "工具：exec", 1),
		strings.Replace(raw, "工具：knowledge_search", "名称：重复", 1),
		strings.Replace(raw, "工具：knowledge_search", "备注：未知", 1),
		strings.Replace(raw, "启用：是", "", 1),
		string([]byte{255}),
	} {
		if _, err := parseSkill([]byte(text)); err != invalidSkill {
			t.Fatalf("expected invalid, got %v", err)
		}
	}
	if _, err := parseSkill([]byte(raw + strings.Repeat("字", skillMaxProcedureRunes+1))); err != oversizedSkill {
		t.Fatalf("expected size error, got %v", err)
	}
	k.ID = 4
	same := k
	same.ID = 2
	disabled := k
	disabled.ID = 1
	disabled.Enabled = false
	selected, scores := routeSkill("ABC判别式", []aiSkill{k, same, disabled})
	if selected == nil || selected.ID != 2 || scores[2] != 2 || len(scores) != 2 {
		t.Fatalf("route: %+v %+v", selected, scores)
	}
	selected, _ = routeSkill("无关问题", []aiSkill{k, same})
	if selected != nil {
		t.Fatal("zero-score skill selected")
	}
	if !strings.Contains(guideProcedure, "默认不给最终答案") {
		t.Fatal("missing teaching constraint")
	}
}

func TestSkillUploadAndClassIsolation(t *testing.T) {
	s, h := testServer(t)
	a, u := loginAs(t, s, h, "teacher_a")
	b, v := loginAs(t, s, h, "teacher_b")
	send := func(want int, method, path, filename, text string, c *http.Cookie, csrf string) []byte {
		t.Helper()
		var buffer bytes.Buffer
		form := multipart.NewWriter(&buffer)
		f, err := form.CreateFormFile("file", filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = fmt.Fprint(f, text); err != nil {
			t.Fatal(err)
		}
		if err = form.Close(); err != nil {
			t.Fatal(err)
		}
		status, _, data := request(t, h, method, path, &buffer, c, csrf, form.FormDataContentType())
		if status != want {
			t.Fatalf("%s %s: %d %s", method, path, status, data)
		}
		return data
	}
	const file = "名称：自定义\n适用条件：解析\n启用：是\n\n询问思路。"
	var k aiSkill
	if err := json.Unmarshal(send(201, "POST", "/api/ai/skills", "skill.md", file, a, u.CSRF), &k); err != nil {
		t.Fatal(err)
	}
	send(404, "PATCH", fmt.Sprintf("/api/ai/skills/%d", k.ID), "skill.txt", file, b, v.CSRF)
	send(400, "POST", "/api/ai/skills", "skill.txt", "invalid", a, u.CSRF)
	send(413, "POST", "/api/ai/skills", "skill.txt", file+strings.Repeat("字", skillMaxProcedureRunes+1), a, u.CSRF)
	// Exercise storage past MySQL TEXT's byte limit, not just the old parser limit.
	large := "名称：长规程\n适用条件：解析\n启用：是\n\n" + strings.Repeat("字", 25000)
	var big aiSkill
	if err := json.Unmarshal(send(201, "POST", "/api/ai/skills", "large.md", large, a, u.CSRF), &big); err != nil || big.Raw != large {
		t.Fatalf("large upload truncated: %v", err)
	}
	replacement := strings.Replace(large, strings.Repeat("字", 25000), strings.Repeat("文", 26000), 1)
	if err := json.Unmarshal(send(200, "PATCH", fmt.Sprintf("/api/ai/skills/%d", big.ID), "large.md", replacement, a, u.CSRF), &big); err != nil || big.Raw != replacement {
		t.Fatalf("replacement truncated: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := Migrate(context.Background(), s.DB); err != nil {
			t.Fatal(err)
		}
	}
	var saved, procedure string
	if err := s.DB.QueryRow("SELECT raw_text,procedure_text FROM skills WHERE id=?", big.ID).Scan(&saved, &procedure); err != nil || saved != replacement || procedure != strings.Repeat("文", 26000) {
		t.Fatalf("persisted large skill mismatch: %v", err)
	}
	statusList, _, body := request(t, h, "GET", "/api/ai/skills", nil, a, "", "")
	if statusList != 200 || !bytes.Contains(body, []byte(strings.Repeat("文", 26000))) {
		t.Fatal("large skill not fully readable")
	}
	status, _, _ := request(t, h, "PATCH", fmt.Sprintf("/api/ai/skills/%d", k.ID), strings.NewReader(`{"enabled":false}`), a, u.CSRF, "application/json")
	if status != 200 {
		t.Fatal(status)
	}
	if err := s.DB.QueryRow("SELECT enabled FROM skills WHERE id=?", k.ID).Scan(&k.Enabled); err != nil || k.Enabled {
		t.Fatalf("disable did not persist: %v", err)
	}
}

func TestSkillCapacityBoundaries(t *testing.T) {
	header := "名称：容量测试\n适用条件：测试\n启用：是\n\n"
	exactBody := header + strings.Repeat("x", skillMaxProcedureRunes)
	if _, err := parseSkill([]byte(exactBody)); err != nil {
		t.Fatal(err)
	}
	if _, err := parseSkill([]byte(exactBody + "x")); err != oversizedSkill {
		t.Fatalf("body over limit: %v", err)
	}
	if _, err := parseSkill([]byte(header + strings.Repeat(" ", skillMaxTextRunes+1))); err != oversizedSkill {
		t.Fatalf("text over limit: %v", err)
	}
	raw := header + strings.Repeat("😀", skillMaxProcedureRunes)
	remaining := skillMaxBytes - len(raw)
	raw += strings.Repeat("\r\n", remaining/2) + strings.Repeat("\n", remaining%2)
	if len(raw) != skillMaxBytes {
		t.Fatal("bad fixture")
	}
	k, err := parseSkill([]byte(raw))
	if err != nil || len([]rune(k.Procedure)) != skillMaxProcedureRunes {
		t.Fatalf("128KB boundary: %v", err)
	}
	if _, err := parseSkill([]byte(raw + "x")); err != oversizedSkill {
		t.Fatalf("byte over limit: %v", err)
	}
}

package campus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestUIManagementCreateAndDelete(t *testing.T) {
	s, h := testServer(t)
	teacher, tu := loginAs(t, s, h, "teacher_a")
	student, su := loginAs(t, s, h, "student_a1")
	other, ou := loginAs(t, s, h, "student_b1")
	if _, err := s.DB.Exec("INSERT INTO users(username,password_hash,role,class_id) SELECT 'student_peer',password_hash,role,class_id FROM users WHERE username='student_a1'"); err != nil {
		t.Fatal(err)
	}
	s.Config.SeedPasswords["student_peer"] = s.Config.SeedPasswords["student_a1"]
	peer, pu := loginAs(t, s, h, "student_peer")
	call := func(want int, method, path, body string, c *http.Cookie, csrf string) []byte {
		t.Helper()
		status, _, raw := request(t, h, method, path, strings.NewReader(body), c, csrf, "application/json")
		if status != want {
			t.Fatalf("%s %s got %d want %d: %s", method, path, status, want, raw)
		}
		return raw
	}
	call(403, "POST", "/api/ai/apps", `{"name":"test"}`, student, su.CSRF)
	call(403, "POST", "/api/ai/apps", `{"name":"test"}`, teacher, "")
	for _, body := range []string{`{"name":" "}`, `{"name":"test","class_id":2}`, `{"name":"test","enabled":true}`, `{"name":"` + strings.Repeat("字", 101) + `"}`} {
		call(400, "POST", "/api/ai/apps", body, teacher, tu.CSRF)
	}
	var app aiApp
	if err := json.Unmarshal(call(201, "POST", "/api/ai/apps", `{"name":" 自定义应用 "}`, teacher, tu.CSRF), &app); err != nil {
		t.Fatal(err)
	}
	if app.ID == 0 || app.Enabled || app.Name != "自定义应用" || app.TemplateID != nil || len(app.SkillIDs) != 0 || len(app.MaterialIDs) != 0 {
		t.Fatalf("unsafe defaults: %+v", app)
	}
	var class int64
	if err := s.DB.QueryRow("SELECT class_id FROM assistants WHERE id=?", app.ID).Scan(&class); err != nil || class != tu.ClassID {
		t.Fatalf("class %d %v", class, err)
	}
	call(404, "POST", "/api/ai/conversations", fmt.Sprintf(`{"app_id":%d}`, app.ID), student, su.CSRF)
	call(200, "PATCH", fmt.Sprintf("/api/ai/apps/%d", app.ID), `{"enabled":true}`, teacher, tu.CSRF)
	var conv aiConversation
	json.Unmarshal(call(201, "POST", "/api/ai/conversations", fmt.Sprintf(`{"app_id":%d}`, app.ID), student, su.CSRF), &conv)
	path := fmt.Sprintf("/api/ai/conversations/%d", conv.ID)
	call(403, "DELETE", path, `{}`, teacher, tu.CSRF)
	call(403, "DELETE", path, `{}`, student, "")
	call(404, "DELETE", path, `{}`, other, ou.CSRF)
	call(404, "DELETE", path, `{}`, peer, pu.CSRF)
	result, err := s.DB.Exec("INSERT INTO ai_turns(conversation_id,client_request_id,status) VALUES(?,'77111111-1111-4111-a111-111111111111','running')", conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	turn, _ := result.LastInsertId()
	call(409, "DELETE", path, `{}`, student, su.CSRF)
	call(200, "GET", path, "", student, "")
	if _, err = s.DB.Exec("UPDATE ai_turns SET status='completed' WHERE id=?", turn); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec("INSERT INTO ai_messages(turn_id,role,body) VALUES(?,'assistant','saved evidence')", turn); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec("INSERT INTO tool_call_audit(attempt_id,terminal,class_id,user_id,conversation_id,turn_id,tool_ref,stage,status) VALUES('7711111111114111a111111111111111',TRUE,?,?,?,?, 'knowledge_search',1,'success')", su.ClassID, su.ID, conv.ID, turn); err != nil {
		t.Fatal(err)
	}
	call(200, "DELETE", path, `{}`, student, su.CSRF)
	call(404, "DELETE", path, `{}`, student, su.CSRF)
	call(404, "GET", path, "", student, "")
	call(404, "POST", path+"/messages", `{"question":"blocked","client_request_id":"88111111-1111-4111-a111-111111111111"}`, student, su.CSRF)
	var list struct{ Items []aiConversation }
	json.Unmarshal(call(200, "GET", "/api/ai/conversations", "", student, ""), &list)
	for _, v := range list.Items {
		if v.ID == conv.ID {
			t.Fatal("deleted conversation listed")
		}
	}
	for _, table := range []string{"ai_messages", "tool_call_audit"} {
		var n int
		if err = s.DB.QueryRow("SELECT COUNT(*) FROM "+table+" WHERE turn_id=?", turn).Scan(&n); err != nil || n != 1 {
			t.Fatalf("lost %s: %d %v", table, n, err)
		}
	}
	for i := 0; i < 2; i++ {
		if err = Migrate(context.Background(), s.DB); err != nil {
			t.Fatal(err)
		}
	}
	call(404, "GET", path, "", student, "")
}

func TestApplicationDisableAndSoftDelete(t *testing.T) {
	s, h := testServer(t)
	teacher, tu := loginAs(t, s, h, "teacher_a")
	student, su := loginAs(t, s, h, "student_a1")
	foreign, fu := loginAs(t, s, h, "teacher_b")
	call := func(want int, method, path, body string, c *http.Cookie, csrf string) []byte {
		t.Helper()
		status, _, raw := request(t, h, method, path, strings.NewReader(body), c, csrf, "application/json")
		if status != want {
			t.Fatalf("%s %s got %d want %d: %s", method, path, status, want, raw)
		}
		return raw
	}
	var appID, skillID int64
	if err := s.DB.QueryRow("SELECT id FROM assistants WHERE class_id=? AND builtin_key='course-qa'", tu.ClassID).Scan(&appID); err != nil {
		t.Fatal(err)
	}
	if err := s.DB.QueryRow("SELECT id FROM skills WHERE class_id=? AND builtin_key='problem-guide'", tu.ClassID).Scan(&skillID); err != nil {
		t.Fatal(err)
	}
	appPath := fmt.Sprintf("/api/ai/apps/%d", appID)
	call(200, "PATCH", appPath, fmt.Sprintf(`{"skill_ids":[%d]}`, skillID), teacher, tu.CSRF)
	call(200, "PUT", appPath+"/tool-grants", `{"refs":["knowledge_search"]}`, teacher, tu.CSRF)
	var c aiConversation
	json.Unmarshal(call(201, "POST", "/api/ai/conversations", fmt.Sprintf(`{"app_id":%d}`, appID), student, su.CSRF), &c)
	convPath := fmt.Sprintf("/api/ai/conversations/%d", c.ID)
	result, err := s.DB.Exec("INSERT INTO ai_turns(conversation_id,client_request_id,status) VALUES(?,'99111111-1111-4111-a111-111111111111','completed')", c.ID)
	if err != nil {
		t.Fatal(err)
	}
	turnID, _ := result.LastInsertId()
	if _, err = s.DB.Exec("INSERT INTO ai_messages(turn_id,role,body) VALUES(?,'assistant','preserved history')", turnID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec("INSERT INTO tool_call_audit(attempt_id,terminal,class_id,user_id,conversation_id,turn_id,tool_ref,stage,status) VALUES('9911111111114111a111111111111111',TRUE,?,?,?,?, 'knowledge_search',1,'success')", su.ClassID, su.ID, c.ID, turnID); err != nil {
		t.Fatal(err)
	}
	call(200, "PATCH", appPath, `{"enabled":false}`, teacher, tu.CSRF)
	assertNotListed := func(cookie *http.Cookie) {
		t.Helper()
		var rows struct{ Items []aiApp }
		json.Unmarshal(call(200, "GET", "/api/ai/apps", "", cookie, ""), &rows)
		for _, a := range rows.Items {
			if a.ID == appID {
				t.Fatal("unavailable app listed")
			}
		}
	}
	assertNotListed(student)
	history := call(200, "GET", convPath, "", student, "")
	if !strings.Contains(string(history), "preserved history") || !strings.Contains(string(history), `"app_enabled":false`) {
		t.Fatal(string(history))
	}
	call(404, "POST", "/api/ai/conversations", fmt.Sprintf(`{"app_id":%d}`, appID), student, su.CSRF)
	rejected := call(403, "POST", convPath+"/messages", `{"question":"旧页面直调","client_request_id":"99111111-1111-4111-a111-111111111112"}`, student, su.CSRF)
	if !strings.Contains(string(rejected), "app_disabled") {
		t.Fatal(string(rejected))
	}
	var n int
	s.DB.QueryRow("SELECT COUNT(*) FROM ai_turns WHERE conversation_id=?", c.ID).Scan(&n)
	if n != 1 {
		t.Fatal("disabled request inserted a turn")
	}
	call(403, "DELETE", appPath, `{}`, student, su.CSRF)
	call(403, "DELETE", appPath, `{}`, teacher, "")
	call(404, "DELETE", appPath, `{}`, foreign, fu.CSRF)
	// Re-enable permits a new accepted turn; deletion must wait for it to terminate.
	call(200, "PATCH", appPath, `{"enabled":true}`, teacher, tu.CSRF)
	running, err := s.acceptAITurn(context.Background(), su, c.ID, turnInput{"活动轮", "99111111-1111-4111-a111-111111111113"})
	if err != nil {
		t.Fatal(err)
	}
	call(409, "DELETE", appPath, `{}`, teacher, tu.CSRF)
	if _, err = s.DB.Exec("UPDATE ai_turns SET status='failed' WHERE id=?", running.ID); err != nil {
		t.Fatal(err)
	}
	call(200, "DELETE", appPath, `{}`, teacher, tu.CSRF)
	call(404, "DELETE", appPath, `{}`, teacher, tu.CSRF)
	call(404, "PATCH", appPath, `{"enabled":true}`, teacher, tu.CSRF)
	call(404, "GET", appPath+"/tool-grants", "", teacher, "")
	assertNotListed(teacher)
	assertNotListed(student)
	call(403, "POST", convPath+"/messages", `{"question":"删除后直调","client_request_id":"99111111-1111-4111-a111-111111111114"}`, student, su.CSRF)
	call(200, "GET", convPath, "", student, "")
	for i := 0; i < 2; i++ {
		if err = Migrate(context.Background(), s.DB); err != nil {
			t.Fatal(err)
		}
	}
	assertNotListed(teacher)
	if err = s.DB.QueryRow("SELECT COUNT(*) FROM assistants WHERE class_id=? AND builtin_key='course-qa'", tu.ClassID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("seed resurrected %d %v", n, err)
	}
	for _, q := range []string{"SELECT COUNT(*) FROM assistant_skills WHERE assistant_id=?", "SELECT COUNT(*) FROM assistant_tool_grants WHERE assistant_id=?"} {
		if err = s.DB.QueryRow(q, appID).Scan(&n); err != nil || n != 1 {
			t.Fatalf("binding lost: %d %v", n, err)
		}
	}
	for _, table := range []string{"ai_messages", "tool_call_audit"} {
		if err = s.DB.QueryRow("SELECT COUNT(*) FROM "+table+" WHERE turn_id=?", turnID).Scan(&n); err != nil || n != 1 {
			t.Fatalf("evidence lost: %s %d %v", table, n, err)
		}
	}
}

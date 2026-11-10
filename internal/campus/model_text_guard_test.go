package campus

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestModelTextGuardFragments(t *testing.T) {
	for _, marker := range []string{"<|FunctionCallBegin|>", "<|FunctionCallEnd|>", "<tool_call>", "</tool_call>", "[TOOL_CALLS]"} {
		for split := 0; split <= len(marker); split++ {
			var output strings.Builder
			g := modelTextGuard{emit: func(text string) error { output.WriteString(text); return nil }}
			err := g.write("普通前缀" + marker[:split])
			if err == nil {
				err = g.write(marker[split:] + `[{"name":"formula_hint"}]`)
			}
			if err == nil {
				err = g.finish()
			}
			if err == nil || err.Error() != "unexpected_tool_text" {
				t.Fatalf("split %d %q: %v", split, marker, err)
			}
			if strings.ContainsAny(output.String(), "<|[") {
				t.Fatalf("protocol prefix leaked: %s", output.String())
			}
		}
	}
	for _, text := range []string{"请检查系数符号。", "x < 3，数组 [1,2]。", "普通括号 <", "文字 [", "<tools>说明</tools>"} {
		var output strings.Builder
		g := modelTextGuard{emit: func(s string) error { output.WriteString(s); return nil }}
		for _, char := range text {
			if err := g.write(string(char)); err != nil {
				t.Fatal(err)
			}
		}
		if err := g.finish(); err != nil || output.String() != text {
			t.Fatalf("ordinary text changed: %q %v", output.String(), err)
		}
	}
	g := modelTextGuard{emit: func(string) error { return nil }}
	_ = g.write("<|FunctionCa")
	if err := g.finish(); err == nil {
		t.Fatal("truncated internal marker allowed")
	}
	expected := errors.New("client disconnected")
	g = modelTextGuard{emit: func(string) error { return expected }}
	if err := g.write("普通文字"); err != expected {
		t.Fatal("emit error lost")
	}
}

func TestRevokedToolsPlaintextFallback(t *testing.T) {
	s, h := testServer(t)
	s.Config.MCPCredentialKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	teacher, tu := loginAs(t, s, h, "teacher_a")
	student, su := loginAs(t, s, h, "student_a1")
	var appID int64
	if err := s.DB.QueryRow("SELECT id FROM assistants WHERE class_id=? AND builtin_key='problem-guide'", su.ClassID).Scan(&appID); err != nil {
		t.Fatal(err)
	}
	// Retain the Skill's tool declaration while the application's allowlist is empty.
	if _, err := s.DB.Exec("INSERT INTO skill_tool_refs(skill_id,class_id,ref,tool_name) SELECT id,class_id,'knowledge_search','knowledge_search' FROM skills WHERE class_id=? AND builtin_key='problem-guide'", su.ClassID); err != nil {
		t.Fatal(err)
	}
	status, raw := bearerRequest(t, h, "PUT", fmt.Sprintf("/api/ai/apps/%d/tool-grants", appID), teacher.Value, tu.CSRF, map[string]any{"refs": []string{}})
	expectStatus(t, 200, status, raw)
	var sequence atomic.Int32
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Messages []modelMessage
			Tools    json.RawMessage
			Choice   string `json:"tool_choice"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		if len(payload.Tools) != 0 || payload.Choice != "" {
			t.Error("revoked tool definitions sent")
		}
		if len(payload.Messages) == 0 || !strings.Contains(payload.Messages[0].Content, noToolsNotice) {
			t.Error("missing no-tool constraint")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		switch sequence.Add(1) {
		case 1:
			modelFrame(w, "当前工具未获授权，请先检查各系数的符号。")
			modelDone(w)
		case 2:
			for _, piece := range []string{"<", "|FunctionCall", "Begin|>", `[{"name":"formula_hint","parameters":{"topic":"quadratic_equation"}}]`, "<|FunctionCallEnd|>"} {
				modelFrame(w, piece)
			}
			modelDone(w)
		default:
			t.Error("unexpected automatic retry")
		}
	}))
	defer model.Close()
	s.Config.ChatBaseURL = model.URL
	s.Config.ChatModel = "fixture"
	var conversation aiConversation
	status, raw = bearerRequest(t, h, "POST", "/api/ai/conversations", student.Value, su.CSRF, map[string]int64{"app_id": appID})
	expectStatus(t, 201, status, raw)
	json.Unmarshal(raw, &conversation)
	path := fmt.Sprintf("/api/ai/conversations/%d/messages", conversation.ID)
	status, raw = bearerRequest(t, h, "POST", path, student.Value, su.CSRF, turnInput{"判别式怎么做", "cc111111-1111-4111-a111-111111111111"})
	expectStatus(t, 200, status, raw)
	if !strings.Contains(string(raw), "event: done") || !strings.Contains(string(raw), "工具未获授权") || strings.Contains(string(raw), "event: tool") {
		t.Fatal(string(raw))
	}
	status, raw = bearerRequest(t, h, "POST", path, student.Value, su.CSRF, turnInput{"判别式怎么做", "cc111111-1111-4111-a111-111111111112"})
	expectStatus(t, 200, status, raw)
	if !strings.Contains(string(raw), "unexpected_tool_text") || !strings.Contains(string(raw), "已拦截且未保存") || strings.Contains(string(raw), "event: done") || strings.Contains(string(raw), "event: delta") || strings.Contains(string(raw), "event: tool") || strings.Contains(string(raw), "FunctionCall") {
		t.Fatal(string(raw))
	}
	if sequence.Load() != 2 {
		t.Fatal("automatic retry occurred")
	}
	var n int
	for _, q := range []struct {
		sql  string
		want int
	}{
		{"SELECT COUNT(*) FROM ai_messages m JOIN ai_turns t ON t.id=m.turn_id WHERE t.conversation_id=? AND m.role='assistant'", 1},
		{"SELECT COUNT(*) FROM ai_turns WHERE conversation_id=? AND status='failed' AND failure_code='unexpected_tool_text'", 1},
		{"SELECT COUNT(*) FROM tool_call_audit WHERE conversation_id=?", 0},
	} {
		if err := s.DB.QueryRow(q.sql, conversation.ID).Scan(&n); err != nil || n != q.want {
			t.Fatalf("unexpected persisted state %d want %d %v", n, q.want, err)
		}
	}
}

func TestNoToolsMessageCopy(t *testing.T) {
	base := []modelMessage{{Role: "system", Content: "原始规则"}, {Role: "user", Content: "问题"}}
	result := noToolsMessages(base)
	if base[0].Content != "原始规则" || !strings.Contains(result[0].Content, noToolsNotice) || result[1] != base[1] {
		t.Fatal("base was mutated")
	}
	if len(noToolsMessages(nil)) != 1 {
		t.Fatal("empty system missing")
	}
	if !strings.Contains(noToolsMessages([]modelMessage{{Role: "user", Content: "问题"}})[0].Content, noToolsNotice) {
		t.Fatal("system not prepended")
	}
}

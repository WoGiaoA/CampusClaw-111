package campus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAIContextWindowAndSkillChanges(t *testing.T) {
	s, h := testServer(t)
	teacher, tu := loginAs(t, s, h, "teacher_a")
	student, su := loginAs(t, s, h, "student_a1")
	captured := make(chan []modelMessage, 8)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input struct{ Messages []modelMessage }
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			w.WriteHeader(400)
			return
		}
		captured <- input.Messages
		w.Header().Set("Content-Type", "text/event-stream")
		modelFrame(w, "可以先检查条件。")
		modelDone(w)
	}))
	defer upstream.Close()
	s.Config.ChatBaseURL = upstream.URL
	s.Config.ChatModel = "fixture"
	call := func(want int, method, path, token, csrf string, payload any) []byte {
		t.Helper()
		status, body := bearerRequest(t, h, method, path, token, csrf, payload)
		expectStatus(t, want, status, body)
		return body
	}
	var guide, qa, skill, template int64
	if err := s.DB.QueryRow("SELECT a.id,a.template_id,k.id FROM assistants a JOIN assistant_skills b ON b.assistant_id=a.id JOIN skills k ON k.id=b.skill_id WHERE a.class_id=? AND a.builtin_key='problem-guide'", su.ClassID).Scan(&guide, &template, &skill); err != nil {
		t.Fatal(err)
	}
	if err := s.DB.QueryRow("SELECT id FROM assistants WHERE class_id=? AND builtin_key='course-qa'", su.ClassID).Scan(&qa); err != nil {
		t.Fatal(err)
	}
	create := func(app int64) aiConversation {
		t.Helper()
		var c aiConversation
		if err := json.Unmarshal(call(201, "POST", "/api/ai/conversations", student.Value, su.CSRF, map[string]int64{"app_id": app}), &c); err != nil {
			t.Fatal(err)
		}
		return c
	}
	guideConv, qaConv := create(guide), create(qa)
	for i := range 4 {
		turn, err := s.acceptAITurn(context.Background(), su, guideConv.ID, turnInput{fmt.Sprintf("历史问题%d", i), fmt.Sprintf("20000000-0000-4000-a000-%012d", i+1)})
		if err != nil {
			t.Fatal(err)
		}
		text := fmt.Sprintf("%d", i) + strings.Repeat("界", 600)
		if _, err = s.finishAITurn(context.Background(), guideConv.ID, turn.ID, text, aiMetadata{}); err != nil {
			t.Fatal(err)
		}
	}
	failed, err := s.acceptAITurn(context.Background(), su, guideConv.ID, turnInput{"失败问题", "20000000-0000-4000-a000-000000000005"})
	if err != nil {
		t.Fatal(err)
	}
	s.failAITurn(failed.ID, "stream_failed")
	patch := func(path string, body any) { call(200, "PATCH", path, teacher.Value, tu.CSRF, body) }
	patch(fmt.Sprintf("/api/ai/prompt-templates/%d", template), map[string]string{"body": "TEMPLATE_UPDATED"})
	send := func(c aiConversation, id int) []modelMessage {
		t.Helper()
		data := call(200, "POST", fmt.Sprintf("/api/ai/conversations/%d/messages", c.ID), student.Value, su.CSRF, turnInput{"判别式怎么做", fmt.Sprintf("30000000-0000-4000-a000-%012d", id)})
		if !strings.Contains(string(data), "event: done") {
			t.Fatal(string(data))
		}
		return <-captured
	}
	a := send(guideConv, 1)
	b := send(qaConv, 2)
	if len(a) != 7 || a[1].Role != "assistant" || len([]rune(a[1].Content)) != 500 ||
		a[2].Content != "历史问题2" || a[4].Content != "历史问题3" || a[6].Content != "问题：判别式怎么做" ||
		!strings.Contains(a[0].Content, "TEMPLATE_UPDATED") || !strings.Contains(a[0].Content, "技能：解题引导") ||
		len(b) != 2 || a[0].Content == b[0].Content || strings.Contains(b[0].Content, "技能：") {
		t.Fatalf("invalid history or application context: %d / %d messages", len(a), len(b))
	}
	patch(fmt.Sprintf("/api/ai/skills/%d", skill), map[string]bool{"enabled": false})
	if x := send(guideConv, 3); strings.Contains(x[0].Content, "技能：") {
		t.Fatal("disabled skill loaded")
	}
	patch(fmt.Sprintf("/api/ai/skills/%d", skill), map[string]bool{"enabled": true})
	patch(fmt.Sprintf("/api/ai/apps/%d", guide), map[string]any{"skill_ids": []int64{}})
	if x := send(guideConv, 4); strings.Contains(x[0].Content, "技能：") {
		t.Fatal("unbound skill loaded")
	}
}

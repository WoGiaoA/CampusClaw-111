package campus

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func modelFrame(w http.ResponseWriter, text string) {
	b, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]string{"content": text}}}})
	fmt.Fprintf(w, "data: %s\n\n", b)
	w.(http.Flusher).Flush()
}

func modelDone(w http.ResponseWriter) {
	fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
}

type lostDoneWriter struct{ http.ResponseWriter }

func (w lostDoneWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w lostDoneWriter) Flush()                      { w.ResponseWriter.(http.Flusher).Flush() }
func (w lostDoneWriter) Write(data []byte) (int, error) {
	if strings.HasPrefix(string(data), "event: done\n") {
		return 0, io.ErrClosedPipe
	}
	return w.ResponseWriter.Write(data)
}

func TestUpstreamProtocolBounds(t *testing.T) {
	cases := []struct {
		name, payload, want string
	}{
		{"split multiline with usage", "data: {\"choices\":[\n" + "data: {\"index\":0,\"delta\":{\"content\":\"提示\"}}]}\n\n" +
			"data: {\"choices\":[{\"index\":0,\"finish_reason\":\"stop\"}]}\n\ndata: {\"choices\":[],\"usage\":{}}\n\ndata: [DONE]\n\n", "提示"},
		{"missing stop", "data: [DONE]\n\n", "upstream_incomplete"},
		{"bad JSON", "data: {broken\n\n", "upstream_protocol"},
		{"truncated finish", "data: {\"choices\":[{\"index\":0,\"finish_reason\":\"length\"}]}\n\n", "upstream_protocol"},
		{"oversized event", "data: " + strings.Repeat("x", 65000) + "\ndata: " + strings.Repeat("y", 600) + "\n\n", "upstream_limit"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				for _, line := range strings.SplitAfter(tc.payload, "\n") {
					fmt.Fprint(w, line)
					w.(http.Flusher).Flush()
				}
			}))
			defer upstream.Close()
			s := Server{Config: Config{ChatBaseURL: upstream.URL, ChatModel: "test"}}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			var output strings.Builder
			err := s.readChatStream(ctx, nil, func(text string) error { output.WriteString(text); return nil }, func() error { return nil })
			got := output.String()
			if err != nil {
				got = err.Error()
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAIConversationStreaming(t *testing.T) {
	s, h := testServer(t)
	g := newRAGGateway(t)
	g.configure(s)
	teacher, tu := loginAs(t, s, h, "teacher_a")
	student, su := loginAs(t, s, h, "student_a1")
	foreign, fu := loginAs(t, s, h, "student_b1")
	if _, err := s.DB.Exec("INSERT INTO users(username,password_hash,role,class_id) SELECT 'peer_a',password_hash,role,class_id FROM users WHERE id=?", su.ID); err != nil {
		t.Fatal(err)
	}
	s.Config.SeedPasswords["peer_a"] = s.Config.SeedPasswords["student_a1"]
	peer, pu := loginAs(t, s, h, "peer_a")
	var app, mid int64
	if err := s.DB.QueryRow("SELECT a.id,m.id FROM assistants a JOIN materials m ON m.class_id=a.class_id WHERE a.class_id=? AND a.builtin_key='problem-guide'", su.ClassID).Scan(&app, &mid); err != nil {
		t.Fatal(err)
	}
	code, data := bearerRequest(t, h, "PATCH", fmt.Sprintf("/api/ai/apps/%d", app), teacher.Value, tu.CSRF, map[string]any{"material_ids": []int64{mid}, "role_prompt": "ROLE_MARKER"})
	expectStatus(t, 200, code, data)
	code, data = uploadRequest(t, h, teacher, tu.CSRF, "unbound.md", []byte("UNBOUND_SAME_CLASS 怎么做首问 课程导论"), nil)
	expectStatus(t, 201, code, data)
	if err := s.enqueueMissing(context.Background()); err != nil {
		t.Fatal(err)
	}
	for s.processNextIndex(context.Background()) {
	}
	// Force Qdrant to return foreign point IDs as well as the real binding.
	g.mu.Lock()
	g.injectForeign = true
	g.requiredMaterial = mid
	g.mu.Unlock()
	captured := make(chan []modelMessage, 8)
	release := make(chan struct{}, 8)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages  []modelMessage
			Stream    bool
			Model     string
			MaxTokens int `json:"max_tokens"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !req.Stream || req.Model != "test-chat" || req.MaxTokens != 600 {
			t.Errorf("stream request invalid: %v", err)
			w.WriteHeader(400)
			return
		}
		captured <- req.Messages
		w.Header().Set("Content-Type", "text/event-stream")
		modelFrame(w, "第一段")
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		if strings.Contains(req.Messages[len(req.Messages)-1].Content, "断流") {
			return
		}
		if strings.Contains(req.Messages[len(req.Messages)-1].Content, "CLASS_A_ONLY") {
			modelFrame(w, "，下一步 [1]")
		} else {
			modelFrame(w, "，一般性提示")
		}
		modelDone(w)
	}))
	defer upstream.Close()
	s.Config.ChatBaseURL = upstream.URL
	code, data = bearerRequest(t, h, "POST", "/api/ai/conversations", student.Value, su.CSRF, map[string]any{"app_id": app})
	expectStatus(t, 201, code, data)
	var c aiConversation
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/api/ai/conversations/%d", c.ID)
	code, data = bearerRequest(t, h, "GET", path, foreign.Value, fu.CSRF, nil)
	expectStatus(t, 404, code, data)
	code, data = bearerRequest(t, h, "GET", path, peer.Value, pu.CSRF, nil)
	expectStatus(t, 404, code, data)
	code, data = bearerRequest(t, h, "PATCH", fmt.Sprintf("/api/ai/apps/%d", app), teacher.Value, tu.CSRF, map[string]any{"enabled": false})
	expectStatus(t, 200, code, data)
	code, data = bearerRequest(t, h, "POST", "/api/ai/conversations", student.Value, su.CSRF, map[string]any{"app_id": app})
	expectStatus(t, 404, code, data)
	code, data = bearerRequest(t, h, "POST", path+"/messages", student.Value, su.CSRF, turnInput{"停用后拒绝", "10000000-0000-4000-a000-000000000099"})
	expectStatus(t, 403, code, data)
	if !strings.Contains(string(data), "app_disabled") {
		t.Fatal(string(data))
	}
	code, data = bearerRequest(t, h, "PATCH", fmt.Sprintf("/api/ai/apps/%d", app), teacher.Value, tu.CSRF, map[string]any{"enabled": true})
	expectStatus(t, 200, code, data)
	const id1 = "10000000-0000-4000-a000-000000000001"
	const id2 = "10000000-0000-4000-a000-000000000002"
	const id3 = "10000000-0000-4000-a000-000000000003"
	open := func(ctx context.Context, id, question string) *http.Response {
		t.Helper()
		raw, _ := json.Marshal(turnInput{Question: question, RequestID: id})
		req, err := http.NewRequestWithContext(ctx, "POST", h.URL+path+"/messages", strings.NewReader(string(raw)))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", su.CSRF)
		req.Header.Set("Authorization", "Bearer "+student.Value)
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { response.Body.Close() })
		return response
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	r := open(ctx, id1, "怎么做首问")
	if r.StatusCode != 200 {
		b, _ := io.ReadAll(r.Body)
		t.Fatalf("open: %d %s", r.StatusCode, b)
	}
	buffered := bufio.NewReader(r.Body)
	var prefix strings.Builder
	for !strings.Contains(prefix.String(), "第一段") {
		line, err := buffered.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		prefix.WriteString(line)
	}
	if !strings.Contains(prefix.String(), "event: meta") || !strings.Contains(prefix.String(), "解题引导") {
		t.Fatal(prefix.String())
	}
	messages := <-captured
	if len(messages) != 2 || !strings.Contains(messages[0].Content, "ROLE_MARKER") || !strings.Contains(messages[0].Content, guideProcedure) ||
		!strings.Contains(messages[1].Content, "CLASS_A_ONLY") || strings.Contains(messages[1].Content, "CLASS_B_ONLY") ||
		strings.Contains(messages[1].Content, "UNBOUND_SAME_CLASS") ||
		strings.Count(messages[1].Content, "怎么做首问") != 1 {
		t.Fatalf("messages: %+v", messages)
	}
	for _, dupe := range []turnInput{{"怎么做首问", id1}, {"修改", id1}, {"并发", id2}} {
		result := open(ctx, dupe.RequestID, dupe.Question)
		if result.StatusCode != 409 {
			t.Fatalf("conflict status %d", result.StatusCode)
		}
	}
	release <- struct{}{}
	rest, err := io.ReadAll(buffered)
	if err != nil || !strings.Contains(string(rest), "event: done") {
		t.Fatalf("stream not saved: %s %v", rest, err)
	}
	code, data = bearerRequest(t, h, "GET", path, student.Value, su.CSRF, nil)
	expectStatus(t, 200, code, data)
	var history struct{ Items []aiTurn }
	if err = json.Unmarshal(data, &history); err != nil {
		t.Fatal(err)
	}
	if len(history.Items) != 1 || len(history.Items[0].Messages) != 2 || history.Items[0].Messages[1].Body != "第一段，下一步 [1]" {
		t.Fatalf("history: %s", data)
	}
	r = open(ctx, id2, "断流")
	messages = <-captured
	if len(messages) != 4 || messages[1].Content != "怎么做首问" || messages[2].Content != "第一段，下一步 [1]" {
		t.Fatalf("follow-up history %+v", messages)
	}
	release <- struct{}{}
	data, err = io.ReadAll(r.Body)
	if err != nil || !strings.Contains(string(data), "event: error") || strings.Contains(string(data), "event: done") {
		t.Fatalf("disconnect: %s %v", data, err)
	}
	s.fail = func(stage string) error {
		if stage == "ai_before_commit" {
			return fmt.Errorf("test commit error")
		}
		return nil
	}
	r = open(ctx, id3, "保存失败")
	<-captured
	release <- struct{}{}
	data, err = io.ReadAll(r.Body)
	if err != nil || !strings.Contains(string(data), "save_failed") {
		t.Fatalf("save failure: %s %v", data, err)
	}
	var assistants, failed int
	if err = s.DB.QueryRow("SELECT COUNT(*) FROM ai_messages WHERE role='assistant'").Scan(&assistants); err != nil || assistants != 1 {
		t.Fatal(assistants, err)
	}
	if err = s.DB.QueryRow("SELECT COUNT(*) FROM ai_turns WHERE status='failed'").Scan(&failed); err != nil || failed != 2 {
		t.Fatal(failed, err)
	}
	if _, err = s.DB.Exec("UPDATE ai_turns SET status='running' WHERE client_request_id=?", id3); err != nil {
		t.Fatal(err)
	}
	if err = s.RecoverAITurns(ctx); err != nil {
		t.Fatal(err)
	}
	var failure string
	if err = s.DB.QueryRow("SELECT failure_code FROM ai_turns WHERE client_request_id=?", id3).Scan(&failure); err != nil || failure != "interrupted" {
		t.Fatal(failure, err)
	}
	s.fail = nil
	g.mu.Lock()
	g.empty = true
	g.mu.Unlock()
	const id4 = "10000000-0000-4000-a000-000000000004"
	r = open(ctx, id4, "风景")
	messages = <-captured
	if strings.Contains(messages[len(messages)-1].Content, "CLASS_A_ONLY") {
		t.Fatal("no-hit turn included evidence")
	}
	release <- struct{}{}
	data, err = io.ReadAll(r.Body)
	if err != nil || !strings.Contains(string(data), `"retrieval_status":"no_hits"`) || !strings.Contains(string(data), "event: done") {
		t.Fatalf("no hits: %s %v", data, err)
	}
	g.mu.Lock()
	g.failSearch = true
	g.mu.Unlock()
	const id5 = "10000000-0000-4000-a000-000000000005"
	r = open(ctx, id5, "检索故障")
	data, err = io.ReadAll(r.Body)
	if r.StatusCode != 503 || err != nil || !strings.Contains(string(data), "retrieval_failed") || len(captured) != 0 {
		t.Fatalf("retrieval fault should not call Chat: %d %s %v", r.StatusCode, data, err)
	}
	code, data = bearerRequest(t, h, "PATCH", fmt.Sprintf("/api/ai/apps/%d", app), teacher.Value, tu.CSRF, map[string]any{"material_ids": []int64{}})
	expectStatus(t, 200, code, data)
	g.mu.Lock()
	before := g.embeddings
	g.mu.Unlock()
	const id6 = "10000000-0000-4000-a000-000000000006"
	r = open(ctx, id6, "没有绑定")
	<-captured
	release <- struct{}{}
	data, err = io.ReadAll(r.Body)
	g.mu.Lock()
	after := g.embeddings
	g.mu.Unlock()
	if err != nil || before != after || !strings.Contains(string(data), `"retrieval_status":"unbound"`) || !strings.Contains(string(data), "event: done") {
		t.Fatalf("unbound turn: %s %v; embedding %d→%d", data, err, before, after)
	}
	handler := s.Handler()
	lost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(lostDoneWriter{w}, r)
	}))
	defer lost.Close()
	h = lost
	const id7 = "10000000-0000-4000-a000-000000000007"
	r = open(ctx, id7, "终止帧丢失")
	<-captured
	release <- struct{}{}
	data, err = io.ReadAll(r.Body)
	if err != nil || strings.Contains(string(data), "event: done") || !strings.Contains(string(data), "event: delta") {
		t.Fatalf("expected missing done with delivered delta: %s %v", data, err)
	}
	if _, err = s.DB.Exec("DELETE FROM knowledge_chunks WHERE material_id=?", mid); err != nil {
		t.Fatal(err)
	}
	code, data = bearerRequest(t, h, "GET", path, student.Value, su.CSRF, nil)
	expectStatus(t, 200, code, data)
	if err = json.Unmarshal(data, &history); err != nil {
		t.Fatal(err)
	}
	last := history.Items[len(history.Items)-1]
	citations := history.Items[0].Messages[1].Metadata.Citations
	if last.RequestID != id7 || last.Status != "completed" || len(last.Messages) != 2 ||
		len(citations) != 1 || citations[0].ChunkID != 0 || citations[0].URL != fmt.Sprintf("/materials?material_id=%d", mid) ||
		!strings.Contains(citations[0].Text, "CLASS_A_ONLY") {
		t.Fatal("lost done or replaced source did not recover from history")
	}
}

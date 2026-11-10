package campus

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const fixtureMCPToken = "lesson06-isolated-mcp-token"

func startLocalMCP(t *testing.T) string {
	t.Helper()
	path := os.Getenv("CAMPUSCLAW_TEST_MCP_EXE")
	if path == "" {
		t.Fatal("run the isolated regression script to build the MCP test process")
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	p := exec.Command(path)
	p.Env = append(os.Environ(), "MCP_LISTEN_ADDR="+addr, "MCP_SERVER_TOKEN="+fixtureMCPToken)
	if err = p.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Process.Kill(); _ = p.Wait() })
	url := "http://" + addr
	client := &http.Client{Timeout: 500 * time.Millisecond}
	for range 30 {
		res, err := client.Get(url + "/health")
		if err == nil {
			res.Body.Close()
			if res.StatusCode == 200 {
				return url
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("local MCP process not ready")
	return ""
}

func TestMCPManagementAndConversation(t *testing.T) {
	s, h := testServer(t)
	s.Config.MCPCredentialKey = base64.StdEncoding.EncodeToString(make([]byte, 32))
	s.Config.mcpTestEndpoint = startLocalMCP(t)
	teacher, tu := loginAs(t, s, h, "teacher_a")
	other, ou := loginAs(t, s, h, "teacher_b")
	student, su := loginAs(t, s, h, "student_a1")
	call := func(want int, method, path, token, csrf string, in any) []byte {
		t.Helper()
		code, body := bearerRequest(t, h, method, path, token, csrf, in)
		if code != want {
			t.Fatalf("%s %s: want %d got %d: %s", method, path, want, code, body)
		}
		if strings.Contains(string(body), fixtureMCPToken) {
			t.Fatal("credential leaked in API")
		}
		return body
	}
	var srv mcpServerRow
	payload := map[string]string{"slug": "campus-tools", "display_name": "教学工具", "base_url": "http://169.254.169.254", "token": fixtureMCPToken}
	call(400, "POST", "/api/mcp/servers", teacher.Value, tu.CSRF, payload)
	payload["base_url"] = s.mcpEndpoint()
	payload["token"] = "wrong"
	if err := json.Unmarshal(call(201, "POST", "/api/mcp/servers", teacher.Value, tu.CSRF, payload), &srv); err != nil || !srv.HasCredential {
		t.Fatal("register:", err)
	}
	prefix := fmt.Sprintf("/api/mcp/servers/%d", srv.ID)
	call(503, "POST", prefix+"/connect", teacher.Value, tu.CSRF, nil)
	call(404, "GET", prefix+"/tools", other.Value, ou.CSRF, nil)
	call(403, "GET", "/api/mcp/servers", student.Value, su.CSRF, nil)
	call(403, "POST", prefix+"/connect", teacher.Value, "", nil)
	call(200, "PATCH", prefix, teacher.Value, tu.CSRF, map[string]string{"token": fixtureMCPToken})
	var encrypted []byte
	if err := s.DB.QueryRow("SELECT credential FROM mcp_servers WHERE id=?", srv.ID).Scan(&encrypted); err != nil || strings.Contains(string(encrypted), fixtureMCPToken) {
		t.Fatal("credential storage:", err)
	}
	for range 2 {
		call(200, "POST", prefix+"/connect", teacher.Value, tu.CSRF, nil)
	}
	if b := call(200, "GET", prefix+"/tools", teacher.Value, tu.CSRF, nil); strings.Count(string(b), "\"inputSchema\"") != 2 {
		t.Fatal(string(b))
	}
	var app, skill int64
	if err := s.DB.QueryRow(`SELECT a.id,b.skill_id FROM assistants a JOIN assistant_skills b ON b.assistant_id=a.id WHERE a.class_id=? AND a.builtin_key='problem-guide'`, su.ClassID).Scan(&app, &skill); err != nil {
		t.Fatal(err)
	}
	refs := []string{"formula_hint@campus-tools", "strategy_hint@campus-tools"}
	oldSnapshot, err := getMCPServer(context.Background(), s.DB, tu.ClassID, srv.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	grants := fmt.Sprintf("/api/ai/apps/%d/tool-grants", app)
	skillPath := fmt.Sprintf("/api/ai/skills/%d", skill)
	call(200, "PATCH", skillPath, teacher.Value, tu.CSRF, map[string]any{"tool_refs": refs})
	call(200, "PUT", grants, teacher.Value, tu.CSRF, map[string]any{"refs": refs})
	call(404, "PUT", grants, teacher.Value, tu.CSRF, map[string]any{"refs": []string{"formula_hint@missing"}})
	if b := call(200, "GET", grants, teacher.Value, tu.CSRF, nil); strings.Count(string(b), "\"available\":true") != 2 {
		t.Fatal(string(b))
	}
	var sequence atomic.Int32
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Messages []struct {
				Role, Content string
				ID            string `json:"tool_call_id"`
			}
			Tools  []modelTool
			Choice string `json:"tool_choice"`
			Stream bool
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		stage := int(sequence.Add(1))
		if !input.Stream {
			t.Error("missing stream mode")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		switch stage {
		case 1, 2:
			if len(input.Tools) != 2 || input.Choice != "auto" {
				t.Error("missing authorized tool definitions")
				w.WriteHeader(400)
				return
			}
			name := "formula_hint"
			arg := `{"topic":"quadratic_equation"}`
			if stage == 2 {
				name = "strategy_hint"
				arg = `{"strategy":"check_discriminant","step":2}`
				last := input.Messages[len(input.Messages)-1]
				if last.Role != "tool" || last.ID != "call_1" || !strings.Contains(last.Content, "check_discriminant") {
					t.Error("actual first tool result was not returned")
				}
			}
			for _, tool := range input.Tools {
				if strings.Contains(tool.Function.Description, map[string]string{"formula_hint": "公式", "strategy_hint": "策略"}[name]) && tool.Function.Name == toolAlias(mcpRef{Ref: name + "@campus-tools", ServerID: &srv.ID}) {
					name = tool.Function.Name
					break
				}
			}
			modelToolFrames(w, stage, name, arg)
		case 3:
			last := input.Messages[len(input.Messages)-1]
			if len(input.Tools) != 0 || input.Choice != "" || last.Role != "tool" || last.ID != "call_2" || !strings.Contains(last.Content, "符号") {
				t.Error("expected actual second tool result and no tools for final stage")
			}
			modelFrame(w, "请先检查各系数的符号。")
			modelDone(w)
		case 4, 6, 8, 10, 12, 13, 15:
			name := toolAlias(mcpRef{Ref: refs[0], ServerID: &srv.ID})
			args := `{"topic":"quadratic_equation"}`
			if stage == 4 {
				ctx, cancel := context.WithTimeout(r.Context(), time.Second)
				defer cancel()
				tx, err := s.DB.BeginTx(ctx, nil)
				if err == nil {
					defer tx.Rollback()
					err = resetMCP(ctx, tx, tu.ClassID, srv.ID, "disconnected")
					if err == nil {
						err = tx.Commit()
					}
				}
				if err != nil {
					t.Error(err)
				}
			}
			if stage == 6 {
				args = `{"topic":"invalid"}`
			}
			modelToolFrames(w, stage, name, args)
		case 5, 7, 9, 11, 14:
			last := input.Messages[len(input.Messages)-1]
			want := map[int]string{5: "未获授权", 7: "参数校验", 9: "执行限制", 11: "执行超时", 14: "未获授权"}[stage]
			if len(input.Tools) != 0 || last.Role != "tool" || !strings.Contains(last.Content, want) {
				t.Errorf("stage %d missing bounded rejection", stage)
			}
			modelFrame(w, "根据已有信息继续。")
			modelDone(w)
		default:
			t.Error("unexpected model call")
		}
	}))
	defer model.Close()
	s.Config.ChatBaseURL = model.URL
	s.Config.ChatModel = "lesson06-mock"
	var conv aiConversation
	if err := json.Unmarshal(call(201, "POST", "/api/ai/conversations", student.Value, su.CSRF, map[string]int64{"app_id": app}), &conv); err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/api/ai/conversations/%d", conv.ID)
	data := string(call(200, "POST", path+"/messages", student.Value, su.CSRF, turnInput{"判别式怎么做", "ae000000-0000-4000-a000-000000000001"}))
	firstDelta := strings.Index(data, "event: delta")
	if firstDelta < 0 || strings.Count(data[:firstDelta], "event: tool") != 4 || !strings.Contains(data, "event: done") {
		t.Fatalf("bad event sequence: %s", data)
	}
	data = string(call(200, "GET", path, student.Value, su.CSRF, nil))
	if !strings.Contains(data, "请先检查各系数的符号。") || strings.Count(data, "\"status\":\"success\"") != 2 {
		t.Fatal(data)
	}
	audit := string(call(200, "GET", "/api/mcp/audit?status=success", teacher.Value, tu.CSRF, nil))
	if !strings.Contains(audit, "strategy_hint@campus-tools") || strings.Contains(audit, "符号") {
		t.Fatal(audit)
	}
	call(403, "GET", "/api/mcp/audit", student.Value, su.CSRF, nil)
	if b := call(200, "GET", "/api/mcp/audit", other.Value, ou.CSRF, nil); !strings.Contains(string(b), "\"items\":[]") {
		t.Fatal(string(b))
	}
	sendDenied := func(index int, reason string) {
		t.Helper()
		input := turnInput{"判别式怎么做", fmt.Sprintf("ae000000-0000-4000-a000-%012d", index)}
		data := string(call(200, "POST", path+"/messages", student.Value, su.CSRF, input))
		if !strings.Contains(data, reason) || !strings.Contains(data, "event: done") || strings.Contains(data, `"status":"running"`) {
			t.Fatalf("wrong rejection %s: %s", reason, data)
		}
	}
	sendDenied(2, "tool_unavailable")
	if err = s.publishMCP(context.Background(), tu.ClassID, srv.ID, oldSnapshot.revision, nil, nil); !errors.Is(err, errMCPConflict) {
		t.Fatalf("a stale discovery publish was accepted: %v", err)
	}
	call(200, "POST", prefix+"/connect", teacher.Value, tu.CSRF, nil)
	if _, err := s.DB.Exec("UPDATE mcp_discovered_tools SET fingerprint=REPEAT('1',64) WHERE server_id=? AND tool_name='formula_hint'", srv.ID); err != nil {
		t.Fatal(err)
	}
	if b := call(200, "GET", grants, teacher.Value, tu.CSRF, nil); strings.Count(string(b), "\"available\":false") != 1 {
		t.Fatal(string(b))
	}
	call(200, "POST", prefix+"/connect", teacher.Value, tu.CSRF, nil)
	sendDenied(3, "invalid_arguments")
	var oldTurn int64
	if err := s.DB.QueryRow("SELECT MIN(id) FROM ai_turns WHERE conversation_id=?", conv.ID).Scan(&oldTurn); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		attempt := fmt.Sprintf("%032d", i)
		if _, err := s.DB.Exec(`INSERT INTO tool_call_audit(attempt_id,terminal,class_id,user_id,conversation_id,turn_id,server_id,tool_ref,stage,status)
		VALUES(?,FALSE,?,?,?,?,?,?,1,'running')`, attempt, su.ClassID, su.ID, conv.ID, oldTurn, srv.ID, refs[0]); err != nil {
			t.Fatal(err)
		}
	}
	sendDenied(4, "tool_limit")
	call(200, "POST", prefix+"/disconnect", teacher.Value, tu.CSRF, nil)
	if b := call(200, "GET", grants, teacher.Value, tu.CSRF, nil); strings.Count(string(b), "\"available\":false") != 2 {
		t.Fatal(string(b))
	}
	call(409, "PATCH", skillPath, teacher.Value, tu.CSRF, map[string]any{"tool_refs": refs})
	if err := s.RecoverMCP(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.RecoverMCP(context.Background()); err != nil {
		t.Fatal(err)
	}
	var recovered int
	if err := s.DB.QueryRow("SELECT COUNT(*) FROM tool_call_audit WHERE reason_code='interrupted'").Scan(&recovered); err != nil || recovered != 10 {
		t.Fatal(recovered, err)
	}
	realEndpoint, _ := url.Parse(s.mcpEndpoint())
	proxy := httputil.NewSingleHostReverseProxy(realEndpoint)
	called := make(chan struct{}, 2)
	blocking := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(strings.NewReader(string(raw)))
		if strings.Contains(string(raw), `"method":"tools/call"`) {
			called <- struct{}{}
			select {
			case <-r.Context().Done():
			case <-time.After(7 * time.Second):
				t.Error("tool request was not cancelled")
			}
			return
		}
		proxy.ServeHTTP(w, r)
	}))
	defer blocking.Close()
	s.Config.mcpTestEndpoint = blocking.URL
	call(200, "PATCH", prefix, teacher.Value, tu.CSRF, map[string]string{"base_url": blocking.URL})
	call(200, "POST", prefix+"/connect", teacher.Value, tu.CSRF, nil)
	var timeoutConv aiConversation
	if err := json.Unmarshal(call(201, "POST", "/api/ai/conversations", student.Value, su.CSRF, map[string]int64{"app_id": app}), &timeoutConv); err != nil {
		t.Fatal(err)
	}
	timeoutPath := fmt.Sprintf("/api/ai/conversations/%d/messages", timeoutConv.ID)
	data = string(call(200, "POST", timeoutPath, student.Value, su.CSRF, turnInput{"判别式怎么做", "ae000000-0000-4000-a000-000000000010"}))
	if !strings.Contains(data, "tool_timeout") || !strings.Contains(data, "event: done") {
		t.Fatal(data)
	}
	<-called
	requestID := "ae000000-0000-4000-a000-000000000011"
	body, _ := json.Marshal(turnInput{"判别式怎么做", requestID})
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "POST", h.URL+timeoutPath, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+student.Value)
	req.Header.Set("X-CSRF-Token", su.CSRF)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-called:
	case <-ctx.Done():
		t.Fatal("tool did not start")
	}
	cancel()
	response.Body.Close()
	for range 30 {
		var n int
		err = s.DB.QueryRow(`SELECT COUNT(*) FROM tool_call_audit a JOIN ai_turns t ON t.id=a.turn_id WHERE t.client_request_id=? AND a.status='cancelled'`, requestID).Scan(&n)
		if err != nil {
			t.Fatal(err)
		}
		if n == 1 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	var n int
	if err = s.DB.QueryRow(`SELECT COUNT(*) FROM tool_call_audit a JOIN ai_turns t ON t.id=a.turn_id WHERE t.client_request_id=? AND a.status='cancelled'`, requestID).Scan(&n); err != nil || n != 1 || sequence.Load() != 12 {
		t.Fatal("cancelled audit or extra model stage", n, err)
	}
	if history := string(call(200, "GET", fmt.Sprintf("/api/ai/conversations/%d", timeoutConv.ID), student.Value, su.CSRF, nil)); !strings.Contains(history, `"status":"cancelled"`) {
		t.Fatal(history)
	}
	var revoked aiConversation
	if err = json.Unmarshal(call(201, "POST", "/api/ai/conversations", student.Value, su.CSRF, map[string]int64{"app_id": app}), &revoked); err != nil {
		t.Fatal(err)
	}
	revokePath := fmt.Sprintf("/api/ai/conversations/%d/messages", revoked.ID)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel2()
	raw, _ := json.Marshal(turnInput{"判别式怎么做", "ae000000-0000-4000-a000-000000000012"})
	req2, _ := http.NewRequestWithContext(ctx2, "POST", h.URL+revokePath, strings.NewReader(string(raw)))
	req2.Header = req.Header.Clone()
	response, err = http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-called:
	case <-ctx2.Done():
		response.Body.Close()
		t.Fatal("revocation fixture did not start")
	}
	call(200, "PUT", grants, teacher.Value, tu.CSRF, map[string]any{"refs": []string{}})
	buf, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || !strings.Contains(string(buf), "tool_unavailable") || !strings.Contains(string(buf), "event: done") {
		t.Fatal(string(buf), err)
	}
	call(200, "PUT", grants, teacher.Value, tu.CSRF, map[string]any{"refs": refs})
	if _, err = s.DB.Exec("RENAME TABLE tool_call_audit TO fixture_tool_call_audit"); err != nil {
		t.Fatal(err)
	}
	defer s.DB.Exec("RENAME TABLE fixture_tool_call_audit TO tool_call_audit")
	data = string(call(200, "POST", revokePath, student.Value, su.CSRF, turnInput{"判别式怎么做", "ae000000-0000-4000-a000-000000000013"}))
	if !strings.Contains(data, "event: error") || strings.Contains(data, `"status":"running"`) || len(called) != 0 || sequence.Load() != 15 {
		t.Fatal("an unaudited tool was dispatched", data)
	}
	t.Log("real MCP process: initialize/list, formula_hint, strategy_hint; Mock Chat stages=3; four tool events before delta; redacted audit and two-class isolation; course gateway calls=0")
}

func TestMCPUpgradeAndCredentialScope(t *testing.T) {
	s, _ := testServer(t)
	s.Config.MCPCredentialKey = base64.StdEncoding.EncodeToString(make([]byte, 32))
	ctx := context.Background()
	var a, b int64
	if err := s.DB.QueryRow("SELECT id FROM classes WHERE code='A'").Scan(&a); err != nil {
		t.Fatal(err)
	}
	if err := s.DB.QueryRow("SELECT id FROM classes WHERE code='B'").Scan(&b); err != nil {
		t.Fatal(err)
	}
	var id int64
	result, err := s.DB.Exec("INSERT INTO skills(class_id,name,declared_tool) VALUES(?,'Legacy','knowledge_search')", a)
	if err != nil {
		t.Fatal(err)
	}
	id, err = result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec("DELETE FROM ai_migrations WHERE name='lesson06-legacy-tool'"); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err = Migrate(ctx, s.DB); err != nil {
			t.Fatal(err)
		}
	}
	refs, err := skillToolRefs(ctx, s.DB, a, id)
	if err != nil || len(refs) != 1 || refs[0] != builtinSearch {
		t.Fatal(refs, err)
	}
	if _, err = s.DB.Exec("INSERT INTO skill_tool_refs(skill_id,class_id,ref,tool_name) VALUES(?,?,?,?)", id, b, builtinSearch, builtinSearch); err == nil {
		t.Fatal("cross-class tool reference was accepted")
	}
	for query, expected := range map[string]int{"SELECT COUNT(*) FROM users": 4, "SELECT COUNT(*) FROM materials": 2, "SELECT COUNT(*) FROM knowledge_entries": 2} {
		var count int
		if err = s.DB.QueryRow(query).Scan(&count); err != nil || count != expected {
			t.Fatal(query, count, err)
		}
	}
	sealed, err := s.mcpSeal(fixtureMCPToken, a, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := s.mcpOpen(sealed, a, 1); err != nil || got != fixtureMCPToken {
		t.Fatal("could not open credential")
	}
	if _, err = s.mcpOpen(sealed, b, 1); err == nil {
		t.Fatal("credential valid in another class")
	}
	if _, err = s.mcpOpen(sealed, a, 2); err == nil {
		t.Fatal("credential valid for another server")
	}
	if _, err = s.DB.Exec("DELETE FROM skill_tool_refs WHERE skill_id=?", id); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, s.DB); err != nil {
		t.Fatal(err)
	}
	err = s.DB.QueryRow("SELECT ref FROM skill_tool_refs WHERE skill_id=?", id).Scan(new(string))
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("migration restored explicit unlink", err)
	}
}

func modelToolFrames(w http.ResponseWriter, id int, name, arg string) {
	first := map[string]any{"index": 0, "id": fmt.Sprintf("call_%d", id), "type": "function", "function": map[string]string{"name": name[:len(name)/2], "arguments": arg[:len(arg)/2]}}
	second := map[string]any{"index": 0, "function": map[string]string{"name": name[len(name)/2:], "arguments": arg[len(arg)/2:]}}
	for _, part := range []any{first, second} {
		raw, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"tool_calls": []any{part}}}}})
		fmt.Fprintf(w, "data: %s\n\n", raw)
		w.(http.Flusher).Flush()
	}
	io.WriteString(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n")
}

func TestMCPClientRejectsRedirectAndMixedStream(t *testing.T) {
	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected.Add(1) }))
	defer target.Close()
	mcp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer mcp.Close()
	s := Server{Config: Config{mcpTestEndpoint: mcp.URL}}
	id := int64(1)
	if err := s.mcpRPC(context.Background(), mcp.URL, fixtureMCPToken, "initialize", &id, nil, new(struct{})); err == nil || redirected.Load() != 0 {
		t.Fatal("redirect accepted")
	}
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		modelFrame(w, "开头")
		modelToolFrames(w, 1, "test_tool", "{}")
	}))
	defer model.Close()
	s.Config.ChatBaseURL = model.URL
	s.Config.ChatModel = "fixture"
	var calls []modelToolCall
	definitions := []modelTool{{"function", modelFunction{"test_tool", "test", json.RawMessage(builtinSchema)}}}
	err := s.readModelStage(context.Background(), nil, definitions, &calls, func(string) error { return nil }, func() error { return nil })
	if err == nil || len(calls) != 0 {
		t.Fatal("mixed stream tool intent accepted")
	}
	var parts toolFragments
	if err = parts.add(json.RawMessage(`[{"index":3,"id":"call_1","function":{"name":"test_tool","arguments":"{}"}}]`)); err != nil {
		t.Fatal(err)
	}
	if _, err = parts.complete(); err == nil {
		t.Fatal("non-contiguous tool index accepted")
	}
}

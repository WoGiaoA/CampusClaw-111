package campus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// HTTP test double for external dependencies only. SQL is real isolated MySQL;
// these synthetic vectors/answers are never used by the application at runtime.
type ragGateway struct {
	mu                                                                      sync.Mutex
	points                                                                  map[int64]map[string]any
	embeddings, chats, searches                                             int
	failWrite, failEmbedding, failSearch, badCitation, empty, injectForeign bool
	prompt                                                                  string
	requiredMaterial                                                        int64
	server                                                                  *httptest.Server
}

func newRAGGateway(t *testing.T) *ragGateway {
	g := &ragGateway{points: map[int64]map[string]any{}}
	g.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.mu.Lock()
		defer g.mu.Unlock()
		var in map[string]any
		if r.Method != "GET" {
			if e := json.NewDecoder(r.Body).Decode(&in); e != nil {
				t.Error(e)
				w.WriteHeader(400)
				return
			}
		}
		switch {
		case r.URL.Path == "/embeddings":
			g.embeddings++
			if r.Header.Get("Authorization") != "Bearer test-embedding-key" {
				t.Error("embedding key missing")
			}
			if g.failEmbedding {
				w.WriteHeader(401)
				return
			}
			data := []any{}
			for i := range in["input"].([]any) {
				data = append(data, map[string]any{"index": i, "embedding": []float64{1, 0}})
			}
			writeJSON(w, 200, map[string]any{"data": data})
		case r.URL.Path == "/chat/completions":
			g.chats++
			if r.Header.Get("Authorization") != "Bearer test-chat-key" {
				t.Error("chat key missing")
			}
			b, _ := json.Marshal(in["messages"])
			g.prompt = string(b)
			answer := "项目成绩占40% [1]"
			if g.badCitation {
				answer = "错误引用 [99]"
			}
			writeJSON(w, 200, map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": answer}, "finish_reason": "stop"}}})
		case r.Method == "GET" && r.URL.Path == collectionPath:
			writeJSON(w, 200, map[string]any{"result": map[string]any{"config": map[string]any{"params": map[string]any{"vectors": map[string]any{"size": 2, "distance": "Cosine"}}}}})
		case strings.HasSuffix(r.URL.Path, "/points/delete"):
			if r.URL.Query().Get("wait") != "true" {
				t.Error("delete not confirmed")
			}
			filter := in["filter"].(map[string]any)["must"].([]any)
			cid := filter[0].(map[string]any)["match"].(map[string]any)["value"].(float64)
			mid := filter[1].(map[string]any)["match"].(map[string]any)["value"].(float64)
			for id, p := range g.points {
				payload := p["payload"].(map[string]any)
				if payload["class_id"] == cid && payload["material_id"] == mid {
					delete(g.points, id)
				}
			}
			writeJSON(w, 200, map[string]any{"result": map[string]string{"status": "completed"}})
		case r.Method == "PUT" && strings.HasSuffix(r.URL.Path, "/points"):
			if g.failWrite {
				w.WriteHeader(400)
				return
			}
			if r.URL.Query().Get("wait") != "true" {
				t.Error("upsert not confirmed")
			}
			for _, p := range in["points"].([]any) {
				point := p.(map[string]any)
				payload := point["payload"].(map[string]any)
				if len(payload) != 5 {
					t.Error("payload must contain IDs only")
				}
				g.points[int64(point["id"].(float64))] = point
			}
			writeJSON(w, 200, map[string]any{"result": map[string]string{"status": "completed"}})
		case strings.HasSuffix(r.URL.Path, "/points/search"):
			g.searches++
			if g.failSearch {
				w.WriteHeader(503)
				return
			}
			cid := in["filter"].(map[string]any)["must"].([]any)[0].(map[string]any)["match"].(map[string]any)["value"].(float64)
			if g.requiredMaterial != 0 {
				must := in["filter"].(map[string]any)["must"].([]any)
				if len(must) != 2 {
					t.Error("material allowlist missing before Top-K")
				} else {
					condition := must[1].(map[string]any)
					ids, _ := condition["match"].(map[string]any)["any"].([]any)
					if condition["key"] != "material_id" || len(ids) != 1 || ids[0] != float64(g.requiredMaterial) {
						t.Error("incorrect vector material allowlist")
					}
				}
			}
			if in["score_threshold"].(float64) != 0.35 || in["limit"].(float64) != 40 {
				t.Error("search bounds")
			}
			result := []any{}
			if !g.empty {
				for id, p := range g.points {
					payload := p["payload"].(map[string]any)
					if payload["class_id"] == cid || g.injectForeign {
						result = append(result, map[string]any{"id": id, "score": 0.9})
					}
				}
				result = append(result, map[string]any{"id": 99999999, "score": 0.99})
			}
			writeJSON(w, 200, map[string]any{"result": result})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(g.server.Close)
	return g
}
func (g *ragGateway) configure(s *Server) {
	s.Config.EmbeddingBaseURL = g.server.URL
	s.Config.EmbeddingModel = "test-embedding"
	s.Config.EmbeddingAPIKey = "test-embedding-key"
	s.Config.EmbeddingDimension = 2
	s.Config.QdrantURL = g.server.URL
	s.Config.ChatBaseURL = g.server.URL
	s.Config.ChatModel = "test-chat"
	s.Config.ChatAPIKey = "test-chat-key"
}
func bearerRequest(t *testing.T, server *httptest.Server, method, path, token, csrf string, value any) (int, []byte) {
	t.Helper()
	var body io.Reader
	if value != nil {
		b, _ := json.Marshal(value)
		body = bytes.NewReader(b)
	}
	r, _ := http.NewRequest(method, server.URL+path, body)
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-CSRF-Token", csrf)
	res, e := http.DefaultClient.Do(r)
	if e != nil {
		t.Fatal(e)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	return res.StatusCode, data
}
func expectStatus(t *testing.T, want, got int, body []byte) {
	t.Helper()
	if got != want {
		t.Fatalf("expected %d got %d: %s", want, got, body)
	}
}

func TestTokenLifecycle(t *testing.T) {
	s, server := testServer(t)
	code, headers, body := request(t, server, "POST", "/api/login", strings.NewReader(`{"username":"teacher_a","password":"isolation-test-a"}`), nil, "", "application/json")
	expectStatus(t, 200, code, nil)
	var login struct {
		User
		Token   string `json:"access_token"`
		Type    string `json:"token_type"`
		Expires int    `json:"expires_in"`
	}
	if json.Unmarshal(body, &login) != nil || len(login.Token) != 64 || login.Type != "Bearer" || login.Expires != 28800 || login.CSRF == "" {
		t.Fatal("login contract")
	}
	var stored string
	if e := s.DB.QueryRow("SELECT id_hash FROM sessions WHERE user_id=?", login.ID).Scan(&stored); e != nil || stored == login.Token || stored != s.sessionHash(login.Token) {
		t.Fatal("token storage")
	}
	code, body = bearerRequest(t, server, "GET", "/api/me", login.Token, "", nil)
	expectStatus(t, 200, code, body)
	if bytes.Contains(body, []byte("access_token")) {
		t.Fatal("me leaked token")
	}
	code, body = bearerRequest(t, server, "GET", "/api/materials", login.Token, "", nil)
	expectStatus(t, 200, code, body)
	code, body = bearerRequest(t, server, "GET", "/api/materials/1/file", login.Token, "", nil)
	expectStatus(t, 200, code, body)
	r := httptest.NewRequest("GET", "/api/me", nil)
	r.AddCookie((&http.Response{Header: headers}).Cookies()[0])
	r.Header.Set("Authorization", "Bearer invalid")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, r)
	if rec.Code != 401 {
		t.Fatal("invalid bearer fell back")
	}
	code, body = bearerRequest(t, server, "POST", "/api/logout", login.Token, "", nil)
	expectStatus(t, 403, code, body)
	code, body = bearerRequest(t, server, "POST", "/api/logout", login.Token, login.CSRF, nil)
	expectStatus(t, 200, code, body)
	code, body = bearerRequest(t, server, "GET", "/api/me", login.Token, "", nil)
	expectStatus(t, 401, code, body)
	cookie, u := loginAs(t, s, server, "teacher_a")
	if _, e := s.DB.Exec("UPDATE users SET must_change_password=TRUE WHERE id=?", u.ID); e != nil {
		t.Fatal(e)
	}
	code, body = bearerRequest(t, server, "POST", "/api/ask", cookie.Value, "", map[string]any{"question": "课程"})
	expectStatus(t, 403, code, body)
	code, body = bearerRequest(t, server, "POST", "/api/password", cookie.Value, u.CSRF, map[string]any{"new_password": "NewPassword123", "confirm_password": "NewPassword123"})
	expectStatus(t, 200, code, body)
	code, body = bearerRequest(t, server, "GET", "/api/me", cookie.Value, "", nil)
	expectStatus(t, 401, code, body)
}

func TestRAGWorkflowAndIsolation(t *testing.T) {
	s, server := testServer(t)
	g := newRAGGateway(t)
	g.configure(s)
	a, u := loginAs(t, s, server, "teacher_a")
	b, _ := loginAs(t, s, server, "teacher_b")
	student, su := loginAs(t, s, server, "student_a1")
	code, body := uploadRequest(t, server, a, u.CSRF, "考核.md", []byte("课程考核：期末项目成绩占40%。\n引用可核对😀。"), nil)
	expectStatus(t, 201, code, body)
	var uploaded map[string]int64
	_ = json.Unmarshal(body, &uploaded)
	mid := uploaded["material_id"]
	if e := s.enqueueMissing(context.Background()); e != nil {
		t.Fatal(e)
	}
	for s.processNextIndex(context.Background()) {
	}
	for _, mode := range []string{"keyword", "vector", "hybrid"} {
		before := g.embeddings
		code, body = bearerRequest(t, server, "POST", "/api/search", student.Value, "", map[string]any{"q": "期末项目", "mode": mode, "class_id": 999, "role": "teacher"})
		expectStatus(t, 200, code, body)
		var res struct{ Hits []Hit }
		_ = json.Unmarshal(body, &res)
		if len(res.Hits) == 0 {
			t.Fatal("expected Chinese retrieval", string(body))
		}
		if mode == "keyword" && before != g.embeddings {
			t.Fatal("keyword made external call")
		}
	}
	code, body = bearerRequest(t, server, "POST", "/api/ask", student.Value, "", map[string]any{"question": "期末项目成绩占多少？", "system": "INJECTED_SYSTEM", "history": []any{"HISTORY_SECRET"}, "class_id": 2})
	expectStatus(t, 200, code, body)
	var answer struct {
		Answer    string
		Citations []Citation
	}
	if json.Unmarshal(body, &answer) != nil || !validAnswer(answer.Answer, len(answer.Citations)) || len(answer.Citations) == 0 {
		t.Fatal("answer contract")
	}
	if strings.Contains(g.prompt, "CLASS_B_ONLY") || strings.Contains(g.prompt, "INJECTED_SYSTEM") || strings.Contains(g.prompt, "HISTORY_SECRET") {
		t.Fatal("untrusted prompt leaked")
	}
	h := answer.Citations[0]
	if h.MaterialID != mid || h.Number != 1 || h.Text == "" {
		t.Fatal("bad source")
	}
	source := fmt.Sprintf("/api/materials/%d/chunks/%d", h.MaterialID, h.ChunkID)
	code, body = bearerRequest(t, server, "GET", source, student.Value, "", nil)
	expectStatus(t, 200, code, body)
	code, body = bearerRequest(t, server, "GET", source, b.Value, "", nil)
	expectStatus(t, 404, code, body)
	code, body = bearerRequest(t, server, "GET", fmt.Sprintf("/api/materials/1/chunks/%d", h.ChunkID), a.Value, "", nil)
	expectStatus(t, 404, code, body)
	g.injectForeign = true
	code, body = bearerRequest(t, server, "POST", "/api/search", a.Value, "", map[string]any{"q": "期末项目", "mode": "vector"})
	expectStatus(t, 200, code, body)
	if bytes.Contains(body, []byte("CLASS_B_ONLY")) || bytes.Contains(body, []byte("B 班")) {
		t.Fatal("foreign candidate leaked")
	}
	// No candidates => no generation. The question deliberately has no ngram hit.
	g.empty = true
	before := g.chats
	code, body = bearerRequest(t, server, "POST", "/api/ask", a.Value, "", map[string]any{"question": "zzzzunrelated"})
	expectStatus(t, 200, code, body)
	if g.chats != before || !bytes.Contains(body, []byte(noEvidence)) {
		t.Fatal("empty generated")
	}
	g.empty = false
	g.badCitation = true
	code, body = bearerRequest(t, server, "POST", "/api/ask", a.Value, "", map[string]any{"question": "期末项目"})
	expectStatus(t, 503, code, body)
	g.badCitation = false
	g.failSearch = true
	code, body = bearerRequest(t, server, "POST", "/api/search", a.Value, "", map[string]any{"q": "期末项目", "mode": "hybrid"})
	expectStatus(t, 503, code, body)
	code, body = bearerRequest(t, server, "POST", "/api/search", a.Value, "", map[string]any{"q": "期末项目", "mode": "keyword"})
	expectStatus(t, 200, code, body)
	code, body = bearerRequest(t, server, "GET", "/health", a.Value, "", nil)
	expectStatus(t, 200, code, body)
	g.failSearch = false
	path := fmt.Sprintf("/api/materials/%d/reindex", mid)
	code, body = bearerRequest(t, server, "POST", path, student.Value, su.CSRF, nil)
	expectStatus(t, 403, code, body)
	code, body = bearerRequest(t, server, "POST", path, a.Value, "", nil)
	expectStatus(t, 403, code, body)
	code, body = bearerRequest(t, server, "POST", path, b.Value, u.CSRF, nil)
	expectStatus(t, 403, code, body)
	code, body = bearerRequest(t, server, "POST", path, a.Value, u.CSRF, nil)
	expectStatus(t, 202, code, body)
	code, body = bearerRequest(t, server, "POST", path, a.Value, u.CSRF, nil)
	expectStatus(t, 409, code, body)
	code, body = bearerRequest(t, server, "GET", source, a.Value, "", nil)
	expectStatus(t, 404, code, body)
	g.failWrite = true
	s.processNextIndex(context.Background())
	code, body = bearerRequest(t, server, "GET", fmt.Sprintf("/api/materials/%d", mid), a.Value, "", nil)
	expectStatus(t, 200, code, body)
	if !bytes.Contains(body, []byte(`"index_status":"failed"`)) || !bytes.Contains(body, []byte("40%")) {
		t.Fatal("failed index lost original", string(body))
	}
	code, body = bearerRequest(t, server, "POST", "/api/search", a.Value, "", map[string]any{"q": "期末项目", "mode": "keyword"})
	expectStatus(t, 200, code, body)
	var empty struct{ Hits []Hit }
	_ = json.Unmarshal(body, &empty)
	if len(empty.Hits) != 0 {
		t.Fatal("partial batch visible")
	}
	g.failWrite = false
	code, body = bearerRequest(t, server, "POST", path, a.Value, u.CSRF, nil)
	expectStatus(t, 202, code, body)
	s.processNextIndex(context.Background())
	code, body = bearerRequest(t, server, "POST", "/api/ask", a.Value, "", map[string]any{"question": "期末项目"})
	expectStatus(t, 200, code, body)
	s.Config.EmbeddingModel = "changed"
	code, body = bearerRequest(t, server, "POST", "/api/search", a.Value, "", map[string]any{"q": "项目", "mode": "vector"})
	expectStatus(t, 503, code, body)
}

func TestRetrievalMigrationRecoveryAndConstraints(t *testing.T) {
	s, server := testServer(t)
	g := newRAGGateway(t)
	g.configure(s)
	ctx := context.Background()
	var before, after string
	if e := s.DB.QueryRow("SELECT password_hash FROM users WHERE id=1").Scan(&before); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		if e := Migrate(ctx, s.DB); e != nil {
			t.Fatal(e)
		}
	}
	_ = s.DB.QueryRow("SELECT password_hash FROM users WHERE id=1").Scan(&after)
	if before != after {
		t.Fatal("migration changed credentials")
	}
	if e := s.enqueueMissing(ctx); e != nil {
		t.Fatal(e)
	}
	if e := s.enqueueMissing(ctx); e != nil {
		t.Fatal(e)
	}
	if _, e := s.DB.Exec("INSERT INTO knowledge_index_jobs(material_id,knowledge_entry_id,class_id) VALUES(999,1,1)"); e == nil {
		t.Fatal("wrong material accepted")
	}
	if _, e := s.DB.Exec(`INSERT INTO knowledge_chunks(index_job_id,generation,material_id,knowledge_entry_id,class_id,chunk_index,chunk_text,char_start,char_end) VALUES(1,1,2,1,1,1,'wrong',0,5)`); e == nil {
		t.Fatal("wrong association accepted")
	}
	if _, e := s.DB.Exec("UPDATE knowledge_index_jobs SET status='running' WHERE material_id=1"); e != nil {
		t.Fatal(e)
	}
	workerCtx, cancel := context.WithCancel(ctx)
	if e := s.StartIndexer(workerCtx); e != nil {
		cancel()
		t.Fatal(e)
	}
	deadline := time.Now().Add(10 * time.Second)
	status := ""
	for time.Now().Before(deadline) {
		_ = s.DB.QueryRow("SELECT status FROM knowledge_index_jobs WHERE material_id=1").Scan(&status)
		if status == "ready" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	if status != "ready" {
		t.Fatal("interrupted running not recovered", status)
	}
	a, u := loginAs(t, s, server, "teacher_a")
	code, body := bearerRequest(t, server, "POST", "/api/materials/2/reindex", a.Value, u.CSRF, nil)
	expectStatus(t, 404, code, body)
	beforeCalls := g.embeddings
	for _, value := range []any{map[string]any{"q": " "}, map[string]any{"q": "x", "mode": "bad"}, map[string]any{"q": "x", "limit": 0}, map[string]any{"q": strings.Repeat("字", 501)}, map[string]any{"q": "x", "limit": 1.5}} {
		code, body = bearerRequest(t, server, "POST", "/api/search", a.Value, "", value)
		expectStatus(t, 400, code, body)
	}
	if beforeCalls != g.embeddings {
		t.Fatal("invalid requests called gateway")
	}
	code, _, body = request(t, server, "POST", "/api/ask", strings.NewReader(`{"question":"课程"}`), nil, "", "application/json")
	expectStatus(t, 401, code, body)
}

// Restores the third-lesson schema in the uniquely named test DB, then upgrades.
func TestLegacyRetrievalMigration(t *testing.T) {
	s, server := testServer(t)
	a, u := loginAs(t, s, server, "teacher_a")
	before := snapshot(t, s)
	var originalHash, originalText string
	if e := s.DB.QueryRow("SELECT password_hash FROM users WHERE id=?", u.ID).Scan(&originalHash); e != nil {
		t.Fatal(e)
	}
	if e := s.DB.QueryRow("SELECT body_text FROM knowledge_entries WHERE material_id=1").Scan(&originalText); e != nil {
		t.Fatal(e)
	}
	for _, ddl := range []string{"DROP TABLE knowledge_chunks", "DROP TABLE knowledge_index_jobs", "ALTER TABLE knowledge_entries DROP INDEX entry_material_class"} {
		if _, e := s.DB.Exec(ddl); e != nil {
			t.Fatal(e)
		}
	}
	for i := 0; i < 2; i++ {
		if e := Migrate(context.Background(), s.DB); e != nil {
			t.Fatal(e)
		}
	}
	if before != snapshot(t, s) {
		t.Fatal("legacy material or files changed")
	}
	var hash, text string
	_ = s.DB.QueryRow("SELECT password_hash FROM users WHERE id=?", u.ID).Scan(&hash)
	_ = s.DB.QueryRow("SELECT body_text FROM knowledge_entries WHERE material_id=1").Scan(&text)
	if hash != originalHash || text != originalText {
		t.Fatal("legacy credentials/text changed")
	}
	code, body := bearerRequest(t, server, "GET", "/api/me", a.Value, "", nil)
	expectStatus(t, 200, code, body)
	if e := s.enqueueMissing(context.Background()); e != nil {
		t.Fatal(e)
	}
	if !s.processNextIndex(context.Background()) {
		t.Fatal("missing configuration task not processed")
	}
	code, body = bearerRequest(t, server, "GET", "/api/materials/1", a.Value, "", nil)
	expectStatus(t, 200, code, body)
	if !bytes.Contains(body, []byte(`"index_status":"failed"`)) || !bytes.Contains(body, []byte("CLASS_A_ONLY")) {
		t.Fatal("missing config must preserve original")
	}
}

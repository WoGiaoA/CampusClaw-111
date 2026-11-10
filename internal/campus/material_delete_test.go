package campus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func deletionUpload(t *testing.T, s *Server, h *httptest.Server, c *http.Cookie, u User) (int64, int64, string) {
	t.Helper()
	status, body := uploadRequest(t, h, c, u.CSRF, "delete-fixture.md", []byte("删除验证专属材料。判别式是b平方减4ac。"), nil)
	expectStatus(t, 201, status, body)
	var ids map[string]int64
	if err := json.Unmarshal(body, &ids); err != nil {
		t.Fatal(err)
	}
	var key string
	if err := s.DB.QueryRow("SELECT storage_key FROM materials WHERE id=?", ids["material_id"]).Scan(&key); err != nil {
		t.Fatal(err)
	}
	path, err := s.storagePath(key)
	if err != nil {
		t.Fatal(err)
	}
	return ids["material_id"], ids["knowledge_entry_id"], path
}

func deletionCount(t *testing.T, s *Server, want int, q string, args ...any) {
	t.Helper()
	var got int
	if err := s.DB.QueryRow(q, args...).Scan(&got); err != nil || got != want {
		t.Fatalf("count got=%d want=%d err=%v query=%s", got, want, err, q)
	}
}

func TestMaterialDeletionLifecycle(t *testing.T) {
	s, h := testServer(t)
	ctx := context.Background()
	g := newRAGGateway(t)
	g.configure(s)
	real := os.Getenv("QDRANT_TEST_URL")
	if real != "" {
		if !strings.HasPrefix(real, "http://127.0.0.1:") || os.Getenv("QDRANT_TEST_CONFIRM_ISOLATED") != "true" {
			t.Fatal("requires isolated Qdrant")
		}
		s.Config.QdrantURL = real
	}
	a, u := loginAs(t, s, h, "teacher_a")
	student, su := loginAs(t, s, h, "student_a1")
	b, bu := loginAs(t, s, h, "teacher_b")
	mid, kid, path := deletionUpload(t, s, h, a, u)
	for s.processNextIndex(ctx) {
	}
	var app int64
	if err := s.DB.QueryRow("SELECT id FROM assistants WHERE class_id=? ORDER BY id LIMIT 1", u.ClassID).Scan(&app); err != nil {
		t.Fatal(err)
	}
	appPath := fmt.Sprintf("/api/ai/apps/%d", app)
	code, body := bearerRequest(t, h, "PATCH", appPath, a.Value, u.CSRF, map[string]any{"material_ids": []int64{mid}})
	expectStatus(t, 200, code, body)
	hits, err := s.retrieveScoped(ctx, u.ClassID, "判别式", "keyword", 4, []int64{mid})
	if err != nil || len(hits) == 0 {
		t.Fatal(err, hits)
	}
	var conv aiConversation
	code, body = bearerRequest(t, h, "POST", "/api/ai/conversations", student.Value, su.CSRF, map[string]any{"app_id": app})
	expectStatus(t, 201, code, body)
	_ = json.Unmarshal(body, &conv)
	turn, err := s.acceptAITurn(ctx, su, conv.ID, turnInput{"判别式怎么做", "99999999-0000-4000-a000-000000000001"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.finishAITurn(ctx, conv.ID, turn.ID, "已保存的历史回答 [1]", aiMetadata{Citations: []Citation{{Number: 1, Hit: hits[0]}}}); err != nil {
		t.Fatal(err)
	}
	// Repeat migrations against live bindings/history and preserve all existing data.
	for range 2 {
		if err = Migrate(ctx, s.DB); err != nil {
			t.Fatal(err)
		}
	}
	deletionCount(t, s, 4, "SELECT COUNT(*) FROM users")
	deletionCount(t, s, 1, "SELECT COUNT(*) FROM assistant_materials WHERE assistant_id=? AND material_id=?", app, mid)
	endpoint := fmt.Sprintf("/api/materials/%d", mid)
	for _, x := range []struct {
		token, csrf string
		want        int
	}{{"", "", 401}, {student.Value, su.CSRF, 403}, {a.Value, "", 403}, {a.Value, "wrong", 403}, {b.Value, bu.CSRF, 404}} {
		code, body = bearerRequest(t, h, "DELETE", endpoint, x.token, x.csrf, nil)
		expectStatus(t, x.want, code, body)
	}
	s.fail = func(stage string) error {
		if stage == "delete-before-commit" {
			return errors.New("injected")
		}
		return nil
	}
	code, body = bearerRequest(t, h, "DELETE", endpoint, a.Value, u.CSRF, nil)
	expectStatus(t, 503, code, body)
	s.fail = nil
	deletionCount(t, s, 0, "SELECT COUNT(*) FROM material_deletions")
	deletionCount(t, s, 1, "SELECT COUNT(*) FROM assistant_materials WHERE material_id=?", mid)
	code, body = bearerRequest(t, h, "DELETE", endpoint, a.Value, u.CSRF, nil)
	expectStatus(t, 202, code, body)
	code, body = bearerRequest(t, h, "DELETE", endpoint, a.Value, u.CSRF, nil)
	expectStatus(t, 202, code, body)
	deletionCount(t, s, 1, "SELECT COUNT(*) FROM material_deletions WHERE material_id=?", mid)
	deletionCount(t, s, 0, "SELECT COUNT(*) FROM assistant_materials WHERE material_id=?", mid)
	for _, p := range []string{endpoint, endpoint + "/file", fmt.Sprintf("%s/chunks/%d", endpoint, hits[0].ChunkID), fmt.Sprintf("/api/knowledge-entries/%d", kid)} {
		code, body = bearerRequest(t, h, "GET", p, student.Value, "", nil)
		expectStatus(t, 410, code, body)
		if !strings.Contains(string(body), "material_deleted") {
			t.Fatal(string(body))
		}
		code, body = bearerRequest(t, h, "GET", p, b.Value, "", nil)
		expectStatus(t, 404, code, body)
	}
	code, body = bearerRequest(t, h, "POST", endpoint+"/reindex", a.Value, u.CSRF, nil)
	expectStatus(t, 410, code, body)
	code, body = bearerRequest(t, h, "PATCH", appPath, a.Value, u.CSRF, map[string]any{"material_ids": []int64{mid}})
	expectStatus(t, 404, code, body)
	for _, mode := range []string{"keyword", "vector", "hybrid"} {
		got, e := s.retrieveScoped(ctx, u.ClassID, "判别式", mode, 4, []int64{mid})
		if e != nil || len(got) != 0 {
			t.Fatal("deleted material retrieved", mode, e, got)
		}
	}
	// A stale Qdrant candidate must never yield its former SQL body.
	got, e := s.retrieveScoped(ctx, u.ClassID, "判别式", "vector", 4, []int64{mid})
	if e != nil || len(got) != 0 {
		t.Fatal(e, got)
	}
	turn, err = s.acceptAITurn(ctx, su, conv.ID, turnInput{"下一题", "99999999-0000-4000-a000-000000000002"})
	if err != nil || len(turn.Materials) != 0 || turn.Materials == nil {
		t.Fatal("expected unbound", err, turn.Materials)
	}
	s.failAITurn(turn.ID, "test")
	g.mu.Lock()
	embeddings, chats := g.embeddings, g.chats
	g.mu.Unlock()
	if !s.processMaterialCleanup(ctx) {
		t.Fatal("cleanup not run")
	}
	code, body = bearerRequest(t, h, "DELETE", endpoint, a.Value, u.CSRF, nil)
	expectStatus(t, 200, code, body)
	for _, table := range []string{"knowledge_entries", "knowledge_index_jobs", "knowledge_chunks", "assistant_materials"} {
		deletionCount(t, s, 0, "SELECT COUNT(*) FROM "+table+" WHERE material_id=?", mid)
	}
	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("file remains", err)
	}
	for range 2 {
		if err = Migrate(ctx, s.DB); err != nil {
			t.Fatal(err)
		}
		if err = s.Seed(ctx); err != nil {
			t.Fatal(err)
		}
		if err = s.Reconcile(ctx); err != nil {
			t.Fatal(err)
		}
		if err = s.enqueueMissing(ctx); err != nil {
			t.Fatal(err)
		}
	}
	deletionCount(t, s, 0, "SELECT COUNT(*) FROM knowledge_index_jobs WHERE material_id=?", mid)
	code, body = bearerRequest(t, h, "GET", fmt.Sprintf("/api/ai/conversations/%d", conv.ID), student.Value, "", nil)
	expectStatus(t, 200, code, body)
	if !strings.Contains(string(body), "已保存的历史回答") || !strings.Contains(string(body), "material_deleted") || strings.Contains(string(body), "删除验证专属材料") {
		t.Fatal("unsafe history", string(body))
	}
	// Original knowledge URL still identifies a withdrawn resource after purge.
	code, body = bearerRequest(t, h, "GET", fmt.Sprintf("/api/knowledge-entries/%d", kid), student.Value, "", nil)
	expectStatus(t, 410, code, body)
	code, body = bearerRequest(t, h, "GET", "/api/materials/2/file", b.Value, "", nil)
	expectStatus(t, 200, code, body)
	g.mu.Lock()
	if g.embeddings != embeddings || g.chats != chats {
		t.Error("cleanup invoked model")
	}
	g.mu.Unlock()
	if real != "" {
		var count struct {
			Result struct {
				Count int `json:"count"`
			} `json:"result"`
		}
		if err = s.qcall(ctx, "POST", "/points/count", map[string]any{"filter": scopeFilter(u.ClassID, mid), "exact": true}, &count); err != nil || count.Result.Count != 0 {
			t.Fatal("residual real vectors", err, count)
		}
	} else {
		g.mu.Lock()
		for _, p := range g.points {
			if p["payload"].(map[string]any)["material_id"] == float64(mid) {
				t.Error("residual vector")
			}
		}
		g.mu.Unlock()
	}
	t.Log("Deletion: permissions, rollback, binding revocation, all search modes, history, files, SQL, vectors and restart passed; no real models")
}

func TestMaterialDeletionRecovery(t *testing.T) {
	for _, stage := range []string{"delete-after-vector", "delete-after-database", "delete-file", "delete-after-file", "qdrant-unavailable", "uncertain-write"} {
		t.Run(stage, func(t *testing.T) {
			s, h := testServer(t)
			ctx := context.Background()
			g := newRAGGateway(t)
			g.configure(s)
			a, u := loginAs(t, s, h, "teacher_a")
			mid, _, path := deletionUpload(t, s, h, a, u)
			code, body := bearerRequest(t, h, "DELETE", fmt.Sprintf("/api/materials/%d", mid), a.Value, u.CSRF, nil)
			expectStatus(t, 202, code, body)
			switch stage {
			case "qdrant-unavailable":
				s.Config.QdrantURL = "http://127.0.0.1:1"
			case "uncertain-write":
				_, _ = s.DB.Exec("UPDATE materials SET vector_uncertain=TRUE WHERE id=?", mid)
			default:
				s.fail = func(at string) error {
					if at == stage {
						return errors.New("injected")
					}
					return nil
				}
			}
			if !s.processMaterialCleanup(ctx) {
				t.Fatal("no cleanup attempt")
			}
			deletionCount(t, s, 1, "SELECT COUNT(*) FROM material_deletions WHERE material_id=? AND status='cleanup_pending' AND last_error_code<>''", mid)
			// Reinstantiate the service with the same isolated DB, simulating restart.
			recovered, err := New(s.DB, s.Config)
			if err != nil {
				t.Fatal(err)
			}
			recovered.Config.QdrantURL = g.server.URL
			if err = recovered.Reconcile(ctx); err != nil {
				t.Fatal(err)
			}
			_, _ = s.DB.Exec("UPDATE material_deletions SET next_attempt=CURRENT_TIMESTAMP(6) WHERE material_id=?", mid)
			recovered.processMaterialCleanup(ctx)
			if stage == "uncertain-write" {
				deletionCount(t, s, 1, "SELECT COUNT(*) FROM material_deletions WHERE material_id=? AND status='cleanup_pending' AND last_error_code='vector_write_unconfirmed'", mid)
				return
			}
			deletionCount(t, s, 1, "SELECT COUNT(*) FROM material_deletions WHERE material_id=? AND status='deleted'", mid)
			if _, err = os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("file remains", err)
			}
			if err = recovered.Reconcile(ctx); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMaterialDeletionDuringIndex(t *testing.T) {
	for _, phase := range []string{"embedding", "upsert"} {
		t.Run(phase, func(t *testing.T) {
			s, h := testServer(t)
			g := newRAGGateway(t)
			g.configure(s)
			entered, release := make(chan struct{}), make(chan struct{})
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				match := (phase == "embedding" && r.URL.Path == "/embeddings") || (phase == "upsert" && r.Method == "PUT" && strings.HasSuffix(r.URL.Path, "/points"))
				if match {
					close(entered)
					if phase == "embedding" {
						select {
						case <-r.Context().Done():
							return
						case <-release:
						}
					} else {
						<-release
					}
				}
				g.server.Config.Handler.ServeHTTP(w, r)
			}))
			defer upstream.Close()
			s.Config.EmbeddingBaseURL = upstream.URL
			s.Config.QdrantURL = upstream.URL
			a, u := loginAs(t, s, h, "teacher_a")
			mid, kid, _ := deletionUpload(t, s, h, a, u)
			// Target only this material, avoiding other seeded pending jobs.
			var job indexJob
			if err := s.DB.QueryRow("SELECT id,generation FROM knowledge_index_jobs WHERE material_id=?", mid).Scan(&job.ID, &job.Generation); err != nil {
				t.Fatal(err)
			}
			job.MaterialID = mid
			job.KnowledgeID = kid
			job.ClassID = u.ClassID
			job.Body = "判别式并发测试"
			_, _ = s.DB.Exec("UPDATE knowledge_index_jobs SET status='running' WHERE id=?", job.ID)
			done := make(chan error, 1)
			go func() { done <- s.buildIndex(context.Background(), job) }()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("index did not enter blocked phase")
			}
			code, body := bearerRequest(t, h, "DELETE", fmt.Sprintf("/api/materials/%d", mid), a.Value, u.CSRF, nil)
			expectStatus(t, 202, code, body)
			if phase == "upsert" && s.processMaterialCleanup(context.Background()) {
				t.Fatal("cleanup overtook live upsert")
			}
			close(release)
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("deleted index published ready")
				}
			case <-time.After(5 * time.Second):
				t.Fatal("index failed to stop")
			}
			if !s.processMaterialCleanup(context.Background()) {
				t.Fatal("cleanup not run")
			}
			deletionCount(t, s, 1, "SELECT COUNT(*) FROM material_deletions WHERE material_id=? AND status='deleted'", mid)
			g.mu.Lock()
			for _, p := range g.points {
				if p["payload"].(map[string]any)["material_id"] == float64(mid) {
					t.Error("late vector survived")
				}
			}
			g.mu.Unlock()
		})
	}
}

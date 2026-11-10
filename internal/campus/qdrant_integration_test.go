//go:build qdrant

package campus

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

// Opt-in only: run against a new disposable Qdrant container, never a real index.
func TestRealQdrantRoundTrip(t *testing.T) {
	endpoint := os.Getenv("QDRANT_TEST_URL")
	if !strings.HasPrefix(endpoint, "http://127.0.0.1:") || os.Getenv("QDRANT_TEST_CONFIRM_ISOLATED") != "true" {
		t.Fatal("qdrant tag requires explicit disposable loopback Qdrant configuration")
	}
	s, server := testServer(t)
	g := newRAGGateway(t)
	g.configure(s)
	s.Config.QdrantURL = endpoint
	a, u := loginAs(t, s, server, "teacher_a")
	student, _ := loginAs(t, s, server, "student_a1")
	b, _ := loginAs(t, s, server, "teacher_b")
	code, body := uploadRequest(t, server, a, u.CSRF, "真实向量协议验证.md", []byte("期末项目成绩占40%。"), nil)
	expectStatus(t, 201, code, body)
	var material map[string]int64
	_ = json.Unmarshal(body, &material)
	mid := material["material_id"]
	if e := s.enqueueMissing(context.Background()); e != nil {
		t.Fatal(e)
	}
	for s.processNextIndex(context.Background()) {
	}
	code, body = bearerRequest(t, server, "GET", fmt.Sprintf("/api/materials/%d", mid), a.Value, "", nil)
	expectStatus(t, 200, code, body)
	if !strings.Contains(string(body), `"index_status":"ready"`) {
		t.Fatal("real Qdrant indexing failed", string(body))
	}
	for _, mode := range []string{"keyword", "vector", "hybrid"} {
		code, body = bearerRequest(t, server, "POST", "/api/search", student.Value, "", map[string]any{"q": "期末项目", "mode": mode})
		expectStatus(t, 200, code, body)
		var data struct{ Hits []Hit }
		_ = json.Unmarshal(body, &data)
		if len(data.Hits) == 0 {
			t.Fatal("no real hit")
		}
		for _, h := range data.Hits {
			if h.MaterialID == 2 {
				t.Fatal("cross-class point leaked")
			}
		}
	}
	code, body = bearerRequest(t, server, "POST", "/api/ask", student.Value, "", map[string]any{"question": "期末项目"})
	expectStatus(t, 200, code, body)
	// The independent B-class query must only get its own seeded source.
	code, body = bearerRequest(t, server, "POST", "/api/search", b.Value, "", map[string]any{"q": "课程", "mode": "vector"})
	expectStatus(t, 200, code, body)
	var data struct{ Hits []Hit }
	_ = json.Unmarshal(body, &data)
	for _, h := range data.Hits {
		if h.MaterialID != 2 {
			t.Fatal("Qdrant filter failed")
		}
	}
	path := fmt.Sprintf("/api/materials/%d/reindex", mid)
	code, body = bearerRequest(t, server, "POST", path, a.Value, u.CSRF, nil)
	expectStatus(t, 202, code, body)
	s.processNextIndex(context.Background())
	code, body = bearerRequest(t, server, "POST", "/api/search", student.Value, "", map[string]any{"q": "项目", "mode": "hybrid"})
	expectStatus(t, 200, code, body)
	t.Log("Real Qdrant collection creation, Cosine configuration, confirmed upsert/delete, filtered search, SQL source lookup and reindex passed; embedding/chat remain HTTP doubles.")
}

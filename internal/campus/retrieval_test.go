package campus

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestChunkCoordinates(t *testing.T) {
	for _, body := range []string{"短😀文\r\n", strings.Repeat("中😀", 1400), strings.Repeat("章\r\n段落。\n\n", 400), "。" + strings.Repeat("汉", 1800), strings.Repeat("字", 800)} {
		chunks := splitText(body)
		r := []rune(body)
		if len(chunks) == 0 || chunks[0].Start != 0 || chunks[len(chunks)-1].End != len(r) {
			t.Fatal("coverage")
		}
		for i, c := range chunks {
			if c.Text != string(r[c.Start:c.End]) || c.End-c.Start > 800 || c.End <= c.Start {
				t.Fatal("coordinates")
			}
			if i > 0 && (c.Start != chunks[i-1].End-80 || c.Start <= chunks[i-1].Start) {
				t.Fatal("overlap/progress")
			}
		}
	}
}
func TestRRFFilteredRanks(t *testing.T) {
	k := []Hit{{ChunkID: 3}, {ChunkID: 2}}
	v := []Hit{{ChunkID: 1}, {ChunkID: 2}, {ChunkID: 3}}
	h := mergeHits(k, v, "hybrid", 20)
	if len(h) != 3 || h[0].ChunkID != 3 || math.Abs(h[0].Score-(1.0/61+1.0/63)) > 1e-12 || h[2].ChunkID != 1 || h[2].KeywordRank != nil {
		t.Fatalf("RRF: %+v", h)
	}
	h = mergeHits([]Hit{{ChunkID: 2}, {ChunkID: 1}}, []Hit{{ChunkID: 1}, {ChunkID: 2}}, "hybrid", 1)
	if h[0].ChunkID != 1 {
		t.Fatal("stable tie")
	}
}
func TestBearerCredentialParsing(t *testing.T) {
	token := strings.Repeat("a", 64)
	for _, tc := range []struct {
		header string
		valid  bool
	}{{"Bearer " + token, true}, {"bearer " + token, true}, {"Basic " + token, false}, {"Bearer " + strings.Repeat("z", 64), false}, {"Bearer", false}, {"Bearer " + token + " extra", false}, {"", false}} {
		r := httptest.NewRequest("GET", "/api/me", nil)
		r.AddCookie(&http.Cookie{Name: cookieName, Value: token})
		r.Header.Set("Authorization", tc.header)
		_, ok := credential(r)
		if ok != tc.valid {
			t.Fatal(tc.header, ok)
		}
	}
	r := httptest.NewRequest("GET", "/api/me", nil)
	r.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	if _, ok := credential(r); !ok {
		t.Fatal("cookie compatibility")
	}
	r.Header.Add("Authorization", "Bearer "+token)
	r.Header.Add("Authorization", "Bearer "+token)
	if _, ok := credential(r); ok {
		t.Fatal("ambiguous headers")
	}
}
func TestRetrievalConfiguration(t *testing.T) {
	t.Setenv("SESSION_KEY", strings.Repeat("k", 32))
	t.Setenv("EMBEDDING_DIMENSION", "")
	c, e := ConfigFromEnv()
	if e != nil {
		t.Fatal(e)
	}
	if c.embeddingReady() {
		t.Fatal("missing gateway should disable vector only")
	}
	for _, dim := range []string{"abc", "0", "-1", "65537"} {
		t.Setenv("EMBEDDING_DIMENSION", dim)
		if _, e := ConfigFromEnv(); e == nil {
			t.Fatal("bad dimension accepted")
		}
	}
	t.Setenv("EMBEDDING_DIMENSION", "3")
	t.Setenv("CHAT_BASE_URL", "https://secret@example.com/v1")
	if _, e := ConfigFromEnv(); e == nil {
		t.Fatal("userinfo URL accepted")
	}
	c.EmbeddingAPIKey = "secret"
	sig := c.signature()
	c.EmbeddingAPIKey = "rotated"
	if sig != c.signature() {
		t.Fatal("key should not affect model signature")
	}
	c.EmbeddingModel = "other"
	if sig == c.signature() {
		t.Fatal("model must affect signature")
	}
}
func TestExternalClientValidation(t *testing.T) {
	for _, tc := range []struct {
		name, body   string
		status       int
		valid, retry bool
	}{
		{"success", `{"data":[{"index":0,"embedding":[0.1,0.2]}]}`, 200, true, false},
		{"dimension", `{"data":[{"index":0,"embedding":[0.1]}]}`, 200, false, false},
		{"zero", `{"data":[{"index":0,"embedding":[0,0]}]}`, 200, false, false},
		{"badindex", `{"data":[{"index":1,"embedding":[1,2]}]}`, 200, false, false},
		{"html", "<html>secret upstream</html>", 200, false, false},
		{"auth", "secret", 401, false, false}, {"limit", "secret", 429, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer private-test-key" {
					t.Error("missing auth")
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer gateway.Close()
			s := &Server{Config: Config{EmbeddingBaseURL: gateway.URL, EmbeddingModel: "test", EmbeddingAPIKey: "private-test-key", EmbeddingDimension: 2, QdrantURL: gateway.URL}}
			_, e := s.embed(context.Background(), []string{"test"})
			if (e == nil) != tc.valid {
				t.Fatal(e)
			}
			if e != nil {
				var de *dependencyError
				if !errors.As(e, &de) || de.Retry != tc.retry || strings.Contains(e.Error(), "secret") {
					t.Fatal("unsafe error")
				}
			}
		})
	}
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer gateway.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if e := callJSON(ctx, "POST", gateway.URL, "", "", nil, new(any)); e == nil {
		t.Fatal("timeout ignored")
	}
}
func TestAnswerReferences(t *testing.T) {
	for _, tc := range []struct {
		text string
		ok   bool
	}{{"答案 [1]", true}, {"答案 [2]", true}, {"答案", false}, {"[0]", false}, {"[3]", false}, {"[1] 和 [99]", false}, {"", false}} {
		if validAnswer(tc.text, 2) != tc.ok {
			t.Fatal(tc.text)
		}
	}
}

func TestIndexRetryBound(t *testing.T) {
	calls := 0
	e := retryDependency(context.Background(), func(context.Context) error { calls++; return &dependencyError{Code: "limited", Retry: true} })
	if e == nil || calls != 3 {
		t.Fatal("retry budget", calls, e)
	}
	calls = 0
	_ = retryDependency(context.Background(), func(context.Context) error { calls++; return dep("unauthorized") })
	if calls != 1 {
		t.Fatal("permanent errors retried")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls = 0
	_ = retryDependency(ctx, func(context.Context) error { calls++; return &dependencyError{Code: "timeout", Retry: true} })
	if calls != 1 {
		t.Fatal("cancel ignored")
	}
}
func TestCollectionAndWriteValidation(t *testing.T) {
	for _, tc := range []struct {
		size     int
		distance string
		ok       bool
	}{{2, "Cosine", true}, {3, "Cosine", false}, {2, "Dot", false}} {
		gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, 200, map[string]any{"result": map[string]any{"config": map[string]any{"params": map[string]any{"vectors": map[string]any{"size": tc.size, "distance": tc.distance}}}}})
		}))
		s := &Server{Config: Config{QdrantURL: gateway.URL, EmbeddingDimension: 2}}
		if e := s.ensureCollection(context.Background()); (e == nil) != tc.ok {
			t.Fatal(e)
		}
		gateway.Close()
	}
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]string{"status": "acknowledged"}})
	}))
	defer gateway.Close()
	s := &Server{Config: Config{QdrantURL: gateway.URL}}
	if e := s.qcall(context.Background(), "PUT", "/points?wait=true", map[string]any{}, nil); e == nil {
		t.Fatal("unconfirmed write accepted")
	}
}

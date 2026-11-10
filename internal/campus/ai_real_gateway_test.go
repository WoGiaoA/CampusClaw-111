//go:build realgateway

package campus

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type gatewayCall struct {
	Model            string   `json:"model"`
	Stream           bool     `json:"stream"`
	MaxTokens        int      `json:"max_tokens"`
	Roles            []string `json:"roles"`
	Template         bool     `json:"template"`
	Skill            bool     `json:"skill"`
	BoundEvidence    bool     `json:"bound_evidence"`
	UpstreamHTTP     int      `json:"upstream_http"`
	UpstreamMedia    string   `json:"upstream_media"`
	UpstreamDeltasMS []int64  `json:"upstream_deltas_ms"`
	ClientDeltasMS   []int64  `json:"client_deltas_ms"`
	ClientEvents     []string `json:"client_events"`
	UpstreamFinish   string   `json:"upstream_finish,omitempty"`
	UpstreamDone     bool     `json:"upstream_done"`
	Saved            bool     `json:"saved"`
	PublicSeedAnswer string   `json:"public_seed_answer,omitempty"`
}

type gatewayEvidence struct {
	mu      sync.Mutex
	Chat    int            `json:"chat_attempts"`
	Queries int            `json:"query_embedding_attempts"`
	Ingest  int            `json:"ingest_embedding_attempts"`
	Rounds  []*gatewayCall `json:"rounds"`
	Result  string         `json:"result"`
}

type gatewayTransport struct {
	base     http.RoundTripper
	report   *gatewayEvidence
	template string
}

func (tr gatewayTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	var call *gatewayCall
	tr.report.mu.Lock()
	switch {
	case strings.HasSuffix(r.URL.Path, "/embeddings"):
		if tr.report.Queries >= 2 {
			tr.report.mu.Unlock()
			return nil, errors.New("embedding verification limit")
		}
		tr.report.Queries++
	case strings.HasSuffix(r.URL.Path, "/chat/completions"):
		if tr.report.Chat >= 2 {
			tr.report.mu.Unlock()
			return nil, errors.New("chat verification limit")
		}
		tr.report.Chat++
		var input struct {
			Model     string
			Stream    bool
			Messages  []modelMessage
			MaxTokens int `json:"max_tokens"`
		}
		body, err := io.ReadAll(r.Body)
		r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(body))
		if err != nil || json.Unmarshal(body, &input) != nil {
			tr.report.mu.Unlock()
			return nil, errors.New("invalid request for verification")
		}
		call = &gatewayCall{Model: input.Model, Stream: input.Stream, MaxTokens: input.MaxTokens}
		for _, m := range input.Messages {
			call.Roles = append(call.Roles, m.Role)
		}
		if len(input.Messages) >= 2 {
			call.Template = strings.Contains(input.Messages[0].Content, tr.template)
			call.Skill = strings.Contains(input.Messages[0].Content, guideProcedure)
			final := input.Messages[len(input.Messages)-1].Content
			call.BoundEvidence = strings.Contains(final, "CLASS_A_ONLY") && !strings.Contains(final, "CLASS_B_ONLY")
		}
		tr.report.Rounds = append(tr.report.Rounds, call)
	}
	tr.report.mu.Unlock()
	started := time.Now()
	response, err := tr.base.RoundTrip(r)
	if call != nil && response != nil {
		tr.report.mu.Lock()
		call.UpstreamHTTP = response.StatusCode
		call.UpstreamMedia = response.Header.Get("Content-Type")
		tr.report.mu.Unlock()
		response.Body = &observedGatewayBody{ReadCloser: response.Body, call: call, report: tr.report, started: started}
	}
	return response, err
}

type observedGatewayBody struct {
	io.ReadCloser
	call    *gatewayCall
	report  *gatewayEvidence
	started time.Time
	pending string
}

func (body *observedGatewayBody) Read(p []byte) (int, error) {
	n, err := body.ReadCloser.Read(p)
	body.pending += string(p[:n])
	body.report.mu.Lock()
	defer body.report.mu.Unlock()
	for {
		line, rest, ok := strings.Cut(body.pending, "\n")
		if !ok {
			break
		}
		body.pending = rest
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(line[5:])
		if data == "[DONE]" {
			body.call.UpstreamDone = true
			continue
		}
		var frame struct {
			Choices []struct {
				Delta  struct{ Content string }
				Finish string `json:"finish_reason"`
			}
		}
		if json.Unmarshal([]byte(data), &frame) == nil && len(frame.Choices) > 0 {
			if frame.Choices[0].Delta.Content != "" {
				body.call.UpstreamDeltasMS = append(body.call.UpstreamDeltasMS, time.Since(body.started).Milliseconds())
			}
			if frame.Choices[0].Finish != "" {
				body.call.UpstreamFinish = frame.Choices[0].Finish
			}
		}
	}
	return n, err
}

func loadGatewaySettings(t *testing.T, root string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".env"))
	if err != nil {
		t.Fatal("local gateway configuration unavailable")
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if ok && (strings.HasPrefix(key, "CHAT_") || strings.HasPrefix(key, "EMBEDDING_")) {
			values[key] = strings.Trim(strings.TrimSpace(value), "\"'")
		}
	}
	for _, key := range []string{"CHAT_BASE_URL", "EMBEDDING_BASE_URL"} {
		u, e := url.Parse(values[key])
		if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" {
			t.Fatal("real gateway requires a configured HTTPS base URL")
		}
	}
	return values
}

func TestExplicitRealGatewayConversation(t *testing.T) {
	if os.Getenv("CAMPUSCLAW_REAL_GATEWAY") != "1" || os.Getenv("QDRANT_TEST_CONFIRM_ISOLATED") != "true" {
		t.Fatal("real validation requires explicit opt-in and disposable Qdrant")
	}
	qdrant := os.Getenv("QDRANT_TEST_URL")
	if !strings.HasPrefix(qdrant, "http://127.0.0.1:") {
		t.Fatal("invalid isolated Qdrant")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	report := &gatewayEvidence{Result: "incomplete"}
	defer func() {
		report.mu.Lock()
		defer report.mu.Unlock()
		if !t.Failed() && len(report.Rounds) == 2 && report.Rounds[1].Saved {
			report.Result = "passed"
		}
		encoded, _ := json.MarshalIndent(report, "", "  ")
		if err := os.WriteFile(filepath.Join(root, ".local", "lesson05-real-gateway.json"), encoded, 0600); err != nil {
			t.Error("could not save redacted evidence")
		}
		t.Logf("real calls: Chat %d, query Embedding %d, ingest Embedding 0", report.Chat, report.Queries)
	}()
	settings := loadGatewaySettings(t, root)
	s, h := testServer(t)
	s.Config.EmbeddingBaseURL = strings.TrimRight(settings["EMBEDDING_BASE_URL"], "/")
	s.Config.EmbeddingModel = settings["EMBEDDING_MODEL"]
	s.Config.EmbeddingAPIKey = settings["EMBEDDING_API_KEY"]
	s.Config.EmbeddingDimension, _ = strconv.Atoi(settings["EMBEDDING_DIMENSION"])
	s.Config.ChatBaseURL = strings.TrimRight(settings["CHAT_BASE_URL"], "/")
	s.Config.ChatModel = settings["CHAT_MODEL"]
	s.Config.ChatAPIKey = settings["CHAT_API_KEY"]
	s.Config.QdrantURL = qdrant
	if !s.Config.embeddingReady() || s.Config.ChatModel == "" || s.Config.ChatAPIKey == "" || s.Config.EmbeddingAPIKey == "" {
		t.Fatal("real gateway settings incomplete")
	}
	data, err := os.ReadFile(filepath.Join(root, ".local", "lesson05-existing-seed.json"))
	var seed struct {
		Text       string
		Signature  string
		Start, End int
		Vector     []float64
	}
	if err != nil || json.Unmarshal(data, &seed) != nil || len(seed.Vector) != s.Config.EmbeddingDimension ||
		seed.Signature != s.Config.signature() || seed.Start != 0 || seed.End != len([]rune(seed.Text)) || !strings.Contains(seed.Text, "CLASS_A_ONLY") {
		t.Fatal("existing seed index does not match the configured model")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	var material, entry, class, app, template int64
	var original string
	if err = s.DB.QueryRow(`SELECT m.id,k.id,m.class_id,a.id,a.template_id,k.body_text FROM materials m JOIN knowledge_entries k ON k.material_id=m.id JOIN assistants a ON a.class_id=m.class_id WHERE m.seed_key='A-sample' AND a.builtin_key='problem-guide'`).Scan(&material, &entry, &class, &app, &template, &original); err != nil || original != seed.Text {
		t.Fatal("public seed does not match isolated fixture")
	}
	var templateBody string
	if err = s.DB.QueryRow("SELECT body FROM prompt_templates WHERE id=?", template).Scan(&templateBody); err != nil {
		t.Fatal(err)
	}
	exec := func(query string, args ...any) int64 {
		t.Helper()
		res, e := s.DB.ExecContext(ctx, query, args...)
		if e != nil {
			t.Fatal(e)
		}
		id, _ := res.LastInsertId()
		return id
	}
	job := exec("INSERT INTO knowledge_index_jobs(material_id,knowledge_entry_id,class_id,generation,status,model_signature) VALUES(?,?,?,1,'ready',?)", material, entry, class, seed.Signature)
	chunk := exec("INSERT INTO knowledge_chunks(index_job_id,generation,material_id,knowledge_entry_id,class_id,chunk_index,chunk_text,char_start,char_end,index_status) VALUES(?,1,?,?,?,1,?,?,?,'ready')", job, material, entry, class, seed.Text, seed.Start, seed.End)
	exec("INSERT INTO assistant_materials(assistant_id,material_id,class_id) VALUES(?,?,?)", app, material, class)
	if err = s.ensureCollection(ctx); err != nil {
		t.Fatalf("disposable collection: %v", err)
	}
	payload := map[string]any{"class_id": class, "material_id": material, "knowledge_entry_id": entry, "index_job_id": job, "generation": 1}
	point := map[string]any{"id": chunk, "vector": seed.Vector, "payload": payload}
	if err = s.qcall(ctx, "PUT", "/points?wait=true", map[string]any{"points": []any{point}}, nil); err != nil {
		t.Fatalf("restore existing vector: %v", err)
	}
	previousOutbound, previousStream := outbound.Transport, streamingClient.Transport
	outbound.Transport = gatewayTransport{http.DefaultTransport, report, templateBody}
	streamingClient.Transport = gatewayTransport{previousStream, report, templateBody}
	defer func() { outbound.Transport = previousOutbound; streamingClient.Transport = previousStream }()
	student, u := loginAs(t, s, h, "student_a1")
	status, response := bearerRequest(t, h, "POST", "/api/ai/conversations", student.Value, u.CSRF, map[string]any{"app_id": app})
	expectStatus(t, 201, status, response)
	var c aiConversation
	if err = json.Unmarshal(response, &c); err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/api/ai/conversations/%d", c.ID)
	questions := []string{"课程导论资料题怎么做？请引用讲义并提示第一步。", "延续刚才的课程导论，接下来怎么做？请引用讲义。"}
	for i, question := range questions {
		input, _ := json.Marshal(turnInput{question, fmt.Sprintf("50000000-0000-4000-a000-%012d", i+1)})
		req, _ := http.NewRequestWithContext(ctx, "POST", h.URL+path+"/messages", bytes.NewReader(input))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+student.Value)
		req.Header.Set("X-CSRF-Token", u.CSRF)
		started := time.Now()
		res, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal("local conversation request failed")
		}
		if res.StatusCode != 200 {
			res.Body.Close()
			t.Fatalf("conversation preflight failed: %d", res.StatusCode)
		}
		var events []string
		var arrival []int64
		var deltas strings.Builder
		var event, streamError string
		reader := bufio.NewScanner(res.Body)
		for reader.Scan() {
			line := reader.Text()
			if strings.HasPrefix(line, "event: ") {
				event = line[7:]
				events = append(events, event)
			}
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			var frame struct {
				Text string
				Code string
			}
			_ = json.Unmarshal([]byte(line[6:]), &frame)
			if event == "delta" {
				arrival = append(arrival, time.Since(started).Milliseconds())
				deltas.WriteString(frame.Text)
			}
			if event == "error" {
				streamError = frame.Code
			}
		}
		res.Body.Close()
		report.mu.Lock()
		var call *gatewayCall
		if len(report.Rounds) > i {
			call = report.Rounds[i]
			call.ClientDeltasMS, call.ClientEvents = arrival, events
		}
		report.mu.Unlock()
		if len(events) < 3 || events[0] != "meta" || events[len(events)-1] != "done" || len(arrival) < 2 {
			t.Fatalf("real stream did not complete: events=%v code=%s", events, streamError)
		}
		if call == nil || !call.Template || !call.Skill || !call.BoundEvidence || len(call.Roles) != 2+i*2 || len(call.UpstreamDeltasMS) < 2 || !call.UpstreamDone {
			t.Fatal("real outgoing context or upstream stream incomplete")
		}
		var saved string
		if err = s.DB.QueryRow("SELECT m.body FROM ai_messages m JOIN ai_turns t ON t.id=m.turn_id WHERE t.conversation_id=? AND t.status='completed' AND m.role='assistant' ORDER BY m.id DESC LIMIT 1", c.ID).Scan(&saved); err != nil || saved != deltas.String() {
			t.Fatal("saved message differs from stream")
		}
		if !strings.Contains(saved, "[1]") {
			t.Fatal("missing confirmed reference")
		}
		t.Logf("round %d saved with %d incremental events", i+1, len(arrival))
		call.Saved = true
		call.PublicSeedAnswer = saved
	}
}

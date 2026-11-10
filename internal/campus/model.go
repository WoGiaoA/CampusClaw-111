package campus

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

// Only safe error categories leave the service boundary; upstream bodies/URLs
// may contain credentials or submitted material and must never be logged.
type dependencyError struct {
	Code   string
	Retry  bool
	Status int
}

func (e *dependencyError) Error() string { return e.Code }
func dep(code string) error              { return &dependencyError{Code: code} }

var outbound = &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func callJSON(ctx context.Context, method, endpoint, header, key string, input, output any) error {
	var body io.Reader
	if input != nil {
		b, e := json.Marshal(input)
		if e != nil {
			return dep("invalid_request")
		}
		body = bytes.NewReader(b)
	}
	req, e := http.NewRequestWithContext(ctx, method, endpoint, body)
	if e != nil {
		return dep("invalid_configuration")
	}
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		if header == "Authorization" {
			key = "Bearer " + key
		}
		req.Header.Set(header, key)
	}
	res, e := outbound.Do(req)
	if e != nil {
		return &dependencyError{Code: "dependency_unavailable", Retry: true}
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return &dependencyError{Code: "dependency_http_error", Status: res.StatusCode, Retry: res.StatusCode == 429 || res.StatusCode >= 500}
	}
	if output == nil {
		return nil
	}
	b, e := io.ReadAll(io.LimitReader(res.Body, (8<<20)+1))
	if e != nil || len(b) > 8<<20 {
		return dep("invalid_response")
	}
	if json.Unmarshal(b, output) != nil {
		return dep("invalid_response")
	}
	return nil
}
func (c Config) embeddingReady() bool {
	return c.EmbeddingBaseURL != "" && c.EmbeddingModel != "" && c.EmbeddingDimension > 0 && c.QdrantURL != ""
}
func (c Config) signature() string {
	b := sha256.Sum256([]byte(fmt.Sprintf("%s\n%s\n%d", strings.TrimRight(c.EmbeddingBaseURL, "/"), c.EmbeddingModel, c.EmbeddingDimension)))
	return hex.EncodeToString(b[:])
}
func (s *Server) embed(ctx context.Context, texts []string) ([][]float64, error) {
	if !s.Config.embeddingReady() {
		return nil, dep("embedding_not_configured")
	}
	var res struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	err := callJSON(ctx, "POST", s.Config.EmbeddingBaseURL+"/embeddings", "Authorization", s.Config.EmbeddingAPIKey, map[string]any{"model": s.Config.EmbeddingModel, "input": texts, "encoding_format": "float"}, &res)
	if err != nil {
		return nil, err
	}
	if len(res.Data) != len(texts) {
		return nil, dep("embedding_count_mismatch")
	}
	out := make([][]float64, len(texts))
	for _, v := range res.Data {
		if v.Index < 0 || v.Index >= len(out) || out[v.Index] != nil || len(v.Embedding) != s.Config.EmbeddingDimension {
			return nil, dep("embedding_dimension_mismatch")
		}
		norm := 0.0
		for _, n := range v.Embedding {
			if math.IsNaN(n) || math.IsInf(n, 0) {
				return nil, dep("invalid_embedding")
			}
			norm += n * n
		}
		if norm == 0 {
			return nil, dep("invalid_embedding")
		}
		out[v.Index] = v.Embedding
	}
	return out, nil
}

const collectionPath = "/collections/campusclaw_chunks"

func (s *Server) qcall(ctx context.Context, method, path string, in, out any) error {
	if out == nil && strings.HasPrefix(path, "/points") {
		var ack struct {
			Result struct {
				Status string `json:"status"`
			} `json:"result"`
		}
		if e := callJSON(ctx, method, s.Config.QdrantURL+collectionPath+path, "api-key", s.Config.QdrantAPIKey, in, &ack); e != nil {
			return e
		}
		if ack.Result.Status != "completed" {
			return dep("vector_write_unconfirmed")
		}
		return nil
	}
	return callJSON(ctx, method, s.Config.QdrantURL+collectionPath+path, "api-key", s.Config.QdrantAPIKey, in, out)
}
func (s *Server) ensureCollection(ctx context.Context) error {
	var res struct {
		Result struct {
			Config struct {
				Params struct {
					Vectors struct {
						Size     int    `json:"size"`
						Distance string `json:"distance"`
					} `json:"vectors"`
				} `json:"params"`
			} `json:"config"`
		} `json:"result"`
	}
	err := s.qcall(ctx, "GET", "", nil, &res)
	var de *dependencyError
	if errors.As(err, &de) && de.Status == 404 {
		if e := s.qcall(ctx, "PUT", "", map[string]any{"vectors": map[string]any{"size": s.Config.EmbeddingDimension, "distance": "Cosine"}}, nil); e != nil {
			return e
		}
		err = s.qcall(ctx, "GET", "", nil, &res)
	}
	if err != nil {
		return err
	}
	if res.Result.Config.Params.Vectors.Size != s.Config.EmbeddingDimension || res.Result.Config.Params.Vectors.Distance != "Cosine" {
		return dep("collection_mismatch")
	}
	return nil
}
func scopeFilter(classID int64, materialID int64) map[string]any {
	must := []any{map[string]any{"key": "class_id", "match": map[string]any{"value": classID}}}
	if materialID > 0 {
		must = append(must, map[string]any{"key": "material_id", "match": map[string]any{"value": materialID}})
	}
	return map[string]any{"must": must}
}
func retryDependency(ctx context.Context, fn func(context.Context) error) error {
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		child, cancel := context.WithTimeout(ctx, 30*time.Second)
		err = fn(child)
		cancel()
		if err == nil {
			return nil
		}
		var de *dependencyError
		if !errors.As(err, &de) || !de.Retry {
			return err
		}
		if attempt < 2 {
			select {
			case <-ctx.Done():
				return dep("index_timeout")
			case <-time.After(time.Duration(1<<attempt) * time.Second):
			}
		}
	}
	return err
}

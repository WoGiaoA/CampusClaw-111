package campus

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type Hit struct {
	SourceStatus string   `json:"source_status,omitempty"`
	MaterialID   int64    `json:"material_id"`
	KnowledgeID  int64    `json:"knowledge_entry_id"`
	ChunkID      int64    `json:"chunk_id"`
	Title        string   `json:"material_title"`
	Index        int      `json:"chunk_index"`
	Start        int      `json:"char_start"`
	End          int      `json:"char_end"`
	Text         string   `json:"chunk_text,omitempty"`
	Excerpt      string   `json:"excerpt"`
	URL          string   `json:"source_url"`
	Score        float64  `json:"score"`
	KeywordScore *float64 `json:"keyword_score"`
	VectorScore  *float64 `json:"vector_score"`
	KeywordRank  *int     `json:"keyword_rank"`
	VectorRank   *int     `json:"vector_rank"`
}

const hitColumns = `c.material_id,c.knowledge_entry_id,c.id,m.title,c.chunk_index,c.char_start,c.char_end,c.chunk_text`
const hitJoin = ` FROM knowledge_chunks c JOIN knowledge_index_jobs j ON j.id=c.index_job_id AND j.generation=c.generation AND j.material_id=c.material_id AND j.knowledge_entry_id=c.knowledge_entry_id AND j.class_id=c.class_id JOIN materials m ON m.id=c.material_id AND m.class_id=c.class_id WHERE c.class_id=? AND m.deleted_at IS NULL AND c.index_status='ready' AND j.status='ready' `

type scanner interface{ Scan(...any) error }

func scanHit(row scanner, extra ...any) (Hit, error) {
	var h Hit
	args := []any{&h.MaterialID, &h.KnowledgeID, &h.ChunkID, &h.Title, &h.Index, &h.Start, &h.End, &h.Text}
	args = append(args, extra...)
	e := row.Scan(args...)
	if e != nil {
		return h, e
	}
	r := []rune(h.Text)
	if len(r) > 200 {
		r = r[:200]
	}
	h.Excerpt = string(r)
	h.URL = fmt.Sprintf("/materials?material_id=%d&chunk_id=%d", h.MaterialID, h.ChunkID)
	return h, nil
}
func materialScope(column string, ids []int64) (string, []any) {
	if ids == nil {
		return "", nil
	}
	args := make([]any, 0, len(ids))
	marks := make([]string, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
		marks = append(marks, "?")
	}
	return " AND " + column + " IN (" + strings.Join(marks, ",") + ")", args
}

func (s *Server) keyword(ctx context.Context, classID int64, q string, materials []int64) ([]Hit, error) {
	scope, ids := materialScope("c.material_id", materials)
	args := append(append([]any{q, classID}, ids...), q)
	rows, e := s.DB.QueryContext(ctx, "SELECT "+hitColumns+",MATCH(c.chunk_text) AGAINST(? IN NATURAL LANGUAGE MODE)"+hitJoin+scope+" AND MATCH(c.chunk_text) AGAINST(? IN NATURAL LANGUAGE MODE)>0 ORDER BY 9 DESC,c.id LIMIT 40", args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Hit{}
	for rows.Next() {
		var score float64
		h, e := scanHit(rows, &score)
		if e != nil {
			return nil, e
		}
		h.Score = score
		h.KeywordScore = &score
		out = append(out, h)
	}
	return out, rows.Err()
}
func (s *Server) vector(ctx context.Context, classID int64, q string, materials []int64) ([]Hit, error) {
	if !s.Config.embeddingReady() {
		return nil, dep("embedding_not_configured")
	}
	scope, scopedArgs := materialScope("material_id", materials)
	var mismatch int
	if e := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM knowledge_index_jobs WHERE class_id=? AND status='ready' AND model_signature<>? AND material_id IN (SELECT id FROM materials WHERE deleted_at IS NULL)"+scope, append([]any{classID, s.Config.signature()}, scopedArgs...)...).Scan(&mismatch); e != nil {
		return nil, e
	}
	if mismatch > 0 {
		return nil, dep("model_changed_reindex_required")
	}
	child, cancel := context.WithTimeout(ctx, 8*time.Second)
	vectors, e := s.embed(child, []string{q})
	cancel()
	if e != nil {
		return nil, e
	}
	var res struct {
		Result []struct {
			ID    int64   `json:"id"`
			Score float64 `json:"score"`
		} `json:"result"`
	}
	child, cancel = context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	filter := scopeFilter(classID, 0)
	if materials != nil {
		filter["must"] = append(filter["must"].([]any), map[string]any{"key": "material_id", "match": map[string]any{"any": materials}})
	}
	e = s.qcall(child, "POST", "/points/search", map[string]any{"vector": vectors[0], "filter": filter, "limit": 40, "score_threshold": 0.35, "with_payload": false, "with_vector": false}, &res)
	if e != nil {
		return nil, e
	}
	// Point IDs are only candidates. Never trust Qdrant for text or tenancy.
	if len(res.Result) > 40 {
		res.Result = res.Result[:40]
	}
	ids := []any{classID}
	marks := []string{}
	scores := map[int64]float64{}
	for _, v := range res.Result {
		if v.ID > 0 && v.Score >= 0.35 {
			if _, exists := scores[v.ID]; !exists {
				ids = append(ids, v.ID)
				marks = append(marks, "?")
			}
			scores[v.ID] = v.Score
		}
	}
	out := []Hit{}
	if len(marks) == 0 {
		return out, nil
	}
	ids = append(ids, s.Config.signature())
	scope, scopedArgs = materialScope("c.material_id", materials)
	ids = append(ids, scopedArgs...)
	rows, e := s.DB.QueryContext(ctx, "SELECT "+hitColumns+hitJoin+" AND c.id IN ("+strings.Join(marks, ",")+") AND j.model_signature=?"+scope, ids...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	for rows.Next() {
		h, e := scanHit(rows)
		if e != nil {
			return nil, e
		}
		score := scores[h.ChunkID]
		h.Score = score
		h.VectorScore = &score
		out = append(out, h)
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}
	sortHits(out)
	return out, nil
}
func sortHits(h []Hit) {
	sort.Slice(h, func(i, j int) bool {
		if h[i].Score == h[j].Score {
			return h[i].ChunkID < h[j].ChunkID
		}
		return h[i].Score > h[j].Score
	})
}
func mergeHits(keyword, vector []Hit, mode string, limit int) []Hit {
	all := map[int64]Hit{}
	for i, h := range keyword {
		rank := i + 1
		h.KeywordRank = &rank
		if mode == "hybrid" {
			h.Score = 1 / float64(60+rank)
		}
		all[h.ChunkID] = h
	}
	for i, h := range vector {
		rank := i + 1
		h.VectorRank = &rank
		if mode == "hybrid" {
			h.Score = 1 / float64(60+rank)
		}
		if old, ok := all[h.ChunkID]; ok {
			h.KeywordRank = old.KeywordRank
			h.KeywordScore = old.KeywordScore
			h.Score += old.Score
		}
		all[h.ChunkID] = h
	}
	out := make([]Hit, 0, len(all))
	for _, h := range all {
		out = append(out, h)
	}
	sortHits(out)
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}
func (s *Server) retrieve(ctx context.Context, classID int64, q, mode string, limit int) ([]Hit, error) {
	return s.retrieveScoped(ctx, classID, q, mode, limit, nil)
}

func (s *Server) retrieveScoped(ctx context.Context, classID int64, q, mode string, limit int, materials []int64) ([]Hit, error) {
	if materials != nil && len(materials) == 0 {
		return []Hit{}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	var k, v []Hit
	var e error
	if mode != "vector" {
		k, e = s.keyword(ctx, classID, q, materials)
		if e != nil {
			return nil, e
		}
	}
	if mode != "keyword" {
		v, e = s.vector(ctx, classID, q, materials)
		if e != nil {
			return nil, e
		}
	}
	return mergeHits(k, v, mode, limit), nil
}
func validQuestion(q string) bool {
	return utf8.RuneCountInString(q) > 0 && utf8.RuneCountInString(q) <= 500
}
func retrievalProblem(w http.ResponseWriter) {
	problem(w, 503, "检索或模型服务暂不可用，请检查服务端配置和索引状态；可尝试 keyword 关键字检索")
}
func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Q     string `json:"q"`
		Mode  string `json:"mode"`
		Limit *int   `json:"limit"`
	}
	if !decodeJSONRequest(w, r, &in, false) {
		return
	}
	in.Q = strings.TrimSpace(in.Q)
	if in.Mode == "" {
		in.Mode = "hybrid"
	}
	limit := 10
	if in.Limit != nil {
		limit = *in.Limit
	}
	if !validQuestion(in.Q) || (in.Mode != "keyword" && in.Mode != "vector" && in.Mode != "hybrid") || limit < 1 || limit > 20 {
		problem(w, 400, "问题、模式或数量无效")
		return
	}
	hits, e := s.retrieve(r.Context(), currentUser(r).ClassID, in.Q, in.Mode, limit)
	if e != nil {
		retrievalProblem(w)
		return
	}
	message := ""
	if len(hits) == 0 {
		message = noEvidence
	}
	// Search returns excerpts; full text is retrieved through protected sources.
	for i := range hits {
		hits[i].Text = ""
	}
	writeJSON(w, 200, map[string]any{"q": in.Q, "mode": in.Mode, "hits": hits, "message": message})
}
func (s *Server) chunkDetail(w http.ResponseWriter, r *http.Request) {
	mid, e1 := strconv.ParseInt(r.PathValue("id"), 10, 64)
	cid, e2 := strconv.ParseInt(r.PathValue("chunk_id"), 10, 64)
	if e1 != nil || e2 != nil || mid < 1 || cid < 1 {
		resourceError(w, sql.ErrNoRows)
		return
	}
	if e := s.materialAvailable(r.Context(), s.DB, mid, currentUser(r).ClassID); e != nil {
		resourceError(w, e)
		return
	}
	h, e := scanHit(s.DB.QueryRowContext(r.Context(), "SELECT "+hitColumns+hitJoin+" AND c.material_id=? AND c.id=?", currentUser(r).ClassID, mid, cid))
	if e != nil {
		resourceError(w, e)
		return
	}
	writeJSON(w, 200, h)
}

package campus

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
)

type textChunk struct {
	Text       string
	Start, End int
}

func splitText(body string) []textChunk {
	runes := []rune(body)
	out := []textChunk{}
	for start := 0; start < len(runes); {
		end := start + 800
		if end >= len(runes) {
			end = len(runes)
		} else {
			window := string(runes[start:end])
			for _, sep := range []string{"\n\n", "\n", "。"} {
				if pos := strings.LastIndex(window, sep); pos >= 0 {
					candidate := start + len([]rune(window[:pos+len(sep)]))
					if candidate-start > 80 {
						end = candidate
						break
					}
				}
			}
		}
		out = append(out, textChunk{string(runes[start:end]), start, end})
		if end == len(runes) {
			break
		}
		start = end - 80
	}
	return out
}
func (s *Server) enqueueMissing(ctx context.Context) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO knowledge_index_jobs(material_id,knowledge_entry_id,class_id) SELECT k.material_id,k.id,k.class_id FROM knowledge_entries k JOIN materials m ON m.id=k.material_id AND m.class_id=k.class_id LEFT JOIN knowledge_index_jobs j ON j.material_id=k.material_id WHERE j.id IS NULL AND m.deleted_at IS NULL ON DUPLICATE KEY UPDATE material_id=VALUES(material_id)`)
	return err
}

// StartIndexer is called once after storage reconciliation, never from Handler.
// This deployment deliberately supports one API instance, without a lease system.
func (s *Server) StartIndexer(ctx context.Context) error {
	if _, e := s.DB.ExecContext(ctx, "UPDATE knowledge_index_jobs SET status='pending' WHERE status='running'"); e != nil {
		return e
	}
	if e := s.enqueueMissing(ctx); e != nil {
		return e
	}
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			if ctx.Err() != nil {
				return
			}
			if e := s.enqueueMissing(ctx); e == nil {
				s.processNextIndex(ctx)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return nil
}

type indexJob struct {
	ID, Generation, MaterialID, KnowledgeID, ClassID int64
	Body                                             string
}

func (s *Server) processNextIndex(parent context.Context) bool {
	var job indexJob
	e := s.DB.QueryRowContext(parent, `SELECT j.id,j.generation,j.material_id,j.knowledge_entry_id,j.class_id,k.body_text FROM knowledge_index_jobs j JOIN knowledge_entries k ON k.id=j.knowledge_entry_id AND k.material_id=j.material_id AND k.class_id=j.class_id JOIN materials m ON m.id=j.material_id AND m.class_id=j.class_id WHERE j.status='pending' AND m.deleted_at IS NULL ORDER BY j.id LIMIT 1`).Scan(&job.ID, &job.Generation, &job.MaterialID, &job.KnowledgeID, &job.ClassID, &job.Body)
	if e != nil {
		return false
	}
	result, e := s.DB.ExecContext(parent, "UPDATE knowledge_index_jobs SET status='running',last_error_code='' WHERE id=? AND generation=? AND status='pending'", job.ID, job.Generation)
	if e != nil {
		return false
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return false
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Minute)
	defer cancel()
	e = s.buildIndex(ctx, job)
	if e != nil {
		code := "index_storage_error"
		var de *dependencyError
		if errors.As(e, &de) {
			code = de.Code
		}
		cleanup, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		_, _ = s.DB.ExecContext(cleanup, "UPDATE knowledge_index_jobs SET status='failed',last_error_code=? WHERE id=? AND generation=? AND status='running'", code, job.ID, job.Generation)
	}
	return true
}
func (s *Server) buildIndex(ctx context.Context, j indexJob) error {
	life := s.materialLife(j.MaterialID)
	life.work.Lock()
	defer life.work.Unlock()
	ctx, cancel := context.WithCancel(ctx)
	life.mu.Lock()
	life.cancel = cancel
	life.mu.Unlock()
	defer func() { cancel(); life.mu.Lock(); life.cancel = nil; life.mu.Unlock() }()
	if err := s.materialAvailable(ctx, s.DB, j.MaterialID, j.ClassID); err != nil {
		return err
	}
	var uncertain bool
	if err := s.DB.QueryRowContext(ctx, "SELECT vector_uncertain FROM materials WHERE id=?", j.MaterialID).Scan(&uncertain); err != nil {
		return err
	}
	if uncertain {
		return dep("vector_write_unconfirmed")
	}
	if !s.Config.embeddingReady() {
		return dep("embedding_not_configured")
	}
	var tokenSize int
	if e := s.DB.QueryRowContext(ctx, "SELECT @@ngram_token_size").Scan(&tokenSize); e != nil {
		return e
	}
	if tokenSize != 2 {
		return dep("ngram_configuration_mismatch")
	}
	if e := retryDependency(ctx, s.ensureCollection); e != nil {
		return e
	}
	if e := retryDependency(ctx, func(c context.Context) error {
		return s.qcall(c, "POST", "/points/delete?wait=true", map[string]any{"filter": scopeFilter(j.ClassID, j.MaterialID)}, nil)
	}); e != nil {
		return e
	}
	if _, e := s.DB.ExecContext(ctx, "DELETE FROM knowledge_chunks WHERE index_job_id=? AND generation=?", j.ID, j.Generation); e != nil {
		return e
	}
	chunks := splitText(j.Body)
	for start := 0; start < len(chunks); start += 16 {
		if err := s.materialAvailable(ctx, s.DB, j.MaterialID, j.ClassID); err != nil {
			return err
		}
		end := start + 16
		if end > len(chunks) {
			end = len(chunks)
		}
		texts := []string{}
		ids := []int64{}
		for i := start; i < end; i++ {
			v := chunks[i]
			res, e := s.DB.ExecContext(ctx, `INSERT INTO knowledge_chunks(index_job_id,generation,material_id,knowledge_entry_id,class_id,chunk_index,chunk_text,char_start,char_end) VALUES(?,?,?,?,?,?,?,?,?)`, j.ID, j.Generation, j.MaterialID, j.KnowledgeID, j.ClassID, i+1, v.Text, v.Start, v.End)
			if e != nil {
				return e
			}
			id, e := res.LastInsertId()
			if e != nil {
				return e
			}
			ids = append(ids, id)
			texts = append(texts, v.Text)
		}
		var vectors [][]float64
		if e := retryDependency(ctx, func(c context.Context) error { var e error; vectors, e = s.embed(c, texts); return e }); e != nil {
			return e
		}
		points := []any{}
		for i, id := range ids {
			points = append(points, map[string]any{"id": id, "vector": vectors[i], "payload": map[string]any{"class_id": j.ClassID, "material_id": j.MaterialID, "knowledge_entry_id": j.KnowledgeID, "chunk_id": id, "chunk_index": start + i + 1}})
		}
		if err := s.materialAvailable(ctx, s.DB, j.MaterialID, j.ClassID); err != nil {
			return err
		}
		// Persist uncertainty before sending. Do not retry an unconfirmed upsert:
		// a lost acknowledgement must not be mistaken for a stopped remote write.
		if _, e := s.DB.ExecContext(ctx, "UPDATE materials SET vector_uncertain=TRUE WHERE id=?", j.MaterialID); e != nil {
			return e
		}
		writeCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), 35*time.Second)
		e := s.qcall(writeCtx, "PUT", "/points?wait=true&ordering=strong", map[string]any{"points": points}, nil)
		var rejected *dependencyError
		if e == nil || (errors.As(e, &rejected) && rejected.Status >= 400 && rejected.Status < 500 && rejected.Status != 408 && rejected.Status != 429) {
			_, clearErr := s.DB.ExecContext(writeCtx, "UPDATE materials SET vector_uncertain=FALSE WHERE id=?", j.MaterialID)
			if e == nil {
				e = clearErr
			}
		}
		stop()
		if e != nil {
			return e
		}
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var deleted bool
	if e = tx.QueryRowContext(ctx, "SELECT deleted_at IS NOT NULL FROM materials WHERE id=? AND class_id=? FOR UPDATE", j.MaterialID, j.ClassID).Scan(&deleted); e != nil {
		return e
	}
	if deleted {
		return errMaterialDeleted
	}
	res, e := tx.ExecContext(ctx, "UPDATE knowledge_index_jobs SET status='ready',model_signature=?,last_error_code='' WHERE id=? AND generation=? AND status='running'", s.Config.signature(), j.ID, j.Generation)
	if e != nil {
		return e
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return dep("stale_generation")
	}
	if _, e = tx.ExecContext(ctx, "UPDATE knowledge_chunks SET index_status='ready' WHERE index_job_id=? AND generation=?", j.ID, j.Generation); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Server) reindex(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u.Role != "teacher" || !validCSRF(r, u) {
		problem(w, 403, "无重建权限或请求校验失败")
		return
	}
	m, e := s.ownedMaterial(r)
	if e != nil {
		resourceError(w, e)
		return
	}
	if e = s.enqueueMissing(r.Context()); e != nil {
		resourceError(w, e)
		return
	}
	tx, e := s.DB.BeginTx(r.Context(), nil)
	if e != nil {
		resourceError(w, e)
		return
	}
	defer tx.Rollback()
	var status string
	var id int64
	var deleted bool
	if e = tx.QueryRowContext(r.Context(), "SELECT deleted_at IS NOT NULL FROM materials WHERE id=? AND class_id=? FOR UPDATE", m.ID, u.ClassID).Scan(&deleted); e != nil {
		resourceError(w, e)
		return
	}
	if deleted {
		resourceError(w, errMaterialDeleted)
		return
	}
	if e = tx.QueryRowContext(r.Context(), "SELECT id,status FROM knowledge_index_jobs WHERE material_id=? AND class_id=? FOR UPDATE", m.ID, u.ClassID).Scan(&id, &status); e != nil {
		resourceError(w, e)
		return
	}
	if status == "pending" || status == "running" {
		problem(w, 409, "索引任务正在处理中")
		return
	}
	if _, e = tx.ExecContext(r.Context(), "DELETE FROM knowledge_chunks WHERE index_job_id=?", id); e == nil {
		_, e = tx.ExecContext(r.Context(), "UPDATE knowledge_index_jobs SET generation=generation+1,status='pending',last_error_code='' WHERE id=?", id)
	}
	if e != nil {
		resourceError(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		resourceError(w, e)
		return
	}
	writeJSON(w, 202, map[string]any{"material_id": m.ID, "index_status": "pending"})
}

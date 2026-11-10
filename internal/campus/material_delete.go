package campus

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var errMaterialDeleted = errors.New("material_deleted")

// A single API instance owns indexing and cleanup. Locks never span the
// withdrawal transaction: DELETE can revoke access while embedding is blocked.
type materialLife struct {
	work   sync.Mutex
	mu     sync.Mutex
	cancel context.CancelFunc
}

func (s *Server) materialLife(id int64) *materialLife {
	v, _ := s.materialLives.LoadOrStore(id, &materialLife{})
	return v.(*materialLife)
}

func migrateMaterialDeletion(ctx context.Context, db *sql.DB) error {
	for _, col := range []struct{ name, ddl string }{
		{"deleted_at", "ALTER TABLE materials ADD COLUMN deleted_at DATETIME(6) NULL"},
		{"vector_uncertain", "ALTER TABLE materials ADD COLUMN vector_uncertain BOOLEAN NOT NULL DEFAULT FALSE"},
	} {
		var n int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='materials' AND column_name=?", col.name).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			if _, err := db.ExecContext(ctx, col.ddl); err != nil {
				return err
			}
		}
	}
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS material_deletions (
 material_id BIGINT PRIMARY KEY, class_id BIGINT NOT NULL, knowledge_entry_id BIGINT NOT NULL,
 storage_key VARCHAR(255) NOT NULL, status VARCHAR(20) NOT NULL DEFAULT 'cleanup_pending',
 attempts INT NOT NULL DEFAULT 0, last_error_code VARCHAR(60) NOT NULL DEFAULT '',
 next_attempt DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 INDEX cleanup_due(status,next_attempt), UNIQUE KEY deleted_entry(knowledge_entry_id),
 FOREIGN KEY(material_id,class_id) REFERENCES materials(id,class_id),
 CHECK(status IN ('cleanup_pending','deleted'))
 ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`)
	return err
}

func (s *Server) materialAvailable(ctx context.Context, db aiReader, id, classID int64) error {
	var deleted sql.NullTime
	if err := db.QueryRowContext(ctx, "SELECT deleted_at FROM materials WHERE id=? AND class_id=?", id, classID).Scan(&deleted); err != nil {
		return err
	}
	if deleted.Valid {
		return errMaterialDeleted
	}
	return nil
}

func (s *Server) deleteMaterial(w http.ResponseWriter, r *http.Request) {
	if !aiAllowed(w, r, "teacher") {
		return
	}
	id, ok := aiID(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	u := currentUser(r)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		aiDBError(w, err)
		return
	}
	defer tx.Rollback()
	var deleted sql.NullTime
	var key string
	err = tx.QueryRowContext(ctx, "SELECT deleted_at,storage_key FROM materials WHERE id=? AND class_id=? FOR UPDATE", id, u.ClassID).Scan(&deleted, &key)
	if err != nil {
		aiDBError(w, err)
		return
	}
	if !deleted.Valid {
		var kid int64
		if err = tx.QueryRowContext(ctx, "SELECT id FROM knowledge_entries WHERE material_id=? AND class_id=?", id, u.ClassID).Scan(&kid); err != nil {
			aiDBError(w, err)
			return
		}
		if _, err = tx.ExecContext(ctx, "UPDATE materials SET deleted_at=CURRENT_TIMESTAMP(6) WHERE id=? AND class_id=?", id, u.ClassID); err == nil {
			_, err = tx.ExecContext(ctx, "DELETE FROM assistant_materials WHERE material_id=? AND class_id=?", id, u.ClassID)
		}
		if err == nil {
			_, err = tx.ExecContext(ctx, "INSERT INTO material_deletions(material_id,class_id,knowledge_entry_id,storage_key) VALUES(?,?,?,?)", id, u.ClassID, kid, key)
		}
		if err == nil {
			err = s.inject("delete-before-commit")
		}
		if err != nil {
			aiDBError(w, err)
			return
		}
	}
	var status string
	if err = tx.QueryRowContext(ctx, "SELECT status FROM material_deletions WHERE material_id=? AND class_id=?", id, u.ClassID).Scan(&status); err != nil {
		aiDBError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		aiDBError(w, err)
		return
	}
	life := s.materialLife(id)
	life.mu.Lock()
	if life.cancel != nil {
		life.cancel()
	}
	life.mu.Unlock()
	code := 202
	if status == "deleted" {
		code = 200
	}
	writeJSON(w, code, map[string]any{"material_id": id, "status": status})
}

// Cleanup has its own worker so an unavailable external service never stalls
// indexing of other materials. Failed jobs are durable and use bounded backoff.
func (s *Server) StartMaterialCleanup(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			if ctx.Err() != nil {
				return
			}
			s.processMaterialCleanup(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (s *Server) processMaterialCleanup(ctx context.Context) bool {
	rows, err := s.DB.QueryContext(ctx, "SELECT material_id FROM material_deletions WHERE status='cleanup_pending' AND next_attempt<=CURRENT_TIMESTAMP(6) ORDER BY next_attempt,material_id LIMIT 20")
	if err != nil {
		return false
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if rows.Scan(&id) != nil {
			rows.Close()
			return false
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return false
	}
	for _, id := range ids {
		life := s.materialLife(id)
		if !life.work.TryLock() {
			continue
		}
		child, cancel := context.WithTimeout(ctx, 35*time.Second)
		err = s.cleanMaterial(child, id)
		cancel()
		life.work.Unlock()
		if err != nil {
			code := "cleanup_storage_error"
			var de *dependencyError
			if errors.As(err, &de) {
				code = de.Code
			}
			retry, c := context.WithTimeout(context.Background(), 3*time.Second)
			_, _ = s.DB.ExecContext(retry, `UPDATE material_deletions SET attempts=attempts+1,last_error_code=?,
    next_attempt=TIMESTAMPADD(SECOND,LEAST(300,POW(2,LEAST(attempts,8))),CURRENT_TIMESTAMP(6))
    WHERE material_id=? AND status='cleanup_pending'`, code, id)
			c()
		}
		return true
	}
	return false
}

func (s *Server) cleanMaterial(ctx context.Context, id int64) error {
	var classID int64
	var key, status string
	var uncertain bool
	err := s.DB.QueryRowContext(ctx, `SELECT d.class_id,d.storage_key,d.status,m.vector_uncertain
 FROM material_deletions d JOIN materials m ON m.id=d.material_id AND m.class_id=d.class_id
 WHERE d.material_id=? AND m.deleted_at IS NOT NULL`, id).Scan(&classID, &key, &status, &uncertain)
	if err != nil {
		return err
	}
	if status == "deleted" {
		return nil
	}
	if s.Config.QdrantURL == "" {
		return dep("vector_cleanup_not_configured")
	}
	err = s.qcall(ctx, "POST", "/points/delete?wait=true&ordering=strong", map[string]any{"filter": scopeFilter(classID, id)}, nil)
	var de *dependencyError
	if err != nil && !(errors.As(err, &de) && de.Status == 404) {
		return err
	}
	// A timeout/crash during an upsert may have lost its acknowledgement. No
	// finite local delay proves the remote write stopped. Keep sweeping but do
	// not claim complete until an operator has resolved that uncertainty.
	if uncertain {
		return dep("vector_write_unconfirmed")
	}
	if err = s.inject("delete-after-vector"); err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, table := range []string{"knowledge_chunks", "knowledge_index_jobs", "knowledge_entries"} {
		if _, err = tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE material_id=? AND class_id=?", id, classID); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if err = s.inject("delete-after-database"); err != nil {
		return err
	}
	path, err := s.storagePath(key)
	if err != nil {
		return err
	}
	// Reject symlinks in every existing component before removing the file.
	root, err := filepath.Abs(s.Config.UploadDir)
	if err != nil {
		return err
	}
	target, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == "." || !filepath.IsLocal(rel) {
		return errors.New("invalid cleanup path")
	}
	for p := target; ; p = filepath.Dir(p) {
		st, e := os.Lstat(p)
		if e != nil && !os.IsNotExist(e) {
			return e
		}
		if e == nil && st.Mode()&os.ModeSymlink != 0 {
			return errors.New("unsafe cleanup path")
		}
		if p == root {
			break
		}
	}
	if err = s.inject("delete-file"); err != nil {
		return err
	}
	if err = os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err = s.inject("delete-after-file"); err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, "UPDATE material_deletions SET status='deleted',storage_key='',last_error_code='' WHERE material_id=? AND status='cleanup_pending'", id)
	return err
}

// The same sanitization is used for history, persisted final metadata and done.
func refreshCitations(ctx context.Context, db aiReader, classID int64, items []Citation) error {
	for i := range items {
		c := &items[i]
		if c.SourceStatus == "material_deleted" {
			continue
		}
		var deleted sql.NullTime
		err := db.QueryRowContext(ctx, "SELECT deleted_at FROM materials WHERE id=? AND class_id=?", c.MaterialID, classID).Scan(&deleted)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if errors.Is(err, sql.ErrNoRows) || deleted.Valid {
			status := "source_unavailable"
			if deleted.Valid {
				status = "material_deleted"
			}
			*c = Citation{Number: c.Number, Hit: Hit{SourceStatus: status}}
			continue
		}
		c.SourceStatus = "available"
	}
	return nil
}

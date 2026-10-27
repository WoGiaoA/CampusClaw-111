package campus

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed schema.sql
var schema string

func Migrate(ctx context.Context, db *sql.DB) error {
	for _, statement := range strings.Split(schema, ";") {
		if strings.TrimSpace(statement) != "" {
			if _, err := db.ExecContext(ctx, statement); err != nil {
				return err
			}
		}
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='users' AND column_name='must_change_password'").Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		_, err := db.ExecContext(ctx, "ALTER TABLE users ADD COLUMN must_change_password BOOLEAN NOT NULL DEFAULT FALSE")
		return err
	}
	return nil
}

// Seed is deliberately separate from schema creation; all missing secrets are
// validated before writing any seed data. Existing users are never rehashed.
func (s *Server) Seed(ctx context.Context) error {
	var count int
	if err := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	names := []string{"teacher_a", "teacher_b", "student_a1", "student_b1"}
	hashes := map[string][]byte{}
	for _, name := range names {
		password := s.Config.SeedPasswords[name]
		if password == demoInitialPassword && !s.Config.DemoMode {
			return errors.New("default seed password requires local demo mode")
		}
		if len(password) == 0 || len(password) > 72 {
			return fmt.Errorf("missing or invalid SEED_%s_PASSWORD", strings.ToUpper(name))
		}
		h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		hashes[name] = h
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, code := range []string{"A", "B"} {
		if _, err = tx.ExecContext(ctx, "INSERT INTO classes(code,name) VALUES (?,?)", code, code+" 班"); err != nil {
			return err
		}
	}
	for _, name := range names {
		code, role := "A", "student"
		if strings.HasSuffix(name, "b") || strings.HasSuffix(name, "b1") {
			code = "B"
		}
		if strings.HasPrefix(name, "teacher") {
			role = "teacher"
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO users(username,password_hash,role,class_id,must_change_password) SELECT ?,?,?,id,? FROM classes WHERE code=?", name, string(hashes[name]), role, s.Config.SeedPasswords[name] == demoInitialPassword, code); err != nil {
			return err
		}
	}
	var paths []string
	committed := false
	defer func() {
		if !committed {
			for _, p := range paths {
				_ = os.Remove(p)
			}
		}
	}()
	for _, sample := range []struct{ code, title, body, ext string }{{"A", "A 班-课程导论.txt", "CLASS_A_ONLY：课程导论\n", ".txt"}, {"B", "B 班-学习指南.md", "CLASS_B_ONLY：学习指南\n", ".md"}} {
		var classID, userID int64
		if err = tx.QueryRowContext(ctx, "SELECT c.id,u.id FROM classes c JOIN users u ON u.class_id=c.id WHERE c.code=? AND u.role='teacher'", sample.code).Scan(&classID, &userID); err != nil {
			return err
		}
		key, path, e := s.saveFile(classID, sample.ext, []byte(sample.body))
		if e != nil {
			return e
		}
		paths = append(paths, path)
		result, e := tx.ExecContext(ctx, "INSERT INTO materials(class_id,uploader,title,original_filename,storage_key,seed_key,byte_size) VALUES(?,?,?,?,?,?,?)", classID, userID, sample.title, sample.title, key, sample.code+"-sample", len([]byte(sample.body)))
		if e != nil {
			return e
		}
		id, _ := result.LastInsertId()
		if _, e = tx.ExecContext(ctx, "INSERT INTO knowledge_entries(material_id,class_id,body_text) VALUES(?,?,?)", id, classID, sample.body); e != nil {
			return e
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

// Reconcile runs before the server accepts requests. Only files inside this
// application's upload root are touched; committed files are never removed.
func (s *Server) Reconcile(ctx context.Context) error {
	if err := os.MkdirAll(s.Config.UploadDir, 0700); err != nil {
		return err
	}
	rows, err := s.DB.QueryContext(ctx, "SELECT m.storage_key,k.id FROM materials m LEFT JOIN knowledge_entries k ON k.material_id=m.id AND k.class_id=m.class_id")
	if err != nil {
		return err
	}
	defer rows.Close()
	referenced := map[string]bool{}
	for rows.Next() {
		var key string
		var kid sql.NullInt64
		if err = rows.Scan(&key, &kid); err != nil {
			return err
		}
		if !kid.Valid {
			return errors.New("incomplete material/knowledge association")
		}
		p, e := s.storagePath(key)
		if e != nil {
			return e
		}
		if _, e = os.Stat(p); e != nil {
			return errors.New("committed material file missing")
		}
		referenced[filepath.Clean(p)] = true
	}
	if err = rows.Err(); err != nil {
		return err
	}
	return filepath.WalkDir(s.Config.UploadDir, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.Type()&os.ModeSymlink != 0 {
			return errors.New("symlink not allowed in upload root")
		}
		if !d.IsDir() && !referenced[filepath.Clean(p)] {
			return os.Remove(p)
		}
		return nil
	})
}

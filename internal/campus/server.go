package campus

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"golang.org/x/crypto/bcrypt"
	"log"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type Server struct {
	DB        *sql.DB
	Config    Config
	dummyHash []byte
	mu        sync.Mutex
	failures  map[string]attempts
	uploadMu  sync.Mutex
	unhealthy atomic.Bool
	// Test-only failure injection, never controlled by HTTP or env.
	fail func(string) error
	now  func() time.Time
}
type attempts struct {
	Count int
	Start time.Time
}
type User struct {
	MustChangePassword bool   `json:"must_change_password"`
	ID                 int64  `json:"id"`
	Username           string `json:"username"`
	Role               string `json:"role"`
	ClassID            int64  `json:"class_id"`
	ClassName          string `json:"class_name"`
	CSRF               string `json:"csrf_token,omitempty"`
}
type identityKey struct{}

func New(db *sql.DB, c Config) (*Server, error) {
	if len(c.SessionKey) < 32 {
		return nil, errors.New("SESSION_KEY requires at least 32 bytes")
	}
	h, e := bcrypt.GenerateFromPassword([]byte("unusable-dummy-password-for-equal-work"), bcrypt.DefaultCost)
	if e != nil {
		return nil, e
	}
	return &Server{DB: db, Config: c, dummyHash: h, failures: map[string]attempts{}, now: time.Now}, nil
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("POST /api/login", s.login)
	mux.HandleFunc("GET /api/account-options", s.accountOptions)
	mux.HandleFunc("POST /api/register", s.register)
	mux.Handle("POST /api/password", s.auth(http.HandlerFunc(s.changePassword)))
	mux.Handle("GET /api/me", s.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, currentUser(r)) })))
	mux.Handle("POST /api/logout", s.auth(http.HandlerFunc(s.logout)))
	mux.Handle("GET /api/materials", s.auth(http.HandlerFunc(s.listMaterials)))
	mux.Handle("GET /api/materials/{id}", s.auth(http.HandlerFunc(s.materialDetail)))
	mux.Handle("GET /api/materials/{id}/file", s.auth(http.HandlerFunc(s.download)))
	mux.Handle("POST /api/materials/upload", s.auth(http.HandlerFunc(s.upload)))
	mux.Handle("GET /api/knowledge-entries", s.auth(http.HandlerFunc(s.listKnowledge)))
	mux.Handle("GET /api/knowledge-entries/{id}", s.auth(http.HandlerFunc(s.knowledgeDetail)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		mux.ServeHTTP(w, r)
	})
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func problem(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
func currentUser(r *http.Request) User { return r.Context().Value(identityKey{}).(User) }
func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(cookieName)
		if err != nil || len(cookie.Value) != 64 {
			problem(w, 401, "请先登录")
			return
		}
		var u User
		err = s.DB.QueryRowContext(r.Context(), `SELECT u.id,u.username,u.role,u.class_id,c.name,se.csrf_token,u.must_change_password FROM sessions se JOIN users u ON se.user_id=u.id JOIN classes c ON c.id=u.class_id WHERE se.id_hash=? AND se.expires_at>?`, s.sessionHash(cookie.Value), s.now().UTC()).Scan(&u.ID, &u.Username, &u.Role, &u.ClassID, &u.ClassName, &u.CSRF, &u.MustChangePassword)
		if errors.Is(err, sql.ErrNoRows) {
			problem(w, 401, "请先登录")
			return
		}
		if err != nil {
			problem(w, 503, "服务暂不可用")
			return
		}
		if u.MustChangePassword && r.URL.Path != "/api/me" && r.URL.Path != "/api/password" && r.URL.Path != "/api/logout" {
			writeJSON(w, 403, map[string]string{"error": "请先修改初始密码", "code": "password_change_required"})
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), identityKey{}, u)))
	})
}
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	ok := !s.unhealthy.Load() && s.DB.PingContext(ctx) == nil
	if ok {
		var one int
		err := s.DB.QueryRowContext(ctx, "SELECT 1 FROM materials LIMIT 1").Scan(&one)
		ok = err == nil || errors.Is(err, sql.ErrNoRows)
	}
	if ok {
		_, err := os.ReadDir(s.Config.UploadDir)
		ok = err == nil
	}
	if ok {
		f, e := os.CreateTemp(s.Config.UploadDir, ".health-")
		if e != nil {
			ok = false
		} else {
			p := f.Name()
			if _, e = f.Write([]byte("ok")); e != nil {
				ok = false
			}
			if e = f.Close(); e != nil {
				ok = false
			}
			if e = os.Remove(p); e != nil {
				ok = false
			}
		}
	}
	if !ok {
		writeJSON(w, 503, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}
func (s *Server) inject(stage string) error {
	if s.fail != nil {
		return s.fail(stage)
	}
	return nil
}
func logReason(reason string) { log.Printf("security event: %s", reason) }

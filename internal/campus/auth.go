package campus

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"golang.org/x/crypto/bcrypt"
	"net"
	"net/http"
	"time"
)

const cookieName = "campusclaw_session"

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return hex.EncodeToString(b), nil
}
func (s *Server) sessionHash(id string) string {
	h := hmac.New(sha256.New, []byte(s.Config.SessionKey))
	_, _ = h.Write([]byte(id))
	return hex.EncodeToString(h.Sum(nil))
}
func (s *Server) cookie(value string, age int) *http.Cookie {
	return &http.Cookie{Name: cookieName, Value: value, Path: "/", HttpOnly: true, Secure: s.Config.SecureCookie, SameSite: http.SameSiteLaxMode, MaxAge: age}
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSONRequest(w, r, &input, false) {
		return
	}
	var u User
	var passwordHash string
	err := s.DB.QueryRowContext(r.Context(), "SELECT u.id,u.username,u.role,u.class_id,c.name,u.password_hash,u.must_change_password FROM users u JOIN classes c ON u.class_id=c.id WHERE u.username=?", input.Username).Scan(&u.ID, &u.Username, &u.Role, &u.ClassID, &u.ClassName, &passwordHash, &u.MustChangePassword)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		problem(w, 503, "服务暂不可用")
		return
	}
	unknown := errors.Is(err, sql.ErrNoRows)
	hash := []byte(passwordHash)
	if unknown {
		hash = s.dummyHash
	}
	verify := bcrypt.CompareHashAndPassword(hash, []byte(input.Password))
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	s.mu.Lock()
	a := s.failures[ip]
	if s.now().Sub(a.Start) >= time.Minute {
		a = attempts{Start: s.now()}
	}
	limited := a.Count >= 10
	if unknown || verify != nil || limited {
		a.Count++
		s.failures[ip] = a
	} else {
		delete(s.failures, ip)
	}
	// Keep the teaching limit bounded even under many distinct clients.
	if len(s.failures) > 4096 {
		for key, v := range s.failures {
			if s.now().Sub(v.Start) >= time.Minute {
				delete(s.failures, key)
			}
		}
	}
	s.mu.Unlock()
	if unknown || verify != nil || limited {
		reason := "wrong password"
		if unknown {
			reason = "unknown account"
		}
		if limited {
			reason = "login rate limit"
		}
		logReason(reason)
		problem(w, 401, "账号或密码错误")
		return
	}
	if s.inject("login-verified") != nil {
		problem(w, 503, "服务暂不可用")
		return
	}
	id, e := randomToken()
	if e != nil {
		problem(w, 500, "登录失败")
		return
	}
	csrf, e := randomToken()
	if e != nil {
		problem(w, 500, "登录失败")
		return
	}
	tx, e := s.DB.BeginTx(r.Context(), nil)
	if e != nil {
		problem(w, 503, "服务暂不可用")
		return
	}
	defer tx.Rollback()
	// Serialize credential changes and session creation for this user.
	var lockedHash string
	if e = tx.QueryRowContext(r.Context(), "SELECT password_hash,must_change_password FROM users WHERE id=? FOR UPDATE", u.ID).Scan(&lockedHash, &u.MustChangePassword); e != nil {
		problem(w, 503, "服务暂不可用")
		return
	}
	if lockedHash != passwordHash {
		problem(w, 401, "账号或密码错误")
		return
	}
	if old, e := r.Cookie(cookieName); e == nil {
		if _, e = tx.ExecContext(r.Context(), "DELETE FROM sessions WHERE id_hash=?", s.sessionHash(old.Value)); e != nil {
			problem(w, 503, "服务暂不可用")
			return
		}
	}
	if _, e = tx.ExecContext(r.Context(), "INSERT INTO sessions(id_hash,user_id,expires_at,csrf_token) VALUES(?,?,?,?)", s.sessionHash(id), u.ID, s.now().UTC().Add(8*time.Hour), csrf); e != nil {
		problem(w, 503, "服务暂不可用")
		return
	}
	if e = tx.Commit(); e != nil {
		problem(w, 503, "服务暂不可用")
		return
	}
	http.SetCookie(w, s.cookie(id, 8*60*60))
	writeJSON(w, 200, u)
}
func validCSRF(r *http.Request, u User) bool {
	return u.CSRF != "" && subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(u.CSRF)) == 1
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if !validCSRF(r, currentUser(r)) {
		problem(w, 403, "请求校验失败")
		return
	}
	c, _ := r.Cookie(cookieName)
	if _, e := s.DB.ExecContext(r.Context(), "DELETE FROM sessions WHERE id_hash=?", s.sessionHash(c.Value)); e != nil {
		problem(w, 503, "服务暂不可用")
		return
	}
	http.SetCookie(w, s.cookie("", -1))
	writeJSON(w, 200, map[string]bool{"ok": true})
}

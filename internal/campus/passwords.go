package campus

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"regexp"

	"github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

const demoInitialPassword = "123456"

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_]{3,32}$`)

func decodeAccountJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	return decodeJSONRequest(w, r, target, true)
}

func (s *Server) accountOptions(w http.ResponseWriter, r *http.Request) {
	classes := []map[string]string{}
	if s.Config.DemoMode {
		rows, e := s.DB.QueryContext(r.Context(), "SELECT code,name FROM classes WHERE code IN ('A','B') ORDER BY code")
		if e != nil {
			problem(w, 503, "服务暂不可用")
			return
		}
		defer rows.Close()
		for rows.Next() {
			var code, name string
			if rows.Scan(&code, &name) != nil {
				problem(w, 503, "服务暂不可用")
				return
			}
			classes = append(classes, map[string]string{"code": code, "name": name})
		}
		if rows.Err() != nil {
			problem(w, 503, "服务暂不可用")
			return
		}
	}
	writeJSON(w, 200, map[string]any{"demo_registration": s.Config.DemoMode, "classes": classes})
}
func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	if !s.Config.DemoMode {
		problem(w, 403, "当前环境不开放注册")
		return
	}
	// Public registration does not use a session; refuse browser cross-origin forms.
	if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+r.Host && origin != "https://"+r.Host {
		problem(w, 403, "请求来源无效")
		return
	}
	var input struct {
		Username  string `json:"username"`
		ClassCode string `json:"class_code"`
	}
	if !decodeAccountJSON(w, r, &input) {
		return
	}
	if !usernamePattern.MatchString(input.Username) || (input.ClassCode != "A" && input.ClassCode != "B") {
		problem(w, 400, "用户名需为 3–32 位字母数字下划线，班级仅 A/B")
		return
	}
	var classID int64
	e := s.DB.QueryRowContext(r.Context(), "SELECT id FROM classes WHERE code=?", input.ClassCode).Scan(&classID)
	if errors.Is(e, sql.ErrNoRows) {
		problem(w, 400, "班级不存在")
		return
	}
	if e != nil {
		problem(w, 503, "服务暂不可用")
		return
	}
	hash, e := bcrypt.GenerateFromPassword([]byte(demoInitialPassword), bcrypt.DefaultCost)
	if e != nil {
		problem(w, 500, "注册失败")
		return
	}
	_, e = s.DB.ExecContext(r.Context(), "INSERT INTO users(username,password_hash,role,class_id,must_change_password) VALUES(?,?,'student',?,TRUE)", input.Username, string(hash), classID)
	if e != nil {
		var m *mysql.MySQLError
		if errors.As(e, &m) && m.Number == 1062 {
			problem(w, 409, "用户名已存在")
		} else {
			problem(w, 503, "注册失败")
		}
		return
	}
	writeJSON(w, 201, map[string]any{"ok": true, "must_change_password": true})
}
func validNewPassword(value string) bool {
	if len(value) < 8 || len(value) > 72 || value == demoInitialPassword {
		return false
	}
	letter, digit := false, false
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
			letter = true
		}
		if r >= '0' && r <= '9' {
			digit = true
		}
	}
	return letter && digit
}
func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if !validCSRF(r, u) {
		problem(w, 403, "请求校验失败")
		return
	}
	var input struct {
		Current string `json:"current_password"`
		New     string `json:"new_password"`
		Confirm string `json:"confirm_password"`
	}
	if !decodeAccountJSON(w, r, &input) {
		return
	}
	if input.New != input.Confirm || !validNewPassword(input.New) {
		problem(w, 400, "新密码须为 8–72 字节并含字母和数字，两次输入一致")
		return
	}
	tx, e := s.DB.BeginTx(r.Context(), nil)
	if e != nil {
		problem(w, 503, "服务暂不可用")
		return
	}
	defer tx.Rollback()
	var hash string
	var first bool
	if e = tx.QueryRowContext(r.Context(), "SELECT password_hash,must_change_password FROM users WHERE id=? FOR UPDATE", u.ID).Scan(&hash, &first); e != nil {
		problem(w, 503, "服务暂不可用")
		return
	}
	cookie, _ := r.Cookie(cookieName)
	var exists int
	if e = tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM sessions WHERE id_hash=? AND user_id=? AND expires_at>?", s.sessionHash(cookie.Value), u.ID, s.now().UTC()).Scan(&exists); e != nil {
		problem(w, 503, "服务暂不可用")
		return
	}
	if exists == 0 {
		problem(w, 401, "请重新登录")
		return
	}
	if !first && bcrypt.CompareHashAndPassword([]byte(hash), []byte(input.Current)) != nil {
		problem(w, 400, "当前密码错误")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(input.New)) == nil {
		problem(w, 400, "新密码不能与当前密码相同")
		return
	}
	newHash, e := bcrypt.GenerateFromPassword([]byte(input.New), bcrypt.DefaultCost)
	if e != nil {
		problem(w, 500, "修改失败")
		return
	}
	if _, e = tx.ExecContext(r.Context(), "UPDATE users SET password_hash=?,must_change_password=FALSE WHERE id=?", string(newHash), u.ID); e != nil {
		problem(w, 503, "修改失败")
		return
	}
	if _, e = tx.ExecContext(r.Context(), "DELETE FROM sessions WHERE user_id=?", u.ID); e != nil {
		problem(w, 503, "修改失败")
		return
	}
	if e = tx.Commit(); e != nil {
		problem(w, 503, "修改失败，请重新登录确认")
		return
	}
	http.SetCookie(w, s.cookie("", -1))
	writeJSON(w, 200, map[string]bool{"ok": true, "login_required": true})
}

// ResetDemoPasswords is an explicit administrative operation, never called by Seed/startup.
func (s *Server) ResetDemoPasswords(ctx context.Context, databaseAddress string) error {
	if err := ValidateDemoReset(s.Config, databaseAddress); err != nil {
		return err
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return errors.New("reset transaction failed")
	}
	defer tx.Rollback()
	for _, name := range []string{"teacher_a", "teacher_b", "student_a1", "student_b1"} {
		var id int64
		if e = tx.QueryRowContext(ctx, "SELECT id FROM users WHERE username=? FOR UPDATE", name).Scan(&id); e != nil {
			return errors.New("reset requires all four preset accounts")
		}
		hash, e := bcrypt.GenerateFromPassword([]byte(demoInitialPassword), bcrypt.DefaultCost)
		if e != nil {
			return errors.New("reset hashing failed")
		}
		if _, e = tx.ExecContext(ctx, "UPDATE users SET password_hash=?,must_change_password=TRUE WHERE id=?", string(hash), id); e != nil {
			return errors.New("reset update failed")
		}
		if _, e = tx.ExecContext(ctx, "DELETE FROM sessions WHERE user_id=?", id); e != nil {
			return errors.New("reset session revocation failed")
		}
	}
	if e = tx.Commit(); e != nil {
		return errors.New("reset commit failed")
	}
	return nil
}

// Validate before connecting or running any schema migration as well.
func ValidateDemoReset(config Config, databaseAddress string) error {
	host, _, e := net.SplitHostPort(databaseAddress)
	ip := net.ParseIP(host)
	if !config.DemoMode || e != nil || !(host == "localhost" || ip != nil && ip.IsLoopback()) {
		return errors.New("reset requires local demo mode and loopback MySQL")
	}
	return nil
}

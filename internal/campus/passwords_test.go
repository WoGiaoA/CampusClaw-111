package campus

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func accountRequest(t *testing.T, server *httptest.Server, path string, value any, cookie *http.Cookie, csrf string) (int, http.Header, []byte) {
	t.Helper()
	data, _ := json.Marshal(value)
	return request(t, server, "POST", path, bytes.NewReader(data), cookie, csrf, "application/json")
}
func passwordLogin(t *testing.T, server *httptest.Server, name, password string) (*http.Cookie, User) {
	t.Helper()
	code, headers, body := accountRequest(t, server, "/api/login", map[string]string{"username": name, "password": password}, nil, "")
	if code != 200 {
		t.Fatalf("password login: %d", code)
	}
	response := http.Response{Header: headers}
	cookies := response.Cookies()
	if len(cookies) != 1 {
		t.Fatal("missing session")
	}
	var u User
	json.Unmarshal(body, &u)
	code, _, body = request(t, server, "GET", "/api/me", nil, cookies[0], "", "")
	if code != 200 {
		t.Fatal(code)
	}
	json.Unmarshal(body, &u)
	return cookies[0], u
}
func TestDemoResetFirstLogin(t *testing.T) {
	s, server := testServer(t)
	s.Config.DemoMode = true
	before := snapshot(t, s)
	var identities string
	rows, e := s.DB.Query("SELECT CONCAT(id,':',username,':',role,':',class_id) FROM users ORDER BY id")
	if e != nil {
		t.Fatal(e)
	}
	for rows.Next() {
		var v string
		rows.Scan(&v)
		identities += v
	}
	rows.Close()
	oldCookie, _ := loginAs(t, s, server, "teacher_a")
	if e = s.ResetDemoPasswords(context.Background(), "db:3306"); e == nil {
		t.Fatal("non-local reset accepted")
	}
	if e = s.ResetDemoPasswords(context.Background(), "127.0.0.1:13306"); e != nil {
		t.Fatal(e)
	}
	if before != snapshot(t, s) {
		t.Fatal("reset touched business data")
	}
	var afterIdentities string
	rows, _ = s.DB.Query("SELECT CONCAT(id,':',username,':',role,':',class_id) FROM users ORDER BY id")
	for rows.Next() {
		var v string
		rows.Scan(&v)
		afterIdentities += v
	}
	rows.Close()
	if identities != afterIdentities {
		t.Fatal("reset touched identities")
	}
	code, _, _ := request(t, server, "GET", "/api/me", nil, oldCookie, "", "")
	if code != 401 {
		t.Fatal("old session retained")
	}
	hashes := map[string]bool{}
	for _, name := range []string{"teacher_a", "teacher_b", "student_a1", "student_b1"} {
		code, _, _ := accountRequest(t, server, "/api/login", map[string]string{"username": name, "password": s.Config.SeedPasswords[name]}, nil, "")
		if code != 401 {
			t.Fatal("old password retained")
		}
		c, u := passwordLogin(t, server, name, demoInitialPassword)
		if !u.MustChangePassword {
			t.Fatal("first change not required")
		}
		var hash string
		s.DB.QueryRow("SELECT password_hash FROM users WHERE username=?", name).Scan(&hash)
		if hash == demoInitialPassword || bcrypt.CompareHashAndPassword([]byte(hash), []byte(demoInitialPassword)) != nil || hashes[hash] {
			t.Fatal("hash/salt invalid")
		}
		hashes[hash] = true
		for _, path := range []string{"/api/materials", "/api/materials/1", "/api/materials/1/file", "/api/knowledge-entries", "/api/knowledge-entries/1"} {
			code, _, body := request(t, server, "GET", path, nil, c, "", "")
			if code != 403 || !bytes.Contains(body, []byte("password_change_required")) {
				t.Fatalf("first permission %s %d", path, code)
			}
		}
		code, _ = uploadRequest(t, server, c, u.CSRF, "a.txt", []byte("hello"), nil)
		if code != 403 {
			t.Fatal("first upload allowed")
		}
		code, _, _ = accountRequest(t, server, "/api/password", map[string]string{"new_password": "Personal9876", "confirm_password": "Personal9876"}, c, u.CSRF)
		if code != 200 {
			t.Fatal("first change", code)
		}
		code, _, _ = request(t, server, "GET", "/api/me", nil, c, "", "")
		if code != 401 {
			t.Fatal("changed session retained")
		}
		code, _, _ = accountRequest(t, server, "/api/login", map[string]string{"username": name, "password": demoInitialPassword}, nil, "")
		if code != 401 {
			t.Fatal("initial still valid")
		}
		fresh, identity := passwordLogin(t, server, name, "Personal9876")
		if identity.MustChangePassword {
			t.Fatal("flag retained")
		}
		code, _, _ = request(t, server, "GET", "/api/materials", nil, fresh, "", "")
		if code != 200 {
			t.Fatal("material blocked after change")
		}
	}
	if e = s.Seed(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e = Migrate(context.Background(), s.DB); e != nil {
		t.Fatal(e)
	}
	passwordLogin(t, server, "teacher_a", "Personal9876")
	if before != snapshot(t, s) {
		t.Fatal("business data changed")
	}
}
func TestRegistrationPasswordManagement(t *testing.T) {
	s, server := testServer(t)
	s.Config.DemoMode = true
	for _, code := range []string{"A", "B"} {
		status, _, body := accountRequest(t, server, "/api/register", map[string]string{"username": "registered_" + code, "class_code": code}, nil, "")
		if status != 201 || bytes.Contains(body, []byte(demoInitialPassword)) || bytes.Contains(body, []byte("hash")) {
			t.Fatal("registration", status)
		}
		c, u := passwordLogin(t, server, "registered_"+code, demoInitialPassword)
		if u.Role != "student" || !u.MustChangePassword {
			t.Fatal("registration identity")
		}
		var actual string
		s.DB.QueryRow("SELECT code FROM classes WHERE id=?", u.ClassID).Scan(&actual)
		if actual != code {
			t.Fatal("class mismatch")
		}
		_, _, _ = request(t, server, "GET", "/api/me", nil, c, "", "")
	}
	for _, v := range []map[string]string{{"username": "registered_A", "class_code": "A"}, {"username": "new_teacher", "class_code": "A", "role": "teacher"}, {"username": "new_class", "class_code": "A", "class_id": "2"}, {"username": "badclass", "class_code": "C"}, {"username": "!", "class_code": "A"}} {
		status, _, _ := accountRequest(t, server, "/api/register", v, nil, "")
		if status != 400 && status != 409 {
			t.Fatal("invalid registration", status)
		}
	}
	c, u := passwordLogin(t, server, "registered_A", demoInitialPassword)
	payload := map[string]string{"new_password": "FirstGood987", "confirm_password": "FirstGood987"}
	status, _, _ := accountRequest(t, server, "/api/password", payload, nil, u.CSRF)
	if status != 401 {
		t.Fatal("anonymous password")
	}
	status, _, _ = accountRequest(t, server, "/api/password", payload, c, "")
	if status != 403 {
		t.Fatal("csrf")
	}
	for _, v := range []map[string]string{{"new_password": "123456", "confirm_password": "123456"}, {"new_password": "abcdefgh", "confirm_password": "abcdefgh"}, {"new_password": "Goodpass1", "confirm_password": "Different2"}, {"new_password": strings.Repeat("A", 72) + "1", "confirm_password": strings.Repeat("A", 72) + "1"}, {"new_password": "FirstGood987", "confirm_password": "FirstGood987", "role": "teacher"}, {"new_password": "FirstGood987", "confirm_password": "FirstGood987", "class_id": "2"}} {
		status, _, _ := accountRequest(t, server, "/api/password", v, c, u.CSRF)
		if status != 400 {
			t.Fatal("invalid password accepted", status)
		}
	}
	status, _, _ = accountRequest(t, server, "/api/password", payload, c, u.CSRF)
	if status != 200 {
		t.Fatal(status)
	}
	first, normal := passwordLogin(t, server, "registered_A", "FirstGood987")
	second, _ := passwordLogin(t, server, "registered_A", "FirstGood987")
	for _, current := range []string{"", "wrong"} {
		status, _, _ := accountRequest(t, server, "/api/password", map[string]string{"current_password": current, "new_password": "SecondGood654", "confirm_password": "SecondGood654"}, first, normal.CSRF)
		if status != 400 {
			t.Fatal("current required")
		}
	}
	status, _, _ = accountRequest(t, server, "/api/password", map[string]string{"current_password": "FirstGood987", "new_password": "FirstGood987", "confirm_password": "FirstGood987"}, first, normal.CSRF)
	if status != 400 {
		t.Fatal("reuse current")
	}
	status, _, _ = accountRequest(t, server, "/api/password", map[string]string{"current_password": "FirstGood987", "new_password": "SecondGood654", "confirm_password": "SecondGood654"}, first, normal.CSRF)
	if status != 200 {
		t.Fatal(status)
	}
	for _, old := range []*http.Cookie{c, first, second} {
		status, _, _ = request(t, server, "GET", "/api/me", nil, old, "", "")
		if status != 401 {
			t.Fatal("all sessions not revoked")
		}
	}
	status, _, _ = accountRequest(t, server, "/api/login", map[string]string{"username": "registered_A", "password": "FirstGood987"}, nil, "")
	if status != 401 {
		t.Fatal("old password accepted")
	}
	fresh, who := passwordLogin(t, server, "registered_A", "SecondGood654")
	if who.Role != "student" || who.ClassID != u.ClassID {
		t.Fatal("identity changed")
	}
	status, _ = uploadRequest(t, server, fresh, who.CSRF, "a.txt", []byte("test"), nil)
	if status != 403 {
		t.Fatal("student promoted")
	}
}
func TestProductionAndIncrementalMigration(t *testing.T) {
	s, server := testServer(t)
	before := snapshot(t, s)
	if _, e := s.DB.Exec("ALTER TABLE users DROP COLUMN must_change_password"); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		if e := Migrate(context.Background(), s.DB); e != nil {
			t.Fatal(e)
		}
	}
	if before != snapshot(t, s) {
		t.Fatal("migration touched materials")
	}
	loginAs(t, s, server, "teacher_a")
	code, _, _ := accountRequest(t, server, "/api/register", map[string]string{"username": "blocked", "class_code": "A"}, nil, "")
	if code != 403 {
		t.Fatal("production registered")
	}
	if e := s.ResetDemoPasswords(context.Background(), "127.0.0.1:3306"); e == nil {
		t.Fatal("production reset")
	}
	t.Setenv("SESSION_KEY", strings.Repeat("x", 32))
	t.Setenv("APP_ENV", "production")
	t.Setenv("DEMO_MODE", "true")
	if _, e := ConfigFromEnv(); e == nil {
		t.Fatal("production demo enabled")
	}
	t.Setenv("APP_ENV", "local")
	c, e := ConfigFromEnv()
	if e != nil || !c.DemoMode {
		t.Fatal("local config")
	}
}
func TestConcurrentLoginReset(t *testing.T) {
	s, server := testServer(t)
	s.Config.DemoMode = true
	verified, resume := make(chan struct{}), make(chan struct{})
	s.fail = func(stage string) error {
		if stage == "login-verified" {
			close(verified)
			<-resume
		}
		return nil
	}
	data, _ := json.Marshal(map[string]string{"username": "teacher_a", "password": s.Config.SeedPasswords["teacher_a"]})
	type result struct {
		cookie *http.Cookie
		code   int
	}
	ch := make(chan result, 1)
	go func() {
		resp, e := http.Post(server.URL+"/api/login", "application/json", bytes.NewReader(data))
		if e != nil {
			ch <- result{code: 0}
			return
		}
		defer resp.Body.Close()
		v := result{code: resp.StatusCode}
		if len(resp.Cookies()) > 0 {
			v.cookie = resp.Cookies()[0]
		}
		ch <- v
	}()
	<-verified // The old password was valid before the explicit reset commits.
	if e := s.ResetDemoPasswords(context.Background(), "127.0.0.1:3306"); e != nil {
		close(resume)
		t.Fatal(e)
	}
	close(resume)
	outcome := <-ch
	if outcome.code != 401 {
		t.Fatal("concurrent login failed", outcome.code)
	}
	if outcome.cookie != nil {
		code, _, _ := request(t, server, "GET", "/api/me", nil, outcome.cookie, "", "")
		if code != 401 {
			t.Fatal("old credential session survived")
		}
	}
}

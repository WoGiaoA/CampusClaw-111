package campus

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Tests create only uniquely named campusclaw_test_* databases. No production
// database or upload directory is reused. Missing MySQL is reported as SKIP.
func testServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	address := os.Getenv("MYSQL_TEST_ADDR")
	if address == "" {
		t.Skip("MYSQL_TEST_ADDR not supplied: real MySQL integration pending")
	}
	cfg := mysql.NewConfig()
	cfg.User = "root"
	cfg.Passwd = os.Getenv("MYSQL_ROOT_PASSWORD")
	cfg.Net = "tcp"
	cfg.Addr = address
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	admin, e := sql.Open("mysql", cfg.FormatDSN())
	if e != nil {
		t.Fatal(e)
	}
	if e = admin.Ping(); e != nil {
		t.Fatal("test MySQL unavailable")
	}
	name := fmt.Sprintf("campusclaw_test_%d", time.Now().UnixNano())
	if _, e = admin.Exec("CREATE DATABASE " + name + " CHARACTER SET utf8mb4"); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if strings.HasPrefix(name, "campusclaw_test_") {
			_, _ = admin.Exec("DROP DATABASE " + name)
		}
		admin.Close()
	})
	cfg.DBName = name
	db, e := sql.Open("mysql", cfg.FormatDSN())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	if e = Migrate(context.Background(), db); e != nil {
		t.Fatal(e)
	}
	c := Config{SessionKey: strings.Repeat("test-key-", 5), UploadDir: t.TempDir(), SeedPasswords: map[string]string{"teacher_a": "isolation-test-a", "teacher_b": "isolation-test-b", "student_a1": "isolation-test-a1", "student_b1": "isolation-test-b1"}}
	s, e := New(db, c)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Seed(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e = s.Reconcile(context.Background()); e != nil {
		t.Fatal(e)
	}
	server := httptest.NewServer(s.Handler())
	t.Cleanup(server.Close)
	return s, server
}
func request(t *testing.T, server *httptest.Server, method, path string, body io.Reader, cookie *http.Cookie, csrf, contentType string) (int, http.Header, []byte) {
	t.Helper()
	r, e := http.NewRequest(method, server.URL+path, body)
	if e != nil {
		t.Fatal(e)
	}
	if cookie != nil {
		r.AddCookie(cookie)
	}
	if csrf != "" {
		r.Header.Set("X-CSRF-Token", csrf)
	}
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	response, e := http.DefaultClient.Do(r)
	if e != nil {
		t.Fatal(e)
	}
	defer response.Body.Close()
	data, e := io.ReadAll(response.Body)
	if e != nil {
		t.Fatal(e)
	}
	return response.StatusCode, response.Header, data
}
func loginAs(t *testing.T, s *Server, server *httptest.Server, name string) (*http.Cookie, User) {
	t.Helper()
	data, _ := json.Marshal(map[string]string{"username": name, "password": s.Config.SeedPasswords[name]})
	status, headers, body := request(t, server, "POST", "/api/login", bytes.NewReader(data), nil, "", "application/json")
	if status != 200 {
		t.Fatalf("login %s: %d %s", name, status, body)
	}
	response := http.Response{Header: headers}
	cookies := response.Cookies()
	if len(cookies) != 1 {
		t.Fatal("missing cookie")
	}
	status, _, body = request(t, server, "GET", "/api/me", nil, cookies[0], "", "")
	if status != 200 {
		t.Fatal(status)
	}
	var u User
	if e := json.Unmarshal(body, &u); e != nil {
		t.Fatal(e)
	}
	return cookies[0], u
}
func uploadRequest(t *testing.T, server *httptest.Server, c *http.Cookie, csrf, name string, data []byte, extras map[string]string) (int, []byte) {
	t.Helper()
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	if name != "" {
		part, e := writer.CreateFormFile("file", name)
		if e != nil {
			t.Fatal(e)
		}
		_, _ = part.Write(data)
	}
	for key, value := range extras {
		_ = writer.WriteField(key, value)
	}
	writer.Close()
	status, _, body := request(t, server, "POST", "/api/materials/upload", &buffer, c, csrf, writer.FormDataContentType())
	return status, body
}
func materialList(t *testing.T, server *httptest.Server, c *http.Cookie) []Material {
	t.Helper()
	status, _, body := request(t, server, "GET", "/api/materials", nil, c, "", "")
	if status != 200 {
		t.Fatal(status)
	}
	var m []Material
	if e := json.Unmarshal(body, &m); e != nil {
		t.Fatal(e)
	}
	return m
}
func snapshot(t *testing.T, s *Server) string {
	t.Helper()
	var a, b int
	s.DB.QueryRow("SELECT COUNT(*) FROM materials").Scan(&a)
	s.DB.QueryRow("SELECT COUNT(*) FROM knowledge_entries").Scan(&b)
	var paths []string
	e := filepath.WalkDir(s.Config.UploadDir, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() {
			rel, _ := filepath.Rel(s.Config.UploadDir, p)
			data, _ := os.ReadFile(p)
			paths = append(paths, rel+":"+string(data))
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	return fmt.Sprintf("%d/%d/%v", a, b, paths)
}
func TestMySQLSchemaSeeds(t *testing.T) {
	s, _ := testServer(t)
	var tables int
	s.DB.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE()").Scan(&tables)
	if tables != 9 {
		t.Fatal(tables)
	}
	var hash string
	s.DB.QueryRow("SELECT password_hash FROM users WHERE username='teacher_a'").Scan(&hash)
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(s.Config.SeedPasswords["teacher_a"])) != nil {
		t.Fatal("hash verify")
	}
	independent, err := bcrypt.GenerateFromPassword([]byte(s.Config.SeedPasswords["teacher_a"]), bcrypt.DefaultCost)
	if err != nil || string(independent) == hash {
		t.Fatal("random salt missing")
	}
	before := snapshot(t, s)
	if e := s.Seed(context.Background()); e != nil {
		t.Fatal(e)
	}
	if before != snapshot(t, s) {
		t.Fatal("seed duplicated")
	}
	var after string
	s.DB.QueryRow("SELECT password_hash FROM users WHERE username='teacher_a'").Scan(&after)
	if hash != after {
		t.Fatal("hash reset")
	}
	statements := []string{"INSERT INTO users(username,password_hash,role,class_id) VALUES('bad','hash','admin',1)", "INSERT INTO users(username,password_hash,role,class_id) VALUES('teacher_a','hash','teacher',1)", "INSERT INTO knowledge_entries(material_id,class_id,body_text) VALUES(1,2,'wrong')", "INSERT INTO users(username,password_hash,role,class_id) VALUES('badclass','hash','student',99999)"}
	for _, statement := range statements {
		if _, e := s.DB.Exec(statement); e == nil {
			t.Fatal("invalid constraint accepted")
		}
	}
	var indexes int
	s.DB.QueryRow("SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND column_name='class_id' AND table_name IN ('materials','knowledge_entries')").Scan(&indexes)
	if indexes < 2 {
		t.Fatal("missing tenant indexes")
	}
	t.Log("nine tables, real MySQL constraints, salted hashes, seed idempotence: PASS")
}
func TestMySQLAuthSessions(t *testing.T) {
	s, server := testServer(t)
	for _, name := range []string{"teacher_a", "teacher_b", "student_a1", "student_b1"} {
		c, u := loginAs(t, s, server, name)
		if strings.Contains(c.Value, "teacher") || len(c.Value) != 64 {
			t.Fatal("identity cookie")
		}
		var stored string
		if e := s.DB.QueryRow("SELECT id_hash FROM sessions WHERE user_id=? LIMIT 1", u.ID).Scan(&stored); e != nil || stored == c.Value {
			t.Fatal("raw ID stored")
		}
		if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" {
			t.Fatal("attributes")
		}
	}
	var failure []byte
	for i := 0; i < 12; i++ {
		name := "teacher_a"
		if i == 1 {
			name = "missing-account"
		}
		data, _ := json.Marshal(map[string]string{"username": name, "password": "incorrect"})
		status, h, body := request(t, server, "POST", "/api/login", bytes.NewReader(data), nil, "", "application/json")
		if status != 401 || h.Get("Set-Cookie") != "" {
			t.Fatal(status)
		}
		if i == 0 {
			failure = body
		} else if !bytes.Equal(body, failure) {
			t.Fatal("failure differs")
		}
	}
	// Correct password is still rejected once this IP is rate limited.
	data, _ := json.Marshal(map[string]string{"username": "teacher_a", "password": s.Config.SeedPasswords["teacher_a"]})
	status, _, body := request(t, server, "POST", "/api/login", bytes.NewReader(data), nil, "", "application/json")
	if status != 401 || !bytes.Equal(body, failure) {
		t.Fatal("limit differs")
	}
	s.mu.Lock()
	s.failures = map[string]attempts{}
	s.mu.Unlock()
	old, u := loginAs(t, s, server, "teacher_a")
	status, headers, _ := request(t, server, "POST", "/api/login", bytes.NewReader(data), old, "", "application/json")
	if status != 200 {
		t.Fatal(status)
	}
	response := http.Response{Header: headers}
	fresh := response.Cookies()[0]
	status, _, _ = request(t, server, "GET", "/api/me", nil, old, "", "")
	if status != 401 {
		t.Fatal("old session survived rotation")
	}
	status, _, body = request(t, server, "GET", "/api/me", nil, fresh, "", "")
	json.Unmarshal(body, &u)
	status, _, _ = request(t, server, "POST", "/api/logout", nil, fresh, "wrong", "")
	if status != 403 {
		t.Fatal(status)
	}
	status, _, _ = request(t, server, "POST", "/api/logout", nil, fresh, u.CSRF, "")
	if status != 200 {
		t.Fatal(status)
	}
	status, _, _ = request(t, server, "GET", "/api/me", nil, fresh, "", "")
	if status != 401 {
		t.Fatal("logout replay")
	}
	c, u := loginAs(t, s, server, "teacher_a")
	_, _ = s.DB.Exec("UPDATE users SET role='student' WHERE id=?", u.ID)
	status, _, body = request(t, server, "GET", "/api/me", nil, c, "", "")
	json.Unmarshal(body, &u)
	if status != 200 || u.Role != "student" {
		t.Fatal("stale role")
	}
	_, _ = s.DB.Exec("UPDATE sessions SET expires_at=? WHERE id_hash=?", time.Now().Add(-time.Hour), s.sessionHash(c.Value))
	status, _, _ = request(t, server, "GET", "/api/me", nil, c, "", "")
	if status != 401 {
		t.Fatal("expired")
	}
	for _, path := range []string{"/api/me", "/api/materials", "/api/materials/1", "/api/materials/1/file", "/api/knowledge-entries", "/api/knowledge-entries/1"} {
		status, _, body = request(t, server, "GET", path, nil, nil, "", "")
		if status != 401 || bytes.Contains(body, []byte("CLASS_")) {
			t.Fatal(path)
		}
	}
	forged := &http.Cookie{Name: cookieName, Value: strings.Repeat("0", 64)}
	status, _, _ = request(t, server, "GET", "/api/me", nil, forged, "", "")
	if status != 401 {
		t.Fatal("forged cookie")
	}
	t.Log("four logins, uniform failures/rate limit, rotation/revocation/expiry, DB roles, anonymous denial: PASS")
}
func TestMySQLIsolationDownload(t *testing.T) {
	s, server := testServer(t)
	for _, name := range []string{"teacher_a", "student_a1", "teacher_b", "student_b1"} {
		c, u := loginAs(t, s, server, name)
		own := materialList(t, server, c)
		if len(own) != 1 || own[0].ClassID != u.ClassID {
			t.Fatal("class list")
		}
		other := int64(1)
		if own[0].ID == 1 {
			other = 2
		}
		for _, suffix := range []string{"", "/file"} {
			status, _, _ := request(t, server, "GET", fmt.Sprintf("/api/materials/%d%s", own[0].ID, suffix), nil, c, "", "")
			if status != 200 {
				t.Fatal("own read")
			}
			status, _, cross := request(t, server, "GET", fmt.Sprintf("/api/materials/%d%s", other, suffix), nil, c, "", "")
			missing, _, no := request(t, server, "GET", fmt.Sprintf("/api/materials/999999%s", suffix), nil, c, "", "")
			if status != 404 || missing != 404 || !bytes.Equal(cross, no) {
				t.Fatal("cross/missing differ")
			}
		}
		status, _, body := request(t, server, "GET", "/api/materials?class_id=999&role=teacher", nil, c, "", "")
		if status != 200 || bytes.Contains(body, []byte(fmt.Sprintf(`"class_id":%d`, other))) {
			t.Fatal("query spoof")
		}
		status, _, body = request(t, server, "GET", fmt.Sprintf("/api/knowledge-entries/%d", other), nil, c, "", "")
		if status != 404 {
			t.Fatal("knowledge cross")
		}
		status, _, body = request(t, server, "GET", fmt.Sprintf("/api/materials/%d/file", own[0].ID), nil, c, "", "")
		if status != 200 || !bytes.Contains(body, []byte("CLASS_")) {
			t.Fatal("download bytes")
		}
	}
	cookie, _ := loginAs(t, s, server, "student_a1")
	for _, id := range []string{"invalid", "0", "-1"} {
		status, _, body := request(t, server, "GET", "/api/materials/"+id+"/file", nil, cookie, "", "")
		if status != 404 || !bytes.Contains(body, []byte("资源不存在")) {
			t.Fatal("invalid download ID did not match missing-resource response")
		}
	}
	status, _, _ := request(t, server, "GET", "/uploads/1/file.txt", nil, nil, "", "")
	if status != 404 {
		t.Fatal("public upload")
	}
	c, u := loginAs(t, s, server, "student_a1")
	_, _ = s.DB.Exec("INSERT INTO classes(code,name) VALUES('EMPTY','Empty class')")
	_, _ = s.DB.Exec("UPDATE users SET class_id=(SELECT id FROM classes WHERE code='EMPTY') WHERE id=?", u.ID)
	status, _, body := request(t, server, "GET", "/api/materials", nil, c, "", "")
	if status != 200 || strings.TrimSpace(string(body)) != "[]" {
		t.Fatal("empty list")
	}
	t.Log("A/B teacher/student lists and download, cross/missing 404, anonymous 401, no public files: PASS")
}
func TestMySQLUploadRollbackRecovery(t *testing.T) {
	s, server := testServer(t)
	c, u := loginAs(t, s, server, "teacher_a")
	student, st := loginAs(t, s, server, "student_a1")
	before := snapshot(t, s)
	status, _ := uploadRequest(t, server, student, st.CSRF, "a.txt", []byte("valid"), map[string]string{"role": "teacher", "class_id": "2"})
	if status != 403 || before != snapshot(t, s) {
		t.Fatal("student wrote")
	}
	status, _ = uploadRequest(t, server, nil, "", "a.txt", []byte("valid"), nil)
	if status != 401 {
		t.Fatal("anonymous upload")
	}
	status, _ = uploadRequest(t, server, c, "wrong", "a.txt", []byte("valid"), nil)
	if status != 403 {
		t.Fatal("csrf")
	}
	for _, tc := range []struct {
		name string
		data []byte
		want int
	}{{"", nil, 400}, {"a.exe", []byte("x"), 400}, {"a.pdf", []byte("x"), 400}, {"a.docx", []byte("x"), 400}, {"a.png", []byte("x"), 400}, {"a.txt", nil, 400}, {"a.md", []byte(" \n"), 400}, {"a.txt", []byte{255}, 400}, {"a.txt", []byte(strings.Repeat("x", maxFileSize+1)), 413}} {
		status, _ = uploadRequest(t, server, c, u.CSRF, tc.name, tc.data, nil)
		if status != tc.want || before != snapshot(t, s) {
			t.Fatalf("invalid %s %d", tc.name, status)
		}
	}
	for _, stage := range []string{"file-write", "knowledge-insert", "commit"} {
		s.fail = func(input string) error {
			if input == stage {
				return errors.New("injected failure")
			}
			return nil
		}
		status, _ = uploadRequest(t, server, c, u.CSRF, "a.txt", []byte("valid"), nil)
		if status != 500 || before != snapshot(t, s) {
			t.Fatalf("residue at %s", stage)
		}
	}
	s.fail = nil
	var ids []int64
	for _, name := range []string{"same.txt", "same.txt", "body.md"} {
		status, body := uploadRequest(t, server, c, u.CSRF, name, []byte("\ufeff# 新讲义\n"), map[string]string{"class_id": "2", "title": "本班新讲义"})
		if status != 201 {
			t.Fatalf("upload %d %s", status, body)
		}
		var result map[string]int64
		json.Unmarshal(body, &result)
		ids = append(ids, result["material_id"])
		var class int64
		var text string
		s.DB.QueryRow("SELECT class_id,body_text FROM knowledge_entries WHERE id=?", result["knowledge_entry_id"]).Scan(&class, &text)
		if class != u.ClassID || text != "# 新讲义\n" {
			t.Fatal("wrong class/text")
		}
		status, _, raw := request(t, server, "GET", fmt.Sprintf("/api/materials/%d/file", result["material_id"]), nil, c, "", "")
		if status != 200 || !bytes.Equal(raw, []byte("\ufeff# 新讲义\n")) {
			t.Fatal("original changed")
		}
	}
	if ids[0] == ids[1] {
		t.Fatal("overwrite")
	}
	if len(materialList(t, server, student)) != 4 {
		t.Fatal("student visibility")
	}
	before = snapshot(t, s)
	if e := s.Seed(context.Background()); e != nil || before != snapshot(t, s) {
		t.Fatal("reseed lost upload")
	}
	_, orphan, e := s.saveFile(u.ClassID, ".txt", []byte("interrupted before commit"))
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Reconcile(context.Background()); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(orphan); !os.IsNotExist(e) {
		t.Fatal("orphan survives")
	}
	if before != snapshot(t, s) {
		t.Fatal("committed removed")
	}
	var key string
	s.DB.QueryRow("SELECT storage_key FROM materials WHERE id=?", ids[0]).Scan(&key)
	path, _ := s.storagePath(key)
	data, _ := os.ReadFile(path)
	os.Remove(path)
	if e = s.Reconcile(context.Background()); e == nil {
		t.Fatal("missing file accepted")
	}
	os.WriteFile(path, data, 0600)
	// Simulate a real write-permission failure by directing uploads at a file.
	original := s.Config.UploadDir
	s.Config.UploadDir = filepath.Join(t.TempDir(), "not-directory")
	os.WriteFile(s.Config.UploadDir, []byte("file"), 0600)
	status, _ = uploadRequest(t, server, c, u.CSRF, "a.txt", []byte("valid"), nil)
	if status != 500 {
		t.Fatal("file failure")
	}
	s.Config.UploadDir = original
	t.Log("txt/md 201, UTF8/BOM, same names, 400/413, 403 unchanged, file/DB/commit rollback, startup recovery: PASS")
}
func TestMySQLHealthAndMissingSeed(t *testing.T) {
	s, server := testServer(t)
	// Only this test's disposable database is emptied, never the running app.
	for _, table := range []string{"knowledge_entries", "materials", "users", "classes"} {
		if _, err := s.DB.Exec("DELETE FROM " + table); err != nil {
			t.Fatal(err)
		}
	}
	password := s.Config.SeedPasswords["teacher_b"]
	s.Config.SeedPasswords["teacher_b"] = ""
	if err := s.Seed(context.Background()); err == nil {
		t.Fatal("missing seed password accepted")
	}
	var partial int
	s.DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&partial)
	if partial != 0 {
		t.Fatal("partial seed users")
	}
	s.Config.SeedPasswords["teacher_b"] = password
	if err := s.Seed(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	status, _, body := request(t, server, "GET", "/health", nil, nil, "", "")
	if status != 200 || strings.TrimSpace(string(body)) != `{"status":"ok"}` {
		t.Fatal("health")
	}
	original := s.Config.UploadDir
	s.Config.UploadDir = filepath.Join(t.TempDir(), "missing")
	status, _, body = request(t, server, "GET", "/health", nil, nil, "", "")
	if status != 503 || bytes.Contains(body, []byte("missing")) {
		t.Fatal("storage health")
	}
	s.Config.UploadDir = original
	s.DB.Close()
	status, _, _ = request(t, server, "GET", "/health", nil, nil, "", "")
	if status != 503 {
		t.Fatal("DB health")
	}
	t.Log("missing seed secret leaves no users; health unauthenticated 200, actual DB/storage fault 503: PASS")
}

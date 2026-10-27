package campus

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
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

// TestFinalCourseworkEvidence uses only testServer's unique test DB and TempDir.
// Its optional evidence contains redacted HTTP responses, never reusable credentials.
func TestFinalCourseworkEvidence(t *testing.T) {
	s, server := testServer(t)
	checks := []map[string]any{}
	var redact func(any) any
	redact = func(value any) any {
		switch v := value.(type) {
		case map[string]any:
			out := map[string]any{}
			for k, x := range v {
				if k == "csrf_token" || k == "password" || k == "password_hash" {
					out[k] = "[REDACTED]"
				} else {
					out[k] = redact(x)
				}
			}
			return out
		case []any:
			out := make([]any, len(v))
			for i, x := range v {
				out[i] = redact(x)
			}
			return out
		default:
			return value
		}
	}
	call := func(name, method, path string, body io.Reader, cookie *http.Cookie, csrf, contentType string, extra map[string]string) (int, http.Header, []byte) {
		t.Helper()
		req, err := http.NewRequest(method, server.URL+path, body)
		if err != nil {
			t.Fatal(err)
		}
		if cookie != nil {
			req.AddCookie(cookie)
		}
		if csrf != "" {
			req.Header.Set("X-CSRF-Token", csrf)
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		for k, v := range extra {
			req.Header.Set(k, v)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		headers := map[string]string{}
		for k, values := range resp.Header {
			headers[k] = strings.Join(values, ", ")
		}
		if cs := resp.Cookies(); len(cs) > 0 {
			headers["Set-Cookie"] = strings.ReplaceAll(resp.Header.Get("Set-Cookie"), cs[0].Value, "[REDACTED]")
		}
		var recorded any
		if strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
			var v any
			if json.Unmarshal(raw, &v) != nil {
				t.Fatal("invalid JSON response")
			}
			recorded = redact(v)
		} else {
			recorded = map[string]any{"bytes": len(raw), "sha256": fmt.Sprintf("%x", sha256.Sum256(raw))}
		}
		requestHeaders := map[string]string{}
		if contentType != "" {
			requestHeaders["Content-Type"] = contentType
		}
		if cookie != nil {
			requestHeaders["Cookie"] = "campusclaw_session=[REDACTED]"
		}
		if csrf != "" {
			requestHeaders["X-CSRF-Token"] = "[REDACTED]"
		}
		checks = append(checks, map[string]any{"name": name, "method": method, "path": path, "request_headers": requestHeaders, "status": resp.StatusCode, "response_headers": headers, "body": recorded})
		return resp.StatusCode, resp.Header, raw
	}
	expect := func(name string, got, want int) {
		t.Helper()
		if got != want {
			t.Fatalf("%s: status %d, expected %d", name, got, want)
		}
	}
	type actor struct {
		Cookie *http.Cookie
		User   User
	}
	actors := map[string]actor{}
	for _, name := range []string{"teacher_a", "teacher_b", "student_a1", "student_b1"} {
		input, _ := json.Marshal(map[string]string{"username": name, "password": s.Config.SeedPasswords[name]})
		code, headers, _ := call("login:"+name, "POST", "/api/login", bytes.NewReader(input), nil, "", "application/json", nil)
		expect("login", code, 200)
		checks[len(checks)-1]["request_payload"] = map[string]string{"username": name, "password": "[REDACTED]"}
		cookies := (&http.Response{Header: headers}).Cookies()
		if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode {
			t.Fatal("session cookie attributes invalid")
		}
		code, _, raw := call("identity:"+name, "GET", "/api/me", nil, cookies[0], "", "", nil)
		expect("identity", code, 200)
		var user User
		if json.Unmarshal(raw, &user) != nil {
			t.Fatal("identity JSON")
		}
		actors[name] = actor{cookies[0], user}
		var digest string
		if s.DB.QueryRow("SELECT id_hash FROM sessions WHERE user_id=?", user.ID).Scan(&digest) != nil || digest == cookies[0].Value || digest != s.sessionHash(cookies[0].Value) {
			t.Fatal("session digest storage mismatch")
		}
	}
	ta, tb, sa, sb := actors["teacher_a"], actors["teacher_b"], actors["student_a1"], actors["student_b1"]
	for _, path := range []string{"/api/me", "/api/materials", "/api/materials/1", "/api/materials/1/file", "/api/knowledge-entries", "/api/knowledge-entries/1"} {
		code, _, _ := call("anonymous:"+path, "GET", path, nil, nil, "", "", nil)
		expect("anonymous", code, 401)
	}
	code, _, _ := call("anonymous:upload", "POST", "/api/materials/upload", nil, nil, "", "", nil)
	expect("anonymous upload", code, 401)
	var wrongBody []byte
	for _, name := range []string{"teacher_a", "not_an_account"} {
		input, _ := json.Marshal(map[string]string{"username": name, "password": "incorrect-fixture-password"})
		code, _, raw := call("login-failure:"+name, "POST", "/api/login", bytes.NewReader(input), nil, "", "application/json", nil)
		expect("login failure", code, 401)
		if wrongBody != nil && !bytes.Equal(wrongBody, raw) {
			t.Fatal("login failure responses differ")
		}
		wrongBody = raw
	}
	ids := map[int64]int64{}
	for _, a := range []actor{ta, tb, sa, sb} {
		code, _, raw := call("own-class-list:"+a.User.Username, "GET", "/api/materials?class_id=999&role=teacher", nil, a.Cookie, "", "", nil)
		expect("list", code, 200)
		var rows []Material
		json.Unmarshal(raw, &rows)
		if len(rows) == 0 {
			t.Fatal("missing class fixture")
		}
		for _, m := range rows {
			if m.ClassID != a.User.ClassID {
				t.Fatal("list class leak")
			}
		}
		ids[a.User.ClassID] = rows[0].ID
	}
	code, _, raw := call("forged-personal-identity", "GET", fmt.Sprintf("/api/me?user_id=%d&role=teacher&class_id=%d", tb.User.ID, tb.User.ClassID), nil, sa.Cookie, "", "", map[string]string{"X-User-ID": fmt.Sprint(tb.User.ID), "X-Role": "teacher", "X-Class-ID": fmt.Sprint(tb.User.ClassID)})
	expect("forged identity", code, 200)
	var actual User
	json.Unmarshal(raw, &actual)
	if actual.ID != sa.User.ID || actual.Role != "student" || actual.ClassID != sa.User.ClassID {
		t.Fatal("client identity trusted")
	}
	for _, a := range []actor{ta, tb, sa, sb} {
		own := ids[a.User.ClassID]
		other := ids[ta.User.ClassID]
		if a.User.ClassID == ta.User.ClassID {
			other = ids[tb.User.ClassID]
		}
		code, _, raw = call("same-class-detail:"+a.User.Username, "GET", fmt.Sprintf("/api/materials/%d", own), nil, a.Cookie, "", "", nil)
		expect("same class detail", code, 200)
		var detail Material
		json.Unmarshal(raw, &detail)
		code, headers, download := call("same-class-download:"+a.User.Username, "GET", fmt.Sprintf("/api/materials/%d/file", own), nil, a.Cookie, "", "", nil)
		expect("same class download", code, 200)
		if !strings.HasPrefix(headers.Get("Content-Disposition"), "attachment") || string(download) != detail.Body {
			t.Fatal("download bytes or disposition mismatch")
		}
		for _, suffix := range []string{"", "/file"} {
			code, _, cross := call("cross-class:"+a.User.Username, "GET", fmt.Sprintf("/api/materials/%d%s", other, suffix), nil, a.Cookie, "", "", nil)
			expect("cross class", code, 404)
			code, _, missing := call("missing-id:"+a.User.Username, "GET", "/api/materials/999999999"+suffix, nil, a.Cookie, "", "", nil)
			expect("missing", code, 404)
			if !bytes.Equal(cross, missing) {
				t.Fatal("cross and missing differ")
			}
		}
		code, _, _ = call("own-knowledge:"+a.User.Username, "GET", fmt.Sprintf("/api/knowledge-entries/%d", detail.KnowledgeID), nil, a.Cookie, "", "", nil)
		expect("own knowledge", code, 200)
	}
	upload := func(name string, a actor, csrf, filename string, data []byte, extras map[string]string) (int, []byte) {
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			t.Fatal(err)
		}
		part.Write(data)
		for k, v := range extras {
			writer.WriteField(k, v)
		}
		writer.Close()
		code, _, raw := call(name, "POST", "/api/materials/upload", &buf, a.Cookie, csrf, writer.FormDataContentType(), nil)
		checks[len(checks)-1]["request_payload"] = map[string]any{"filename": filename, "file_bytes": len(data), "fields": extras}
		return code, raw
	}
	before := snapshot(t, s)
	code, _ = upload("student-forged-role", sa, sa.User.CSRF, "denied.txt", []byte("denied"), map[string]string{"role": "teacher", "class_id": fmt.Sprint(tb.User.ClassID)})
	expect("student upload", code, 403)
	code, _ = upload("teacher-missing-csrf", ta, "", "denied.txt", []byte("denied"), nil)
	expect("csrf", code, 403)
	for _, bad := range []struct {
		name string
		data []byte
		want int
	}{{"bad.pdf", []byte("pdf"), 400}, {"bad.docx", []byte("docx"), 400}, {"bad.png", []byte("png"), 400}, {"empty.txt", nil, 400}, {"bad.txt", []byte{255}, 400}, {"large.txt", bytes.Repeat([]byte("x"), maxFileSize+1), 413}} {
		code, _ = upload("rejected:"+bad.name, ta, ta.User.CSRF, bad.name, bad.data, nil)
		expect("invalid upload", code, bad.want)
	}
	if snapshot(t, s) != before {
		t.Fatal("rejected uploads changed DB/files")
	}
	var uploaded map[string]int64
	for _, name := range []string{"lecture.txt", "lecture.md"} {
		original := []byte("\ufeff课堂最终验收材料\n")
		code, raw = upload("teacher-upload:"+name, ta, ta.User.CSRF, name, original, map[string]string{"class_id": fmt.Sprint(tb.User.ClassID), "title": "最终验收资料"})
		expect("upload", code, 201)
		json.Unmarshal(raw, &uploaded)
		path := fmt.Sprintf("/api/materials/%d", uploaded["material_id"])
		code, _, raw = call("student-sees-upload", "GET", path, nil, sa.Cookie, "", "", nil)
		expect("shared upload", code, 200)
		var m Material
		json.Unmarshal(raw, &m)
		if m.ClassID != ta.User.ClassID || m.Body != "课堂最终验收材料\n" {
			t.Fatal("uploaded class/body mismatch")
		}
		code, _, download := call("uploaded-original-bytes", "GET", path+"/file", nil, sa.Cookie, "", "", nil)
		expect("uploaded download", code, 200)
		if !bytes.Equal(download, original) {
			t.Fatal("original bytes changed")
		}
		code, _, _ = call("other-class-upload-denied", "GET", path, nil, sb.Cookie, "", "", nil)
		expect("other class upload", code, 404)
		code, _, _ = call("other-class-knowledge-denied", "GET", fmt.Sprintf("/api/knowledge-entries/%d", uploaded["knowledge_entry_id"]), nil, sb.Cookie, "", "", nil)
		expect("other knowledge", code, 404)
	}
	code, _, _ = call("public-storage-denied", "GET", "/uploads/1/guessed.txt", nil, ta.Cookie, "", "", nil)
	expect("public storage", code, 404)
	code, _, _ = call("health-no-cookie", "GET", "/health", nil, nil, "", "", nil)
	expect("health", code, 200)
	// Restart the Go handler against the same test DB and files; this is not a container restart.
	restarted, err := New(s.DB, s.Config)
	if err != nil {
		t.Fatal(err)
	}
	if err = restarted.Seed(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = restarted.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	second := httptest.NewServer(restarted.Handler())
	defer second.Close()
	server = second
	code, _, _ = call("handler-restart-preserves-session", "GET", fmt.Sprintf("/api/materials/%d/file", uploaded["material_id"]), nil, sa.Cookie, "", "", nil)
	expect("restart read", code, 200)
	code, _, _ = call("logout", "POST", "/api/logout", nil, ta.Cookie, ta.User.CSRF, "", nil)
	expect("logout", code, 200)
	code, _, _ = call("logout-replay-denied", "GET", "/api/me", nil, ta.Cookie, "", "", nil)
	expect("logout replay", code, 401)
	if path := os.Getenv("CAMPUSCLAW_EVIDENCE_PATH"); path != "" {
		report := map[string]any{"date": "2026-10-27", "executed_at": time.Now().UTC().Format(time.RFC3339), "transport": "Real Go HTTP server and actual MySQL; unique test DB and temporary files. Nginx checked separately.", "passed": true, "count": len(checks), "checks": checks}
		raw, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("%d final coursework HTTP checks passed; cookies, CSRF and passwords redacted", len(checks))
}

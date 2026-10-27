package campus

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoginRejectsMalformedJSON(t *testing.T) {
	// A nil database makes this also prove malformed requests stop before credential lookup.
	server := &Server{}
	for _, tc := range []struct{ name, media, body string }{
		{"wrong media", "application/jsonp", `{"username":"x","password":"y"}`},
		{"bad media parameter", "application/json; charset", `{}`},
		{"second value", "application/json", `{"username":"x"} {"password":"y"}`},
		{"trailing garbage", "application/json", `{"username":"x"} junk`},
		{"oversized whitespace", "application/json", `{}` + strings.Repeat(" ", 16<<10)},
		{"truncated", "application/json", `{"username":`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/api/login", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", tc.media)
			w := httptest.NewRecorder()
			server.Handler().ServeHTTP(w, r)
			if w.Code != 400 || w.Header().Get("Set-Cookie") != "" {
				t.Fatalf("status=%d; unexpected session", w.Code)
			}
		})
	}
}

func TestAccountJSONMediaType(t *testing.T) {
	for _, tc := range []struct {
		media, body string
		valid       bool
	}{
		{"application/json; charset=utf-8", `{"username":"student"}`, true},
		{"application/jsonp", `{}`, false},
		{"application/json", `{"username":"student"} {}`, false},
		{"application/json", `{"username":"student","role":"teacher"}`, false},
	} {
		r := httptest.NewRequest("POST", "/api/register", strings.NewReader(tc.body))
		r.Header.Set("Content-Type", tc.media)
		w := httptest.NewRecorder()
		var input struct {
			Username string `json:"username"`
		}
		if got := decodeAccountJSON(w, r, &input); got != tc.valid {
			t.Fatalf("media=%s valid=%v", tc.media, got)
		}
	}
}

func TestMySQLLoginJSONParameters(t *testing.T) {
	s, server := testServer(t)
	code, headers, _ := request(t, server, "POST", "/api/login", strings.NewReader(`{"username":"teacher_a","password":"`+s.Config.SeedPasswords["teacher_a"]+`","role":"student","class_id":999}`), nil, "", "application/json; charset=utf-8")
	if code != 200 || headers.Get("Set-Cookie") == "" {
		t.Fatal("valid JSON with charset rejected")
	}
}

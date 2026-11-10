package campus

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNicknameAcrossSessions(t *testing.T) {
	s, server := testServer(t)
	if _, err := s.DB.Exec("ALTER TABLE users DROP COLUMN nickname"); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := Migrate(t.Context(), s.DB); err != nil {
			t.Fatal(err)
		}
	}
	type session struct {
		User
		Token string `json:"access_token"`
	}
	login := func() session {
		t.Helper()
		code, _, body := request(t, server, "POST", "/api/login", strings.NewReader(`{"username":"teacher_a","password":"isolation-test-a"}`), nil, "", "application/json")
		expectStatus(t, 200, code, body)
		var result session
		if err := json.Unmarshal(body, &result); err != nil || result.Token == "" {
			t.Fatal("missing session", err)
		}
		return result
	}
	first, second := login(), login()
	if first.Nickname != nil || first.Token == second.Token {
		t.Fatal("expected an unset nickname and independent sessions")
	}
	for _, tc := range []struct {
		token, csrf, input string
		status             int
	}{
		{"", "", `{"nickname":"张老师"}`, 401},
		{first.Token, "", `{"nickname":"张老师"}`, 403},
		{first.Token, first.CSRF, `{"nickname":"张老师","user_id":2}`, 400},
		{first.Token, first.CSRF, `{"nickname":"a\nb"}`, 400},
		{first.Token, first.CSRF, `{"nickname":"  张老师  "}`, 200},
	} {
		code, body := bearerRequest(t, server, "PATCH", "/api/me", tc.token, tc.csrf, json.RawMessage(tc.input))
		expectStatus(t, tc.status, code, body)
	}
	check := func(want string) {
		t.Helper()
		code, body := bearerRequest(t, server, "GET", "/api/me", second.Token, "", nil)
		expectStatus(t, 200, code, body)
		var user User
		if err := json.Unmarshal(body, &user); err != nil || user.Nickname == nil || *user.Nickname != want {
			t.Fatalf("expected nickname %q, got %s", want, body)
		}
	}
	check("张老师")
	code, body := bearerRequest(t, server, "PATCH", "/api/me", first.Token, first.CSRF, json.RawMessage(`{"nickname":""}`))
	expectStatus(t, 200, code, body)
	check("")
	if _, err := s.DB.Exec("UPDATE users SET must_change_password=TRUE WHERE id=?", first.ID); err != nil {
		t.Fatal(err)
	}
	code, body = bearerRequest(t, server, "PATCH", "/api/me", first.Token, first.CSRF, json.RawMessage(`{"nickname":"不能写入"}`))
	expectStatus(t, 403, code, body)
	check("")
}

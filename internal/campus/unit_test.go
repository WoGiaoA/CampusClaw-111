package campus

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateText(t *testing.T) {
	cases := []struct {
		name  string
		data  []byte
		valid bool
	}{{"a.txt", []byte("hello"), true}, {"a.MD", []byte("\ufeff# 讲义"), true}, {"a.pdf", []byte("hello"), false}, {"a.docx", []byte("hello"), false}, {"a.png", []byte("hello"), false}, {"a.txt", nil, false}, {"a.md", []byte(" \n\t"), false}, {"a.txt", []byte{255}, false}, {"a.txt", []byte(strings.Repeat("x", maxFileSize+1)), false}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, text, e := validateText(tc.name, tc.data)
			if (e == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, e)
			}
			if strings.HasPrefix(text, "\ufeff") {
				t.Fatal("BOM retained")
			}
		})
	}
	_, _, e := validateText("limit.txt", []byte(strings.Repeat("x", maxFileSize)))
	if e != nil {
		t.Fatal("exact boundary rejected")
	}
}
func TestConfigAndSecrets(t *testing.T) {
	t.Setenv("SESSION_KEY", "")
	if _, e := ConfigFromEnv(); e == nil {
		t.Fatal("missing key accepted")
	}
	t.Setenv("SESSION_KEY", strings.Repeat("k", 32))
	c, e := ConfigFromEnv()
	if e != nil {
		t.Fatal(e)
	}
	s, e := New(nil, c)
	if e != nil {
		t.Fatal(e)
	}
	id, e := randomToken()
	if e != nil {
		t.Fatal(e)
	}
	if len(id) != 64 || s.sessionHash(id) == id || s.sessionHash(id) == s.sessionHash(id+"x") {
		t.Fatal("invalid token digest")
	}
	for _, secure := range []bool{false, true} {
		s.Config.SecureCookie = secure
		c := s.cookie(id, 28800)
		if !c.HttpOnly || c.Path != "/" || c.SameSite != http.SameSiteLaxMode || c.Secure != secure {
			t.Fatal("cookie attributes")
		}
	}
}
func TestStorageConfinedAndRollback(t *testing.T) {
	s, e := New(nil, Config{SessionKey: strings.Repeat("k", 32), UploadDir: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	for _, key := range []string{"../escape", "/absolute", "C:\\escape", "a\\..\\escape"} {
		if _, e = s.storagePath(key); e == nil {
			t.Fatalf("accepted %s", key)
		}
	}
	key, p, e := s.saveFile(1, ".txt", []byte("raw bytes"))
	if e != nil {
		t.Fatal(e)
	}
	if !strings.HasPrefix(key, "1/") {
		t.Fatal(key)
	}
	bytes, e := os.ReadFile(p)
	if e != nil || string(bytes) != "raw bytes" {
		t.Fatal("raw bytes not retained")
	}
	s.fail = func(stage string) error {
		if stage == "file-write" {
			return errors.New("injected")
		}
		return nil
	}
	if _, _, e = s.saveFile(1, ".txt", []byte("fail")); e == nil {
		t.Fatal("injection ignored")
	}
	entries, _ := os.ReadDir(filepath.Join(s.Config.UploadDir, ".tmp"))
	if len(entries) != 0 {
		t.Fatal("temp residue")
	}
}
func TestCSRF(t *testing.T) {
	r, _ := http.NewRequest("POST", "/", nil)
	u := User{CSRF: "secret"}
	if validCSRF(r, u) {
		t.Fatal("empty token")
	}
	r.Header.Set("X-CSRF-Token", "wrong")
	if validCSRF(r, u) {
		t.Fatal("wrong token")
	}
	r.Header.Set("X-CSRF-Token", "secret")
	if !validCSRF(r, u) {
		t.Fatal("valid token rejected")
	}
}

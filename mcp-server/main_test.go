package main

import (
	"campusclaw/internal/mcpwire"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMCPHTTP(t *testing.T) {
	s := httptest.NewServer(handler("isolated"))
	defer s.Close()
	get, err := http.Get(s.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	get.Body.Close()
	if get.StatusCode != 200 {
		t.Fatal(get.StatusCode)
	}
	post := func(token, body string, status, code int) string {
		t.Helper()
		req, _ := http.NewRequest("POST", s.URL+"/mcp", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, err := io.ReadAll(res.Body)
		if err != nil || res.StatusCode != status {
			t.Fatalf("status %d: %s (%v)", res.StatusCode, raw, err)
		}
		if status == 200 {
			var r mcpwire.Response
			if json.Unmarshal(raw, &r) != nil || r.Version != "2.0" {
				t.Fatal(string(raw))
			}
			if code != 0 && (r.Error == nil || r.Error.Code != code) {
				t.Fatal(string(raw))
			}
			if code == 0 && (r.Error != nil || r.ID == nil || *r.ID != 8) {
				t.Fatal(string(raw))
			}
		}
		return string(raw)
	}
	const init = `{"jsonrpc":"2.0","id":8,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`
	post("bad", init, 401, 0)
	post("isolated", init, 200, 0)
	post("isolated", `{"jsonrpc":"2.0","method":"notifications/initialized"}`, 202, 0)
	list := post("isolated", `{"jsonrpc":"2.0","id":8,"method":"tools/list"}`, 200, 0)
	if !strings.Contains(list, "formula_hint") || !strings.Contains(list, "strategy_hint") {
		t.Fatal(list)
	}
	first := post("isolated", `{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"formula_hint","arguments":{"topic":"quadratic_equation"}}}`, 200, 0)
	if !strings.Contains(first, "check_discriminant") {
		t.Fatal(first)
	}
	second := post("isolated", `{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"strategy_hint","arguments":{"strategy":"check_discriminant","step":2}}}`, 200, 0)
	if !strings.Contains(second, "符号") {
		t.Fatal(second)
	}
	post("isolated", `{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"strategy_hint","arguments":{"strategy":"check_discriminant","step":5}}}`, 200, -32602)
	post("isolated", `{"jsonrpc":"2.0","id":8,"method":"nope"}`, 200, -32601)
	post("isolated", `{"jsonrpc":"2.0","id":8,"id":9,"method":"tools/list"}`, 200, -32600)
	post("isolated", strings.Repeat(" ", mcpwire.Limit+1), 413, 0)
}

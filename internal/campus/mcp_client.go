package campus

import (
	"bytes"
	"campusclaw/internal/mcpwire"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const mcpBaseURL = "http://mcp-server:8080"

var mcpName = regexp.MustCompile("^[a-z][a-z0-9_]{0,47}$")
var mcpSlug = regexp.MustCompile("^[a-z][a-z0-9-]{0,47}$")
var errMCPConflict = errors.New("mcp state conflict")

func (s *Server) mcpEndpoint() string {
	if s.Config.mcpTestEndpoint != "" {
		return s.Config.mcpTestEndpoint
	}
	return mcpBaseURL
}

func (s *Server) mcpCipher() (cipher.AEAD, error) {
	key, err := base64.StdEncoding.DecodeString(s.Config.MCPCredentialKey)
	if err != nil || len(key) != 32 {
		return nil, dep("mcp_key_unavailable")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func (s *Server) mcpSeal(token string, classID, id int64) ([]byte, error) {
	a, err := s.mcpCipher()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, a.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	return a.Seal(nonce, nonce, []byte(token), []byte(fmt.Sprintf("mcp:%d:%d", classID, id))), nil
}

func (s *Server) mcpOpen(data []byte, classID, id int64) (string, error) {
	a, err := s.mcpCipher()
	if err != nil {
		return "", err
	}
	if len(data) < a.NonceSize() {
		return "", dep("mcp_credential_unavailable")
	}
	clear, err := a.Open(nil, data[:a.NonceSize()], data[a.NonceSize():], []byte(fmt.Sprintf("mcp:%d:%d", classID, id)))
	if err != nil {
		return "", dep("mcp_credential_unavailable")
	}
	return string(clear), nil
}

var mcpHTTP = &http.Client{
	Transport:     &http.Transport{DisableKeepAlives: true, ResponseHeaderTimeout: 4 * time.Second},
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

func (s *Server) mcpRPC(ctx context.Context, base, token, method string, id *int64, params any, out any) error {
	if base != s.mcpEndpoint() || token == "" {
		return dep("mcp_endpoint_or_token")
	}
	in := mcpwire.Request{Version: "2.0", ID: id, Method: method}
	if params != nil {
		in.Params = mcpwire.Raw(params)
	}
	b := mcpwire.Raw(in)
	if len(b) > mcpwire.Limit {
		return dep("mcp_request_limit")
	}
	req, err := http.NewRequestWithContext(ctx, "POST", base+"/mcp", bytes.NewReader(b))
	if err != nil {
		return dep("mcp_endpoint_or_token")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	res, err := mcpHTTP.Do(req)
	if err != nil {
		var timeout net.Error
		if errors.As(err, &timeout) && timeout.Timeout() {
			return dep("tool_timeout")
		}
		return dep("mcp_transport")
	}
	defer res.Body.Close()
	if res.StatusCode == 401 {
		return dep("mcp_auth")
	}
	if res.Header.Get("Mcp-Session-Id") != "" {
		return dep("mcp_protocol")
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, mcpwire.Limit+1))
	if err != nil || len(raw) > mcpwire.Limit {
		return dep("mcp_response_limit")
	}
	if id == nil {
		if res.StatusCode != 202 || len(raw) != 0 {
			return dep("mcp_protocol")
		}
		return nil
	}
	media, _, _ := mime.ParseMediaType(res.Header.Get("Content-Type"))
	if res.StatusCode != 200 || media != "application/json" {
		return dep("mcp_protocol")
	}
	var response mcpwire.Response
	if mcpwire.Object(raw, &response) != nil || response.Version != "2.0" || response.ID == nil || *response.ID != *id ||
		(response.Error != nil) == (len(response.Result) != 0) {
		return dep("mcp_protocol")
	}
	if response.Error != nil {
		return dep("mcp_rpc_error")
	}
	if mcpwire.Object(response.Result, out) != nil {
		return dep("mcp_protocol")
	}
	return nil
}

type scalarRule struct {
	Type      string            `json:"type"`
	Enum      []json.RawMessage `json:"enum,omitempty"`
	MinLength *int              `json:"minLength,omitempty"`
	MaxLength *int              `json:"maxLength,omitempty"`
	Minimum   *int64            `json:"minimum,omitempty"`
	Maximum   *int64            `json:"maximum,omitempty"`
}

type toolSchema struct {
	Type       string                `json:"type"`
	Properties map[string]scalarRule `json:"properties"`
	Required   []string              `json:"required"`
	Additional *bool                 `json:"additionalProperties"`
}

func (r scalarRule) accepts(raw json.RawMessage, checkEnum bool) bool {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return false
	}
	switch r.Type {
	case "string":
		var s string
		if json.Unmarshal(raw, &s) != nil {
			return false
		}
		n := utf8.RuneCountInString(s)
		if n > 256 || (r.MinLength != nil && n < *r.MinLength) || (r.MaxLength != nil && n > *r.MaxLength) {
			return false
		}
	case "integer":
		n, err := strconv.ParseInt(string(raw), 10, 64)
		if err != nil || n < -1000000 || n > 1000000 || (r.Minimum != nil && n < *r.Minimum) || (r.Maximum != nil && n > *r.Maximum) {
			return false
		}
	case "boolean":
		if string(raw) != "true" && string(raw) != "false" {
			return false
		}
	default:
		return false
	}
	if !checkEnum || len(r.Enum) == 0 {
		return true
	}
	var target any
	_ = json.Unmarshal(raw, &target)
	for _, option := range r.Enum {
		var v any
		_ = json.Unmarshal(option, &v)
		if v == target {
			return true
		}
	}
	return false
}

func parseToolSchema(raw json.RawMessage) (toolSchema, error) {
	var schema toolSchema
	if len(raw) > 4096 || mcpwire.Object(raw, &schema) != nil || schema.Type != "object" || schema.Additional == nil || *schema.Additional ||
		schema.Properties == nil || len(schema.Properties) > 12 {
		return schema, dep("mcp_schema")
	}
	required := map[string]bool{}
	if schema.Required == nil {
		schema.Required = []string{}
	}
	for _, k := range schema.Required {
		if _, ok := schema.Properties[k]; !ok || required[k] {
			return schema, dep("mcp_schema")
		}
		required[k] = true
	}
	for key, rule := range schema.Properties {
		if !mcpName.MatchString(key) || len(rule.Enum) > 16 {
			return schema, dep("mcp_schema")
		}
		switch rule.Type {
		case "string":
			if rule.Minimum != nil || rule.Maximum != nil || (rule.MinLength != nil && (*rule.MinLength < 0 || *rule.MinLength > 256)) ||
				(rule.MaxLength != nil && (*rule.MaxLength < 0 || *rule.MaxLength > 256)) ||
				(rule.MinLength != nil && rule.MaxLength != nil && *rule.MinLength > *rule.MaxLength) {
				return schema, dep("mcp_schema")
			}
		case "integer":
			if rule.MinLength != nil || rule.MaxLength != nil || (rule.Minimum != nil && (*rule.Minimum < -1000000 || *rule.Minimum > 1000000)) ||
				(rule.Maximum != nil && (*rule.Maximum < -1000000 || *rule.Maximum > 1000000)) ||
				(rule.Minimum != nil && rule.Maximum != nil && *rule.Minimum > *rule.Maximum) {
				return schema, dep("mcp_schema")
			}
		case "boolean":
			if rule.MinLength != nil || rule.MaxLength != nil || rule.Minimum != nil || rule.Maximum != nil {
				return schema, dep("mcp_schema")
			}
		default:
			return schema, dep("mcp_schema")
		}
		for _, opt := range rule.Enum {
			if !rule.accepts(opt, false) {
				return schema, dep("mcp_schema")
			}
		}
	}
	return schema, nil
}

func (schema toolSchema) accepts(raw json.RawMessage) bool {
	var values map[string]json.RawMessage
	if len(raw) > 4096 || mcpwire.Object(raw, &values) != nil {
		return false
	}
	for _, key := range schema.Required {
		if _, ok := values[key]; !ok {
			return false
		}
	}
	for key, value := range values {
		rule, ok := schema.Properties[key]
		if !ok || !rule.accepts(value, true) {
			return false
		}
	}
	return true
}

func safeToolText(text string, limit int) (string, bool) {
	text = strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, text))
	lower := strings.ToLower(text)
	for _, phrase := range []string{"ignore previous", "ignore all", "system prompt", "忽略之前", "忽略所有", "系统提示", "bearer ", "api key", "<script"} {
		if strings.Contains(lower, phrase) {
			return "", false
		}
	}
	return trimRunes(text, limit), text != ""
}

func (s *Server) discoverMCP(ctx context.Context, base, token string) ([]mcpwire.Tool, error) {
	initID, listID := int64(1), int64(2)
	var init struct {
		Protocol     string `json:"protocolVersion"`
		Capabilities struct {
			Tools map[string]any `json:"tools"`
		} `json:"capabilities"`
		ServerInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	params := map[string]any{"protocolVersion": mcpwire.Version, "capabilities": map[string]any{}, "clientInfo": map[string]string{"name": "CampusClaw", "version": "1.0"}}
	if err := s.mcpRPC(ctx, base, token, "initialize", &initID, params, &init); err != nil {
		return nil, err
	}
	if init.Protocol != mcpwire.Version || init.Capabilities.Tools == nil || init.ServerInfo.Name == "" || init.ServerInfo.Version == "" {
		return nil, dep("mcp_protocol")
	}
	if err := s.mcpRPC(ctx, base, token, "notifications/initialized", nil, nil, nil); err != nil {
		return nil, err
	}
	var list struct {
		Tools []mcpwire.Tool `json:"tools"`
	}
	if err := s.mcpRPC(ctx, base, token, "tools/list", &listID, nil, &list); err != nil {
		return nil, err
	}
	if list.Tools == nil || len(list.Tools) > 16 {
		return nil, dep("mcp_schema")
	}
	seen := map[string]bool{}
	for i, tool := range list.Tools {
		description, ok := safeToolText(tool.Description, 400)
		if !mcpName.MatchString(tool.Name) || seen[tool.Name] || !ok || utf8.RuneCountInString(tool.Description) > 400 ||
			strings.Contains(tool.Description, token) || strings.Contains(tool.Description, base) || tool.Name == token {
			return nil, dep("mcp_schema")
		}
		schema, err := parseToolSchema(tool.InputSchema)
		if err != nil {
			return nil, err
		}
		tool.InputSchema = mcpwire.Raw(schema)
		tool.Description = description
		list.Tools[i] = tool
		seen[tool.Name] = true
	}
	return list.Tools, nil
}

func toolFingerprint(tool mcpwire.Tool) string {
	sum := sha256.Sum256(mcpwire.Raw(tool))
	return hex.EncodeToString(sum[:])
}

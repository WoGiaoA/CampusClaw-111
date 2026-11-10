package mcpwire

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"
)

const Version = "2025-06-18"
const Limit = 32 << 10

type Request struct {
	Version string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	Version string          `json:"jsonrpc"`
	ID      *int64          `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type Text struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type Result struct {
	Content []Text `json:"content"`
	IsError bool   `json:"isError"`
}

var ErrJSON = errors.New("invalid JSON object")

func Object(raw []byte, target any) error {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '{' || !utf8.Valid(raw) {
		return ErrJSON
	}
	check := json.NewDecoder(bytes.NewReader(raw))
	check.UseNumber()
	if err := value(check, 0); err != nil {
		return err
	}
	if _, err := check.Token(); err != io.EOF {
		return ErrJSON
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	return d.Decode(target)
}

func value(d *json.Decoder, depth int) error {
	if depth > 16 {
		return ErrJSON
	}
	t, e := d.Token()
	if e != nil {
		return ErrJSON
	}
	switch t {
	case json.Delim('{'):
		seen := map[string]bool{}
		for d.More() {
			k, e := d.Token()
			key, ok := k.(string)
			if e != nil || !ok || seen[key] {
				return ErrJSON
			}
			seen[key] = true
			if e = value(d, depth+1); e != nil {
				return e
			}
		}
	case json.Delim('['):
		for d.More() {
			if e = value(d, depth+1); e != nil {
				return e
			}
		}
	default:
		return nil
	}
	_, e = d.Token()
	return e
}

func Raw(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

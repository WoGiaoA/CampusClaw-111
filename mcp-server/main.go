package main

import (
	"campusclaw/internal/mcpwire"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"strings"
	"time"
)

var tools = []mcpwire.Tool{
	{Name: "formula_hint", Description: "按题型给出公式提示和后续策略键。", InputSchema: json.RawMessage(`{"type":"object","properties":{"topic":{"type":"string","enum":["quadratic_equation","linear_equation"]}},"required":["topic"],"additionalProperties":false}`)},
	{Name: "strategy_hint", Description: "按策略键和步骤给出解题引导。", InputSchema: json.RawMessage(`{"type":"object","properties":{"strategy":{"type":"string","enum":["check_discriminant","isolate_variable"]},"step":{"type":"integer","minimum":1,"maximum":3}},"required":["strategy","step"],"additionalProperties":false}`)},
}

var hints = map[string][]string{
	"check_discriminant": {"先把方程整理到右侧为零，再指出二次项系数。", "代入判别式前先核对各系数的符号。你找到的是哪三个系数？", "观察判别式的正负，想想它对应几类实根情况。"},
	"isolate_variable":   {"先找出含未知数的项。怎样让这些项集中在一侧？", "对等式两边做相同变换，再合并同类项。", "检查未知数系数是否为零，再决定能否同除。"},
}

func runTool(name string, args json.RawMessage) (string, bool) {
	switch name {
	case "formula_hint":
		var in struct {
			Topic string `json:"topic"`
		}
		if mcpwire.Object(args, &in) != nil {
			return "", false
		}
		switch in.Topic {
		case "quadratic_equation":
			return "提示：先整理成 ax²+bx+c=0，检查 a 是否非零。策略键：check_discriminant", true
		case "linear_equation":
			return "提示：先整理含未知数的项，检查其系数。策略键：isolate_variable", true
		}
	case "strategy_hint":
		var in struct {
			Strategy string `json:"strategy"`
			Step     int    `json:"step"`
		}
		if mcpwire.Object(args, &in) != nil {
			return "", false
		}
		options := hints[in.Strategy]
		if in.Step >= 1 && in.Step <= len(options) {
			return options[in.Step-1], true
		}
	}
	return "", false
}

func handler(token string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		status, body := 200, "ok"
		if token == "" {
			status, body = 503, "unavailable"
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": body})
	})
	mux.HandleFunc("POST /mcp", func(w http.ResponseWriter, r *http.Request) {
		if len(r.Header.Values("Origin")) != 0 {
			w.WriteHeader(403)
			return
		}
		header := r.Header.Values("Authorization")
		if token == "" || len(header) != 1 || subtle.ConstantTimeCompare([]byte(header[0]), []byte("Bearer "+token)) != 1 {
			w.WriteHeader(401)
			return
		}
		media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || media != "application/json" {
			w.WriteHeader(415)
			return
		}
		raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, mcpwire.Limit))
		var max *http.MaxBytesError
		if errors.As(err, &max) {
			w.WriteHeader(413)
			return
		}
		var in mcpwire.Request
		output := mcpwire.Response{Version: "2.0"}
		fail := func(code int, message string) { output.Error = &mcpwire.RPCError{Code: code, Message: message} }
		switch {
		case err != nil || !json.Valid(raw):
			fail(-32700, "Parse error")
		case mcpwire.Object(raw, &in) != nil:
			fail(-32600, "Invalid request")
		default:
			output.ID = in.ID
			switch {
			case in.Version != "2.0" || in.Method == "":
				fail(-32600, "Invalid request")
			case in.Method == "notifications/initialized" && in.ID == nil:
				if len(in.Params) == 0 || string(in.Params) == "{}" {
					w.WriteHeader(202)
					return
				}
				fail(-32602, "Invalid params")
			case in.ID == nil:
				fail(-32600, "Request ID required")
			default:
				switch in.Method {
				case "initialize":
					var p struct {
						Protocol     string         `json:"protocolVersion"`
						Capabilities map[string]any `json:"capabilities"`
						Client       struct {
							Name    string `json:"name"`
							Version string `json:"version"`
						} `json:"clientInfo"`
					}
					if mcpwire.Object(in.Params, &p) != nil || p.Protocol == "" || p.Capabilities == nil || p.Client.Name == "" || p.Client.Version == "" {
						fail(-32602, "Invalid initialize params")
					} else {
						output.Result = mcpwire.Raw(map[string]any{"protocolVersion": mcpwire.Version, "capabilities": map[string]any{"tools": map[string]bool{"listChanged": false}}, "serverInfo": map[string]string{"name": "campus-tools", "version": "1.0"}})
					}
				case "tools/list":
					if len(in.Params) > 0 && mcpwire.Object(in.Params, &struct{}{}) != nil {
						fail(-32602, "Invalid params")
					} else {
						output.Result = mcpwire.Raw(map[string]any{"tools": tools})
					}
				case "tools/call":
					var p struct {
						Name string          `json:"name"`
						Args json.RawMessage `json:"arguments"`
					}
					if mcpwire.Object(in.Params, &p) != nil {
						fail(-32602, "Invalid params")
						break
					}
					result, ok := runTool(p.Name, p.Args)
					if !ok {
						fail(-32602, "Unknown tool or invalid arguments")
						break
					}
					output.Result = mcpwire.Raw(mcpwire.Result{Content: []mcpwire.Text{{Type: "text", Text: result}}})
				default:
					fail(-32601, "Method not found")
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(output)
	})
	return mux
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		client := http.Client{Timeout: 2 * time.Second}
		r, e := client.Get("http://127.0.0.1:8080/health")
		if e != nil {
			os.Exit(1)
		}
		r.Body.Close()
		if r.StatusCode != 200 {
			os.Exit(1)
		}
		return
	}
	addr := os.Getenv("MCP_LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	srv := http.Server{Addr: addr, Handler: handler(strings.TrimSpace(os.Getenv("MCP_SERVER_TOKEN"))), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 15 * time.Second}
	log.Fatal(srv.ListenAndServe())
}

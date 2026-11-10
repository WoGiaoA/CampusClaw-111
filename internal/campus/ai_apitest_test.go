//go:build apitest

package campus

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAPITestRunner(t *testing.T) {
	python := os.Getenv("CAMPUSCLAW_APITEST_PYTHON")
	if python == "" {
		t.Fatal("run through python apitest/run_all.py")
	}
	s, api := testServer(t)
	s.Config.mcpTestEndpoint = startLocalMCP(t)
	s.Config.MCPCredentialKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	g := newRAGGateway(t)
	g.configure(s)
	if err := s.enqueueMissing(context.Background()); err != nil {
		t.Fatal(err)
	}
	for s.processNextIndex(context.Background()) {
	}
	cancelled := make(chan struct{}, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Messages []modelMessage
			Stream   bool
			Tools    []modelTool
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || !request.Stream || len(request.Messages) < 2 {
			w.WriteHeader(400)
			return
		}
		system := request.Messages[0].Content
		user := request.Messages[len(request.Messages)-1].Content
		isTool := request.Messages[len(request.Messages)-1].Role == "tool"
		if !strings.Contains(system, "APITEST_TEMPLATE") || !strings.Contains(system, "APITEST_SKILL") ||
			(!isTool && !strings.Contains(user, "CLASS_A_ONLY")) || strings.Contains(user, "CLASS_B_ONLY") {
			t.Error("fixture outbound scope or prompt injection mismatch")
			w.WriteHeader(400)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if strings.Contains(user, "MCP 撤权") {
			if len(request.Tools) != 0 || !strings.Contains(system, "当前阶段没有可调用的工具") {
				t.Error("revoked tools remained available or constraint missing")
				return
			}
			if strings.Contains(user, "异常") {
				modelFrame(w, "<|Function")
				modelFrame(w, `CallBegin|>[{"name":"formula_hint"}]<|FunctionCallEnd|>`)
			} else {
				modelFrame(w, "工具不可用，先检查系数。 [1]")
			}
			modelDone(w)
			return
		}
		if strings.Contains(user, "MCP 接口验收") {
			if len(request.Tools) == 2 {
				for _, tool := range request.Tools {
					if strings.Contains(tool.Function.Description, "公式") {
						modelToolFrames(w, 1, tool.Function.Name, `{"topic":"quadratic_equation"}`)
						return
					}
				}
			}
			w.WriteHeader(400)
			return
		}
		if isTool {
			last := request.Messages[len(request.Messages)-1].Content
			if strings.Contains(last, "check_discriminant") {
				for _, tool := range request.Tools {
					if strings.Contains(tool.Function.Description, "按策略键") {
						modelToolFrames(w, 2, tool.Function.Name, `{"strategy":"check_discriminant","step":2}`)
						return
					}
				}
			}
			if strings.Contains(last, "符号") && len(request.Tools) == 0 {
				modelFrame(w, "根据工具提示，先检查符号。 [1]")
				modelDone(w)
				return
			}
			w.WriteHeader(400)
			return
		}
		modelFrame(w, "先复述条件")
		if strings.Contains(user, "取消测试") {
			progress := time.NewTicker(150 * time.Millisecond)
			defer progress.Stop()
			timeout := time.After(5 * time.Second)
			for {
				select {
				case <-r.Context().Done():
					cancelled <- struct{}{}
					return
				case <-progress.C:
					modelFrame(w, " 继续")
				case <-timeout:
					t.Error("client cancellation did not reach model substitute")
					return
				}
			}
		}
		time.Sleep(220 * time.Millisecond)
		modelFrame(w, "，再尝试下一步 [1]")
		modelDone(w)
	}))
	defer upstream.Close()
	s.Config.ChatBaseURL = upstream.URL
	var db string
	if err := s.DB.QueryRow("SELECT DATABASE()").Scan(&db); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	origin := api.URL
	if nginx := os.Getenv("CAMPUSCLAW_APITEST_NGINX_EXE"); nginx != "" {
		origin = startTestNginx(t, nginx, root, api.URL)
		t.Log("API cases use real isolated Nginx")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, python, filepath.Join(root, "apitest", "run_all.py"), "--fixture")
	command.Dir = root
	command.Env = append(os.Environ(), "CAMPUSCLAW_APITEST_ORIGIN="+origin, "CAMPUSCLAW_APITEST_DATABASE="+db, "CAMPUSCLAW_APITEST_MCP="+s.mcpEndpoint())
	result, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("API runner: %v\n%s", err, result)
	}
	t.Log(string(result))
	select {
	case <-cancelled:
		t.Log("client abort reached the upstream model")
	case <-time.After(6 * time.Second):
		t.Error("no upstream cancellation observed")
	}
	if os.Getenv("CAMPUSCLAW_APITEST_BROWSER") == "1" {
		command = exec.CommandContext(ctx, python, filepath.Join(root, "apitest", "browser_check.py"), origin)
		command.Dir = root
		result, err = command.CombinedOutput()
		if err != nil {
			t.Fatalf("browser: %v\n%s", err, result)
		}
		t.Log(string(result))
	}
}

func startTestNginx(t *testing.T, executable, root, upstream string) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	config, err := os.ReadFile(filepath.Join(root, "deploy", "nginx.conf"))
	if err != nil {
		t.Fatal(err)
	}
	server := strings.NewReplacer("listen 80;", "listen "+address+";",
		"root /usr/share/nginx/html;", fmt.Sprintf("root %q;", filepath.ToSlash(filepath.Join(root, "web"))),
		"http://api:8080", upstream).Replace(string(config))
	prefix := filepath.ToSlash(t.TempDir()) + "/"
	for _, dir := range []string{"logs", "temp"} {
		if err := os.Mkdir(prefix+dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	body := "worker_processes 1; daemon off; error_log stderr; pid nginx.pid; events {} http { types { text/html html; text/css css; application/javascript js; application/json json; } " + server + " }"
	if err := os.WriteFile(prefix+"isolated.conf", []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	log, err := os.Create(prefix + "nginx-test.log")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { log.Close() })
	command := exec.Command(executable, "-p", prefix, "-c", "isolated.conf")
	command.Stdout, command.Stderr = log, log
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	t.Cleanup(func() {
		_ = exec.Command(executable, "-p", prefix, "-c", "isolated.conf", "-s", "quit").Run()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = command.Process.Kill()
			<-done
		}
	})
	origin := "http://" + address
	client := &http.Client{Timeout: 400 * time.Millisecond}
	for range 30 {
		response, err := client.Get(origin + "/docs/openapi.json")
		if err == nil {
			response.Body.Close()
			if response.StatusCode == 200 {
				return origin
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	data, _ := os.ReadFile(prefix + "nginx-test.log")
	t.Fatalf("isolated Nginx did not start: %s", data)
	return ""
}

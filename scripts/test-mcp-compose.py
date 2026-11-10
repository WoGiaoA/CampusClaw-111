"""Lesson 6 isolated Compose smoke. Never load the working deployment's .env."""
import json
import os
from pathlib import Path
import secrets
import socket
import subprocess
import tempfile
import time
from urllib.error import HTTPError
from urllib.request import ProxyHandler, Request, build_opener

ROOT = Path(__file__).resolve().parent.parent
WORK = ROOT / ".local"
WORK.mkdir(exist_ok=True)


def main():
    project = "campusclaw-lesson06-test-" + secrets.token_hex(5)
    with tempfile.TemporaryDirectory(prefix="compose-mcp-", dir=WORK) as temporary:
        run(project, Path(temporary))


def run(project, directory):
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        port = sock.getsockname()[1]
    password, token = secrets.token_hex(20), secrets.token_hex(24)
    env = {
        "MYSQL_DATABASE": "campusclaw", "MYSQL_USER": "campusclaw",
        "MYSQL_PASSWORD": secrets.token_hex(20), "MYSQL_ROOT_PASSWORD": secrets.token_hex(20),
        "SESSION_KEY": secrets.token_hex(32), "SESSION_COOKIE_SECURE": "false",
        "SEED_TEACHER_A_PASSWORD": password, "SEED_TEACHER_B_PASSWORD": password,
        "SEED_STUDENT_A1_PASSWORD": password, "SEED_STUDENT_B1_PASSWORD": password,
        "APP_ENV": "production", "DEMO_MODE": "false", "WEB_PORT": str(port),
        "MCP_SERVER_TOKEN": token, "MCP_CREDENTIAL_KEY": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
        "CHAT_BASE_URL": "http://model-fixture", "CHAT_MODEL": "isolated-model", "CHAT_API_KEY": "",
        "EMBEDDING_BASE_URL": "", "EMBEDDING_MODEL": "", "EMBEDDING_API_KEY": "",
        "EMBEDDING_DIMENSION": "", "QDRANT_API_KEY": "",
    }
    env_file = directory / "fixture.env"
    env_file.write_text("".join(f"{k}={v}\n" for k, v in env.items()), encoding="utf-8")
    child_env = {**os.environ, **env, "COMPOSE_PROFILES": ""}
    frame = 'data: {"choices":[{"delta":{"content":"isolated response"}}]}\n\n'
    frame += 'data: {"choices":[{"delta":{},"finish_reason":"stop"}]}\n\ndata: [DONE]\n\n'
    fixture = directory / "model.conf"
    fixture.write_text("server { listen 80; location = /chat/completions { default_type text/event-stream; return 200 "
                       + json.dumps(frame) + "; } }\n", encoding="utf-8")
    override = directory / "compose.json"
    override.write_text(json.dumps({"services": {"model-fixture": {
        "image": "public.ecr.aws/docker/library/nginx:1.28-alpine",
        "volumes": [str(fixture) + ":/etc/nginx/conf.d/default.conf:ro"],
    }}}), encoding="utf-8")
    compose = ["docker", "compose", "--profile", "mcp", "--env-file", str(env_file), "-p", project, "-f",
               str(ROOT / "docker-compose.yml"), "-f", str(override)]
    def docker(*args, quiet=False):
        return subprocess.run([*compose, *args], cwd=ROOT, env=child_env, check=True, text=True,
                              stdout=subprocess.PIPE if quiet else None, timeout=360)

    origin = f"http://127.0.0.1:{port}"
    opener = build_opener(ProxyHandler({}))
    def request(path, method="GET", body=None, identity=None):
        headers = {"Content-Type": "application/json"}
        if identity:
            headers["Authorization"] = "Bearer " + identity["access_token"]
            headers["X-CSRF-Token"] = identity["csrf_token"]
        req = Request(origin + path, method=method, headers=headers,
                      data=json.dumps(body).encode() if body is not None else None)
        try:
            with opener.open(req, timeout=15) as response:
                return response.status, response.read().decode()
        except HTTPError as error:
            return error.code, error.read().decode()

    def checked(path, status, method="GET", body=None, identity=None):
        got, response = request(path, method, body, identity)
        assert got == status, f"{method} {path}: expected {status}, got {got}"
        return json.loads(response)

    try:
        baseline = subprocess.run([*compose[:2], *compose[4:], "config", "--services"], cwd=ROOT,
                                  env=child_env, check=True, capture_output=True, text=True, timeout=30)
        assert "api" in baseline.stdout.splitlines() and "mcp-server" not in baseline.stdout.splitlines()
        model_config = json.loads(docker("config", "--format", "json", quiet=True).stdout)
        api = model_config["services"]["api"]["environment"]
        assert api["CHAT_BASE_URL"] == "http://model-fixture" and not api["EMBEDDING_BASE_URL"]
        assert not model_config["services"]["mcp-server"].get("ports")
        assert model_config["networks"]["tools"]["internal"]
        docker("up", "-d", "--build", "--wait", "--wait-timeout", "120")
        checked("/health", 200)
        teacher = checked("/api/login", 200, "POST", {"username":"teacher_a","password":password})
        record = checked("/api/mcp/servers", 201, "POST", {"slug":"campus-tools","display_name":"测试工具",
                         "base_url":"http://mcp-server:8080","token":token}, teacher)
        path = f'/api/mcp/servers/{record["id"]}'
        connected = checked(path + "/connect", 200, "POST", {}, teacher)
        assert connected["tool_count"] == 2
        docker("stop", "mcp-server")
        checked("/health", 200)
        checked(path + "/connect", 503, "POST", {}, teacher)
        student = checked("/api/login", 200, "POST", {"username":"student_a1","password":password})
        checked("/api/search", 200, "POST", {"q":"课程导论","mode":"keyword"}, student)
        app = checked("/api/ai/apps", 200, identity=student)["items"][0]["id"]
        convo = checked("/api/ai/conversations", 201, "POST", {"app_id":app}, student)["id"]
        status, stream = request(f"/api/ai/conversations/{convo}/messages", "POST",
                                 {"question":"isolated fixture","client_request_id":"40000000-0000-4000-a000-000000000001"}, student)
        assert status == 200 and "event: done" in stream and "isolated response" in stream and "event: tool" not in stream
        print("Isolated Compose: internal discovery=2; MCP port unpublished; stop/reconnect fails safely; health/login/search/no-tool SSE pass. Course gateway calls 0/0.")
    finally:
        if project.startswith("campusclaw-lesson06-test-"):
            docker("down", "--volumes", "--remove-orphans")


if __name__ == "__main__":
    main()

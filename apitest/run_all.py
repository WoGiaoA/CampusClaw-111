"""Run Lesson 5/6 API checks with disposable MySQL, local MCP and model substitutes."""
import html
from http.client import HTTPConnection
import importlib.util
import json
import os
from pathlib import Path
import re
import socket
import struct
import subprocess
import sys
import time
from urllib.error import HTTPError
from urllib.request import Request, build_opener, ProxyHandler

ROOT = Path(__file__).resolve().parent.parent
REPORT = ROOT / "apitest" / "reports" / "report.html"
CASES = [
    "api/login/POST.py", "api/search/POST.py", "api/ai/apps/GET.py", "api/ai/apps/POST.py",
    "api/ai/prompt-templates/GET.py", "api/ai/prompt-templates/POST.py",
    "api/ai/prompt-templates/{template_id}/PATCH.py",
    "api/ai/skills/GET.py", "api/ai/skills/POST.py", "api/ai/skills/{skill_id}/PATCH.py",
    "api/ai/apps/{app_id}/PATCH.py", "api/ai/conversations/POST.py",
    "api/ai/conversations/{conversation_id}/messages/POST.py",
    "api/ai/conversations/GET.py", "api/ai/conversations/{conversation_id}/GET.py",
    "api/mcp/servers/POST.py", "api/mcp/servers/GET.py", "api/mcp/servers/{id}/PATCH.py",
    "api/mcp/servers/{id}/connect/POST.py", "api/mcp/servers/{id}/tools/GET.py",
    "api/ai/apps/{app_id}/tool-grants/PUT.py", "api/ai/apps/{app_id}/tool-grants/GET.py",
    "api/mcp/audit/GET.py", "api/mcp/servers/{id}/disconnect/POST.py",
    "mcp-server/health/GET.py", "mcp-server/mcp/POST.py",
    "api/materials/{id}/DELETE.py",
    "api/ai/conversations/{conversation_id}/DELETE.py", "api/ai/apps/{app_id}/DELETE.py",
]


def validate_cases():
    """Catch stale endpoint paths, missing scripts and forgotten runner entries."""
    root = ROOT / "apitest" / "cases"
    discovered = {path.relative_to(root).as_posix() for path in root.rglob("*.py")}
    assert len(CASES) == len(set(CASES)), "接口清单包含重复脚本"
    assert set(CASES) == discovered, "接口脚本与run_all.py清单不一致"
    paths = json.loads((ROOT / "web" / "docs" / "openapi.json").read_text(encoding="utf-8"))["paths"]
    for case in CASES:
        endpoint = "/" + str(Path(case).parent).replace("\\", "/")
        if endpoint.startswith("/api/"):
            assert Path(case).stem.lower() in paths.get(endpoint, {}), f"OpenAPI缺少 {case}"


class Suite:
    def __init__(self):
        self.base = os.environ["CAMPUSCLAW_APITEST_ORIGIN"]
        self.mcp = os.environ["CAMPUSCLAW_APITEST_MCP"]
        self.database = os.environ["CAMPUSCLAW_APITEST_DATABASE"]
        address = os.environ["MYSQL_TEST_ADDR"]
        if not all(re.fullmatch(r"http://127\.0\.0\.1:\d+", url) for url in [self.base, self.mcp]):
            raise ValueError("API fixture must use loopback")
        if not re.fullmatch(r"campusclaw_test_\d+", self.database) or not re.fullmatch(r"127\.0\.0\.1:\d+", address):
            raise ValueError("database must be an isolated loopback fixture")
        self.port = address.split(":")[1]
        self.opener = build_opener(ProxyHandler({}))
        self.accounts = {}
        self.state = {}
        self.observations = []

    def tool_request(self, method, path, expected, payload=None, token=None):
        headers = {"Content-Type": "application/json"}
        if token is not None:
            headers["Authorization"] = "Bearer " + token
        data = None if payload is None else json.dumps(payload).encode()
        request = Request(self.mcp + path, data=data, method=method, headers=headers)
        try:
            result = self.opener.open(request, timeout=8)
        except HTTPError as failure:
            result = failure
        with result as response:
            assert response.status == expected, f"MCP {path}: expected {expected}, got {response.status}"
            raw = response.read()
            return json.loads(raw) if raw else None

    def request(self, method, path, expected, account=None, data=None, csrf=True, file=None, stream=False):
        headers = {}
        if account:
            who = self.accounts[account]
            headers["Authorization"] = "Bearer " + who["access_token"]
            if csrf:
                headers["X-CSRF-Token"] = who["csrf_token"]
        if file is not None:
            boundary = "campusclaw-isolated-test-boundary"
            payload = (f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="skill.md"\r\n'
                       f"Content-Type: text/plain\r\n\r\n{file}\r\n--{boundary}--\r\n").encode()
            headers["Content-Type"] = f"multipart/form-data; boundary={boundary}"
        elif data is not None:
            payload = json.dumps(data, ensure_ascii=False).encode()
            headers["Content-Type"] = "application/json"
        else:
            payload = None
        request = Request(self.base + path, data=payload, method=method, headers=headers)
        connection = None
        if stream == "abort":
            connection = HTTPConnection(self.base.removeprefix("http://"), timeout=12)
            connection.request(method, path, body=payload, headers=headers)
            result = connection.getresponse()
        else:
            try:
                result = self.opener.open(request, timeout=12)
            except HTTPError as failure:
                result = failure
        with result as response:
            assert response.status == expected, f"{method} {path}: expected {expected}, got {response.status}"
            if stream:
                assert response.headers.get_content_type() == "text/event-stream", "SSE content type"
                started = time.monotonic()
                frames = []
                kind = None
                for raw in response:
                    line = raw.decode().rstrip("\r\n")
                    if line.startswith("event: "):
                        kind = line[7:]
                    elif line.startswith("data: "):
                        frames.append((kind, json.loads(line[6:]), time.monotonic() - started))
                        if stream == "abort" and kind == "delta":
                            linger = struct.pack("HH" if os.name == "nt" else "ii", 1, 0)
                            connection.sock.setsockopt(socket.SOL_SOCKET, socket.SO_LINGER, linger)
                            connection.close()
                            return frames
                return frames
            return json.load(response)

    def sql(self, statement):
        result = subprocess.run(
            [os.environ["CAMPUSCLAW_APITEST_MYSQL_EXE"], "--protocol=TCP", "--host=127.0.0.1",
             "--port=" + self.port, "--user=root", "--batch", "--skip-column-names",
             "--default-character-set=utf8mb4", "--database=" + self.database, "--execute=" + statement],
            capture_output=True, encoding="utf-8", timeout=8, check=False,
        )
        assert result.returncode == 0, "read-only fixture DB assertion failed"
        return result.stdout.rstrip("\r\n")


def report(rows):
    REPORT.parent.mkdir(exist_ok=True)
    table = "\n".join(
        "<tr>" + "".join(f"<td>{html.escape(str(field))}</td>" for field in row) + "</tr>"
        for row in rows
    )
    passed = sum(row[1] == "PASS" for row in rows)
    REPORT.write_text(f"""<!doctype html><html lang="zh-CN"><meta charset="utf-8">
<title>CampusClaw 第 5、6 课接口测试</title><style>
body{{font:16px system-ui;max-width:1050px;margin:2rem auto;padding:1rem}}
table{{width:100%;border-collapse:collapse}}td,th{{text-align:left;border-bottom:1px solid #ddd;padding:.6rem}}
</style><h1>CampusClaw 第 5、6 课接口测试</h1>
<p>通过 {passed} / {len(rows)}。隔离 MySQL、真实本地 MCP 与本地模型替身。课程网关 Chat/Embedding：0/0。</p>
<table><tr><th>用例</th><th>结果</th><th>耗时 (ms)</th><th>断言</th></tr>{table}</table></html>""", encoding="utf-8")


def finish_report(code, browser):
    # A passing API table must not mask a later browser or fixture failure.
    status = "PASS" if code == 0 else "FAIL"
    scope = "接口及Chrome/Swagger检查" if browser else "接口与隔离夹具检查"
    note = f'<p id="runner-result">整体执行结果：{status}（{scope}）。用例表仅统计接口脚本；详情见命令输出。</p>'
    body = REPORT.read_text(encoding="utf-8")
    REPORT.write_text(body.replace("</html>", note + "</html>"), encoding="utf-8")


def fixture_run():
    rows = []
    suite = Suite()
    sys.dont_write_bytecode = True
    for index, case in enumerate(CASES):
        started = time.monotonic()
        try:
            spec = importlib.util.spec_from_file_location(f"case_{index}", ROOT / "apitest" / "cases" / case)
            module = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(module)
            detail = module.run(suite) or "HTTP、权限及相关断言通过"
            result = "PASS"
        except Exception as error:
            detail = str(error) if isinstance(error, AssertionError) else type(error).__name__
            result = "FAIL"
        rows.append((case, result, round((time.monotonic() - started) * 1000), detail))
    report(rows)
    print(f"API cases: {sum(row[1] == 'PASS' for row in rows)}/{len(rows)}; {REPORT.relative_to(ROOT)}")
    return int(any(row[1] == "FAIL" for row in rows))


def main():
    if sys.argv[1:] == ["--init-demo"]:
        from demo_setup import main as initialize_demo
        try:
            initialize_demo(ROOT)
            return 0
        except Exception as failure:
            print(f"演示初始化未完成：{failure}", file=sys.stderr)
            return 1
    try:
        validate_cases()
    except (AssertionError, OSError, ValueError) as failure:
        report([("接口清单与OpenAPI", "FAIL", 0, str(failure))])
        print("接口清单与OpenAPI检查失败，请查看本地HTML报告", file=sys.stderr)
        return 1
    if sys.argv[1:] == ["--fixture"]:
        return fixture_run()
    if sys.argv[1:]:
        print("Usage: python apitest/run_all.py [--init-demo]", file=sys.stderr)
        return 2
    if REPORT.is_file():
        REPORT.unlink()
    browser = os.environ.get("CAMPUSCLAW_APITEST_BROWSER") == "1"
    if browser and importlib.util.find_spec("playwright") is None:
        report([("浏览器依赖", "FAIL", 0, "当前Python缺少Playwright，请安装依赖或关闭浏览器模式")])
        return 1
    environment = os.environ.copy()
    environment["CAMPUSCLAW_APITEST_PYTHON"] = sys.executable
    code = subprocess.run(
        ["powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File",
         str(ROOT / "scripts" / "test-isolated.ps1"), "-APITest"],
        cwd=ROOT, env=environment, check=False,
    ).returncode
    if not REPORT.is_file():
        report([("启动隔离环境", "FAIL", 0, "未生成用例结果，请查看命令输出")])
        return 1
    finish_report(code, browser)
    return code


if __name__ == "__main__":
    sys.exit(main())

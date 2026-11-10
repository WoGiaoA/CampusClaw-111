"""Explicit teacher demo setup through authenticated APIs; no Chat requests."""
import json
from pathlib import Path
import re
from urllib.error import HTTPError
from urllib.request import Request, build_opener, ProxyHandler


class DemoClient:
    def __init__(self, origin, password, username="teacher_a"):
        if not re.fullmatch(r"http://127\.0\.0\.1:\d+", origin):
            raise ValueError("演示初始化只允许本机 http://127.0.0.1:端口")
        self.origin = origin
        self.opener = build_opener(ProxyHandler({}))
        self.auth = {}
        self.auth = self.call("POST", "/api/login", {"username": username, "password": password})

    def call(self, method, path, data=None, source=None):
        headers = {}
        if self.auth:
            headers.update(Authorization="Bearer " + self.auth["access_token"])
            headers["X-CSRF-Token"] = self.auth["csrf_token"]
        payload = None
        if source:
            boundary = "campusclaw-demo-upload"
            payload = (f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="{source.name}"\r\n'
                       'Content-Type: text/plain\r\n\r\n').encode() + source.read_bytes() + f"\r\n--{boundary}--\r\n".encode()
            headers["Content-Type"] = "multipart/form-data; boundary=" + boundary
        elif data is not None:
            payload = json.dumps(data, ensure_ascii=False).encode()
            headers["Content-Type"] = "application/json"
        try:
            with self.opener.open(Request(self.origin + path, data=payload, headers=headers, method=method), timeout=30) as response:
                return json.load(response)
        except HTTPError as error:
            # Do not log bodies or request credentials.
            raise RuntimeError(f"{method} {path}: HTTP {error.code}；检查账号、服务配置或同名资源冲突") from None


def unique(items, name, key="name"):
    matches = [item for item in items if item[key] == name]
    if len(matches) > 1:
        raise ValueError(f"同名资源不唯一，停止初始化：{name}")
    return matches[0] if matches else None


def provision(client, root, token, mcp_url="http://mcp-server:8080"):
    """Reuse originals, skip identical objects, refuse conflicting named objects."""
    if len(list((root / "examples/knowledge").glob("*.md"))) != 13 or len(list((root / "examples/skills").glob("*.md"))) != 3:
        raise ValueError("仓库样例不完整，需要13份资料和3份Skill文件")
    materials = client.call("GET", "/api/materials")
    mids = []
    for source in sorted((root / "examples/knowledge").glob("*.md")):
        old = unique(materials, source.name, "title")
        if old:
            detail = client.call("GET", f'/api/materials/{old["id"]}')
            if detail["body_text"].replace("\r\n", "\n") != source.read_text(encoding="utf-8"):
                raise ValueError(f"已有同名资料内容不同，保留原数据：{source.name}")
            mids.append(old["id"])
        else:
            mids.append(client.call("POST", "/api/materials/upload", source=source)["material_id"])
    servers = client.call("GET", "/api/mcp/servers")["items"]
    server = unique(servers, "campus-tools", "slug")
    if server and server["base_url"] != mcp_url:
        raise ValueError("已有campus-tools地址不同，保留原配置")
    if not server:
        server = client.call("POST", "/api/mcp/servers", {"slug": "campus-tools", "display_name": "课程解题工具", "base_url": mcp_url, "token": token})
    if server["status"] != "connected":
        client.call("POST", f'/api/mcp/servers/{server["id"]}/connect', {})
    tools = client.call("GET", f'/api/mcp/servers/{server["id"]}/tools')["items"]
    if not {"formula_hint", "strategy_hint"} <= {item["name"] for item in tools}:
        raise ValueError("MCP发现未包含需要的两个工具")
    skills = client.call("GET", "/api/ai/skills")["items"]
    selected = {}
    for source in sorted((root / "examples/skills").glob("*.md")):
        raw = source.read_text(encoding="utf-8")
        name = raw.splitlines()[0].split("：", 1)[1].strip()
        skill = unique(skills, name)
        if skill:
            if skill["conditions"] != raw.splitlines()[1].split("：", 1)[1].strip() or skill["procedure"] != raw.split("\n\n", 1)[1].strip() or not skill["enabled"]:
                raise ValueError(f"已有Skill配置不同，保留原配置：{name}")
            expected_refs = ["formula_hint@campus-tools", "strategy_hint@campus-tools"] if name == "MCP验收解题引导" else []
            if sorted(skill["tool_refs"]) != sorted(expected_refs):
                raise ValueError(f"已有Skill工具声明不同，保留原配置：{name}")
        else:
            skill = client.call("POST", "/api/ai/skills", source=source)
        selected[name] = skill["id"]
    apps = client.call("GET", "/api/ai/apps")["items"]
    qa = unique(apps, "课程资料问答")
    guide = unique(apps, "解题引导")
    if not qa or not guide:
        raise ValueError("缺少默认应用，请使用空库初始化后的环境")
    # Bind additively; retain role instructions, templates, enabled state and other bindings.
    for app, material_ids, skill_ids in [(qa, mids, []), (guide, mids, [selected["解题引导"], selected["人工测试解题引导"]])]:
        body = {"material_ids": sorted(set(app["material_ids"] + material_ids)), "skill_ids": sorted(set(app["skill_ids"] + skill_ids))}
        if body["material_ids"] != sorted(app["material_ids"]) or body["skill_ids"] != sorted(app["skill_ids"]):
            client.call("PATCH", f'/api/ai/apps/{app["id"]}', body)
    app = unique(apps, "MCP工具验收")
    if not app:
        app = client.call("POST", "/api/ai/apps", {"name": "MCP工具验收"})
        app = client.call("PATCH", f'/api/ai/apps/{app["id"]}', {"enabled": True, "role_prompt": guide["role_prompt"]})
    refs = client.call("GET", f'/api/ai/apps/{app["id"]}/tool-grants')["items"]
    required = {"formula_hint@campus-tools", "strategy_hint@campus-tools"}
    merged = sorted(required | {item["ref"] for item in refs})
    if len(merged) != len(refs):
        client.call("PUT", f'/api/ai/apps/{app["id"]}/tool-grants', {"refs": merged})
    sid = selected["MCP验收解题引导"]
    if sid not in app["skill_ids"]:
        client.call("PATCH", f'/api/ai/apps/{app["id"]}', {"skill_ids": app["skill_ids"] + [sid]})
    return {"materials": len(mids), "skills": len(selected), "mcp_tools": 2}


def main(root):
    values = {}
    for line in (root / ".env").read_text(encoding="utf-8-sig").splitlines():
        if line.strip() and not line.lstrip().startswith("#") and "=" in line:
            key, value = line.split("=", 1)
            values[key.strip()] = value.strip().strip('"').strip("'")
    if not values.get("SEED_TEACHER_A_PASSWORD") or not values.get("MCP_SERVER_TOKEN"):
        raise ValueError("本机.env缺少教师初始密码或MCP令牌")
    # Existing accounts may have changed their password; never reset it automatically.
    password = __import__("os").environ.get("CAMPUSCLAW_DEMO_TEACHER_PASSWORD") or values["SEED_TEACHER_A_PASSWORD"]
    client = DemoClient("http://127.0.0.1:" + values.get("WEB_PORT", "8080"), password)
    try:
        counts = provision(client, root, values["MCP_SERVER_TOKEN"])
        print(f'演示初始化完成：A班{counts["materials"]}份资料、{counts["skills"]}项Skill、2项MCP工具；账号无需注册。')
    finally:
        client.call("POST", "/api/logout", {})

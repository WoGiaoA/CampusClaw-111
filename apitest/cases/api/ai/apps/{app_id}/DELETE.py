from uuid import uuid4


def run(s):
    app = s.state["lifecycle_app"]
    path = f"/api/ai/apps/{app}"
    conv = s.request("POST", "/api/ai/conversations", 201, "student_a1", {"app_id": app})
    cid = int(conv["id"])
    s.request("DELETE", path, 401)
    s.request("DELETE", path, 403, "student_a1")
    s.request("DELETE", path, 403, "teacher_a", csrf=False)
    s.request("DELETE", path, 404, "teacher_b")
    assert s.request("DELETE", path, 200, "teacher_a") == {"id": app, "status": "deleted"}
    assert s.sql(f"SELECT CONCAT(enabled,':',deleted_at IS NOT NULL) FROM assistants WHERE id={app}") == "0:1"
    assert s.sql(f"SELECT COUNT(*) FROM assistant_materials WHERE assistant_id={app}") == "1"
    s.request("DELETE", path, 404, "teacher_a")
    s.request("PATCH", path, 404, "teacher_a", {"enabled": True})
    for who in ["teacher_a", "student_a1"]:
        assert all(row["id"] != app for row in s.request("GET", "/api/ai/apps", 200, who)["items"])
    s.request("POST", "/api/ai/conversations", 404, "student_a1", {"app_id": app})
    history = s.request("GET", f"/api/ai/conversations/{cid}", 200, "student_a1")
    assert not history["conversation"]["app_enabled"]
    denied = s.request("POST", f"/api/ai/conversations/{cid}/messages", 403, "student_a1",
                       {"question": "判别式删除应用检查", "client_request_id": str(uuid4())})
    assert denied["code"] == "app_disabled"
    assert s.sql(f"SELECT COUNT(*) FROM ai_turns WHERE conversation_id={cid}") == "0"
    # Run the public initializer twice in the untouched B-class fixture only.
    import sys
    from pathlib import Path
    root = next(parent for parent in Path(__file__).resolve().parents if (parent / "apitest/demo_setup.py").is_file())
    sys.path.insert(0, str(root / "apitest"))
    from demo_setup import DemoClient, provision
    client = DemoClient(s.base, "isolation-test-b", username="teacher_b")
    first = provision(client, root, s.state["mcp_token"], s.mcp)
    counts = s.sql("SELECT CONCAT((SELECT COUNT(*) FROM materials),',',(SELECT COUNT(*) FROM skills),',',(SELECT COUNT(*) FROM assistants),',',(SELECT COUNT(*) FROM mcp_servers))")
    assert first == {"materials": 13, "skills": 3, "mcp_tools": 2}
    demo_apps = client.call("GET", "/api/ai/apps")["items"]
    by_name = {item["name"]: item for item in demo_apps}
    assert len(by_name["课程资料问答"]["material_ids"]) == 13
    assert len(by_name["解题引导"]["material_ids"]) == 13
    assert len(by_name["解题引导"]["skill_ids"]) == 2
    assert by_name["MCP工具验收"]["enabled"] and len(by_name["MCP工具验收"]["skill_ids"]) == 1
    grants = client.call("GET", f'/api/ai/apps/{by_name["MCP工具验收"]["id"]}/tool-grants')["items"]
    assert len(grants) == 2 and all(item["available"] for item in grants)
    assert provision(client, root, s.state["mcp_token"], s.mcp) == first
    assert s.sql("SELECT CONCAT((SELECT COUNT(*) FROM materials),',',(SELECT COUNT(*) FROM skills),',',(SELECT COUNT(*) FROM assistants),',',(SELECT COUNT(*) FROM mcp_servers))") == counts
    client.call("POST", "/api/logout", {})
    return "应用软删除及停用、权限隔离、重复删除、保留绑定/历史且禁止继续发送"

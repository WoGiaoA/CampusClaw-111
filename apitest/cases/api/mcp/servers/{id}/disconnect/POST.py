def run(s):
    base = f'/api/mcp/servers/{s.state["mcp_server"]}'
    s.request("POST", base + "/disconnect", 404, "teacher_b")
    data = s.request("POST", base + "/disconnect", 200, "teacher_a")
    assert data["status"] == "disconnected" and data["tool_count"] == 0
    assert not s.request("GET", base + "/tools", 200, "teacher_a")["items"]
    path = f'/api/ai/apps/{s.state["app"]}/tool-grants'
    assert all(not item["available"] for item in s.request("GET", path, 200, "teacher_a")["items"])

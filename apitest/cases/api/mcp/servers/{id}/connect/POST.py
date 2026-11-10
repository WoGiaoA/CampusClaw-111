def run(s):
    path = f'/api/mcp/servers/{s.state["mcp_server"]}'
    s.request("POST", path + "/connect", 403, "teacher_a", csrf=False)
    failed = s.request("POST", path + "/connect", 503, "teacher_a")
    assert failed["code"] == "mcp_auth"
    s.request("PATCH", path, 200, "teacher_a", {"token": s.state["mcp_token"]})
    for _ in range(2):
        data = s.request("POST", path + "/connect", 200, "teacher_a")
        assert data["status"] == "connected" and data["tool_count"] == 2
    return "错误令牌拒绝；真实 MCP 发现及重连整体替换"

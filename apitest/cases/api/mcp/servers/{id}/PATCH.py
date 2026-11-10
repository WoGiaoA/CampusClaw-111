def run(s):
    path = f'/api/mcp/servers/{s.state["mcp_server"]}'
    s.request("PATCH", path, 404, "teacher_b", {"display_name": "越权"})
    data = s.request("PATCH", path, 200, "teacher_a", {"display_name": "课程 MCP"})
    assert data["has_credential"] and data["status"] == "disconnected"

def run(s):
    path = "/api/mcp/servers"
    s.state["mcp_token"] = "lesson06-isolated-mcp-token"
    body = {"slug": "campus-tools", "display_name": "课程工具", "base_url": s.mcp,
            "token": "invalid-isolated-token"}
    s.request("POST", path, 403, "student_a1", body)
    s.request("POST", path, 400, "teacher_a", {**body, "base_url": "http://169.254.169.254"})
    data = s.request("POST", path, 201, "teacher_a", body)
    assert data["has_credential"] and "token" not in data and "credential" not in data
    s.state["mcp_server"] = int(data["id"])

def run(s):
    path = "/api/mcp/servers"
    s.request("GET", path, 403, "student_a1")
    assert not s.request("GET", path, 200, "teacher_b")["items"]
    data = s.request("GET", path, 200, "teacher_a")
    assert data["configured"] and len(data["items"]) == 1
    assert "invalid-isolated-token" not in str(data)

def run(s):
    path = "/api/mcp/audit"
    s.request("GET", path, 403, "student_a1")
    assert not s.request("GET", path, 200, "teacher_b")["items"]
    items = s.request("GET", path + "?status=success", 200, "teacher_a")["items"]
    assert len(items) == 2 and all(item["conversation_id"] == s.state["mcp_conversation"] for item in items)
    assert "check_discriminant" not in str(items) and s.state["mcp_token"] not in str(items)

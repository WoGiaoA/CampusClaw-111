def run(s):
    path = "/api/ai/conversations"
    assert not s.request("GET", path, 200, "student_b1")["items"]
    items = s.request("GET", path + "?limit=1", 200, "student_a1")["items"]
    assert [row["id"] for row in items] == [s.state["conversation"]]
    s.request("GET", path + "?limit=0", 400, "student_a1")

def run(s):
    path = f'/api/ai/apps/{s.state["app"]}/tool-grants'
    s.request("GET", path, 404, "teacher_b")
    assert len(s.request("GET", path, 200, "teacher_a")["items"]) == 2

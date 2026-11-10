def run(s):
    path = "/api/ai/prompt-templates"
    assert len(s.request("GET", path, 200, "teacher_a")["items"]) == 2
    s.request("GET", path, 403, "student_a1")

def run(s):
    path = "/api/ai/skills"
    skills = s.request("GET", path, 200, "teacher_a")["items"]
    assert any(skill["name"] == "解题引导" and skill["enabled"] for skill in skills)
    s.request("GET", path, 403, "student_a1")

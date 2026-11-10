def run(s):
    key = s.state["template"]
    path = f"/api/ai/prompt-templates/{key}"
    s.request("PATCH", path, 404, "teacher_b", {"body": "越权"})
    s.request("PATCH", path, 200, "teacher_a", {"body": "APITEST_TEMPLATE"})
    assert s.sql(f"SELECT body FROM prompt_templates WHERE id={key}") == "APITEST_TEMPLATE"

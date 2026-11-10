def run(s):
    path = "/api/ai/prompt-templates"
    fields = {"name": "隔离模板", "body": "APITEST_INITIAL"}
    s.request("POST", path, 403, "teacher_a", fields, csrf=False)
    result = s.request("POST", path, 201, "teacher_a", fields)
    s.state["template"] = int(result["id"])
    assert result["body"] == fields["body"]

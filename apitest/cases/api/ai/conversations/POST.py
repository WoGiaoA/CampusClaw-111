def run(s):
    path = "/api/ai/conversations"
    body = {"app_id": s.state["app"]}
    s.request("POST", path, 403, "student_a1", body, csrf=False)
    s.request("POST", path, 404, "student_b1", body)
    row = s.request("POST", path, 201, "student_a1", body)
    assert row["app_id"] == body["app_id"] and row["title"] == "新对话"
    s.state["conversation"] = int(row["id"])
    assert s.sql(f'SELECT assistant_id FROM ai_conversations WHERE id={s.state["conversation"]}') == str(body["app_id"])

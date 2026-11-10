def run(s):
    path = "/api/ai/apps"
    fields = {"name": "接口生命周期应用"}
    s.request("POST", path, 401, data=fields)
    s.request("POST", path, 403, "student_a1", fields)
    s.request("POST", path, 403, "teacher_a", fields, csrf=False)
    s.request("POST", path, 400, "teacher_a", {"name": " "})
    item = s.request("POST", path, 201, "teacher_a", fields)
    key = int(item["id"])
    assert not item["enabled"] and not item["skill_ids"] and not item["material_ids"]
    assert item["name"] == fields["name"]
    assert s.sql(f"SELECT enabled FROM assistants WHERE id={key}") == "0"
    assert s.sql(f"SELECT COUNT(*) FROM assistant_materials WHERE assistant_id={key}") == "0"
    assert all(row["id"] != key for row in s.request("GET", path, 200, "student_a1")["items"])
    s.state["lifecycle_app"] = key
    return "新建默认停用且无绑定；匿名、学生、CSRF、输入与数据库校验"

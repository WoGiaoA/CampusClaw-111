def run(s):
    key = s.state["skill"]
    path = f"/api/ai/skills/{key}"
    s.request("PATCH", path, 404, "teacher_b", {"enabled": False})
    assert not s.request("PATCH", path, 200, "teacher_a", {"enabled": False})["enabled"]
    assert s.request("PATCH", path, 200, "teacher_a", {"enabled": True, "conditions": "判别式、怎么做"})["enabled"]
    assert s.sql(f"SELECT enabled FROM skills WHERE id={key}") == "1"
    large = s.state["large_skill"]
    source = "名称：替换长规程\n适用条件：长文本专用\n启用：是\n\nAPITEST_SKILL " + "提示" * 6500
    result = s.request("PATCH", f"/api/ai/skills/{large}", 200, "teacher_a", file=source)
    assert result["raw_text"] == source
    reread = next(item for item in s.request("GET", "/api/ai/skills", 200, "teacher_a")["items"] if item["id"] == large)
    assert reread["raw_text"] == source and reread["procedure"] == source.split("\n\n", 1)[1]
    assert s.sql(f"SELECT CHAR_LENGTH(raw_text) FROM skills WHERE id={large}") == str(len(source))
    return "跨班拒绝、启停与条件保存；大文本替换、重读和数据库一致"

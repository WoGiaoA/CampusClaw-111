def run(s):
    key = s.state["teacher_a_apps"][0]["id"]
    path = f"/api/ai/apps/{key}"
    s.request("PATCH", path, 404, "teacher_a", {"material_ids": [1, 2], "enabled": False})
    s.request("PATCH", path, 403, "student_a1", {"enabled": False})
    result = s.request("PATCH", path, 200, "teacher_a", {"skill_ids": [s.state["skill"]],
                       "template_id": s.state["template"], "material_ids": [1]})
    assert result["enabled"] and result["material_ids"] == [1]
    assert s.sql(f"SELECT COUNT(*) FROM assistant_materials WHERE assistant_id={int(key)}") == "1"
    s.state["app"] = int(key)
    # Use a separate application so the main lesson 5/6 fixture stays usable.
    from uuid import uuid4
    app = s.state["lifecycle_app"]
    target = f"/api/ai/apps/{app}"
    settings = {"enabled": True, "skill_ids": [s.state["skill"]],
                "template_id": s.state["template"], "material_ids": [1]}
    assert s.request("PATCH", target, 200, "teacher_a", settings)["enabled"]
    conv = s.request("POST", "/api/ai/conversations", 201, "student_a1", {"app_id": app})
    cid = int(conv["id"])
    s.state["lifecycle_conversation"] = cid
    assert not s.request("PATCH", target, 200, "teacher_a", {"enabled": False})["enabled"]
    assert s.sql(f"SELECT enabled FROM assistants WHERE id={app}") == "0"
    assert all(row["id"] != app for row in s.request("GET", "/api/ai/apps", 200, "student_a1")["items"])
    s.request("POST", "/api/ai/conversations", 404, "student_a1", {"app_id": app})
    history = s.request("GET", f"/api/ai/conversations/{cid}", 200, "student_a1")
    assert not history["conversation"]["app_enabled"]
    denied = s.request("POST", f"/api/ai/conversations/{cid}/messages", 403, "student_a1",
                       {"question": "判别式停用检查", "client_request_id": str(uuid4())})
    assert denied["code"] == "app_disabled"
    assert s.sql(f"SELECT COUNT(*) FROM ai_turns WHERE conversation_id={cid}") == "0"
    assert s.request("PATCH", target, 200, "teacher_a", {"enabled": True})["enabled"]
    return "绑定及SQL一致；停用拒绝新会话和旧会话发送、历史只读、无新轮次，恢复启用"

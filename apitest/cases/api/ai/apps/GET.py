def run(s):
    path = "/api/ai/apps"
    s.request("GET", path, 401)
    for account in ("teacher_a", "teacher_b", "student_a1"):
        s.state[account + "_apps"] = s.request("GET", path, 200, account)["items"]
    assert len(s.state["teacher_a_apps"]) == 2
    assert all(set(item) == {"id", "name"} for item in s.state["student_a1_apps"])
    assert not {a["id"] for a in s.state["teacher_a_apps"]} & {a["id"] for a in s.state["teacher_b_apps"]}
    for account in ("teacher_a", "teacher_b"):
        apps = s.state[account + "_apps"]
        assert {item["name"] for item in apps} == {"课程资料问答", "解题引导"}
        assert all(item["enabled"] and item["template_id"] and not item["material_ids"] for item in apps)
        guide = next(item for item in apps if item["name"] == "解题引导")
        assert len(guide["skill_ids"]) == 1

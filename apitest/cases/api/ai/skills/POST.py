def run(s):
    path = "/api/ai/skills"
    s.request("POST", path, 400, "teacher_a", file="缺字段")
    source = "名称：接口规程\n适用条件：判别式\n启用：是\n\nAPITEST_SKILL 逐步提示。"
    s.request("POST", path, 403, "student_a1", file=source)
    item = s.request("POST", path, 201, "teacher_a", file=source)
    assert item["raw_text"] == source and item["enabled"]
    s.state["skill"] = int(item["id"])
    long_source = "名称：接口长规程\n适用条件：长文本专用\n启用：是\n\nAPITEST_SKILL " + "提示" * 6000
    assert 32 * 1024 < len(long_source.encode("utf-8")) < 128 * 1024
    large = s.request("POST", path, 201, "teacher_a", file=long_source)
    assert large["raw_text"] == long_source
    s.state["large_skill"] = int(large["id"])
    assert s.sql(f'SELECT CHAR_LENGTH(raw_text) FROM skills WHERE id={s.state["large_skill"]}') == str(len(long_source))
    too_large = "名称：超限规程\n适用条件：长文本专用\n启用：是\n\n" + "引" * 44000
    assert s.request("POST", path, 413, "teacher_a", file=too_large)["code"] == "skill_too_large"
    from pathlib import Path
    sample_root = next(parent for parent in Path(__file__).resolve().parents if (parent / "examples/skills").is_dir())
    source = (sample_root / "examples/skills/解题引导.md").read_text(encoding="utf-8")
    sample = s.request("POST", "/api/ai/skills", 201, "teacher_a", file=source)
    assert sample["raw_text"] == source and sample["procedure"] == source.split("\n\n", 1)[1].strip()
    s.request("PATCH", f'/api/ai/skills/{sample["id"]}', 200, "teacher_a", {"enabled": False})
    source = (sample_root / "examples/skills/人工测试解题引导.md").read_text(encoding="utf-8")
    sample = s.request("POST", "/api/ai/skills", 201, "teacher_a", file=source)
    assert sample["raw_text"] == source and sample["procedure"] == source.split("\n\n", 1)[1].strip()
    s.request("PATCH", f'/api/ai/skills/{sample["id"]}', 200, "teacher_a", {"enabled": False})
    return "有效格式、权限、超过旧32 KB文本完整上传和SQL长度一致、新上限拒绝"

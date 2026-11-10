def run(s):
    # Create a new synthetic file; never delete the shared seeded fixtures that
    # the lesson 5/6 cases and optional browser regression rely on.
    material = s.request("POST", "/api/materials/upload", 201, "teacher_a",
                         file="# Disposable delete fixture\nDELETE_API_ONLY\n")
    mid = int(material["material_id"])
    path = f"/api/materials/{mid}"
    s.request("DELETE", path, 401)
    s.request("DELETE", path, 403, "student_a1")
    s.request("DELETE", path, 403, "teacher_a", csrf=False)
    s.request("DELETE", path, 404, "teacher_b")
    result = s.request("DELETE", path, 202, "teacher_a")
    assert result == {"material_id": mid, "status": "cleanup_pending"}
    assert s.request("DELETE", path, 202, "teacher_a")["status"] == "cleanup_pending"
    assert s.request("GET", path, 410, "student_a1")["code"] == "material_deleted"
    assert s.request("GET", path + "/file", 410, "teacher_a")["code"] == "material_deleted"
    assert all(item["id"] != mid for item in s.request("GET", "/api/materials", 200, "teacher_a"))
    assert s.sql(f"SELECT COUNT(*) FROM material_deletions WHERE material_id={mid}") == "1"
    assert s.sql(f"SELECT COUNT(*) FROM assistant_materials WHERE material_id={mid}") == "0"

    from pathlib import Path
    root = next(parent for parent in Path(__file__).resolve().parents if (parent / "examples/knowledge").is_dir())
    files = sorted((root / "examples/knowledge").glob("*.md"))
    assert len(files) == 13
    for source in files:
        body = source.read_text(encoding="utf-8")
        uploaded = s.request("POST", "/api/materials/upload", 201, "teacher_a", file=body)
        mid = int(uploaded["material_id"])
        restored = s.request("GET", f"/api/materials/{mid}", 200, "student_a1")
        assert restored["body_text"] == body
        assert restored["byte_size"] == len(body.encode("utf-8"))
        s.request("GET", f"/api/materials/{mid}", 404, "student_b1")
    return "材料删除；13份已有样例上传和正文/字节数读回一致，跨班隔离"

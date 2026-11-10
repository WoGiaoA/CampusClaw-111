def run(s):
    path = "/api/search"
    s.request("POST", path, 401, data={"q": "课程导论"})
    for mode in ("keyword", "vector", "hybrid"):
        items = s.request("POST", path, 200, "student_a1", {"q": "课程导论", "mode": mode})["hits"]
        assert items and all(hit["material_id"] == 1 for hit in items)
    s.request("POST", path, 400, "student_a1", {"q": ""})
    assert s.sql("SELECT COUNT(*) FROM knowledge_chunks WHERE material_id=1 AND index_status='ready'") != "0"
    return "三模式、本班范围、空查询与真实 SQL 切片"

def run(s):
    for name, password in {"teacher_a": "isolation-test-a", "teacher_b": "isolation-test-b",
                           "student_a1": "isolation-test-a1", "student_b1": "isolation-test-b1"}.items():
        row = s.request("POST", "/api/login", 200, data={"username": name, "password": password})
        assert row["token_type"] == "Bearer" and row["csrf_token"]
        s.accounts[name] = row
    s.request("POST", "/api/login", 401, data={"username": "unknown", "password": "bad"})
    assert s.sql("SELECT COUNT(*) FROM sessions") == "4"
    return "四账号、拒绝无效登录、Session 持久化"

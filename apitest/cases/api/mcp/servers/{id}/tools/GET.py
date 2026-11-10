def run(s):
    path = f'/api/mcp/servers/{s.state["mcp_server"]}/tools'
    s.request("GET", path, 404, "teacher_b")
    rows = s.request("GET", path, 200, "teacher_a")["items"]
    assert {item["name"] for item in rows} == {"formula_hint", "strategy_hint"}
    assert all(item["inputSchema"]["additionalProperties"] is False for item in rows)

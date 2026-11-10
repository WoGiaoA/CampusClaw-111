def run(s):
    assert s.tool_request("GET", "/health", 200) == {"status": "ok"}

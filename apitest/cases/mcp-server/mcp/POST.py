def run(s):
    body = {"jsonrpc": "2.0", "id": 8, "method": "tools/list"}
    s.tool_request("POST", "/mcp", 401, body)
    s.tool_request("POST", "/mcp", 401, body, "wrong-fixture")
    result = s.tool_request("POST", "/mcp", 200, body, s.state["mcp_token"])
    assert result["id"] == 8 and len(result["result"]["tools"]) == 2
    unknown = s.tool_request("POST", "/mcp", 200, {**body, "method": "invalid"}, s.state["mcp_token"])
    assert unknown["error"]["code"] == -32601

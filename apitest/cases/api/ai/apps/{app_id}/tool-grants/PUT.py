from uuid import uuid4


def run(s):
    app = s.state["app"]
    path = f"/api/ai/apps/{app}/tool-grants"
    refs = ["formula_hint@campus-tools", "strategy_hint@campus-tools"]
    s.request("PUT", path, 403, "student_a1", {"refs": refs})
    s.request("PATCH", f'/api/ai/skills/{s.state["skill"]}', 200, "teacher_a", {"tool_refs": refs})
    data = s.request("PUT", path, 200, "teacher_a", {"refs": refs})
    assert all(item["available"] for item in data["items"])
    s.request("PUT", path, 404, "teacher_a", {"refs": ["formula_hint@unknown"]})
    conv = s.request("POST", "/api/ai/conversations", 201, "student_a1", {"app_id": app})
    frames = s.request("POST", f'/api/ai/conversations/{conv["id"]}/messages', 200, "student_a1",
                       {"question": "MCP 接口验收：判别式怎么做？", "client_request_id": str(uuid4())}, stream=True)
    assert [frame[0] for frame in frames] == ["meta", "tool", "tool", "tool", "tool", "delta", "done"]
    assert [frame[1]["status"] for frame in frames[1:5]] == ["running", "success", "running", "success"]
    assert "符号" in frames[-2][1]["text"]
    s.state["mcp_conversation"] = int(conv["id"])
    assert s.sql(f'SELECT COUNT(*) FROM tool_call_audit WHERE conversation_id={s.state["mcp_conversation"]}') == "4"
    cid = s.state["mcp_conversation"]
    message_path = f"/api/ai/conversations/{cid}/messages"
    s.request("PUT", path, 200, "teacher_a", {"refs": []})
    plain = s.request("POST", message_path, 200, "student_a1",
                      {"question": "MCP 撤权普通判别式", "client_request_id": str(uuid4())}, stream=True)
    assert [frame[0] for frame in plain] == ["meta", "delta", "done"]
    aid = int(plain[-1][1]["assistant_message_id"])
    assert s.sql(f"SELECT body FROM ai_messages WHERE id={aid}") == plain[1][1]["text"]
    bad = s.request("POST", message_path, 200, "student_a1",
                    {"question": "MCP 撤权异常判别式", "client_request_id": str(uuid4())}, stream=True)
    assert [frame[0] for frame in bad] == ["meta", "error"], "内部标记不得作为正文发出"
    assert bad[-1][1]["code"] == "unexpected_tool_text"
    turn = int(bad[0][1]["turn_id"])
    assert s.sql(f"SELECT status FROM ai_turns WHERE id={turn}") == "failed"
    assert s.sql(f"SELECT COUNT(*) FROM ai_messages WHERE turn_id={turn} AND role='assistant'") == "0"
    assert s.sql(f"SELECT COUNT(*) FROM tool_call_audit WHERE conversation_id={cid}") == "4"
    s.request("PUT", path, 200, "teacher_a", {"refs": refs})
    from pathlib import Path
    sample_root = next(parent for parent in Path(__file__).resolve().parents if (parent / "examples/skills").is_dir())
    source = (sample_root / "examples/skills/MCP验收解题引导.md").read_text(encoding="utf-8")
    sample = s.request("POST", "/api/ai/skills", 201, "teacher_a", file=source)
    assert sample["raw_text"] == source and sample["procedure"] == source.split("\n\n", 1)[1].strip()
    s.request("PATCH", f'/api/ai/skills/{sample["id"]}', 200, "teacher_a", {"enabled": False})
    return "真实本地MCP两次调用及审计；撤权后普通回答保存、异常分片拦截且无工具/助手记录"

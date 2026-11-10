from uuid import uuid4


def run(s):
    cid = s.state["lifecycle_conversation"]
    path = f"/api/ai/conversations/{cid}"
    frames = s.request("POST", path + "/messages", 200, "student_a1",
                       {"question": "判别式生命周期检查", "client_request_id": str(uuid4())}, stream=True)
    assert frames[-1][0] == "done"
    aid = int(frames[-1][1]["assistant_message_id"])
    body = "".join(frame[1]["text"] for frame in frames if frame[0] == "delta")
    assert s.sql(f"SELECT body FROM ai_messages WHERE id={aid}") == body
    s.request("DELETE", path, 401)
    s.request("DELETE", path, 403, "teacher_a")
    s.request("DELETE", path, 403, "student_a1", csrf=False)
    s.request("DELETE", path, 404, "student_b1")
    result = s.request("DELETE", path, 200, "student_a1")
    assert result == {"id": cid, "status": "deleted"}
    assert s.sql(f"SELECT deleted_at IS NOT NULL FROM ai_conversations WHERE id={cid}") == "1"
    assert s.sql(f"SELECT body FROM ai_messages WHERE id={aid}") == body, "软删除保留消息"
    s.request("GET", path, 404, "student_a1")
    s.request("DELETE", path, 404, "student_a1")
    s.request("POST", path + "/messages", 404, "student_a1",
              {"question": "判别式删除检查", "client_request_id": str(uuid4())})
    assert all(row["id"] != cid for row in s.request("GET", "/api/ai/conversations", 200, "student_a1")["items"])
    return "本人会话软删除；匿名/角色/CSRF/跨班、重复删除、消息保留与列表/发送不可见"

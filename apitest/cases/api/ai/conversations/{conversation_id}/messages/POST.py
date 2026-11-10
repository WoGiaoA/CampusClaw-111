from uuid import uuid4
import time


def run(s):
    path = f'/api/ai/conversations/{s.state["conversation"]}/messages'
    body = {"question": "判别式怎么做？", "client_request_id": str(uuid4())}
    s.request("POST", path, 404, "student_b1", body)
    frames = s.request("POST", path, 200, "student_a1", body, stream=True)
    assert [row[0] for row in frames] == ["meta", "delta", "delta", "done"], "SSE 事件顺序"
    assert frames[0][1]["skill"]["id"] == s.state["skill"], "命中 Skill"
    assert frames[0][1]["citations"][0]["material_id"] == 1, "仅绑定资料"
    assert frames[2][2] - frames[1][2] >= 0.12, "首个增量早于第二个增量"
    assistant = int(frames[-1][1]["assistant_message_id"])
    text = "".join(row[1]["text"] for row in frames if row[0] == "delta")
    assert s.sql(f"SELECT body FROM ai_messages WHERE id={assistant}") == text, "增量拼接等于保存正文"
    s.request("POST", path, 409, "student_a1", body)
    abort_id = str(uuid4())
    aborted = s.request("POST", path, 200, "student_a1", {
        "question": "判别式取消测试", "client_request_id": abort_id,
    }, stream="abort")
    assert [frame[0] for frame in aborted] == ["meta", "delta"]
    query = f"SELECT status FROM ai_turns WHERE client_request_id='{abort_id}'"
    for _ in range(30):
        if s.sql(query) == "failed":
            break
        time.sleep(0.1)
    assert s.sql(query) == "failed", "主动取消标记失败"
    s.state["reply"] = text
    return f"SSE 正文片段间隔 {round((frames[2][2] - frames[1][2]) * 1000)} ms；命中、引用、去重、取消及数据库一致"

def run(s):
    path = f'/api/ai/conversations/{s.state["conversation"]}'
    s.request("GET", path, 404, "student_b1")
    history = s.request("GET", path, 200, "student_a1")
    assert len(history["items"]) == 2
    turn = history["items"][0]
    assert turn["status"] == "completed" and turn["messages"][1]["body"] == s.state["reply"]
    assert turn["messages"][1]["metadata"]["citations"][0]["material_id"] == 1
    assert history["items"][1]["status"] == "failed" and len(history["items"][1]["messages"]) == 1

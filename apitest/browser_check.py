"""Optional Chrome checks against the same isolated API/Nginx fixture."""
import os
import json
from pathlib import Path
import subprocess
import sys
import time
from uuid import uuid4

from playwright.sync_api import sync_playwright, expect


def login(page, origin, username, password):
    page.goto(origin + "/login")
    page.locator("[name=username]").fill(username)
    page.locator("[name=password]").fill(password)
    page.locator("#login-form button[type=submit]").click()
    page.wait_for_url("**/workspace")


def check_width(page):
    assert page.evaluate("document.documentElement.scrollWidth <= innerWidth"), "horizontal overflow"


def main(origin):
    chrome = os.environ.get("CAMPUSCLAW_APITEST_CHROME", r"C:\Program Files\Google\Chrome\Application\chrome.exe")
    assert Path(chrome).is_file()
    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(executable_path=chrome, headless=True)
        teacher = browser.new_page(viewport={"width": 1280, "height": 720})
        login(teacher, origin, "teacher_a", "isolation-test-a")
        teacher.goto(origin + "/applications")
        teacher.get_by_role("button", name="提示词模板").click()
        prompt = teacher.locator(".config-controls textarea").first
        prompt.fill("浏览器保存角色提示词")
        teacher.get_by_role("button", name="保存应用提示词").click()
        expect(teacher.locator(".app-config > .feedback")).to_contain_text("操作已保存")
        teacher.reload()
        teacher.get_by_role("button", name="提示词模板").click()
        expect(prompt).to_have_value("浏览器保存角色提示词")
        teacher.set_viewport_size({"width": 390, "height": 780})
        check_width(teacher)
        teacher.get_by_role("button", name="MCP 工具", exact=True).click()
        expect(teacher.locator(".config-controls")).to_contain_text("保存工具授权")
        teacher.goto(origin + "/tools")
        teacher.get_by_role("button", name="连接并发现").click()
        expect(teacher.locator(".config-controls > .feedback")).to_contain_text("工具清单已更新")
        teacher.get_by_role("button", name="查看工具清单").click()
        expect(teacher.locator("summary").filter(has_text="formula_hint")).to_be_visible()
        check_width(teacher)
        teacher.goto(origin + "/audit")
        expect(teacher.get_by_text("formula_hint@campus-tools").first).to_be_visible()

        student = browser.new_page(viewport={"width": 1463, "height": 775})
        login(student, origin, "student_a1", "isolation-test-a1")
        student.goto(origin + "/assistant")
        select = student.locator(".application-picker select")
        expect(select).to_be_enabled()
        select.select_option(label="课程资料问答")
        student.get_by_role("textbox", name="你的问题").fill("判别式怎么做？")
        student.get_by_role("textbox", name="你的问题").press("Enter")
        expect(student.locator(".chat-transcript > .feedback")).to_contain_text("回答已保存")
        expect(student.locator(".conversation-turn")).to_contain_text("接口规程")
        expect(student.locator(".conversation-turn")).to_contain_text("再尝试下一步")
        expect(student.locator(".compact-source a")).to_have_attribute("href", "/materials?material_id=1&chunk_id=1")
        student.locator(".compact-source summary").click()
        expect(student.locator(".compact-source details")).to_have_attribute("open", "")
        student.set_viewport_size({"width": 390, "height": 780})
        check_width(student)
        student.reload()
        student.locator(".session-open").filter(has_text="判别式").first.click()
        expect(student.locator(".conversation-turn")).to_contain_text("再尝试下一步")
        student.locator(".reset-chat").click()
        student.get_by_role("textbox", name="你的问题").fill("MCP 接口验收：判别式怎么做？")
        student.get_by_role("textbox", name="你的问题").press("Enter")
        expect(student.locator(".chat-transcript > .feedback")).to_contain_text("回答已保存")
        student.locator(".conversation-turn summary").filter(has_text="工具执行过程").click()
        expect(student.locator(".conversation-turn .chat-capabilities")).to_contain_text("strategy_hint@campus-tools")
        check_width(student)

        docs = browser.new_page()
        separate = playwright.request.new_context()
        login_data = separate.post(origin + "/api/login", data={"username": "teacher_a", "password": "isolation-test-a"}).json()
        docs.goto(origin + "/docs/swagger/")
        expect(docs.locator(".info .title")).to_contain_text("CampusClaw API")
        docs.locator(".auth-wrapper button.authorize").click()
        dialog = docs.locator(".dialog-ux")
        dialog.locator("input").first.fill(login_data["access_token"])
        dialog.get_by_role("button", name="Apply credentials").first.click()
        dialog.locator("button.btn-done").first.click()
        operation = docs.locator(".opblock-get").filter(has_text="教师看本班配置")
        operation.locator(".opblock-summary").click()
        operation.get_by_role("button", name="Try it out").click()
        with docs.expect_response(lambda response: response.url.endswith("/api/ai/apps")) as result:
            operation.get_by_role("button", name="Execute").click()
        response = result.value
        assert response.status == 200
        assert response.request.headers.get("authorization", "").startswith("Bearer ")
        mcp_op = docs.locator(".opblock-get").filter(has_text="读取本班 MCP 服务")
        mcp_op.locator(".opblock-summary").click()
        mcp_op.get_by_role("button", name="Try it out").click()
        with docs.expect_response(lambda response: response.url.endswith("/api/mcp/servers")) as result:
            mcp_op.get_by_role("button", name="Execute").click()
        assert result.value.status == 200 and result.value.json()["items"][0]["tool_count"] == 2
        listed = result.value.json()
        csrf = login_data["csrf_token"]

        def swagger_try(method, title, path, parameters=None, payload=None):
            operation = docs.locator(".opblock-" + method.lower()).filter(has_text=title)
            operation.locator(".opblock-summary").click()
            operation.get_by_role("button", name="Try it out").click()
            for key, value in (parameters or {}).items():
                operation.locator(f'tr[data-param-name="{key}"] input').fill(str(value))
            if method != "GET":
                operation.locator('tr[data-param-name="X-CSRF-Token"] input').fill(csrf)
            if payload is not None:
                operation.locator("textarea.body-param__text").fill(json.dumps(payload))
            with docs.expect_response(lambda response: response.url.endswith(path) and response.request.method == method) as result:
                operation.get_by_role("button", name="Execute").click()
            return result.value

        service = swagger_try("POST", "登记本班 MCP 服务", "/api/mcp/servers", payload={
            "slug": "swagger-tools", "display_name": "Swagger 试调",
            "base_url": listed["allowed_base_urls"][0], "token": "lesson06-isolated-mcp-token",
        })
        assert service.status == 201 and "token" not in service.json()
        server_id = service.json()["id"]
        endpoint = f"/api/mcp/servers/{server_id}"
        assert swagger_try("POST", "连接并发现 MCP 工具", endpoint + "/connect", {"id": server_id}).status == 200
        assert len(swagger_try("GET", "读取当前 MCP 工具清单", endpoint + "/tools", {"id": server_id}).json()["items"]) == 2
        app_id = response.json()["items"][0]["id"]
        headers = {"Authorization": "Bearer " + login_data["access_token"]}
        skill = next(item for item in separate.get(origin + "/api/ai/skills", headers=headers).json()["items"]
                     if "APITEST_SKILL" in item["procedure"])
        refs = ["formula_hint@campus-tools", "strategy_hint@campus-tools"]
        assert swagger_try("PATCH", "教师编辑适用条件", f'/api/ai/skills/{skill["id"]}', {"skill_id": skill["id"]},
                           {"tool_refs": refs}).status == 200
        assert swagger_try("PUT", "整组保存本班应用工具授权", f"/api/ai/apps/{app_id}/tool-grants", {"app_id": app_id},
                           {"refs": refs}).status == 200
        endpoint = origin + f"/api/ai/apps/{app_id}"
        assert separate.patch(endpoint, headers=headers, data={"enabled": True}).status == 403
        headers["X-CSRF-Token"] = login_data["csrf_token"]
        assert separate.patch(endpoint, headers=headers, data={"enabled": True}).status == 200
        student_token = separate.post(origin + "/api/login", data={
            "username": "student_a1", "password": "isolation-test-a1",
        }).json()
        student_headers = {
            "Authorization": "Bearer " + student_token["access_token"],
            "X-CSRF-Token": student_token["csrf_token"],
        }
        created = separate.post(origin + "/api/ai/conversations", headers=student_headers, data={"app_id": app_id})
        assert created.status == 201
        body = json.dumps({"question": "判别式怎么做？", "client_request_id": str(uuid4())}, ensure_ascii=False)
        url = origin + f'/api/ai/conversations/{created.json()["id"]}/messages'
        config = "\n".join(
            "header = " + json.dumps(f"{key}: {value}", ensure_ascii=False)
            for key, value in {**student_headers, "Content-Type": "application/json"}.items()
        )
        config += "\ndata = " + json.dumps(body, ensure_ascii=False) + "\n"
        with subprocess.Popen(["curl.exe", "-fsS", "-N", "--max-time", "10", "--config", "-", url],
                              stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL) as curl:
            curl.stdin.write(config.encode("utf-8"))
            curl.stdin.close()
            arrivals, events = [], []
            for line in curl.stdout:
                if line.startswith(b"event: "):
                    kind = line[7:].strip().decode()
                    events.append(kind)
                    if kind == "delta":
                        arrivals.append(time.monotonic())
            assert curl.wait(timeout=10) == 0
        assert events == ["meta", "delta", "delta", "done"]
        assert arrivals[1] - arrivals[0] > 0.12
        # Browser verification only operates on this disposable API fixture.
        # Verify both source compatibility and withdrawn history at the end so
        # the earlier lesson 5/6 checks continue to use their seeded material.
        retained_client = playwright.request.new_context()
        retained_login = retained_client.post(origin + "/api/login", data={
            "username": "teacher_a", "password": "isolation-test-a",
        }).json()
        retained_headers = {"Authorization": "Bearer " + retained_login["access_token"],
                            "X-CSRF-Token": retained_login["csrf_token"]}
        retained = retained_client.post(origin + "/api/materials/upload", headers=retained_headers,
            multipart={"file": {"name": "retained-browser-material.md", "mimeType": "text/markdown", "buffer": b"BROWSER_RETAINED_MATERIAL"}})
        assert retained.status == 201, f"retained fixture upload: {retained.status} {retained.text()}"
        retained_id = retained.json()["material_id"]
        teacher.goto(origin + "/knowledge")
        if teacher.locator(".material-delete").count() == 0:
            teacher.goto(origin + "/materials")
        first = teacher.locator(".material").filter(has=teacher.get_by_text("A 班-课程导论.txt", exact=True)).first
        expect(first).to_be_visible()
        first.get_by_role("button", name="查看", exact=True).click()
        expect(teacher.locator("dialog pre").last).to_contain_text("CLASS_A_ONLY")
        teacher.locator("dialog").get_by_role("button", name="关闭", exact=True).click()
        first.locator(".material-delete").click()
        confirm = teacher.locator(".delete-material-dialog")
        expect(confirm).to_contain_text("不可撤销")
        confirm.get_by_role("button", name="取消", exact=True).click()
        expect(first).to_be_visible()
        check_width(teacher)
        first.locator(".material-delete").click()
        confirm.get_by_role("button", name="确认删除", exact=True).click()
        expect(first).to_have_count(0)
        expect(teacher.locator("#status")).to_contain_text("材料已停止使用")
        student.reload()
        student.locator(".session-open").filter(has_text="MCP 接口验收").first.click()
        expect(student.locator(".deleted-source").first).to_contain_text("材料已删除")
        assert student.locator(".compact-source a").count() == 0
        check_width(student)
        teacher.set_viewport_size({"width": 1280, "height": 720})
        teacher.goto(origin + "/materials")
        check_width(teacher)
        assert teacher.locator(".material-delete").count() >= 1
        kept = retained_client.get(origin + f"/api/materials/{retained_id}/file", headers=retained_headers)
        assert kept.status == 200 and kept.body() == b"BROWSER_RETAINED_MATERIAL"
        print("Chrome: teacher deletion confirmation/cancel, narrow layout, withdrawn source history and legacy material page passed")
        browser.close()
    print("Chrome: teacher save/connection/discovery/audit, student tool events and SSE replay, narrow layout, Swagger Bearer try-out passed")
    print(f"curl -N: two deltas separated by {round((arrivals[1] - arrivals[0]) * 1000)} ms")


if __name__ == "__main__":
    main(sys.argv[1])

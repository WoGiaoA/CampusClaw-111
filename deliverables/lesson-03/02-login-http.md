# 作业2：登录 HTTP 请求与响应

当前公开 URL 为 http://127.0.0.1:8080/api/login，方法 POST。Content-Type: application/json（允许 charset=utf-8）。请求为单一 JSON：

    {"username":"teacher_a","password":"[已打码]"}

成功为 200 JSON，不由后端发送 302；前端读取 must_change_password 后进入 /profile 或 /materials。真实成功响应摘自隔离测试（密码/Cookie 已脱敏）：

    Content-Type: application/json; charset=utf-8
    Cache-Control: no-store
    X-Content-Type-Options: nosniff
    Set-Cookie: campusclaw_session=[REDACTED]; Path=/; Max-Age=28800; HttpOnly; SameSite=Lax

    {"must_change_password":false,"id":1,"username":"teacher_a","role":"teacher","class_id":1,"class_name":"A 班"}

完整头与逐次响应在 [final-http.json](final-http.json)：login:teacher_a 及其他三账号均 200；错误口令、未知账号均 401，正文同为 {"error":"账号或密码错误"}。限流达到阈值亦为同一 401，已在 TestMySQLAuthSessions 验证。非法 JSON/类型/尾随值或超限为 400，不创建 Session；依赖故障可为 503，不能当作密码错误。当前公开入口的非法 JSON 实测为 400，见 [final-runtime.json](final-runtime.json)。成功证据来自真实 Go HTTP + MySQL 隔离测试，经 Nginx 的当前检查为匿名与非法输入，二者范围不同。

Set-Cookie 让浏览器保存随机会话 ID，后续按作用域自动携带；不是把角色、班级或密码存进 Cookie。HttpOnly 禁止脚本读 Cookie；SameSite=Lax 限制部分跨站携带；HTTP 演示不含 Secure，配置 HTTPS 时应开启。登录成功换发 ID，旧 ID 失效。

源码：[login/cookie/sessionHash](../../internal/campus/auth.go)、[JSON 边界](../../internal/campus/json.go)、[前端登录与导航](../../web/app.js)、[真实 HTTP 测试](../../internal/campus/acceptance_test.go)。

自行截图：F12 → Network → 勾选 Preserve log → 清空请求 → 登录 → 找 api/login。Payload 查看 JSON；Headers 查看 Request URL/Method/Status，以及 Response Headers 的 view source（浏览器支持时）；Response 看完整 JSON。另分别错误口令和不存在账号各一次，比较 401 与正文。截图至少保留 Payload、完整响应头、Response；密码与 Set-Cookie 值打码，保留 Cookie 名称和属性。

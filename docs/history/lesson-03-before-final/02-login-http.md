# 作业2：真实登录 HTTP

> **第三课最终整理阶段状态：开发修正与隔离测试已通过，等待用户启动 Docker 并最终验收。** 下方原有容器运行结论与 JSON/TXT 文件属于 **第三课初次验收阶段的历史证据**，不是本轮重新部署结果。最新测试边界及操作顺序见 [本轮复验](lesson-03-regression.md)。截图全部由用户完成。

## 接口和响应

URL http://127.0.0.1:8080/api/login；POST，Content-Type: application/json，JSON username/password。通过公开 Nginx 同源转发，Go校验bcrypt。

实际主入口 teacher_b 返回200及 id/username/role/class_id/class_name/must_change_password=true，设置随机ID Cookie。API没有302；SPA根据改密标记导航 /profile 或 /materials。错误密码、未知账号返回401与 {"error":"账号或密码错误"}，不设置有效新会话。限流同构在真实MySQL集成测试验证。

成功响应包含 application/json; charset=utf-8、Cache-Control:no-store、X-Content-Type-Options:nosniff、Nginx Server、Date、Set-Cookie。完整脱敏响应见 [主入口记录](primary-login-http.json)，容器错误响应见 [91项记录](container-http-first.json)。Set-Cookie包含 Path=/、Max-Age=28800、HttpOnly、SameSite=Lax；本机HTTP Secure=false，HTTPS配置才开启Secure。

Set-Cookie要求浏览器保存服务器创建的随机Session ID，不含角色或班级，也不是密码。后续自动携带 Cookie，服务端查摘要找到用户。

## 源码

[登录](../../internal/campus/auth.go)、[身份](../../internal/campus/server.go)、[Nginx](../../deploy/nginx.conf)、[SPA跳转](../../web/app.js)。

## F12 自行截图

1. F12 → Network，启用 Preserve log，输入账号密码并提交。
2. 选择 /api/login，Payload查看真实JSON。
3. Headers中查看Request URL/Method/Status、完整Response Headers和Set-Cookie。
4. Response查看身份/首次标记；额外提交一次错误密码截图401。
5. 密码和Cookie值打码，保留属性与状态码。不要把教材样例当实测响应。

## 本轮补充

登录媒体类型精确解析为 application/json，允许 charset=utf-8；application/jsonp、尾随第二个JSON、尾随垃圾和超过16KiB均返回400且不建会话。源码：internal/campus/json.go/decodeJSONRequest、auth.go/login。TestLoginRejectsMalformedJSON与TestMySQLLoginJSONParameters已通过；本轮HTTP经Go测试入口和真实独立MySQL，不经Nginx，公开入口响应待用户复核。

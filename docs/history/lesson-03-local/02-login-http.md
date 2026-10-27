# 作业2：登录 HTTP 请求与响应

## 真实报文分析
本机URL：http://127.0.0.1:8080/api/login，方法POST，Content-Type: application/json，请求参数username/password。没有旧版POST /login或302服务端登录跳转。

请求体（口令脱敏）：
```json
{"username":"teacher_a","password":"[REDACTED]"}
```
实际成功状态200，无Location；前端随后主动导航/materials。真实响应体：
```json
{"id":1,"username":"teacher_a","role":"teacher","class_id":1,"class_name":"A 班"}
```
实际响应头摘要：
```http
Content-Type: application/json; charset=utf-8
Cache-Control: no-store
X-Content-Type-Options: nosniff
Set-Cookie: campusclaw_session=[REDACTED]; Path=/; Max-Age=28800; HttpOnly; SameSite=Lax
```
完整脱敏响应头（含真实Date与Content-Length）在http-evidence.json 的 login:teacher_a。当前本地HTTP未启用Secure；HTTPS配置启用Secure。
错误口令和未知用户名实际均401、相同正文 {"error":"账号或密码错误"}，无Set-Cookie；失败不会建立新会话。隔离测试还验证限流同401和同正文，非429。输入格式错误400，数据库依赖故障503。
Set-Cookie指示浏览器保存随机会话ID，后续同源请求自动携带Cookie；它不把role/class_id放进Cookie，也不返回密码哈希。

## 源码与结果
[auth.go](../../internal/campus/auth.go) login/cookie/sessionHash， [app.js](../../web/app.js) loginPage。
真实24项HTTP检查已通过；TestMySQLAuthSessions验证失败一致、换发与撤销。Nginx容器链路待安装Docker后重验；当前证据经过Go开发代理。

## F12自行截图
1. 打开/login，F12→Network，勾Preserve log，过滤login。
2. 提交账号密码，点击/api/login。
3. Payload→查看JSON请求体；截图前打码password。
4. Headers→General查看URL/Method/Status，再查看完整Response Headers；Set-Cookie值必须打码，属性保留。
5. Response查看用户JSON，另以错误密码请求查看401和错误正文。
6. 前端跳转页面也可能出现新的document请求，不要把它误当登录接口302。

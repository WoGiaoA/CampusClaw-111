# 作业3：登录后如何识别用户

1. 后端 bcrypt 校验成功，生成 32 随机字节的十六进制 Session ID 与 CSRF token；事务保存 sessions 的 id_hash、user_id、8 小时到期时间及 CSRF。id_hash 为用服务端 SESSION_KEY 计算的 HMAC-SHA256，数据库不保存可直接重放的 Session ID。
2. 响应 Set-Cookie。浏览器保存 campusclaw_session，之后同源请求自动携带 Cookie，无需反复发送密码。HttpOnly Cookie 不由前端 JavaScript 读取。
3. auth 中间件计算摘要，查询未过期 sessions 并关联 users/classes，每次取得当前用户、角色、class_id。/api/me 返回当前身份及 CSRF，前端据此显示上传入口，但真正授权仍在服务端。
4. 列表 SQL 用可信 class_id；详情、知识库及文件先取资源再检查班级。跨班与不存在都 404；教师才可上传，学生伪造请求也 403。客户端参数、JSON 或 X-Role/X-Class-ID 等自报身份不能覆盖用户表。
5. 登出删除当前会话；重新登录换发 ID；改密撤销该用户全部旧会话。首次改密状态在后端禁止材料/知识库访问。

源码：[auth.go randomToken/sessionHash/login/logout](../../internal/campus/auth.go)、[server.go auth](../../internal/campus/server.go)、[materials.go ownedMaterial/upload](../../internal/campus/materials.go)、[passwords.go changePassword](../../internal/campus/passwords.go)。这不是旧 Flask 签名 Cookie Session。

实际结果：[final-http.json](final-http.json) 验证四身份、摘要存储、同班跨用户读取 200、伪造 user_id/role/class_id 和请求头仍保持原身份/班级、学生上传 403、退出重放 401；[Go 输出](final-go-tests.txt) 的 TestMySQLAuthSessions 另验证过期、篡改、换发、DB 当前角色及统一失败。TestConcurrentLoginReset 验证凭据变更并发安全。

自行截图：F12 → Application → Cookies 看 Cookie 属性（遮住值）；Network 找 /api/me 和 /api/materials，展示请求头有 Cookie、没有再次发送密码。登录 A 班后请求 /api/me?user_id=2&role=teacher&class_id=2，确认身份不变；请求 /api/materials?class_id=2 仍只本班。按 B 班实际资源 ID 请求详情为 404。不同主机 localhost 与 127.0.0.1 的 Cookie 分开，整个过程统一用 127.0.0.1。所有 Cookie/CSRF 截图打码。

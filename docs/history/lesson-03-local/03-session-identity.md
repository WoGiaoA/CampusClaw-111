# 作业3：登录后如何识别身份

## 最终实现
1. POST /api/login 比对bcrypt。成功生成crypto/rand随机32字节Session ID及CSRF token，在事务内删除浏览器旧ID对应Session，并插入新行。
2. Cookie只保存随机ID，HttpOnly阻止普通JS读取；Path=/、SameSite=Lax，浏览器按作用域自动把它加在下一次请求头Cookie中。不存localStorage，不携带明文口令。
3. sessions只存HMAC-SHA256(SESSION_KEY,ID)摘要、user_id、expires_at、csrf_token。服务端收到Cookie计算摘要，查未过期会话并join users/classes；role/class_id每次以users为准。
4. 下游从请求context取得User，列表WHERE class_id，详情/下载先取行后比班级；跨班/不存在同404。
5. 请求体、参数、Header自行声明role/class_id不会被认证代码读取。/api/me只供前端渲染入口；teacher授权在上传处理器中再次校验，学生直接调用仍403。
6. 登出删除服务端行，旧Cookie重放立即401；重复登录换发，旧ID401。会话8h有效。写请求另校验X-CSRF-Token。
这不是Flask签名Cookie Session；Cookie不含身份载荷，服务端存状态并可即时撤销。

## 源码与证据
- [auth.go](../../internal/campus/auth.go)：randomToken/sessionHash/login/logout。
- [server.go](../../internal/campus/server.go)：auth统一查Session与users，context身份。
- [schema.sql](../../internal/campus/schema.sql)：sessions/users结构。
- [materials.go](../../internal/campus/materials.go)：班级和上传角色检查。
TestMySQLAuthSessions实际验证摘要存储、过期、伪造、换发、登出重放及DB角色即时变化；HTTP记录logout:old-cookie-replay为401。Cookie和CSRF值均未公开。

## 自行截图与验证
登录后F12→Application→Cookies，核对Cookie名称、HttpOnly、SameSite、Path，打码Value。
Network查看/api/me的用户身份和材料列表请求的Cookie Header；没有再次提交password。/api/me csrf_token也要打码。
在地址栏访问/api/materials?role=teacher&class_id=2，A班账号仍只返回A班；不能据参数切换班级。
登出后访问/api/me，应401。不要把完整Cookie复制到公开材料。

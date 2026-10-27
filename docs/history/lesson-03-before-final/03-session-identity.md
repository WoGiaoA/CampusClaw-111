# 作业3：登录后身份识别

> **第三课最终整理阶段状态：开发修正与隔离测试已通过，等待用户启动 Docker 并最终验收。** 下方原有容器运行结论与 JSON/TXT 文件属于 **第三课初次验收阶段的历史证据**，不是本轮重新部署结果。最新测试边界及操作顺序见 [本轮复验](lesson-03-regression.md)。截图全部由用户完成。

## 最终实现

bcrypt验证后，Go生成32字节随机Session ID和独立CSRF token。事务锁用户，确认密码哈希未并发改变，写sessions(id_hash,user_id,expires_at,csrf_token)。浏览器只存随机ID，数据库存HMAC-SHA256摘要。默认有效8小时。

后续Cookie自动发送；auth计算摘要，联合sessions/users/classes读取可信ID、role、class_id及must_change_password。客户端提交role/class_id不能替代数据库身份。班级列表SQL WHERE class_id，详情/下载先读取再比较，跨班与不存在同404。

初始密码账户只能me/password/logout，材料和知识库API在服务端403 password_change_required；前端进入/profile。普通改密校验当前密码、CSRF和强度；成功后删除该用户全部Session，旧Cookie401。登出也删除当前Session。注册仅student，个人页没有角色/班级写接口。

这是服务端状态会话，不使用Flask签名Cookie承载身份。

## 实际验证与源码

初始限制、改密两设备撤销、登出重放、跨班、伪造归属均经真实容器HTTP通过；过期/篡改、角色即时变化与并发凭据锁经Docker MySQL集成测试通过。

[auth.go](../../internal/campus/auth.go)、[server.go](../../internal/campus/server.go)、[passwords.go](../../internal/campus/passwords.go)、[schema.sql](../../internal/campus/schema.sql)、[HTTP记录](container-http-first.json)。

## 自行截图

1. 登录后Application → Cookies查看campusclaw_session的HttpOnly/SameSite/期限。
2. Network选择/api/me与材料请求，查看可信角色/班级及请求Cookie（值打码）。
3. 初始账号截图业务403和/profile流程；改密完成重新登录。
4. 用第二个登录窗口验证改密后旧会话失效。个人页只展示角色与班级。

## 本轮补充

TestMySQLAuthSessions、TestDemoResetFirstLogin、TestRegistrationPasswordManagement、TestConcurrentLoginReset重新通过，覆盖Session换发/撤销、数据库身份、首次限制及并发凭据。使用独立测试库，没有登录或修改业务账号。源码入口：auth.go/login、server.go/auth、passwords.go/changePassword。

# 账号、注册与密码

当前流程以代码及本文为准；历史账号状态见 history/lesson-03-before-final/project-README.md，不作为当前口令表。

四个预置账号：teacher_a / teacher_b 分别为 A/B 班教师，student_a1 / student_b1 分别为 A/B 班学生。已有用户以最后自行设置的密码登录；服务端仅保存 bcrypt 哈希，无法查询明文。私有 .env 的 SEED_*_PASSWORD 仅用于空库初始化，不会覆盖已有凭据。不要把当前个人密码写回 .env，也不要提交 .env。

本地演示模式 APP_ENV=local、DEMO_MODE=true 时可在 /register 注册。用户名 3–32 位英文字母、数字或下划线，选择 A/B 班；后端固定 student，初始密码 123456。首次登录进入 /profile，服务端在改密前禁止材料和知识库访问。生产默认禁止演示注册和重置，矛盾环境配置拒绝启动。

个人信息展示用户名、角色与班级。首次改密凭当前有效会话和 CSRF，无需再提交初始口令；普通改密必须校验当前密码。新密码 8–72 UTF-8 字节，含 ASCII 英文字母和数字，两次一致且不能等于当前密码或初始口令。提交成功后全部旧会话撤销，重新登录。页面不能修改角色和班级，附带权限字段的账号请求会被拒绝。

启动、重建、初始化都不重置已有用户。重置是单独管理员操作，会撤销旧会话；不要为验收执行。现有 scripts/reset-demo-passwords.ps1 只针对历史 Windows 备用 MySQL，不能用来重置当前 Docker 账号。本轮没有重置任何真实用户。

源码：[密码管理](../internal/campus/passwords.go)、[登录](../internal/campus/auth.go)、[身份中间件](../internal/campus/server.go)、[迁移](../internal/campus/db.go)、[页面](../web/app.js)。验证：[密码测试](../internal/campus/passwords_test.go)、[最终测试输出](../deliverables/lesson-03/final-go-tests.txt)。注册、首次限制、改密失败、全部会话撤销、并发与生产禁用均在隔离库测试通过。

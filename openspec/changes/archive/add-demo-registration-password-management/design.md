# Design

## Context
现有 Go/MySQL/数据库 Session 与 SPA。users 尚无改密状态，启动 Seed 对已有用户不操作；材料与知识库不能被重置影响。见 proposal.md。

## Goals / Non-Goals
目标是增加本地演示凭据生命周期，保留身份、班级和业务数据。排除生产默认密码、公开教师创建、找回密码及改班级接口。

## Decisions
- APP_ENV 默认 production；DEMO_MODE 默认 false。只有 APP_ENV=local 且 DEMO_MODE=true 才启用注册/重置；矛盾配置启动拒绝。本机启动脚本明确设置这两个变量，Compose 环境默认关闭。不通过浏览器传参启用。
- users 增加 must_change_password BOOL DEFAULT false，information_schema 检查后 ALTER，兼容现有表。密码仍仅存 bcrypt。不用比较前端字符串判断状态，也不把密码存数据库。
- GET /api/account-options 仅返回开关和 A/B 班级；POST /api/register JSON username/class_code，严格拒绝多余字段，用户名 ASCII 3–32 位字母数字下划线。只选择已有 A/B，后端写死 student，bcrypt 独立盐，状态 true，成功 201 不含密码/哈希。页面在本地模式静态提示 123456。
- /api/me 返回 must_change_password；POST /api/password JSON current_password/new_password/confirm_password，必须 Session 与 CSRF。首次流程无需旧密码，普通用户必须提供旧密码。新密码 8–72 UTF-8 字节，至少 ASCII 字母和数字，禁止 123456 和当前密码。
- 首次状态只允许 me/password/logout；所有材料/知识库 API 返回 403 password_change_required；SPA 登录转 /profile，材料页也检查后转向。个人信息仅展示，拒绝 role/class_id 等额外写入字段。
- 改密事务锁用户、重新校验当前状态及会话仍存在，更新哈希/状态并删除该用户全部 Session。登录建会话前锁用户并确认哈希未被并发修改，避免重置后创建旧凭据会话。
- 显式 reset-demo-passwords CLI 子命令：校验模式和 MySQL loopback 地址、四账号齐全，事务锁定、逐个 bcrypt、设强制改密、删除全部会话。仅更新四账号凭据，无文件操作。配套脚本只执行该命令；启动绝不调用。

## Risks / Trade-offs
- 默认口令可猜 → 本地双开关、loopback 入口、强制改密、生产禁用；不适合开放到校园网。
- MySQL DDL 自动提交 → 增量列不删表，失败即停止；回滚代码前需保留强制改密限制，不能退回旧二进制绕过。
- 会话撤销影响所有设备 → 这是预期安全行为，成功后重新登录。

## Migration Plan
先验证新增列兼容及独立 MySQL 测试；停止本项目 API、构建并启动；记录业务行/文件校验和，显式执行一次重置并核对数据保留及旧会话失效。原始 .env 随机种子值保留但不再代表已重置账号的当前密码，README 必须说明。不自动重置、不删数据库。

## 第三课最终回归
注册、首次改密、普通改密、会话撤销、并发与生产禁用重新在独立测试库验证通过，见 deliverables/lesson-03/final-go-tests.txt。历史本机重启/重置为当时操作，不代表本轮再次执行；本轮没有修改当前 Docker 真实账号。当前账号说明 docs/account-management.md，现行部署为 Docker Compose。

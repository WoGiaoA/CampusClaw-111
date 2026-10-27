# Proposal

## Why
课堂演示逐个查随机口令不便。需要本地默认初始口令、学生注册及强制首次改密，同时保留可信 Session、班级边界与现有数据。

## What Changes
- 显式本地 CLI 将四个指定账号重置为 bcrypt 初始口令、强制改密并撤销全部会话；不随启动重置。
- 演示模式公开学生注册，用户名和 A/B 班级；注册后页面说明初始口令，API 不返回口令。
- 增加个人信息和 CSRF 保护的改密 API；改密前拒绝材料访问，成功后撤销全部会话并重新登录。
- APP_ENV=local 与 DEMO_MODE=true 双开关；生产禁止默认口令注册与批量重置。

## Capabilities
### New Capabilities
- `account-password-management`: 演示注册、显式重置、个人信息、首次改密及会话撤销。
### Modified Capabilities
无已归档能力。旧 add-auth-rbac-class-knowledge 仍在实施；本次能力增补其登录后的密码状态限制，并取代旧 proposal 中“不实现注册”的本次范围约束，保留其第三课历史。

## Impact
Go users 增量字段、身份与登录事务、注册/改密 API、SPA 页面、本地 CLI/配置、测试与 README。材料和知识库结构不变，不删除重建数据库。

## Non-goals
找回密码、教师公开注册、角色/班级自助修改、邮件/SSO、AI、作业业务和本次 Docker 运行验收。

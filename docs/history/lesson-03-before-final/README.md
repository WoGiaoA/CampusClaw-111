# 第三课最终容器验收与截图索引

> **第三课最终整理阶段状态：开发修正与隔离测试已通过，等待用户启动 Docker 并最终验收。** 下方原有容器运行结论与 JSON/TXT 文件属于 **第三课初次验收阶段的历史证据**，不是本轮重新部署结果。最新测试边界及操作顺序见 [本轮复验](lesson-03-regression.md)。截图全部由用户完成。

第三课初次验收阶段。主系统 http://127.0.0.1:8080/login，实际 Nginx → Go → MySQL，三服务 healthy。Windows CMD 标准命令见项目 README；无需 PowerShell 7。原本机证据归档 docs/history/lesson-03-local。

| 作业 | 实际验收 | 用户自行截图 |
| --- | --- | --- |
| [1 系统运行](01-system-running.md) | Compose、三服务健康、原数据迁移及教师流程通过 | compose ps、教师材料页 |
| [2 登录 HTTP](02-login-http.md) | 主入口真实登录 200/失败 401，JSON 和 Set-Cookie 已记录 | F12 Payload、Headers、Response |
| [3 Session 身份](03-session-identity.md) | 数据库 Session、首次改密、角色班级可信、撤销通过 | Application Cookie 属性、/api/me、改密后旧会话拒绝 |
| [4 上传](04-upload-analysis.md) | txt/md 201，类型/编码/大小/CSRF/角色拒绝、事务故障通过 | 上传请求、成功列表、拒绝 PDF |
| [5 下载权限](05-download-permission.md) | 本班 200、匿名 401、跨班/无效 404、公开存储拒绝 | 下载与拒绝请求响应 |
| [6 API 暴露](06-backend-exposure.md) | 仅 web 发布，Nginx 转发、health 200/503/unhealthy、卷重建通过 | compose ps、同源 API、health |

账号当前状态：teacher_a 已设置个人密码，完整保留；teacher_b/student_a1/student_b1 为 123456 且首次须改密。独立验收项目验证了四初始账号、注册及普通改密，不覆盖主库个人密码。

## 实际证据

- [主入口登录](primary-login-http.json)：三个初始账号及 HTTP 响应，密码/Cookie 脱敏。
- [91 项容器请求](container-http-first.json)、[28 项重建后请求](container-http-after.json)。
- [容器服务与真实端口](container-services.json)、[故障健康](container-health-faults.json)、[持久化](container-persistence.json)。
- [13 项 Go 测试](container-go-tests.txt)、[8 项页面逻辑](container-ui-tests.txt)。
- [测试汇总](test-results.md)、[34 个 Scenario](scenario-results.md)。

所有功能测试使用实际 Docker 容器；改密/上传主要在独立 campusclaw-acceptance 项目，主库迁移保留4用户、6材料、6知识库及全部原文件。测试项目停止且卷保留，没有 down -v。无截图或图片生成。

截图中真实密码、完整 Cookie、CSRF、.env 和 Docker 容器完整环境变量必须打码；仅截图 compose ps 等不含秘密的输出。

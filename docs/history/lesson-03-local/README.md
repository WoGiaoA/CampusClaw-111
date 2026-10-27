# 第三课交付与截图索引

> 后续独立账号变更已增加注册与首次改密，当前四账号初始口令为 123456；旧 HTTP 证据保留作为第三课当时验收记录。当前流程见 [账号与密码](../../docs/account-management.md)。

第三课初次验收阶段。系统已实现，当前本机入口 http://127.0.0.1:8080/login，采用真实 Go/MySQL，前端通过 Go 开发代理同源调用 API。Docker Desktop/WSL 由用户安装中，容器验收明确待验证。

| 作业 | 状态 | 自行截图位置 |
| --- | --- | --- |
| [1 系统运行](01-system-running.md) | 本地运行/教师登录通过；Compose待验 | teacher_a 登录后的 /materials |
| [2 登录HTTP](02-login-http.md) | 真实请求通过，响应已脱敏 | F12 Network 登录 Payload/Headers/Response |
| [3 Session身份](03-session-identity.md) | 创建/换发/过期/撤销通过 | Application Cookie 属性；/api/me 与下一次列表请求 |
| [4 上传](04-upload-analysis.md) | txt/md、拒绝类型、失败回滚通过 | 教师上传成功与 PDF 拒绝请求 |
| [5 下载权限](05-download-permission.md) | 本班200、未登录401、跨班/无效404通过 | 下载请求、无登录和跨班请求响应 |
| [6 API暴露](06-backend-exposure.md) | 配置分析/本地请求通过；容器端口待验 | Docker就绪后 compose ps、/health 和同源 API |

没有生成任何截图、图片或浏览器截图环境。密码、完整 Cookie、CSRF、密钥截图必须打码。

## 证据
- [真实HTTP记录](http-evidence.json)：24项，实际请求，Set-Cookie/CSRF与密码已脱敏，明确不是Docker结果。
- [测试结果](test-results.md)：真实MySQL隔离集成、单元与DOM检查。
- [Scenario 对照](scenario-results.md)：逐条列出证据；容器相关 pending。
- [规约基线调整](../../docs/lesson-03-baseline.md)：完整教程来源、差异与历史保存位置。

## Docker安装后继续
确认 Docker Desktop Linux containers 可运行，在项目根执行 init-env.ps1（不覆盖）、docker compose up --build -d；按06验证仅web发布8080，再使用独立项目名/测试.env及独立卷验收down/up持久化。不要删除当前开发数据，不执行down -v，不把配置正确当运行通过。完成后更新 tasks 6.3/6.4 及剩余 Scenario。

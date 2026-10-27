# 账号与密码管理交付

最终容器补充：实际Compose验收已完成，见 [第三课最终记录](../lesson-03/README.md)。此处47项本机检查为此前历史阶段；当前teacher_a已个人改密，迁移保留，其余三账号保持初始流程。独立Compose再次验证四初始账号、注册和普通改密，不重置真实个人密码。

第三课初次验收阶段，本机系统已升级并显式执行一次重置。访问 http://127.0.0.1:8080/login；四账号 teacher_a、teacher_b、student_a1、student_b1 的初始密码均为 123456，第一次登录必须改密。首次改密后仅新密码有效，之后启动不会恢复默认口令。

## 实际结果

| 检查 | 结果与证据 |
| --- | --- |
| Go 自动化 | 13 个顶层测试通过，其中 9 个真实隔离 MySQL 集成测试；0 失败、0 跳过；[原始输出](test-output.txt) |
| 页面 | 8 项 DOM/VM 测试通过，无截图环境 |
| 构建 / vet / JS | 编译、go vet、node --check 成功 |
| 真实本机 HTTP | 47 项通过；[脱敏结果](http-results.json) |
| 页面及健康 | /login、/register、/profile、/health 实际 200 |
| 原有数据 | 重置前后业务行快照一致；4 用户的 ID/角色/班级、5 材料、5 知识库和原文件 SHA-256 全部保留 |
| 重置存储 | 四个独立 bcrypt 哈希，4 个首次标志；明文密码记录 0，旧 Session 0 |
| Nginx / Docker | Nginx Host 保留入口端口，配置语法检查；Docker 实际验收继续沿用第三课待验证状态 |
| OpenSpec | 新变更严格校验 valid=true、issues=[]，7/7 任务完成 |

真实 HTTP 验证只对临时验收学生执行首次及普通改密，没有消耗四预置账号的首次流程。临时学生与其 Session 随后清理，只清理本次随机命名测试账号。旧 Session 证据临时存私有忽略目录，验证后已删除，交付不含密码、Cookie、CSRF 或哈希值。

## 六项验收对照

1. 四账号 123456 登录成功、must_change_password=true，直接业务 API 403；旧密码/会话 401。真实 HTTP 和 TestDemoResetFirstLogin。
2. 首次改密后 123456 登录失败、个人密码成功，全部旧会话撤销。隔离测试与真实临时学生请求。
3. A/B 学生注册 201、角色固定 student，首次改密；重复/额外权限字段拒绝。TestRegistrationPasswordManagement、页面与真实 HTTP。
4. 普通改密须当前密码和 CSRF；新密码成功旧密码失败，多设备会话失效。隔离与实际 HTTP。
5. 注册不能创建教师，个人信息不能改班级/角色；改密后学生上传依然 403，原双班隔离/下载测试通过。
6. 增量迁移及实际重置未改原有身份/材料/知识库/文件；启动/Seed/Migrate 不重置已修改密码。

## Scenario 对照（9 项）

| Scenario | 结果 | 依据 |
| --- | --- | --- |
| 生产禁用 | PASS | TestProductionAndIncrementalMigration；生产 API 403、CLI 拒绝、冲突开关失败 |
| 四账号重置与数据保留 | PASS | TestDemoResetFirstLogin；本机快照和 47 HTTP |
| 启动不重置 | PASS | TestDemoResetFirstLogin；改密后 Seed/Migrate，再用新密码登录成功 |
| 注册学生 | PASS | TestRegistrationPasswordManagement、真实 B 班 HTTP、DOM |
| 防止提权及非法输入 | PASS | 隔离测试与实际额外 role/class_id 拒绝 |
| 首次登录限制 | PASS | 全部材料/知识库 API 403、首次导航 DOM |
| 修改成功及全部会话撤销 | PASS | 首次和普通改密测试、真实 HTTP |
| 错误输入及越权 | PASS | 401/403/400，弱密码/不一致/旧密码/权限字段测试 |
| 凭据并发安全 | PASS | TestConcurrentLoginReset；强制在旧密码校验成功后完成重置，继续登录返回 401 |

本次不扩大第三课 Docker 已有未完成项。

## 修改文件与复验

后端：[passwords.go](../../internal/campus/passwords.go)、[auth.go](../../internal/campus/auth.go)、[server.go](../../internal/campus/server.go)、[config.go](../../internal/campus/config.go)、[schema.sql](../../internal/campus/schema.sql)、[db.go](../../internal/campus/db.go)、[CLI](../../cmd/api/main.go)。

页面：[app.js](../../web/app.js)、[index.html](../../web/index.html)、[开发路由](../../cmd/devweb/main.go)。

环境/脚本：[Compose](../../docker-compose.yml)、[Nginx](../../deploy/nginx.conf)、[环境示例](../../.env.example)、[本机启动](../../scripts/start-local.ps1)、[显式重置](../../scripts/reset-demo-passwords.ps1)、[实际请求验证](../../scripts/verify-demo-accounts.cjs)。

测试：[passwords_test.go](../../internal/campus/passwords_test.go)、[页面测试](../../tests/ui.test.cjs)。完整使用方式见 [账号说明](../../docs/account-management.md)；规约在 openspec/changes/add-demo-registration-password-management。

当前容器复验证据和方法见第三课最终交付。本机 ps1 测试仅供备用环境，可在 CMD 用内置 powershell.exe -File 调用，无需 PowerShell7。reset-demo-passwords.ps1 只针对本机备用库且显式执行，不应用它重置容器数据。verify-demo-accounts.cjs before/after 是此前操作记录，before 假设账号尚使用种子口令，不能在已重置后照搬。

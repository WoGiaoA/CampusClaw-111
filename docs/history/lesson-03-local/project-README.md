# CampusClaw

价值：面向高校班级教学的材料与知识库平台。
场景：教师上传 txt/md，教师与学生只查看、下载本班材料。
不做：AI 对话、RAG、模型、作业流程、技能业务及完整演示复刻。后续独立变更新增本地学生注册与密码管理。

现行技术：Go net/http + MySQL 8.4 + 服务端 Session + Nginx SPA。第二课历史仅在 docs/history/lesson-02，现行规约在 openspec/changes/add-auth-rbac-class-knowledge。

## 标准 Compose 启动（待 Docker 安装后验收）

在本目录 PowerShell：

```powershell
pwsh -File scripts/init-env.ps1
docker compose up --build -d
docker compose ps
curl.exe -f http://localhost:8080/health
```

入口 http://localhost:8080/login，登录后 /materials。仅 web 发布 loopback 8080；api 与 MySQL 不发布宿主机端口。api 使用随机 Session ID Cookie，数据库只存其摘要。

停止/重启：docker compose down，再 docker compose up -d。保留数据卷，不执行 down -v；删除卷会丢账号、材料与文件。启动不自动迁移开发库数据。

## 本机非 Docker 开发环境（已验证）

本次已在忽略目录准备官方 Go 1.27.2、MySQL 8.4.9。运行：

```powershell
pwsh -File scripts/start-local.ps1
```

入口 http://127.0.0.1:8080/login。当前使用 Go 开发代理，不是 Nginx；数据库 127.0.0.1:13306、API 127.0.0.1:18081 仅本地绑定。这些开发端口与 Compose 网络隔离不能混为一谈。

MySQL 因 Windows 中文路径限制，独立数据目录是 $env:TEMP/campusclaw-local-mysql-data，工具目录 $env:TEMP/campusclaw-mysql-tools；上传在 .local/uploads。不要用系统清理工具删除这些目录，否则开发数据会丢失。标准部署改用 named volumes。

停止本机开发进程：pwsh -File scripts/stop-local.ps1，数据保留。脚本不注册系统服务。

## 当前本地账号与凭据

四个预置账号已显式重置初始密码为 **123456**，首次登录必须进入 `/profile` 修改密码，才能访问材料。新密码为 8–72 字节，包含字母和数字。登录页可注册 A/B 班学生，同样首次改密；个人信息页支持普通改密，成功后全部会话退出。

启动不会重置密码。如确实需要再次重置四个账号，显式执行 `pwsh -File scripts/reset-demo-passwords.ps1`。详情见 [账号与密码说明](docs/account-management.md)。

| 账号 | 身份/班级 | 空库首次种子变量（不代表重置后的当前密码） |
| --- | --- | --- |
| teacher_a | 教师/A | SEED_TEACHER_A_PASSWORD |
| teacher_b | 教师/B | SEED_TEACHER_B_PASSWORD |
| student_a1 | 学生/A | SEED_STUDENT_A1_PASSWORD |
| student_b1 | 学生/B | SEED_STUDENT_B1_PASSWORD |

scripts/init-env.ps1 随机生成服务端密钥和空库种子口令，不覆盖已有 .env。现有账号重置或改密后不使用这些种子口令，个人密码不应写回 .env。首次种子写入后改 .env 不会重置数据库密码或用户哈希。SESSION_KEY 不可缺省；本地 HTTP Secure=false，部署 HTTPS 时设置 true。

本机启动脚本设置 APP_ENV=local 与 DEMO_MODE=true。Compose 默认 production/false，禁止默认口令注册和批量重置；生产加演示开关会启动失败。

## 测试与作业

```powershell
pwsh -File scripts/test.ps1 -Integration
node scripts/http-verify.cjs
```

集成测试只创建并清理随机 campusclaw_test_* 数据库，使用临时上传目录，不操作运行中的业务库。无 MYSQL_TEST_ADDR 时明确跳过 MySQL 测试；此跳过不能当通过。test.ps1 -Integration 指向本地独立 MySQL。

Go 单元/真实 MySQL 集成、DOM 测试、24 次真实 HTTP 检查已通过；具体证据与待验范围见 [第三课作业](deliverables/lesson-03/README.md)。Docker 实际启动、容器健康/端口发布和卷重建尚待安装后验证。

状态约定：未登录 API 401，学生上传 403，跨班/不存在 404，合法上传 201。文件 5 MiB 上限，仅 txt/md UTF-8，PDF/DOCX/图片不解析。六项作业截图全部由用户自行完成。

# CampusClaw

最新前端：[全站UI/UX视觉优化与测试记录](docs/lesson-03-ui-refresh.md)。仅修改静态前端，刷新页面即可，无须重新构建或重启Docker。


本轮交接：[第三课修正、测试与用户验收](deliverables/lesson-03/lesson-03-regression.md)。已迁至英文路径，Docker启动及最终验收由用户完成；本轮未启动Docker或重置账号。16个Go测试（含10个独立MySQL/真实HTTP集成）和10项页面逻辑通过。

全新空库的种子口令由init-env随机生成，只在自己的.env查看；下方123456是历史已重置演示账号状态，不适用于所有新库或已改密账号。启动不会重置已有口令。

高校班级材料与知识库平台。Go + MySQL 8.4 + 数据库 Session + Nginx，同源 /api，Compose 是标准运行方式。教师上传 UTF-8 txt/md，师生只读下载本班文件；无 AI/RAG/作业业务。第三课初次验收阶段 有容器验收记录；第三课 开发修正及隔离测试通过，等待用户自行启动 Docker 并最终验收。

## Windows CMD 启动

Docker Desktop 必须运行 Linux containers。本项目已有私有 .env，不要覆盖。

    cd /d "D:\PKU\Projects\campusclaw-111"
    scripts\init-env.cmd
    docker compose up --build -d --force-recreate --wait
    docker compose ps
    curl.exe -f http://127.0.0.1:8080/health

init-env.cmd 仅在 .env 不存在时生成随机秘密，使用 Windows 内置 powershell.exe，不需要 PowerShell 7。不打印或共享 .env。当前本地文件已明确设置 APP_ENV=local、DEMO_MODE=true；示例及生产默认 production/false，禁止默认口令注册与批量重置。

访问 [登录](http://127.0.0.1:8080/login)、/register、/profile、/materials。只发布 Nginx 到 127.0.0.1:8080，Go API 与 MySQL 没有宿主机端口；文件卷仅 api 挂载。

停止（保留容器和卷）：

    docker compose stop

重新启动停止的系统：

    docker compose up -d --wait

重启运行中的系统：

    docker compose restart
    docker compose up -d --wait

重新构建：docker compose up --build -d --wait。down 后正常 up 也保留卷，已在独立项目实测。**不执行 down -v，不删除 volumes**，删除卷会丢失数据库与上传。

## 最新账号操作（第三课）

用户授权后，仅在当前Docker数据库将teacher_a重置为课堂初始密码123456，并启用首次改密、撤销其旧会话。公开Nginx入口登录200及首次标记已验证，验证会话已退出；其他账号、教学数据和上传文件摘要核对不变。下表为此前历史状态，teacher_a当前以本节为准，用户再次改密后以新密码为准。
## 历史账号状态（第三课初次验收阶段）

| 账号 | 身份 | 当前密码状态 |
| --- | --- | --- |
| teacher_a | A 班教师 | 您已设置个人密码，迁移完整保留；不再是 123456 |
| teacher_b | B 班教师 | 初始密码 123456，首次登录必须改密 |
| student_a1 | A 班学生 | 初始密码 123456，首次登录必须改密 |
| student_b1 | B 班学生 | 初始密码 123456，首次登录必须改密 |

新注册仅学生，选择 A/B 班，初始密码 123456，先改密再访问材料。新密码 8–72 字节并含字母、数字。个人信息只能看角色/班级，不能修改；普通改密要当前密码和 CSRF，改密撤销所有旧 Session。初始状态由服务端拦截业务接口。页面说明初始口令，API/日志不输出密码。

SEED_*_PASSWORD 只用于空库初始化，不能用来查已重置或个人修改后的密码。启动不重置现有账号。现行细节见 [账号说明](docs/account-management.md)。

## 数据、镜像和备用运行

主卷 campusclaw_db-data、campusclaw_uploads 已从原本机完整迁入：4 用户、6 材料、6 知识库和6原文件，含个人 bcrypt 哈希和首次状态；迁移前后逐行与文件 SHA256 一致。原 Windows MySQL 数据与 .local/uploads 仍保留，原服务已关闭。只导入确认空的新卷，不会在每次启动再次导入。

Docker Hub 认证网络超时已通过 [AWS 公共 ECR 的 Docker Official Images 副本](https://aws.amazon.com/blogs/containers/docker-official-images-now-available-on-amazon-elastic-container-registry-public/) 绕过。镜像版本不变；IMAGE_REGISTRY 可配置，不改全局 Docker 配置。

本机非 Docker 仅备用：先停止 Compose，再执行以下 CMD 命令。备用使用保留的 Windows 旧库，不会自动同步后续容器中新数据，勿用它覆盖容器卷。

    powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts\start-local.ps1
    powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts\stop-local.ps1

## 验收证据

[六项作业](deliverables/lesson-03/README.md)：13 个 Go 测试（9 个真实 Docker MySQL 集成）、8 个页面测试、91+28 个实际容器 HTTP 检查、主入口登录、实际 unhealthy 故障及卷重建、端口隔离均通过。测试使用独立 campusclaw-acceptance 项目，没有修改主库个人密码或材料。测试项目停止但保留数据卷。

规约：add-auth-rbac-class-knowledge 与 add-demo-registration-password-management，历史在 docs/history。截图由用户完成，密码、Cookie、CSRF 须打码；没有生成截图。
# CampusClaw

第四课作业已准备为[OpenSpec变更描述](deliverables/lesson-04/README.md)：限定班级范围的知识库检索与内容溯源。当前仅完成规划文档及校验，尚未实施；以下仍描述已完成的第三课系统。

《互联网软件开发技术与实践》第三课个人项目。教师上传教学文本，师生在所属班级查看正文和下载原文件。采用 Go net/http、MySQL 8.4、数据库 Session、原生 HTML/CSS/JavaScript、Nginx 和 Docker Compose，与第三课的服务端认证、班级隔离及持久化路线一致。

第三课最终功能验收通过：17 项 Go 测试（含真实 MySQL/HTTP 集成，0 跳过）、73 项隔离 HTTP 检查、15 项前端测试，以及当前公开入口 12 项检查。截图由本人自行准备。见[第三课交付索引](deliverables/lesson-03/README.md)和[验收记录](deliverables/lesson-03/lesson-03-acceptance.md)。

## Windows CMD 启动

需要 Docker Desktop 运行 Linux containers。已有 .env 不覆盖；新环境由脚本生成随机密钥和种子口令，仅在自己电脑查看。

    cd /d D:\PKU\Projects\campusclaw-111
    scripts\init-env.cmd
    docker compose up --build -d --wait
    docker compose ps
    curl.exe -f http://127.0.0.1:8080/health

访问 [登录](http://127.0.0.1:8080/login)。仅 Nginx 发布宿主机 127.0.0.1:8080；api 和 db 仅在 Compose 网络内通信。生产默认关闭演示注册；本地课堂演示需显式 APP_ENV=local、DEMO_MODE=true。HTTP 演示 SESSION_COOKIE_SECURE=false；HTTPS 部署应开启。

日常启动已构建的服务用 docker compose up -d --wait；后端代码更新用上述 --build。前端由只读目录挂载，刷新即可。停止用 docker compose stop。数据库与文件保存在 db-data、uploads 命名卷，不能删除卷或执行 docker compose down -v。8080 被 Litera 等项目占用时先停止占用项目，或在自己的 .env 调整 WEB_PORT 并使用相应网址；不需要反复修改整套 Docker 配置。

## 账号与使用

| 预置账号 | 班级 | 权限 |
| --- | --- | --- |
| teacher_a / teacher_b | A / B | 上传、查看、下载本班材料 |
| student_a1 / student_b1 | A / B | 查看、下载本班材料，不能上传 |

密码以用户最后设置的密码为准，不能从哈希还原。.env 的 SEED_*_PASSWORD 只适用于空库初次初始化；重启或编辑 .env 不会重置已有密码。历史重置记录不代表当前密码。详见[账号与改密说明](docs/account-management.md)。

登录后进入 /materials；首次密码账号先进入 /profile 改密，再重新登录。本地演示 /register 只创建 A/B 班学生，初始口令 123456，必须改密；生产不提供该流程。个人信息入口显示可信身份，普通改密需当前密码。新密码 8–72 UTF-8 字节，含英文字母与数字，确认一致且不同于旧密码；修改后全部旧会话失效。

教师选择 UTF-8 txt/md（不超过 5 MiB、正文非空），可选标题，点击上传。材料列表提供查看正文与下载。学生不显示上传表单；直接调用 API 仍由后端拒绝。跨班读取与不存在 ID 均返回 404；同班不同用户共享材料是预期行为。

## 架构与目录

浏览器 → Nginx:80 → api:8080 → MySQL:3306；Nginx 同源转发 /api/ 与 /health，静态页面不包含业务数据。密码 bcrypt，Cookie 仅随机会话 ID，数据库保存 HMAC-SHA256 摘要；每次请求查会话及 users 的角色/班级，写请求另验 CSRF。

| 路径 | 内容 |
| --- | --- |
| cmd/api、cmd/devweb | Go 正式入口、备用本地静态代理；cmd 是必要源码目录 |
| internal/campus | 认证、权限、数据库、材料、文件与测试 |
| web | 无大型框架的前端，直接静态服务，无打包构建步骤 |
| deploy/nginx.conf、Dockerfile、docker-compose.yml | 标准容器部署 |
| scripts | 初始化环境及开发测试脚本 |
| openspec/specs | 四项当前主规格 |
| openspec/changes/archive | 已完成变更及原设计/任务历史 |
| deliverables/lesson-03 | 六项答案、逐场景记录、脱敏测试证据及截图步骤 |
| docs/history | 第二课基线及最终整理前的文档快照 |

## 开发与测试

[测试说明](docs/testing.md)区分隔离集成、前端 DOM、公开 Nginx 和历史容器持久化证据。普通开发入口（Go、Node.js 及 jsdom 就绪）：

    npm install --prefix .local/ui --no-save jsdom@30.1.2
    powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts\test.ps1

默认脚本明确跳过 MySQL，不算完整验收。全量隔离脚本还要求 .local/tools 下便携 MySQL：

    powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts\test-isolated.ps1

无需 PowerShell 7。当前最终 Go 测试是在已运行 db 容器内使用独立测试库执行，方法与输出见测试说明。旧 start-local.ps1/stop-local.ps1 仅备用：使用历史 Windows 数据，不会自动与容器数据同步，不能用于覆盖当前卷。旧 reset-demo-passwords.ps1 针对备用库，不是当前 Docker 重置入口。

## 范围与限制

仅 txt/md 全文保存与入库，无 PDF/DOCX/图片解析、AI、RAG、在线编辑或材料删除。lectures/assignments/assistants/skills 只预留结构。使用单实例文件对账与进程内登录限流；代理后的限流按后端连接来源聚合，属于教学部署限制，非完整生产风控。没有账号找回或自主修改角色班级的功能。

第三课初次验收阶段的独立容器 down/up、存储故障与迁移证据保留；第三课最终整理阶段没有重启或重建主服务，也没有改真实账号/教学材料。本轮处理的是验收、测试与文档，UI 保持已完成状态。全部结论的验证范围见交付记录。

最新顶栏尺寸见[品牌比例调整](docs/lesson-03-header-scale.md)：68px顶栏、桌面品牌22px、副标题11px，取代此前50px视觉参数。

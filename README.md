# CampusClaw

CampusClaw 是面向班级教学的 AI 应用平台，也是《互联网软件开发技术与实践》的个人课程项目。教师管理本班应用、资料和工具，学生通过多轮流式对话获得提示，并查看知识库引用和工具执行过程。权限由服务端按角色、班级和会话归属校验。

采用 Go net/http、MySQL 8.4、原生 HTML/CSS/JavaScript、Nginx、Qdrant 和 Docker Compose；通过兼容接口连接 Chat/Embedding 模型，通过 JSON-RPC 2.0 接入自建 MCP 服务。

## 主要功能

| 功能 | 实现 |
| --- | --- |
| 多个 AI 应用 | 内置“课程资料问答”和“解题引导”，支持新增、配置、启停及删除 |
| 提示词模板 | 班级共享模板与应用提示词分别管理，绑定后参与模型请求 |
| 知识库检索 | UTF-8 TXT/MD上传、切片、关键词/向量/混合检索，回答保留引用及原文入口 |
| Skill | 内置解题引导，支持文本技能上传、配置、启停与应用绑定；按问题匹配并注入规程 |
| 外部工具与 MCP | 自建服务提供formula_hint、strategy_hint；支持连接、发现、应用授权、串联调用及审计 |

## 首次启动（Windows CMD）

需要 Docker Desktop 运行 Linux containers，以及 Python 3.9及以上。在项目根目录执行：

```cmd
scripts\init-env.cmd
notepad .env
```

脚本仅在`.env`不存在时生成数据库密码、四账号初始密码、Session密钥、MCP令牌及加密密钥。填写自己的模型配置：

| 用途 | .env变量 |
| --- | --- |
| Chat | CHAT_BASE_URL、CHAT_MODEL、CHAT_API_KEY |
| Embedding | EMBEDDING_BASE_URL、EMBEDDING_MODEL、EMBEDDING_API_KEY、EMBEDDING_DIMENSION |

Base URL使用兼容接口的基础地址，模型名称和Embedding维度以接口提供方为准。然后执行：

```cmd
docker compose --profile mcp up --build -d --wait
docker compose ps
curl.exe -f http://127.0.0.1:8080/health
python apitest\run_all.py --init-demo
```

最后一条命令通过正式API为A班导入随仓库提供的13份知识库资料、复用/导入3项Skill、完成应用绑定，并连接本地MCP、创建及授权“MCP工具验收”应用。**无需注册或手动逐份上传、绑定。**B班保留基础内容用于隔离对照。重复执行复用相同内容；遇到同名内容冲突会停止，保留已有数据。已有账号改过密码时的处理见[教师指南](docs/teacher-quickstart.md)。

初始化不发送Chat请求；上传后的后台索引会使用已配置的Embedding。没有模型配置仍可登录、导入资料、管理应用/Skill和发现MCP工具；检索及AI问答需要有效配置和就绪索引。若资料此前索引失败，补齐配置、重新启动服务后由教师重建失败索引。

## 登录与核对

访问[登录页面](http://127.0.0.1:8080/login)，密码从测试者本机`.env`读取：

| 账号 | 身份 | 初始密码变量 |
| --- | --- | --- |
| teacher_a | A班教师 | SEED_TEACHER_A_PASSWORD |
| student_a1 | A班学生 | SEED_STUDENT_A1_PASSWORD |
| teacher_b | B班教师 | SEED_TEACHER_B_PASSWORD |
| student_b1 | B班学生 | SEED_STUDENT_B1_PASSWORD |

密码是变量的值，不是变量名；空库初始化后，重启或修改SEED变量不会重置已有密码。教师可检查应用、模板、资料绑定、Skill及MCP连接；学生可使用以下问题核对功能：

- 课程资料问答：“根据资料，星桥班期末项目要提交哪些内容？”继续追问并展开来源。
- 解题引导：“青竹验收：方程x²-5x+6=0应该怎样开始分析？”查看命中Skill和下一步提示。
- MCP工具验收：“MCP验收：方程x²-5x+6=0应该怎样开始分析？请先使用公式提示工具，再根据返回的策略键使用策略提示工具，只给下一步提示，不给最终答案。”核对两个工具过程及教师审计。

样例中的分组、截止时间和评分规则均为虚构测试语料，不代表本课程要求。详细操作见[教师首次测试指南](docs/teacher-quickstart.md)；账号规则见[账号管理](docs/account-management.md)。

## 接口文档与自动化测试

服务启动后可打开[Swagger](http://127.0.0.1:8080/docs/swagger/)或[OpenAPI JSON](http://127.0.0.1:8080/docs/openapi.json)，登录后使用Bearer Token试调接口；写请求还需CSRF Token。

```cmd
python apitest\run_all.py
```

**无参数命令只执行隔离测试；`--init-demo`才初始化本地运行站点。**隔离测试需本机Go及便携MySQL，工具准备方法和可选Chrome检查见[apitest说明](apitest/README.md)。测试按URL/HTTP方法组织，通过HTTP及数据库断言检查认证、检索、应用、模板、Skill、会话和MCP等接口。

最近一次隔离接口测试29/29通过，包含同一演示初始化器连续执行两次，核对资料/Skill绑定、工具授权及资源不重复。Chat/Embedding使用本地HTTP替身，MCP使用本地工具服务，不能将其等同于真实模型效果测试。HTML生成到`apitest/reports/report.html`，仅本地查看，不随作业提交。

其他回归入口及验证范围见[接口测试说明](apitest/README.md)：

```cmd
powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts\test-isolated.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts\test-rag-isolated.ps1
```

后者使用独立临时Qdrant容器；两者均需先准备测试依赖。部署演示本身无需安装本机Go/MySQL。

## 目录

| 路径 | 内容 |
| --- | --- |
| cmd/api、internal/campus | API、认证、检索、对话、Skill、MCP编排及Go测试 |
| mcp-server、internal/mcpwire | 自建MCP服务与协议实现 |
| web、web/docs | 前端、Swagger与OpenAPI |
| examples | 已有知识库及Skill样例 |
| apitest | 接口脚本、一键隔离测试及演示初始化入口 |
| scripts、deploy、docker-compose.yml | 初始化、测试脚本及部署配置 |
| docs | 教师指南、账号、测试及材料删除说明 |
| openspec | 功能规约、设计与实施任务记录 |
| deliverables | 分课交付材料及脱敏验证证据，历史记录保留原验证范围 |

第5、6课设计见[AI会话与Skill规约](openspec/changes/add-ai-conversation-and-skills/)和[MCP集成规约](openspec/changes/add-mcp-tool-integration/)，交互改进见[UI规约](openspec/changes/redesign-campusclaw-ui/)。

## 数据与范围

`.env`、本机工具、数据库与上传卷、索引、生成的HTML、日志和个人Word/PDF不提交仓库；`.env.example`、源码、测试脚本、样例和OpenAPI随仓库提交。首次运行无需导入开发者数据库备份。

现有数据保存在命名卷中，升级前保留数据库与上传文件的配对备份，不执行`docker compose down -v`。修改后端或环境配置后重新运行上述构建启动命令；默认网页端口为127.0.0.1:8080，端口冲突可修改本机`.env`的WEB_PORT。演示账号已预置，无需开启学生自助注册。

当前不支持PDF/DOCX/图片内容解析或原文在线编辑；作业、错题及学情分析为预留入口。应用停用后禁止新会话和已有会话继续发送，历史记录保留。资料删除和历史引用失效规则见[材料删除说明](docs/material-deletion.md)。

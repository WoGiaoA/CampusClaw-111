# Design

## Context

项目已初始化 OpenSpec，当前没有业务代码、数据库或既有主规格；动机见 proposal.md。已阅读工作区《封面 · 第2课 · SDD 与 CampusClaw 规约.html》第 35–56 页，覆盖任务、原始提示词、补全、参考四件套及 Review/验收要求。样板的中小学背景按用户明确要求调整为高校；截图只作产品结构参考。

本次仅 Propose 与 Review。下述路由、表、文件、命令都是未来 Apply 的设计，不在本课创建或运行。需求以 specs/class-access、teaching-materials、project-runtime 为准。

## Goals / Non-Goals

**Goals:** 单应用实现“可信登录 → 服务端隔离 → 教师上传 → 保存解析 → 双表提交 → 本班可见”，让每个拒绝与失败结果可复验。

**Non-Goals:** 见 proposal.md 全部非目标。知识库仅保存原文件与解析全文记录；不生成向量、不分发 RAG 请求、不调用模型。未来六类结构的预留字段不产生作业、助手或技能页面与任务。

## Decisions

### 1. Flask + SQLite 单进程教学栈

- Flask 提供 SSR 登录/材料页面和同源 JSON API，SQLite 通过 Python sqlite3 参数化访问；Compose 内 Gunicorn 单 worker，数据库启用外键、WAL 与有限 busy timeout。
- 未来模块分为应用工厂、auth、materials、knowledge、db 与初始化脚本；本次不创建这些文件。
- 相较 Node/Express 与外部数据库，此方案贴合课程示例且便于 SQL 验收。SQLite 单实例并发和容量有限，满足本课演示，不承诺生产 HA。

### 2. 登录、可信会话与密码

- GET /login 渲染普通账号密码表单，POST /login 验证成功后清除旧会话，服务端写入 user_id、role、class_id，302 到 /materials。未知用户名和错误密码均返回 401 与统一提示，失败不建立会话。
- 使用 Flask 默认签名 cookie session：内容保存在客户端，由 SECRET_KEY 签名校验，并非服务端存储会话，也不加密。cookie 不含密码、哈希或密钥。比 JWT 减少独立 token 管理；不另引入 Redis。
- 每次受保护请求先验证签名和 8 小时有效期，再按 user_id 读取用户并核对 role/class_id。授权使用该可信会话身份；角色/班级不匹配时清除会话，客户端 query/body 不能覆盖它。角色变更不会继续使用旧权限。
- cookie 设置 HttpOnly、SameSite=Lax；HTTPS 设置 Secure，本地 HTTP 演示以 SESSION_COOKIE_SECURE=false 显式配置。POST /logout 校验 CSRF 后清除当前 cookie 并跳转登录页。已复制的旧 cookie 在到期前仍可能重放，集中撤销属于后续会话增强。
- bcrypt 生成随机盐的 password_hash，bcrypt.checkpw 验证，不设明文 password 列。种子口令从环境变量输入，使用 bcrypt 限制内的口令；日志不记录请求口令、哈希或 cookie。
- 受保护页面无有效身份为 302 /login，所有业务 API 为 401 JSON。检查顺序固定：身份 → 角色（学生上传 403）→ CSRF → 文件校验，避免学生上传落盘后才拒绝。
- 登录时生成会话绑定 CSRF token，所有已登录成员的 /materials 页面均可通过 meta 字段获得 token；仅教师显示上传表单。上传和退出缺失/错误 token 为 403。学生即使提供正确 token 仍为 403；服务端鉴权独立于前端。

### 3. 服务端班级隔离与接口契约

所有材料、知识库查询都接受从可信会话得出的 class_id；材料详情、下载使用 id AND class_id 查询，不先返回别班记录再隐藏。知识库查询也带 class_id，必要 join 同时核对材料归属。上传归属与 uploaded_by 从会话取得，忽略客户端 role/class_id。跨班与不存在资源统一 404，正文不含标题、路径、存储键。

| 路由 | 成功行为 | 权限/失败 |
| --- | --- | --- |
| GET /materials | 200 SSR 本班列表，教师可见上传表单 | 未登录 302 /login |
| GET /api/materials | 200 JSON 本班 id/title 列表 | 未登录 401 |
| GET /api/materials/{id} | 200 JSON 标题、解析文本及关联知识库 ID | 未登录 401；跨班/不存在 404 |
| GET /api/materials/{id}/file | 200 下载原文件 | 同上 |
| GET /api/knowledge-entries | 200 本班条目列表 | 未登录 401 |
| GET /api/knowledge-entries/{id} | 200 本班条目与完整 body_text | 未登录 401；跨班/不存在 404 |
| POST /api/materials/upload | 201 material_id、knowledge_entry_id | 未登录 401；学生/CSRF 403；输入 400；超限 413；内部失败 500 |
| GET /health | 200 或 503 JSON | 公开，无需 cookie |

列表没有本次搜索、分页业务，客户端附加 class_id 参数不影响本班过滤。uploads 置于静态根以外，仅受保护下载路由可读；API 不返回内部绝对路径或存储键。页面转义标题与文本，md 按纯文本展示，避免执行 HTML。不存在材料编辑、覆盖、删除接口。

### 4. 核心数据库结构

时间字段为 UTC ISO 8601；每个连接启用 foreign_keys。所有班级归属字段 NOT NULL，用户名与班级代码唯一。用户本次只属于一个班级，多班教师是后续变更。

| 表（领域） | 关键字段与约束 | 本次用途 |
| --- | --- | --- |
| classes（班级） | id PK、code UNIQUE（A/B）、name、created_at | 数据边界 |
| users（用户） | id PK、username UNIQUE、password_hash、role CHECK teacher/student、class_id FK、created_at；UNIQUE(id,class_id) | 四个登录账号 |
| lectures（讲义） | id PK、class_id FK、title、created_at；UNIQUE(id,class_id) | 将材料归类为教学讲义，无独立管理页面 |
| assignments（作业） | id PK、class_id FK、title、description、due_at 可空、created_at | 仅空表 |
| assistants（助手） | id PK、class_id FK、name、prompt_text 可空、created_at | 仅空表，不含真实模型密钥 |
| skills（技能） | id PK、class_id FK、assistant_id 可空、name、description 可空、created_at | 仅空表；有助手关联时用复合外键保证同班 |
| materials（材料） | id PK、class_id FK、lecture_id、uploaded_by、title、original_filename、storage_key UNIQUE、seed_key 可空 UNIQUE、media_type、byte_size、created_at；UNIQUE(id,class_id) | 文件元数据；lecture_id/class_id、uploaded_by/class_id 为同班复合 FK |
| knowledge_entries（知识库） | id PK、material_id UNIQUE、class_id FK、body_text、created_at；复合 FK(material_id,class_id) → materials(id,class_id) | 每份材料一条完整解析文本 |

assistants 同样提供 UNIQUE(id,class_id) 供 skills 外键引用。materials/knowledge_entries 的 class_id 建索引；禁止通过存储冗余字段产生跨班关联。每次教师上传创建一份 lectures 与一份 materials、knowledge_entries；讲义不是额外文件副本。三条记录在同一事务中提交或回滚。

### 5. 双班种子与账号规划

| 账号 | 角色 | 班级 | 服务端初始化口令变量 |
| --- | --- | --- | --- |
| teacher_a | teacher | A | SEED_TEACHER_A_PASSWORD |
| teacher_b | teacher | B | SEED_TEACHER_B_PASSWORD |
| student_a1 | student | A | SEED_STUDENT_A1_PASSWORD |
| student_b1 | student | B | SEED_STUDENT_B1_PASSWORD |

不在文档或源码固定真实口令，操作者填写四个变量后可按变量值登录。空库缺任一变量则初始化前失败，不落部分账号；已有库启动不要求种子口令。

A 班样本标题“A 班-课程导论.txt”、正文“CLASS_A_ONLY：课程导论”；B 班样本标题“B 班-学习指南.md”、正文“CLASS_B_ONLY：学习指南”。各有原文件、讲义、材料与同班知识库记录。assignments/assistants/skills 为空，仅验表结构，不种入虚假业务实现。

初始化按唯一 code/username 和 materials.seed_key（A-intro、B-guide）判断已存在记录，普通上传 seed_key 为 NULL；事务内创建缺失种子，不覆盖已有哈希或业务记录。重复执行保持行数和已有上传不变。种子文件同样使用上传一致性机制。若已有库缺失关联文件，启动报告一致性错误而非默默覆盖。首启与重启均只有一个初始化进程。

### 6. txt/md 保存、解析、事务与清理

1. 完成登录、teacher 角色、CSRF 检查后才接收文件到 uploads/.tmp 中的随机临时文件。文件最大 5 MiB；请求总大小限制为 6 MiB 给 multipart 头部留余量，超过文件限制均为 413。
2. 校验扩展名（大小写不敏感，仅 .txt/.md）、非空内容、严格 UTF-8（允许 BOM），去除 BOM 后得到全文，strip 后为空返回 400。md 不进行 HTML 渲染或模型处理；标题默认为原文件名的安全显示名称。
3. 生成 uploads/{class_id}/{uuid}.{ext} 存储位置，用户文件名仅用于显示，不参与磁盘路径；拒绝路径穿越，UUID 保证同名文件不覆盖。文件 close 后原子重命名到最终路径。
4. BEGIN 事务，写 lectures、materials、knowledge_entries（保留完整解析 body_text），均用可信班级。COMMIT 确认成功后返回 201。
5. 任何输入/解析错误为 400（超限 413），文件 I/O、DB 插入或提交异常为 500。异常回滚完整事务，清除本次临时及最终文件；已有文件不得删除。若提交结果不确定，重新查询本次 UUID 的材料与知识库关联：已完整提交则保留文件，未提交才删除，禁止把已提交文件误清理。
6. SQLite 与文件系统不能组成原子事务。启动阶段在接收请求前扫描专属 uploads/.tmp 与 class 子目录，将未关联已提交材料的残留文件删除；已提交且关联完整的文件保留。缺文件或不完整关联则服务不进入健康状态，报告脱敏错误。清理失败时记录存储键供服务端处理，启动再次尝试，未恢复一致不得报告 ready。

因此普通失败请求结束后无三表残留与孤立文件；进程中断的窗口通过单 worker、启动对账封闭。这里的清理仅针对应用自有上传目录，不遍历或删除其他项目文件。

### 7. Compose、环境变量、volume 与健康检查

未来新增 Dockerfile、docker-compose.yml、requirements.txt、.env.example、.gitignore 和入口脚本。Compose 服务 app 映射 8080:8080，Gunicorn 单 worker 监听 0.0.0.0:8080。入口先检查配置、初始化、对账，再启动 HTTP 服务；不开 debug。

环境变量：SECRET_KEY 必填且无默认值；DATABASE_PATH=/app/data/app.db、UPLOAD_DIR=/app/uploads；SESSION_COOKIE_SECURE 本地 HTTP 为 false；四个 SEED_*_PASSWORD 仅首启所需。真实值通过未提交的 .env/env_file 注入，示例只列空占位，.env、data、uploads 不入库，不把秘密复制进镜像或前端。

named volumes campusclaw_data → /app/data，campusclaw_uploads → /app/uploads。SQLite WAL/SHM 与库文件同卷；保留 volume 的 down/up 或重建不丢数据。删除 volume 为显式数据销毁，README 说明且不作为普通验证步骤。

README 将写明 PowerShell Copy-Item .env.example .env、填写变量、docker compose up --build -d、登录 URL、curl.exe -f http://localhost:8080/health、测试账号以及 down/up。本次仅规划，不创建部署文件、不启动 Compose。

GET /health 无需身份；初始化/对账完成、SQLite 可 SELECT 1 且上传目录可读写时为 200 application/json {"status":"ok"}，否则 503 {"status":"unavailable"}。不返回库路径、账号或密钥。Compose 使用镜像已有 Python urllib 请求健康端点，避免依赖未安装 curl；interval 10s、timeout 3s、retries 3、start_period 30s。

## Risks / Trade-offs

- [签名 cookie 可见且缺集中撤销] → 不放秘密，短期有效，逐请求核验用户归属，当前登出只清当前客户端会话。
- [遗漏 class_id 过滤或公开静态目录] → 查询层统一封装，详情与下载均双条件查库，对所有读取端点做双向跨班验证。
- [磁盘和 DB 不原子、清理失败] → 同事务三表、保存先于提交、失败补偿、启动前对账；单实例避免对账误删正在上传文件。
- [SQLite 锁竞争] → 单 worker、短事务、busy timeout，锁超时视为上传失败；多实例另立 change。
- [文本恶意内容、过大或路径穿越] → 5 MiB、严格 UTF-8、UUID 路径、纯文本转义展示，不执行上传内容。

## Migration Plan

没有旧库迁移。本次交付规约，未来经明确 Apply 授权后按 tasks 顺序实现。首次启动创建结构与种子，重复启动幂等；更新前备份数据库与上传卷，失败回滚应用版本并恢复成对备份，不在初始化中删库重建。

## Open Questions

无阻塞问题。字体、页面间距等表现细节留给实现，不改变本规约验收；不要求复刻截图。

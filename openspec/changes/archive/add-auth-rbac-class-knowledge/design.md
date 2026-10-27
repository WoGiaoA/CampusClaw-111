# Design

## Context
第三课初次验收阶段 用户授权第三课 Apply。已读本地首页及线上 scope、architecture、auth-schemes、security、s2-data、s4-isolation、s5-upload、s7-compose、s8-verify、think 十页，无缺失页面。来源 https://devops.hello1023.com/课件/第3课-课件-认证授权与知识库入库/index.html 。第二课原文保存 docs/history/lesson-02，仅为历史，不是并行方案。技术差异见 docs/lesson-03-baseline.md。

## Goals / Non-Goals
实现登录、角色/班级边界、txt/md 保存正文入库、下载。北大红暖白最小 SPA。不做 AI/RAG/模型/技能业务/作业/注册/演示复刻。六类教学结构仍保留，但讲义/作业/助手/技能仅预留空表。

## Decisions
### 教程技术基线
Go net/http、database/sql、MySQL 8.4、bcrypt，替代 Flask/SQLite/Gunicorn。静态 SPA 由 Nginx 返回，/api 原样代理 api:8080，/health 也代理 Go。Compose web/api/db，仅 web 发布 127.0.0.1:8080:80，api/db 无 ports，uploads 仅挂 api，db-data 挂 MySQL。教程没有限定 HTTP 框架、Cookie 名、大小值；标准库、campusclaw_session、5 MiB 为实现选择。/week03 是课程站点前缀，本项目本地 /api。开发代理仅绑定 loopback，非 Nginx 验收替代。

### 登录和会话
POST /api/login JSON username/password，成功 200 返回 id/username/role/class_id/class_name，并设置 HttpOnly/SameSite=Lax/Path=/ Cookie。Cookie 只含 crypto/rand 32字节随机 ID；sessions 存 HMAC-SHA256(SESSION_KEY,ID) 摘要、user_id、expires_at、csrf_token。SESSION_KEY 必填至少32字节无默认值，8h 过期。成功登录事务删除旧 Session 并换发；POST /api/logout 校验 CSRF、删除行、清 Cookie，旧 Cookie 立即无效。GET /api/me 返回身份与 csrf_token。每请求查 sessions join users/classes，以数据库 role/class_id 为准，不接受客户端 header/query/body 身份。HTTP 本地 Secure=false，HTTPS 配置 true。
未知账号走虚拟 bcrypt 验证；IP 每分钟10次失败限流，仍走哈希路径。未知、错误、限流统一401 {"error":"账号或密码错误"}，不返回429，真实原因仅脱敏日志；DB 故障503。密码 bcrypt 带盐，禁止明文列。写请求 X-CSRF-Token，顺序认证→角色→CSRF→解析文件；学生即使 token 正确仍403。

### 数据模型
InnoDB/utf8mb4，UTC。五业务表 classes(id/code唯一/name)、users(id/username唯一/password_hash/role CHECK/class_id FK)、sessions(id_hash PK/user_id FK/expires_at/csrf_token)、materials(id/class_id/uploader/title/original_filename/storage_key唯一/seed_key唯一可空/byte_size/created_at)、knowledge_entries(id/material_id唯一/class_id/body_text/created_at)。租户列非空有索引，复合外键保证上传者与正文同班。预留 lectures/assignments/assistants/skills(id/class_id/title或name/可空描述)，无业务接口；上传只写两表。
A/B各 teacher_a/teacher_b、student_a1/student_b1，口令首启从四个 SEED_*_PASSWORD 哈希，后续不重置。样本 A 班-课程导论.txt（CLASS_A_ONLY）、B 班-学习指南.md（CLASS_B_ONLY），均有文件及正文，seed_key 确保幂等。首启缺口令在写种子前失败。已有库不要求种子口令，不删库重建。

### API 与隔离
/api/materials、/api/knowledge-entries 列表 WHERE class_id=可信身份。/api/materials/{id}、/api/materials/{id}/file、/api/knowledge-entries/{id} 按教程先取行再比班级；跨班与不存在均404 {"error":"资源不存在"}。无登录401。本班空列表200 []。下载通过认证和班级校验后才读取文件，attachment；学生可下载本班。内部路径不返回，Nginx不静态映射 uploads。
/login、/materials 是公开静态页面壳，不含业务数据；未登录 /api/me 401 后前端 replace /login。学生不显示上传入口。POST /api/materials/upload 成功201两个ID，输入400/413，存储事务异常500。不提供编辑覆盖删除。

### 上传与一致性
multipart 文件与可选标题，请求6MiB/文件5MiB；txt/md大小写不敏感，严格UTF8、正文去头BOM、拒绝空白；PDF/DOCX/图片不支持。内存有界读取验证后才落盘，服务端随机名/class_id目录，原名只展示。原字节保留，正文纯文本不执行HTML，同名不覆盖。临时文件关闭rename后事务插入材料/正文，commit后201。失败rollback与删除本次文件。commit结果未知按storage_key查完整关联，确认两表已提交则保留并返回201，否则清理；无法判断标记未就绪待启动恢复。单实例启动清孤立/临时文件并验证所有已提交文件/正文，再接请求；运行上传互斥。测试注入hook不暴露生产HTTP。

### 部署与健康
MySQL服务端环境变量数据库名/用户/口令，api连接内部db:3306。SESSION_KEY、种子口令、MySQL口令仅.env注入；示例空占位。.env/.local/私有数据忽略，镜像上下文排除秘密。提供不覆盖现有.env的生成脚本，用户本地安全读取口令。
GET /health免登录，DB Ping、目录可读写且一致时200 {"status":"ok"}，否则503 {"status":"unavailable"}，无业务信息。api二进制healthcheck子命令，web wget，db mysqladmin。README提供up与down/up不删卷。
第三课初次验收阶段已实测Docker三服务healthy、Nginx转发、仅web宿主发布、健康故障503/unhealthy与独立卷down/up持久化，全部通过。启动/停止/重启操作统一CMD；保留原本机数据库作为备用历史数据。

## Risks / Trade-offs
- 磁盘/DB不原子 → 补偿、启动对账，失去一致性503。
- Session/秘密泄漏 → Cookie仅ID、库存摘要、日志/交付脱敏。
- 共享IP限流 → 教学级10次/分钟，生产分布式方案后续。
- 静态页面壳公开 → 壳无数据，API真鉴权，客户端跳登录。
- 镜像源网络故障 → 官方ECR副本、可配置IMAGE_REGISTRY，真实拉取构建已通过。

## Migration Plan
第三课最终容器验收：保留本机 MySQL 与 uploads，先逻辑导出并记录摘要，仅向确认空的新 Compose 卷导入，不覆盖任何已有业务库或卷。本地演示 .env 显式 APP_ENV=local/DEMO_MODE=true，保留个人密码及首次状态。独立验收项目用于注册、改密、上传和卷重建，不变更真实用户密码。宿主仅 Nginx loopback 8080，备用本机进程按确切路径停止且保留数据。

当前网络无法访问 Docker Hub auth.docker.io，镜像通过公共 ECR 的 Docker Official Images 副本获取；Go/MySQL/Nginx/Alpine 版本不变，IMAGE_REGISTRY 可配置，不改全局 Docker 配置。README 和课堂操作统一 Windows CMD，默认 Compose、本机开发仅备用。

原首次实现无旧业务库；本次最终部署已保留并迁移累积的4用户、6材料/知识库及全部文件。保留Git和原数据，先快照再向空的新卷导入，MySQL DDL/种子幂等。不执行down -v，不删真实卷；未来备份必须成对保存DB与文件。

本地 Windows 开发因 MySQL/Nginx 中文路径限制，官方便携工具在 ASCII TEMP/campusclaw-mysql-tools，独立开发库在 TEMP/campusclaw-local-mysql-data；应用上传在忽略的 .local/uploads。Compose 使用 named volumes，与本地库分开，不自动迁移或覆盖。


## 第三课 迁移后修正
项目根目录改为 D:\PKU\Projects\campusclaw-111。沿用 Go/MySQL/服务端Session/Nginx；不重写业务，不改真实账号、数据或卷。
登录及账号JSON必须是 application/json（允许合法charset参数）且仅一个完整JSON值；拒绝 application/jsonp、尾随JSON/垃圾与超限请求，400不创建会话。登录的客户端role/class_id仍不参与身份判定。
SPA为网络中断、网关非JSON和业务JSON错误提供可读提示；上传返回201与刷新列表是两个结果，刷新失败明确提示“上传已成功、刷新失败”，保留已提交状态避免重复上传。
开发测试不得导入业务.env或隐式连接13306旧库。集成地址须显式提供；独立便携MySQL脚本只创建全新.local测试目录及随机口令，测试仅使用campusclaw_test_*库；结束停止本次进程，保留本次测试目录与脱敏输出。缺MySQL为SKIP、缺页面依赖报错，不宣称全量通过。
Docker的构建/启动/重启与最终用户功能/UI验收由用户自行执行。开发者只运行Go/页面/独立数据库测试及静态配置校验；本轮结果与第三课初次验收阶段历史证据分别保存。
## 第三课 页面视口适配
用户反馈顶部过高、页脚多余及登录/材料页面需要整页滚动。顶栏固定50px，移除课程说明页脚；body采用视口高度和50px+剩余空间的网格，main可用高度由布局分配，取消旧min-height与大块上下留白。登录/注册/个人页在可用区域内居中；低高度窗口允许main内部滚动以保留全部输入和按钮。桌面材料页标题/提示不挤占整页，列表与上传区域独立滚动；窄屏单列并在main内部滚动，长文件名和身份不会造成横向溢出。使用vh回退和dvh适配浏览器可视高度，不用overflow隐藏表单内容冒充适配。
## 第三课 材料页密度与控件
根据第二轮UI反馈，材料主区域取消1120px宽度上限，使用16–24px边距；压缩卡片内边距、列表行高、标题/提示间距。材料与上传卡片按实际内容高度显示、超过可用高度才内部滚动，不强制拉长产生空白。正文和材料标题用普通字重，仅页面/区域标题保留适度强调。原生file input保留可访问性、required及文件提交行为，通过::file-selector-button设置统一红白按钮及hover/focus样式，不替换上传逻辑。静态资源版本更新，不要求Docker构建或重启。
## 第三课 统一视觉规范（本轮现行）
用户要求完整视觉优化，取代前两轮逐点修补的视觉参数。仅修改web静态前端和页面测试，不改后端/API/数据库/认证注册规则。采用#9B0000品牌色、中性暖灰背景、白卡片、细边框、4/8/12/16/24/32间距，50px品牌栏与最大1440px内容对齐，正文常规字重。按钮统一primary/secondary/text三级，字段默认/焦点/错误状态一致；登录双栏最大约1000px，去冗余英文装饰；材料页约七三双栏，列表文件图标/元信息/轻量操作，上传卡片与列表顶部对齐。短列表按内容高度、长列表内部滚动，窄屏单列，低高度可滚动到全部操作。

表单继续使用现有字段、Session和CSRF；仅增加文件名/大小显示、按钮loading、aria-busy/invalid、成功/失败视觉反馈和对话框可访问性/关闭清理。file input仍是原生name=file、accept txt/md与required，FormData协议不变。autofill背景覆盖为中性白色。所有动态材料/身份/文件名用textContent安全展示，不插入不可信HTML。

验证沿用DOM/VM回归，补充真实交互载荷/错误/忙碌/选择文件/正文打开关闭测试；使用现有浏览器检查1920x1080、1463x775、1280x720、平板及手机。浏览器布局验证使用仅本机fixture模拟响应，不操作真实账号和业务材料，明确区分模拟UI检查与后端集成验证。Docker构建/启停仍由用户负责。
## 第三课最终验收授权与证据分层
本轮用户明确委托最终验收及OpenSpec/项目文档收尾，替代此前等待用户功能确认的8.4流程；截图仍由用户准备。不得启停或重建主Docker服务。使用已运行db容器中的独立campusclaw_test_*库和临时上传目录测试，现有主入口作不含业务凭据的检查；本轮没有亲自执行的容器down/up只能引用标明阶段的历史证据，不冒称重跑。同班材料为共享资源，同班不同用户读取200是正确授权，个人身份仍只由Session决定。核心第三课范围不变，注册/首次与普通改密为后续独立账号变更的实际补充。
## 第三课最终对齐说明
原 Goals / Non-Goals 的“不做注册”仅界定第三课基础变更；独立 add-demo-registration-password-management 已扩展本地演示注册与改密，当前系统包含该能力，不存在第二套认证路线。Cookie 没有身份载荷，篡改场景改为随机标识篡改。17 项 Go、73 项记录 HTTP、15 项前端测试及12项当前入口检查作为最终证据；容器重建与 unhealthy 的实测沿用第三课初次验收阶段的历史证据。恢复测试构造中断残留状态，不冒称真实杀进程。详见 deliverables/lesson-03/lesson-03-acceptance.md。

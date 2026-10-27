# Tasks

本文件是未来 Apply 的实现与验证清单，本次不执行。全部保持 `[ ]`；规约已写完或已校验不代表实现任务完成。测试使用临时库/上传目录或独立 Compose 验收卷，不删除真实数据。每组同步补充该组测试及 README 操作说明。

## 1. 应用骨架与规则

- [ ] 1.1 建立 Flask 应用工厂、auth/materials/knowledge/db 模块与依赖清单，Compose 使用 Gunicorn 单 worker — verify: 安装依赖后可导入应用，检查仅有登录和材料范围模块，无未来业务实现。
- [ ] 1.2 对照 README 与 AGENTS 保留高校背景和“无规约不写代码”，补充本地开发说明 — verify: 文档与 proposal 非目标一致，未加入预置账号选择首页或截图复刻要求。

## 2. 数据模型与双班种子

- [ ] 2.1 实现八表建库、复合外键、唯一约束、角色 CHECK 和班级索引 — verify: 临时库查 sqlite_master/PRAGMA table_info、foreign_key_list、index_list；非法角色、重复用户名、无效班级、跨班关联插入均失败；作业/助手/技能仅空表。（project-runtime：核心数据结构）
- [ ] 2.2 实现环境变量配置及空库初始化前置校验，添加秘密与数据的忽略规则 — verify: SECRET_KEY 缺失/空值启动失败；空库缺任一 SEED_*_PASSWORD 不建部分记录；检查 .env 未跟踪、示例仅空占位、日志不打印值。（project-runtime：环境变量与密钥边界）
- [ ] 2.3 实现 A/B、四账号及两份标题正文可区分的原文件/讲义/材料/知识库种子；密码使用 bcrypt — verify: SQL 查询两班四用户角色与归属，逐条核对材料→讲义/知识库同班关联、文件字节和 CLASS_A_ONLY/CLASS_B_ONLY；验证 password_hash 格式及正确/错误口令，schema 无明文密码列。（双班验收种子；密码与会话安全）
- [ ] 2.4 实现种子幂等且不重置既有哈希、记录或文件，补充种子文档 — verify: 重复运行初始化比较主键、行数、哈希与文件集合不变；5.2 完成后加入上传记录再重跑，上传数据仍在；已有库移除四个种子口令变量仍可启动。（双班验收种子）

## 3. 登录、会话与密码

- [ ] 3.1 实现 GET/POST /login、签名会话和哈希验证，加入登录测试 — verify: 四账号正确口令均 302 /materials、后续身份角色/班级准确；错误密码和未知用户名均 401 相同提示、未建立有效会话；同口令独立 bcrypt 哈希因盐不同。（账号密码登录；密码与会话安全）
- [ ] 3.2 实现统一页面/API 保护、8 小时过期、每请求用户核对与 POST /logout — verify: 无 cookie、篡改 cookie、过期 cookie 分别导致页面 302 /login、各受保护 API 401；登出后当前客户端 API 401；变更用户归属后旧会话失效。（会话保护与退出）
- [ ] 3.3 实现 cookie 属性、上传/登出 CSRF token 和服务端检查顺序，补充登录安全说明 — verify: Set-Cookie 含 HttpOnly/SameSite=Lax，HTTPS 配置含 Secure；缺失/错误 token 为 403，数据库/文件/会话无变更；检查 cookie、响应与日志不含口令或密钥。（密码与会话安全）

## 4. 本班读取与角色隔离

- [ ] 4.1 实现材料/知识库数据库列表及最小材料页面，查询 class_id 只来自可信身份 — verify: 四账号访问页面和两个列表均 200，A/B 双向只含本班标题正文；伪造 query class_id 无效；临时空班返回 200 空集合；直接新增合法本班 DB 记录后刷新可见。（本班材料与知识库读取；可信班级隔离）
- [ ] 4.2 实现材料详情、知识库详情与受保护文件下载，上传目录不作为静态目录 — verify: 本班访问三端点 200，下载字节一致；A/B 双向教师和学生访问别班 ID 均 404，与不存在 ID 同响应；无他班标题/正文/路径/存储键，公开上传 URL 无法下载。（可信班级隔离；本班材料与知识库读取）
- [ ] 4.3 实现 teacher 上传授权与 student 只读页面，加入越权测试并记录状态码 — verify: 学生以合法 cookie/token 直接上传、伪造 role 都为 403，三表行数和上传文件集合不变；页面无上传表单；没有编辑/覆盖/删除业务接口；未登录上传优先 401。（教师上传与学生只读）

## 5. 教师上传、解析与一致性

- [ ] 5.1 实现 5 MiB txt/md 校验、严格 UTF-8/BOM 处理、空白拒绝、UUID 存储与安全文本展示 — verify: 合法 txt/md 可解析；缺文件、exe、空/空白、坏 UTF-8 为 400，超限为 413；路径穿越名称不逃出上传目录，HTML 内容只按文本展示；每次失败无新增行或文件。（教师文本上传入库；上传失败一致性）
- [ ] 5.2 实现保存→解析→讲义/材料/知识库同事务提交，返回 201 与两类 ID — verify: txt/md 各提交一次，SQL 验证三表各增 1 且归属会话班级、上传者正确、全文一致；伪造 body class_id=B 的 teacher_a 上传仍为 A；下载字节一致；同名两次上传不覆盖。（教师文本上传入库；可信班级隔离）
- [ ] 5.3 验证成功后可见与学生仍只读，补充上传说明 — verify: teacher_a/student_a1 的材料和知识库列表均含新 ID/title，B 班无记录且按 ID 为 404；student_a1 再上传仍 403；代码与依赖无向量服务、RAG 或模型调用。（教师上传与学生只读；教师文本上传入库）
- [ ] 5.4 实现保存失败和 DB 插入/提交失败回滚及临时/最终文件补偿清理 — verify: 在临时目录/库注入文件写入、知识库插入、提交失败，返回 500；逐次比较三表行数、记录内容与文件集合，无本次脏行、孤立文件或已有数据损坏；提交结果不确定时核查 UUID 后再决定清理。（上传失败一致性）
- [ ] 5.5 实现启动前残留对账及清理失败的未就绪处理 — verify: 模拟保存后提交前中断，重启删除残留；模拟提交后中断，重启保留三表和文件；清理失败或已提交文件缺失时不报告健康；扫描只限应用自有目录。（上传失败一致性）

## 6. Docker Compose、health 与持久化

- [ ] 6.1 添加 Dockerfile、Compose、.env.example 与启动入口，挂载数据库及上传 named volumes — verify: docker compose config 可解析，变量名、8080 映射、两卷、单 worker 与 design 一致；镜像不包含 .env、真实口令或开发数据库。（Docker Compose 标准部署；环境变量与密钥边界）
- [ ] 6.2 实现免登录 GET /health 及 Python urllib healthcheck，补充健康说明 — verify: 无 cookie 为 200 application/json 且仅 status=ok；临时环境模拟库不可读/目录不可用为 503 status=unavailable；响应无路径，Compose 判 unhealthy。（免登录健康检查）
- [ ] 6.3 集成首启初始化和对账，完成 README PowerShell 启动、账号、变量、URL、持久化说明 — verify: 使用独立空验收卷按 README 从 .env 到 docker compose up --build -d，服务 healthy，/login 200，四账号可登录；步骤不依赖预先手动建库；示例无不安全密钥默认值。（Docker Compose 标准部署；双班验收种子）
- [ ] 6.4 验证重建与种子幂等，记录删除卷的影响 — verify: 上传后记录 ID、正文、文件校验和；执行 down/up --build 不删卷，账号仍可登录且三表记录、下载校验和不变；再次初始化不覆盖上传；README 明确删除卷会丢数据。（持久化数据库与文件；双班验收种子）

## 7. 综合验收与规约复核

- [ ] 7.1 运行 openspec validate add-auth-rbac-class-knowledge --strict，核对完整制品 — verify: exit 0 无 error/warning，proposal、design、三个 capability delta、tasks 均在；规约内容与实现状态一致。
- [ ] 7.2 逐条执行全部 Scenario 并形成验收报告 — verify: 每条记录账号/请求、预期与实测状态码、DB/文件证据及通过/失败；明确列出学生上传 403、跨班 404、上传后本班可见、失败无残留、哈希和 Compose/health/重建结果，不以隐藏按钮证明权限。
- [ ] 7.3 按课件第 56 页进行独立新对话只读验收 — verify: 新对话仅凭仓库准确列出硬性要求与路径，引用“可信班级隔离”“教师文本上传入库”Scenario 原文回答跨班和上传变化，不依赖旧聊天记录；没有代码修改或越界未来功能。

## Requirement → Task 对照

| 能力 / Requirement | 任务 |
| --- | --- |
| class-access / 账号密码登录 | 3.1 |
| class-access / 会话保护与退出 | 3.2、3.3 |
| class-access / 教师上传与学生只读 | 4.3、5.3 |
| class-access / 可信班级隔离 | 4.1、4.2、5.2 |
| class-access / 密码与会话安全 | 2.3、3.1、3.3 |
| teaching-materials / 本班材料与知识库读取 | 4.1、4.2 |
| teaching-materials / 教师文本上传入库 | 5.1–5.3 |
| teaching-materials / 上传失败一致性 | 5.1、5.4、5.5 |
| project-runtime / 核心数据结构 | 2.1 |
| project-runtime / 双班验收种子 | 2.3、2.4、6.3、6.4 |
| project-runtime / 环境变量与密钥边界 | 2.2、3.3、6.1、6.3 |
| project-runtime / Docker Compose 标准部署 | 6.1、6.3 |
| project-runtime / 免登录健康检查 | 6.2 |
| project-runtime / 持久化数据库与文件 | 6.4 |

7.2 覆盖上述所有 Scenario 的综合复验；各组已有自己的验证方法，不将单元与安全检查全部拖到收尾。

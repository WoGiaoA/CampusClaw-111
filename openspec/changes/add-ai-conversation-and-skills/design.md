# 第 5 课设计

## Context

当前工作区已实现第 4 课 RAG。Go 的 `Handler` 与 `auth` 保护站内接口，MySQL 的 `sessions` 保存凭据摘要，登录签发不透明 Bearer 并兼容 Cookie。身份和班级每请求重读，写操作校验 `X-CSRF-Token`，首次改密门禁沿用。第四课 `/api/ask` 使用班级 hybrid 前四条，`model.go` 的 JSON 客户端整段解码，未命中短路。MySQL ngram、Qdrant 班级过滤、ready 代次回表和 `/materials?material_id=…&chunk_id=…` 已存在。当前 Go WriteTimeout 和 Nginx proxy_read_timeout 均为 45 秒，代理未给会话流单设配置。Compose 仅发布 web 入口。

工作区 `redesign-campusclaw-ui` 17/17、`add-profile-display-name` 2/2 均完成但未归档。 `add-traceable-vector-retrieval` 34/38，余项涉及真实模型及主 Compose 人工验收、保留卷验证和规格归档。以源码与相关活动 delta 为基础，历史分析中的「尚未实现 RAG」和主规约的助手预留描述分别属于早期状态。教师参考图 05、12、13 用于配置区、真实命中标记和来源展开。任务目录和课件规则由本地第 5/6 课概览及仓库分析复核，原始任务截图由分析记录转述。

## Goals / Non-Goals

- 在单实例 Go、MySQL、原生 SPA 和 Nginx 中提供明确的班级边界、可恢复的个人历史、每轮一致的配置快照及可追溯的证据。
- 将上游流、本站 SSE 与持久化分开验收。首版采用固定文本模板、一次选一项 Skill、已有材料集合，工具执行和断线续传留在本 change 之外。

## Decisions

### 班级配置与数据库

| 结构 | 数据及约束 |
| --- | --- |
| `assistants` | 沿用 id、class_id、name、prompt_text。加 enabled、可空 template_id、可空 builtin_key。同班模板复合外键，唯一 (class_id,builtin_key) |
| `prompt_templates` | id、class_id、name、body、可空 builtin_key；同班共享，唯一 (id,class_id)、(class_id,builtin_key) |
| `skills` | 沿用 id、class_id、name、description 及旧 assistant_id，新增启停、条件、规程、原始文本、可选资料标题条件和工具声明、可空 builtin_key；补同班复合唯一键 |
| `assistant_skills` / `assistant_materials` | 同班复合外键和唯一绑定对；各应用最多 50 个 Skill、100 份资料，整组替换先校验后事务提交 |
| `ai_conversations` | id、user_id、class_id、assistant_id、title、创建和最近活动时间；同班复合外键，索引按所有者、班级、更新时间和 ID |
| `ai_turns` / `ai_messages` | turn 含会话 ID、client_request_id、状态、脱敏失败类别、时间；唯一 (conversation_id,client_request_id)。message 含 turn ID、user/assistant、正文、时间；同轮角色唯一。助手行另存当轮 Skill ID/名称及真实引用快照（JSON，含编号、ID、标题、切片序号、原文位置与实际入模片段） |

每班幂等预置「课程资料问答」「解题引导」和各自的模板。资料问答指令侧重总结、引用及区分无资料时的一般性回答。解题引导指令强调提问与分步提示，并预绑「解题引导」Skill。材料绑定由教师显式选择，默认空。旧应用无可用配置时默认停用，旧 Skill 缺有效规程则停用且需有效文本替换，合法的旧 assistant_id 关系只回填一次。以后关联仅以绑定表为准。旧行保留，重复迁移不得把解绑关系或停用种子恢复。迁移阶段标记只在成功回填后更新，DDL 可重跑，所有班级含既有班级按唯一预置键补齐。

角色指令去首尾空白，空用默认角色。非空绑定模板作为额外指令附在角色段后，纯文本不做表达式求值。名称 1–100、角色 0–2000、模板 1–2000 个 Unicode 码点。教师可新增模板及编辑内置模板和应用配置，首版应用为内置集合，模板不提供删除。使用共享模板引用避免重复复制，修改对之后接受的轮次生效。每轮用短 SQL 事务读取一致的应用、模板、Skill 和材料绑定快照，随后释放锁。材料资格回表依然验证当前同班、ready 及快照材料范围。应用停用阻止新建，已有会话仍可使用，符合课程参考。若需撤回旧会话的某项能力，教师可解绑材料/Skill 或停用 Skill，下一轮即生效。UI 标明这一规则。

### 文本 Skill

上传 `multipart/form-data` 的 `file` 字段，接受 TXT/MD，限制请求及文件字节数（文件至多 32 KiB）和 UTF-8 有效性。去 BOM、统一换行，第一空行分隔元数据和正文。元数据每行一个键值，支持全/半角冒号。必填 `名称`、`适用条件`、`启用`（支持 是/否 与 true/false），可选 `资料`（材料标题含词）和 `工具`（唯一允许 `knowledge_search`）。拒绝未知键、重复键、无效 UTF-8、非法工具、空名称/条件/正文。整个文本最多 8000 码点，规程最多 2000。超限 413，其他格式错误 400。原文保存供教师管理，运行字段单独保存。PATCH 可改启停及条件，替换正文用同一解析流程上传文件至原 Skill；启停优先于原文件声明。

将适用条件按空白、逗号、顿号、分号、竖线切分，去空、转小写并去重，至多 32 词，每词至多 50 码点。对当前问题小写后按子串计分，每词至多一分，正分最高胜出并以最小 ID 打破平局。预置解题引导含「解题」「怎么做」「判别式」等关键词。教师受控规程作为系统指令的一部分，授权和输出预算由宿主强制。资料字段只将当轮绑定材料按同样分词规则做标题任一词匹配，在查询前缩小集合。工具字段是兼容的静态元数据，可表达本课已有检索，不生成模型 tools 参数或执行任何工具。证据记录候选 ID、得分、胜者与关联轮次，默认日志只保留脱敏元信息。

### HTTP 契约

成功 JSON 返回下表字段，集合用 `items`，分页再返回 `next_cursor`（可空）。请求对象仅接受列出的字段，服务端决定身份、班级与模型。绑定数组完整替换，有重复或超限为 400。所有路径同源，沿用 Bearer/Cookie。所有新增写请求含 CSRF，读请求免 CSRF。

| 方法及路径 | 请求与权限 | 成功响应 |
| --- | --- | --- |
| GET /api/ai/apps | 师生；教师含停用项 | 200 items，学生只见启用应用的 id/name，教师另见 enabled/role_prompt/template_id/skill_ids/material_ids |
| PATCH /api/ai/apps/{app_id} | 教师；可选 name/enabled/role_prompt/template_id/skill_ids/material_ids，null template_id 解绑 | 200 完整配置 |
| GET /api/ai/prompt-templates | 教师 | 200 items，id/name/body |
| POST /api/ai/prompt-templates | 教师；name/body | 201 模板 |
| PATCH /api/ai/prompt-templates/{template_id} | 教师；name/body 可选 | 200 模板 |
| GET /api/ai/skills | 教师 | 200 items，id/name/enabled/conditions/procedure/material_terms/declared_tool/raw_text |
| POST /api/ai/skills | 教师；上传 file | 201 Skill 及解析字段 |
| PATCH /api/ai/skills/{skill_id} | 教师；JSON enabled/conditions 或 multipart file 整份替换 | 200 Skill |
| GET /api/ai/conversations | 学生；cursor/limit | 200 items（id/app_id/app_name/app_enabled/title/updated_at）及 next_cursor |
| POST /api/ai/conversations | 学生；app_id | 201 会话，初始标题「新对话」 |
| GET /api/ai/conversations/{conversation_id} | 学生本人；cursor/limit 按轮分页 | 200 会话、items（轮次及按角色排列的消息和快照）、next_cursor |
| POST /api/ai/conversations/{conversation_id}/messages | 学生本人；question（去首尾空白后 1–500 码点）及 UUID client_request_id | 200 SSE |

会话列表按 updated_at、ID 倒序，轮次按 ID 倒序分页、页内从旧到新呈现，默认 20、最大 50。首个成功受理的问题前 24 个码点成为标题，失败轮也保留该标题。400 输入非法，401 凭据无效，403 角色/CSRF/首次改密，404 跨班、非本人、不存在或创建时应用停用，409 重复/并发/同 ID 正文冲突，413 请求过大，503 依赖失败。错误为 `{error,code}`，重复可附该轮 `turn_id,status`，并发只返回忙状态。发送接口预检失败用 HTTP 错误。流打开后用 SSE 终止错误。

### 授权检索与模型输入

把现有检索参数区分为「旧入口的整个班级」与「新入口的显式材料集合」，避免空集合变为全班。对后者，在 MySQL keyword 候选及模型签名校验条件中加入材料集合，在 Qdrant 候选请求中同时使用 class_id 和 material_id 集合条件（复用数值 payload，以实际固定版本协议验收）。按旧候选预算、阈值与 RRF 规则融合，SQL 对 Qdrant ID 复核同班、材料集合、当前 ready 代次及签名。只取前 4 个非空片段，每段最多 800 码点，顺序编号。绑定资料但学科过滤后为空为 `no_hits`。无绑定为 `unbound`，跳过外部检索。有命中为 `hits`；检索故障在开流前返回 503 并标记失败，避免将故障误报为无资料。旧 search/ask 继续全班语义，原引用及访问规则保留。

出站同一服务端 Chat 配置，目标报文含 `model`、`messages`、`stream:true`、输出预算，初版目标 `max_tokens:600`，字段名以真实网关最小验证为准。消息顺序为单个 system（角色、选定模板、仅命中 Skill 名称和规程，以及宿主的引用/数据边界约束）、最近最多 5 条已完成轮次中的非空 user/assistant 文本（按消息顺序取最近再正序，每条最多 500 码点）、一个当前 user（问题一次，加本轮有界编号证据）。包含当前问题的窗口共 6 条，system 与证据附段单独计预算。历史只保留消息正文，无历史引用附件或 Skill 内容。无依据时系统提示是一般性回答且不生成引用，有证据时要求以 [n] 引用。外部文本按明确分隔作为资料；后端不根据模型文字授予权限。已有回答及其出处为当轮历史，解绑影响新检索，不擦除已完成会话。引用快照读取仍校验会话和当前材料班级，若来源 chunk 因重建失效则回退到受保护的材料原文定位，并显示当轮片段；原文不可访问时显示来源不可用并隐藏该引用的受保护详情。

### 真实流与事务边界

- 预检鉴权、CSRF、请求、会话归属和 SSE 写能力。短事务锁会话行，先判重复再判有无活动轮，原子写 running 轮和 user 消息并更新会话时间。模型请求期间不持有事务。重复 ID 统一 409，正文不同标记冲突。显式重试采用新 ID，同会话前一轮仍活动时返回 409。
- 受理后执行检索和模型输入组装。SSE 使用 `text/event-stream; charset=utf-8`、`Cache-Control: no-store`、`X-Accel-Buffering: no`。每个事件以空行分隔，data 为 JSON，逐帧 flush，允许注释心跳。
- `meta` 首事件，含 `turn_id,user_message_id,retrieval_status,skill`（null 或 id/name）、`citations`（有编号的候选引用）。`delta` 含 `turn_id,text`，逐段转发上游有效正文，过滤空内容和仅用量帧。解析上游 SSE 多行 data 与分块，校验首选项、结束原因和协议终止信号。文本模型输出禁止工具调用。OpenAI 兼容预期 `choices[0].delta.content`、`finish_reason` 和 `[DONE]`，以 Apply 真实报文确认合法终止组合。EOF 本身不表示完成。
- `done` 含 `turn_id,assistant_message_id,citations`（最终有效引用）。成功终止前验证非空、正常结束和引用编号，任何越界引用或有证据却无引用均按生成失败处理；无证据时带 [n] 也失败。禁止在已发 delta 后改写正文。助手正文使用全部 delta 的原样拼接，引用按正文实际合法编号保留。保存助手、快照及状态和更新时间在同一事务，commit 后才发 done。
- 开流后 `error` 含 `turn_id,code,message`，连接仍可写时发送且作为唯一终止事件；断线只能靠重读确认。取消、中断、超时、输出超限、无正常结束或落库失败均保留用户消息，未提交的助手正文与引用只属临时 UI，标为未保存。commit 成功后写 done 失败仍可回放已提交结果。用短且独立的有界清理上下文，在请求取消后按 running 条件更新失败。commit 结果不明先重读，已完成不可降级；单实例启动恢复遗留 running 为失败。运行中数据库暂不可达导致无法落失败时，在固定总预算加清理宽限到期后按会话锁及 running 条件恢复，晚到请求仅能提交自己的仍为 running 的轮次。
- Nginx 单独匹配会话 messages 路径，保留 Host 与来源头，关闭 SSE 响应缓冲和压缩，HTTP/1.1 代理，read timeout 120 秒。流总预算 90 秒（含检索最多 20 秒），上游流首字节 20 秒、相邻读取空闲 30 秒，空闲时每 15 秒发送注释心跳。以路由级 Go ResponseController 设置写截止（总预算加 5 秒），保留其他路由 45 秒写超时。取消传到上游请求；独立流客户端沿用安全配置和凭据策略，不使用现有整段 JSON 的 30 秒全局客户端，禁止自动重试。限制上游 SSE 单事件 64 KiB、全流读取 2 MiB、助手正文 16 KiB，超限终止。通过代理测试慢流首帧、逐帧及客户端取消传播。

### 前端与课程验证

教师沿用 `/applications` AI 应用管理五分区，接通前四区的加载、配置保存、模板编辑、Skill 文件解析结果与启停、资料勾选；MCP 分区保持预留。学生在 `/assistant` 沿用应用栏、会话栏、消息与输入区域。新对话从启用应用创建，选历史恢复并锁定应用，保留建议问题和原文弹窗。以带现有 Authorization 和 CSRF 的 POST fetch 读取 ReadableStream，按 UTF-8 和事件边界解析，AbortController 取消或切换，轮次标识丢弃迟到帧。meta 显示真实 Skill 与检索状态，done 确认保存，断开或 409 后重读详情核对。保留紧凑布局、窄屏、可键盘操作、文本安全渲染、导航和材料链接。

同源 `/docs/swagger/` 与 `/docs/openapi.json`，使用固定版本本地 Swagger 资源。OpenAPI 描述上述路径和实际错误、Bearer scheme 无 JWT 格式标记、Cookie 兼容、CSRF header、SSE 事件顺序，补验证需要的 login、me、search、ask 及来源读取。Swagger 输入 login 返回的 access_token 和同会话 csrf_token（也可用 /api/me 取得 CSRF），禁持久化授权、示例全用占位值；流式通过 Network 或 `curl.exe -N` 实测。

```text
apitest/
  run_all.py
  cases/api/login/POST.py
  cases/api/search/POST.py
  cases/api/ai/apps/GET.py
  cases/api/ai/apps/{app_id}/PATCH.py
  cases/api/ai/prompt-templates/{GET,POST}.py
  cases/api/ai/prompt-templates/{template_id}/PATCH.py
  cases/api/ai/skills/{GET,POST}.py
  cases/api/ai/skills/{skill_id}/PATCH.py
  cases/api/ai/conversations/{GET,POST}.py
  cases/api/ai/conversations/{conversation_id}/GET.py
  cases/api/ai/conversations/{conversation_id}/messages/POST.py
  reports/report.html
```

花括号参数目录为字面占位，方法组合表示分别建文件。runner 从隔离夹具创建师生、两班、已有索引及独立数据库，统一登录、测试顺序、只读数据库核对和脱敏输出，不读取主站 .env。用 `python apitest/run_all.py` 汇总用例、状态/断言、通过失败计数、耗时和流式时间证据，失败退出非零，HTML 本地生成。继续用现有 Go、HTTP、独立 MySQL/Qdrant 和 DOM 测试承担各自回归。关键对照覆盖学生配置越权、缺 CSRF、伪造 ID、同班未绑定、跨班、假 Qdrant ID、模板替换、Skill 停用/同分、窗口边界、重复/并发、取消与提交错误。

### 额度与真实验证

本轮 Propose 为零 Chat/Embedding 请求。Apply 默认模型及查询 Embedding 均用隔离 HTTP 替身，禁用真实网关出网，自动化、健康检查和报告 runner 无真实模式。已有索引尽量复用，合成索引只在隔离环境以替身建立。出站替身记录实际 JSON，覆盖模型、角色、模板、Skill、片段、历史、stream 和输出上限；对两应用及所有反例使用替身。

Mock、浏览器和代理验证通过后，通过显式启用的独立验收步骤做最小真实验证。首问用已有 ready 资料和简短合成或可公开问题，同时触发模板、Skill、RAG 和真实 SSE。随后同会话用短追问验证历史和已保存结果。基准为两次 Chat，若每轮有绑定资料则每轮至多一次查询 Embedding，零入库 Embedding。若阶段人工验收需补第二款应用的真实对话而现有证据缺失，另一次无绑定短问即可。成功后复用证据；失败先检查网关状态、帧字段、代理和替身，不盲重试。取证记录场景、脱敏后的实际请求字段和响应结构、两跳增量到达时刻、Skill/知识片段/模板与数据库对应、调用计数和是否需补测。遮盖凭据、Cookie、CSRF、私有资料与用户身份，替身和真实结果分开标识。真实首问还人工核对引导规程执行和引用语义。

## Risks / Trade-offs

- 网关真实流字段、预算字段、终止信号未知 → 保留隔离流协议测试和最小真实验证任务；真实能力失败必须记录并解决，不能把完整 JSON 拆字当作通过。
- Qdrant 与 SQL 范围不一致或旧索引换代 → 两路候选均先过滤，SQL 回表二次授权，引用保存当轮快照。
- 代理缓冲、总超时与断线时 commit 竞态 → 走真实 Nginx 隔离测增量；以数据库状态为准并在页面重读。
- 模型可能不遵循教学指令 → 取证验证实际指令，人工核对真实代表性输出；程序限制引用与权限，文案不承诺每次回答质量。

## Migration Plan

Apply 从空库和既有库双路径验证幂等迁移，保留账号/会话、材料/索引、旧助手和 Skill。更新部署前配对备份数据库与存储卷。旧 API、健康检查、前端兼容入口先回归。回滚时恢复旧版服务与前端并保留新增表和列，已有生成记录待重新部署后恢复可见。保持当前单 API 实例的部署约束。工作台 delta 的三个修改项来自仍活跃的 `redesign-campusclaw-ui`，未来经人工同意同步规格时须先合入其基础要求，再合入本变更；本轮不修改该 change 或主规格。

## Apply 协议确认

真实课程网关接受 `stream:true` 和 `max_tokens:600`，返回 `choices[0].delta.content`，两轮均以 `finish_reason:stop` 加 `[DONE]` 结束。隔离验收复用了已有样例向量，实际调用次数和两跳时序见[第 5 课开发与验收](../../../docs/lesson-05-development.md)。


## 验收关闭记录

用户确认第5课验收完成，7.2关闭。该确认不补写缺失的回答原文，不扩大原真实网关取证范围，也不代替第6课真实Tool Calling验收。后续已实施的应用启停删除、128 KB Skill与UI修订以redesign-campusclaw-ui为准。

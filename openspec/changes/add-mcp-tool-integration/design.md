# 第 6 课设计

## Context

依据课程本地概览、仓库第 6 课分析记录的任务清单与参考图 06、08、13。课程站实践分章本轮未能在线读取。阶段验收截图要求外部工具/MCP 和解释出站模型参数。课程任务另明确同工程 Compose 自建服务、至少两个工具、Swagger、审计与同伴交叉验证，扩充接口脚本属课程建议并纳入本次交付。

第 5 课 17/18。当前 `ai_stream.go` 已有请求去重、独占轮次、90 秒总预算、可取消 HTTP 流、meta/delta/done/error 与提交后发 done。模型解析器目前拒绝非空 tool_calls，`ai_skills.go` 仅接受 `knowledge_search` 声明。旧 `/api/ask` 独立运行。工作台 `/tools`、`/audit` 与应用 MCP 分区仍是占位。Compose 只发布 web，Nginx 流式路径已禁用缓冲并设 120 秒读取超时。已有隔离 MySQL/Qdrant、HTTP 模型替身、Nginx、Swagger 和 Python runner。第 5 课的真实普通 Chat 流通过，网关流式工具能力未验证。

## Goals / Non-Goals

- 同一 Go 宿主中追加两跳工具循环，保留应用配置、授权 RAG、Skill 选择、历史窗口、真实增量和落库边界。
- 支持受控只读工具、可追溯授权及失败恢复。通用远端连接、任意脚本、工具副作用、MCP 资源和提示模板、代理编排及完整统计留待独立需求。

## Decisions

### 独立服务与协议

`mcp-server/` 是独立 Go HTTP 进程，使用标准库和静态教学表，同 Compose 内由 `http://mcp-server:8080` 访问。独立内部网络只连 API 与 MCP，MCP 无宿主端口、无数据库/向量库/LLM 访问、无额外卷。非 root 运行。GET /health 无凭据且只给就绪状态，缺 `MCP_SERVER_TOKEN` 时服务拒绝业务请求且不就绪。API 和 web 的启动及健康无需 MCP 可用，既有 RAG 服务和卷保留。主站不代理 /mcp，隔离测试直接访问内部端点。

Apply 将独立服务放入同一 Compose 的 `mcp` profile，由 `docker compose --profile mcp up --build -d --wait` 启用。旧部署未配置工具令牌时仍可按原命令等待 API 与 web 就绪。

首版选课程 HTTP JSON-RPC 2.0 子集。宿主向 /mcp 发送带服务 Bearer 的 UTF-8 POST，单个对象。按 initialize → notifications/initialized → tools/list；初始化报告支持 tools，协商固定 `2025-06-18` 报文版本，初始化通知回 202 空体。tools/call 传原工具名与 arguments，成功响应为文本 content 数组；执行失败用 isError 或规范的 RPC error。请求 ID 必须匹配，拒绝未知方法、重复 JSON 键、未知参数、非对象、尾随值或超限。RPC 错误约定 -32700/-32600/-32601/-32602/-32603，鉴权失败 HTTP 401，媒体类型不符 415，超限 413，未实现的 HTTP GET /mcp 为 405。有 Origin 时拒绝浏览器来源，客户端服务间调用不带 Origin。工具服务比较令牌时恒定时间。

选用 JSON POST 往返，宿主只接收 JSON 响应，固定无协议会话、单页列表、不提供 MCP SSE、批处理和通用客户端协商；遇 SSE、分页游标或会话 ID 明确拒绝连接。HTTP 请求取消是本项目这一子集的机制。与标准 Streamable HTTP 的握手、Accept、会话及服务器 SSE 能力边界分别标注。浏览器 SSE 属于本站会话 API。依据 [MCP 生命周期](https://modelcontextprotocol.io/specification/2025-06-18/basic/lifecycle)、[HTTP 传输](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports) 和 [工具报文](https://modelcontextprotocol.io/specification/2025-06-18/server/tools)。

| 工具 | 输入与结果 |
| --- | --- |
| `formula_hint` | topic 枚举 `quadratic_equation` / `linear_equation`。返回公式提示、可检查条件和固定策略键，例如 `check_discriminant`。 |
| `strategy_hint` | strategy 枚举 `check_discriminant` / `isolate_variable`，step 为 1–3 整数。返回该步引导及自查问题。 |

Schema 为严格对象，拒绝多余字段。使用固定输入枚举避免把学生正文、资料和凭据转发给独立工具；返回只读提示，默认不给最终答案。用前一结果的策略键驱动后一请求，证明真实因果。工具每请求与响应各 32 KiB 上限。

### 配置、发现、授权与数据

首版登记地址精确限制为部署配置允许的 Compose 服务 Base URL，默认候选为 `http://mcp-server:8080`。教师在受控列表选择并可查看 Base URL。禁止 URL 用户信息、参数、片段、额外路径、IP 字面量、重定向、代理环境变量及任意 DNS 目的地。宿主从可信配置构造实际 URL 和内部网络拨号目标，执行时再次比对。隔离测试可注入固定 loopback 夹具地址，生产入口不接受该注入。工具 Bearer 仅发送到对应受控服务。

| 结构 | 约束及用途 |
| --- | --- |
| `mcp_servers` | id/class_id、不可变 slug、可编辑显示名、允许端点、密文令牌、连接状态、revision、最近脱敏错误。班级内 slug 唯一。凭据用独立环境密钥和认证加密，关联班级与行 ID。 |
| `mcp_discovered_tools` | 同班服务 ID、工具名、当前 revision、受支持的 Schema、说明和指纹。只含当前可调用快照。 |
| `assistant_tool_grants` | 同班应用复合外键，外部项关联稳定服务 ID、原工具名及获批指纹，内置项为固定检索键。整组替换事务，最多 16 项。与临时发现行解耦以保留失效配置。 |
| `skill_tool_refs` | 同班 Skill 关联声明列表，外部记录稳定服务 ID 和工具名，保留人类可读引用。旧 `skills.declared_tool` 回填内置声明，保留旧读写兼容字段。 |
| `tool_call_audit` | 追加事件，以 attempt_id 关联开始与终态。关联 class/user/conversation/turn，服务和工具名快照、阶段序号、状态、原因、耗时、结构化脱敏摘要。按班级/时间/ID 与会话索引。 |

持久化密钥缺失或解密失败时工具管理明确不可用、工具路径关闭，基础业务仍可用。教师只能写入或轮换令牌，列表、详情、错误和审计只返回是否配置。更新地址或令牌使连接失效。首版不开放删除和修改 slug，避免旧引用被重新指向。旧技能无外部声明，既有应用的工具授权默认为空。按幂等迁移验证带旧会话、资料和索引的库。

连接动作先短事务增加 revision、标记 connecting、清空当前发现快照，再在事务外握手与发现，总计 8 秒。只接受白名单 Schema 子集（闭合对象，字符串/整数/布尔标量，受限 enum、长度与数值范围，无引用或任意嵌套）。最多 16 个工具，名称唯一。完整校验后以 revision 条件在事务内整体发布，错误以相同条件标记 disconnected 及安全原因。断开增加 revision 并清空当前快照，迟到连接不得覆盖。应用授权保存时核对本班及当前快照，说明或 Schema 指纹变化要求教师重新保存授权。断开或失败时旧授权可见为不可用。重连且相同指纹恢复时，下一轮可复用既有授权。

Skill 的工具字段为单行文本，使用逗号或顿号分隔，去重且最多 8 项。外部工具名 `^[a-z][a-z0-9_]{0,47}$`，服务 slug `^[a-z][a-z0-9-]{0,47}$`。创建、替换文件或显式编辑工具声明时，格式错误 400、未知或他班资源统一 404、已登记本班但不可用 409。仅改条件或启停时保留旧声明和不连接提示。原文件留作教师查看，显式编辑的生效声明与原文件分开；旧 `declared_tool` 可读，第 6 课新增 `tool_refs` 权威数组。客户端声明和模型名称都无授权效力。

### API 契约与工作台

新增管理 API 均为教师本班访问，Bearer/Cookie 和首次改密检查沿用，写操作验证 X-CSRF-Token。错误用 `{code,error}`，输入非法 400、超限 413、无效认证 401、角色或 CSRF 403、不可见 404、状态冲突 409、外部依赖或密钥不可用 503。发现失败返回受控原因，前端可通过列表重读实际连接状态。

| 路径 | 方法与结果 |
| --- | --- |
| /api/mcp/servers | GET 返回 items（id、slug、display_name、base_url、status、has_credential、tool_count、last_error_code）；POST 接收 slug/display_name/base_url/token，201 |
| /api/mcp/servers/{id} | PATCH 修改 display_name/base_url/token，200；省略 token 保留，null 清除 |
| /api/mcp/servers/{id}/connect | POST 握手及发布，200；已连接时表示重连 |
| /api/mcp/servers/{id}/disconnect | POST 幂等断开，200 |
| /api/mcp/servers/{id}/tools | GET 返回当前清单及状态，断开时 items 为空 |
| /api/ai/apps/{app_id}/tool-grants | GET 返回授权与可用性；PUT 接收 refs 整组替换，200 |
| /api/ai/skills、/api/ai/skills/{skill_id} | 保留原 GET/POST/PATCH；上传和替换解析外部引用，PATCH JSON 扩展 tool_refs |
| /api/mcp/audit | GET 分页，默认 20、最多 50，状态和同班会话可筛选，按事件 ID 倒序 |
| /api/ai/conversations/{conversation_id}/messages | 保留原 POST 及 SSE，新增 tool 事件；会话详情可附本人各轮已记录摘要 |

教师沿用 /tools 的服务卡、/applications 的 MCP 和 Skill 分区以及 /audit 表格。显示正在连接、断开、工具数、脱敏错误和过期授权。学生沿用 /assistant 的消息区，增加折叠过程。只展示受控工具名、状态及结果类别，不显示完整参数、URL、凭据或思维链。桌面、窄屏、键盘与取消/切换会话回归。

### 两跳模型与工具循环

1. 沿用已接受的会话、一次初始 RAG、模板、唯一当前问题、最多五条历史文本和确定性命中 Skill。候选为命中 Skill 声明、当前应用允许项、班级权限、服务当前快照的交集。内置 `knowledge_search` 只在显式授权时可调用，使用空对象参数复用已计算且仍授权的当轮引用，不再请求 Embedding，也不扩展搜索范围。
2. 有候选时发 Chat Completions `tools:[{type:function,function:{name,description,parameters}}]`、`tool_choice:auto`。使用稳定且无冲突的本轮别名映射服务与工具名，模型不接触地址、令牌或 RPC ID。候选为空、耗尽预算或最终收尾时省略 tools/tool_choice；无候选沿用原有流逻辑。使用原模型与每请求 max_tokens 600。
3. 按 tool_calls 的 index 聚合 ID、名称、arguments 分片。只在完整终止后解析一次。拒绝重复 ID、不连续/超限索引、无效 JSON、身份与工具列表不符。先出现正文的模型轮按直接回答流转发，之后再出现工具意图按协议错误且不得执行；先出现工具意图的轮只处理工具，不把工具阶段内容当最终正文。工具模式期望以 tool_calls 结束并收到 [DONE]；混合帧的真实兼容性留待验证。畸形流使用既有 error 边界；可归属的单项非法参数回传安全拒绝。
4. 按 index 串行调度，每次派发前在短事务复核会话凭据/归属、应用授权、原命中 Skill 仍启用且绑定、服务 revision 和指纹、参数 Schema 与额度，锁定相应授权行并追加派发记录。撤权提交后不得再获派发许可。事务外在出站前检查取消、最新代次及授权修订；已派发或与撤权并发的旧请求尽力取消，结果返回后再次核对，撤权后到达的结果不得入模。限额计数以已派发记录为准，失败和取消均消耗。审计不可用时禁止派发。
5. 模型结果使用前一 assistant 的完整调用数组及每项匹配的 `role:tool,tool_call_id,content`。每个可关联的拒绝/超限也对应一条安全结果；不将未知模型别名归入他班工具。结果仅接纳文本，先验结构，再清除控制字符、截断至 800 码点并按固定格式标注为外部数据。检测到诱导指令或格式不符时用固定失败结果替换。模型请求的工具说明同样来自校验并获批的短文本。工具结果不能改变授权或生成虚假资料引用，历史窗口只保存原有 user/assistant 正文。
6. 每个模型批次最多解码 4 个调用意图，最多执行 2 个。每次提问最多 2 个工具阶段、4 次派发、3 次 Chat；会话累计最多 12 次派发。单工具 4 秒，所有工具共 12 秒，在原 90 秒会话总预算内执行，至少预留 20 秒给最终生成。工具运行时保持原 SSE 心跳。额度不足或工具失败时使用已有信息发起无工具收尾，未留出时间或上游持续要求工具则明确失败。禁止自动重试真实请求或工具。取消停止后续派发、取消在途 HTTP、保留已记录终态；遵循原用户问题与最终正文持久化规则。

### SSE、审计与恢复

meta 仍是首个事件。课程材料以正文前的 meta 汇报工具过程，CampusClaw 保留第 5 课的首帧含义，用同样早于正文的独立事件实时汇报。新增 `tool` 事件，字段为 turn_id、attempt_id、tool、stage、status、reason_code、summary，允许已记录的 running 后接 success/denied/error/cancelled。只有最终答复的 delta 可进入助手正文。工具摘要从与该轮关联的脱敏审计/轮次元数据恢复，失败轮也保留真实执行记录。无法归属的模型别名可显示当次受控拒绝，历史仅在轮次元数据中保留通用提示，独立审计无需记录原值。保持 done 仅在助手正文落库后发送，error 为互斥终止。Nginx 保持逐帧和断开传导。第 5 课旧前端可跳过未知事件。

对已登记服务的拒绝直接追加 denied，实际调用先写 running 再发网络请求，随后追加唯一终态。无原始错误体。参数摘要仅允许批准的枚举/步数等低敏字段，其他只记录结构校验类别。结果摘要只写类型、长度和安全原因或预定义工具提示摘要；绝不记录任意结果全文。进程重启对未终结事件追加 interrupted，条件唯一约束保证幂等。终态写入失败时不把结果回传模型，保留开始记录供恢复。学生只能读取本人会话有限状态，教师本班分页读取；同伴其他班不能读、改或借由错误探测资源。

### 验证、证据和额度

Apply 默认真实 Chat 和 Embedding 均为 0。现有 runner 增加隔离真实 MCP 进程及 Compose 验证；Mock LLM 脚本返回分片 tool_calls，并断言后续 assistant/tool 消息和结果内容。使用已有测试索引或由 Mock Embedding 生成的合成索引。自动化只允许模型地址指向测试替身，禁止回退真实配置。Swagger 同源试调登记、连接、清单、Skill 和授权，JSON-RPC /mcp 在内部用流式客户端或脚本取证。沿用一接口一脚本及 HTML 报告，含双班、学生权限、令牌不回传、断开失效、审计隔离、限额、取消、真实执行和 RAG/单轮回归。四项同伴交叉核查分别记录服务/工具隔离、令牌不回传、断开后拒绝且留痕、审计权限隔离。

阶段问题 5 以同一 turn 的脱敏模型声明、模型意图、RPC、tool_call_id 回传和结果影响回答串起。阶段问题 1 与 4 同时记录每次模型请求、模板、Skill 和引用，问题 2 与 3 沿用第 5 课证据。课堂本地链路能证明真实 MCP 执行，网关兼容及真实模型使用结果单列待验收。额度重置不是授权信号。获得明确授权后，优先用已有 ready 资料和合成问题做一次两工具综合首问，上限 3 Chat、1 查询 Embedding、0 入库 Embedding；确需补历史或第 5 课人工复查时，再在另行同意的预算内做一次短追问，上限 1 Chat、1 查询 Embedding。先查协议和替身证据，失败后不盲重试。记录每次实际计数、工具增量与最终回答原文的安全摘录，复用通过的证据。

## Risks / Trade-offs

- 网关可能不支持流式 tool_calls、混合正文或工具角色回传 → 先用受控脚本验证解析及收尾，再在获授权的最小真实场景核对。真实失败不能以 Mock 通过代替。
- 授权与出站并发存在已派发窗口 → 事务复核后立即派发，撤权提交阻止新许可；在途取消与返回后复核避免旧结果入模。
- 外部 Schema 或说明变化与内容注入 → 受限 Schema、获批指纹、服务端复核和结果净化，权限始终由宿主执行。
- 多阶段生成拉长首字延迟 → 工具状态与心跳及时送达，限制调用、上下文及总耗时，保留无工具路径。

## Migration Plan

只增表、索引和兼容回填，保留数据和卷。空库、含第 5 课数据的隔离库和重复迁移分别验证。回滚以旧 API、web 和 Compose 恢复原会话能力，保留新增表以便恢复，停用新增服务后原功能仍可运行。未来若人工授权同步，先合入 UI 基础及第 5 课 delta，再同步本 change 的三个修改项；第 4 课检索和原部署条款保留，新增 MCP Compose 要求独立记在服务规约。当前其他 change 保持原状态。


## 撤权后的无工具回答及协议文本保护

1. 未得到可用候选时，在原system消息末尾追加当前工具不可用的约束，优先于Skill中的调用步骤：使用已有依据进行普通文字引导，必要时说明无法使用工具，禁止模拟执行、伪造结果或输出内部调用格式。有限工具循环进入无工具收尾时使用同一约束，并保留既有真实工具结果。
2. 各模型流式阶段的正文增量经过保留前缀的过滤器，识别FunctionCallBegin/End及tool_call等协议标记；跨delta切分时先暂存潜在标记前缀，确认是正常文字才放行，正常结束刷新残余普通标点。发现完整或已明显开始但被截断的内部标记时返回unexpected_tool_text，不执行、不保存为成功助手回答、不自动追加模型请求。
3. SSE对该错误返回可读说明，失败轮保留原用户问题。已流出的普通前缀可显示为未保存片段，但协议标记不得泄露。
4. 对已有历史助手回答的同类标记，在页面渲染时用“未执行的工具调用文本已隐藏”说明替代；不修改数据库历史，不影响用户原始问题，也不伪造工具过程。

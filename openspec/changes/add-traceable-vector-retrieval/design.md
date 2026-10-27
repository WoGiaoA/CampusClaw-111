# Design

## Context

动机和作业边界见proposal。本设计是待实施方案，不是对当前代码功能的描述。

已审查的当前实现：Go `net/http` + `database/sql`、MySQL8.4、数据库Session、Nginx同源静态SPA；`materials`保存文件元数据，`knowledge_entries`按材料唯一保存全文。`auth`每请求关联users获取角色、班级和首次改密状态；新增接口直接使用同一上下文。

`upload`当前在进程级 `uploadMu` 下保存文件并事务写入材料/全文，成功返回201及两个ID。数据库事务、锁、Go写超时和Nginx代理超时不适合容纳整份5MiB材料的外部嵌入调用。`db.go`只有建表及单列兼容迁移，没有通用迁移系统；新增表、索引和复合约束需逐项幂等处理。当前Compose只有web/api/db，Qdrant和嵌入网关尚未配置。

## Goals / Non-Goals

**Goals:**

- 用现有服务完成本班检索和真实文本出处，不复制认证、材料存储或前端技术栈。
- 将文本持久化与检索索引解耦；状态可观察，失败可重试，外部依赖故障不影响原文。
- 为本轮OpenSpec作业提供可开发、可验收的方案，所有实现任务保持未完成。

**Non-Goals:**

- 本轮不执行迁移、模型调用、容器操作或业务实现。
- 当前变更不提供 `/api/ask`、回答引用生成、历史会话、自定义/标题切分或流式输出。它们可在后续独立变更承接本检索能力。
- 不引入Redis、消息队列平台、编排框架、多实例索引调度或无缝影子索引。以下SQL任务只是单实例索引恢复机制，不扩展为通用异步任务平台。

## Decisions

### 1. 采用课程兼容检索方案，明确它不是作业强制选型

选择MySQL ngram全文索引 + 课程兼容的服务端Embeddings接口 + Qdrant + Go内RRF。MySQL保留文本和业务关系，Qdrant仅保存向量与标识。相较仅用LIKE，中文全文索引有明确排序；相较更换为pgvector或全文引擎，当前方案保持第三课MySQL和会话架构。

作业允许考虑向量数据库，并未强制使用。本项目在proposal中选择了该路线，因此三模式是此变更后续Apply的合同；不声称其他同学仅采用关键词方案就不符合老师要求。自动窗口已足够表达本次溯源；暂不将所有课堂演示策略都纳入作业变更。

### 2. 原文提交后登记持久任务，后台索引

流程：

```text
上传：Session → teacher → CSRF → 现有校验/文件保存/材料与全文事务 → 201
                                                        ↓ 提交后登记
                                    knowledge_index_jobs(pending)
                                                        ↓ 单实例worker
清理旧向量/旧片 → 自动切分 → 写pending切片 → 有界批量Embedding → Qdrant upsert
                                                        ↓ 全批确认
                                SQL事务将本批切片及任务标ready

搜索：Session → 参数校验 → 本班关键词 / 问句Embedding+本班Qdrant → SQL本班ready回表
                                                      → RRF/排序 → 命中及出处
```

不在上传锁或原保存事务内调用外部服务；201及原有响应字段保持兼容。提交后登记失败不删除已提交原文；详情对无任务材料推导为pending。启动后及定期仅扫描“没有任务记录”的已保存材料，幂等补登记；已经failed的任务不自动无限重试，由教师明确重建。

采用MySQL持久任务，而非只用goroutine内存队列，是为了进程中断后可找回工作。一个worker按批处理，网络调用不持有SQL事务；单材料最多一个活动任务。后台批调用默认30秒超时、最多3次可重试尝试（网络/429/5xx，退避1秒、2秒）；鉴权、维度或非法响应不盲重试，记录脱敏失败类别。每个任务处理期限为10分钟，到期failed；大材料可以明确失败和重试，不能承诺5MiB总能在任意网关额度下完成。

进程退出时取消worker；启动恢复本实例遗留running任务为pending，先清理该材料可能残存的向量/片再重试。不支持两个API实例共享同一任务库运行；部署文档明确单实例，不能假称具备分布式租约。工作更新以任务generation和状态做条件更新，重建或取消后的旧执行结果不得标ready。

### 3. 数据模型与归属约束

以下是设计字段，不是本轮已执行DDL：

| 对象 | 关键字段和约束 |
| --- | --- |
| knowledge_entries | 保留全文及现有唯一material_id；增加供复合外键引用的唯一键(id,material_id,class_id) |
| knowledge_index_jobs | id；material_id唯一；knowledge_entry_id/class_id复合关联全文；generation；status(pending/running/ready/failed)；model_signature；last_error_code；timestamps |
| knowledge_chunks | id自增；index_job_id/generation；material_id/knowledge_entry_id/class_id；chunk_index；chunk_text；char_start/char_end；index_status(pending/ready/failed)；timestamps |
| FULLTEXT与普通索引 | chunk_text使用ngram解析器；班级/状态索引；材料与generation索引；同批chunk_index唯一 |
| Qdrant | collection=campusclaw_chunks，Cosine；point ID=knowledge_chunks.id；payload只含class_id、material_id、knowledge_entry_id、chunk_id、chunk_index |

任务需有(id,generation,material_id,knowledge_entry_id,class_id)复合唯一键供切片引用；切片同时通过复合外键保证任务批次、材料、条目、班级一致。仅分别外键到三张表无法防止“同班但挂错材料”，必须对齐组合。更换generation前，在事务中将旧片退出ready并移除/处理其外键关联；不能直接更新被旧切片引用的复合键导致DDL或重建失败。

同一材料重建时，事务先把旧批次改为非ready、移除旧切片，再更新同一任务行generation/status=pending；旧向量暂留也无法通过回表。worker按可信material_id及class_id删除旧点并等待确认，再写新片/向量。清理失败则任务failed，不发布部分新批次。此顺序与“旧索引先失效、再建新索引”的教程目标一致，重建期间允许暂时无命中。

批次ready发布在单个SQL事务中完成，查询同时检查切片ready、任务ready和generation相同，避免只检查切片状态。向量写成功而SQL发布失败时，点不对用户可见；重试删除后重建。HTTP超时不等于Qdrant一定没写入，因此清理和upsert须以ID及材料过滤幂等完成。失效切片ID不复用。

### 4. 默认切分与精确出处

只实现auto：Go按 `[]rune(body_text)` 计数，最大800、重叠80。每次拟取到start+800或末尾，非末尾优先在该窗口内最后一个空行、其次换行、其次句号后断开；候选断点必须令片长大于80，否则强制按窗口上限断开。非末片nextStart=end-80，必须大于start；末片直接结束。短文本单片。

位置是保存后的 `knowledge_entries.body_text`，不是文件字节：UTF-8 BOM已由第三课解析移除，CR和LF分别算码点。区间[start,end)，序号1起，chunk_text必须严格等于对应区间；本变更不折叠空白、不去链接、不改写原文，省去预处理坐标映射。完整切片中重复的80码点可使同材料多个结果包含重叠内容；按chunk_id去重，不按字符串去重。

### 5. 检索、过滤、RRF与错误语义

`POST /api/search` 是只读检索，需Session及首次改密校验；不会因HTTP动词为POST而拥有索引写权限。参数q/mode/limit见spec；可复用有界JSON解析，但客户端class_id/role/user_id明确忽略，不能误用账号修改的未知字段全拒绝策略。索引重建是写操作，另校验teacher、CSRF及本班材料。

keyword：MySQL `MATCH(chunk_text) AGAINST(... IN NATURAL LANGUAGE MODE)`，同一表达式分数>0，加入可信class_id及本批ready条件。使用ngram token=2，验证运行配置和中文样本；不使用LIKE替代。查询使用参数绑定，不拼接用户SQL。取最多40候选；不访问嵌入/Qdrant。

vector：同索引配置嵌入q，调用Qdrant时即加入class_id过滤、score_threshold=0.35，最多40候选；以point ID批量回MySQL，限制班级、当前ready批次及关联材料。过滤后按余弦降序、ID升序。为控制开销，不继续无限补取被回表剔除的候选，结果允许少于limit。

hybrid：两路各最多40候选，先做各自绝对分数过滤和回表校验，过滤后重新编号rank=1…N。按chunk_id融合 `Σ1/(60+rank)`，缺席一路0；按融合分降序、ID升序取limit。keyword全文分与余弦分不直接相加。40是本项目有界候选数选择，不是作业原文数值。

搜索总预算20秒以内；查询嵌入及Qdrant分别最多8秒，不在HTTP内对耗时外部调用循环重试。SQL或相关外部依赖故障503，无有效会话401，首次改密403，非法输入400；无命中200。hybrid任一路失败503并提示可显式选keyword，不静默降级成混合成功。文本正常存储但索引失败时空结果仅表示“没有可检索命中”，详情状态提供原因，不声称正文一定不存在。

### 6. API契约与原生前端

| 接口 | 行为 |
| --- | --- |
| POST /api/search | q、mode、limit；200含q/mode/hits/message；hits非空时message为空字符串，空命中时为固定提示 |
| GET /api/materials/{id} | 保留所有字段，新增index_status和chunk_count；无任务为pending/0，非ready计数0 |
| POST /api/materials/{id}/reindex | teacher + CSRF + 本班；无需策略参数；202含material_id、index_status=pending；活动任务409 |
| GET /api/materials/{id}/chunks/{chunk_id} | 本班ready且材料匹配才返回完整chunk_text、位置和来源信息；跨班/失效/不匹配404 |

单条hit固定含spec的来源字段；另含score、keyword_score、vector_score、keyword_rank、vector_rank。keyword模式score为全文分，vector模式为余弦分，hybrid为RRF；不存在的一路score/rank为null。API不返回query_vector或原始点数组。

source_url使用本站 `/materials?material_id=<id>&chunk_id=<id>`，前端读取参数后通过受保护接口打开本班材料和切片；接口未授权时照常处理401/403/404。excerpt为chunk_text前200码点，不加入假省略号到原文字段。当前UI沿用textContent展示，材料和摘录不解释为HTML。

现有材料工作区增加检索区、模式选择、结果及出处；教师详情可查看索引状态和重建，学生没有写按钮。后台任务可用材料详情刷新观察，不新增全站轮询体系；pending/running时可短时轮询，页面关闭即取消。保留68px顶栏、现有宽度/滚动/移动布局及原接口协议；不复刻教师完整演示站或迁移React。

### 7. 配置、网络与基础健康

后续服务端配置拟为 `EMBEDDING_BASE_URL`、`EMBEDDING_MODEL`、`EMBEDDING_API_KEY`、`EMBEDDING_DIMENSION`、`QDRANT_URL`、可选 `QDRANT_API_KEY`。集合名固定campusclaw_chunks；维度必须为正且与网关响应和已存在集合一致，不能猜一个默认维度。集合不存在时按已配置维度创建，已存在却不匹配时失败，绝不删真实集合重建。

模型签名由规范化base URL、模型名和维度构成，不含密钥；每批记录。当前配置与已有ready索引签名不同，vector/hybrid返回503并要求明确重建，避免混用；keyword仍可读取文本。网关确切地址、模型和维度在Apply前通过实际课程账号配置确认，这不会改变接口或任务划分。

Compose增加内部qdrant服务和qdrant-data卷，不配置ports，浏览器不直连。api不以Qdrant健康为启动硬前提；基础health仍检查MySQL与上传存储。Qdrant失败由索引状态和vector/hybrid 503表达，不让整站被标为无法使用。

## Risks / Trade-offs

- 双库写入不原子 → Qdrant先完成、SQL最后发布ready；非ready回表不显示，按ID幂等清理重建，不承诺分布式事务。
- 重建有短暂无命中窗口 → 明确pending/running和失败反馈；影子切换延后，原材料阅读不受影响。
- 阈值0.35对不同模型效果不同 → 作为课程初始基线，在双班固定语料上验证原词、改写和无关问题；不宣称阈值能证明所有命中都是答案。
- 大文件耗时/限额 → 后台批处理、超时及有限重试，不占上传事务和锁；失败可见，限额属于网关配置前置检查。
- 中文检索受ngram/停用词影响 → 用真实MySQL验证中文，不拿SQLite或mock代替全文索引验收。
- 误用第三课恢复逻辑 → 文件Reconcile继续只管理原uploads，不将索引不一致解释为原文件应删除；Qdrant缺失不触发删正文。
- 多实例并发 → 当前明确单实例；若后续扩容须另设计任务租约/文件对账，不在本课扩张范围。

## Migration Plan

以下均为后续Apply及运行计划，本轮不执行：

1. Review本变更，确认网关配置；保存数据库/原文件的配对备份和现有行、文件摘要，不重置真实凭据。
2. 增量建立所需唯一键、两个新表和全文索引；DDL逐项检查是否存在，已有不兼容约束明确报错，不DROP旧表。验证MySQL ngram配置，保留已有卷。
3. 更新服务端配置、Compose及文档，由用户执行构建/启停。Go先完成原第三课初始化和文件对账，再登记缺失任务、启动worker并接受请求；外部网关不可用不阻止原业务。
4. 默认策略幂等补索引所有无任务的既有材料，包含种子；仅新增索引，不再次Seed或覆盖原文。失败由教师显式重试。
5. 在独立库/目录和独立Qdrant验证约束、失败、中断恢复、双班权限及原功能回归。用户执行真实容器持久化和UI验收，记录真实与模拟结果。
6. 回退时停止worker并回到第三课API/页面，保留新增表和向量卷，不回退或覆盖用户数据；第三课旧二进制忽略新增表。未完成任务保留，重新启用本能力前先恢复清理，不把旧批次直接标ready。

## Open Questions

- 课程网关实际可用的base URL、模型、维度和限额：作为部署参数在Apply前核实，未知值不得填写虚构默认值或公开凭据。
- Qdrant镜像的可获取固定版本：在实际部署时选择并记录验证版本，本次不拉镜像、不承诺未经验证的兼容版本。

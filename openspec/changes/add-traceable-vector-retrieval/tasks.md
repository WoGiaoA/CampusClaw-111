# Tasks

按用户最新要求 Apply Token + RAG；复用原规划的单实例worker、三模式、来源与权限设计。下列开发/自动化项仅在通过后勾选，外部参数、真实部署和人工验收单列。作业3 PDF 暂缓，不生成或准备。

## 1. 实施前配置与兼容基线

- [x] 1.1 更新 proposal/design/specs、AGENTS，明确 Token/RAG 和无损迁移；保留第三课与既有分析。验证：文档自审及第三课73项隔离HTTP回归。
- [x] 1.2 实现服务端环境配置和空占位模板，不猜模型/维度。验证：TestRetrievalConfiguration、TestExternalClientValidation、TestLegacyRetrievalMigration；缺配置仅索引failed，原文保留。
- [x] 1.3 增加内部 Qdrant v1.14.1、独立卷及可选空密钥启动处理。验证：Compose静态校验、真实隔离Qdrant测试；主服务未启停。
- [x] 1.4 按用户截图填写本地.env课程模型配置；仅Embedding/Chat各一次轻量连通性检查通过（2048维），不查询或假称剩余额度、不跑真实模型全量测试。实际资料效果与浏览器验收仍见9.1。

## 2. 数据结构、切分与坐标

- [x] 2.1 实现增量复合唯一键、jobs/chunks、ngram全文索引及批次约束。验证：TestLegacyRetrievalMigration、TestRetrievalMigrationRecoveryAndConstraints，旧库结构模拟升级、重复迁移、原密码/正文/文件/会话保留、错误关联拒绝。
- [x] 2.2 实现800码点/80重叠自动切分。验证：TestChunkCoordinates 的中文、emoji、CRLF、断点、尾部、无断点与前进性。
- [x] 2.3 文档明确序号1起、Unicode码点零基[start,end)，不改原文。

## 3. 外部客户端与持久索引

- [x] 3.1 实现真实HTTP Embedding/Qdrant客户端、维度与模型签名、集合Cosine校验及写入确认。验证：外部HTTP替身错误分支及TestRealQdrantRoundTrip，向量payload仅ID。
- [x] 3.2 实现单worker状态流、10分钟预算、30秒批超时、16片批量、有限重试。验证：TestIndexRetryBound及索引失败不发布批次测试；外呼在原文事务/上传锁之外。
- [x] 3.3 接入上传提交后登记、启动/定期幂等补齐。验证：上传201字段和第三课回归、重复登记与缺配置原文保留测试。
- [x] 3.4 实现running恢复、旧片/点清理与generation条件发布。验证：SQL构造running中断恢复、真实Qdrant重建、失效来源404；这是模拟中断，不等于真实进程强杀或容器重建验收。
- [x] 3.5 文档明确故障类别、任务预算、失败教师重试、原文保存不回滚。

## 4. 索引状态、重建与来源

- [x] 4.1 材料详情增加index_status/index_error/chunk_count；验证：ready/failed原文仍可读，非ready计数0，原权限回归。
- [x] 4.2 实现本班教师+CSRF重建202、活动任务409、学生/缺CSRF403、跨班404。验证：RAG集成与恢复测试。
- [x] 4.3 实现本班ready且材料匹配的完整切片来源，错材料/跨班/旧ID404。验证：RAG集成与真实Qdrant重建。
- [x] 4.4 编写API与重建无命中窗口说明，明确202仅代表登记。

## 5. 三模式搜索

- [x] 5.1 实现认证/首次改密门禁、JSON与q/mode/limit边界、忽略伪造身份。验证：Token生命周期、参数与匿名测试、既有JSON解析回归。
- [x] 5.2 实现MySQL ngram中文全文检索、ready过滤、正相关度排序。验证：真实MySQL中文测试；keyword调用Embedding计数不增加。
- [x] 5.3 实现同模型问句Embedding、本班Qdrant过滤、0.35阈值与SQL二次核对。验证：真实Qdrant、恶意跨班/过期ID替身候选、签名变更与依赖503。
- [x] 5.4 实现40候选上限、过滤后rank、RRF k=60、去重与稳定截断。验证：TestRRFFilteredRanks、hybrid失败不静默降级。
- [x] 5.5 构造来源、200码点摘录、各路分数与null缺席字段、空结果200。验证：RAG集成和来源核对。
- [x] 5.6 文档区分全文分、余弦与RRF，空命中不表示正文不存在。

## 6. 原生前端

- [x] 6.1 师生共用问答/三模式检索、空/错误/忙状态、安全文本展示。验证：21项DOM测试（含第三课15项回归）。
- [x] 6.2 增加来源深链接解析、受保护原文与切片对话框。验证：DOM来源API、坐标/安全文本及后端权限测试。
- [x] 6.3 教师索引状态、手动刷新与重建，学生没有写入口。验证：DOM重建CSRF/Bearer，上传已保存与索引结果区分；本轮采用手动刷新，无页面轮询。
- [x] 6.4 更新使用与截图说明，保留68px顶栏、原下载协议并改认证fetch。验证：DOM下载/Bearer与语法检查；真实多视口另见人工项。

## 7. 集成回归与交付

- [x] 7.1 在真实隔离MySQL及独立Qdrant完成上传→ready→三模式→来源→重建；模型使用HTTP替身，模拟/真实边界已记录。
- [x] 7.2 完成第三课认证、改密、CSRF、班级、上传回滚、下载、持久数据与迁移自动化回归；完整测试0跳过。
- [x] 7.3 验证基础health、原文、keyword与向量/生成依赖故障相容；Qdrant故障不静默降级hybrid。
- [x] 7.4 更新README、开发说明、测试证据和规约自审，OpenSpec严格校验；保持change未归档，第三课历史主规格待验收后同步。

## 8. Token 与 RAG 增补

- [x] 8.1 实现随机不透明Bearer、摘要存储、八小时到期、优先认证、Cookie兼容、撤销与改密/CSRF门禁；TokenLifecycle及原会话测试通过。
- [x] 8.2 前端sessionStorage、统一Authorization（含下载）、401/退出/改密清理；DOM验证。
- [x] 8.3 实现本班hybrid前4条→真实生成接口→编号引用校验；验证无依据不生成、客户端system/history不入提示、非法编号/依赖失败503。
- [x] 8.4 完成人工操作、Network截图和服务配置文档；未生成PDF。

## 9. 等待人工验收（本轮不代替用户确认）

- [ ] 9.1 用户配置真实Embedding/Chat服务后验证语义检索、回答正确性、引用对应和实际限额。
- [ ] 9.2 用户执行主Compose升级、检查Nginx/Network Bearer、师生双班流程与桌面/窄屏展示。
- [ ] 9.3 用户确认容器保留卷重启与实际运行中断恢复；本轮只做SQL构造中断与临时容器测试，未做主部署持久化验收。
- [ ] 9.4 用户亲自测试确认后再同步主规格/归档；作业3实现方案PDF留下一轮，本轮无PDF准备工作。

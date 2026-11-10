# 第四课实施自审

2026-11-04。复用原proposal/design，先补Token和RAG规格再实施。此为编写者自审，用户尚未验收。

| 检查 | 本轮结果 |
| --- | --- |
| 最新范围 | Token认证、资料导入、三模式检索、模型问答与引用；不生成PDF，不做历史会话/流式/额外切分策略 |
| 架构 | 复用Go/MySQL/Nginx/Compose/原生前端，仅增加内部Qdrant及HTTP客户端，无框架重构 |
| Token | 同一随机Session值作为Bearer，库内HMAC摘要，优先头部且不回退非法头，保留Cookie/CSRF/首次改密，退出与改密撤销 |
| 无损数据 | 真实隔离MySQL旧结构升级、重复迁移、错关联约束、原文件/正文/哈希/会话保留验证；主库未操作 |
| RAG | 真实代码HTTP调用Embedding→Qdrant→SQL→Chat，模型无生产模拟开关；测试替身仅存在_test.go |
| 班级 | SQL/向量filter/SQL回表三层约束，跨班与失效ID不进提示，来源受保护 |
| 索引 | 持久任务/代次、单worker、有限重试、确认向量后事务发布、失败保留原文、教师重建 |
| 生成 | 本班前4证据、无命中不生成、忽略客户端system/history、编号检查、错误503；编号检查不能证明语义正确 |
| 前端 | sessionStorage+Authorization、下载fetch、textContent、问答/来源/状态、保留原身份与布局 |
| 默认部署 | 实测发现Qdrant空API_KEY触发401，已通过共用启动脚本移除空变量，并用真实隔离容器回归 |
| 验证与交付 | 完整结果见test-results.md；真实模型、主部署、浏览器截图、多视口与卷持久化待用户验收 |

不将OpenSpec校验、HTTP模型替身或第三课历史记录冒充真实模型效果。change保持未归档，主规格待人工确认后同步；作业3留下一轮。

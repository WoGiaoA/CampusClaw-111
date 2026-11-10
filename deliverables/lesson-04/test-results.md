# 第四课开发测试结果

日期：2026-11-04。状态：**开发和隔离自动化完成，等待人工验收**。作业3 PDF 暂缓，未生成或准备。

## 本轮最终结果

命令（Windows CMD）：

    powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts\test-rag-isolated.ps1

| 检查 | 结果及边界 |
| --- | --- |
| Go测试 | 30个顶层测试通过，0失败、0跳过，含第三课全部回归及新Token/RAG测试 |
| 原第三课HTTP场景 | TestFinalCourseworkEvidence内73项本轮重新通过，不引用旧容器结果替代 |
| 前端DOM | 21项通过，含原15项回归及Token/下载/问答/来源新增测试 |
| 静态与构建 | go vet、API build、JS语法、Compose config、OpenSpec strict通过 |
| 真实MySQL | 每次新数据目录、新随机测试库；中文ngram全文、复合约束、旧结构升级/重复迁移、原密码/正文/会话/文件保留验证 |
| 真实Qdrant | v1.14.1独立tmpfs容器：集合创建、Cosine/维度校验、确认upsert/delete、班级过滤、SQL回表、重建通过 |
| 外部模型 | Embedding/Chat使用HTTP替身验证请求、鉴权、JSON、维度、超时、限额、故障和编号；未调用真实网关，不表示真实模型效果 |
| 临时资源 | 隔离MySQL停止、专用标签Qdrant自动移除，无业务卷挂载；测试目录留在忽略的.local下 |

证据：[完整最终输出](development-tests.txt)、[机器摘要](development-test-summary.json)、[严格规约校验](development-openspec-validation.txt)。测试脚本需便携MySQL/Go、Node.js/jsdom、Docker与已获取的Qdrant镜像，不读取真实.env。

## 覆盖与修复

- Token：登录返回/摘要存储、纯Bearer请求、Cookie兼容、非法Bearer不回退、CSRF、退出撤销、首次改密门禁与改密后旧Token失效。测试发现改密内部仍依赖Cookie并已修复；原有到期、动态角色/班级和并发改密回归继续通过。
- RAG：上传后ready、三模式、伪造班级/system/history无效、向量跨班/失效候选回表剔除、受保护出处、重建后旧ID404、失败非ready且原文保留、恢复与模型签名变更。
- 生成：证据取本班hybrid前4条、真实接口调用代码；测试无命中不调用Chat、合法编号、非法引用/依赖错误503。生产代码无假向量/硬编码回答分支，固定无依据提示属于无命中行为。
- 索引：有限重试及永久错误不重试、running模拟中断恢复、generation、写入确认、ready发布、教师重建权限与冲突。恢复是SQL构造中断，未模拟真实进程强杀、磁盘故障或SQL提交结果不确定的全部组合。
- 默认部署：实测Qdrant空API_KEY会触发401，增加共用启动脚本在空时unset；最终真实Qdrant测试使用与Compose相同脚本且显式传入空密钥，通过。
- 第三课：认证/改密/注册/CSRF/班级/上传回滚/下载/健康/种子幂等与数据迁移回归均保留。

## 验证过程中的限制

最初沙箱拒绝系统临时目录rename及本地网络连接，改用项目内.local/test-tmp并在获准的隔离测试环境运行；最终结果取上述无跳过运行。首次临时容器清理遇PowerShell引号问题，修复为JSON标签核验后仅清理本次容器；最后运行已验证自动清理。不涉及主业务服务。

DOM测试中的“Not implemented: navigation to another Document”来自jsdom不支持真实浏览器导航，相关存储和错误分支断言通过；原有导航另有VM断言，但真实浏览器仍待用户验收。

## 尚未验收

- 真实Embedding与Chat网关：用户需填入实际base URL、模型、维度、密钥与额度，验证语义质量、无关问题和实际响应耗时。
- 主Compose/API镜像/Nginx实际升级、真实浏览器Network Bearer截图、桌面/窄屏视觉与双班手动使用。
- 主部署保留卷重启与真实运行中断恢复、大文件在真实网关额度下完成情况。
- 引用编号校验只证明编号落在证据列表，不证明模型每句结论正确，需人工对原文核验。

主库/真实凭据/原卷未操作，已有业务容器未启停。OpenSpec仍保留人工项，不归档，不将开发测试说成作业全部验收通过。操作步骤见[开发与人工验收](../../docs/lesson-04-development.md)。


## 后续轻量连通性补充

用户提供课程网关截图并授权配置后，仅执行Embedding、Chat各一次真实请求（共2次、无重试），均HTTP200；Embedding实际2048维。密钥已写入忽略的本地.env，未纳入交付文件。参见[脱敏记录](course-gateway-connectivity.json)。上文“未调用真实网关”描述的是此前全量自动化运行；本次补充仅证明连通性，不能替代完整RAG、实际资料与浏览器验收。按用户要求停止真实调用，不执行批量建索引或大规模真实模型测试。

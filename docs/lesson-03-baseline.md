# 第三课基线调整

第三课来源说明：本轮使用本地首页及.local/tutorial十页缓存，未重新在线获取；下文线上阅读结论为第三课初次验收阶段历史记录。沿用Go/MySQL/服务端Session/Nginx。用户后续已委托开发者完成最终功能验收，Docker启停仍由用户负责；当前结果见[最终验收](../deliverables/lesson-03/lesson-03-acceptance.md)，迁移阶段记录保留。
已读教程首页及十个线上关联页，无缺失。来源目录：https://devops.hello1023.com/课件/第3课-课件-认证授权与知识库入库/pages/ 。原文缓存.local/tutorial不提交课件全文。

| 原第二课 | 现第三课 | 来源 |
| --- | --- | --- |
| Flask/SQLite | Go/MySQL | architecture/s2-data |
| Cookie载荷身份 | 随机Session ID、库存摘要、users身份 | auth-schemes/think |
| POST登录302、SSR | /api/login JSON 200、SPA | think |
| 单app发布8080 | web/api/db仅Nginx发布 | s7-compose |
| 详情双条件查询 | 先取行再比班级，同构404 | s4-isolation |
| 上传写讲义三表 | 仅材料/正文两表，讲义预留 | s2-data/s5-upload |
| 登出清当前Cookie | 删除服务端会话即撤销 | auth-schemes |

第二课原始四件套在docs/history/lesson-02；不是现行方案。五业务表外保留第二课领域空表。大小5MiB、cookie名、标准库是实现选择。/week03是课程站点前缀，本地/api。第三课初次验收阶段已完成实际Docker构建/转发/健康/端口/不删卷持久化，主系统在8080；操作以CMD和Compose为标准，旧本机阶段归档docs/history/lesson-03-local。

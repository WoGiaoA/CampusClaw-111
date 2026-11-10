# 教学材料删除实施验证

change保留未归档；未启停或重建主部署，未删除真实教学材料，未修改真实凭据，真实课程Chat/Embedding调用为 **0/0**。

## 实施结果

- DELETE `/api/materials/{id}`：服务端教师、班级、CSRF和现有首次改密门禁；事务内撤回、解除应用绑定和登记唯一清理任务；重复请求返回已有清理状态。
- `materials.deleted_at`、`materials.vector_uncertain`和`material_deletions`通过幂等增量迁移加入。保留非正文删除标记，正常清理移除全文、切片、索引任务、原文件和Qdrant向量。
- 全部材料、知识库、切片、下载、重建、应用绑定和检索入口遵守删除状态；本班旧资源410，跨班404。Qdrant残留候选不能绕过MySQL正文和权限复核。
- 单实例按材料协调索引和清理；删除通知取消索引，已在发送的向量写入等待确认后收尾，不能重新发布ready；后台清理可重试和重启恢复。
- 教师材料行提供危险操作与名称/影响确认，默认取消、Escape退出、提交防重、结果反馈及刷新；学生没有入口。历史正文和编号保留，删除引用附件被剥离，界面明确提示来源已删除。
- OpenAPI说明DELETE和410及引用状态；接口用例和人工验收说明同步完成。

## 验证证据

| 检查 | 结果 | 本地证据 |
| --- | --- | --- |
| Go完整回归，真实隔离MySQL及临时Qdrant | 45项通过，0失败，0跳过 | `.local/material-deletion-tests-final.log`、`.local/test-results/summary.json` |
| 新增删除生命周期 | 权限、事务回滚、重复请求、解绑、三模式排除、真实向量删除、文件及SQL清除、历史正文保留/附件剥离、两次恢复均通过 | `TestMaterialDeletionLifecycle` |
| 清理失败恢复 | 向量删除后/数据库清理后/文件删除后中断、文件删除失败、Qdrant离线及未确认写入保护通过 | `TestMaterialDeletionRecovery`，6个子场景 |
| 索引并发 | 阻塞Embedding和延迟向量upsert期间删除均通过，清理不抢先越过活动写入，最终无残留向量 | `TestMaterialDeletionDuringIndex`，2个子场景 |
| DOM回归 | 31项通过 | 全量回归日志；教师/学生、纯文本名称、CSRF/Bearer、防重、取消、焦点、刷新失败、旧来源410及失效附件显示 |
| HTTP接口脚本 | 26/26通过，含新增DELETE | `apitest/reports/report.html`、`.local/material-deletion-api-verified.log` |
| Chrome + 本地Nginx | 确认/取消、资料删除、历史来源失效、存续文件下载、旧材料页及390px布局通过；既有第5/6课配置、SSE、工具、审计、Swagger回归通过 | `.local/material-deletion-api-verified.log` |

首轮发现旧测试将数据库表数固定为23，增加清理表后已更新为24并全量复验。浏览器夹具曾误复用切换到学生后的教师会话，已用独立教师请求上下文修复；最终记录为复验通过结果。测试模型均为本地HTTP替身，不能据此宣称真实课程模型质量通过。

## 边界及用户验收

- 向量服务离线或文件删除失败时保持资料不可用、后台重试，只有全部确认才返回deleted。
- 向量写入回执丢失/进程写入中断时保留vector_write_unconfirmed并继续清扫，不自动清除确认保护标记或谎报完成；具体运维边界见 `docs/material-deletion.md`。
- 历史正文不重写；模型已收到的上下文及其他已打开页面已显示的字节无法追溯撤回，刷新或旧来源请求时会失效。
- 部署由用户执行；启动命令及逐项人工验收见 `docs/material-deletion.md`。第5课未完成的人工语义验收项及其他change状态均不改动。

# 测试结果

课程阶段：第三课最终验收。

| 验证 | 结果 | 证据 |
| --- | --- | --- |
| Go真实数据库/HTTP及单位测试 | 17通过、0跳过、0失败 | [原始输出](final-go-tests.txt) |
| 最终课程HTTP证据 | 73通过，属于上述一个Go测试 | [脱敏报文](final-http.json) |
| 前端DOM/VM | 15通过、0跳过、0失败 | [输出](final-ui-tests.txt) |
| 当前公开Nginx入口 | 12通过 | [真实状态/响应](final-runtime.json) |
| 当前容器/Nginx | 3 healthy；nginx -t退出0 | [运行证据](final-runtime.json) |
| go vet、API build、JS语法 | 通过 | [摘要](final-summary.json) |
| OpenSpec全部严格校验 | 以最终归档后JSON为准 | [校验](final-openspec-validation.json) |

方法与环境见[测试说明](../../docs/testing.md)，每项Scenario见[对照](scenario-results.md)。Go集成使用当前db内独立测试库和临时文件，前端使用模拟fetch，当前公开入口仅匿名和健康检查，未用真实用户密码。没有修改真实账号/教学材料。

历史第三课初次验收阶段：[容器持久化](container-persistence.json)、[重建后HTTP](container-http-after.json)、[故障健康](container-health-faults.json)。没有在本轮重跑Docker down/up，不把Go handler重建混称容器重启。课堂截图与主账号个人操作由用户完成。

# 第三课作业交付

第三课功能与现有扩展回归通过，可以进入提交准备。以下六项答案包含真实源码、测试结果和自行截图步骤；本仓库不生成课堂截图。

| 作业 | 说明 | 状态 |
| --- | --- | --- |
| 1 系统运行 | [入口、启动与教师首页](01-system-running.md) | 实现及运行验证通过；待自行截图 |
| 2 登录 HTTP | [请求、完整响应、Cookie](02-login-http.md) | 真实隔离 HTTP 验证通过；待自行截图 |
| 3 身份识别 | [服务端 Session 与隔离](03-session-identity.md) | 伪造身份/过期/撤销验证通过；待自行截图 |
| 4 上传 | [解析、持久化、回滚](04-upload-analysis.md) | 合法及非法输入/失败回滚通过；待自行截图 |
| 5 下载 | [权限与文件读取](05-download-permission.md) | 未登录/同班/跨班/无效 ID 通过；待自行截图 |
| 6 API 暴露 | [公开入口与内部端口](06-backend-exposure.md) | 当前 Nginx/端口/health 验证通过；待自行截图 |

[最终验收记录](lesson-03-acceptance.md)、[测试结果与范围](test-results.md)、[逐 Scenario 对照](scenario-results.md)、[复现方法](../../docs/testing.md)。

证据：[17 项 Go 原始输出](final-go-tests.txt)、[73 项脱敏 HTTP](final-http.json)、[当前公开入口 12 项及容器状态](final-runtime.json)、[前端测试](final-ui-tests.txt)、[最终校验](final-openspec-validation.json)。JSON 中的随机 httptest 地址仅为当次测试端口，不是日常访问地址。原始HTTP证据保留UTC时间戳，中文说明按第三课整理。

课程来源：已保存首页及本机 .local/tutorial 中十个关联页缓存（目标、架构、认证、安全、数据、班级隔离、上传、编排、验收、思考题），此前已读取；本轮没有重新获取在线教程。教师演示的 /week03 前缀属于其站点部署，本项目公开路径为 /api/...，应以 Network 实际请求为准。

历史证据 container-*、primary-login-http.json、http-evidence.json 及 lesson-03-regression.md 保留并标明所属阶段；不冒充最新容器重建验证。整理前版本在 ../../docs/history/lesson-03-before-final/。

提交前自行准备：教师 /materials；登录 Network 的 Payload、Headers、Response；会话请求与 Cookie 属性；教师上传成功和学生只读；同班下载、匿名 401、跨班/无效 404；docker compose ps 与 /health。真实密码、Session Cookie、CSRF、私有 .env 全部打码。课程平台要求的压缩包、命名及报告格式需按老师通知核对。

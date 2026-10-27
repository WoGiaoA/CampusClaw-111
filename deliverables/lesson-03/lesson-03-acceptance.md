# 第三课最终验收记录

结论：课程范围内的登录、可信身份、角色/班级隔离、文本入库与下载已实现，当前运行服务健康，隔离集成与前端回归通过。可进入作业提交准备；需本人补截图。没有为验收重置真实密码、修改教学材料或重启主容器。

| 要求 | 实现位置 | 实际测试与结果 |
| --- | --- | --- |
| 登录与HTTP | auth.go、json.go、web/app.js | 四角色登录200；错误/未知/限流同401；媒体/尾随JSON400；Cookie脱敏真实记录 |
| 服务端身份 | auth.go、server.go | DB摘要、每请求关联users；换发/过期/退出/改密撤销；伪造用户/角色/班级无效 |
| 角色与班级 | materials.go | 学生上传403；双向跨班详情/文件/知识库404；不存在同404；同班不同用户200是正确共享 |
| 教学文本入库 | materials.go、db.go | txt/md201、原字节下载及正文去BOM；非法类型/编码/空白400、超限413 |
| 失败一致性 | materials.go、db.go | 文件/知识库/提交前失败注入500，DB和文件无残留；启动对账模拟中断残留 |
| 页面与账户扩展 | web、passwords.go | 15项DOM测试；学生只读、注册、首次/普通改密、查看与下载入口回归 |
| 网络与健康 | deploy/nginx.conf、docker-compose.yml、server.go | 当前12项公开HTTP检查、nginx -t成功、3服务healthy；仅web映射8080 |
| 持久化 | Compose volumes、db.go | 本轮同库/文件新建Go handler仍可下载；第三课初次验收阶段独立容器down/up行/文件摘要一致 |

源码均在 ../../internal/campus/，完整链接和手动步骤见各作业。逐条规范见[Scenario对照](scenario-results.md)。

实际执行：17项Go全过（0跳过、0失败）、其中最终HTTP证据73项全过；15项前端全过；Go vet、API构建、JS语法通过。当前公开入口12项全过。不是把73项HTTP再计作73个独立Go测试。完整证据见本目录 final-*。

本次生产业务代码未发现需要修复的失败。新增acceptance_test.go，补充integration_test.go非法下载ID测试；修正规约中的“Cookie身份载荷”表述为随机ID，补充同班共享/可信身份边界，同步文档中过时的等待验收、账号口令与旧测试总数。UI、API协议、数据库架构与真实凭据未变。

技术对应：课程第三课Go服务端、MySQL、数据库Session、Nginx唯一公开入口与命名卷均采用；net/http、Cookie名、5MiB上限为项目实现选择。只做txt/md全文入库，无AI/RAG；课程站点/星期前缀不移植到本机。教程依据为已保存首页和十个关联缓存页，本轮未声称在线重新读取。

限制：当前主入口仅无凭据检查，教师完整流程由独立真实HTTP/DB覆盖，主账号截图本人补；容器重建/真实unhealthy使用明确历史证据，本轮不启停Docker。恢复测试是残留状态模拟，不是断电实验；提交结果不确定分支经代码审查，无实际断网提交实验。保留这些边界，不声称完整生产环境认证。

提交需备：六项答案、源码/README/OpenSpec、脱敏测试输出、本人六类截图；不提交.env、.local、数据库私有文件、上传私有文件、明文个人密码、Session或CSRF。不要为了打包删除本地真实数据。

OpenSpec收尾：主规格class-access、project-runtime、teaching-materials、account-password-management已同步；基础变更29/29、账号变更7/7完成并归档，活动变更0。归档前6/6（两变更+四主规格）、归档后4/4主规格严格校验通过。[归档后状态](final-openspec-state.json)，[归档前校验](final-openspec-before-archive.json)，[最终校验](final-openspec-validation.json)。

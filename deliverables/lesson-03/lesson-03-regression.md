> 历史阶段记录：本文保留迁移修正时的测试与等待状态。后续最终验收见[最终记录](lesson-03-acceptance.md)，不能将本文的旧账号/待验状态作为现状。

# 第三课 开发修正与用户验收交接

项目：D:\PKU\Projects\campusclaw-111。开发与隔离测试已完成，Docker构建/启动/重启和最终功能/UI验收由用户完成。本轮未启动Docker、Nginx或主系统，未重置业务账号，未修改业务.env、业务库、上传文件或命名卷，未生成图片、截图或浏览器自动化环境。未自动提交、推送或归档。

## 教程依据

本轮线上首页不可访问；已读本地首页及.local/tutorial十份正文缓存：scope、architecture、auth-schemes、security、s2-data、s4-isolation、s5-upload、s7-compose、s8-verify、think。缓存是历史保存内容，不代表本轮成功访问线上最新版；未缺失缓存子页。

现行Go net/http、MySQL、bcrypt、数据库Session、Nginx SPA及Compose与缓存教程一致。Cookie仅随机ID、库存摘要、每请求从users/classes取角色与班级；txt/md只保存原文件与正文，不涉及AI/RAG。标准库、Cookie名、5MiB为已写明的实现选择。第二课Flask/SQLite方案只保留在docs/history/lesson-02。

第三课初次验收阶段已有Docker迁移/验收记录，项目不是停留在仅本机运行。container-*.json/txt、primary-login-http.json及旧作业结论保留为历史，不能替代本轮用户验收。

## 已修正

1. 登录从Content-Type前缀匹配改为精确媒体类型解析，拒绝jsonp、第二个JSON、尾随垃圾、截断及超限；合法charset可登录。注册/改密复用解析器，继续拒绝未知角色/班级写入字段。
2. 页面网络中断与非JSON网关错误显示可读提示，不直接显示JSON解析异常或网关HTML，不误报为密码错误。
3. 上传已201但列表刷新失败，明确提示上传成功，并提供刷新列表；刷新不再次上传。
4. 测试不读取业务.env或隐式连接旧13306；单元模式明确报告MySQL跳过，缺DOM依赖报错；完整回归使用全新便携MySQL目录和随机凭据，结束关闭本次进程。
5. 作业文档区分历史与本轮；账号说明区分随机首启口令、已重置演示口令及个人已修改口令。

## 本轮真实结果

- Windows PowerShell5.1运行scripts/test-isolated.ps1：全新.local/regression-db/<随机ID>/data，独立随机root口令，loopback临时端口；只建campusclaw_test_*库，上传使用测试临时目录。
- **16个Go顶层测试通过，0跳过、0失败**，其中**10个真实MySQL/HTTP集成测试**。HTTP经httptest的真实loopback TCP服务，不是模拟DB，也没有经过Nginx。
- **10项DOM/VM页面测试通过，0跳过、0失败**。页面测试模拟fetch，不能替代后端或用户浏览器验收。
- go vet、API编译、node --check web/app.js通过。
- docker compose config --quiet通过，只解析配置，不启动服务。
- 便携Nginx1.28.0 -t通过；仅在临时副本替换容器DNS为loopback，不是容器内转发验证。
- 工具：Go1.27.2/windows-amd64、MySQL8.4.9、Node24.15.0、jsdom30.1.2。Docker镜像未更改。
- 证据：[摘要](regression-summary.json)、[脱敏Go输出](regression-go-tests.txt)。原始JSONL及私有日志在.local/test-results和.local/regression-db。测试数据库进程已停止，测试目录保留，不触碰业务数据。

## Scenario对应与边界

| 范围 | 本轮依据 | 结论 |
| --- | --- | --- |
| 结构/种子/幂等 | TestMySQLSchemaSeeds、TestMySQLHealthAndMissingSeed | 独立MySQL通过 |
| 登录/会话/角色/CSRF | TestMySQLAuthSessions、TestConfigAndSecrets、TestCSRF | 通过 |
| 列表/详情/知识库/下载/隔离 | TestMySQLIsolationDownload | Go及真实DB通过；Nginx存储拒绝待用户复核 |
| 上传/BOM/类型/大小/事务清理 | TestMySQLUploadRollbackRecovery、TestValidateText、TestStorageConfinedAndRollback | 通过；中断恢复为遗留状态模拟，未强杀上传进程 |
| 首次改密/注册/迁移/并发 | TestDemoResetFirstLogin、TestRegistrationPasswordManagement、TestProductionAndIncrementalMigration、TestConcurrentLoginReset | 独立库通过，未改真实账号 |
| 非法登录JSON及合法charset | TestLoginRejectsMalformedJSON、TestAccountJSONMediaType、TestMySQLLoginJSONParameters | 新增2个Scenario通过 |
| 网关/网络提示、上传成功后刷新失败 | tests/ui.test.cjs新增2项 | 新增2个Scenario通过，属页面逻辑测试 |
| health200及DB/存储故障503 | TestMySQLHealthAndMissingSeed | Go入口通过，容器unhealthy待用户复核 |
| Compose首次运行/Nginx转发/端口/卷重建 | 本轮只有静态校验，历史证据保留 | **本轮待用户最终验收** |

原34个Scenario的历史结果仍在scenario-results.md；新增4个Scenario，共38个，不能写成本轮容器38项全部通过。

## 用户启动（Windows CMD）

先启动Docker Desktop并选择Linux containers，然后：

```cmd
cd /d "D:\PKU\Projects\campusclaw-111"
scripts\init-env.cmd
docker compose up --build -d --force-recreate --wait
docker compose ps
curl.exe -i http://127.0.0.1:8080/health
```

init-env仅在.env不存在时创建，已有配置保留。force-recreate确保迁移后的web挂载和新API镜像生效，命名卷仍保留。不要执行down -v。默认入口 http://127.0.0.1:8080/login，首次账号先/profile改密，重新登录后/materials。

teacher_a/teacher_b为A/B教师，student_a1/student_b1为A/B学生。第三课初次验收阶段记录teacher_a已有个人密码，另三账号当时123456且需首次改密；本轮未重置或查询当前密码状态，后来改过就用新密码。全新空库用.env中SEED_*随机口令，可只在自己电脑用记事本查看，不截图、不提交.env。SEED_*不代表已有库当前口令，改.env不能重置已有用户。

## 用户最终验证与自行截图

1. 系统运行：compose ps三服务healthy；教师/materials有身份、班级、列表及上传表单。
2. 登录HTTP：F12→Network保留日志，查看/api/login的Payload、完整Headers和Response；成功200、错误401，密码/Cookie/CSRF打码。
3. 身份：Application查看Cookie属性，Network查看/api/me；首次改密后重新登录，旧窗口会话应失效。
4. 上传：教师txt/md上传201，列表可见、正文可读；PDF拒绝。若上传成功但刷新失败，点刷新列表，不重复上传。
5. 下载：本班学生200，无痕匿名401，A访问B材料404、无效ID404；/uploads/1/guessed.txt应404。不同身份使用不同浏览器或独立会话，同一无痕会话的多个窗口可能共享Cookie。
6. 暴露：只发布web；Network为同源/api，匿名/api/materials为401，/health免认证200。docker compose port api 8080和port db 3306应无映射。
7. 持久化：记录测试材料ID、正文及原文件；用户自行down再up（不加-v），核对仍存在。保留业务卷，仅截图课程要求的位置。

用户确认后才勾tasks8.4，开发测试不代勾。

## 可复现开发测试（可选）

```cmd
powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts\test.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts\test-isolated.ps1
```

前者为单元+页面，明确SKIP集成；后者用项目已有便携MySQL做完整隔离回归，不启动Docker。Go不在PATH时用.local/tools/go/bin/go.exe。缺jsdom报错，安装命令：npm install --prefix .local/ui --no-save jsdom@30.1.2。便携MySQL/Go未提供时明确失败，不自动下载或操作真实库。
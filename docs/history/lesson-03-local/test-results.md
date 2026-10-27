# 自动化与本机运行验收结果

第三课初次验收阶段；全部结果来自实际执行。Docker Desktop/WSL 2 尚未就绪，以下结果不代表容器验收。

| 检查 | 实际结果 | 证据 |
| --- | --- | --- |
| Go 测试 | 9 个顶层测试通过，含 5 个真实 MySQL 集成测试；没有跳过 | [原始输出](test-output.txt) |
| Go 编译、vet | 成功 | scripts/start-local.ps1 构建、scripts/test.ps1 执行 |
| 页面逻辑 | 4 项 DOM/VM 测试通过；匿名跳转、教师上传、学生只读、登录请求 | [原始输出](test-output.txt)、tests/ui.test.cjs |
| JavaScript 语法 | node --check 成功 | scripts/test.ps1 |
| 实际 HTTP | 24 项通过，经过本机 Go 同源代理；响应已脱敏 | [记录](http-evidence.json) |
| 应用探针 | 本机 healthcheck 子命令退出 0 | cmd/api/main.go |
| Nginx 配置 | 官方 Windows Nginx 1.28.0 执行 -t 成功 | deploy/nginx.conf；将上游替换为本机地址进行语法检查 |
| Compose 静态结构 | 三服务；只有 web 发布端口；uploads 仅 api 挂载 | docker-compose.yml；实际解析配置 |
| OpenSpec | 严格校验有效，无问题 | 最终 CLI 校验；[Scenario 表](scenario-results.md) |
| Docker 启动、转发、端口、卷重建 | 待验证 | 用户安装 Docker Desktop/WSL 2 后继续 |

## 数据与覆盖范围

集成测试连接实际 MySQL 8.4.9，每个测试创建独立的 campusclaw_test_<唯一值> 数据库和临时文件目录，结束只清理自己创建的数据库。没有删除已有开发数据或真实数据，没有执行 docker compose down -v。

覆盖四账号登录、未知/错误/限流同构失败、Session 摘要/换发/撤销/过期、Cookie/CSRF、角色即时生效、双班列表/详情/下载、客户端伪造归属、上传类型/编码/大小、两表事务、文件与提交失败、种子幂等、健康故障。重启恢复测试通过构造提交前后遗留文件状态并调用实际启动恢复函数完成；没有对运行中的服务器执行强制断电或进程终止。

真实 HTTP 验证使用独立本机演示数据库，上传验收材料会保留。固定测试口令仅用于隔离测试，实际演示账号口令来自被忽略的 .env。

## 复验命令

在项目根目录 PowerShell 执行：

```powershell
.\scripts\start-local.ps1
.\scripts\test.ps1 -Integration
node scripts/http-verify.cjs
```

当前本机环境已准备便携 Go/MySQL。其他机器需先安装 Go 和可用 MySQL；页面逻辑测试还需 jsdom 位于 .local/ui/node_modules/jsdom。只有提供真实 MySQL 测试连接才应将集成测试记为通过；未设置连接时跳过不算通过。

源码：[集成测试](../../internal/campus/integration_test.go)、[单元测试](../../internal/campus/unit_test.go)、[页面测试](../../tests/ui.test.cjs)、[真实请求脚本](../../scripts/http-verify.cjs)。

没有配置浏览器截图工具，没有生成图片；页面视觉及课堂截图由用户手动完成。

# 第5、6课接口测试

从项目根目录运行：

```cmd
python apitest\run_all.py
```

入口会检查用例清单与OpenAPI路径，然后启动一次性MySQL、独立本地MCP和Chat/Embedding HTTP替身。测试通过实际HTTP接口访问Go服务，并以只读SQL核对保存结果。不会读取业务`.env`、使用主数据库或调用课程网关；不需要启动主Compose项目。

## 依赖

- Windows自带Windows PowerShell 5.1；Python 3.9及以上。
- Go位于PATH或`.local/tools/go/bin/`。
- 便携MySQL解压至`.local/tools/mysql-*-winx64/`，含`bin/mysqld.exe`及客户端。工具和数据不提交仓库。
- 可选：`.local/tools/nginx-*/nginx.exe`。存在时接口请求经过独立Nginx，否则直连隔离API。

缺少依赖或任何断言失败时，入口返回非零退出码。报告保存在`apitest/reports/report.html`，仅供本地查看，不随作业或Git提交；启动失败也会生成失败报告，不复用旧结果。用例表只统计接口脚本，页末“整体执行结果”还包括夹具和可选浏览器检查，避免浏览器失败却只看到接口通过数。

## 用例组织与范围

`cases/`按URL和HTTP方法组织，例如`cases/api/login/POST.py`。`run_all.py`明确执行顺序，共享的身份和资源ID由隔离夹具提供；有依赖的脚本应通过总入口运行。

- 登录、Bearer/CSRF、跨班权限、三种知识库检索模式及SQL切片核对。
- 应用创建、配置、完整停用、删除；停用拒绝新会话和旧会话发送，历史保留。
- 模板复用、Skill上传/启停/替换；超过旧32 KB的文本完整保存及新上限拒绝。
- 多轮会话、SSE事件和增量时序、正文与数据库一致、请求去重、取消、本人会话软删除。
- MCP登记、错误令牌、连接/重连、工具发现、双工具串联、授权与审计、断开失效；另直接检查工具服务健康和JSON-RPC。
- 撤销工具授权后普通回答保存；内部调用标记跨分片拦截，失败不执行工具、不保存异常助手消息。
- 教学资料删除接口及来源失效；13份随仓库提供的已有资料上传/正文读回/跨班隔离，3份Skill样例导入校验。

模型由HTTP替身控制，MCP由编译后的本地工具进程执行。因此该报告证明接口、编排、权限和持久化，不能代替真实课程模型效果或人工同伴验收。真实网关既有证据另见开发说明，本入口不提供真实网关模式。

## 可选浏览器与其他回归

先安装Playwright并准备Chrome；设置浏览器模式后运行同一入口，验证网页与Swagger试调。此模式还需要上述便携Nginx。

```cmd
set CAMPUSCLAW_APITEST_BROWSER=1
python apitest\run_all.py
set CAMPUSCLAW_APITEST_BROWSER=
```

Go与DOM全量隔离回归、真实Qdrant专项分别执行，不把这些数量相加为接口脚本数：

```cmd
powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts\test-isolated.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts\test-rag-isolated.ps1
```

提交`apitest/`源码、测试脚本、Go测试与`web/docs/`接口文档；排除生成的`apitest/reports/`、`.local/`、日志、缓存及真实凭据。普通接口测试不启动Docker，Qdrant专项会使用独立临时容器，使用前应阅读其脚本。

## 本地演示初始化（与隔离测试不同）

服务启动后执行`python apitest\run_all.py --init-demo`，通过本机.env凭据为A班导入现有13份资料、3份Skill，完成应用绑定和本地MCP连接/工具授权。不需要手动注册或逐份上传。重复运行复用相同内容，遇到同名冲突停止，保留已有启停和其他配置；不会发送Chat，但上传后的后台索引可能调用已配置的Embedding。默认无参数运行仍只做隔离测试，不接触主站。详细步骤及密码已修改时的处理见[教师指南](../docs/teacher-quickstart.md)。

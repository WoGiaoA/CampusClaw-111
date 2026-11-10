# 第 6 课 MCP 与工具接入

## Why

CampusClaw 已能用角色指令、文本 Skill 和本班资料生成可回放的流式回答。第 6 课要让模型提出工具调用，由宿主决定是否执行，并能查证独立工具服务的结果怎样进入回答。

## What Changes

- 在主工程的 `mcp-server/` 提供带令牌的独立 HTTP JSON-RPC 服务，随现有 Compose 部署。提供两个确定性的只读教学提示工具。
- 接通教师登记、连接、重连、断开、发现清单与应用授权；在 Skill 中支持内置检索和 `工具名@服务器名`。
- 在原会话路径中聚合模型提出的工具调用、复核授权、执行并回传结果。学生可查看自己的工具过程，教师可查看本班追加式审计。
- 扩充同源 Swagger 与 `apitest/`，用真实本地 MCP 和 Mock LLM 验证两跳链路。Apply 默认真实 Chat/Embedding 调用为零，额度重置后的真实验收需用户另行明确授权。
- 首版覆盖只读工具和受控服务地址。通用 MCP 客户端互操作、带副作用的工具、作业批改、复杂统计和阶段 PDF 留在本变更范围之外。

## Capabilities

### New Capabilities

- `mcp-tool-service`：独立 MCP 进程、课程 HTTP 协议和教学工具。
- `mcp-tool-management`：受控登记、连接、发现及应用级工具授权。
- `mcp-tool-audit`：调用证据、只增审计、隔离验证和课程接口交付。

### Modified Capabilities

- `ai-applications`：扩展文本 Skill 的工具引用及保存约束。
- `ai-conversations`：扩展既有 SSE 与持久化契约，纳入模型工具循环。
- `teaching-workspaces`：把教师 MCP、应用工具绑定、工具审计及学生工具过程接入现有工作台。

## Impact

影响 Go 宿主、MySQL 增量结构、Compose 内部网络、原生工作台、OpenAPI 与接口脚本。复用第 5 课的会话、模板、确定性 Skill 路由、授权 RAG、去重及 SSE 生命周期。第 5 课开发规约仍为 17/18，真实回答内容待人工验收。未来同步依赖 UI 和第 5 课基础规约先合入；本 change 单独 Review。


## 撤权后回答修正

用户真实人工测试已观察到两工具成功及教师审计；撤销应用工具授权后，工具未执行，但模型将FunctionCallBegin内部调用文本当正文输出。补充无工具约束、流式协议标记拦截和历史显示保护，禁止把调用文本转换为执行。保留审计与现有授权校验，不自动重试。

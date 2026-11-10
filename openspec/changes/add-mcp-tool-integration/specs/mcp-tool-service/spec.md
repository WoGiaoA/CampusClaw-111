# 独立 MCP 工具服务

## Purpose

让 CampusClaw 宿主通过内部网络发现和调用独立教学工具，以可重复的服务响应验证第六课 JSON-RPC 协议、身份凭据与工具实际执行。

## ADDED Requirements

### Requirement: Compose 内部工具服务

主项目 MUST 在 `mcp-server/` 交付独立服务，由现有 Compose 构建，并向 API 提供内部 `GET /health` 和 `POST /mcp`。服务 MUST 从环境读取令牌，业务请求缺失或带错误令牌时返回 401。工具服务故障 MUST 与 API 基础健康和无工具会话隔离。

#### Scenario: 健康、鉴权和故障隔离
- **WHEN** 隔离 Compose 启动、试用错误令牌，再停止独立 MCP 服务
- **THEN** 正常时 API 可通过内部地址发现工具，工具端口未发布；错误令牌被拒且不返回发现数据；工具停机后登录、资料检索和无工具流式会话仍可用

### Requirement: JSON-RPC 生命周期与两种教学工具

服务 MUST 支持单条 JSON-RPC 2.0 初始化、工具列表和执行，回显请求 ID，并返回规范错误或文本工具结果。工具列表 MUST 提供 `formula_hint` 和 `strategy_hint` 的名称、描述及输入 Schema。两者 MUST 使用确定性规则生成分步提示。

#### Scenario: 发现、串联及错误
- **WHEN** 客户端完成握手，先按题型取公式提示，再按返回的策略键和指定步骤取后续提示
- **THEN** 两个结果可复算，不泄露最终答案；未知方法、未知工具、非法参数与超大报文得到稳定错误，且不会执行任意代码

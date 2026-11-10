# 接口验证增量

## Purpose

为课程开发与验收提供可在浏览器试调的接口契约，以及可一键运行的隔离验证和脱敏证据，以便区分本地逻辑正确性与真实网关的协议兼容性。

## ADDED Requirements

### Requirement: 同源 Swagger 与认证试调

系统 MUST 提供浏览器可打开的 Swagger 和可获取的 OpenAPI 描述，覆盖新增接口与验证所需既有接口的入参、出参、错误、鉴权和 SSE 事件。受保护接口 MUST 支持现有 Bearer，写操作 MUST 同时验证同一会话的 CSRF。

#### Scenario: 使用登录凭据试调
- **WHEN** 从登录响应或同一 Bearer 下的身份接口取得 CSRF 并在 Swagger 试调
- **THEN** Bearer 可读取本班应用并在角色允许时写入；缺失 CSRF 的写操作返回 403；SSE 增量以网络面板或流式客户端取证

### Requirement: 接口脚本与本地报告

主项目 MUST 提供 apitest/cases 下按 URL 和方法组织的一接口一脚本及 apitest/run_all.py，单次运行生成本地 HTML，失败返回非零退出码。

#### Scenario: 一键验证核心路径和错误
- **WHEN** 在隔离环境执行 runner
- **THEN** 报告覆盖登录、检索、应用与提示词、Skill、会话创建及发送等接口，验证响应、状态码、权限、关键数据库记录及流事件；已有 Go/HTTP 和前端回归继续通过

### Requirement: 模型验证分层与额度保护

自动化、启动检查和常规回归 MUST 默认禁止真实模型出网。真实验证 MUST 显式启用，仅在替身验证后使用最少短请求和已有索引，保留脱敏证据，区分替身结果和真实网关结果。

#### Scenario: 替身捕获与最小真实闭环
- **WHEN** 先完成隔离验证，再显式执行综合首问与短追问
- **THEN** 替身可查实际 model、messages、stream 和预算；真实验证记录网关增量协议、两跳到达时序、模板/技能/资料生效、历史与保存结果，记录 Chat 和查询 Embedding 次数，不重复索引或为同一证据重复成功调用

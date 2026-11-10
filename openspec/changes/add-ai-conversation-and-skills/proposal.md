# 第 5 课 AI 应用与流式会话

## Why

工作台已预留应用管理、技能和会话界面，当前 AI 助手仍调用第四课单轮问答。第 5 课需要让师生在现有工作台完成配置、带出处的连续对话与回放，并提供可复验的模型调用及接口证据。

## What Changes

- 教师管理本班应用角色指令、复用模板、启停及 Skill/材料绑定。每班预置「课程资料问答」「解题引导」。
- 教师上传并维护文本 Skill。后端按关键词确定性选中至多一项规程，学生看到实际命中的技能。
- 学生选择应用新建会话，恢复本人历史。每轮在应用允许的材料中检索，转发真实模型增量，保存成功回答与出处。
- 为新增接口交付同源 Swagger，以及主项目 `apitest/` 下按 URL/方法组织的脚本、`run_all.py` 和本地 HTML 报告。保留原 `/api/search`、`/api/ask`、材料与来源链接。
- 本 change 覆盖第 5 课。第 6 课的 `mcp-server/`、工具发现、调用循环与审计另立 change。脚本型 Skill、复杂模板系统、整站重设计及阶段验收 PDF 在本 change 范围之外。

## Capabilities

### New Capabilities

- `ai-applications`：班级应用、提示词模板、文本 Skill 与能力绑定。
- `ai-conversations`：个人会话、上下文、范围化 RAG、SSE 和持久化。
- `api-verification`：Swagger、接口测试与模型证据。

### Modified Capabilities

- `project-runtime`：将助手和技能预留结构升级为可用结构并兼容旧库。
- `teaching-workspaces`：在未归档的 `redesign-campusclaw-ui` 增量基础上，将教师应用配置和学生助手预览接入实际业务。未来同步须先合入其基础要求。

## Impact

影响 Go API、MySQL 增量迁移、检索范围、模型流式客户端、Nginx SSE 路由及原生 SPA 的两个既有入口。复用 Cookie Session、可撤销不透明 Bearer、CSRF、班级隔离、MySQL/Qdrant 索引和隔离测试。课程 JWT 示例 `2026-10-06-use-jwt-bearer-auth` 为历史参考。根据课程概览和仓库中整理的任务清单，第 5 课明确要求 `apitest/`，第 6 课要求 `mcp-server/`，未见固定 OpenSpec change 名。任务清单原始截图和分章依据见仓库第 5、6 课分析中的记录。

本轮 Propose 交付规约。待 Review 授权后才进入 Apply。



## 当前验收状态

用户明确确认第5课验收完成，tasks为18/18。实现、既有真实网关记录与用户人工复查共同构成验收依据；本次未追加真实模型调用。证据范围见tasks末尾验收记录及docs/lesson-05-development.md。保留change，不自动同步主规格或归档。第6课真实工具调用兼容性另验。

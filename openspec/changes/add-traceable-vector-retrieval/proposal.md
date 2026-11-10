# Proposal

## Why

CampusClaw 第三课已支持登录、角色权限、班级隔离及材料全文入库，但师生仍需打开整份材料人工寻找答案。本变更拟增加限定本班范围、可回到材料原文的知识库检索，完成第四课 Token 登录与基于知识库的 AI 问答开发。

最新用户要求取代此前仅规划范围：本轮实现 Token 登录和真实 RAG 问答，完成隔离自动化测试及开发文档，等待人工验收；PDF 实现方案报告暂缓，不生成或提前准备。

## What Changes

- 复用现有 Go、MySQL、Nginx、原生前端和数据库 Session；本班教师与学生均可检索，教师仍独占上传及索引重建权限。
- 为已保存的正文建立可定位切片，保留原文件与全文，命中返回材料标题、切片序号、字符区间、摘录和材料链接。
- 增加受保护的 `POST /api/search`，支持关键字、向量及混合模式，默认混合；两条检索路径和回表均使用可信会话班级，跨班内容不泄露。
- 选用 MySQL 中文 ngram 全文索引、服务端 Embedding、Qdrant 余弦检索和 RRF 排序。这是本项目采用的实现方案，不是老师强制要求使用向量数据库。
- 索引工作与材料保存分离，提供状态、失败重试和本班教师重建入口；覆盖第三课已有材料，失败不影响查看和下载原文。
- 区分无命中与依赖故障，提供空结果和可读错误；保持旧登录、改密、上传、详情和下载协议兼容。
- 新增 POST /api/ask：本班 hybrid 前4条证据调用真实模型，回答及编号引用；无命中不调用生成服务。新增不透明 Bearer Token，复用 sessions 摘要/过期/撤销及 Cookie 兼容。仍不包含自定义/标题切分、流式会话、重排序、PDF/OCR、MCP或技能业务。

## Capabilities

### New Capabilities

- `knowledge-retrieval`：本班切片索引、三模式检索、来源追溯、索引权限与失败处理。

### Modified Capabilities

- `teaching-materials`：明确原文件/全文保存事务不依赖模型；允许提交后独立建索引，保留201和既有读写边界。
- `class-access`：登录返回 Token、优先 Bearer、Cookie 兼容、撤销和浏览器携带。
- `project-runtime`：增加切片与索引任务的持久化结构、内部Qdrant与嵌入配置，保留既有服务和基础健康检查。

`class-access` 新增可撤销 Bearer Token 契约，保留原会话边界；`account-password-management` 规则直接复用。

## Impact

- 代码影响：`internal/campus` 的schema/迁移/上传后索引及新检索模块，`server.go`路由、`config.go`配置，`web`检索与出处展示；本轮按 tasks 实施这些文件。
- 部署影响：`docker-compose.yml`增加内部Qdrant及卷，`.env.example`增加嵌入服务与集合配置；不开放数据库或向量库宿主机端口。
- 数据影响：增量新增 `knowledge_chunks`、`knowledge_index_jobs`及所需索引/复合唯一键；保留用户、凭据、Session、材料ID、全文、文件和现有卷。
- 验证：双班师生、匿名/首次改密/伪造身份、Token 撤销、三模式、问答与引用、失败保留、重建与第三课回归；实际结果见交付索引，真实模型及部署留待用户验收。
- 作业范围映射、文件索引和文档校验记录见 `deliverables/lesson-04/README.md`；本轮 Apply 已由用户明确授权。

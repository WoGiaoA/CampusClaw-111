# Project Runtime Spec Delta

## MODIFIED Requirements

### Requirement: Docker Compose 标准部署

系统 MUST 提供包含 Nginx/web、Go/api、MySQL/db及内部向量库的Docker Compose标准启动方式和README操作步骤，首次启动自动初始化；系统 MUST 仅通过web发布宿主机8080，api、MySQL和向量库均不发布宿主机端口。Compose MUST 保留基于GET /health的基础healthcheck；向量库暂不可用 MUST 不阻断既有登录、材料读取和keyword模式。

#### Scenario: 按文档首次启动
- **WHEN** 在全新测试目录按 README 准备 .env 并执行 docker compose up --build -d
- **THEN** 基础服务到达healthy，http://localhost:8080/login返回200，种子账号可登录；向量服务和有效配置就绪后索引任务可完成
- **AND** README说明测试账号、变量填写、HTTP演示cookie设置、停止与保留volume的重启方法，以及索引状态与基础健康的区别

#### Scenario: 内部向量服务故障
- **WHEN** 向量服务不可用但MySQL与上传存储正常
- **THEN** 基础GET /health仍为200，登录、本班材料查看下载和已有ready切片的keyword搜索仍可处理，vector/hybrid按knowledge-retrieval返回503

## ADDED Requirements

### Requirement: 检索数据持久化与兼容迁移

系统 MUST 持久保存切片、原文位置、索引状态、任务和向量，迁移 MUST 保留现有账号密码、改密状态、Session、材料ID、全文与文件。切片、知识条目、材料和任务 MUST 具有一致班级及正确关联，非法跨班或跨材料关联 MUST 被拒绝。重复初始化 MUST 不重复正文或活动索引任务。

#### Scenario: 已有第三课数据升级
- **WHEN** 对包含师生账号与材料的第三课数据库执行迁移及缺失索引登记
- **THEN** 原账号凭据、改密状态、Session、材料ID、全文和文件字节不变，仅增加索引结构与缺失任务
- **AND** 对同一数据库重复执行不会重复材料、切片批次或活动任务

#### Scenario: 拒绝不一致归属
- **WHEN** 插入与材料或知识条目班级不一致的切片/任务，或将切片挂到不对应的材料和知识条目
- **THEN** 数据库约束拒绝写入，不产生可检索的错误归属

#### Scenario: 保留卷的重建恢复
- **WHEN** 在独立验收环境重建容器且保留数据库、文件和向量卷
- **THEN** 原有账号、材料与ready切片/向量可恢复，索引任务能继续或明确失败，不要求删除原数据重新开始

### Requirement: 嵌入配置与服务端密钥

系统 MUST 仅在服务端使用嵌入网关凭据，校验模型与向量维度配置；索引和查询 MUST 使用兼容的模型配置。缺失或不兼容配置 MUST 使向量索引/检索明确失败，不得返回伪造向量；既有登录与材料读取 MUST 保持可用。页面、API、日志和版本库 MUST 不含真实密钥，浏览器只能访问本站API。

#### Scenario: 无凭据或维度不符
- **WHEN** 未配置嵌入凭据，网关输出维度与集合不一致，或模型配置与已有ready索引不兼容
- **THEN** 向量相关路径返回503或将后台索引标记failed，错误只说明配置类别，不输出凭据；原材料仍可读取

#### Scenario: 网络与凭据边界
- **WHEN** 检查Compose端口、浏览器请求、公开API响应及提交文件
- **THEN** 仅web具有宿主机发布入口，无浏览器直连向量库/嵌入网关请求，无真实密钥、原始向量数组或私有.env内容

### Requirement: 可复现的检索验证说明

README和检索验证文档 MUST 说明Windows CMD启动/校验步骤、索引状态、失败重试、三模式差异和数据保留方法；验证记录 MUST 区分文档校验、模拟依赖、真实服务和历史证据，不能将OpenSpec校验或未执行任务写成功能验收通过。

#### Scenario: 仅完成本次规约交付
- **WHEN** 本次只生成并校验proposal、design、delta specs和tasks
- **THEN** 交付说明明确“规划完成、未实施”，实现任务全部未勾选，不生成虚假的运行结果或第四课功能截图

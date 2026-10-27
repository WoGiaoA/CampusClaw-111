# Spec Delta

## Purpose

提供可复现的课程验收环境：明确六类教学核心结构、双班师生与可区分材料种子、仅服务端持有的密钥，以及 Docker Compose 启动、免登录健康检查和文件数据库持久化约定，便于后续按规约实施与复验。

## ADDED Requirements

### Requirement: 核心数据结构

系统 MUST 建立班级、用户、讲义、作业、助手、技能六类核心结构及材料、知识库结构；用户归属一个班级，角色仅 teacher/student，材料与知识库关联同一班级。作业、助手、技能仅预留结构，不提供业务接口或页面。

#### Scenario: 初始化结构与约束
- **WHEN** 在空测试库执行初始化
- **THEN** 存在 classes、users、lectures、assignments、assistants、skills、sessions、materials、knowledge_entries 九类表
- **AND** 用户名唯一，外键有效，非法角色、无效班级以及知识库与材料班级不一致的插入被拒绝
- **AND** 作业、助手、技能表可为空，访问未来功能无可用业务页面或 API

### Requirement: 双班验收种子

系统 MUST 预置 A/B 两班、teacher_a/teacher_b 两名教师、student_a1/student_b1 两名学生及每班至少一份可区分材料和对应知识库记录；讲义仅预留结构。测试密码 MUST 通过服务端环境变量输入并哈希入库。重复初始化 MUST 不覆盖账号或已上传材料且不重复种子记录。

#### Scenario: 初次种子完成
- **WHEN** 对空数据目录提供测试口令环境变量并执行初始化
- **THEN** A/B 各有一名教师与学生，四个账号按口令可登录
- **AND** 存在“A 班-课程导论.txt”及“B 班-学习指南.md”标题、不同正文和可读取文件，每份有同班知识库关联，讲义结构独立预留

#### Scenario: 幂等且保留已上传数据
- **WHEN** 初始化两次，第二次之前已上传一条材料
- **THEN** 仍仅有两班与四个预置用户，种子记录未重复且账号哈希未被重置
- **AND** 已上传材料、知识库与文件保留

### Requirement: 环境变量与密钥边界

系统 MUST 仅从服务端环境变量获取 SESSION_KEY；缺失或为空 MUST 启动失败，禁止不安全默认值。真实密钥、测试口令和 .env MUST 不提交版本库，不暴露给页面、日志或 API；示例文件只列变量名、空占位及说明。

#### Scenario: 必需变量缺失
- **WHEN** 未设置或设置空 SESSION_KEY 启动应用，或空库初始化缺少任一测试口令变量
- **THEN** 启动失败且说明缺失变量名，不创建部分种子，不打印变量值
- **AND** 已有数据库再次启动不要求测试种子口令，但仍要求 SESSION_KEY

#### Scenario: 密钥不外泄
- **WHEN** 检查源码、版本库、渲染页面、API 响应与日志
- **THEN** 没有真实 SESSION_KEY、测试明文口令或 .env 内容；.env.example 仅含空占位与填写说明

### Requirement: Docker Compose 标准部署

系统 MUST 提供 包含 Nginx/web、Go/api、MySQL/db 的 Docker Compose 标准启动方式和 README 操作步骤，首次启动自动初始化；系统 MUST 仅通过 web 发布宿主机 8080；api 与 MySQL 不发布宿主机端口，登录页可访问。Compose MUST 提供基于 GET /health 的 healthcheck。

#### Scenario: 按文档首次启动
- **WHEN** 在全新测试目录按 README 准备 .env 并执行 docker compose up --build -d
- **THEN** 服务到达 healthy，http://localhost:8080/login 返回 200，种子账号可登录
- **AND** README 说明测试账号、变量填写、HTTP 演示 cookie 设置、停止与保留 volume 的重启方法

### Requirement: 免登录健康检查

系统 MUST 提供免登录 GET /health，就绪时返回 200 application/json 与 {"status":"ok"}；数据库不可读或存储目录不可用时 MUST 返回 503，不泄露业务内容、密钥与路径。

#### Scenario: 健康服务
- **WHEN** 无 cookie 请求就绪应用的 GET /health
- **THEN** 返回 200 与 {"status":"ok"}，不重定向登录页，不包含材料或用户数据

#### Scenario: 持久化存储不可用
- **WHEN** 在隔离环境模拟数据库不可读或上传目录不可用并请求 GET /health
- **THEN** 返回 503 与 {"status":"unavailable"}，Compose healthcheck 判定失败，响应无服务器路径

### Requirement: 持久化数据库与文件

系统 MUST 使用持久化 volume 保存数据库和上传文件，容器重建不得丢失账号、材料、知识库或文件。README MUST 说明删除 volume 会丢失数据。

#### Scenario: 重建后数据完整
- **WHEN** 上传一份材料后执行 docker compose down，再执行 docker compose up --build -d，未删除 volume
- **THEN** 原有账号仍可登录，材料与知识库标识及正文保持一致
- **AND** 本班列表仍可查到上传记录，下载文件字节保持一致



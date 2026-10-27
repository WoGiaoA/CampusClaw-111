# Spec Delta

## Purpose

让教师将教学文本保存并解析为本班材料与知识库记录，让本班教师和学生能从数据库列表读取上传结果，同时保证跨班读取被拒绝、上传失败不会留下脏数据或孤立文件。本次不包含向量检索或模型调用。

## ADDED Requirements

### Requirement: 本班材料与知识库读取

系统 MUST 提供受保护的材料页面、材料列表/详情/文件读取及知识库列表/详情 API；每个列表 MUST 查询数据库并只返回会话班级记录，每个单资源读取 MUST 遵守 class-access 班级边界。无材料时 MUST 返回正常空列表。

#### Scenario: 两班教师学生列表隔离
- **WHEN** teacher_a、student_a1、teacher_b、student_b1 分别请求 GET /materials、GET /api/materials 和 GET /api/knowledge-entries
- **THEN** 每个响应为 200，A 班仅含 A 班记录、B 班仅含 B 班记录
- **AND** 不包含另一班标题或正文；数据库新增合法本班记录后刷新即能看到，不依赖页面硬编码

#### Scenario: 读取本班文本与文件
- **WHEN** 本班成员请求 GET /api/materials/{id}、GET /api/materials/{id}/file、GET /api/knowledge-entries/{id}
- **THEN** 返回 200，详情包含本班标题及解析文本，文件下载与上传字节一致，知识库包含对应材料标识与文本
- **AND** 上传目录不能通过公开静态 URL 直接读取

#### Scenario: 本班空列表
- **WHEN** 测试班级没有材料与知识库记录且用户合法登录
- **THEN** 列表 API 返回 200 与空集合，页面显示空状态而不报错或展示别班内容

### Requirement: 教师文本上传入库

系统 MUST 接受教师上传 UTF-8 编码、扩展名 txt/md、大小不超过 5 MiB 且解析后非空的文本文件；成功 MUST 返回 201 与材料标识，持久化原文件、材料和关联知识库全文记录，班级与上传者取自可信会话。入库 MUST 不调用模型、向量服务或 RAG。

#### Scenario: txt 与 md 上传成功
- **WHEN** teacher_a 分别上传非空合法 txt、md 文件到 POST /api/materials/upload
- **THEN** 每次返回 201 与 material_id、knowledge_entry_id，材料和知识库各增加一条关联 A 班的记录
- **AND** 下载文件与原始字节一致，知识库正文为 UTF-8 解码后的完整文本（文件头 BOM 如有则移除）

#### Scenario: 上传后本班成员可见
- **WHEN** teacher_a 上传成功后，teacher_a 和 student_a1 刷新材料和知识库列表
- **THEN** 两人都可看到新材料标题与对应知识库条目
- **AND** student_a1 仍只能读取且上传为 403，B 班用户列表无新记录且按 ID 访问为 404

#### Scenario: 同名文件独立保存
- **WHEN** teacher_a 连续上传两份同名合法文件
- **THEN** 返回两个不同材料标识，两份文件及知识库记录均保留，互不覆盖

### Requirement: 上传失败一致性

系统 MUST 拒绝非法上传并返回明确错误；文件保存、解析、数据库写入或提交失败时 MUST 不留下本次材料/知识库记录与孤立文件，不改变已有数据。进程意外中断的残留 MUST 在下一次服务就绪前清理或恢复一致。

#### Scenario: 输入或解析失败
- **WHEN** 教师提交缺失文件、exe 文件、空文件、纯空白文本、无效 UTF-8 或大于 5 MiB 的文件
- **THEN** 超限返回 413，其余返回 400，说明失败原因但不泄露服务器路径
- **AND** 材料、知识库行数及文件集合与请求前一致

#### Scenario: 文件保存失败
- **WHEN** 在隔离测试目录模拟无写入权限或磁盘写入异常，教师提交合法文件
- **THEN** 返回 500，不返回成功标识；数据库无新增记录，临时文件和部分写入文件已清理

#### Scenario: 数据库第二次写入或提交失败
- **WHEN** 在隔离测试库注入知识库插入失败或事务提交失败
- **THEN** 返回 500，材料与知识库均无本次记录
- **AND** 本次临时/最终文件均已删除，已有材料和文件保持原样

#### Scenario: 上传进程中断后恢复
- **WHEN** 在保存文件后、提交事务前中断进程并重新启动
- **THEN** 服务就绪前移除未被已提交材料关联的临时或最终文件，不出现不完整记录
- **AND** 若提交已完成则保留关联文件与两条记录，已有文件不被误删

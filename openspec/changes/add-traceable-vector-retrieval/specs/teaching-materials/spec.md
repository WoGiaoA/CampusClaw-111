# Teaching Materials Spec Delta

## MODIFIED Requirements

### Requirement: 教师文本上传入库

系统 MUST 接受教师上传 UTF-8 编码、扩展名 txt/md、大小不超过 5 MiB 且解析后非空的文本文件；成功 MUST 返回 201 与材料标识，持久化原文件、材料和关联知识库全文记录，班级与上传者取自可信会话。材料保存事务 MUST 不调用模型、向量服务或生成流程；提交后 MUST 为该材料登记独立索引任务。201 MUST 仅表示原材料及全文已保存，不表示索引ready；索引失败 MUST 保留已提交材料与文件，并允许恢复登记或教师重试。

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

#### Scenario: 索引依赖失败不回滚原材料
- **WHEN** 文件和两条记录已提交，但随后任务登记、嵌入或向量写入失败
- **THEN** 上传保存结果仍为201，原有两个ID有效，查看与下载仍可用
- **AND** 索引状态显示pending或failed而非ready，待恢复登记或教师重试；未登记任务能由幂等补齐流程发现

## ADDED Requirements

### Requirement: 材料索引状态兼容读取

系统 MUST 在现有本班材料详情中增加index_status和chunk_count，状态读取 MUST 不依赖嵌入或向量服务；chunk_count MUST 表示当前ready切片数，非ready时为0。原有字段、ID、详情/下载权限与正文 MUST 保持兼容，不能因索引不可用拒绝本班原文读取。

#### Scenario: 向量依赖中断仍可读取
- **WHEN** 本班用户在向量服务不可用时查看GET /api/materials/{id}或下载原文件
- **THEN** 仍按第三课规则返回200及原文/文件，详情包含已有索引状态，不临时向外部服务查询状态

#### Scenario: 跨班状态不泄露
- **WHEN** A班用户读取B班材料详情以探测索引状态
- **THEN** 返回404，不包含标题、正文、状态或切片计数

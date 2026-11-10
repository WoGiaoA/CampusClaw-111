# 运行时增量

## MODIFIED Requirements

### Requirement: 核心数据结构

系统 MUST 建立班级、用户、讲义、作业、助手、技能及材料、知识库结构。用户归属一个班级，角色仅 teacher/student，材料与知识库关联同一班级。助手和技能 MUST 支持班级配置，并增加模板、绑定、会话及消息结构。作业继续预留。

#### Scenario: 初始化结构与约束
- **WHEN** 在空测试库执行初始化
- **THEN** 存在 classes、users、lectures、assignments、assistants、skills、sessions、materials、knowledge_entries 及新增会话配置结构
- **AND** 用户名唯一，外键有效，非法角色、无效班级、跨班绑定及知识库与材料班级不一致的插入被拒绝
- **AND** 作业表可为空且作业仍无可用业务接口，助手和技能提供本班预置配置

#### Scenario: 重复升级已使用的数据库
- **WHEN** 含账号、Session、资料索引及旧助手和技能预留行的库执行升级，教师修改预置配置后再运行升级
- **THEN** 旧数据及已设置的应用配置保留，种子不重复，原有索引无需重建

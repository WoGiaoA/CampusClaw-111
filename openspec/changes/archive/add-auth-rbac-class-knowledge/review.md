> 历史记录：以下为第二课 第二课 Review；第三课已更新基线并授权 Apply，当前结果见 deliverables/lesson-03。

# 第 2 课规约 Review

课程阶段：第二课。本次为同一对话中的制品 Review，没有执行 Apply、业务测试或 Docker。

## 依据与范围

已读取本地《封面 · 第2课 · SDD 与 CampusClaw 规约.html》第 35–41、42–52、53–56 页。课件内容已核对，无不可访问的指定页；产品定位按用户明确的高校背景。截图未转化为复刻要求。

proposal 列出全部必需非目标；未来作业、助手、技能仅空表。tasks 没有 AI、RAG、模型调用、技能管理、作业流程、注册或完整演示页面的实现任务。

## 需求与验收覆盖

| 检查点 | 依据与结论 |
| --- | --- |
| 正常/失败登录、未登录、角色权限 | class-access：正确登录、错误口令/未知账号、页面 302、API 401、学生上传 403 均有明确 Scenario |
| 班级边界 | class-access + teaching-materials：双向跨班详情/文件/知识库 404、列表同班、伪造 class_id 无效；过滤在服务端 |
| 保存、解析与入库 | teaching-materials：txt/md、201、材料/知识库关联、本班教师学生可见、同名不覆盖；禁止向量/RAG/模型 |
| 失败一致性 | 输入 400/413、保存/插入/提交失败 500、行数与文件集合不变、启动残留恢复；design 三表事务与补偿清理对应 |
| 数据与安全 | project-runtime + class-access：六类核心加材料/知识库、双班四账号、差异材料、bcrypt、种子幂等、环境变量密钥 |
| 部署 | project-runtime：Compose 首启、免登录 health 200/故障 503、named volumes、重建后文件与记录保留 |
| tasks 覆盖 | 14 条 Requirement 全部有任务映射，32 条 Scenario 均含 WHEN/THEN；24 项任务均带 verify 且全为 [ ] |

## Review 修正

- 统一跨班 404、未登录 API 401、学生上传 403，不保留“403 或 404”歧义。
- 明确 Flask 默认 session 是客户端签名 cookie，可信身份由服务端设置并验证，不误称为服务端存储会话。
- 明确讲义、材料、知识库同事务，失败同时回滚；文件读取必须受保护。
- 增补 materials.seed_key 让种子幂等可实施；明确所有登录角色获取 CSRF token 的方式，便于独立验证学生上传 403。
- 统一 UTF-8 BOM 处理，原文件保持字节不变，解析记录移除文件头 BOM。

## 校验与限制

OpenSpec 版本 1.14.1；运行 `openspec validate add-auth-rbac-class-knowledge --strict`，退出码 0，change valid；规划制品 4/4 完成。该状态不代表业务实现完成。

最初 CLI 被沙箱 native realpath 的 EPERM 阻止读取全局 schema；允许读取后命令恢复正常，未修改全局安装或 course-demo。

无未解决的规约阻塞问题。业务行为、Docker 运行及课件第 56 页独立新对话验收尚未执行，均属后续验证；本次只能确认制品完整、需求可判定和任务覆盖，不能声称应用已通过验收。


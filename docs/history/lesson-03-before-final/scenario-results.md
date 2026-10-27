# Scenario 逐项验收

> **第三课最终整理阶段状态：开发修正与隔离测试已通过，等待用户启动 Docker 并最终验收。** 下方原有容器运行结论与 JSON/TXT 文件属于 **第三课初次验收阶段的历史证据**，不是本轮重新部署结果。最新测试边界及操作顺序见 [本轮复验](lesson-03-regression.md)。截图全部由用户完成。

第三课初次验收阶段。共 15 个 Requirement、34 个 Scenario，最终全部按下表方法通过。测试已在实际 Go/Nginx/MySQL Compose 中执行，13 个 Go 测试无跳过、91+28 个真实容器HTTP通过；主库完整迁移且密码/材料保留。上传进程中断恢复仍采用可控遗留状态模拟，未强杀上传进程。

证据简写：A = TestMySQLAuthSessions；D = TestMySQLIsolationDownload；U = TestMySQLUploadRollbackRecovery；S = TestMySQLSchemaSeeds；H = TestMySQLHealthAndMissingSeed；UI = tests/ui.test.cjs；HTTP = container-http-first.json / container-http-after.json。Go 原始输出 container-go-tests.txt，页面 container-ui-tests.txt。源码与完整证据见 [测试结果](test-results.md)。

## class-access（15 项）

| Scenario | 结果 | 实际依据 |
| --- | --- | --- |
| 四类种子账号正常登录 | PASS | A、HTTP；A/B 教师学生 200 |
| 错误密码或未知账号 | PASS | A、HTTP；相同 401/JSON |
| 未登录请求 | PASS | A、HTTP、UI；API 401，页面遇 401 导航登录 |
| 篡改与过期会话 | PASS | A；伪造、过期 ID 均 401 |
| 登出当前会话 | PASS | A、HTTP；撤销后重放 401 |
| 学生绕过页面直接上传 | PASS | U、HTTP；403，文件和两表不变 |
| 教师进入上传流程 | PASS | U、HTTP；有效登录和 CSRF 后 201 |
| 学生页面只读 | PASS | UI；无上传表单，读取与下载可用 |
| A 班教师与学生访问 B 班资源 | PASS | D、HTTP；详情/下载与不存在 ID 同为 404 |
| 伪造查询参数和上传归属 | PASS | A、D、U；依据服务端身份而非参数 |
| 哈希存储与验证 | PASS | S、单元测试；bcrypt、独立随机盐、密码验证 |
| cookie 属性 | PASS | A、单元测试、HTTP；HttpOnly/Lax/Path/期限，Secure 开关 |
| 缺少写请求 token | PASS | A、U；上传与登出 403 |
| 登录换发与登出撤销 | PASS | A；旧 Session 撤销、退出撤销 |
| 三种失败同构 | PASS | A；未知、错误、限流同状态码和 JSON，bcrypt 工作路径 |

## teaching-materials（10 项）

| Scenario | 结果 | 实际依据 |
| --- | --- | --- |
| 两班教师学生列表隔离 | PASS | D、HTTP；仅本班材料 |
| 读取本班文本与文件 | PASS | D、HTTP；正文可见、原文件字节一致 |
| 本班空列表 | PASS | D；空班返回 200 和空数组 |
| txt 与 md 上传成功 | PASS | U、HTTP；201，两表同班记录、文件存在 |
| 上传后本班成员可见 | PASS | U、HTTP；学生可读正文及下载 |
| 同名文件独立保存 | PASS | U；独立随机路径、不覆盖旧文件 |
| 输入或解析失败 | PASS | U、单元测试；PDF/DOCX/图片/空文本/坏 UTF-8/超限拒绝且无残留 |
| 文件保存失败 | PASS | U；实际不可用存储及注入失败，500，无新增数据 |
| 数据库第二次写入或提交失败 | PASS | U；隔离故障注入、回滚及文件清理，已有数据保留 |
| 上传进程中断后恢复 | PASS（状态模拟） | U；构造提交前残留与提交后完整状态，再执行实际 Reconcile；未执行强杀进程 |

## project-runtime（9 项）

| Scenario | 结果 | 实际依据 |
| --- | --- | --- |
| 初始化结构与约束 | PASS | S；九表、角色/外键/用户名约束；未来功能无接口 |
| 初次种子完成 | PASS | S、A、HTTP；双班四账号及可区分文件/正文 |
| 幂等且保留已上传数据 | PASS | S、U；重复初始化不重置账号和上传记录 |
| 必需变量缺失 | PASS | 单元测试、H；缺密钥失败、缺种子口令不写部分种子 |
| 密钥不外泄 | PASS | 忽略规则、源码扫描、脱敏 HTTP 证据；真实环境值未写入交付 |
| 按文档首次启动 | PASS | CMD 构建启动实测，独立空卷初始化三服务healthy；四种子初始登录后强制改密按最新账号规约 |
| 健康服务 | PASS | H、主入口、HTTP、容器探针；无Cookie200与仅status JSON |
| 持久化存储不可用 | PASS | container-health-faults.json；目录权限/DB停机503，Compose实际unhealthy后恢复healthy |
| 重建后数据完整 | PASS | container-persistence.json、28项重建后HTTP；新容器ID，旧卷与用户/材料/知识库/文件摘要相同 |

## 最终范围

已使用独立验收项目完成首次初始化、实际Nginx转发、端口隔离、故障健康及不删卷持久化。主库原4用户/6材料/6知识库/文件逐行摘要一致，teacher_a个人密码保留，另三账号保留123456与首次标记。注册和改密按独立账号变更补充课堂规约，没有把旧非注册范围当成当前行为。

34项均具证据后更新tasks 6.3、6.4、7.2；变更不自动归档。

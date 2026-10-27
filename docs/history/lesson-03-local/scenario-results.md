# Scenario 逐项验收

第三课初次验收阶段。共 15 个 Requirement、34 个 Scenario。31 项在本机通过，1 项部分通过，2 项待验证。PASS 只表示下表列明的环境和方法，没有把本机开发代理当作 Nginx 容器。

证据简写：A = TestMySQLAuthSessions；D = TestMySQLIsolationDownload；U = TestMySQLUploadRollbackRecovery；S = TestMySQLSchemaSeeds；H = TestMySQLHealthAndMissingSeed；UI = tests/ui.test.cjs；HTTP = http-evidence.json。源码与完整输出见 [测试结果](test-results.md)。

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
| 按文档首次启动 | PENDING | Docker/WSL 未就绪；本机启动不替代 Compose 首启 |
| 健康服务 | PASS（本机） | H、HTTP、探针；无 Cookie 200 和仅 status JSON |
| 持久化存储不可用 | PARTIAL | H 验证 DB/目录故障 503；Compose healthcheck 状态仍待实际运行 |
| 重建后数据完整 | PENDING | 配置已有命名卷；必须实际 down/up 后核对正文、ID 和下载字节 |

## 尚待完成

Docker 就绪后使用独立验收项目与卷，执行 Compose 首启、Nginx 同源 HTTP、实际发布端口检查、故障 healthcheck 与不删卷的 down/up 持久化验证。不得将配置解析或 Windows Nginx 语法检查代替这些结果。

tasks 6.3、6.4、7.2 保持未勾选；完整 Scenario 验收完成前不归档变更。

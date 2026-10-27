# Scenario 验收对照（第三课）

逐条按当前规格列出证据，不以OpenSpec格式校验代替运行测试。Go输出见[final-go-tests.txt](final-go-tests.txt)，HTTP见[final-http.json](final-http.json)，前端见[final-ui-tests.txt](final-ui-tests.txt)，公开入口见[final-runtime.json](final-runtime.json)。部分部署/视觉场景明确使用较早阶段或第三课初次验收阶段的历史证据，未重新启停主服务。

## class-access

| Scenario | 验证结果及实际范围 |
| --- | --- |
| 四类种子账号正常登录 | 通过：TestFinalCourseworkEvidence；TestDemoResetFirstLogin；UI登录导航 |
| 错误密码或未知账号 | 通过：TestFinalCourseworkEvidence；TestMySQLAuthSessions：同401/同正文 |
| 未登录请求 | 通过：TestMySQLAuthSessions；final-runtime.json；UI匿名导航 |
| 篡改与过期会话 | 通过：TestMySQLAuthSessions：随机ID篡改/过期401 |
| 登出当前会话 | 通过：TestFinalCourseworkEvidence：logout与重放401 |
| 学生绕过页面直接上传 | 通过：TestFinalCourseworkEvidence；TestMySQLUploadRollbackRecovery：403且无副作用 |
| 教师进入上传流程 | 通过：TestFinalCourseworkEvidence：txt/md201 |
| 学生页面只读 | 通过：tests/ui.test.cjs：student无上传；服务端403 |
| A 班教师与学生访问 B 班资源 | 通过：TestFinalCourseworkEvidence；TestMySQLIsolationDownload：双向资源404 |
| 伪造查询参数和上传归属 | 通过：TestFinalCourseworkEvidence：列表/上传仍是本班 |
| 哈希存储与验证 | 通过：TestMySQLSchemaSeeds：bcrypt盐、正误验证及表结构 |
| cookie 属性 | 通过：TestMySQLAuthSessions；TestCSRF：HttpOnly/SameSite/Path/Secure配置 |
| 缺少写请求 token | 通过：TestMySQLAuthSessions；TestMySQLUploadRollbackRecovery：403 |
| 登录换发与登出撤销 | 通过：TestMySQLAuthSessions：旧ID401；DB当前角色 |
| 三种失败同构 | 通过：TestMySQLAuthSessions：未知/错误/限流同401 |
| 非法登录请求不进入认证 | 通过：TestLoginRejectsMalformedJSON：6种输入400；当前入口400 |
| 合法JSON媒体参数 | 通过：TestMySQLLoginJSONParameters：charset成功200 |
| 同班跨用户读取共享材料 | 通过：TestFinalCourseworkEvidence：同班学生读取教师资源200 |
| 客户端伪造其他用户身份 | 通过：TestFinalCourseworkEvidence：伪造query/header仍是当前学生 |

## project-runtime

| Scenario | 验证结果及实际范围 |
| --- | --- |
| 初始化结构与约束 | 通过：TestMySQLSchemaSeeds：9表/角色/外键/唯一约束 |
| 初次种子完成 | 通过：TestMySQLSchemaSeeds；TestFinalCourseworkEvidence：A/B种子可读 |
| 幂等且保留已上传数据 | 通过：TestMySQLSchemaSeeds；TestProductionAndIncrementalMigration |
| 必需变量缺失 | 通过：TestConfigAndSecrets；TestMySQLHealthAndMissingSeed |
| 密钥不外泄 | 通过：TestConfigAndSecrets；源码/忽略配置及脱敏证据检查（不公开.env） |
| 按文档首次启动 | 通过：第三课初次验收阶段 container-services/container-http-first；本轮只复核现有3服务healthy及页面200（未重做空环境启动） |
| 健康服务 | 通过：final-runtime.json：公开免cookie200；TestMySQLHealthAndMissingSeed |
| 持久化存储不可用 | 通过：TestMySQLHealthAndMissingSeed：隔离DB/目录503；第三课初次验收阶段 container-health-faults的真实unhealthy |
| 重建后数据完整 | 通过：第三课初次验收阶段 container-persistence/container-http-after：独立容器重建；本轮仅同库Go handler重建，不混称容器重启 |

## teaching-materials

| Scenario | 验证结果及实际范围 |
| --- | --- |
| 两班教师学生列表隔离 | 通过：TestMySQLIsolationDownload；TestFinalCourseworkEvidence；DOM师生页面 |
| 读取本班文本与文件 | 通过：TestFinalCourseworkEvidence：详情/原文件/知识库200与字节一致 |
| 本班空列表 | 通过：TestMySQLIsolationDownload：材料API空集合；DOM空状态；知识库SQL按班级审查 |
| txt 与 md 上传成功 | 通过：TestFinalCourseworkEvidence；TestMySQLUploadRollbackRecovery：201/两表/BOM |
| 上传后本班成员可见 | 通过：TestMySQLUploadRollbackRecovery；TestFinalCourseworkEvidence：同班可见/跨班404 |
| 同名文件独立保存 | 通过：TestMySQLUploadRollbackRecovery：不同ID与文件 |
| 输入或解析失败 | 通过：TestMySQLUploadRollbackRecovery；TestValidateText；最终HTTP400/413 |
| 文件保存失败 | 通过：TestMySQLUploadRollbackRecovery：file-write注入500且无残留 |
| 数据库第二次写入或提交失败 | 通过：TestMySQLUploadRollbackRecovery：knowledge-insert/commit提交前注入500且回滚；非真实网络不确定提交 |
| 上传进程中断后恢复 | 通过：TestMySQLUploadRollbackRecovery：构造提交前残留、提交后保留/缺文件状态并Reconcile；通过残留模型验证，未杀真实进程 |
| 上传成功后列表刷新失败 | 通过：DOM：201后刷新失败保留成功并可刷新，不诱导重传 |
| 网关错误及断网 | 通过：DOM：非JSON网关/模拟fetch失败，提示可读 |
| 桌面窗口布局 | 通过：docs/lesson-03-ui-layout.md：此前浏览器fixture多尺寸检查 |
| 窄屏或低高度窗口 | 通过：docs/lesson-03-ui-layout.md：窄屏/低高度、内容可达 |
| 紧凑桌面与长列表 | 通过：docs/lesson-03-ui-layout.md：短长列表布局检查 |
| 文件选择与窄屏 | 通过：DOM原生file协议；此前浏览器窄屏检查 |
| 材料工作区与操作层次 | 通过：docs/lesson-03-ui-refresh.md、ui-layout：先前实测与DOM |
| 文件选择与异步反馈 | 通过：DOM：文件名/大小、安全文本、FormData/CSRF、busy及reset |
| 全站表单与正文交互 | 通过：DOM：登录/注册/个人协议及正文关闭/Escape清理 |

## account-password-management

| Scenario | 验证结果及实际范围 |
| --- | --- |
| 生产禁用 | 通过：TestProductionAndIncrementalMigration；DOM生产不注册 |
| 四账号重置与数据保留 | 通过：TestDemoResetFirstLogin：仅独立测试库执行，不重置主用户 |
| 启动不重置 | 通过：TestDemoResetFirstLogin；TestProductionAndIncrementalMigration |
| 注册学生 | 通过：TestRegistrationPasswordManagement；DOM注册请求 |
| 防止提权及非法输入 | 通过：TestRegistrationPasswordManagement：未知字段/非法班级400，重复409 |
| 首次登录限制 | 通过：TestDemoResetFirstLogin：业务403；DOM导航/profile |
| 修改成功及全部会话撤销 | 通过：TestRegistrationPasswordManagement：新旧口令与全部Session |
| 错误输入及越权 | 通过：TestRegistrationPasswordManagement；TestAccountJSONMediaType：401/403/400 |
| 凭据并发安全 | 通过：TestConcurrentLoginReset：旧凭据不产生有效新Session |

共56项场景均有对应测试、代码审查或标明验证阶段的运行证据。恢复以中断残留模型测试，空环境首次启动和容器重建使用历史实测；不声称这些在本轮重做。课堂本人截图仍待准备。

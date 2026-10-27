# 第三课实施 Review

## 最终容器验收

第三课初次验收阶段已实际完成：主系统 Nginx/Go/MySQL healthy，原4用户/6材料/6知识库/文件与密码哈希无损迁移；teacher_a个人密码不重置。91+28容器HTTP、额外同源Origin注册、13Go测试（9DockerMySQL）和8页面测试通过。目录权限与数据库停机503/unhealthy恢复、独立卷实际down/up以及主项目CMD启动/stop/restart通过。进度18/18，严格校验无问题，未归档，未生成截图。证据见deliverables/lesson-03。

## 此前本机阶段记录（历史）

第三课初次验收阶段。已依据第三课完整关联教程调整技术基线，原第二课四件套保存在 docs/history/lesson-02。当前只维护 Go/MySQL/服务端 Session/Nginx Compose 一套方案，未实现 AI、RAG、作业或技能业务。

已实现并本机运行：登录、哈希口令、数据库 Session 摘要、角色/班级鉴权、txt/md 保存解析与两表入库、本班列表/正文/下载、失败回滚/恢复、三类页面和健康探针。六项作业文档及脱敏证据已完成，未生成截图。

验收：9 个 Go 顶层测试（含 5 个实际 MySQL 集成测试）、4 项页面逻辑、24 项真实 HTTP 检查通过；Nginx 语法与 Compose 静态结构检查通过。严格校验命令 openspec validate add-auth-rbac-class-knowledge --strict --json 返回 valid=true、issues=[]。

Apply 进度 15/18。用户明确要求先继续非 Docker 工作，由用户安装 Docker Desktop/WSL 2。6.3 容器启动及端口、6.4 卷重建、7.2 全部 Scenario 最终验收保持 [ ]；没有运行 Docker，没有将静态配置等同容器通过。

34 个 Scenario：31 项按记录的本机验证方法通过、1 项部分通过、2 项待验证。中断恢复使用遗留状态模拟和实际恢复函数验证，未强杀进程。具体逐项结果与尚待验证范围见 deliverables/lesson-03/scenario-results.md。

本机开发数据与上传保留，未执行 down -v。真实凭据仅在忽略的 .env，公开文件扫描没有发现其值。不归档当前变更，等待容器验收完成。

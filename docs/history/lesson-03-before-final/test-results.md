# 最终容器测试结果

> **第三课最终整理阶段状态：开发修正与隔离测试已通过，等待用户启动 Docker 并最终验收。** 下方原有容器运行结论与 JSON/TXT 文件属于 **第三课初次验收阶段的历史证据**，不是本轮重新部署结果。最新测试边界及操作顺序见 [本轮复验](lesson-03-regression.md)。截图全部由用户完成。

第三课初次验收阶段，已实际执行。主系统三服务healthy、仅Nginx8080发布；备用本机进程已停止，源数据保留。

- Docker Engine29.8.2、Compose5.6.0，Linux containers。
- Go镜像内13个顶层测试通过（9个实际Docker MySQL集成），无失败/跳过；go vet、编译成功。见container-go-tests.txt。
- 8项DOM/VM页面逻辑通过，见container-ui-tests.txt；没有浏览器截图环境。
- 独立Compose首次91项HTTP与实际不删卷重建后28项HTTP通过；登录/改密/注册/上传下载/隔离均经过Nginx。
- 额外验证浏览器Origin：Nginx同源注册201、跨源403，见container-origin.json。
- 主系统三个初始账号登录与首次限制通过，teacher_a已个人改密完整保留。
- 原4用户/6材料/6知识库逐行内容SHA256及6原文件SHA256迁移一致；不删除源数据库或文件。
- actual上传目录权限/数据库停机503、unhealthy，恢复healthy；未将静态配置当作运行结果。
- 独立验收容器实际down/up，确认新容器ID、原卷保留、哈希/记录/文件不变；不运行down -v。
- 主项目CMD构建启动、stop、restart实际验证；init-env.cmd已通过Windows内置PowerShell5执行，不需PowerShell7。
- 34个第三课Scenario全部按scenario-results列明方法通过；账户变更9个Scenario经测试通过，严格OpenSpec校验通过。

测试仅创建随机campusclaw_test_*库、临时上传目录与独立campusclaw-acceptance卷。其功能测试修改的是隔离用户；主库个人密码与教学材料没有被验收覆盖。验收项目停止保留卷。

源代码：internal/campus/integration_test.go、passwords_test.go、unit_test.go、tests/ui.test.cjs、scripts/container-verify.cjs。脚本first只对全新隔离验收项目使用；不要对真实账号重复初始化测试。post阶段用私有.local状态验证持久化，交付没有密码/Cookie/CSRF。

原本机证据归档docs/history/lesson-03-local。未生成截图；恢复测试包含故障状态模拟，并非强杀上传进程。

# 开发测试与证据边界

第三课最终记录位于 [第三课交付](../deliverables/lesson-03/README.md)。测试不读取业务 .env、不使用真实账号口令；integration_test.go 的 testServer 创建唯一 campusclaw_test_* 库及临时上传目录，结束仅清理该测试库。缺少 MYSQL_TEST_ADDR 时集成测试 SKIP，不能把默认 go test 退出0等同全量通过。

## Windows CMD 常规开发

需要 Go（模块最低版本见 go.mod）、Node.js 与前端测试依赖：

    npm install --prefix .local/ui --no-save jsdom@30.1.2
    powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts\test.ps1

默认单位模式会显式报告 MySQL 跳过，并运行 vet、API build、JS语法与前端 DOM 测试。前端为原生静态文件，没有单独 npm build。全量独立 Windows MySQL 运行需先将官方便携 MySQL 解压到 .local/tools/mysql-*-winx64（含 bin/mysqld.exe）：

    powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts\test-isolated.ps1

该脚本使用新的 .local/regression-db 随机目录和随机口令，测试后停止本次进程，保留忽略的测试文件。Go 可安装至 PATH 或 .local/tools/go/bin。无需 PowerShell7。.local 工具不随仓库提交，不保证新电脑已具备依赖。

## 本轮真实执行方法

本轮利用已运行 MySQL 容器的 loopback，用 Windows Go 交叉编译 Linux amd64 测试二进制，复制进 db 容器；没有启停、重建主服务。以下是内置 Windows PowerShell 可复现的核心步骤，根目录执行；后续执行前应自行确定没有相同临时文件名的并发测试。

    $env:GOOS='linux'
    $env:GOARCH='amd64'
    $env:CGO_ENABLED='0'
    .local/tools/go/bin/go.exe test -c -o .local/final-campus-tests ./internal/campus
    docker compose cp .local/final-campus-tests db:/tmp/campusclaw-final-tests-lesson03
    docker compose exec -T db chmod 700 /tmp/campusclaw-final-tests-lesson03
    docker compose exec -T db sh -c 'MYSQL_TEST_ADDR=127.0.0.1:3306 CAMPUSCLAW_EVIDENCE_PATH=/tmp/campusclaw-final-http-lesson03.json /tmp/campusclaw-final-tests-lesson03 -test.v'
    docker compose cp db:/tmp/campusclaw-final-http-lesson03.json .local/final-http-rerun.json
    docker compose exec -T db rm -f /tmp/campusclaw-final-tests-lesson03 /tmp/campusclaw-final-http-lesson03.json

MYSQL_ROOT_PASSWORD 由已有 db 容器环境提供，不打印或在命令中展开。只在独立测试库建表、注册、重置、改密和故障注入，原业务库不操作。测试导出 Cookie、口令、CSRF 打码，二进制和容器内临时报告本轮已清理。恢复 Windows 编译需在新的 CMD/PowerShell 执行，避免继承 Linux GOOS；本轮另执行 go vet ./... 与 go build ./cmd/api 通过。

## 检查层次

- final-go-tests.txt：17 项顶层测试全过，0跳过，包含11项真实 MySQL 集成、JSON边界/配置/存储/CSRF测试。
- final-http.json：其中一个测试的73项真实 HTTP 请求，使用 httptest 随机端口和真实 MySQL，不经过 Nginx；不是额外73个Go测试。
- final-ui-tests.txt：15项 DOM/VM 测试使用模拟 fetch，验证表单协议、导航、权限入口、加载和错误反馈，不等于浏览器端真实业务操作。
- final-runtime.json：当前公开 Nginx 的12项 HTTP检查、三服务健康及 nginx -t；仅匿名/页面/探针，未用个人密码重登主系统。
- docs/lesson-03-ui-layout.md：此前浏览器多视口与 fixture 检查，未在本轮重复视觉优化。
- container-persistence.json / container-http-after.json：第三课初次验收阶段 独立 campusclaw-acceptance 项目的容器重建后数据/字节验证；本轮仅Go handler同库重建验证，不能冒称重新做过 Docker down/up。
- container-health-faults.json：第三课初次验收阶段 真实容器故障503/unhealthy；本轮使用隔离DB关闭/目录故障验证503，不停止主库。

## 已知验证限制

没有在本轮杀死真实上传进程、断电或模拟网络提交结果不确定；通过构造中断残留状态、注入提交前错误与对账测试覆盖恢复分支。实际容器重建沿用历史证据。完整本人口令、截图和最终课堂平台材料由用户准备。单实例教学系统不宣称具有多实例共享文件对账或分布式限流能力。

旧 http-verify.cjs、verify-demo-accounts.cjs 等脚本可能假设历史口令或本机入口，不应直接对当前业务环境运行。container-verify.cjs 为历史独立验收流程；不要为了补证据将其指向主库。

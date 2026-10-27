# 作业1：系统运行

> **第三课最终整理阶段状态：开发修正与隔离测试已通过，等待用户启动 Docker 并最终验收。** 下方原有容器运行结论与 JSON/TXT 文件属于 **第三课初次验收阶段的历史证据**，不是本轮重新部署结果。最新测试边界及操作顺序见 [本轮复验](lesson-03-regression.md)。截图全部由用户完成。

## 结论与实际结果

主地址 http://127.0.0.1:8080/login；GET /health 实际 200，web/api/db 均 healthy。Nginx 使用 Go API 与 MySQL，非原 Go 开发代理。原4用户、6材料、6知识库和文件完整迁入新卷，逐行 SHA256（含 bcrypt 哈希）及文件 SHA256 一致。

## Windows CMD

    cd /d "D:\PKU\Projects\campusclaw-111"
    docker compose up --build -d --wait
    docker compose ps
    curl.exe -f http://127.0.0.1:8080/health

停止 docker compose stop；再次启动 docker compose up -d --wait；重启 docker compose restart，随后 docker compose up -d --wait。不删卷。上述 CMD 命令已实际执行验证。

## 账号与目标页面

teacher_a/A教师：使用您已设置的个人密码，数据库迁移没有重置它。teacher_b/B教师、student_a1/A学生、student_b1/B学生：初始123456，先进入 /profile 改密，完成后重新登录进入 /materials。新密码至少8字节且含字母数字。不得从 .env 的旧 SEED_* 值推断当前密码。

实际教师上传/材料列表在独立 Compose 入口18080验证，主入口的三个初始登录及首次拒绝实际通过。验收账号改密不影响主库密码。

## 源码/证据与自行截图

[docker-compose.yml](../../docker-compose.yml)、[初始化](../../cmd/api/main.go)、[页面](../../web/app.js)、[容器状态](container-services.json)、[真实请求](container-http-first.json)。

1. CMD 执行 compose ps，截图三服务 healthy 与仅web8080端口。
2. 在主入口用教师账号登录；初始账号先改密再重新登录。
3. 截 /materials，包含教师身份、班级、材料列表和上传表单。

## 本轮补充

从新路径启动时，按本轮复验文档重新构建并重建容器，以更新移动前的 web bind mount。命名卷和已有密码保持不变。全新空库使用 init-env 生成的随机 SEED_* 口令；只有明确重置过的演示账号才是123456，已改密账号用个人密码。本轮没有重置账号。

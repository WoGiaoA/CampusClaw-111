# 作业1：系统运行

Go + MySQL 8.4 + 服务端 Session + Nginx，同源前后端；Compose 命名卷保存数据库与上传文件。符合第三课技术路线，未沿用 Flask/SQLite/签名身份 Cookie。

Windows CMD（由用户启动 Docker）：

    cd /d D:\PKU\Projects\campusclaw-111
    scripts\init-env.cmd
    docker compose up --build -d --wait
    docker compose ps
    curl.exe -f http://127.0.0.1:8080/health

已有 .env 不覆盖。日常无后端变更可省略 --build；停止 docker compose stop，不能删除数据卷。入口 http://127.0.0.1:8080/login，教师登录后目标 /materials；首次账号先 /profile 改密并重新登录。

teacher_a/teacher_b 为 A/B 教师，student_a1/student_b1 为 A/B 学生。已有密码以本人最后设置的值为准；空库种子密码仅从自己的 .env 安全查看，不写入作业。演示新注册学生初始口令及强制改密见[账号说明](../../docs/account-management.md)。本次不查询、重置或公开个人密码。

源码：[应用入口](../../cmd/api/main.go)、[数据库与种子](../../internal/campus/db.go)、[页面](../../web/app.js)、[Compose](../../docker-compose.yml)。

实际验证：第三课 三个当前容器 healthy、公开 /login 200、/health 200；独立测试库四账号登录及教师读取本班材料通过。分别见 [final-runtime.json](final-runtime.json)、[final-http.json](final-http.json) 中 login/identity/list 检查。未使用真实用户凭据重新登录主系统，不把隔离测试冒称主账号登录。

自行截图：打开 /login，以当前 teacher_a 密码登录，首次提示则先改密；到 /materials，截图包含教师身份、A 班标签和材料列表。不要把密码或 Cookie 放进截图。容器重建持久化使用 第三课初次验收阶段 历史独立项目证据，本轮没有启停主服务。

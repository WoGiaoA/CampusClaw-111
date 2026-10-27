# 作业1：系统运行

> 后续账号管理变更已显式重置本机四账号为初始密码 123456，并强制首次改密。以下种子口令描述记录第三课首次实现状态，当前使用步骤请看 [账号说明](../../docs/account-management.md)。

## 答案与状态
真实 Go 后端和 MySQL 8.4.9 已启动，本机同源入口 http://127.0.0.1:8080/login。teacher_a 登录成功为200，材料列表只含A班，目标页面 /materials。teacher_b、student_a1、student_b1 也可登录，学生页面只读。
当前入口是 Go 开发代理；标准 Nginx/Docker 三服务配置已编写，但尚未运行，不能将本地运行写成 Compose 验收通过。

## 启动与账号
项目根：pwsh -File scripts/start-local.ps1。Go/MySQL便携工具已放忽略目录；独立MySQL数据在TEMP/campusclaw-local-mysql-data，上传在.local/uploads。
teacher_a 口令安全获取：用本地编辑器打开项目 .env，仅查看 SEED_TEACHER_A_PASSWORD；不要共享文件或把口令贴到报告。首次种子使用此值生成bcrypt哈希，重复启动不重置。
Docker安装后：pwsh -File scripts/init-env.ps1；docker compose up --build -d；访问 http://localhost:8080/login。该步骤本次待验。

## 源码与证据
- [入口与初始化](../../cmd/api/main.go)：配置、Migrate、Seed、Reconcile、HTTP服务。
- [种子](../../internal/campus/db.go)：双班四账号、可区分材料。
- [页面](../../web/app.js)：materialsPage读取/api/me，refresh读取真实列表。
- [开发代理](../../cmd/devweb/main.go)：同源转发，不是Nginx。
- http-evidence.json：/login200、login:teacher_a200、/api/materials200；集成测试 TestMySQLSchemaSeeds/IsolationDownload。

## 自行截图
1. 打开入口/login，输入teacher_a和本地口令。
2. 登录后在/materials确认右上角教师账号、A班、材料列表及上传表单。
3. 截取教师页面，不包含口令、Cookie或.env。
4. Docker安装后如要求容器截图，另补docker compose ps，不能以当前本地进程代替。

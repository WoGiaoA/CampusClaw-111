# 作业6：API 暴露与浏览器访问

| 服务 | 容器内端口 | 宿主机发布 |
| --- | --- | --- |
| web / Nginx | 80 | 127.0.0.1:8080 |
| api / Go | 8080 | 无 |
| db / MySQL | 3306（镜像另声明 33060） | 无 |

Compose 的 ports 只有 web。docker compose ps 中单独的 8080/tcp 或 3306/tcp 是容器端口声明，不等于宿主机映射；127.0.0.1:8080->80/tcp 才是公开入口。WEB_PORT 可在本机配置，默认 8080；不需要开放数据库或额外后端端口。

浏览器向 http://127.0.0.1:8080/api/... 发请求。Nginx location /api/ 将原路径转发到 http://api:8080，api 是 Compose 内部服务名；不由浏览器直接解析。/health 单独代理到 Go /health。Nginx 提供静态页面并转发，业务认证在 Go。

所以 API 在网络层通过代理可达，受保护操作在业务层仍必须认证。直接在地址栏或 curl 调本站 API 不会跳过 Session、角色和班级检查；未登录 /api/materials 为 401。/login、/register、/profile、/materials 返回公开 SPA 页面壳，业务数据要再请求 API，不是页面壳 200 就能读取材料。

Windows CMD：

    docker compose ps
    docker compose exec -T web nginx -t
    curl.exe -i http://127.0.0.1:8080/health
    curl.exe -i http://127.0.0.1:8080/api/materials
    curl.exe -i http://127.0.0.1:8080/uploads/1/guessed.md

实际验证：[final-runtime.json](final-runtime.json) 的三服务均 running/healthy，Nginx 配置检查退出 0，12 项公开 HTTP 检查全部符合预期：health 200 {"status":"ok"}，匿名受保护接口 401，存储路径 404，非法登录 JSON 400，页面壳 200。health 检查数据库及文件目录读写，会创建并立即移除探针，不返回用户或教学数据。

源码：[docker-compose.yml](../../docker-compose.yml)、[deploy/nginx.conf](../../deploy/nginx.conf)、[server.go Handler/health](../../internal/campus/server.go)。教程演示站 /week03 是部署前缀，本地直接 /api；未额外暴露数据库。

自行截图：CMD 的 docker compose ps 和 /health 响应；浏览器 F12 Network 查看已登录 /api/materials 的 Request URL 与 200，再无痕访问同 URL 得 401。保留地址、状态与响应，Cookie/CSRF 打码。容器 down/up 持久化证据为 第三课初次验收阶段 历史独立测试，本轮未启停主服务。

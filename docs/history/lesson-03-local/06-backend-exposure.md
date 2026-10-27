# 作业6：API可达性与暴露

## 标准Compose配置结论（运行待验）
web=Nginx唯一ports：127.0.0.1:${WEB_PORT:-8080}:80。api监听容器内部8080但无ports，db内部3306无ports。数据库和api不向宿主机发布，uploads只挂api。
浏览器访问http://localhost:8080/api/...；Nginx location /api/ 的proxy_pass http://api:8080 无尾斜杠，因此保留/api路径，转发Cookie与请求体。/health显式代理至api:8080/health。Nginx不鉴权、不查数据库；Go每次鉴权。
API网络可达不代表业务可访问：公开入口能发请求，但材料无Cookie仍401，学生上传403，跨班404；/health免认证200且无材料内容。

## 已验证与待验证
已验证：Compose YAML解析、仅web有ports、uploads不挂web；Nginx -t通过（Windows测试时将api DNS替换为loopback，静态根替换为测试根，未启动Nginx）；本机入口真实/health200、登录200、无会话下载401。
当前实际运行是Go开发代理，仅loopback：8080入口、18081开发API、13306独立开发MySQL。它们是本地开发监听，不是Docker发布结果，也不证明Go容器可被直连。没有对外绑定数据库。
待Docker安装：三服务启动/healthy、Nginx真实转发、仅web发布端口、api/db无法宿主机直连、down/up卷数据保留。本次不声称这些通过。

## 源码
[docker-compose.yml](../../docker-compose.yml)、[nginx.conf](../../deploy/nginx.conf)、[Dockerfile](../../Dockerfile)、[cmd/api](../../cmd/api/main.go)（监听/探针）、[health](../../internal/campus/server.go)、[开发代理](../../cmd/devweb/main.go)。

## 安装后验证/自行截图
若本地开发还占8080，先pwsh -File scripts/stop-local.ps1（保留数据）。再docker compose up --build -d；docker compose ps。
检查docker compose port web 80应localhost:8080；检查api/db无宿主机映射（docker inspect端口仅内部，勿公开完整环境变量）。不要为证明API可达额外添加ports。
浏览器F12 Network看登录和材料请求主机均localhost:8080，路径/api而不是api进程端口。无痕访问/api/materials应401，/health应200。
截compose ps、同源请求和health。随后用独立验收卷上传，down/up不删卷核对文件与记录，完成持久化证据。严禁down -v。

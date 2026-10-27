# 作业6：后端可达性与暴露

> **第三课最终整理阶段状态：开发修正与隔离测试已通过，等待用户启动 Docker 并最终验收。** 下方原有容器运行结论与 JSON/TXT 文件属于 **第三课初次验收阶段的历史证据**，不是本轮重新部署结果。最新测试边界及操作顺序见 [本轮复验](lesson-03-regression.md)。截图全部由用户完成。

## 实际标准配置

只发布web：127.0.0.1:8080→Nginx80。api容器8080、MySQL3306/33060没有宿主机端口映射；EXPOSE不是发布。仅api挂uploads，数据库使用独立命名卷。

浏览器统一请求http://127.0.0.1:8080/api/...。Nginx /api/ proxy_pass http://api:8080不带尾斜杠，保留/api路径，Host保留入口端口、转发Cookie和请求体；/health显式代理。Nginx负责网络转发，Go负责身份和班级权限。

网络可达不等于业务授权：材料匿名401，学生上传403，跨班404；/health免认证200 {"status":"ok"}。首次密码状态也拒绝业务403。

## 已完成实测

- 三服务healthy；Nginx -t成功、API探针exit0。
- inspect api/db端口映射null；原开发18081、MySQL13306及宿主3306拒绝连接，仅主web8080发布。
- 主入口JSON登录及health实测；独立web18080用于验收，结束后测试项目停止，卷保留。
- 独立DB停机或uploads不可写→/health503且容器unhealthy，恢复healthy。
- 不删卷down/up后新容器ID，用户哈希/材料/正文/文件摘要不变，28项HTTP复验通过。

[Compose](../../docker-compose.yml)、[Nginx](../../deploy/nginx.conf)、[探针](../../cmd/api/main.go)、[真实端口](container-services.json)、[故障](container-health-faults.json)、[持久化](container-persistence.json)。

## Windows CMD 检查与截图

    docker compose ps
    docker compose port web 80
    docker compose port api 8080
    docker compose port db 3306
    curl.exe -i http://127.0.0.1:8080/health
    curl.exe -i http://127.0.0.1:8080/api/materials

api/db的port查询应提示没有发布端口（非服务故障），匿名materials应401。截compose ps、health与Network同源/api请求；不要截图完整docker inspect环境变量。数据库和API不额外开放端口。

## 本轮补充

docker compose config --quiet通过，仍仅发布web的loopback8080，api/db没有ports，uploads只挂api。便携Nginx -t通过；临时校验副本仅将api:8080替换为127.0.0.1:18081，以消除容器DNS依赖。此为语法检查，不是实际反向代理验证。health的200/503已在独立Go/MySQL测试通过，容器状态、公开入口和卷重建待用户验收。

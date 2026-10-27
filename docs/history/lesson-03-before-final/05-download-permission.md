# 作业5：下载权限

> **第三课最终整理阶段状态：开发修正与隔离测试已通过，等待用户启动 Docker 并最终验收。** 下方原有容器运行结论与 JSON/TXT 文件属于 **第三课初次验收阶段的历史证据**，不是本轮重新部署结果。最新测试边界及操作顺序见 [本轮复验](lesson-03-regression.md)。截图全部由用户完成。

GET /api/materials/{id}/file。必须有效Session且完成首次改密；本班teacher和student均可下载。初始账号403，未登录401；跨班和无效ID同404 {"error":"资源不存在"}。合法下载200，Content-Disposition: attachment，原字节一致。

服务器先从Session查可信用户，检查首次状态；查材料关联知识库、比较class_id后才打开文件。此读接口允许teacher/student，无教师专属限制。用户无法通过query role/class_id改变身份。

上传卷只挂api，Nginx无文件路径映射，/uploads/返回404；宿主机不能绕过公开入口访问后端端口。请求猜测存储路径也失败。

## 实际验证

独立Compose中teacher/student本班200、匿名401、两种角色跨班404、无效ID404、公开存储404；上传后学生下载原字节，重建后文件SHA256一致。主库原文件6个完整迁入。

[鉴权](../../internal/campus/server.go)、[材料读取/下载](../../internal/campus/materials.go)、[Nginx](../../deploy/nginx.conf)、[HTTP](container-http-first.json)、[持久化](container-persistence.json)。

## 手动验证与截图

1. 完成首次改密，登录学生，下载本班材料；Network截200/Content-Disposition。
2. 无痕窗口打开同一下载URL，截401 JSON。
3. 登录A班，将URL的id换成B班材料id，截404；对比明显不存在id也404。
4. 访问/uploads/1/guessed.txt截404。不要把初始改密403误当普通班级拒绝。

## 本轮补充

TestMySQLIsolationDownload重新验证双班教师/学生的列表、详情、知识库与下载，匿名401、本班200、跨班/无效ID404。源码：server.go/auth，materials.go/ownedMaterial、download、storagePath。Nginx公开存储路径拒绝及容器网络由用户启动后最终验证，本轮Go404不能替代Nginx验证。

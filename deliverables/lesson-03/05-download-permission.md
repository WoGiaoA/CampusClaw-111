# 作业5：下载权限

受保护接口 GET /api/materials/{id}/file。必须登录，且已完成首次改密；教师和学生均可下载本班文件。材料是班级共享资源，同班学生下载其他用户上传的文件是正常授权。

| 场景 | 实测响应 |
| --- | --- |
| 未登录或无效/过期会话 | 401，请先登录 |
| 本班教师/学生下载 | 200，attachment，原字节一致 |
| A 班读 B 班，B 班读 A 班 | 404，资源不存在 |
| 已登录读取缺失、非法或非正数 ID | 404，资源不存在 |
| 首次未改密账号 | 403，password_change_required |
| 公开 /uploads/猜测路径 | 404 |

跨班与不存在返回同一错误体，不泄露标题、正文、存储键。auth 查 sessions/users，ownedMaterial 比较可信 class_id；download 在通过检查后才打开文件，设置 Content-Disposition: attachment 与 application/octet-stream。读取权限对师生开放，不要求必须是上传者；上传权限另验 teacher。

源码：[server.go auth 与路由](../../internal/campus/server.go)、[materials.go ownedMaterial/resourceError/download/storagePath](../../internal/campus/materials.go)、[Nginx /uploads/ 拒绝规则](../../deploy/nginx.conf)、[卷仅挂载 api](../../docker-compose.yml)。Nginx 静态根目录只有 web，不挂载 uploads。直接调公开 API 仍经过 Go 鉴权，猜存储路径不能绕过。

实际验证：[final-http.json](final-http.json) 覆盖四账号本班下载、双向跨班/缺失 404、匿名 401、学生上传 403、BOM 原字节；TestMySQLIsolationDownload 另测非法 ID。首次限制在 TestDemoResetFirstLogin；当前 Nginx 的匿名下载 401 与 /uploads/ 404 见 [final-runtime.json](final-runtime.json)。本轮全程不改真实业务数据。

自行截图：正常登录学生，点击下载，Network 记录 200 与 attachment。复制该下载 URL 到无痕窗口，不登录，应看到 401 JSON。使用 A 班会话将 ID 换成实际 B 班材料 ID，再用一个确定不存在的 ID，均为 404。B 班 ID 请先在 B 班自己的合法会话中记录，不能只凭猜中一个不存在 ID 就声称跨班验证。不同角色用独立浏览器会话；不要共享 Cookie。截图遮住请求凭据。

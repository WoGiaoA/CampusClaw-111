# 作业5：下载权限与实现

下载必须登录。学生可下载本班文件；A班教师/学生不能下载B班。接口GET /api/materials/{id}/file，没有公开uploads下载URL。

| 实测场景 | 状态/内容 |
| --- | --- |
| 无Cookie下载已有文件 | 401，JSON请先登录，无文件 |
| student_a1下载A班文件 | 200，原文件字节，Content-Disposition attachment |
| teacher_a下载B班文件 | 404，资源不存在，无他班内容 |
| 已登录访问无效ID | 404，与跨班相同JSON |
| 尝试 /uploads/ 静态路径 | 404，不能绕过鉴权 |
| B班读A班（隔离MySQL测试） | 404 |

以上为真实HTTP和真实MySQL测试结果，完整响应头/正文在http-evidence.json的download:*。Nginx容器入口待验，但配置没有挂载uploads，且明确 /uploads/ 路径404。

## 源码
[server.go](../../internal/campus/server.go) auth：先Session查询再users角色班级。
[materials.go](../../internal/campus/materials.go) ownedMaterial：先取记录再比class_id，未通过不会打开文件；download随后校验存储键、os.Open、ServeContent，返回attachment；学生和教师均可读。
[nginx.conf](../../deploy/nginx.conf) 不把uploads作为静态目录；[Compose](../../docker-compose.yml) uploads仅api挂载。
本次没有编辑或删除接口，掌握文件ID或存储名不等于获得授权。

## 手动验证/截图
1. student_a1登录，在列表下载一份A班文件，F12 Network记录真实ID和200。
2. 无痕窗口不登录，直接打开同一下载URL，应401；截图错误JSON。
3. 用另一个浏览器配置/隐私窗口登录student_b1，从B班列表记下ID；原A班窗口访问此B班ID下载URL，应404。
4. A班窗口把ID改为99999999，应同404。比较状态和正文，不仅看按钮。
5. 截图打码Cookie。无需截图保存路径或任何真实凭据。


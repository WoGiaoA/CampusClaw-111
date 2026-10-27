# 作业4：上传与入库

接口 POST /api/materials/upload，multipart/form-data，字段 file 与可选 title，已登录请求头带 X-CSRF-Token。浏览器 FormData 自动生成 boundary，不手填 Content-Type。

处理链：auth 验会话及首次改密状态 → upload 验 teacher → CSRF → 解析有界 multipart → 读取并验证文件 → 生成服务端存储键 → 临时文件写入/同步/关闭/rename → 同一 MySQL 事务写 materials 与 knowledge_entries → 提交成功返回 201（material_id、knowledge_entry_id）。上传者和班级取自 Session 关联的用户，忽略表单伪造值。

当前只支持 .txt/.md（扩展名大小写不敏感），单文件最大 5 MiB，整请求最大 6 MiB，严格 UTF-8，移除正文开头 BOM，拒绝空白正文；文件名/标题最长 255 个 Unicode 字符。标题留空使用原文件名。PDF、DOCX、图片和其他扩展名返回 400，不支持解析。按扩展名和文本校验，不是通用 MIME 魔数识别器，也不识别伪装成 txt 的特殊格式。

原字节保存至私有 uploads 卷下按班级和随机名组织的位置；原始文件名仅元信息，不能控制路径。正文完整保存，不执行 Markdown/HTML，也不切分、向量化或调用 AI。相同原名两次上传生成不同资源。

失败一致性：格式/编码等 400、文件超限 413，落盘前拒绝；文件写入或事务失败 500，事务 rollback，defer 删除本次临时/最终文件。提交结果不确定时查完整关联，确认已提交就保留，否则补偿；依赖不可判断或清理失败标记不就绪。启动 Reconcile 清临时/孤立文件并校验已提交记录。DB 与文件不是同一个原子事务，依赖补偿和单实例启动对账。

源码：[materials.go validateText/saveFile/upload](../../internal/campus/materials.go)、[启动对账](../../internal/campus/db.go)、[Nginx 请求上限](../../deploy/nginx.conf)。

实际结果：TestFinalCourseworkEvidence 的 txt/md 上传 201、去 BOM 正文/原字节下载、学生和缺 token 403、PDF/DOCX/PNG/空白/非法 UTF-8 400、超 5 MiB 413，拒绝后 DB/文件集合不变。TestMySQLUploadRollbackRecovery 另验证同名、不完整输入、file-write/knowledge-insert/commit 失败及恢复。故障在隔离库/目录注入；恢复为构造中断残留状态后启动对账，未对真实服务断电/杀进程。[输出](final-go-tests.txt)、[HTTP 证据](final-http.json)。

自行截图：教师上传一个 UTF-8 txt 或 md，Network 记录 multipart 字段、201 响应；列表查看正文并下载核对内容。再上传一个 PDF 看 400 提示；学生登录只读且无上传入口。课堂文件可自行保留，勿为验证修改真实目录权限。打码 Cookie 与 CSRF。

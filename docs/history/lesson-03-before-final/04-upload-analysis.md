# 作业4：文件上传分析

> **第三课最终整理阶段状态：开发修正与隔离测试已通过，等待用户启动 Docker 并最终验收。** 下方原有容器运行结论与 JSON/TXT 文件属于 **第三课初次验收阶段的历史证据**，不是本轮重新部署结果。最新测试边界及操作顺序见 [本轮复验](lesson-03-regression.md)。截图全部由用户完成。

## 实际链路

POST /api/materials/upload，multipart/form-data含file与可选title。Nginx总请求6MiB；Go先查Session、首次状态、teacher角色及X-CSRF-Token，再有界读取、文件验证、保存原字节、解析正文、数据库事务插入materials与knowledge_entries，提交后201（两个ID）。

支持txt/md，扩展名大小写不敏感，UTF-8、最大5MiB，空白/非法编码拒绝，正文去UTF-8头BOM但下载保留原字节。PDF、DOCX、图片不支持，不包含向量检索、RAG或模型调用。

原文件以随机名保存在/uploads/class_id，不使用客户端class_id。材料及正文同事务；失败回滚并清理本次临时/最终文件。提交结果不确定时查关联决定是否保留，不能判断则不健康。启动Reconcile清孤立并检查已提交文件，已有正确记录不误删。

## 实际测试

容器txt/md上传201，本班学生看到正文、知识库与原字节下载；学生403、缺CSRF403，PDF/DOCX/图片/坏UTF8/空白400、超过5MiB或Nginx总请求限制413；失败请求未增加列表记录。隔离Docker MySQL测试注入文件保存、正文插入、提交失败并核对回滚/残留。

中断恢复通过构造提交前后遗留状态执行实际Reconcile验证，未强杀运行中的上传进程。

[处理源码](../../internal/campus/materials.go)、[恢复](../../internal/campus/db.go)、[Nginx](../../deploy/nginx.conf)、[91项请求](container-http-first.json)、[Go测试](container-go-tests.txt)。

## 自行截图

教师完成改密登录后上传txt，F12看multipart、201和两个ID，截更新列表。再上传PDF截400；如查看CSRF请求头，值打码。学生页面无上传表单，直接API上传仍403。

## 本轮补充

TestMySQLUploadRollbackRecovery重新通过，使用真实HTTP、MySQL事务及临时目录验证格式/编码/大小、学生拒绝、同名保存、正文/BOM、原字节下载与失败补偿。web/app.js/materialsPage现区分上传201和刷新失败：提示上传已成功，可点击刷新列表，不应重复上传。新增页面回归覆盖该行为。

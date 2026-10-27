# 作业4：文件上传逻辑

## 真实链路
同源multipart → 认证Session → role=teacher（学生403）→ X-CSRF-Token → 请求/文件大小 → 扩展名/UTF8/非空 → 随机临时文件 → close/sync/rename → MySQL事务插材料/正文 → commit → 201两个ID → 本班列表刷新。
file必填，title可选，默认文件名；role/class_id表单值丢弃，数据库归属只取服务端身份。

支持大小写不敏感.txt/.md；文件上限5MiB，请求上限6MiB。严格UTF8，解析正文移除头BOM，拒绝空文件和纯空白。原字节不变，md按纯文本显示，不解析执行HTML。
PDF、DOCX、图片扩展名返回400，不支持这些格式的解析。采用后缀白名单和文本编码校验，不是复杂格式识别器；把任意UTF8文本改名txt会按文本保存，不会识别/解析PDF或Word结构。
路径由服务端class_id和随机ID生成，原文件名只作显示，路径穿越不能决定磁盘路径，同名文件不会覆盖。
两张表materials/knowledge_entries同事务，正文完整留存。讲义/作业/助手/技能仅预留，没有上传顺带业务，也没有向量切分、检索、RAG或模型请求。

## 失败清理
输入/编码/类型失败400、超限413，在落盘前拒绝。写文件失败500，临时文件清理；DB插入/提交确认失败rollback并删除本次最终文件。已有记录文件保持原样。
提交结果不确定时再次查询关联：确认已提交则返回201并保留；DB不可查询则保留待启动对账并health未就绪，避免删除已提交文件。启动清理孤立/临时文件并检查所有已提交材料的文件与正文；清理失败不能报告健康。单实例运行。

## 源码与实测
[materials.go](../../internal/campus/materials.go)：upload、validateText、saveFile、storagePath。
[db.go](../../internal/campus/db.go)：Reconcile；[integration_test.go](../../internal/campus/integration_test.go)：TestMySQLUploadRollbackRecovery。
真实HTTP教师上传201、学生403、PDF400，上传后学生详情/知识库均200。真实隔离MySQL测试txt/md/BOM/同名/无效UTF8/空/超限、文件写入/正文插入/commit注入失败均通过，无两表或文件残留；恢复测试验证未提交孤立文件被删除、已提交文件保留。

## 自行截图
teacher_a登录，选择UTF8 txt或md→上传，截新列表；F12 Network查看upload的multipart字段和201响应。
再上传.pdf，截400响应与提示。学生只读页面没有上传入口；直接请求测试证据为403。
不要截Cookie/CSRF完整值或私有磁盘路径。

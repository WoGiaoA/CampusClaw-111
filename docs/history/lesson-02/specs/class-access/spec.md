# Spec Delta

## Purpose

为 CampusClaw 高校班级教学建立账号密码身份、可信会话与服务端角色和班级边界，使教师与学生只访问所属班级的数据，并能明确验收未登录、错误密码和越权请求的拒绝行为。

## ADDED Requirements

### Requirement: 账号密码登录

系统 MUST 支持教师与学生账号密码登录，成功后建立含用户、角色与班级身份的可信会话；失败 MUST 不建立有效会话且不得泄露密码或哈希。

#### Scenario: 四类种子账号正常登录
- **WHEN** teacher_a、teacher_b、student_a1、student_b1 分别使用正确口令提交 POST /login
- **THEN** 每次返回 302 到 /materials，并设置有效会话 cookie
- **AND** 后续受保护请求识别的角色与班级分别为 teacher/A、teacher/B、student/A、student/B

#### Scenario: 错误密码或未知账号
- **WHEN** 无有效会话的请求提交错误密码或不存在的用户名
- **THEN** 返回 401 和一致的“账号或密码错误”提示，不建立有效会话
- **AND** 再次访问材料 API 仍为 401，响应不含密码、哈希或材料信息

### Requirement: 会话保护与退出

系统 MUST 对受保护页面和 API 验证会话，页面未登录返回 302 至 /login，API 未登录返回 401；失效或篡改会话 MUST 等同未登录。登出 MUST 清除当前会话。

#### Scenario: 未登录请求
- **WHEN** 不带 cookie 请求 GET /materials 以及材料列表、详情、文件、知识库读取和上传 API
- **THEN** 页面返回 302 至 /login，所有 API 返回 401
- **AND** 无材料数据、数据库变更或新增文件

#### Scenario: 篡改与过期会话
- **WHEN** 修改会话 cookie 中身份内容或使用已过 8 小时有效期的 cookie 请求受保护资源
- **THEN** 页面返回 302 至 /login，API 返回 401，不信任其中角色或 class_id

#### Scenario: 登出当前会话
- **WHEN** 已登录用户携带有效 CSRF token 提交 POST /logout 后，用客户端当前 cookie 请求材料 API
- **THEN** 登出返回 302 至 /login，随后材料 API 返回 401

### Requirement: 教师上传与学生只读

系统 MUST 在服务端按可信角色授权；学生调用上传接口 MUST 返回 403 且不得写文件或数据库。教师只能向所属班级上传。学生页面 MUST 不显示上传入口；本次不提供编辑、覆盖、删除接口。

#### Scenario: 学生绕过页面直接上传
- **WHEN** student_a1 或 student_b1 带有效会话和 CSRF token 向 POST /api/materials/upload 提交合法文件
- **THEN** 返回 403，即使请求伪造 role=teacher 也不能上传
- **AND** 材料与知识库行数、上传目录文件集合均与请求前相同

#### Scenario: 教师进入上传流程
- **WHEN** teacher_a 携带有效会话与 CSRF token 提交合法 txt/md 文件
- **THEN** 请求通过角色检查，按 teaching-materials 的上传规约处理

#### Scenario: 学生页面只读
- **WHEN** student_a1 访问 /materials
- **THEN** 页面可查看本班材料且没有上传表单
- **AND** 不存在可用的材料编辑、覆盖或删除业务接口

### Requirement: 可信班级隔离

系统 MUST 在服务端对材料、知识库、文件读取和写入实施班级边界，以可信会话 class_id 为依据，不得使用客户端 role/class_id 决定授权；跨班资源按 ID 读取 MUST 返回 404 且不得泄露他班内容。

#### Scenario: A 班教师与学生访问 B 班资源
- **WHEN** teacher_a 或 student_a1 请求 B 班材料详情、文件或知识库条目的 ID
- **THEN** 每个请求返回 404，与不存在的 ID 一致
- **AND** 响应不含 B 班标题、正文、路径或存储键；B 班用户读取 A 班资源结果相同

#### Scenario: 伪造查询参数和上传归属
- **WHEN** A 班用户在列表查询参数中指定 class_id=B，或 teacher_a 在合法上传表单中指定 class_id=B、role=teacher
- **THEN** 列表仍只返回 A 班记录；成功上传的材料和知识库归属仍为 A 班
- **AND** 客户端参数无法改变服务端鉴权身份

### Requirement: 密码与会话安全

系统 MUST 使用带盐的单向密码哈希保存和验证密码，禁止明文密码列；会话 cookie MUST 为 HttpOnly、SameSite=Lax，HTTPS 环境启用 Secure。带会话的写请求 MUST 验证 CSRF token。

#### Scenario: 哈希存储与验证
- **WHEN** 检查种子账号的 password_hash 并尝试正确和错误密码登录
- **THEN** 每个值均为可验证的 bcrypt 哈希且不等于明文口令，无明文密码列
- **AND** 正确口令验证通过，错误口令验证失败，相同口令的独立哈希因随机盐不同

#### Scenario: cookie 属性
- **WHEN** 登录成功并检查 Set-Cookie
- **THEN** cookie 含 HttpOnly、SameSite=Lax；HTTPS 配置下包含 Secure，HTTP 本地演示可关闭 Secure

#### Scenario: 缺少写请求 token
- **WHEN** 已登录教师向上传或退出接口提交缺失或错误 CSRF token 的请求
- **THEN** 返回 403，不改变会话、材料、知识库或上传文件

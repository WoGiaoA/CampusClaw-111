# 教师首次测试指南

本指南面向从仓库下载代码后首次启动的环境。不要复制开发者的`.env`、数据库卷或`.local/`，仓库已提供必要源码、内置种子及已有验收文本。

## 1 启动与账号

安装并启动Docker Desktop的Linux containers，在项目根目录打开Windows CMD：

```cmd
scripts\init-env.cmd
notepad .env
docker compose --profile mcp up --build -d --wait
docker compose ps
curl.exe -f http://127.0.0.1:8080/health
```

初始化脚本仅在不存在`.env`时生成本机随机密码、Session密钥、MCP令牌及独立的32字节Base64加密密钥；已存在的文件保持不变。模型相关配置保留为空，需使用测试者自己的兼容接口信息填写，不提供或提交开发者的真实密钥。

空数据库会创建以下账号，登录地址为`http://127.0.0.1:8080/login`：

| 账号 | 班级/角色 | 初始密码从本机.env读取 |
| --- | --- | --- |
| teacher_a | A班教师 | SEED_TEACHER_A_PASSWORD |
| student_a1 | A班学生 | SEED_STUDENT_A1_PASSWORD |
| teacher_b | B班教师 | SEED_TEACHER_B_PASSWORD |
| student_b1 | B班学生 | SEED_STUDENT_B1_PASSWORD |

这四个密码由各自初始化环境随机生成，不是变量名本身，也不是统一的`123456`。已有数据库仍使用最后设置的密码，重启、修改SEED变量不会重置账号。

每班另有1份基础资料、2个启用应用“课程资料问答/解题引导”、2个模板，以及已绑定到“解题引导”的内置Skill。资料绑定默认空，由教师显式选择。额外数学资料、自定义Skill、MCP服务和工具授权由下面的一键演示入口准备。

## 一键准备演示内容（推荐）

启动服务后，在项目根目录执行：

```cmd
python apitest\run_all.py --init-demo
```

此模式读取本机`.env`的A班教师初始密码与MCP令牌，通过正式接口导入13份已有资料，绑定“课程资料问答”和“解题引导”，导入自定义Skill，连接本地campus-tools并创建已授权的“MCP工具验收”应用。无需注册、逐份上传或手动绑定。之后用`student_a1`登录体验；B班保留基础内容用于隔离对照。

同内容资源会复用，重复执行不创建重复资料、Skill、应用或服务；保留原有提示词、模板、应用启停及其他绑定，遇到同名资料/Skill内容冲突则停止，不覆盖。执行中断可能已导入一部分，排除问题后可重新执行。已有教师密码改变时，在本机CMD临时设置`CAMPUSCLAW_DEMO_TEACHER_PASSWORD`为当前密码，运行后用`set CAMPUSCLAW_DEMO_TEACHER_PASSWORD=`清除，不要提交该值。

初始化不发送Chat请求；上传会触发后台索引，若已经配置Embedding会产生对应调用。未配置时资料仍可导入，之后补齐配置并重建失败索引即可检索。该命令显式修改本地运行站点的A班演示数据，不是隔离测试。**不带参数**的`python apitest\run_all.py`仍只运行隔离测试并生成本地HTML，不初始化主站。

下面第3–5节保留手动配置和功能核对步骤；已经运行一键初始化时，跳过上传、新建、绑定和连接步骤，直接查看配置并测试。

## 2 模型配置与可测试范围

在本机`.env`填写`CHAT_BASE_URL`、`CHAT_MODEL`、`CHAT_API_KEY`和`EMBEDDING_BASE_URL`、`EMBEDDING_MODEL`、`EMBEDDING_API_KEY`、`EMBEDDING_DIMENSION`，维度和模型名称以所用服务为准。Base URL使用兼容接口的基础地址，不把`/chat/completions`追加到配置值中。配置后重新执行带`--build`的启动命令。

没有模型配置仍可测试登录、班级隔离、应用/模板/Skill管理、资料上传和MCP连接/发现；不能据此宣称真实AI回答已通过。知识库关键词检索也需要已有ready索引。填写Embedding配置后等待索引完成；此前失败的资料可在教师详情中重建，再刷新至ready。资料建索引和真实问答会使用测试者配置的接口。

若只验证接口而不使用任何课程Token，执行`python apitest\run_all.py`。该入口使用独立MySQL与HTTP模型替身，依赖见[apitest说明](../apitest/README.md)，生成HTML仅本地查看。

## 3 知识库与模板

1. 用`teacher_a`登录，在“知识库管理”上传`examples/knowledge/`中的现有资料。可先上传“星桥班课程考核与提交说明.md”和“03_一元二次方程与判别式.md”；其余文件用于扩展检索与绑定范围测试。
2. 等待需要检索的材料索引为ready。在“AI应用管理 → 课程资料问答 → 知识库绑定”勾选刚上传的资料，保存。
3. 在“提示词模板”查看或修改所选模板，再保存应用提示词。模板管理和应用绑定分别保存。
4. 用`student_a1`进入“讲义与检索”，以“判别式”查询并查看片段/原文；进入“AI助手”选择“课程资料问答”，提问“根据资料，星桥班期末项目要提交哪些内容？”，核对来源编号并展开片段。
5. 继续追问“请压缩为三项，并保留引用编号”。再用`student_b1`检查看不到A班资料。资料中的演示截止时间、分组规则仅为测试数据。

## 4 内置与上传Skill

1. 在“解题引导 → Skill技能”检查内置“解题引导”已启用且已绑定；如果需要资料，另在该应用绑定相应数学材料。
2. 展开“上传新Skill”，选择`examples/skills/人工测试解题引导.md`并上传，然后在当前应用的绑定列表勾选它并保存。
3. 学生选择“解题引导”，提问“青竹验收：方程x²-5x+6=0应该怎样开始分析？”。界面应显示命中“人工测试解题引导”；回答按该规程给下一步提示和检查问题。模型输出质量须结合实际返回检查。
4. 可取消绑定或停用该自定义Skill后再次检查其不再命中。启用与绑定是两个独立状态。

## 5 MCP连接与两工具调用

1. 教师在“MCP工具管理”登记服务标识`campus-tools`、显示名“课程解题工具”、Base URL `http://mcp-server:8080`；连接令牌填写本机`.env`的`MCP_SERVER_TOKEN`值。
2. 点击“连接并发现”，确认已连接，清单包含`formula_hint`和`strategy_hint`。不要把开发者电脑上的旧令牌写进仓库。
3. 在“AI应用管理”新建“MCP工具验收”，在基本配置启用并保存；角色指令可沿用内置解题引导。这个应用用于工具测试，可不绑定知识库以避免查询Embedding。
4. 在新应用的“MCP工具”中授权`formula_hint@campus-tools`与`strategy_hint@campus-tools`并保存；再到“Skill技能”上传`examples/skills/MCP验收解题引导.md`，勾选绑定并保存。务必先完成服务发现，未知工具引用会被拒绝。
5. 学生选择“MCP工具验收”，提问“MCP验收：方程x²-5x+6=0应该怎样开始分析？请先使用公式提示工具，再根据返回的策略键使用策略提示工具，只给下一步提示，不给最终答案。”
6. 核对学生工具过程及教师“审计与统计”：两工具依次成功，随后给出引导正文。不能仅根据回答中声称调用工具判定执行成功。

连接/发现只访问本地MCP；发起对话才会调用所配置的LLM。可断开服务或撤销授权检查引用不可调用；不要重建或删除当前数据库卷来重复初始化。

## 6 新旧环境的区别

- 新环境无需导入数据库备份，程序自动建表和生成基础种子；额外样例按上述界面操作导入。
- 已有环境不重置账号、应用启停、绑定、教师修改或软删除记录；不要执行`docker compose down -v`。
- 旧`.env`不会由初始化脚本补写。若缺少MCP配置，请对照`.env.example`补齐：`MCP_SERVER_TOKEN`使用随机令牌，`MCP_CREDENTIAL_KEY`使用32字节随机值的Base64编码。不要轮换已有加密密钥来“修复”数据；完整设计见[OpenSpec MCP规约](../openspec/changes/add-mcp-tool-integration/)。
- 原始资料和Skill文件在`examples/`，源码、测试和Swagger在仓库；`.env`、上传卷、索引、生成报告与个人Word/PDF不提交。

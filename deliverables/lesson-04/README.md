# 第四课作业：OpenSpec变更描述

本地文档完成，业务未实施。公开仓库发布和课程平台提交由本人自行处理；本地没有替代实际提交的虚构仓库链接。

## 作业原文与范围

依据用户提供的第四课作业截图，正文为：

> 完成第四次课的openspec的变更描述。
>
> 基本要求：实现限定班级范围的知识库检索，支持内容溯源，可以考虑使用向量数据库来存储。
>
> 提交一个开放的仓库地址，仓库中包含本次变更内容。

本次要求的是描述拟实现能力的OpenSpec变更，而非本轮同步编码。向量数据库是可考虑的方案；本项目选择Qdrant，但不把该选择冒称为老师强制要求。截图没有另要求运行截图、思考题答案、报告或压缩包。

## 变更与文件

变更名：`add-traceable-vector-retrieval`，状态：规划四件套齐备、待后续Review及Apply授权。实现任务全部未勾选，未同步主规格、未归档。

| 文件 | 内容 |
| --- | --- |
| [proposal.md](../../openspec/changes/add-traceable-vector-retrieval/proposal.md) | 原因、范围、新增/修改能力、影响和作业边界 |
| [design.md](../../openspec/changes/add-traceable-vector-retrieval/design.md) | 复用第三课架构、自动切片、两库分工、三模式、RRF、持久任务、迁移与风险 |
| [knowledge-retrieval/spec.md](../../openspec/changes/add-traceable-vector-retrieval/specs/knowledge-retrieval/spec.md) | 本班检索、双侧过滤、来源、状态、重建和失败的WHEN/THEN场景 |
| [teaching-materials/spec.md](../../openspec/changes/add-traceable-vector-retrieval/specs/teaching-materials/spec.md) | 保存原文与索引分离的增量，保留现有201/查看/下载行为 |
| [project-runtime/spec.md](../../openspec/changes/add-traceable-vector-retrieval/specs/project-runtime/spec.md) | 新存储、服务端配置、内部向量服务与基础健康的增量 |
| [tasks.md](../../openspec/changes/add-traceable-vector-retrieval/tasks.md) | 按依赖排序的后续实现任务，每项有完成验证方式 |
| [.openspec.yaml](../../openspec/changes/add-traceable-vector-retrieval/.openspec.yaml) | OpenSpec生成的变更元数据 |

补充阅读：[课程分析](../../docs/lesson-04-analysis.md)、[文档检查记录](review.md)。JSON原始证据见本目录的 `openspec-validation.json`、`openspec-status.json` 和 `document-audit.json`。

## 作业要求对应关系

| 要求 | 变更如何覆盖 |
| --- | --- |
| 第四课OpenSpec变更描述 | proposal/design/specs/tasks完整，严格校验无问题；并不声称编码完成 |
| 限定班级范围 | 新接口复用可信Session；关键词、向量和回表均按班级过滤；伪造class_id及跨班来源场景明确 |
| 内容溯源 | 命中包含材料标题/ID、切片ID/序号、Unicode区间、真实摘录与受保护来源链接 |
| 可以考虑向量数据库 | 设计选MySQL全文索引+Embedding+Qdrant+RRF，保留关键字独立路径 |
| 开放仓库地址且包含变更 | 用户自行将本变更目录及相应说明放入公开仓库并提交地址；本轮不操作远程仓库 |

自动窗口和三模式属于本项目在基本要求上的具体方案。回答生成、自定义/标题切分、流式会话、重排序和其他课业务不在本次正式变更中；它们仍可作为后续学习与开发方向。

## 本地复核（Windows CMD）

```bat
cd /d D:\PKU\Projects\campusclaw-111
openspec status --change add-traceable-vector-retrieval
openspec validate add-traceable-vector-retrieval --strict
```

OpenSpec的“4/4 artifacts complete”表示文档存在，不能解释为功能完成；tasks中的所有实现项仍为 `[ ]`。本次未运行第四课功能测试、未安装依赖、未访问真实模型或修改主数据。

用户提交时保留本变更目录的相对结构即可；当前 `openspec/specs/` 是已实施的第三课主规格，新增行为位于 `openspec/changes/`，这符合待实施变更的状态。不要为了提交描述而提前同步或归档。

# KnowledgeBase 与 public_service.go

## 一句话先懂

KnowledgeBase（知识库）是项目的 RAG 数据源：文档会被切分、向量化并存到 Milvus，Agent 收到用户问题时检索相关片段作为参考内容。

`backend/app/internal/knowledges/public_service.go` 不是 HTTP 接口实现；它是知识库模块提供给应用内部其他模块调用的事件适配层。

## 通用操作步骤

1. 创建知识库并配置嵌入模型。
2. 上传文档，等待文档切分和向量索引完成。
3. 将知识库关联到 Agent。
4. 用户向 Agent 提问时，系统检索相关片段并把结果作为上下文交给 Agent。

## 在本项目中的落点

| 位置 | 作用 |
| --- | --- |
| `backend/model/knowledges.go` | 定义知识库、文档和文档分片的数据模型。 |
| `backend/app/internal/knowledges/public_service.go` | 接收内部事件，查询知识库或执行检索。 |
| `backend/app/internal/router/event.go` | 注册 `getKnowledgeBase`、`searchKnowledgeBase` 两个内部事件。 |
| `backend/app/internal/agents/service.go` | Agent 侧触发检索并组装 RAG 上下文。 |
| `backend/app/internal/knowledges/service.go` | 用嵌入模型和 Milvus 实际执行向量检索。 |

## 调用链

1. Agent 的 `buildRagContext` 遍历其关联的知识库。
2. 它触发 `searchKnowledgeBase` 内部事件，并传入用户 ID、知识库 ID 和当前问题。
3. `router/event.go` 将事件交给 `PublicService.SearchKnowledgeBase`。
4. `PublicService` 新建知识库 service，调用真实的 `searchKnowledgeBase`：解析问题、取得嵌入模型、查询 Milvus。
5. 它把检索结果转换为只含 `Content` 的共享响应，返回给 Agent。

`GetKnowledgeBase` 是较轻的路径：直接按用户 ID 和知识库 ID 从 PostgreSQL 读取知识库元数据。

## 核心概念

### 知识库

知识库保存文档、模型配置、存储类型等元数据；文档正文会被拆成多个分片，而不是整篇塞进对话提示词。

### RAG

RAG 是“先检索、再回答”。它让 Agent 在回答当前问题前，从关联知识库里取出相关内容，降低只依赖模型自身记忆的风险。

### 内部事件

这里的 `event.Register` / `event.Trigger` 是进程内调用，不是 HTTP 请求。它让 Agent 模块不必直接依赖知识库模块的具体 service 实现。

## 本项目实践路线

先确保 PostgreSQL、Milvus 与嵌入模型配置可用；创建知识库、上传一份文档并将其关联到 Agent。然后询问文档中存在的问题，观察 Agent 返回的知识库参考内容。

## 注意点

- `PublicService` 对 `event.Event.Data` 做直接类型断言；错误的事件参数类型会导致 panic。
- 搜索响应只回传内容文本，不包含文档 ID、分数或来源位置，因此上层无法直接展示可追溯引用。
- Agent 当前组装 RAG 上下文时最多使用一条检索结果；多结果不会全部进入上下文。
- 需要区分 HTTP 的知识库接口（`router/knowledges.go`）和本文件的内部事件接口。

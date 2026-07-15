# core 说明

## 模块定位
`core` 是后端 AI 能力核心库，封装工作流执行、智能体、工具、MCP、知识库、节点和模型调用能力。

## 主要职责
- 初始化和执行工作流。
- 管理系统工具、MCP 客户端工具和工具注册表。
- 封装知识库向量检索、文档解析和存储能力。
- 提供智能体构建、深度智能体、节点执行和消息结构。
- 为 `app`、`mcp-server` 和 `a2a-server` 提供共享 AI 能力。

## 目录结构

### main.go / 入口文件
- 本模块没有 `main.go` 文件。

### ai/
- `workflow_execute.go`、`workflow_tool.go`：工作流执行和工具节点协作。
- `message.go`：AI 消息结构。
- `template.go`：提示词或模板相关能力。
- `agentbuilder/`：智能体构建入口。
- `deepagent/`：深度智能体工厂、子智能体和执行逻辑。
- `interview/`：访谈类智能体逻辑。
- `kbs/`：向量库、HTML 解析、ES/Milvus 检索和文件类型处理。
- `mcps/`：MCP 客户端封装。
- `nodes/`：文本组合、文本展示、HTML 展示、QwenVL 等节点实现。
- `store/`：执行状态或运行时存储。
- `tools/`：天气、Git、Kubernetes、文件写入、HTML 转 PPT 等系统工具。

### go.mod / go.sum
- `go.mod`：模块名 `core`，声明 Go 1.25.0，依赖 Eino、Eino 扩展、mcp-go、Elasticsearch、Milvus 和 Thunder。
- `go.sum`：依赖校验文件。

## 上下游关系
- 上游：`app` 调用工作流、智能体、知识库和工具能力；`mcp-server` 和 `a2a-server` 复用工具实现。
- 下游：依赖 `model` 的配置结构、`app/shared` 的共享封装、外部 LLM、MCP 服务、Elasticsearch、Milvus、Kubernetes 和文件系统。
- 共享依赖：CloudWeGo Eino、Eino 扩展、mcp-go、Thunder、Go 标准库。

## 何时先看这个模块
- 修改工作流节点执行、工具注册、MCP 调用或知识库检索时。
- 新增系统工具、模型适配、节点类型或智能体执行模式时。
- 排查 `app` 中智能体聊天、工作流执行和知识库搜索的底层行为时。

## 不要碰
- `go.sum` 由 Go 命令维护。
- 工具中涉及 Kubernetes、Git、文件写入和 HTML 转 PPT 的逻辑有外部副作用，改动前要确认调用边界。
- 不要把本地模型、向量库或检索服务的临时数据写进源码目录。

# AI_INDEX

## 作用
本文件是 MyAgent 后端 Go 工作区的导航入口。修改服务启动、HTTP 接口、MCP 接入、A2A 接入、通用工具或数据库模型前，先按模块清单进入对应的 `AI_CONTEXT.md`，再定位具体源码。

## 技术栈

### 开发语言
- Go workspace 声明为 Go 1.25.0；`app` 和 `core` 声明为 Go 1.25.0，`common` 声明为 Go 1.24.6，`a2a-server`、`mcp-server` 和 `model` 声明为 Go 1.24。

### 框架与运行时
- Gin 提供 HTTP 路由；`github.com/mszlu521/thunder` 负责配置、服务器、日志、数据库、JWT 和事件能力。
- CloudWeGo Eino 及其扩展用于模型、智能体、文档处理、检索和工具调用。
- `mark3labs/mcp-go` 提供 MCP 服务端与 SSE 传输。
- CloudWeGo Hertz 与 Eino A2A 扩展提供 A2A JSON-RPC 服务入口。

### 数据与中间件
- PostgreSQL 由 GORM 和 Thunder 数据库组件初始化；Redis 用作缓存或运行时依赖。
- Elasticsearch 8 用于知识库文档索引与检索。

### 基础设施
- 每个可执行服务从各自的 `etc/config.yml` 读取监听地址、日志和依赖配置。

## 模块清单

### app
- 主业务 API 服务，提供认证、智能体、模型配置、工具、订阅和知识库 HTTP 接口。
- 下一步看：`app/AI_CONTEXT.md`

### a2a-server
- 独立 A2A 服务，使用 Hertz 暴露 `/a2a`，把 Eino ChatModelAgent 注册为天气查询智能体。
- 下一步看：`a2a-server/AI_CONTEXT.md`

### benchmark
- 压测模块，包含智能体聊天接口的并发请求脚本。
- 下一步看：`benchmark/AI_CONTEXT.md`

### common
- 提供业务错误码、Markdown 文本切分和 token 计数等可复用辅助能力。
- 下一步看：`common/AI_CONTEXT.md`

### core
- 提供跨服务共用的 AI 消息格式、MCP 客户端封装和系统工具注册实现。
- 下一步看：`core/AI_CONTEXT.md`

### mcp-server
- 独立 MCP 服务，通过 SSE 暴露天气工具，并把 Eino 工具定义转换为 MCP 工具。
- 下一步看：`mcp-server/AI_CONTEXT.md`

### model
- 定义用户、智能体、模型、工具、订阅和知识库的 GORM 持久化模型与 JSON 字段类型。
- 下一步看：`model/AI_CONTEXT.md`

## 典型调用链
1. 客户端调用 `app` 的 `/api/v1/agents/chat`。
2. `app/internal/router` 将请求交给 `app/internal/agents` 的处理器与服务层。
3. 智能体服务读取 `model` 中的智能体、LLM、工具和知识库记录，并使用 `core/ai` 的系统工具或 MCP 客户端能力。
4. 知识库检索经 `app/internal/knowledges` 调用 Elasticsearch，模型调用经 Eino 返回流式智能体结果。
5. 外部 MCP 客户端可独立连接 `mcp-server` 的 `/sse` 与 `/message`，调用其基于 `core/ai/tools` 构建的天气工具。
6. 外部 A2A 客户端可连接 `a2a-server` 的 `/a2a`，调用基于 `core/ai/tools` 天气工具构建的 Eino 智能体。

## 不要碰
- `go.sum` 应由 Go 模块命令维护，不应手工修改。
- `app/etc/config.yml` 中含本地运行参数与凭据；提交或共享前应审查敏感配置。
- Go 构建缓存、编辑器目录和依赖缓存不属于源代码或文档维护范围。

# app 说明

## 模块定位
`app` 是面向客户端的主业务 API 服务，负责启动 HTTP 服务器并编排认证、智能体、模型、工具、订阅和知识库能力。

## 主要职责
- 从 `etc/config.yml` 加载服务、日志、认证、PostgreSQL、Redis、邮件和 JWT 配置。
- 初始化 PostgreSQL、Redis、JWT 与系统天气工具，并注册 Gin 路由。
- 为认证、智能体聊天、LLM 配置、工具管理、订阅和知识库提供 HTTP 处理器、服务层与数据库访问。
- 使用 Eino、Elasticsearch 和内部共享服务完成智能体编排与知识库检索。

## 目录结构

### main.go / 入口文件
- 依次执行 `config.Init`、读取配置、`logs.Init`、`server.NewServer`、`inits.Init` 和 `s.Start`。

### etc/
- `config.yml`：定义 8888 端口、认证规则、日志、PostgreSQL、Redis、邮件和 JWT 参数。

### internal/inits/
- `inits.go`：初始化 PostgreSQL、Redis 和 JWT，注册系统天气工具及全部业务路由。

### internal/router/
- `agents.go`、`auths.go`、`knowledges.go`、`llms.go`、`subscriptions.go`、`tools.go`：注册 `/api/v1` 下的业务路由。
- `event.go`：注册应用事件处理逻辑。

### internal/agents/
- `handler.go`、`service.go`、`model.go`、`repository.go`、`req.go`、`res.go`：实现智能体管理、流式聊天、工具绑定和知识库绑定。

### internal/auths/
- `handler.go`、`service.go`、`model.go`、`repository.go`、`req.go`、`res.go`：实现注册、邮件验证、登录、令牌刷新与密码重置。

### internal/knowledges/
- `handler.go`、`service.go`、`model.go`、`repo.go`、`public_service.go`、`req.go`、`res.go`：实现知识库、文档、分块、索引和检索。

### internal/llms/
- `handler.go`、`service.go`、`model.go`、`repository.go`、`public_service.go`、`req.go`、`res.go`：管理提供商配置和 LLM 记录，并向事件调用方提供模型配置。

### internal/tools/
- `handler.go`、`service.go`、`model.go`、`repository.go`、`public_service.go`、`req.go`、`res.go`：管理内置工具与 MCP 工具，并支持连通性测试和工具发现。

### internal/subscriptions/
- `handler.go`、`resp.go`：返回当前用户的订阅信息。

### shared/
- `knowledge.go`、`llms.go`、`tools.go`：声明跨业务包使用的知识库检索、模型配置和工具服务契约。

## 上下游关系
- 上游：浏览器或其他 HTTP 客户端经 `/api/v1` 路由访问本服务。
- 下游：PostgreSQL、Redis、Elasticsearch、SMTP、Eino 支持的模型供应商和 MCP 服务。
- 共享依赖：依赖 `common` 的业务错误码和文本工具，依赖 `core` 的 AI、MCP 和系统工具能力，依赖 `model` 的 GORM 实体。

## 何时先看这个模块
- 需要新增、修改或排查 REST API、认证、智能体聊天和知识库行为时。
- 需要核对服务启动、路由注册、数据库或 Redis 初始化顺序时。
- 需要追踪客户端请求如何进入模型、工具或检索流程时。

## 不要碰
- `etc/config.yml` 包含本地数据库、SMTP 与 JWT 配置；处理时避免泄露或覆盖环境参数。
- `go.sum` 由 Go 模块工具维护。

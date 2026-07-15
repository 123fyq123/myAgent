# app 说明

## 模块定位
`app` 是后端主业务 HTTP API 服务，负责认证、智能体、工作流、模型、工具、知识库、A2A 管理和订阅等业务接口。

## 主要职责
- 启动 Thunder/Gin 服务并注册业务路由。
- 初始化 PostgreSQL、Redis、JWT、系统工具和工作流执行器。
- 按业务域组织 handler、service、repository、request 和 response。
- 通过 Thunder 事件机制向 AI 执行链提供模型配置、工具和知识库查询能力。

## 目录结构

### main.go / 入口文件
- `config.Init()` 加载 `etc/config.yml`。
- `logs.Init(conf.Log)` 初始化日志。
- `server.NewServer(conf)` 创建 Thunder 服务。
- `inits.Init(s, conf)` 初始化数据库、Redis、JWT、系统工具、工作流执行器和路由。
- `s.Start()` 启动 HTTP 服务。

### etc/
- `config.yml`：服务监听、日志、数据库、Redis、JWT 和业务依赖配置。

### internal/inits/
- `inits.go`：初始化 PostgreSQL、Redis、JWT、系统工具、工作流执行器和所有业务路由。

### internal/router/
- `auths.go`、`agents.go`、`workflows.go`、`tools.go`、`knowledges.go`、`llms.go`、`nodes.go`、`skills.go`、`subscriptions.go`、`markets.go`、`a2a.go`、`health.go`：注册各业务域 HTTP 路由。
- `event.go`：注册 `getProviderConfig`、`getEmbeddingConfig`、`getToolsByIds`、`getKnowledgeBase`、`searchKnowledgeBase` 事件。

### internal/
- `auths/`：登录注册、JWT 与用户认证。
- `agents/`：智能体管理与执行。
- `workflows/`：工作流定义、版本和执行相关业务。
- `knowledges/`：知识库管理、上传、检索和公开事件服务。
- `llms/`：模型提供商、模型配置和公开事件服务。
- `tools/`：工具管理与公开事件服务。
- `a2a/`：A2A 能力管理。
- `nodes/`、`skills/`、`subscriptions/`：节点、技能和订阅业务。

### shared/
- `llms.go`、`knowledge.go`、`tools.go`：供 `core` 或业务层复用的共享类型和封装。

## 上下游关系
- 上游：`frontend` 通过 `src/api` 调用本服务的 HTTP 接口；压测脚本也会请求智能体聊天接口。
- 下游：依赖 `model` 的 GORM 模型、`core/ai` 的智能体和工具能力、`common` 的错误码与工具函数、PostgreSQL、Redis、Elasticsearch 和外部 LLM 服务。
- 共享依赖：Thunder、Gin、CloudWeGo Eino、GORM、JWT、系统工具注册表。

## 何时先看这个模块
- 修改后端业务 API、路由、中间件或请求响应结构时。
- 排查登录、智能体、模型配置、工具、知识库、工作流和订阅接口问题时。
- 需要确认服务启动顺序、系统工具注册或事件注册逻辑时。

## 不要碰
- `go.sum` 由 Go 命令维护。
- `etc/config.yml` 可能包含本地数据库、Redis、密钥或外部服务地址，提交前必须审查。
- 不要把运行日志、上传文件、构建产物写进模块文档。

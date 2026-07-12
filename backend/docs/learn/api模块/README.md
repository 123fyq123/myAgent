# API 模块

## 一句话先懂

本项目的 API 模块就是 `app`：它像前台接待层，把浏览器发来的 HTTP 请求分发给各个业务包，再把数据库、知识库、模型和工具能力组合成统一的接口返回。最特殊的接口是智能体聊天，它用 SSE 持续把模型生成的内容推给客户端，而不是等整段回答完成后一次返回。

## 学习目标

- 理解 `app` 服务从启动到路由注册的顺序。
- 能从一个 URL 找到对应的 router、handler、service 和数据库模型。
- 能读懂 `/api/v1/agents/chat` 的流式响应链路，并在本地启动 API 服务验证接口。

## 通用背景

REST API 通常按资源组织 URL：路由层匹配 HTTP 方法和路径，处理器层负责解析请求、提取身份和输出响应，服务层负责业务规则，仓储或模型层负责持久化。本项目的 `internal` 目录利用 Go 的访问约束，保证业务包只在 `app` 模块内部使用。

SSE（Server-Sent Events）是服务端单向持续推送的 HTTP 响应。服务端以 `text/event-stream` 返回多条 `data: <内容>` 消息；本项目的聊天接口还每 5 秒发送一次以冒号开头的心跳，并在结束时发送 `data: [DONE]`。

## 通用操作步骤

### 常用命令或操作

```bash
# 在工作区根目录启动主 API 服务
go run ./app

# 运行 app 模块测试或编译检查
go test ./app/...
go build ./app

# 搜索一个路由或处理器的实现
rg -n 'agents/chat|AgentMessage' app
```

### 标准流程

1. 从 `internal/router` 找到 URL、HTTP 方法和处理器方法。
2. 到对应业务包的 `handler.go` 查看参数解析、用户身份获取和响应方式。
3. 跟进 `service.go` 的业务逻辑；需要数据时继续查看 `model.go`、`repository.go` 和 `model` 模块实体。
4. 对流式接口，用浏览器开发者工具或支持 SSE 的客户端确认响应头、心跳、数据帧和结束标记。

## 在本项目中的落点

| 位置 | 作用 | 为什么相关 |
| --- | --- | --- |
| `app/main.go` | API 服务启动入口 | 按配置、日志、服务器、初始化、启动的顺序组装服务。 |
| `app/internal/inits/inits.go` | 基础设施和路由总装配 | 初始化 PostgreSQL、Redis、JWT、系统工具，并注册全部路由。 |
| `app/internal/router/` | HTTP 路由定义 | 定义 `/api/v1/auth`、`/agents`、`/knowledge`、`/llms`、`/tools`、`/subscription` 下的端点。 |
| `app/internal/agents/handler.go` | 智能体请求入口 | 解析聊天请求、设置 SSE 响应并消费服务层返回的通道。 |
| `app/internal/agents/service.go` | 智能体核心编排 | 读取智能体配置，构建 Eino Agent、工具和 RAG 上下文，再把流式事件转为输出消息。 |
| `app/etc/config.yml` | 本地服务配置 | 定义监听地址、超时、认证规则及外部基础设施连接参数。 |

## 调用链或数据流

以 `POST /api/v1/agents/chat` 为例：

1. `app/main.go` 创建 Thunder Server，`internal/inits.Init` 注册 `AgentRouter`。
2. `internal/router/agents.go` 将该 URL 绑定到 `agents.Handler.AgentMessage`。
3. handler 用 `req.JsonParam` 读取 `AgentMessageReq`，用 `req.GetUserIdUUID` 从认证上下文取用户，再设置 SSE 头与无写入截止时间。
4. handler 调用 `service.agentMessage`，服务层在 goroutine 中查询智能体、构造 LLM、工具与知识库上下文，并启动 Eino ADK Runner。
5. 服务层把模型的推理文本、普通文本或错误写入 `dataChan`、`errChan`；handler 将它们编码为 SSE 的 `data:` 帧，持续 `Flush` 给客户端。
6. 数据通道关闭后，handler 写入 `[DONE]` 并结束请求；客户端主动断开时，`context` 会取消，服务层停止继续发送。

## 核心概念

### Router

Router 是 URL 到处理器的映射表。例如 `AgentRouter` 负责 `/api/v1/agents`，其中 `/chat` 映射到 `AgentMessage`。新增 API 时，先确定资源归属，再在相应 router 中注册。

### Handler

Handler 是 HTTP 边界层：它不应堆积业务规则，而是解析 JSON 或路径参数、取得当前用户、调用 service，并通过 `res.Success` 或 `res.Error` 返回结果。聊天接口是例外，它手工写 SSE 数据帧。

### Service

Service 处理业务编排。`agents/service.go` 会查询数据、通过事件拿到 LLM 和工具服务、构建模型代理、检索 RAG 上下文；普通增删改查服务通常会把数据库操作委托给内部仓储接口。

### Context 与超时

普通智能体管理操作会创建 5 秒超时的子 Context，避免数据库操作无限等待。聊天接口本身可能很长，因此 handler 清除了写入截止时间，同时保留可取消 Context 以在客户端离开时及时停止后台工作。

### SSE 与 channel

channel 是 Go 协程间传递流式数据的管道；SSE 是将这些数据传到浏览器的协议格式。`dataChan` 传正常内容，`errChan` 传错误，心跳避免中间网络设备因空闲连接而关闭会话。

### 事件解耦

`agents/service.go` 经 Thunder 的 `event.Trigger` 请求 LLM、工具与知识库能力，而不是直接依赖各业务包的具体服务。这降低了业务包之间的直接耦合，但事件名称和入参类型必须保持一致。

## 本项目实践路线

1. 先读 `app/main.go` 与 `app/internal/inits/inits.go`，画出服务启动顺序。
2. 任选一个简单接口，例如 `GET /api/v1/llms/all`，按照 router、handler、service、model 的顺序定位完整实现。
3. 再阅读 `agents/handler.go` 的 `AgentMessage`，重点确认 SSE 头、心跳和 `[DONE]` 的写出位置。
4. 启动 PostgreSQL、Redis、Elasticsearch 与模型服务的本地依赖后，执行下列命令启动服务；认证和业务接口需携带有效的用户身份信息。

```bash
go run ./app
```

## 注意点

- `app/etc/config.yml` 含本地基础设施连接和认证配置；阅读或提交时不要复制、暴露或覆盖其中的敏感字段。
- `auth.ignores` 放行认证相关 URL，`needLogins` 列出了需要登录的路由前缀；新增路由时应检查它是否需要认证。
- 路由对 REST 风格并不完全一致，例如列表接口既有 GET 也有 POST；应以实际 router 定义为准，不要只凭 URL 猜测方法。
- `agentMessage` 使用后台 goroutine 和无缓冲通道；给通道发送数据时必须尊重 Context 取消，避免客户端断开后协程泄漏。
- `buildRagContext` 直接向 `dataChan` 写入知识库结果，而没有通过 `sendData` 监听取消；排查断连时应特别关注这一点。

## 资料来源

- 未查询外部资料；本说明基于当前工作区的 `app` 源码、配置和模块文档整理。

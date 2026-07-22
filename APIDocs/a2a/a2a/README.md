# A2A（Agent-to-Agent）

## 一句话说明

本项目的 A2A 能力分为两部分：`app` 服务发现并保存远程 Agent 的 Agent Card，随后把这些远程 Agent 关联到本地 Agent；本地 Agent 聊天时会把已关联的远程 Agent 封装成 Eino 子 Agent 并交由 Supervisor 调度。`a2a-server` 则是一个可独立启动的 A2A JSON-RPC 示例服务，默认对外提供天气查询智能体。

## 归属模块

- 主模块：`a2a`
- 相关模块：`frontend`、`backend/app`、`backend/a2a-server`、`backend/core`、`backend/model`
- 模块判定依据：`backend/app/internal/a2a` 承载市场管理 API，`backend/a2a-server` 承载协议服务入口。

## 链路总览

### 远程 Agent 发现与市场管理

1. 用户在 `AgentMarket.vue` 中添加远程 Agent、查看详情、列出或删除条目。
2. 前端经 `a2aService.ts` 请求 `app` 的 `/api/v1/a2a/*` 接口。
3. `A2ARouter` 把请求交给 `internal/a2a.Handler`，由其绑定参数并统一包装响应。
4. `service.getAgentCard` / `saveAgentCard` 通过 Thunder 的 `a2a.GetAgentCard` 向远程端点发现 Agent Card。
5. 保存时将 URL、名称、描述和 handler 路径写入 PostgreSQL 的 `agent_market` 表；列表与删除直接读写该表。

### 远程 Agent 的实际执行

1. 先通过 `/api/v1/agents/market/add` 将市场条目关联到本地 Agent，关系写入 `agent_agents`。
2. 用户调用 `/api/v1/agents/chat` 后，`agents.service` 在构建主 Agent 的同时遍历 `agent.Agents`。
3. 对每个市场条目以 `URL + HandlerPath` 创建 A2A JSON-RPC transport 和 A2A client，再包装为 Eino Agent。
4. 主 Agent 与远程子 Agent 被传给 Eino Supervisor；Runner 以流式方式执行，`AgentMessage` 将结果通过 SSE 返回浏览器。

### 项目自带的 A2A 服务端

1. `backend/a2a-server/main.go` 读取配置，在 `127.0.0.1:8777` 启动 Hertz。
2. `jsonrpc.NewRegistrar` 把 JSON-RPC handler 注册在 `/a2a`。
3. 代码创建 Ollama ChatModel 和“高德天气查询智能体”，再由 `eino.RegisterServerHandlers` 暴露为 A2A 服务。
4. 市场页默认的 `http://localhost:8777`、`/a2a` 与该服务端配置相匹配。

## 关键文件定位

| 位置 | 作用 | 为什么相关 |
| --- | --- | --- |
| `frontend/src/views/AgentMarket.vue` | 市场页面 | 添加、查看、删除远程 Agent 的用户入口。 |
| `frontend/src/api/a2aService.ts` | 前端 API 封装 | 定义市场管理的 4 个 HTTP 调用。 |
| `backend/app/internal/router/markets.go` | 路由注册 | 注册 `/api/v1/a2a` 下的市场接口。 |
| `backend/app/internal/a2a/handler.go` | HTTP handler | 绑定 JSON/路径参数并调用 service。 |
| `backend/app/internal/a2a/service.go` | 市场业务 | 发现 Agent Card、去重、保存、列表、删除。 |
| `backend/app/internal/a2a/model.go` | 市场 repository | 对 `agent_market` 执行 GORM 查询和写入。 |
| `backend/model/market.go` | 市场持久化模型 | 声明 `agent_market` 的字段与表名。 |
| `backend/app/internal/agents/service.go` | A2A 执行接入 | 在聊天执行中创建远程 A2A 子 Agent 并交给 Supervisor。 |
| `backend/a2a-server/main.go` | 独立协议服务 | 注册 `/a2a`、配置 Ollama、注册天气 Agent。 |
| `backend/a2a-server/etc/config.yml` | 服务监听配置 | 当前监听 `127.0.0.1:8777`。 |

## 接口与数据结构

### HTTP 接口

前端 Axios 实例的默认 `baseURL` 为 `/api`，响应拦截器从后端 `{ code, data, msg }` 中取出 `data`。因此以下为后端完整路径。

| 方法 | 路径 | 请求参数 | 作用 |
| --- | --- | --- | --- |
| POST | `/api/v1/a2a/getAgentCard` | `{ agentUrl, handlerPath? }` | 向远程端点发现 Agent Card。 |
| POST | `/api/v1/a2a/saveAgentCard` | `{ agentUrl, handlerPath }` | 发现并持久化一个市场 Agent。 |
| GET | `/api/v1/a2a/list` | 无 | 返回市场 Agent 列表。 |
| DELETE | `/api/v1/a2a/delete/:id` | UUID 路径参数 | 删除市场记录。 |
| POST | `/api/v1/agents/market/add` | `{ agentId, agentMarketIds }` | 将市场 Agent 关联到本地 Agent。 |
| POST | `/api/v1/agents/market/delete` | `{ agentId, agentMarketId }` | 移除上述关联。 |
| POST | `/api/v1/agents/chat` | `AgentMessageReq` | 触发含 A2A 子 Agent 的主聊天链路，响应为 SSE。 |

`GetAgentCardReq` 定义在 `backend/app/internal/a2a/req.go`，字段为 `AgentUrl` 与 `HandlerPath`。市场管理 handler 使用 `req.JsonParam` 绑定请求；Agent 管理相关 handler 还通过 `req.GetUserIdUUID` 取得当前用户。A2A 市场路由本身未声明单独中间件，实际认证行为取决于全局 Thunder/JWT 配置。

### A2A 协议入口

- 传输：Eino A2A JSON-RPC。
- 服务端路径：`http://localhost:8777/a2a`（代码与本地配置的组合）。
- Agent Card：由 `eino.RegisterServerHandlers` 基于 `ChatModelAgent` 注册；`AgentCardPath: nil`，具体发现 URL 由所用 Eino/Thunder A2A 库处理。
- 无 gRPC 或 protobuf 调用：本仓库的该功能使用 HTTP、JSON-RPC 和本地 Go service，不存在源码可证实的 gRPC 链路。

### 数据层

| 表 | 模型 | 用途 |
| --- | --- | --- |
| `agent_market` | `model.AgentMarket` | 保存远程 A2A 服务的 URL、名称、描述和 handler path。 |
| `agent_agents` | `model.AgentAgent` | 保存本地 Agent 与市场 Agent 的关联。 |

市场 repository 用 `Where("url = ?").First` 按 URL 去重，`Create` 写入市场记录，`Find` 列表查询，`Delete` 删除。关联 repository 使用 `(agent_id, agent_market_id)` 查询、创建和删除关系。当前未在 `backend/app/internal/inits/schema.go` 中定位到这两张表的显式 DDL；其建表来源需要结合 Thunder 数据库初始化或现有数据库进一步确认。

## 详细调用链

### 1. Agent 市场：发现并保存

1. `AgentMarket.vue` 的 `saveAgent()` 调用 `a2aService.saveAgentCard(agentUrl, handlerPath)`。
2. `frontend/src/api/a2aService.ts` 向 `/v1/a2a/saveAgentCard` 发起 POST；Axios 拼接为 `/api/v1/a2a/saveAgentCard`。
3. `backend/app/internal/router/markets.go` 的 `A2ARouter.Register` 注册该路由，`a2a.Handler.SaveAgentCard` 通过 `req.JsonParam` 获取 `GetAgentCardReq`。
4. `a2a.service.saveAgentCard` 先查 `agent_market` 是否已有相同 URL；存在时返回 `biz.ErrAgentCardExisted`。
5. 不存在时，service 调用 `thunder/ai/a2a.GetAgentCard(context.Background(), AgentUrl, HandlerPath)` 获取远程卡片。
6. service 取卡片的 `Name`、`Description` 连同 URL、handler path 组装 `model.AgentMarket`，并经 repository 的 `Create` 写入 `agent_market`。
7. handler 用 `res.Success` 返回；前端响应拦截器取 `data`，页面提示“添加成功”并重新拉取列表。

查看详情走相似路径：`handleViewDetails` → `/getAgentCard` → `service.getAgentCard` → `a2a.GetAgentCard`，但不落库。删除则是 `DELETE /delete/:id` → UUID 路径绑定 → `db.Delete(&model.AgentMarket{}, id)`。

### 2. 关联到本地 Agent

1. `AgentRouter` 注册 `/api/v1/agents/market/add` 和 `/market/delete`。
2. `agents.Handler.AddAgentAgent` / `DeleteAgentAgent` 绑定关联请求并获取 JWT 用户 ID。
3. `agents.service.addAgentAgent` 先确认本地 Agent 存在，随后对每个市场 ID 查询是否已有 `(agent_id, agent_market_id)` 关系；不存在则创建 `model.AgentAgent`。
4. `agents.service.deleteAgentAgent` 以该复合条件删除关联。
5. `model.Agent` 的 `Agents []*AgentMarket` 使用 `gorm:"many2many:agent_agents"` 声明加载关系，为后续聊天构建子 Agent 提供数据。

### 3. 聊天时调用远程 A2A Agent

1. `/api/v1/agents/chat` 路由进入 `agents.Handler.AgentMessage`；handler 设置 `text/event-stream`、关闭写入 deadline，并把 service 的 data/error channel 写回 SSE。
2. `agents.service.agentMessage` 读取本地 Agent、会话历史和关联的 `agent.Agents`，随后调用 `buildMainAgent` 创建本地主 Agent。
3. 对每个 `AgentMarket`，service 用 `jsonrpc.NewTransport` 建立 `{ BaseURL: v.URL, HandlerPath: v.HandlerPath }` transport。
4. service 依次调用 `client.NewA2AClient` 和 `eino.NewAgent`，把远程 A2A 服务包装为 Eino `adk.Agent`；单个远程 Agent 初始化失败时记录日志并跳过，不会立刻中断整个聊天。
5. `supervisor.New` 接收主 Agent 和 `subAgents`，`adk.NewRunner(... EnableStreaming: true)` 执行任务。Eino Supervisor 决定何时委派给远程子 Agent。
6. 迭代产生的事件经 service 的 data channel 回到 handler，最终作为 SSE `data:` 消息发送给前端。

### 4. 独立 A2A 服务端如何接收调用

1. `backend/a2a-server/main.go` 用 Thunder `config.Init()` 加载 `etc/config.yml`，并启动 Hertz。
2. `jsonrpc.NewRegistrar` 将 A2A JSON-RPC 协议注册到 `/a2a`。
3. 服务使用 `http://127.0.0.1:11434` 的 Ollama 模型 `modelscope.cn/Qwen/Qwen3-32B-GGUF:latest` 创建 ChatModelAgent。
4. 若环境变量 `AMAP_API_KEY` 存在，才会注册 `core/ai/tools.NewWeatherTool`；没有该变量时服务仍能启动，但没有天气工具可调用。
5. `eino.RegisterServerHandlers` 以 URL `http://localhost:8777` 注册该 Agent 的服务端 handler，随后 `h.Run()` 开始监听。

## 调试与验证

### 最小阅读路径

1. 市场管理：`frontend/src/views/AgentMarket.vue` → `frontend/src/api/a2aService.ts` → `backend/app/internal/router/markets.go` → `backend/app/internal/a2a/service.go`。
2. 远程执行：`backend/app/internal/router/agents.go` → `backend/app/internal/agents/handler.go` → `backend/app/internal/agents/service.go` 中的 `jsonrpc.NewTransport` 调用处。
3. 独立服务端：`backend/a2a-server/main.go` → `backend/a2a-server/etc/config.yml`。

### 最小验证步骤

```powershell
# 终端 1：启动独立 A2A 服务（需先启动本地 Ollama 并确保目标模型可用）
cd D:\MyAgent\backend\a2a-server
$env:AMAP_API_KEY = "<可选；启用天气工具时填写>"
go run .

# 终端 2：检查主业务 API 的市场列表（需先完成登录并替换 Token）
curl.exe -H "Authorization: Bearer <token>" http://localhost:8888/api/v1/a2a/list

# 在前端 Agent 市场中添加：Agent URL = http://localhost:8777，Handler Path = /a2a
# 然后将该市场 Agent 关联给一个本地 Agent，并在该本地 Agent 的聊天页发送天气请求。
```

### 建议断点位置

- `backend/app/internal/a2a/service.go`：`saveAgentCard` 中的 `a2a.GetAgentCard`，确认发现请求的 URL 与返回 Card。
- `backend/app/internal/agents/service.go`：`jsonrpc.NewTransport`、`client.NewA2AClient`，确认市场记录的 URL/路径被正确使用。
- `backend/a2a-server/main.go`：`eino.RegisterServerHandlers`，确认独立服务端已完成 Agent 注册。

## 注意点

- **前端获取详情遗漏了 handlerPath。** `a2aService.getAgentCard(agentUrl)` 只传 `agentUrl`，但后端 `GetAgentCardReq` 支持 `handlerPath`，而详情按钮也未把 `agent.handlerPath` 传入。因此非默认 handler 路径的详情发现是否成功依赖底层库的默认行为，当前前端没有把已保存的路径带回去。
- **前端声明与调用不一致。** `frontend/src/types/a2a.ts` 声明了 `A2AService.sendTask`、`getTaskStatus` 等接口，`WorkflowExecutor.ts` 和 `agentCommunicationService.ts` 也调用它们；但 `frontend/src/api/a2aService.ts` 实际只导出市场管理方法，未实现这些方法。该工作流/直连调用链当前没有可执行的前端实现证据。
- **市场 API 与协议服务是两层不同入口。** `/api/v1/a2a/*` 是主业务 API 的市场管理接口；远程协议请求发往外部服务的 `/a2a`，两者不可混用。
- **URL 去重不区分 handler path。** `saveAgentCard` 仅按 `URL` 查询已存在记录；同一主机下不同 handler path 不能同时作为不同市场条目保存。
- **远程子 Agent 只在聊天的 Supervisor 链路中调用。** 保存到市场不会自动执行，必须再建立 `agent_agents` 关联并触发本地 Agent 聊天。
- **独立服务的模型与 URL 是本地演示配置。** Ollama 地址、模型名称和 `http://localhost:8777` 写在 `main.go`，部署到其他机器时必须同步调整；Agent Card 对外 URL 也可能需要改为实际可访问地址。

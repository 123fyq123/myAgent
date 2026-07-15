# a2a-server 说明

## 模块定位
`a2a-server` 是后端独立 A2A JSON-RPC 服务，用 Eino ADK 将天气查询智能体暴露给外部 A2A 客户端。

## 主要职责
- 读取 Thunder 配置并启动 Hertz HTTP 服务。
- 在 `/a2a` 路径注册 A2A JSON-RPC handler。
- 构建 Ollama ChatModel 与 Eino ChatModelAgent。
- 复用 `core/ai/tools` 的天气工具作为智能体工具能力。

## 目录结构

### main.go / 入口文件
- `config.Init()` 加载 `etc/config.yml`。
- 从 `conf.Server` 拼接 Hertz 监听地址。
- `jsonrpc.NewRegistrar` 将 A2A handler 注册到 Hertz 路由。
- `ollama.NewChatModel` 创建本地 Ollama 模型客户端。
- `tools.NewWeatherTool` 创建天气工具。
- `adk.NewChatModelAgent` 创建名为“高德天气查询智能体”的 Eino 智能体。
- `eino.RegisterServerHandlers` 将智能体注册为 A2A 服务。
- `h.Run()` 启动 Hertz 服务。

### etc/
- `config.yml`：服务监听配置。

### go.mod / go.sum
- `go.mod`：模块名 `a2a-server`，声明 Go 1.24，依赖 Eino A2A 与 Hertz。
- `go.sum`：依赖校验文件。

## 上下游关系
- 上游：外部 A2A JSON-RPC 客户端。
- 下游：依赖本地 Ollama 服务、`core/ai/tools` 天气工具和 Thunder 配置加载。
- 共享依赖：CloudWeGo Eino ADK、Eino A2A 扩展、Hertz。

## 何时先看这个模块
- 修改或排查 A2A `/a2a` 服务入口时。
- 调整 A2A AgentCard、智能体名称、指令或工具绑定时。
- 验证 A2A 协议与 Eino 智能体集成方式时。

## 不要碰
- `go.sum` 由 Go 命令维护。
- `main.go` 中写死的 Ollama 地址和模型名属于本地演示配置，改动前要确认运行环境。
- `etc/config.yml` 的监听端口会影响外部 A2A 客户端连接。

# mcp-server 说明

## 模块定位
`mcp-server` 是独立运行的 MCP 服务，使用 SSE 向 MCP 客户端暴露由 `core` 提供的天气工具。

## 主要职责
- 启动监听 7777 端口的 Gin 服务。
- 创建支持工具、资源和提示能力的 MCP 服务。
- 把 Eino 工具元数据和调用接口转换为 MCP 工具协议，并提供 `/sse` 与 `/message` 端点。

## 目录结构

### main.go / 入口文件
- 依次执行 `config.Init`、读取配置、`logs.Init`、`server.NewServer`、`inits.Init` 和 `s.Start`。

### etc/
- `config.yml`：定义 7777 端口、日志和跨域配置。

### internal/inits/
- `inits.go`：注册事件处理器和 MCP 路由。

### internal/router/
- `mcp.go`：创建 MCP 与 SSE 服务，注册 `/sse` 和 `/message`。
- `event.go`：注册服务事件处理逻辑。

### internal/tool/
- `base.go`：定义 MCP 工具的抽象接口。
- `weather.go`：以 `core/ai/tools` 的天气工具为基础构建 MCP 工具，转换参数 schema 并转发调用。

## 上下游关系
- 上游：支持 MCP SSE 传输的客户端连接 `/sse` 并经 `/message` 发送工具调用。
- 下游：调用高德天气 API，并依赖 Thunder 的 HTTP 服务和配置能力。
- 共享依赖：依赖 `core/ai/tools` 的天气工具定义。

## 何时先看这个模块
- 需要调整 MCP 协议能力、SSE 端点、工具参数映射或工具调用返回值时。
- 需要排查独立 MCP 服务启动和客户端连接问题时。

## 不要碰
- `etc/config.yml` 是本地运行配置，修改端口或跨域规则前应确认部署环境。

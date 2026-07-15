# mcp-server 说明

## 模块定位
`mcp-server` 是后端独立 MCP 服务，使用 Gin 与 mcp-go 通过 SSE 暴露工具能力。

## 主要职责
- 启动 Thunder/Gin 服务。
- 注册 MCP SSE 与 message 端点。
- 将天气工具注册为 MCP 工具。
- 提供事件路由扩展点。

## 目录结构

### main.go / 入口文件
- `config.Init()` 加载 `etc/config.yml`。
- `logs.Init(conf.Log)` 初始化日志。
- `server.NewServer(conf)` 创建 Thunder 服务。
- `inits.Init(s, conf)` 注册 MCP 和事件路由。
- `s.Start()` 启动 HTTP 服务。

### etc/
- `config.yml`：服务监听和日志等配置。

### internal/inits/
- `inits.go`：注册 `router.Event` 与 `router.McpRouter`。

### internal/router/
- `mcp.go`：创建 mcp-go 服务，注册天气工具，暴露 `/sse` 和 `/message`。
- `event.go`：事件路由扩展点。

### internal/tool/
- `weather.go`：MCP 侧天气工具构建与调用。
- `base.go`：工具基础结构或公共配置。

## 上下游关系
- 上游：外部 MCP 客户端通过 `/sse` 和 `/message` 调用。
- 下游：依赖 `internal/tool` 天气工具、mcp-go、Gin 和 Thunder 服务启动能力。
- 共享依赖：`core/ai/tools` 在依赖扫描中被引用，MCP 工具语义与核心工具保持接近。

## 何时先看这个模块
- 修改 MCP 服务端协议入口、SSE 地址或 message 地址时。
- 新增 MCP 工具或排查天气工具调用问题时。
- 调整 MCP 服务名、协议版本或工具能力声明时。

## 不要碰
- `go.sum` 由 Go 命令维护。
- `mcp.go` 中 `WithBaseURL("http://localhost:7777")` 与配置端口强相关，改动前要确认客户端连接方式。
- `etc/config.yml` 可能含本地运行配置，提交前要审查。

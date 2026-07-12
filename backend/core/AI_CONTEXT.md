# core 说明

## 模块定位
`core` 是被主 API 服务和 MCP 服务共同复用的 AI 基础库，封装消息格式、MCP 客户端和系统工具。

## 主要职责
- 构建智能体输出、推理和错误消息文本。
- 连接 MCP SSE 服务并转换 MCP 工具为 Eino 可调用工具。
- 注册、查询和构建系统天气工具。

## 目录结构

### go.mod / 模块文件
- 声明 `core` 模块，源码使用 Eino、MCP Go 和 Thunder 的 AI 工具接口。

### ai/
- `message.go`：构建智能体消息、工具推理消息和错误消息。
- `template.go`：定义 AI 相关提示或文本模板。

### ai/mcps/
- `mcp.go`：实现 MCP SSE 客户端连接及 MCP 工具到 Eino 工具的转换。

### ai/tools/
- `registry.go`：维护系统工具注册表并按名称查询工具。
- `weather_tool.go`：定义调用高德天气接口的 Eino 天气工具。
- `weather_tool_test.go`：覆盖天气工具行为。

## 上下游关系
- 上游：`app` 用于注册和调用系统工具、连接 MCP；`mcp-server` 用于构建天气工具。
- 下游：依赖 Eino 工具和 schema 接口、MCP Go 客户端以及 Thunder AI 工具抽象。
- 共享依赖：不依赖工作区内其他模块。

## 何时先看这个模块
- 需要新增系统工具、修改天气工具或排查工具注册时。
- 需要对接外部 MCP SSE 服务或调整 MCP 到 Eino 的适配时。
- 需要统一智能体输出的消息结构时。

## 不要碰
- `go.sum` 不存在于该模块；若后续生成，应由 Go 模块工具维护。

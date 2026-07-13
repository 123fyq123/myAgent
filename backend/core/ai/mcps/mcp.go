package mcps

import (
	"context"
	"fmt"
	"strings"

	mcpp "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/components/tool"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mszlu521/thunder/ai/einos"
)

// GetEinoBaseTools 根据 MCP 配置，连接远程 MCP 服务器并动态获取其可用的工具列表
func GetEinoBaseTools(ctx context.Context, config *einos.McpConfig) ([]tool.BaseTool, error) {
	// 1. 鉴权准备：构建 HTTP 请求头
	headers := make(map[string]string)
	if config.Token != "" {
		// 如果配置了 Token，则加上标准的 Bearer 认证请求头，用于通过远程服务器的鉴权
		headers["Authorization"] = fmt.Sprintf("Bearer %s", config.Token)
	}

	// 将请求头封装为通用的传输层配置项
	options := transport.WithHeaders(headers)
	url := config.BaseUrl

	var cli *client.Client
	var err error

	// 2. 连接策略适配：根据 URL 后缀决定使用哪种底层通信网络协议
	// --- 策略 A：如果 URL 以 "/sse" 结尾，说明远程服务支持 SSE（Server-Sent Events）双向通信协议 ---
	if strings.HasSuffix(url, "/sse") {
		cli, err = client.NewSSEMCPClient(url, options)
		if err != nil {
			return nil, err
		}
		// --- 策略 B：否则，使用兼容流式传输的标准 HTTP 客户端（Streamable HTTP） ---
	} else {
		cli, err = client.NewStreamableHttpClient(url, transport.WithHTTPHeaders(headers))
		if err != nil {
			return nil, err
		}
	}

	// 3. 启动客户端：正式与远程服务器建立网络长连接
	err = cli.Start(ctx)
	if err != nil {
		return nil, err
	}

	// 4. 构建 MCP 协议标准的初始化请求体（握手协议）
	initRequest := mcp.InitializeRequest{}
	// 传入当前客户端支持的最新的 MCP 协议版本号
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	// 传入当前智能体/客户端的名称和版本元数据，让远程服务器知道是谁连过来了
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    config.Name,
		Version: config.Version,
	}

	// 5. 向远程服务器发起 Initialize（初始化握手）调用
	//    双方对齐协议版本，握手成功后连接才算真正可用
	_, err = cli.Initialize(ctx, initRequest)
	if err != nil {
		return nil, err
	}

	// 6. 核心操作：通过 MCP 协议插件（mcpp）向远程服务器发送“工具列表查询”请求
	//    远程服务器会返回它所拥有的所有工具的名称、描述以及入参的 JSON Schema (也就是你之前看到的 Params 规矩)
	tools, err := mcpp.GetTools(ctx, &mcpp.Config{Cli: cli})
	if err != nil {
		return nil, err
	}

	// 7. 返回动态获取到的工具箱切片，这批工具可以直接喂给大模型做 Tool Calling
	return tools, nil
}

func GetMCPTool(ctx context.Context, config *einos.McpConfig) ([]mcp.Tool, error) {
	headers := make(map[string]string)
	if config.Token != "" {
		headers["Authorization"] = fmt.Sprintf("Bearer %s", config.Token)
	}
	options := transport.WithHeaders(headers)
	url := config.BaseUrl
	//支持streamable http
	var cli *client.Client
	var err error
	if strings.HasSuffix(url, "/sse") {
		cli, err = client.NewSSEMCPClient(url, options)
		if err != nil {
			return nil, err
		}
	} else {
		cli, err = client.NewStreamableHttpClient(url, transport.WithHTTPHeaders(headers))
		if err != nil {
			return nil, err
		}
	}
	err = cli.Start(ctx)
	if err != nil {
		return nil, err
	}

	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    config.Name,
		Version: config.Version,
	}

	_, err = cli.Initialize(ctx, initRequest)
	if err != nil {
		return nil, err
	}
	tools, err := cli.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, err
	}

	return tools.Tools, nil
}

package main

import (
	"context"
	"core/ai/tools"
	"fmt"
	"os"

	// 引入 CloudWeGo 的 A2A 协议扩展与传输层组件 (JSON-RPC)
	"github.com/cloudwego/eino-ext/a2a/extension/eino"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc"

	// 引入 Ollama 模型适配器
	"github.com/cloudwego/eino-ext/components/model/ollama"
	// 引入 Eino 框架的 ADK (Agent Development Kit) 开发套件
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"

	// 引入 Hertz Web 框架服务端
	hertzServer "github.com/cloudwego/hertz/pkg/app/server"
	// 引入自定义配置管理包
	"github.com/mszlu521/thunder/config"
)

func main() {
	// 1. 初始化并加载 etc/config.yml 中的全局配置文件
	config.Init()
	// 获取读取到的配置对象
	conf := config.GetConfig()
	// 从配置中获取监听的主机 IP 和端口号，格式化为 "ip:port" 字符串
	addr := fmt.Sprintf("%s:%d", conf.Server.GetHost(), conf.Server.GetPort())

	// 2. 初始化 Hertz HTTP Web 服务器实例
	h := hertzServer.Default(
		hertzServer.WithHostPorts(addr),                // 设置服务器监听的 IP 和端口
		hertzServer.WithSenseClientDisconnection(true), // 开启客户端断开连接的感知能力，方便及时释放资源
	)

	// 创建全局根 Context 上下文
	ctx := context.Background()

	// 3. 基于 JSON-RPC 协议创建 A2A 协议注册器 (Registrar)
	r, err := jsonrpc.NewRegistrar(ctx, &jsonrpc.ServerConfig{
		Router:        h,      // 将 Hertz 路由绑定给 A2A 注册器
		HandlerPath:   "/a2a", // 指定 A2A 消息交互与任务调用的基础路由 Path
		AgentCardPath: nil,    // 传 nil 表示使用默认的名片路径 (通常为 /.well-known/agent.json)
	})
	if err != nil {
		panic(err) // 如果注册器初始化失败，打印异常信息并中断程序启动
	}

	// 4. 初始化 LLM 大语言模型 (连接本地 Ollama 服务)
	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL: "http://127.0.0.1:11434",                   // 本地 Ollama 服务的 API 地址
		Model:   "modelscope.cn/Qwen/Qwen3-32B-GGUF:latest", // 使用的模型名称/版本
	})
	if err != nil {
		panic(err) // 模型连接/创建失败，终止程序
	}

	// 5. 初始化工具节点配置，初始为空切片
	toolsNodeConfig := compose.ToolsNodeConfig{
		Tools: []tool.BaseTool{},
	}

	// 检查系统环境变量中是否配置了高德地图 API Key (AMAP_API_KEY)
	// 未配置时跳过工具注册，保证基础 Agent 服务依旧可正常编译和启动
	if apiKey := os.Getenv("AMAP_API_KEY"); apiKey != "" {
		// 实例化高德天气查询工具
		weatherTool := tools.NewWeatherTool(&tools.WeatherConfig{
			ApiKey: apiKey,
		})
		// 将天气工具追加到 Agent 的可用工具列表中
		toolsNodeConfig.Tools = append(toolsNodeConfig.Tools, weatherTool)
	}

	// 6. 使用 ADK 创建核心的 ChatModelAgent (智能体实体)
	chatModelAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "高德天气查询智能体",                 // 智能体名称 (会被写入名片暴露给外部)
		Description: "一个可以查询天气的智能体",              // 智能体功能描述 (暴露给外部市场)
		Instruction: "你是一个天气助手，请使用高德天气API查询天气信息", // System Prompt 设定 Agent 的角色与人设
		Model:       chatModel,                   // 注入之前初始化的 LLM 模型
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: toolsNodeConfig, // 绑定 Agent 可以调用的工具列表
		},
	})
	if err != nil {
		panic(err) // Agent 构建失败，终止程序
	}

	// 7. 将创建好的 Agent 绑定到 A2A 协议并注册路由 Handler
	err = eino.RegisterServerHandlers(ctx, chatModelAgent, &eino.ServerConfig{
		Registrar: r,                       // 使用之前创建的 JSON-RPC 注册器
		URL:       "http://localhost:8777", // 宣告当前 Agent 外部可访问的基准 Base URL
	})
	if err != nil {
		panic(err) // 路由注册失败，终止程序
	}

	// 8. 正式启动 Hertz Web 服务器，开始监听网络请求
	err = h.Run()
	if err != nil {
		panic(err) // 服务器运行异常崩溃，终止程序
	}
}

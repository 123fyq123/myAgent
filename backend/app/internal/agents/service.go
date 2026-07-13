package agents

import (
	"app/shared"
	"common/biz"
	"context"
	"core/ai"
	"core/ai/mcps"
	"core/ai/tools"
	"encoding/json"
	"errors"
	"fmt"
	"model"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino-ext/components/model/qwen"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/supervisor"
	aiModel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/ollama/api"
	"github.com/google/uuid"
	"github.com/mszlu521/thunder/ai/einos"
	"github.com/mszlu521/thunder/database"
	"github.com/mszlu521/thunder/errs"
	"github.com/mszlu521/thunder/event"
	"github.com/mszlu521/thunder/logs"
)

type service struct {
	repo repository
}

func (s *service) createAgent(ctx context.Context, userId uuid.UUID, req CreateAgentReq) (any, error) {
	//子上下文 不能超过10s
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	agent := model.DefaultAgent(userId, req.Name, req.Description, req.Status)
	err := s.repo.createAgent(ctx, agent)
	if err != nil {
		logs.Errorf("创建智能代理失败: %v", err)
		return nil, errs.DBError
	}
	return agent, nil
}

func (s *service) listAgents(ctx context.Context, userID uuid.UUID, req SearchAgentReq) (*ListAgentResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	filter := AgentFilter{
		Name:   req.Params.Name,
		Status: req.Params.Status,
		Limit:  req.Params.PageSize,
		Offset: (req.Params.Page - 1) * req.Params.PageSize,
	}
	list, total, err := s.repo.listAgents(ctx, userID, filter)
	if err != nil {
		logs.Errorf("查询智能代理列表失败: %v", err)
		return nil, errs.DBError
	}
	return &ListAgentResponse{
		Agents: list,
		Total:  total,
	}, nil
}

// getAgent 根据用户ID和代理ID查询智能代理信息
//
// 参数:
//
//	ctx: 上下文，用于控制请求超时和取消
//	userID: 用户唯一标识符，用于权限验证
//	id: 智能代理唯一标识符
//
// 返回值:
//
//	*model.Agent: 查询到的智能代理对象指针
//	error: 错误信息，可能包含:
//	  - errs.DBError: 数据库查询失败
//	  - biz.AgentNotFound: 代理不存在
//
// 功能说明:
//   - 设置5秒超时上下文，防止长时间阻塞
//   - 调用repository层查询代理信息
//   - 处理查询失败和代理不存在的情况
//   - 返回查询结果或相应错误
func (s *service) getAgent(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*model.Agent, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	agent, err := s.repo.getAgent(ctx, userID, id)
	if err != nil {
		logs.Errorf("查询智能代理失败: %v", err)
		return nil, errs.DBError
	}
	if agent == nil {
		return nil, biz.AgentNotFound
	}
	return agent, nil
}

// 更新agent
func (s *service) updateAgent(ctx context.Context, userId uuid.UUID, req UpdateAgentReq) (any, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	//先查询id是否存在
	agent, err := s.repo.getAgent(ctx, userId, req.ID)
	if err != nil {
		logs.Errorf("查询智能代理失败: %v", err)
		return nil, errs.DBError
	}
	if agent == nil {
		return nil, biz.AgentNotFound
	}
	//对更新的字段进行判断
	if req.Name != "" {
		agent.Name = req.Name
	}
	if req.Description != "" {
		agent.Description = req.Description
	}
	if req.Status != "" {
		agent.Status = req.Status
	}
	if req.SystemPrompt != "" {
		agent.SystemPrompt = req.SystemPrompt
	}
	if req.ModelProvider != "" {
		agent.ModelProvider = req.ModelProvider
	}
	if req.ModelName != "" {
		agent.ModelName = req.ModelName
	}
	if req.ModelParameters != nil {
		agent.ModelParameters = req.ModelParameters
	}
	if req.OpeningDialogue != "" {
		agent.OpeningDialogue = req.OpeningDialogue
	}
	err = s.repo.updateAgent(ctx, agent)
	if err != nil {
		logs.Errorf("更新智能代理失败: %v", err)
		return nil, errs.DBError
	}
	return agent, nil
}

// agentMessage 处理与智能体的对话，采用流式返回（通过两个单向通道传递数据和错误）
func (s *service) agentMessage(ctx context.Context, userID uuid.UUID, req AgentMessageReq) (<-chan string, <-chan error) {
	// 创建用于传递模型流式响应数据（JSON 字符串）的通道
	dataChan := make(chan string)
	// 创建用于传递异步处理过程中发生错误的通道
	errChan := make(chan error)

	// 开启一个独立的 Goroutine（协程）去异步处理 AI 的流式调用，避免阻塞主线程
	go func() {
		// defer 函数会在当前 Goroutine 结束时（无论正常退出还是发生 panic）被最后调用
		defer func() {
			// 捕获可能发生的 panic，防止子协程崩溃导致整个 Go 服务挂掉（进程退出）
			if err := recover(); err != nil {
				logs.Errorf("处理智能代理消息失败: %v", err)
				// 尝试将致命错误写入错误通道，并配合 select 防止通道阻塞
				select {
				case errChan <- errors.New("internal server error"):
				case <-ctx.Done(): // 如果客户端已经取消了连接，则放弃写入
					logs.Warnf("发送取消 context Done")
				}
			}
			// 必须在最后关闭这两个通道！通知外部的消费者（如 HTTP SSE/WebSocket 处理器）数据已发完
			close(dataChan)
			close(errChan)
		}()

		// 1. 从数据库/缓存中获取当前 Agent 的元数据（包括其配置、绑定的工具和知识库）
		agent, err := s.repo.getAgent(ctx, userID, req.AgentID)
		if err != nil {
			logs.Errorf("查询智能代理失败: %v", err)
			// 自定义方法：封装错误并通过 errChan 发送给外部消费者，随后退出协程
			s.sendError(ctx, errChan, err)
			return
		}

		// 2. 基于 Eino ADK 构建主智能体（Main Agent），负责直接对接用户输入和分发任务
		mainAgent, err := s.buildMainAgent(ctx, agent, req.Message, dataChan)
		if err != nil {
			logs.Errorf("构建主智能体失败: %v", err)
			s.sendError(ctx, errChan, err)
			return
		}

		// 3. 构建 Supervisor 智能体（协同管理器）。
		// 这种模式下，mainAgent 作为主管，可以管理 SubAgents 数组里的多个子智能体进行团队协作
		supervisorAgent, err := supervisor.New(ctx, &supervisor.Config{
			Supervisor: mainAgent,
			SubAgents:  []adk.Agent{
				// 这里预留了扩展空间，未来可以根据配置添加不同的专业子 Agent（如：代码Agent、翻译Agent）
			},
		})
		if err != nil {
			logs.Errorf("构建supervisorAgent失败: %v", err)
			s.sendError(ctx, errChan, err)
			return
		}

		// 4. 构建执行器（Runner），并开启流式传输支持（EnableStreaming: true）
		runner := adk.NewRunner(ctx, adk.RunnerConfig{
			Agent:           supervisorAgent,
			EnableStreaming: true,
		})

		// 5. 向智能体集群发起流式查询，返回一个流式迭代器 iter
		iter := runner.Query(ctx, req.Message)

		// 6. 开启死循环，死磕迭代器，实时接收大模型输出的每一个数据碎片（Chunk）
		for {
			// 获取流式返回的下一个事件节点
			events, ok := iter.Next()
			if !ok {
				// 如果 ok 为 false，代表大模型吐字结束或流已断开，跳出循环
				break
			}

			// 7. 每一次循环前，通过 select 检查客户端是否已经主动取消了请求（例如关闭了网页）
			select {
			case <-ctx.Done():
				logs.Warnf("客户端取消了请求")
				return // 客户端撤了，服务方立刻终止计算，节省算力
			default:
				// 如果没有取消，则走 default 分支，不阻塞，继续往下执行
			}

			// 8. 检查该事件节点中是否包含智能体执行错误（如模型内部报错、Tool 调用失败等）
			if events.Err != nil {
				// 将 Agent 级别的错误包装为特定 JSON 格式，通过数据通道返回给前端展示
				s.sendData(ctx, dataChan, ai.BuildErrMessage(events.AgentName, events.Err.Error()))
				return
			}

			// 9. 检查是否有有效的内容输出
			if events.Output != nil && events.Output.MessageOutput != nil {
				// 从框架的 MessageOutput 中解析出标准的统一消息结构体
				msg, err := events.Output.MessageOutput.GetMessage()
				if err != nil {
					logs.Errorf("获取模型返回内容失败: %v", err)
					s.sendError(ctx, errChan, err)
					return
				}

				// 如果内容和深度思考都是空的，说明是空包（可能是纯元数据元事件），直接跳过
				if msg.Content == "" && msg.ReasoningContent == "" {
					continue
				}

				// 10. 处理深度思考内容（Reasoning）—— 对应 DeepSeek-R1 这类模型的 <think> 标签内容
				if msg.ReasoningContent != "" {
					// 封装为思考消息流，实时推给前端展示思维链动画
					s.sendData(ctx, dataChan, ai.BuildReasoningMessage(events.AgentName, msg.ToolName, msg.ReasoningContent))
				}

				// 打印日志，记录是哪个 Agent 在用哪个 Tool 输出了什么内容
				logs.Infof("Agent名称[%s], 工具名称:[%s], 模型返回内容: %s", events.AgentName, msg.ToolName, msg.Content)

				// 11. 处理最终的回答文本内容（Content）
				if msg.Content != "" {
					// 封装为普通文本消息流，实时推给前端渲染文字
					s.sendData(ctx, dataChan, ai.BuildMessage(events.AgentName, msg.ToolName, msg.Content))
				}
			}
		}
	}()

	// 将两个通道作为只读通道（<-chan）返回给外部调用者（通常是 Controller 层）
	return dataChan, errChan
}

func (s *service) sendError(ctx context.Context, errChan chan error, err error) {
	select {
	case errChan <- err:
	case <-ctx.Done():
		logs.Warnf("发送取消 context Done")
	}
}

// buildMainAgent 组装并构建 Eino 框架所需的 ChatModelAgent (主智能体)
func (s *service) buildMainAgent(ctx context.Context, agent *model.Agent, message string, dataChan chan string) (adk.Agent, error) {
	// 1. 获取该 Agent 配置的大模型厂商信息 (如 OpenAI, Anthropic, 火山方舟等) 和具体的模型名称 (如 gpt-4o, deepseek-v3)
	providerConfig, err := s.getProviderConfig(ctx, model.LLMTypeChat, agent.ModelProvider, agent.ModelName)
	if err != nil {
		return nil, errs.DBError // 数据库查询失败，返回统一定义的数据库错误
	}
	if providerConfig == nil {
		return nil, biz.ErrProviderConfigNotFound // 如果没找到该厂商的配置（比如 API Key 没配置），返回业务错误
	}

	// 2. 根据厂商配置，实例化真正的对话模型对象 (ChatModel)。
	// 这里会做多厂商适配，并让模型具备 Tool Calling (工具回调/函数调用) 的能力
	chatModel, err := s.buildToolCallingChatModel(ctx, agent, providerConfig)
	if err != nil {
		logs.Errorf("构建chatmodel失败: %v", err)
		return nil, err
	}

	// 3. 构建工具箱：将这个 Agent 在数据库里关联的所有工具，转换为 Eino 框架识别的 BaseTool 切片
	var allTools []tool.BaseTool
	allTools = append(allTools, s.buildTools(agent)...)

	// 4. 执行 RAG (检索增强生成)：拿着用户当前输入的 message，去向量数据库里检索关联的知识库内容。
	// 检索出来的文本会被包装成 ragContext，后续会塞进提示词里，让 AI 拥有背景知识
	ragContext := s.buildRagContext(ctx, dataChan, message, agent)

	// 5. 调用 Eino ADK 正式创建基于对话模型的智能体 (ChatModelAgent)
	modelAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Model:       chatModel,           // 注入上面适配好的大模型引擎
		Name:        agent.Name,          // 智能体名称
		Description: agent.Description,   // 智能体描述
		Instruction: ai.BaseSystemPrompt, // 注入系统基础提示词模板（作为兜底或骨架）

		// GenModelInput 是一个极其关键的回调函数！
		// 它的触发时机是：在请求真正发送给大模型的前一刻。用来做最后的提示词动态组装和润色。
		GenModelInput: func(ctx context.Context, instruction string, input *adk.AgentInput) ([]adk.Message, error) {

			// 使用 Eino 的 prompt 组件，基于我们定义的基础系统提示词模板创建一个格式化模板
			template := prompt.FromMessages(schema.FString, schema.SystemMessage(ai.BaseSystemPrompt))

			// 动态替换模板中的变量，把当前 Agent 的个性化设定、知识库上下文、工具信息全部合成为一条最终的 System Message
			messages, err2 := template.Format(ctx, map[string]any{
				"role":       agent.SystemPrompt,          // 注入用户在后台给这个 Agent 写的个性化角色设定/人设
				"ragContext": ragContext,                  // 注入刚刚从向量数据库检索出来的知识库切片
				"toolsInfo":  s.formatToolsInfo(allTools), // 将工具的名称和用途描述格式化为文本，让 AI 知道自己有哪些工具可用
				"agentsInfo": "",                          // 预留字段：协同智能体的信息（当前主智能体暂不填）
			})
			if err2 != nil {
				logs.Errorf("格式化模板失败: %v", err2)
				return nil, err2
			}

			// 将组装好的系统提示词 (System Message) 和用户实际输入的历史聊天消息队列 (input.Messages) 合并起来
			messages = append(messages, input.Messages...)

			// 返回最终构造成型的、包含完整上下文的消息链路，准备丢给大模型
			return messages, nil
		},

		// 6. 注入工具配置，大模型在生成回复时，如果发现需要调用工具，会根据这里的配置选择合适的工具执行
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: allTools, // 把工具箱挂载到智能体节点上
			},
		},
	})
	if err != nil {
		logs.Errorf("构建ChatModelAgent失败: %v", err)
		return nil, err
	}

	// 7. 构建成功，返回这个具备完整灵智（模型、人设、知识库、工具）的智能体对象
	return modelAgent, nil
}

// getProviderConfig 根据大模型厂商和模型名称，获取对应的提供商配置信息（如 API Key、Base URL 等）
// 入参：
//   - chat: 语言模型类型（例如：Chat 对话模型、Embedding 向量模型等）
//   - provider: 模型厂商名称（例如：openai, anthropic, ark）
//   - name: 具体模型名称（例如：gpt-4o, deepseek-v3）
func (s *service) getProviderConfig(ctx context.Context, chat model.LLMType, provider string, name string) (*model.ProviderConfig, error) {

	// 1. 核心操作：通过事件触发器，跨模块/跨服务发起调用。
	//    事件名称为 "getProviderConfig"，并把入参打包成一个共享的请求结构体指针 &shared.GetProviderConfigsRequest
	trigger, err := event.Trigger("getProviderConfig", &shared.GetProviderConfigsRequest{
		Provider:  provider,
		ModelName: name,
		LLMType:   chat,
	})

	// 2. 检查事件触发或执行过程中是否发生错误（比如：没有注册对应的处理器，或者远端服务报错）
	if err != nil {
		logs.Errorf("触发getProviderConfig事件失败: %v", err)
		// 打印具体错误日志后，向外屏蔽细节，返回统一定义的业务错误
		return nil, errs.DBError
	}

	// 3. 这里的 trigger 变量是 any (interface{}) 类型。
	//    由于我们明确知道 "getProviderConfig" 事件处理器成功后一定会返回 *model.ProviderConfig 类型的指针，
	//    所以这里使用 Go 的“类型断言 (Type Assertion)”：trigger.(*model.ProviderConfig) 将其转换为具体类型。
	return trigger.(*model.ProviderConfig), nil
}

// buildToolCallingChatModel 根据传入的厂商配置与智能体参数，动态实例化并返回一个统一的、支持工具调用的聊天模型对象
func (s *service) buildToolCallingChatModel(ctx context.Context, agent *model.Agent, config *model.ProviderConfig) (aiModel.ToolCallingChatModel, error) {
	var chatModel aiModel.ToolCallingChatModel
	var err error

	// 1. 解析参数：将数据库中存储的模型配置（可能是 JSON 或加密文本）转换为统一的业务参数结构体
	modelParams := agent.ModelParameters.ToModelParams()

	// 2. 类型转换与类型对齐：
	//    由于各厂商 SDK 要求的浮点数类型不同，这里统一从 float64 转为 Go 开发中更常用的 float32
	temperature := float32(modelParams.Temperature)
	topP := float32(modelParams.TopP)
	maxTokens := modelParams.MaxTokens // 通常是 int 类型

	// 3. 多厂商分支适配开始：根据 config.Provider 决定实例化哪家模型

	// --- 分支 A：Ollama 厂商（用于本地私有化部署的大模型，如 Llama, DeepSeek 本地版） ---
	if config.Provider == model.OllamaProvider {
		chatModel, err = ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
			Model:   agent.ModelName, // 具体模型名，例如 "deepseek-r1:14b"
			BaseURL: config.APIBase,  // 本地服务地址，例如 "http://localhost:11434"
			Options: &api.Options{
				Temperature: temperature,
				TopP:        topP,
				Runner: api.Runner{
					// 注意：Ollama 中用 NumCtx 来定义或限制上下文窗口/最大 token 长度
					NumCtx: maxTokens,
				},
			},
		})

		// --- 分支 B：OpenAI 官方服务（或者完全兼容 OpenAI 协议的标准云服务） ---
	} else if config.Provider == model.OpenAIProvider {
		// OpenAI 的 SDK 配置通常需要传入指针类型（以支持 nil 代表使用官方默认值），所以这里用了 & 符号取地址
		chatModel, err = openai.NewChatModel(ctx, &openai.ChatModelConfig{
			Model:               agent.ModelName, // 例如 "gpt-4o"
			BaseURL:             config.APIBase,  // 例如 "https://api.openai.com/v1"
			APIKey:              config.APIKey,   // 鉴权密钥
			MaxCompletionTokens: &maxTokens,      // 限制最大生成 token 数
			Temperature:         &temperature,
			TopP:                &topP,
		})

		// --- 分支 C：阿里云通义千问 (Qwen) 厂商 ---
	} else if config.Provider == model.QwenProvider {
		chatModel, err = qwen.NewChatModel(ctx, &qwen.ChatModelConfig{
			Model:       agent.ModelName, // 例如 "qwen-max"
			BaseURL:     config.APIBase,  // 灵积或百炼平台的 API 地址
			APIKey:      config.APIKey,
			MaxTokens:   &maxTokens, // 千问对应的参数名字叫 MaxTokens
			Temperature: &temperature,
			TopP:        &topP,
		})

		// --- 分支 D：兜底/默认策略 ---
	} else {
		// 这是一个非常聪明的工程设计。目前市面上绝大多数大模型中转站、新厂商（如智谱、月之暗面、DeepSeek 官方 API）
		// 都百分之百兼容 OpenAI 的 HTTP 协议。如果遇到代码中没写死的老虎或新厂商，直接套用 OpenAI 的配置即可无缝运行。
		chatModel, err = openai.NewChatModel(ctx, &openai.ChatModelConfig{
			Model:               agent.ModelName,
			BaseURL:             config.APIBase,
			APIKey:              config.APIKey,
			MaxCompletionTokens: &maxTokens,
			Temperature:         &temperature,
			TopP:                &topP,
		})
	}

	// 4. 返回组装完毕的模型对象和初始化错误（如果有的话）
	return chatModel, err
}

func (s *service) sendData(ctx context.Context, dataChan chan string, data string) {
	select {
	case dataChan <- data:
	case <-ctx.Done():
		logs.Warnf("sendData 发送取消 context Done")
	}
}

func (s *service) updateAgentTool(ctx context.Context, userID uuid.UUID, agentId uuid.UUID, req UpdateAgentToolReq) (any, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()
	//先检查agent是否存在
	agent, err := s.repo.getAgent(ctx, userID, agentId)
	if err != nil {
		return nil, errs.DBError
	}
	if agent == nil {
		return nil, biz.AgentNotFound
	}
	if len(req.Tools) <= 0 {
		return nil, biz.ErrToolNotExisted
	}
	//先删除agent现有关联的工具
	err = s.repo.deleteAgentTools(ctx, agentId)
	if err != nil {
		return nil, errs.DBError
	}
	//创建新的关联记录
	var agentTools []*model.AgentTool
	var toolIds []uuid.UUID
	for _, v := range req.Tools {
		toolIds = append(toolIds, v.ID)
	}
	//获取到工具的ID，去工具表查询出对应的工具信息
	toolsList, err := s.getToolsByIds(toolIds)
	for _, t := range toolsList {
		agentTools = append(agentTools, &model.AgentTool{
			AgentID:   agentId,
			ToolID:    t.ID,
			Status:    model.Enabled,
			CreatedAt: time.Now(),
		})
	}
	//批量插入
	err = s.repo.createAgentTools(ctx, agentTools)
	if err != nil {
		logs.Errorf("批量插入agent_tools失败: %v", err)
		return nil, errs.DBError
	}
	return agentTools, nil
}

func (s *service) getToolsByIds(ids []uuid.UUID) ([]*model.Tool, error) {
	//这里我们一会去实现event 获取工具信息
	trigger, err := event.Trigger("getToolsByIds", &shared.GetToolsByIdsRequest{
		Ids: ids,
	})
	return trigger.([]*model.Tool), err
}

func (s *service) buildTools(agent *model.Agent) []tool.BaseTool {
	var agentTools []tool.BaseTool
	for _, v := range agent.Tools {
		//这里面工具的类型有system和mcp两种，我们这里先处理system
		switch v.ToolType {
		case model.SystemToolType:
			systemTool := s.loadSystemTool(v.Name)
			if systemTool == nil {
				logs.Warnf("加载系统工具时，找不到工具: %v", v.Name)
				continue
			}
			agentTools = append(agentTools, systemTool)
		case model.McpToolType:
			//获取到mcp的所有tools，并且需要转换为eino的tool
			mcpConfig := einos.McpConfig{
				BaseUrl: v.McpConfig.Url,
				Token:   v.McpConfig.CredentialType,
				Name:    "mszlu-AI",
				Version: "1.0.0",
			}
			baseTools, err := mcps.GetEinoBaseTools(context.Background(), &mcpConfig)
			if err != nil {
				logs.Errorf("获取mcp tools失败: %v", err)
				continue
			}
			agentTools = append(agentTools, baseTools...)
		default:
			logs.Warnf("未知的工具类型: %v", v.ToolType)

		}
	}
	return agentTools
}

func (s *service) loadSystemTool(name string) tool.BaseTool {
	return tools.FindTool(name)
}

func (s *service) formatToolsInfo(allTools []tool.BaseTool) string {
	var builder strings.Builder
	builder.WriteString("【可用工具列表】\n")
	for _, t := range allTools {
		info, _ := t.Info(context.Background())
		builder.WriteString(fmt.Sprintf("- name: `%s` \n", info.Name))
		builder.WriteString(fmt.Sprintf("  description: `%s` \n", info.Desc))
		//参数要转成json字符串
		marshal, _ := json.Marshal(info.ParamsOneOf)
		builder.WriteString(fmt.Sprintf("  params: `%s` \n", string(marshal)))
	}
	return builder.String()
}

func (s *service) addAgentKnowledgeBase(ctx context.Context, userId uuid.UUID, agentId uuid.UUID, addReq addAgentKnowledgeBaseReq) (any, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()
	//先检查agent是否存在
	agent, err := s.repo.getAgent(ctx, userId, agentId)
	if err != nil {
		logs.Errorf("addAgentKnowledgeBase 获取agent失败: %v", err)
		return nil, errs.DBError
	}
	if agent == nil {
		return nil, biz.AgentNotFound
	}
	//先检查知识库是否存在
	kb, err := s.getKnowledgeBase(ctx, userId, addReq.KnowledgeBaseID)
	if err != nil {
		logs.Errorf("addAgentKnowledgeBase 获取知识库失败: %v", err)
		return nil, errs.DBError
	}
	if kb == nil {
		return nil, biz.ErrKnowledgeBaseNotFound
	}
	//查询关联关系是否存在
	exist, err := s.repo.isAgentKnowledgeBaseExist(ctx, agentId, addReq.KnowledgeBaseID)
	if err != nil {
		logs.Errorf("addAgentKnowledgeBase 查询关联关系是否存在失败: %v", err)
		return nil, errs.DBError
	}
	//如果存在 就不需要再次添加了
	if exist {
		return nil, nil
	}
	err = s.repo.createAgentKnowledgeBase(ctx, &model.AgentKnowledgeBase{
		AgentID:         agentId,
		KnowledgeBaseId: addReq.KnowledgeBaseID,
		Status:          model.AgentKnowledgeStatusEnabled,
	})
	if err != nil {
		logs.Errorf("addAgentKnowledgeBase 创建关联关系失败: %v", err)
		return nil, errs.DBError
	}
	return nil, nil
}

func (s *service) getKnowledgeBase(ctx context.Context, userId uuid.UUID, kbId uuid.UUID) (*model.KnowledgeBase, error) {
	trigger, err := event.Trigger("getKnowledgeBase", &shared.GetKnowledgeBaseRequest{
		UserId:          userId,
		KnowledgeBaseId: kbId,
	})
	return trigger.(*model.KnowledgeBase), err
}

func (s *service) deleteAgentKnowledgeBase(ctx context.Context, userID uuid.UUID, agentId uuid.UUID, kbId uuid.UUID) (any, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()
	err := s.repo.deleteAgentKnowledgeBase(ctx, agentId, kbId)
	if err != nil {
		logs.Errorf("deleteAgentKnowledgeBase 删除关联关系失败: %v", err)
		return nil, errs.DBError
	}
	return nil, nil
}

// buildRagContext 拿着用户消息去检索绑定的知识库，拼接成 RAG 上下文，并异步推送到前端展示
// 入参：
//   - message: 用户当前输入的文本（问题）
//   - agent: 包含了该智能体所关联的知识库列表（KnowledgeBases）的元数据
//
// 出参：
//   - string: 最终拼接好、供大模型参考的上下文文本
func (s *service) buildRagContext(ctx context.Context, dataChan chan string, message string, agent *model.Agent) string {
	var ragContext string

	// 1. 安全校验：只有当当前智能体真的绑定了至少一个知识库时，才执行检索逻辑
	if len(agent.KnowledgeBases) > 0 {

		// 用于汇总所有关联知识库检索出来的结果切片
		var allResult []*shared.SearchKnowledgeBaseResult

		// 2. 循环遍历该智能体绑定的每一个知识库
		for _, v := range agent.KnowledgeBases {
			// 调用内部方法，传入创建者ID、用户消息、知识库ID，去向量数据库里做相似度检索（Embedding + Vector Search）
			results, err := s.searchKnowledgeBase(ctx, agent.CreatorID, message, v.ID)
			if err != nil {
				// 如果某一个知识库检索失败，打印日志，但 continue 略过，不影响其他知识库的检索
				logs.Errorf("searchKnowledgeBase 搜索知识库失败: %v", err)
				continue
			}
			// 将当前知识库检索到的切片追加到总结果集中
			allResult = append(allResult, results...)
		}

		// 3. 如果从所有知识库里真的捞到了相关干货
		if len(allResult) > 0 {
			// 采用 strings.Builder 高效拼接字符串（比使用 + 号性能好很多，减少内存分配）
			var contextBuilder strings.Builder
			contextBuilder.WriteString("【 参考以下知识库内容回答问题 】\n")

			// 4. 遍历所有检索到的文本片段
			for i, v := range allResult {
				// 【截断策略】：为了防止知识库内容太长导致大模型上下文爆掉（或产生过高 Token 费用），
				// 这里实行硬编码截断，只取相关度最高的前 3 条结果。这个数字可以根据实际业务调大或调小。
				if i >= 3 {
					break
				}
				// 格式化拼装成：“1. [知识库文本内容] \n”
				contextBuilder.WriteString(fmt.Sprintf("%d.  %s \n", i+1, v.Content))
			}

			// 5. 将拼装好的最终文本赋值给返回值变量
			ragContext = contextBuilder.String()

			// 6. 联动前端展示：为了让用户有更好的体验（看到 AI 正在检索和使用了哪些知识），
			//    这里把所有知识库的名字用制表符（\t）拼接起来，作为“工具名称”返回给前端
			var names strings.Builder
			for _, v := range agent.KnowledgeBases {
				names.WriteString(v.Name + "\t")
			}

			// 7. 将检索到的参考资料封装成特定的 AI 消息协议格式
			buildMessage := ai.BuildMessage(agent.Name, names.String(), ragContext)

			// 8. 通过数据通道直接塞进去！因为这个函数是在上层 agentMessage 的异步协程里调用的，
			//    所以这里塞入通道，前端通过 SSE 流就能立刻实时收到并渲染出“知识库引用”的效果。
			dataChan <- buildMessage
		}
	}

	// 9. 返回给上层函数。上层会把这个返回值填入 System Prompt 的 {ragContext} 变量中送给大模型。
	return ragContext
}

func (s *service) searchKnowledgeBase(ctx context.Context, userId uuid.UUID, message string, id uuid.UUID) ([]*shared.SearchKnowledgeBaseResult, error) {
	trigger, err := event.Trigger("searchKnowledgeBase", &shared.SearchKnowledgeBaseRequest{
		UserId:          userId,
		KnowledgeBaseId: id,
		Query:           message,
	})
	if err != nil {
		logs.Errorf("searchKnowledgeBase 搜索知识库失败: %v", err)
		return nil, err
	}
	response := trigger.(*shared.SearchKnowledgeBaseResponse)
	return response.Results, nil
}

func newService() *service {
	return &service{
		repo: newModels(database.GetPostgresDB().GormDB),
	}
}

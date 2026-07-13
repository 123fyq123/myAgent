package ai

import "encoding/json"

// AgentMessage 统一定义返回给客户端的标准 JSON 消息协议结构体。
// 无论大模型是正在思考、正在调用工具、正常回答还是报错，都必须统一转换为这个格式。
type AgentMessage struct {
	// Action 客户端（前端）的路由行为指令。
	// 作用：前端会有一个 switch(msg.action) 逻辑，根据不同的指令触发对应的 UI 更新（如这里的 "agent_answer"）。
	Action string `json:"action"`

	// AgentName 标明当前正在说话/执行任务的智能体名称（在多智能体协作或 Supervisor 模式下极度重要）。
	AgentName string `json:"agentName"`

	// ToolName 标明当前智能体是否正在使用某种外部工具（如 RAG 知识库名称、画图、代码执行器等）。
	// 前端如果看到这个字段不为空，可以渲染一个“正在调用 [ToolName]...”的特殊组件卡片。
	ToolName string `json:"toolName"`

	// IsErr 错误标记。如果为 true，代表这是一个异常通知，Content 里装的是报错信息。
	IsErr bool `json:"isErr"`

	// Content 核心对话内容。大模型流式吐出的最终文本（或者是错误消息的文本描述）。
	Content string `json:"content"`

	// ReasoningContent 推理/思维链内容。专门适配 DeepSeek-R1 这类推理大模型吐出来的 <think> 标签内容。
	// 前端看到它有值，可以在界面上渲染一个“折叠的思考过程”并展示打字机动画。
	ReasoningContent string `json:"reasoningContent"`
}

// BuildErrMessage 快捷构建一个错误类型的 JSON 消息字符串。
func BuildErrMessage(agentName string, errMsg string) string {
	msg := AgentMessage{
		Action:    "agent_answer", // 保持统一的动作指令
		AgentName: agentName,
		IsErr:     true,   // 标志为错误消息
		Content:   errMsg, // 将错误描述塞入普通内容区供前端打印
	}
	// 将结构体序列化为 JSON 字节切片。
	// 💡 提示：因为结构体是纯内部控制生成的，字段合法，所以这里忽略了 json.Marshal 的第二个返回参数 err
	bytes, _ := json.Marshal(msg)

	// 转换为字符串返回，方便直接写入 dataChan
	return string(bytes)
}

// BuildReasoningMessage 快捷构建一个大模型“正在深度思考/推理”阶段的 JSON 消息字符串。
// 专门用于实时推送 AI 的思维链给前端，保持打字机的思考动效。
func BuildReasoningMessage(name string, toolName string, reasoningContent string) string {
	msg := AgentMessage{
		Action:           "agent_answer",
		AgentName:        name,
		ToolName:         toolName,
		ReasoningContent: reasoningContent, // 仅填充推理字段，Content 留空
	}
	bytes, _ := json.Marshal(msg)
	return string(bytes)
}

// BuildMessage 快捷构建一个大模型“正常回答/吐字”或者“展示工具结果”阶段的 JSON 消息字符串。
func BuildMessage(name string, toolName string, content string) string {
	msg := AgentMessage{
		Action:    "agent_answer",
		AgentName: name,
		ToolName:  toolName,
		Content:   content, // 填充最终输出的内容，ReasoningContent 留空
	}
	bytes, _ := json.Marshal(msg)
	return string(bytes)
}

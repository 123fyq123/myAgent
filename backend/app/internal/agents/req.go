package agents

import (
	"model"

	"github.com/google/uuid"
)

// 创建 Agent（名称、描述、状态）
type CreateAgentReq struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Status      model.AgentStatus `json:"status"`
}

// 列表查询（支持名称/状态过滤、分页）
type SearchAgentReq struct {
	Params struct {
		Name     string            `json:"name"`
		Status   model.AgentStatus `json:"status"`
		Page     int               `json:"page"`
		PageSize int               `json:"pageSize"`
	} `json:"params"`
}

// 更新 Agent（含系统提示词、模型配置等）
type UpdateAgentReq struct {
	ID              uuid.UUID         `json:"id"`
	Name            string            `json:"name"`
	Description     string            `json:"description"`
	Status          model.AgentStatus `json:"status"`
	SystemPrompt    string            `json:"systemPrompt"`
	ModelProvider   string            `json:"modelProvider"`
	ModelName       string            `json:"modelName"`
	ModelParameters model.JSON        `json:"modelParameters"`
	OpeningDialogue string            `json:"openingDialogue"`
}

// 发送消息（AgentID、消息内容、会话ID）
type AgentMessageReq struct {
	AgentID   uuid.UUID `json:"agentId"`
	Message   string    `json:"message"`
	SessionId uuid.UUID `json:"sessionId,omitempty"`
}

// 更新 Agent 关联的工具列表
type UpdateAgentToolReq struct {
	Tools []ToolItem `json:"tools"`
}


type ToolItem struct {
	ID   uuid.UUID `json:"id"`
	Type string    `json:"type"`
}

// 为 Agent 添加知识库
type addAgentKnowledgeBaseReq struct {
	KnowledgeBaseID uuid.UUID `json:"kb_id"`
}

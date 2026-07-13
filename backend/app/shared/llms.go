package shared

import (
	"model"

	"github.com/google/uuid"
)

// GetProviderConfigsRequest 用于请求特定大模型厂商（Provider）的身份/链接配置信息
type GetProviderConfigsRequest struct {
	// LLMType 区分大模型的基本类型。例如：Chat（对话文本模型）、Embedding（向量模型）
	LLMType model.LLMType
	// Provider 大模型厂商标识。例如："openai"、"ollama"、"qwen"
	Provider string
	// ModelName 具体的模型代号。例如："gpt-4o"、"deepseek-v3"
	ModelName string
}

// LLMParams 一个通用的“大模型定位参数”结构体
type LLMParams struct {
	// Provider 大模型厂商名字（如 "openai"）
	Provider string
	// Model 具体的模型名字（如 "text-embedding-3-small"）
	Model string
	// ModelType 模型的类型（对话、向量、重排等）
	ModelType model.LLMType
	// UserId 绑定或正在使用该模型的用户 ID（用于计费、日志审计或用户自定义 API Key 的隔离）
	UserId uuid.UUID
}

// EmbeddingConfigResponse 获取文本向量模型（Embedding Model）配置后，返回的响应数据
// RAG（检索增强生成）系统在把知识库文档拆分并存储到向量数据库之前，或者在把用户的提问转成向量时，
// 必须通过这个结构体拿到具体的 Embedding 模型实例、维度等关键元数据。
type EmbeddingConfigResponse struct {
	// 向量模型
	Model *model.LLM
}

package shared

import "github.com/google/uuid"

// GetKnowledgeBaseRequest 用于向知识库服务请求“单个知识库的元数据/详情”时传入的入参
type GetKnowledgeBaseRequest struct {
	UserId          uuid.UUID `json:"userId"`
	KnowledgeBaseId uuid.UUID `json:"knowledgeBaseId"`
}

// SearchKnowledgeBaseRequest 用于在 RAG 流程中，拿着用户的问题去指定知识库“检索相关文本片段”时传入的入参
type SearchKnowledgeBaseRequest struct {
	UserId          uuid.UUID `json:"userId"`
	KnowledgeBaseId uuid.UUID `json:"knowledgeBaseId"`
	Query           string    `json:"query"`
}

// SearchKnowledgeBaseResponse 知识库检索完成后，返回给请求方的外层包装结果
type SearchKnowledgeBaseResponse struct {
	Results []*SearchKnowledgeBaseResult `json:"results"`
}

// SearchKnowledgeBaseResult 每一个具体被捞出来的“知识点/文本片段”的数据模型
type SearchKnowledgeBaseResult struct {
	Content string `json:"content"`
}

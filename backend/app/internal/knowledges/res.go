package knowledges

import (
	"model"

	"github.com/google/uuid"
)

// ListResp 是知识库分页列表的响应载体。
type ListResp struct {
	KnowledgeBases []*model.KnowledgeBase `json:"knowledgeBases"`
	Total          int64                  `json:"total"`
}

// KnowledgeBaseResponse 是知识库详情 DTO，额外携带实时统计出的文档数量与总大小。
type KnowledgeBaseResponse struct {
	Id                     uuid.UUID         `json:"id"`
	Name                   string            `json:"name"`
	Tags                   []string          `json:"tags"`
	Description            string            `json:"description"`
	EmbeddingModelName     string            `json:"embeddingModelName"`
	EmbeddingModelProvider string            `json:"embeddingModelProvider"`
	ChatModelName          string            `json:"chatModelName"`
	ChatModelProvider      string            `json:"chatModelProvider"`
	StorageType            model.StorageType `json:"storageType"`
	StorageConfig          model.JSON        `json:"storageConfig"`
	DocumentCount          int               `json:"documentCount"`
	TotalSize              int64             `json:"totalSize"`
	CreatedAt              int64             `json:"createdAt"`
	UpdatedAt              int64             `json:"updatedAt"`
	CreatorId              uuid.UUID         `json:"creatorId"`
}

// ListDocumentsResp 是知识库内文档分页列表的响应载体。
type ListDocumentsResp struct {
	Documents []*model.Document `json:"items"`
	Total     int64             `json:"total"`
}

// SearchResponse 描述一次向量检索的命中结果与耗时。
type SearchResponse struct {
	Query   string          `json:"query"`
	Results []*SearchResult `json:"results"`
	Total   int64           `json:"total"`
	Took    int64           `json:"took"` //耗时
	KbId    uuid.UUID       `json:"kbId"`
}

// SearchResult 定义了知识库检索出的单个文档分段结果（常作为大模型 RAG 的上下文参考）
type SearchResult struct {
	// Id 分段的唯一标识（通常对应父分段的 Chunk ID）
	Id uuid.UUID `json:"id"`

	// DocumentId 该分段所属的原始文档唯一 ID
	DocumentId uuid.UUID `json:"documentId"`

	// Content 该分段的文本内容，会被送入大模型作为提示词（Prompt）的一部分
	Content string `json:"content"`

	// Score 向量匹配相似度分数（分数越高代表该分段与用户提问的相关性越强）
	Score float64 `json:"score"`

	// Metadata 存储分段扩展信息的 JSON 对象（如：章节号 chapter_num、卷号 volume_num 等元数据）
	Metadata model.JSON `json:"metadata"`

	// Position 在本次召回结果列表中的排序位置（从 0 开始，0 代表最匹配）
	Position int `json:"position"`

	// Document 该分段所属的完整文档模型指针（包含文件名、文件类型、大小等详细属性，方便前端展示出处）
	Document *model.Document `json:"document"`
}

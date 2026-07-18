package knowledges

// createKnowledgeBaseReq 是创建知识库的请求体。
// Chat 模型用于查询意图分析，Embedding 模型用于文档向量化和后续召回。
type createKnowledgeBaseReq struct {
	Name                   string   `json:"name"`
	Description            string   `json:"description"`
	EmbeddingModelName     string   `json:"embeddingModelName"`
	EmbeddingModelProvider string   `json:"embeddingModelProvider"`
	ChatModelName          string   `json:"chatModelName"`
	ChatModelProvider      string   `json:"chatModelProvider"`
	Tags                   []string `json:"tags"`
}

// updateKnowledgeBaseReq 是知识库元数据的局部更新请求；空字符串字段由 Service 保持原值。
type updateKnowledgeBaseReq struct {
	Name                   string   `json:"name"`
	Description            string   `json:"description"`
	EmbeddingModelName     string   `json:"embeddingModelName"`
	EmbeddingModelProvider string   `json:"embeddingModelProvider"`
	ChatModelName          string   `json:"chatModelName"`
	ChatModelProvider      string   `json:"chatModelProvider"`
	Tags                   []string `json:"tags"`
}

// listReq 描述知识库列表的分页与名称模糊搜索条件。
type listReq struct {
	Page     int    `json:"page"`
	PageSize int    `json:"size"`
	Search   string `json:"search"`
}

// searchReq 包装列表查询参数，匹配前端当前发送的 JSON 结构。
type searchReq struct {
	Params listReq `json:"params"`
}

// searchParams 是一次 RAG 检索的原始问题文本。
type searchParams struct {
	Query string `json:"query"`
}

// listDocumentReq 描述某知识库下文档列表的分页、筛选和排序参数。
type listDocumentReq struct {
	Page      int    `json:"page" form:"page"`
	PageSize  int    `json:"pageSize" form:"pageSize"`
	Search    string `json:"search" form:"search"`
	SortBy    string `json:"sortBy" form:"sortBy"`
	Status    string `json:"status" form:"status"`
	SortOrder string `json:"sortOrder" form:"sortOrder"`
}

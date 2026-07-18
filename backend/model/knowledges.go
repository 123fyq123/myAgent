package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
)

type StringArrayJSON []string

// Value  写入 PG 时调用
func (s StringArrayJSON) Value() (driver.Value, error) {
	if len(s) == 0 {
		return "[]", nil
	}
	return json.Marshal(s)
}

// Scan 从 PG 读取时调用
func (s *StringArrayJSON) Scan(value interface{}) error {
	if value == nil {
		*s = StringArrayJSON{}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("failed to scan StringArrayJSON")
	}

	return json.Unmarshal(bytes, s)
}

type StorageType string

const (
	StorageTypeElasticSearch StorageType = "es"
	StorageTypeMilvus        StorageType = "milvus"
)

// KnowledgeBase 知识库实体模型（对应数据库中的 knowledge_bases 表）
// 该表是 RAG（检索增强生成）系统的核心，定义了知识库的基础元数据、绑定的 AI 模型以及向量存储配置
type KnowledgeBase struct {
	// BaseModel 嵌套基础模型，通常包含 ID、CreatedAt、UpdatedAt、DeletedAt（软删除）等公共字段
	BaseModel

	// CreatorID 创建该知识库的用户唯一 ID，用于多租户数据隔离
	CreatorID uuid.UUID `json:"creatorId" gorm:"column:creator_id;type:uuid;not null;index"`

	// Name 知识库名称（如：凡人修仙传小说集），建立了数据库索引以加速列表检索和前缀查询
	Name string `json:"name" gorm:"column:name;type:varchar(255);not null;index"`

	// Description 知识库的详细描述、备注或使用说明
	Description string `json:"description" gorm:"column:description;type:text"`

	// ChatModelName 关联的对话大模型标识（如：gpt-4o, qwen-max），用于对该知识库进行意图解析或 RAG 问答
	ChatModelName string `json:"chatModelName" gorm:"column:chat_model_name;type:varchar(255)"`

	// ChatModelProvider 对话模型的供应商/厂商标识（如：openai, ollama, dashscope）
	ChatModelProvider string `json:"chatModelProvider" gorm:"column:chat_model_provider;type:varchar(50)"`

	// EmbeddingModelName 绑定的向量化/嵌入模型名称（如：text-embedding-3-small）
	// 提示：文档上传切片和后续提问搜索时，必须使用同一个 Embedding 模型，否则向量空间不一致导致检索失效
	EmbeddingModelName string `json:"embeddingModelName" gorm:"column:embedding_model_name;type:varchar(255)"`

	// EmbeddingModelProvider 向量化模型的供应商/厂商标识（如：openai, bge）
	EmbeddingModelProvider string `json:"embeddingModelProvider" gorm:"column:embedding_model_provider;type:varchar(50)"`

	// EmbeddingDimension 向量特征维度（如：1536 维或 768 维），在 Milvus/ES 创建索引集合时需要该强约束参数
	EmbeddingDimension int `json:"embeddingDimension" gorm:"column:embedding_dimension;type:integer;not null"`

	// StorageType 向量数据存储引擎类型（自定义枚举，如：es, milvus, pgvector），默认值为 es
	StorageType StorageType `json:"storageType" gorm:"column:storage_type;type:varchar(50);not null;default:'es'"`

	// StorageConfig 向量存储的专属个性化配置（使用 PostgreSQL 的 JSONB 格式存储，如特殊的 Collection 参数等）
	StorageConfig JSON `json:"storageConfig" gorm:"column:storage_config;type:jsonb"`

	// DocumentCount 缓存该知识库下关联的文档总数量，默认值为 0
	DocumentCount uint `json:"documentCount" gorm:"column:document_count;type:integer;not null;default:0"`

	// Tags 知识库的标签列表（使用 JSONB 存储字符串数组，如：["小说", "热血"]），便于分类筛选
	Tags StringArrayJSON `json:"tags" gorm:"column:tags;type:jsonb"`

	// Status 知识库的启用状态（自定义枚举，如：active 正常，inactive 禁用），默认状态为 active
	Status KnowledgeBaseStatus `json:"status" gorm:"column:status;type:varchar(20);not null;default:'active'"`

	// 关联关系
	// Agents 与 Agent（智能体）的多对多（M2M）关联
	// 通过第三张中间表 `agent_knowledge_bases` 维护，一个智能体可以挂载多个知识库，一个知识库也可以被多个智能体共享
	Agents []Agent `json:"agents" gorm:"many2many:agent_knowledge_bases;"`
}

type KnowledgeBaseStatus string

const (
	KnowledgeBaseStatusActive   KnowledgeBaseStatus = "active"
	KnowledgeBaseStatusDisabled KnowledgeBaseStatus = "disabled"
)

// TableName 返回表名
func (*KnowledgeBase) TableName() string {
	return "knowledge_bases"
}

// Document 存储原始文档的元数据
type Document struct {
	BaseModel
	// 1. 归属信息
	KnowledgeBaseID uuid.UUID `json:"knowledgeBaseId" gorm:"column:kb_id;type:uuid;not null;index"` // 归属哪个知识库
	CreatorID       uuid.UUID `gorm:"type:uuid;not null;index"`
	// 2. 文件基本信息
	Name       string `json:"name" gorm:"column:name;type:varchar(255);not null"`          // 文件名: "员工手册.pdf"
	FileType   string `json:"fileType" gorm:"column:file_type;type:varchar(50);not null"`  // 后缀: pdf, docx, md
	Size       int64  `json:"size" gorm:"column:size;type:bigint;not null;default:0"`      // 文件大小(字节)
	TokenCount int    `json:"tokenCount" gorm:"column:token_count;type:integer;default:0"` // 总 Token 数消耗统计
	// 3. 存储与去重
	StorageKey string `json:"storageKey" gorm:"column:storage_key;type:varchar(512);not null"` // S3/OSS 上的路径 key
	FileHash   string `json:"fileHash" gorm:"column:file_hash;type:varchar(64);index"`         // SHA256 Hash，用于防止重复上传
	// 4. 处理状态
	Status       DocumentStatus `json:"status" gorm:"column:status;type:varchar(20);not null;default:'pending';index"`
	ErrorMessage string         `json:"errorMessage" gorm:"column:error_message;type:text"` // 如果失败，存错误堆栈
	// 5. 解析结果元数据 (可选)
	// 存放如: {"page_count": 10, "author": "CEO"}
	MetaInfo JSON `json:"metaInfo" gorm:"column:meta_info;type:jsonb"`
	// 6. 是否启用
	Enabled bool `json:"enabled" gorm:"column:enabled;type:boolean;not null;default:true"` // 软开关，关闭后检索不到
	// 关联
	Chunks []DocumentChunk `json:"chunks,omitempty" gorm:"foreignKey:DocumentID"`
}
type DocumentStatus string

const (
	DocumentStatusPending    DocumentStatus = "pending"
	DocumentStatusProcessing DocumentStatus = "processing"
	DocumentStatusCompleted  DocumentStatus = "completed"
	DocumentStatusFailed     DocumentStatus = "failed"
)

func (*Document) TableName() string {
	return "documents"
}

// DocumentChunk 存储切分后的片段 (PostgreSQL 侧备份与管理)
type DocumentChunk struct {
	BaseModel
	// 1. 关联关系
	DocumentID      uuid.UUID `json:"documentId" gorm:"column:document_id;type:uuid;not null;index"`
	KnowledgeBaseID uuid.UUID `json:"knowledgeBaseId" gorm:"column:kb_id;type:uuid;not null;index"` // 冗余字段，为了方便按库查询
	// 2. 索引同步 (关键字段)
	// 记录该切片在 ES 中的 ID (_id)，用于后续的更新或删除操作
	ElasticSearchID string `json:"esId" gorm:"column:es_id;type:varchar(100);index"`
	// 3. 内容数据
	ChunkIndex int    `json:"chunkIndex" gorm:"column:chunk_index;type:integer;not null"` // 切片在原文中的顺序 (0, 1, 2...)
	Content    string `json:"content" gorm:"column:content;type:text;not null"`           // 切片文本内容 (PG中存一份用于展示/编辑)
	// 4. 向量化相关
	TokenCount int `json:"tokenCount" gorm:"column:token_count;type:integer"` // 该切片的 Token 数
	// 5. 元数据 (Metadata)
	// 极其重要！这里存放 {"page_num": 1, "heading": "第一章", "image_url": "..."}
	// 这些数据会同步写入 ES 的 metadata 字段，用于 filter
	MetaInfo JSON        `json:"metaInfo" gorm:"column:meta_info;type:jsonb"`
	Status   ChunkStatus `gorm:"column:status;type:varchar(20);not null;default:'pending'"`
}
type ChunkStatus string

const (
	ChunkStatusPending  ChunkStatus = "pending"
	ChunkStatusEmbedded ChunkStatus = "embedded"
	ChunkStatusDeleted  ChunkStatus = "deleted"
	ChunkStatusDisabled ChunkStatus = "disabled"
)

func (*DocumentChunk) TableName() string {
	return "document_chunks"
}

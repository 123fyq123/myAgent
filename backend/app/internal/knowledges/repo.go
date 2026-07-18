package knowledges

import (
	"context"
	"model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// repository 定义了知识库模块与底层关系型数据库（如 PostgreSQL/MySQL）交互的仓储层接口
// 它隔离了具体的 ORM 框架（如 GORM），只暴露纯粹的业务数据操作方法
type repository interface {

	// ==========================================
	// 知识库 (KnowledgeBase) 相关操作
	// ==========================================

	// createKnowledgeBase 向数据库中持久化创建一条新的知识库记录
	createKnowledgeBase(ctx context.Context, m *model.KnowledgeBase) error

	// listKnowledgeBases 根据过滤条件（如分页、关键词等）以及用户 ID，获取当前用户的知识库列表并返回总数
	listKnowledgeBases(ctx context.Context, userId uuid.UUID, filter KnowledgeBaseFilter) ([]*model.KnowledgeBase, int64, error)

	// getKnowledgeBase 精确查询单个知识库详情（包含多租户鉴权校验，防止越权查看他人知识库）
	getKnowledgeBase(ctx context.Context, userId uuid.UUID, id uuid.UUID) (*model.KnowledgeBase, error)

	// countKnowledgeBaseDocuments 统计指定知识库下的核心资产指标
	// 第一个返回值通常为文档总数(document_count)，第二个返回值通常为总字节数或总 Token 数
	countKnowledgeBaseDocuments(ctx context.Context, id uuid.UUID) (int64, int64, error)

	// updateKnowledgeBase 更新已存在的知识库元数据配置（如修改名称、描述或绑定的 AI 模型）
	updateKnowledgeBase(ctx context.Context, kb *model.KnowledgeBase) error

	// deleteKnowledgeBase 物理或软删除某个指定的知识库
	deleteKnowledgeBase(ctx context.Context, id uuid.UUID) error

	// ==========================================
	// 文档 (Document) 相关操作
	// ==========================================

	// listDocuments 分页且按条件获取某个特定知识库下的所有上传文档列表（支持状态、文件名等条件筛选）
	listDocuments(ctx context.Context, userId uuid.UUID, kbId uuid.UUID, filter DocumentFilter) ([]*model.Document, int64, error)

	// createDocument 登记并创建一条新的上传文档元数据记录（此时文档通常处于“正在解析/处理”状态）
	createDocument(ctx context.Context, doc *model.Document) error

	// updateDocumentStatus 动态更新文档的生命周期状态（例如从 active 变更为 processing、success 或 failed）
	updateDocumentStatus(ctx context.Context, id uuid.UUID, status model.DocumentStatus) error

	// getDocument 获取某个特定知识库下指定文档的详细元数据信息
	getDocument(ctx context.Context, userId uuid.UUID, kbId uuid.UUID, documentId uuid.UUID) (*model.Document, error)

	// ==========================================
	// 文档分段切片 (DocumentChunk) 相关操作
	// ==========================================

	// createDocumentChunks 批量插入文档切片记录（通常在文档通过本地或大模型解析切片完成后批量落库）
	createDocumentChunks(ctx context.Context, chunks []*model.DocumentChunk) error

	// getDocumentChunksByIds 核心高级检索依赖项：根据父分段 ID 集合批量抓取完整的文本块内容（用于大模型 RAG 上下文组装）
	getDocumentChunksByIds(ctx context.Context, ids []string) ([]*model.DocumentChunk, error)

	// ==========================================
	// 事务 (Transaction) 与级联删除操作
	// ==========================================

	// transaction 提供标准的 GORM 事务包裹器，确保闭包内的所有数据库写操作（如级联删除）保持 ACID 原子性
	transaction(ctx context.Context, f func(tx *gorm.DB) error) error

	// deleteDocuments 级联删除指定文档记录（接收事务对象 tx，确保与分段清理处于同一事务中）
	deleteDocuments(ctx context.Context, tx *gorm.DB, userId uuid.UUID, kbId uuid.UUID, documentId uuid.UUID) error

	// deleteDocumentChunks 级联删除指定文档在关系型数据库中的所有切片内容（防止产生无头脏数据）
	deleteDocumentChunks(ctx context.Context, tx *gorm.DB, kbId uuid.UUID, documentId uuid.UUID) error
}

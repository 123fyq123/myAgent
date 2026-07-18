package knowledges

import (
	"context"
	"model"

	"github.com/google/uuid"
	"github.com/mszlu521/thunder/gorms"
	"gorm.io/gorm"
)

// models 是知识库仓储层的 GORM 实现，集中封装 PostgreSQL 的读写操作。
type models struct {
	db *gorm.DB
}

// getDocumentChunksByIds 批量读取父分片，并按 ids 的召回顺序重新排序。
// SQL 的 IN 查询不保证返回顺序，因此这里必须手动恢复向量检索的相关性顺序。
func (m *models) getDocumentChunksByIds(ctx context.Context, ids []string) ([]*model.DocumentChunk, error) {
	var documentChunks []*model.DocumentChunk
	err := m.db.WithContext(ctx).Where("id in ?", ids).Find(&documentChunks).Error
	if err != nil {
		return nil, err
	}
	//这里我们需要手动排序 保证查询结果和ids的顺序一致
	chunkMap := make(map[string]*model.DocumentChunk)
	for _, chunk := range documentChunks {
		chunkMap[chunk.ID.String()] = chunk
	}
	orderChunks := make([]*model.DocumentChunk, 0, len(ids))
	for _, id := range ids {
		if chunk, ok := chunkMap[id]; ok {
			orderChunks = append(orderChunks, chunk)
		}
	}
	return orderChunks, nil
}

// deleteDocuments 在给定事务中物理删除文档，并同时以用户和知识库 ID 限制删除范围。
func (m *models) deleteDocuments(ctx context.Context, tx *gorm.DB, userId uuid.UUID, kbId uuid.UUID, documentId uuid.UUID) error {
	if tx == nil {
		tx = m.db
	}
	return tx.WithContext(ctx).Where("id = ? and creator_id=? and kb_id = ?", documentId, userId, kbId).Unscoped().Delete(&model.Document{}).Error
}

// deleteDocumentChunks 删除文档在 PostgreSQL 中保存的全部父分片记录。
func (m *models) deleteDocumentChunks(ctx context.Context, tx *gorm.DB, kbId uuid.UUID, documentId uuid.UUID) error {
	if tx == nil {
		tx = m.db
	}
	//如果不想要通过deleted_at进行软删除，可以加上Unscoped
	return tx.WithContext(ctx).Where("document_id = ? and kb_id = ?", documentId, kbId).Unscoped().Delete(&model.DocumentChunk{}).Error
}

// getDocument 按用户、知识库和文档三重条件读取文档，避免跨租户访问。
func (m *models) getDocument(ctx context.Context, userId uuid.UUID, kbId uuid.UUID, documentId uuid.UUID) (*model.Document, error) {
	var doc model.Document
	err := m.db.WithContext(ctx).Where("id = ? and creator_id=? and kb_id = ?", documentId, userId, kbId).First(&doc).Error
	if gorms.IsRecordNotFoundError(err) {
		return nil, nil
	}
	return &doc, err
}

// transaction 为多表删除等操作提供原子性：闭包返回错误时自动回滚。
func (m *models) transaction(ctx context.Context, f func(tx *gorm.DB) error) error {
	return m.db.WithContext(ctx).Transaction(f)
}

// createDocumentChunks 批量持久化父分片，供检索结果回填完整上下文使用。
func (m *models) createDocumentChunks(ctx context.Context, chunks []*model.DocumentChunk) error {
	return m.db.WithContext(ctx).CreateInBatches(chunks, len(chunks)).Error
}

// createDocument 先登记文档元数据和处理状态，再由异步任务完成解析与索引。
func (m *models) createDocument(ctx context.Context, doc *model.Document) error {
	return m.db.WithContext(ctx).Create(doc).Error
}

// updateDocumentStatus 更新异步文档处理的生命周期状态。
func (m *models) updateDocumentStatus(ctx context.Context, id uuid.UUID, status model.DocumentStatus) error {
	return m.db.WithContext(ctx).Model(&model.Document{}).Where("id = ?", id).Update("status", status).Error
}

// listDocuments 按知识库和创建者分页查询文档，并返回总数供前端分页使用。
func (m *models) listDocuments(ctx context.Context, userId uuid.UUID, kbId uuid.UUID, filter DocumentFilter) ([]*model.Document, int64, error) {
	var documents []*model.Document
	var count int64
	query := m.db.WithContext(ctx).Model(&model.Document{})
	if filter.Search != "" {
		query = query.Where("name LIKE ?", "%"+filter.Search+"%")
	}
	query = query.Where("kb_id = ? and creator_id = ?", kbId, userId)
	query = query.Count(&count)
	query = query.Limit(filter.Limit).Offset(filter.Offset)
	return documents, count, query.Find(&documents).Error
}

// DocumentFilter 是仓储层使用的文档分页和名称筛选条件。
type DocumentFilter struct {
	Limit  int
	Offset int
	Search string
	Status string
}

// deleteKnowledgeBase 删除知识库元数据；关联文档与向量索引的清理由 Service 协调。
func (m *models) deleteKnowledgeBase(ctx context.Context, id uuid.UUID) error {
	return m.db.WithContext(ctx).Delete(&model.KnowledgeBase{}, id).Error
}

// updateKnowledgeBase 持久化知识库的可编辑元数据。
func (m *models) updateKnowledgeBase(ctx context.Context, kb *model.KnowledgeBase) error {
	return m.db.WithContext(ctx).Updates(kb).Error
}

// getKnowledgeBase 读取一个知识库；调用方负责将 nil 结果转换为业务“未找到”错误。
func (m *models) getKnowledgeBase(ctx context.Context, userId uuid.UUID, id uuid.UUID) (*model.KnowledgeBase, error) {
	var kb model.KnowledgeBase
	err := m.db.WithContext(ctx).Where("id = ?", id).First(&kb).Error
	if gorms.IsRecordNotFoundError(err) {
		return nil, nil
	}
	return &kb, err
}

// countKnowledgeBaseDocuments 仓储层方法：动态统计指定知识库下关联文档的【总大小】和【文档总数】
func (m *models) countKnowledgeBaseDocuments(ctx context.Context, id uuid.UUID) (int64, int64, error) {
	var docCount int64
	var totalSize int64

	// 1. 统计当前知识库下的文档总数
	// 执行 SQL: SELECT COUNT(*) FROM documents WHERE kb_id = 'xxx'
	err := m.db.WithContext(ctx).
		Model(&model.Document{}).
		Where("kb_id = ?", id).
		Count(&docCount).Error
	if err != nil {
		return 0, 0, err
	}

	// 2. 统计当前知识库下所有文档的总体积 (单位: 字节)
	// 执行 SQL: SELECT COALESCE(sum(size), 0) FROM documents WHERE kb_id = 'xxx'
	// 提示: COALESCE 的作用是当 sum(size) 结果为 NULL (即无文档) 时，安全地将其转化为 0
	err = m.db.WithContext(ctx).
		Model(&model.Document{}).
		Where("kb_id = ?", id).
		Select("COALESCE(sum(size), 0)").
		Scan(&totalSize).Error

	// 3. 【核心修正】调整返回值顺序为 (totalSize, docCount, err)，完美契合 Service 层的接收顺序
	return totalSize, docCount, err
}

// listKnowledgeBases 仓储层方法：从数据库中查询指定用户的知识库列表及总数（支持分页与模糊搜索）
func (m *models) listKnowledgeBases(ctx context.Context, userId uuid.UUID, filter KnowledgeBaseFilter) ([]*model.KnowledgeBase, int64, error) {
	var kbs []*model.KnowledgeBase
	var count int64

	// 1. 初始化查询基础对象，绑定上下文并指定目标模型（knowledge_bases 表）
	query := m.db.WithContext(ctx).Model(&model.KnowledgeBase{})

	// 2. 动态拼接过滤条件：如果传入了搜索关键词，则按名称进行模糊匹配
	if filter.Search != "" {
		query = query.Where("name LIKE ?", "%"+filter.Search+"%")
	}

	// 3. 权限与隔离：只查询当前登录用户创建的知识库
	query = query.Where("creator_id = ?", userId)

	// 4. 【核心修复】先统计符合上述条件的总记录数（必须在 Limit/Offset 之前执行）
	// 注意：不要用 query = query.Count(&count) 覆盖原 query，避免后续查询被污染
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	// 5. 注入分页参数（限制返回条数与偏移量），并执行最终的记录查询
	err := query.Limit(filter.Limit).Offset(filter.Offset).Find(&kbs).Error

	// 6. 返回查询到的数据集、总数据量以及可能发生的错误
	return kbs, count, err
}

// KnowledgeBaseFilter 是仓储层使用的知识库分页和名称搜索条件。
type KnowledgeBaseFilter struct {
	Limit  int
	Offset int
	Search string
}

// createKnowledgeBase 持久化新建知识库的元数据，不负责创建向量索引。
func (m *models) createKnowledgeBase(ctx context.Context, kb *model.KnowledgeBase) error {
	return m.db.WithContext(ctx).Create(kb).Error
}

// newModels 用传入的 GORM 连接构造知识库仓储实现。
func newModels(db *gorm.DB) *models {
	return &models{
		db: db,
	}
}

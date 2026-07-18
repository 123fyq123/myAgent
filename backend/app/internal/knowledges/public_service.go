package knowledges

import (
	"app/shared"
	"context"

	"github.com/mszlu521/thunder/database"
	"github.com/mszlu521/thunder/event"
)

// PublicService 是知识库模块对其他业务模块暴露的进程内事件服务。
// 它不处理 HTTP 请求，而是把通用请求转换为本模块的查询和检索调用。
type PublicService struct {
	repo repository
}

// GetKnowledgeBase 处理 getKnowledgeBase 内部事件，并按用户与知识库 ID 查询元数据。
func (s *PublicService) GetKnowledgeBase(e event.Event) (any, error) {
	request := e.Data.(*shared.GetKnowledgeBaseRequest)
	knowledgeBase, err := s.repo.getKnowledgeBase(context.Background(), request.UserId, request.KnowledgeBaseId)
	return knowledgeBase, err
}

// SearchKnowledgeBase 处理 searchKnowledgeBase 内部事件。
// 返回值被裁剪为文本片段，避免上层模块依赖本模块的私有检索响应类型。
func (s *PublicService) SearchKnowledgeBase(e event.Event) (any, error) {
	request := e.Data.(*shared.SearchKnowledgeBaseRequest)
	kbService := newService()
	response, err := kbService.searchKnowledgeBase(context.Background(), request.UserId, request.KnowledgeBaseId, searchParams{
		Query: request.Query,
	})
	if err != nil {
		return nil, err
	}
	var results []*shared.SearchKnowledgeBaseResult
	for _, v := range response.Results {
		results = append(results, &shared.SearchKnowledgeBaseResult{
			Content: v.Content,
		})
	}
	return &shared.SearchKnowledgeBaseResponse{
		Results: results,
	}, nil
}

// NewPublicService 使用共享 PostgreSQL 连接创建内部事件服务。
func NewPublicService() *PublicService {
	return &PublicService{
		repo: newModels(database.GetPostgresDB().GormDB),
	}
}

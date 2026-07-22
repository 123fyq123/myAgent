package knowledges

import (
	"app/shared"
	"bufio"
	"bytes"
	"common/biz"
	"common/utils"
	"context"
	"core/ai/kbs"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"model"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/cloudwego/eino-ext/components/document/loader/file"
	"github.com/cloudwego/eino-ext/components/document/parser/docx"
	"github.com/cloudwego/eino-ext/components/document/parser/html"
	"github.com/cloudwego/eino-ext/components/document/parser/pdf"
	"github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino-ext/components/model/qwen"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/components/document/parser"
	"github.com/cloudwego/eino/components/embedding"
	aiModel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/google/uuid"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/mszlu521/thunder/ai/einos"
	"github.com/mszlu521/thunder/config"
	"github.com/mszlu521/thunder/database"
	"github.com/mszlu521/thunder/einos/components/document/parser/epub"
	"github.com/mszlu521/thunder/errs"
	"github.com/mszlu521/thunder/event"
	"github.com/mszlu521/thunder/logs"
	html2 "golang.org/x/net/html"
	"gorm.io/gorm"
)

// service 编排知识库业务：关系库元数据、文件解析、嵌入模型、Milvus 与 Elasticsearch。
// repo 管理 PostgreSQL，两个客户端分别服务于向量检索和全文/索引清理。
type service struct {
	repo         repository
	esClient     *elasticsearch.Client
	milvusClient client.Client
}

// createKnowledgeBase 将前端模型配置转换为知识库元数据并写入 PostgreSQL。
// 此处只创建知识库记录；向量集合会在文档首次处理时按需使用。
func (s *service) createKnowledgeBase(ctx context.Context, userId uuid.UUID, req createKnowledgeBaseReq) (any, error) {
	kb := model.KnowledgeBase{
		BaseModel: model.BaseModel{
			ID: uuid.New(),
		},
		CreatorID:              userId,
		Name:                   req.Name,
		Description:            req.Description,
		ChatModelName:          req.ChatModelName,
		ChatModelProvider:      req.ChatModelProvider,
		EmbeddingModelName:     req.EmbeddingModelName,
		EmbeddingModelProvider: req.EmbeddingModelProvider,
		StorageType:            model.StorageTypeElasticSearch,
		StorageConfig:          model.JSON{},
		DocumentCount:          0,
		Tags:                   req.Tags,
	}
	err := s.repo.createKnowledgeBase(ctx, &kb)
	if err != nil {
		logs.Errorf("create knowledge base error: %v", err)
		return nil, errs.DBError
	}
	return &kb, nil
}

// listKnowledgeBases 内部服务层方法：根据用户ID和查询参数获取知识库分页列表
func (s *service) listKnowledgeBases(ctx context.Context, userId uuid.UUID, params listReq) (*ListResp, error) {
	// 1. 设置默认页码：如果前端传入的页码小于等于 0，则默认查询第 1 页
	page := params.Page
	if page <= 0 {
		page = 1
	}

	// 2. 设置默认每页大小：如果未传入或小于等于 0，则默认每页展示 10 条数据
	size := params.PageSize
	if size <= 0 {
		size = 10
	}

	// 3. 构建数据库查询过滤器，将“页码/数量”转换为 SQL 的 “Limit/Offset” 限制
	filter := KnowledgeBaseFilter{
		Search: params.Search,     // 模糊搜索关键词
		Limit:  size,              // 限制返回的记录数
		Offset: (page - 1) * size, // 数据的偏移量（跳过前多少条记录）
	}

	// 4. 调用 Repository 仓储层，从数据库中查询匹配的知识库列表及总记录数
	kbs, total, err := s.repo.listKnowledgeBases(ctx, userId, filter)
	if err != nil {
		// 记录错误日志，并向外返回统一定义的数据库内部错误（屏蔽底层敏感信息）
		logs.Errorf("list knowledge base error: %v", err)
		return nil, errs.DBError
	}

	// 5. 成功获取数据，封装分页结果并返回
	return &ListResp{
		KnowledgeBases: kbs,   // 当前页的知识库数据切片
		Total:          total, // 满足查询条件的总数据量，用于前端生成分页器
	}, nil
}

// getKnowledgeBase 内部服务层方法：根据用户ID和知识库ID获取详细信息，并动态统计其关联的文档数据
func (s *service) getKnowledgeBase(ctx context.Context, userId uuid.UUID, id uuid.UUID) (*KnowledgeBaseResponse, error) {
	// 1. 调用 Repository 仓储层，查询指定用户名下的知识库基础信息
	kb, err := s.repo.getKnowledgeBase(ctx, userId, id)
	if err != nil {
		// 记录错误日志，向外返回统一定义的数据库内部错误
		logs.Errorf("get knowledge base error: %v", err)
		return nil, errs.DBError
	}

	// 2. 动态统计该知识库名下所有文档的【总大小 (字节数)】和【文档总数】
	// 这样做能保证前端拿到的文档体积和数量是实时且准确的
	totalSize, docCount, err := s.repo.countKnowledgeBaseDocuments(ctx, kb.ID)
	if err != nil {
		// 统计失败时记录日志，并同样返回数据库内部错误
		logs.Errorf("count knowledge base documents error: %v", err)
		return nil, errs.DBError
	}

	// 3. 将数据库模型（Model）组装映射为前端所需的响应结构体（DTO）
	return &KnowledgeBaseResponse{
		Id:                     kb.ID,
		Name:                   kb.Name,
		Description:            kb.Description,
		EmbeddingModelName:     kb.EmbeddingModelName,     // 向量化模型名称
		EmbeddingModelProvider: kb.EmbeddingModelProvider, // 向量化模型厂商
		ChatModelName:          kb.ChatModelName,          // 对话模型名称
		ChatModelProvider:      kb.ChatModelProvider,      // 对话模型厂商
		StorageType:            kb.StorageType,            // 存储类型（如 es, pgvector 等）
		StorageConfig:          kb.StorageConfig,          // 存储的配置信息 (JSONB)
		Tags:                   kb.Tags,                   // 标签列表
		TotalSize:              totalSize,                 // 动态统计出的文档总大小 (Bytes)
		DocumentCount:          int(docCount),             // 动态统计出的文档总数 (转为 int)
		CreatorId:              kb.CreatorID,              // 创建者 ID
		CreatedAt:              kb.CreatedAt.Unix(),       // 创建时间（转换为秒级时间戳）
		UpdatedAt:              kb.UpdatedAt.Unix(),       // 更新时间（转换为秒级时间戳）
	}, nil
}

// updateKnowledgeBase 先确认归属，再按非空字段覆盖可编辑配置，避免空值误清空旧配置。
func (s *service) updateKnowledgeBase(ctx context.Context, userId uuid.UUID, id uuid.UUID, req updateKnowledgeBaseReq) (any, error) {
	kb, err := s.repo.getKnowledgeBase(ctx, userId, id)
	if err != nil {
		logs.Errorf("get knowledge base error: %v", err)
		return nil, errs.DBError
	}
	if kb == nil {
		return nil, biz.ErrKnowledgeBaseNotFound
	}
	if req.Name != "" {
		kb.Name = req.Name
	}
	if req.Description != "" {
		kb.Description = req.Description
	}
	if req.EmbeddingModelName != "" {
		kb.EmbeddingModelName = req.EmbeddingModelName
	}
	if req.EmbeddingModelProvider != "" {
		kb.EmbeddingModelProvider = req.EmbeddingModelProvider
	}
	if req.ChatModelName != "" {
		kb.ChatModelName = req.ChatModelName
	}
	if req.ChatModelProvider != "" {
		kb.ChatModelProvider = req.ChatModelProvider
	}
	err = s.repo.updateKnowledgeBase(ctx, kb)
	if err != nil {
		logs.Errorf("update knowledge base error: %v", err)
		return nil, errs.DBError
	}
	return kb, nil
}

// deleteKnowledgeBase 删除知识库元数据；调用前先验证当前用户拥有该知识库。
func (s *service) deleteKnowledgeBase(ctx context.Context, userId uuid.UUID, id uuid.UUID) error {
	kb, err := s.repo.getKnowledgeBase(ctx, userId, id)
	if err != nil {
		logs.Errorf("get knowledge base error: %v", err)
		return errs.DBError
	}
	if kb == nil {
		return biz.ErrKnowledgeBaseNotFound
	}
	err = s.repo.deleteKnowledgeBase(ctx, kb.ID)
	if err != nil {
		logs.Errorf("delete knowledge base error: %v", err)
		return errs.DBError
	}
	return nil
}

// listDocuments 将 HTTP 分页参数转换为仓储层的 Limit/Offset 查询。
func (s *service) listDocuments(ctx context.Context, userId uuid.UUID, kbId uuid.UUID, params listDocumentReq) (*ListDocumentsResp, error) {
	page := params.Page
	if page <= 0 {
		page = 1
	}
	size := params.PageSize
	if size <= 0 {
		size = 10
	}
	filter := DocumentFilter{
		Status: params.Status,
		Search: params.Search,
		Limit:  size,
		Offset: (page - 1) * size,
	}
	documents, total, err := s.repo.listDocuments(ctx, userId, kbId, filter)
	if err != nil {
		logs.Errorf("list documents error: %v", err)
		return nil, errs.DBError
	}
	return &ListDocumentsResp{
		Documents: documents,
		Total:     total,
	}, nil
}

// uploadDocuments 同步完成文件接收与文档登记，随后异步完成解析、切分、嵌入和索引。
// 调用方应通过文档 status 判断后台任务是否完成，而非假定接口返回即已可检索。
func (s *service) uploadDocuments(ctx context.Context, userId uuid.UUID, kbId uuid.UUID, uploadFile *multipart.FileHeader) (any, error) {
	//读取文件信息，创建Document对象
	//读取文件内容，进行向量化和索引，其中要进行切分，切分后的数据存入documentchunk表中
	//同时将切分后的内容，向量化后存入向量数据库中
	//文件可以存入云存储中
	//我们先写读取文件信息，创建Document对象，并存入数据库中这个步骤
	//先检查知识库是否存在
	kb, err := s.repo.getKnowledgeBase(ctx, userId, kbId)
	if err != nil {
		logs.Errorf("get knowledge base error: %v", err)
		return nil, errs.DBError
	}
	if kb == nil {
		return nil, biz.ErrKnowledgeBaseNotFound
	}
	ext := strings.ToLower(filepath.Ext(uploadFile.Filename))
	fileType := kbs.FromExtension(ext)
	var selectParser parser.Parser
	switch fileType {
	case kbs.Markdown:
		selectParser = parser.TextParser{}
	case kbs.Docx:
		selectParser, err = kbs.DocxParser(&docx.Config{
			ToSections:     true,
			IncludeTables:  true,
			IncludeFooters: true,
			IncludeHeaders: true,
		})
		if err != nil {
			logs.Errorf("new docx parser error: %v", err)
			return nil, biz.FileLoadError
		}
	case kbs.PDF:
		selectParser, err = kbs.PDFParser(&pdf.Config{
			//不按分页 获取全部内容
			ToPages: false,
		})
		if err != nil {
			logs.Errorf("new pdf parser error: %v", err)
			return nil, biz.FileLoadError
		}
	case kbs.Html:
		selectParser, err = kbs.HtmlParser(&kbs.HtmlConfig{
			Selector: &html.BodySelector,
		})
		if err != nil {
			logs.Errorf("new html parser error: %v", err)
			return nil, biz.FileLoadError
		}
	case kbs.Epub:
		selectParser, err = kbs.EpubParser(&epub.Config{
			StripHTML: true,
		})
		if err != nil {
			logs.Errorf("new epub parser error: %v", err)
			return nil, biz.FileLoadError
		}

	default:
		selectParser = parser.TextParser{}
	}
	//读取文件内容
	loader, err := file.NewFileLoader(ctx, &file.FileLoaderConfig{
		Parser: selectParser,
	})
	if err != nil {
		logs.Errorf("new file loader error: %v", err)
		return nil, biz.FileLoadError
	}
	src, err := uploadFile.Open()
	if err != nil {
		logs.Errorf("open file error: %v", err)
		return nil, biz.FileLoadError
	}
	defer src.Close()
	tempFile, err := s.createTempFileFromUploadFile(src, uploadFile.Filename)
	if err != nil {
		logs.Errorf("create temp file error: %v", err)
		return nil, biz.FileLoadError
	}
	defer tempFile.Close()
	defer os.Remove(tempFile.Name())
	//这个URL是文件的地址，正常我们应该上传到云存储中，这里我们先创建一个本地临时文件来获取内容
	docs, err := loader.Load(ctx, document.Source{
		URI: tempFile.Name(),
	})
	if err != nil {
		logs.Errorf("load file error: %v", err)
		return nil, biz.FileLoadError
	}
	//文件后轴
	doc := &model.Document{
		KnowledgeBaseID: kb.ID,
		CreatorID:       userId,
		Name:            uploadFile.Filename,
		FileType:        ext,
		Size:            uploadFile.Size,
		StorageKey:      uploadFile.Filename,
		FileHash:        "",
		Status:          model.DocumentStatusPending,
		ErrorMessage:    "",
	}
	err = s.repo.createDocument(ctx, doc)
	if err != nil {
		logs.Errorf("create document error: %v", err)
		return nil, errs.DBError
	}
	//对文件内容的处理，切分+向量化+索引 放入go协程中，进行处理过程比较长
	go func() {
		//做状态更新
		//这个执行时间长，不能用上面的上下文
		ctx = context.Background()
		err = s.repo.updateDocumentStatus(ctx, doc.ID, model.DocumentStatusProcessing)
		if err != nil {
			logs.Errorf("update document status error: %v", err)
			return
		}
		//处理文件
		err = s.processDocumentAndVectorAndStore(ctx, doc, docs, kb)
		if err != nil {
			logs.Errorf("process file error: %v", err)
			//更新状态为失败
			err = s.repo.updateDocumentStatus(ctx, doc.ID, model.DocumentStatusFailed)
			if err != nil {
				logs.Errorf("update document status error: %v", err)
				return
			}
			return
		}
		//最后更新状态
		err = s.repo.updateDocumentStatus(ctx, doc.ID, model.DocumentStatusCompleted)
		if err != nil {
			logs.Errorf("update document status error: %v", err)
			return
		}
	}()
	return doc, nil
}

const (
	maxChildSize     = 500 //子块最大的长度
	childOverlapSize = 150 // 子块重叠的长度
)

// processDocumentAndVectorAndStore 按文件类型生成父分片和子分片，并同步保存关系库与向量库。
// 父分片用于给模型提供完整上下文，子分片用于更细粒度的语义召回。
func (s *service) processDocumentAndVectorAndStore(ctx context.Context, doc *model.Document, docs []*schema.Document, kb *model.KnowledgeBase) error {
	//获取文档内容
	var content string
	if len(docs) > 0 && docs[0] != nil {
		content = docs[0].Content
	}
	//如果文档内容为空 直接返回
	if content == "" {
		logs.Warnf("document content is empty")
		return nil
	}
	var parentModels []*model.DocumentChunk
	var childSchemaDocs []*schema.Document
	fileType := kbs.FromExtension(doc.FileType)
	//这里我们先支持md文档
	if fileType == kbs.Markdown {
		//md格式有清晰的标题 我们按照标题进行切分
		//documents = s.parseMarkdownHeaders(content)
		//if len(documents) == 0 {
		//	documents = append(documents, &schema.Document{
		//		ID:      doc.ID.String(),
		//		Content: content,
		//	})
		//}
		//对md文档进行层次性划分，我们以资料中提供的md文档为例子
		//h1认为是文档名称 h2认为是章节 h3认为是小节 进行层次性划分
		//chunk表中 存储h2的内容
		//提取标题 这里我们写个通用的
		parentModels, childSchemaDocs = s.processMarkdown(content, doc, parentModels, kb, childSchemaDocs)
	} else if fileType == kbs.Docx {
		parentModels, childSchemaDocs = s.processDocx(docs, doc, parentModels, kb, childSchemaDocs)
	} else if fileType == kbs.PDF {
		parentModels, childSchemaDocs = s.processPDF(docs, doc, parentModels, kb, childSchemaDocs)
	} else if fileType == kbs.Html {
		parentModels, childSchemaDocs = s.processHtml(docs, doc, parentModels, kb, childSchemaDocs)
	} else if fileType == kbs.Epub {
		parentModels, childSchemaDocs = s.processEpub(docs, doc, parentModels, kb, childSchemaDocs)
	} else {
		//这个通用的处理，我们按照长度进行切分
		parentTexts := utils.SplitByWindow(content, 1200, 200)
		for i, pText := range parentTexts {
			parentModels = append(parentModels, &model.DocumentChunk{
				BaseModel:       model.BaseModel{ID: uuid.New()},
				DocumentID:      doc.ID,
				KnowledgeBaseID: kb.ID,
				Content:         pText,
				ChunkIndex:      i,
				MetaInfo: map[string]interface{}{
					"source":    doc.Name,
					"file_type": doc.FileType,
					"type":      "generic",
				},
				TokenCount: utils.GetTokenCount(pText),
				Status:     model.ChunkStatusEmbedded,
			})
			pathPrefix := fmt.Sprintf("【文档:%s】【片段:%d】\n", doc.Name, i+1)
			childTexts := utils.SplitByWindow(content, 400, 50)
			for j, cText := range childTexts {
				childSchemaDocs = append(childSchemaDocs, s.buildChildSchemaDoc(parentModels[i].ID, doc, kb, pathPrefix+cText, i, j, 0, nil))
			}
		}
	}
	return s.saveToStores(ctx, kb, parentModels, childSchemaDocs)
}

// processMarkdown 按 Markdown 标题层级生成较完整的父分片，再按窗口切出用于向量检索的子分片。
func (s *service) processMarkdown(content string, doc *model.Document, parentModels []*model.DocumentChunk, kb *model.KnowledgeBase, childSchemaDocs []*schema.Document) ([]*model.DocumentChunk, []*schema.Document) {
	h1Title := utils.ExtractTitle(content, "#")
	if h1Title == "" {
		h1Title = doc.Name
	}
	//获取h2的内容
	h2Block := utils.SplitByHeading(content, "##")
	for i, h2 := range h2Block {
		parentId := uuid.New()
		h2Title := utils.ExtractTitle(h2, "##")
		if h2Title == "" {
			h2Title = "概览"
		}
		//h2的内容是parent
		parentModels = append(parentModels, &model.DocumentChunk{
			BaseModel:       model.BaseModel{ID: parentId},
			DocumentID:      doc.ID,
			KnowledgeBaseID: kb.ID,
			Content:         h2,
			ChunkIndex:      i,
			MetaInfo: map[string]interface{}{
				"h1": h1Title,
				"h2": h2Title,
			},
			TokenCount: utils.GetTokenCount(h2),
			Status:     model.ChunkStatusEmbedded,
		})
		//获取h3的内容 这部分做为child
		h3Block := utils.SplitByHeading(h2, "###")
		for j, h3 := range h3Block {
			h3Title := utils.ExtractTitle(h3, "###")
			//这里我们给child的内容 添加一个前缀 表明所属的上级
			pathPrefix := fmt.Sprintf("【文档:%s】 > 【主题:%s】", h1Title, h2Title)
			if h3Title != "" {
				h3Title += " > 【子题: " + h3Title + "】"
			}
			//添加一个换行
			pathPrefix += "\n"
			//为了防止子内容过长，我们设定一个长度，做一次切分
			subTexts := utils.SplitTextByLength(h3, maxChildSize-len(pathPrefix), childOverlapSize)
			for k, text := range subTexts {
				//text就是最终子块的内容
				childSchemaDocs = append(childSchemaDocs, s.buildChildSchemaDoc(parentId, doc, kb, pathPrefix+text, i, j, k, nil))
			}
		}
	}
	return parentModels, childSchemaDocs
}

// createTempFileFromUploadFile 将 multipart 上传流落为临时文件，供仅接受文件路径的解析器读取。
func (s *service) createTempFileFromUploadFile(src multipart.File, fileName string) (*os.File, error) {
	//创建一个临时文件
	tempFile, err := os.CreateTemp("", "upload-*.tmp")
	if err != nil {
		return nil, err
	}
	defer tempFile.Close()
	//复制文件内容到临时文件
	_, err = io.Copy(tempFile, src)
	if err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		return nil, err
	}
	//重置文件指针到开始位置
	_, err = tempFile.Seek(0, io.SeekStart)
	if err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		return nil, err
	}
	return tempFile, nil
}

// getEmbeddingConfig 内部服务层方法：根据厂商、模型名称和用户ID，动态加载并初始化对应的向量化（Embedding）客户端实例
// getEmbeddingConfig 根据知识库绑定的提供商和模型名创建嵌入器，用于入库和检索时保持同一向量空间。
func (s *service) getEmbeddingConfig(provider string, embeddingModelName string, creatorID uuid.UUID) (embedding.Embedder, error) {
	if provider == "" || embeddingModelName == "" {
		return nil, biz.ErrEmbeddingConfigNotFound
	}

	// 1. 触发内置事件总线（Event Bus）或插件系统
	// 核心考量：模型配置信息（API Key, Base URL 等）可能由其他微服务、模块或专门的模型管理模块维护。
	// 这里通过事件机制解耦，异步或同步去获取该用户专属的向量模型详细配置信息。
	trigger, err := event.Trigger("getEmbeddingConfig", &shared.LLMParams{
		Provider:  provider,               // 模型厂商（如: openai, ollama）
		Model:     embeddingModelName,     // 模型名称（如: text-embedding-3-small）
		UserId:    creatorID,              // 用户 ID（用于捞取该用户自己配置的私有 API Key）
		ModelType: model.LLMTypeEmbedding, // 指定模型类型为嵌入式向量模型 (Embedding)
	})
	if err != nil {
		return nil, err
	}

	// 2. 将事件触发返回的结果（interface{} 类型）断言转换为预期的结构体指针
	response, ok := trigger.(*shared.EmbeddingConfigResponse)
	if !ok || response == nil || response.Model == nil {
		return nil, biz.ErrEmbeddingConfigNotFound
	}

	// 3. 使用大模型框架（这里使用的是 einos 框架）动态加载并驱动 Embedding 实例
	// 传入厂商标识和拼装好的模型配置（ToEmbeddingConfig 包含了最终的 APIKey、Endpoint 等秘密信息）
	embedder, err := einos.LoadEmbedding(context.Background(),
		response.Model.ProviderConfig.Provider,
		response.Model.ToEmbeddingConfig())

	// 4. 返回初始化完毕的、立即可用的向量化客户端实例（实现了 embedding.Embedder 接口）
	return embedder, err
}

// deleteDocuments 协调删除 PostgreSQL 文档/分片、Elasticsearch 索引和 Milvus 向量，避免残留检索结果。
func (s *service) deleteDocuments(ctx context.Context, userId uuid.UUID, kbId uuid.UUID, documentId uuid.UUID) error {
	//先确认参数正确
	knowledgeBase, err := s.repo.getKnowledgeBase(ctx, userId, kbId)
	if err != nil {
		logs.Errorf("get knowledge base error: %v", err)
		return errs.DBError
	}
	if knowledgeBase == nil {
		return biz.ErrKnowledgeBaseNotFound
	}
	doc, err := s.repo.getDocument(ctx, userId, kbId, documentId)
	if err != nil {
		logs.Errorf("get document error: %v", err)
		return errs.DBError
	}
	if doc == nil {
		return biz.ErrDocumentNotFound
	}
	//删除多个表的数据，所以这里必须要用事务
	err = s.repo.transaction(ctx, func(tx *gorm.DB) error {
		//先删除文档
		err = s.repo.deleteDocuments(ctx, tx, userId, kbId, documentId)
		if err != nil {
			logs.Errorf("delete documents error: %v", err)
			return err
		}
		//删除文档片段
		err = s.repo.deleteDocumentChunks(ctx, tx, kbId, documentId)
		if err != nil {
			logs.Errorf("delete document chunks error: %v", err)
			return err
		}
		//删除es的索引
		if knowledgeBase.StorageType == model.StorageTypeElasticSearch {
			err = s.deleteEsIndex(ctx, kbId, documentId)
			if err != nil {
				logs.Errorf("delete es index error: %v", err)
				return err
			}
		}
		if knowledgeBase.StorageType == model.StorageTypeMilvus {
			err = s.deleteMilvusIndex(ctx, kbId, documentId)
		}
		return nil
	})
	if err != nil {
		logs.Errorf("delete documents error: %v", err)
		return errs.DBError
	}
	return nil
}

// deleteEsIndex 删除文档在 Elasticsearch 中的索引记录；当前主检索虽使用 Milvus，仍保留清理逻辑。
func (s *service) deleteEsIndex(ctx context.Context, kbId uuid.UUID, documentId uuid.UUID) error {
	index := s.buildIndex(kbId)
	//需要删除doc_id这个字段匹配的文档
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"doc_id.keyword": documentId.String(), //使用keyword精确匹配
			},
		},
	}
	//查询条件转换为json
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	err := encoder.Encode(query)
	if err != nil {
		logs.Errorf("encode query error: %v", err)
		return err
	}
	res, err := s.esClient.DeleteByQuery(
		[]string{index},
		&buf,
		s.esClient.DeleteByQuery.WithContext(ctx),
	)
	if err != nil {
		logs.Errorf("delete by query error: %v", err)
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		logs.Errorf("delete by query error: %v", res)
		return err
	}
	return nil
}

// buildIndex 为每个知识库生成隔离的向量集合/索引名称。
func (s *service) buildIndex(kbId uuid.UUID) string {
	sprintf := fmt.Sprintf("kb_%s", kbId.String())
	sprintf = strings.ReplaceAll(sprintf, "-", "_")
	return sprintf
}

const (
	maxSearchResult = 5 //设置一个最大搜索结果数量
)

// searchKnowledgeBase 核心业务层方法：处理高级知识库检索逻辑（支持 LLM 意图提取与父子分段召回）
// searchKnowledgeBase 执行 RAG 检索：解析问题意图、向量召回子分片，再回填完整父分片。
func (s *service) searchKnowledgeBase(ctx context.Context, userId uuid.UUID, kbId uuid.UUID, params searchParams) (*SearchResponse, error) {
	// 1. 记录函数开始时间，用于统计整个检索流程的耗时
	startTime := time.Now()

	// 2. 根据知识库唯一 ID 构建对应的向量数据库索引/集合名称
	index := s.buildIndex(kbId)

	// 3. 验证知识库是否存在，并校验用户是否拥有该知识库的访问权限
	knowledgeBase, err := s.repo.getKnowledgeBase(ctx, userId, kbId)
	if err != nil {
		logs.Errorf("get knowledge base error: %v", err)
		return nil, errs.DBError
	}
	if knowledgeBase == nil {
		return nil, biz.ErrKnowledgeBaseNotFound
	}

	// 4. 空值防御：如果用户的查询词为空，直接返回标准的空结果结构体，避免无效计算
	if params.Query == "" {
		return &SearchResponse{
			KbId:    kbId,
			Query:   params.Query,
			Results: []*SearchResult{},
			Took:    time.Since(startTime).Microseconds(), // 计算微秒级耗时
			Total:   0,
		}, nil
	}

	// 5. 【RAG 优化亮点 1】调用大模型（LLM）解析查询意图
	// 例如：用户问“第100章讲了什么？”，LLM 能精准提取出章节数 100 及其关键字，
	// 后续可以通过精确匹配（Metadata Filter）大幅缩减向量搜索范围，提升检索准确率。
	intent, _ := s.parseQueryIntent(ctx, knowledgeBase, params.Query)

	// 6. 获取当前知识库配置的向量化（Embedding）模型实例
	embedder, err := s.getEmbeddingConfig(knowledgeBase.EmbeddingModelProvider, knowledgeBase.EmbeddingModelName, userId)
	if err != nil {
		logs.Errorf("get embedding config error: %v", err)
		return nil, biz.ErrEmbeddingConfigNotFound
	}

	// 7. 初始化向量数据库存储引擎（这里为了简化写死用了 Milvus，也可以切换成注释中的 ES）
	// 初始化一个具备特定配置的“查询工具人”
	// store, err := kbs.NewESVectorStore(ctx, s.esClient, index, embedder)
	store, err := kbs.NewMilvusVectorStore(ctx, s.milvusClient, index, embedder)
	if err != nil {
		logs.Errorf("new vector store error: %v", err)
		return nil, err
	}

	// 8. 组装元数据过滤条件（Metadata Filter）
	// 如果 LLM 成功解析出了具体的章节或卷号，将其注入过滤器进行硬过滤
	filter := make(kbs.SearchFilter)
	if intent.ChapterNum > 0 {
		filter["chapter_num"] = intent.ChapterNum
	}
	if intent.VolumeNum > 0 {
		filter["volume_num"] = intent.VolumeNum
	}

	// 9. 执行向量检索，传入提取的关键字、召回数量（10条）以及过滤条件
	// 这里搜索出的是粒度较小的“子文档分段 (Child Docs)”
	childDocs, err := store.Search(ctx, intent.Keywords, 10, filter)
	if err != nil {
		logs.Errorf("search error: %v", err)
		return nil, err
	}

	// 10. 【RAG 优化亮点 2：父子分段机制】
	// 向量库中存的是信息密度高的短分段（子），但为了给大模型提供更完整的上下文，
	// 我们需要通过子文档里的 parent_id 去关联网页/数据库里更长更完整的原始分段（父）。
	parentIdMap := make(map[string]float64) // 用于存储映射关系 doc_chunk_id -> score
	var orderedParentIds []string           // 用于保持子文档原本从高到低的分数排序顺序

	for _, cd := range childDocs {
		pId, ok := cd.MetaData["parent_id"].(string)
		if !ok {
			continue
		}

		// 对父 ID 进行去重，并在第一次见到时记录其原本的向量分值和排序顺序
		if _, seen := parentIdMap[pId]; !seen {
			orderedParentIds = append(orderedParentIds, pId)
			parentIdMap[pId] = cd.Score() // 记录该父段对应的最高匹配分数
		}
	}

	// 11. 如果没有召回到任何有效的父文档 ID，直接返回空结果
	if len(orderedParentIds) == 0 {
		return &SearchResponse{
			KbId:  kbId,
			Query: params.Query,
			Took:  time.Since(startTime).Microseconds(),
			Total: 0,
		}, nil
	}

	// 12. 截断阈值防御：限制返回给大模型的最大上下文数量
	// 防止召回过多低相关度的数据，既浪费 Token 也会干扰大模型的判断
	if len(orderedParentIds) > maxSearchResult {
		orderedParentIds = orderedParentIds[:maxSearchResult]
	}

	// 13. 去关系型数据库（Repository）中批量批量查询出这些父分段的“完整文本内容”
	parentChunks, err := s.repo.getDocumentChunksByIds(ctx, orderedParentIds)
	if err != nil {
		logs.Errorf("get document chunks error: %v", err)
		return nil, errs.DBError
	}

	// 14. 重新组装成前端和 LLM 所需的搜索结果列表
	results := make([]*SearchResult, 0, len(parentChunks))
	for i, chunk := range parentChunks {
		results = append(results, &SearchResult{
			Content:    chunk.Content,                  // 完整的父分段文本内容
			DocumentId: chunk.DocumentID,               // 所属文档的 ID
			Id:         chunk.ID,                       // 当前分段的 ID
			Metadata:   chunk.MetaInfo,                 // 元数据信息
			Position:   i,                              // 排序位置（从 0 开始）
			Score:      parentIdMap[chunk.ID.String()], // 回填刚才记录的向量匹配分数
		})
	}

	// 15. 构建最终响应，返回结果集及整个知识库检索过程的总耗时
	return &SearchResponse{
		KbId:    kbId,
		Query:   params.Query,
		Results: results,
		Took:    time.Since(startTime).Microseconds(),
		Total:   int64(len(results)),
	}, nil
}

// parseMarkdownHeaders 将 Markdown 的标题与正文拆为带层级元数据的文档片段。
func (s *service) parseMarkdownHeaders(content string) []*schema.Document {
	var docs []*schema.Document
	scanner := bufio.NewScanner(strings.NewReader(content))
	// 正则匹配 #, ##, ### (最多支持到 h6)
	// 格式: 行首 + 1-6个# + 空格 + 标题内容
	headerRegex := regexp.MustCompile(`^(#{1,6})\s+(.*)`)
	var currentBuffer strings.Builder
	// 记录当前的标题层级状态 {"h1": "标题A", "h2": "子标题B"}
	currentHeaders := make(map[string]string)

	flushBuffer := func() {
		text := strings.TrimSpace(currentBuffer.String())
		if text == "" {
			return
		}

		// 深拷贝当前的 header 状态，防止被后续修改影响
		meta := make(map[string]interface{})
		for k, v := range currentHeaders {
			meta[k] = v
		}

		docs = append(docs, &schema.Document{
			Content:  text,
			MetaData: meta,
		})
		currentBuffer.Reset()
	}

	for scanner.Scan() {
		line := scanner.Text()
		matches := headerRegex.FindStringSubmatch(line)

		if len(matches) == 3 {
			// === 发现新标题 ===

			// 1. 如果缓冲区有上一段的内容，先保存上一段
			flushBuffer()

			// 2. 更新层级上下文
			hashes := matches[1]                       // "##"
			titleText := strings.TrimSpace(matches[2]) // "部署指南"
			level := len(hashes)                       // 2

			// 记录当前级别标题
			levelKey := fmt.Sprintf("h%d", level)
			currentHeaders[levelKey] = titleText

			// 清除比当前级别更深的标题 (例如遇到新的 h2，旧的 h3, h4 应该失效)
			for i := level + 1; i <= 6; i++ {
				delete(currentHeaders, fmt.Sprintf("h%d", i))
			}
			// 3. 将标题行本身也写入新缓冲区的开头 (可选项，有助于语义完整)
			currentBuffer.WriteString(line + "\n")
		} else {
			// === 普通正文 ===
			currentBuffer.WriteString(line + "\n")
		}
	}
	// 处理最后剩余的内容
	flushBuffer()

	return docs
}

// buildChildSchemaDoc 构造可写入向量库的子分片，并附带父分片、文档和知识库关联信息。
func (s *service) buildChildSchemaDoc(parentId uuid.UUID, doc *model.Document, kb *model.KnowledgeBase, text string, i int, j int, k int, meta map[string]any) *schema.Document {

	data := map[string]interface{}{
		"doc_id":    doc.ID.String(),
		"kb_id":     kb.ID.String(),
		"parent_id": parentId.String(),
		"seq":       fmt.Sprintf("%d.%d.%d", i, j, k),
	}
	if meta != nil {
		for k, v := range meta {
			data[k] = v
		}
	}
	return &schema.Document{
		ID:       uuid.New().String(),
		Content:  text,
		MetaData: data,
	}
}

// saveToStores 先保存 PostgreSQL 父分片，再将可召回的子分片嵌入并写入向量存储。
func (s *service) saveToStores(ctx context.Context, kb *model.KnowledgeBase, parentModels []*model.DocumentChunk, docs []*schema.Document) error {
	//父分段直接存入数据库pg
	err := s.repo.createDocumentChunks(ctx, parentModels)
	if err != nil {
		logs.Errorf("create document chunks error: %v", err)
		return err
	}
	//子分段存入向量数据库，这里我们存入es中
	embedder, err := s.getEmbeddingConfig(kb.EmbeddingModelProvider, kb.EmbeddingModelName, kb.CreatorID)
	if err != nil {
		logs.Errorf("get embedding config error: %v", err)
		return biz.ErrEmbeddingConfigNotFound
	}
	//store, err := kbs.NewESVectorStore(ctx, s.esClient, s.buildIndex(kb.ID), embedder)
	store, err := kbs.NewMilvusVectorStore(ctx, s.milvusClient, s.buildIndex(kb.ID), embedder)
	if err != nil {
		logs.Errorf("new indexer error: %v", err)
		return err
	}

	err = store.Store(ctx, docs)
	return nil
}

// processDocx 将 DOCX 解析器输出的章节转换为父/子两层分片。
func (s *service) processDocx(sections []*schema.Document, doc *model.Document, parentModels []*model.DocumentChunk, kb *model.KnowledgeBase, childSchemaDocs []*schema.Document) ([]*model.DocumentChunk, []*schema.Document) {
	for _, sec := range sections {
		//main header footers tables
		sectionType := sec.MetaData["sectionType"].(string)
		//构建一个面包屑的前缀，放在内容的前面
		sectionLabel := s.mapSectionToChinese(sectionType)
		breadcrumb := fmt.Sprintf("【文档：%s】> 【%s】", doc.Name, sectionLabel)
		//父分段，这里word文档是直接全部读出来的，我们按照字符进行切分
		parentTexts := utils.SplitByWindow(sec.Content, 1200, 200)
		for i, text := range parentTexts {
			endContent := breadcrumb + "> " + text
			parentId := uuid.New()
			parentModel := &model.DocumentChunk{
				BaseModel: model.BaseModel{
					ID: parentId,
				},
				Content:         endContent,
				DocumentID:      doc.ID,
				KnowledgeBaseID: kb.ID,
				ChunkIndex:      i,
				MetaInfo:        sec.MetaData,
				TokenCount:      utils.GetTokenCount(endContent),
				Status:          model.ChunkStatusEmbedded,
			}
			parentModels = append(parentModels, parentModel)
			//子分段 这个数值 可以做成可配置的
			pathPrefix := breadcrumb + "\n"
			childTexts := utils.SplitByWindow(text, 400, 50)
			for j, childText := range childTexts {
				childSchemaDoc := s.buildChildSchemaDoc(parentId, doc, kb, pathPrefix+childText, i, j, 0, nil)
				childSchemaDocs = append(childSchemaDocs, childSchemaDoc)
			}
		}
	}
	return parentModels, childSchemaDocs
}

// mapSectionToChinese 将解析器章节类型转为便于前端和模型理解的中文标签。
func (s *service) mapSectionToChinese(sectionType string) string {
	switch sectionType {
	case "main":
		return "正文"
	case "header":
		return "标题"
	case "footer":
		return "页脚"
	case "table":
		return "表格"
	default:
		return "文档片段"
	}
}

// processPDF 清洗 PDF 文本并以页面/语义块为基础构建父/子分片。
func (s *service) processPDF(pages []*schema.Document, doc *model.Document, parentModels []*model.DocumentChunk, kb *model.KnowledgeBase, childSchemaDocs []*schema.Document) ([]*model.DocumentChunk, []*schema.Document) {
	if len(pages) == 0 {
		return parentModels, childSchemaDocs
	}
	//自定义去处理整个内容，切分为父分段
	parentTexts := s.cleanPDFText(pages[0].Content)
	for j, text := range parentTexts {
		breadcrumb := fmt.Sprintf("【文档：%s】> 【第%d页】", doc.Name, j+1)
		endContent := breadcrumb + "\n" + text
		parentId := uuid.New()
		parentModel := &model.DocumentChunk{
			BaseModel: model.BaseModel{
				ID: parentId,
			},
			Content:         endContent,
			DocumentID:      doc.ID,
			KnowledgeBaseID: kb.ID,
			ChunkIndex:      j,
			MetaInfo: map[string]interface{}{
				"page": j + 1,
			},
			TokenCount: utils.GetTokenCount(endContent),
			Status:     model.ChunkStatusEmbedded,
		}
		parentModels = append(parentModels, parentModel)
		//子分段 这个数值 可以做成可配置的
		pathPrefix := breadcrumb + "\n"
		childTexts := utils.SplitByWindow(text, 400, 50)
		for k, childText := range childTexts {
			childSchemaDoc := s.buildChildSchemaDoc(parentId, doc, kb, pathPrefix+childText, j, k, 0, nil)
			childSchemaDocs = append(childSchemaDocs, childSchemaDoc)
		}
	}
	return parentModels, childSchemaDocs
}

// cleanPDFText 清理 PDF 提取文本中的空白、页眉页脚和无意义短块，输出候选语义块。
func (s *service) cleanPDFText(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\t", "")
	content = regexp.MustCompile(`\s+`).ReplaceAllString(content, " ")
	boundaryPatterns := []string{
		`\s#\s*`,        //标题
		`Chapter\s+\d+`, //英文章节
		`第[一二三四五六七八九十]+[章节]`, //中文章节
	}
	for _, p := range boundaryPatterns {
		re := regexp.MustCompile(p)
		content = re.ReplaceAllString(content, "\n\n$0")
	}
	var parents []string
	var buf strings.Builder
	flush := func() {
		if buf.Len() == 0 {
			return
		}
		parents = append(parents, strings.TrimSpace(buf.String()))
		buf.Reset()
	}
	rawBlocks := strings.Split(content, "\n")
	for _, block := range rawBlocks {
		block = strings.TrimSpace(block)
		if block == "" {
			flush()
			continue
		}
		//处理代码行
		if looksLikeCode(block) {
			flush()
			parents = append(parents, block)
			continue
		}
		if buf.Len() > 0 {
			buf.WriteString(" ")
		}
		buf.WriteString(block)
		//判断是否有强语义结束
		if lookLikeSentenceEnd(block) {
			flush()
		}
	}
	flush()
	//去重
	return deduplicateParents(parents)
}

// processHtml 去除网页结构噪声后按正文块拆分，并生成可检索的子分片。
func (s *service) processHtml(docs []*schema.Document, doc *model.Document, parentModels []*model.DocumentChunk, kb *model.KnowledgeBase, childSchemaDocs []*schema.Document) ([]*model.DocumentChunk, []*schema.Document) {
	if len(docs) == 0 {
		return parentModels, childSchemaDocs
	}
	htmlDoc := docs[0]
	htmlContent := htmlDoc.Content
	if htmlContent == "" {
		return parentModels, childSchemaDocs
	}
	//解析html
	dom, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		logs.Errorf("new document from reader error: %v", err)
		return parentModels, childSchemaDocs
	}
	webTitle := htmlDoc.MetaData[html.MetaKeyTitle].(string)
	if webTitle == "" {
		webTitle = doc.Name
	}
	type Block struct {
		Tag    string
		Text   string
		IsCode bool
	}
	var blocks []Block
	isHeading := func(tag string) bool {
		return regexp.MustCompile(`^h[1-6]$`).MatchString(tag)
	}
	isAtom := func(tag string) bool {
		switch tag {
		case "code", "pre", "blockquote", "ul", "ol", "li", "table":
			return true
		}
		return false
	}
	body := dom.Find("body")
	if body.Length() == 0 {
		body = dom.Selection
	}
	processed := make(map[*html2.Node]bool)
	body.Find("*").Each(func(i int, s *goquery.Selection) {
		node := s.Get(0)
		if processed[node] {
			return
		}
		tag := strings.ToLower(goquery.NodeName(s))
		//标题
		if isHeading(tag) {
			blocks = append(blocks, Block{
				Tag:  tag,
				Text: strings.TrimSpace(s.Text()),
			})
			processed[node] = true
			return
		}
		//原子块
		if isAtom(tag) {
			txt := strings.TrimSpace(s.Text())
			if txt != "" {
				blocks = append(blocks, Block{
					Tag:    tag,
					Text:   txt,
					IsCode: tag == "code" || tag == "pre",
				})
			}
			s.Find("*").Each(func(i int, sub *goquery.Selection) {
				processed[sub.Get(0)] = true
			})
			processed[node] = true
			return
		}
		//普通文本
		if s.Children().Length() == 0 {
			txt := strings.TrimSpace(s.Text())
			if txt != "" {
				blocks = append(blocks, Block{
					Tag:  "p",
					Text: txt,
				})
			}
			processed[node] = true
		}
	})
	//语义聚合
	var (
		h1, h2, h3  string
		buf         strings.Builder
		parentIndex = 0
	)
	flush := func() {
		content := strings.TrimSpace(buf.String())
		if content == "" {
			return
		}
		parentId := uuid.New()
		breadcrumb := fmt.Sprintf("【网页:%s】", webTitle)
		if h1 != "" {
			breadcrumb += " > " + h1
		}
		if h2 != "" {
			breadcrumb += " > " + h2
		}
		if h3 != "" {
			breadcrumb += " > " + h3
		}
		fullContent := breadcrumb + "\n" + content
		parentModel := &model.DocumentChunk{
			BaseModel: model.BaseModel{
				ID: parentId,
			},
			Content:         fullContent,
			DocumentID:      doc.ID,
			KnowledgeBaseID: kb.ID,
			ChunkIndex:      parentIndex,
			MetaInfo: map[string]interface{}{
				"h1": h1,
				"h2": h2,
				"h3": h3,
			},
			TokenCount: utils.GetTokenCount(fullContent),
			Status:     model.ChunkStatusEmbedded,
		}
		parentModels = append(parentModels, parentModel)
		//子分段切分
		pathPrefix := breadcrumb + "\n"
		childTexts := utils.SplitByWindow(content, 400, 50)
		for k, childText := range childTexts {
			childSchemaDoc := s.buildChildSchemaDoc(parentId, doc, kb, pathPrefix+childText, parentIndex, k, 0, nil)
			childSchemaDocs = append(childSchemaDocs, childSchemaDoc)
		}
		buf.Reset()
		parentIndex++
	}
	for _, b := range blocks {
		switch b.Tag {
		case "h1":
			flush()
			h1, h2, h3 = b.Text, "", ""
		case "h2":
			flush()
			h2, h3 = b.Text, ""
		case "h3":
			buf.WriteString("\n### ")
			buf.WriteString(b.Text)
			buf.WriteString("\n")
		default:
			if b.IsCode {
				buf.WriteString("\n```\n")
				buf.WriteString(b.Text)
				buf.WriteString("\n```\n")
			} else {
				buf.WriteString(b.Text)
				buf.WriteString("\n")
			}
		}
		//父块的理想长度
		if buf.Len() >= 1200 {
			flush()
		}
	}
	flush()
	return parentModels, childSchemaDocs
}

// processEpub 按 EPUB 章节构建父分片，并在章节内滑窗生成用于召回的子分片。
func (s *service) processEpub(chapters []*schema.Document, doc *model.Document, parentModels []*model.DocumentChunk, kb *model.KnowledgeBase, childSchemaDocs []*schema.Document) ([]*model.DocumentChunk, []*schema.Document) {
	if len(chapters) == 0 {
		return parentModels, childSchemaDocs
	}
	for i, chapter := range chapters {
		//提取元数据
		bookTitle := chapter.MetaData["book_title"].(string)
		if bookTitle == "" {
			bookTitle = doc.Name
		}
		//章节
		chapterTitle := chapter.MetaData["chapter"].(string)
		if chapterTitle == "" {
			chapterTitle = "未定义章节"
		}
		breadcrumb := fmt.Sprintf("【书名:%s】 > 【章节:%s】", bookTitle, chapterTitle)
		//章节的内容 可能很多，这里正常是需要进行一下切分，也可以不切分
		//如果要切分 尽量切的大一些，一般比如小说 字数大概在2000-3000字
		//parentTexts := utils.SplitByWindow(chapter.Content, 2500, 300)
		fullParentContent := breadcrumb + "\n" + chapter.Content
		parentId := uuid.New()
		//解析复杂标题，比如卷名，章节号 卷号 标题等等
		parsed := utils.ParseComplexTitle(chapterTitle)
		parentModel := &model.DocumentChunk{
			BaseModel: model.BaseModel{
				ID: parentId,
			},
			Content:         fullParentContent,
			DocumentID:      doc.ID,
			KnowledgeBaseID: kb.ID,
			ChunkIndex:      i,
			MetaInfo: map[string]interface{}{
				"chapter_num": parsed.ChapterNum,
				"volume_num":  parsed.VolumeNum,
				"volume_name": parsed.VolumeName,
				"raw_title":   parsed.RawTitle,
				"full_title":  chapterTitle,
			},
			TokenCount: utils.GetTokenCount(fullParentContent),
			Status:     model.ChunkStatusEmbedded,
		}
		parentModels = append(parentModels, parentModel)
		//生成child
		childTexts := utils.SplitByWindow(chapter.Content, 400, 50)
		for k, childText := range childTexts {
			childSchemaDoc := s.buildChildSchemaDoc(parentId, doc, kb, breadcrumb+"\n"+childText, i, k, 0, parentModel.MetaInfo)
			childSchemaDocs = append(childSchemaDocs, childSchemaDoc)
		}
	}
	return parentModels, childSchemaDocs
}

// QueryIntent 是由聊天模型抽取的检索意图，提供关键词及可选的章节、卷号过滤条件。
type QueryIntent struct {
	Keywords   string `json:"keywords"`
	VolumeNum  int    `json:"volume_num"`  //卷号 0 表示未指定
	ChapterNum int    `json:"chapter_num"` //章节号 0 表示未指定
	DocName    string `json:"doc_name"`
}

// parseQueryIntent 内部服务层方法：利用大模型（LLM）对用户的提问进行意图解析，结构化提取出关键词、卷号和章节号
// parseQueryIntent 调用知识库绑定的聊天模型，把自然语言问题转换为关键词和元数据过滤条件。
func (s *service) parseQueryIntent(ctx context.Context, kb *model.KnowledgeBase, query string) (*QueryIntent, error) {
	// 1. 构建 System Prompt 指导大模型进行结构化数据提取
	// 严格限制返回格式为 JSON，并包含清晰的转换规则（如中文数字转阿拉伯数字）与 Few-Shot 示例
	prompt := `你是一个结构化数据提取助手。请从用户的提问中提取查询关键词、卷号和章节号。
规则：
1. volume_num: 提取"卷"的信息（如：第四卷、卷4）。若未提取到则返回 0。
2. chapter_num: 提取"章/回/节"的信息（如：第500章、五百回）。若未提取到则返回 0。
3. keywords: 除去卷和章信息后的核心查询关键词。
4. 所有的中文数字（如：第四卷、第五百回）必须转换为阿拉伯数字整数（4, 500）。
5. 必须仅返回 JSON 格式数据。

示例：
问题："凡人修仙传第四卷风起海外第五百章讲了什么？"
输出：{"keywords": "讲了什么", "volume_num": 4, "chapter_num": 500}

问题："斗罗大陆第10章唐三的魂环"
输出：{"keywords": "唐三的魂环", "volume_num": 0, "chapter_num": 10}`

	// 2. 根据知识库绑定的配置，获取对应的大模型（LLM）对话客户端实例
	chatModel, err := s.getChatModel(kb.ChatModelName, kb.ChatModelProvider)
	if err != nil {
		logs.Errorf("getChatModel 获取对话模型失败: %v", err)
		return nil, err
	}

	// 3. 组装消息历史并调用大模型进行文本生成
	// 包含 System 设定和 User 实际输入
	message, err := chatModel.Generate(ctx, []*schema.Message{
		{
			Role:    schema.System, // 系统提示词
			Content: prompt,
		},
		{
			Role:    schema.User, // 用户提示词
			Content: query,
		},
	})
	if err != nil {
		logs.Errorf("generate 模型生成失败: %v", err)
		return nil, err
	}

	// 4. 对大模型返回的文本进行清洗
	// 防御性编程：很多大模型即使被要求只返回 JSON，仍会顽固地包裹 Markdown 代码块标签（如 ```json ... ```）
	// 通过前后缀裁剪，剥离标签，确保拿到的是纯净的 JSON 字符串
	rawJSON := message.Content
	rawJSON = strings.TrimPrefix(rawJSON, "```json")
	rawJSON = strings.TrimPrefix(rawJSON, "```")
	rawJSON = strings.TrimSuffix(rawJSON, "```")
	rawJSON = strings.TrimSpace(rawJSON)

	// 5. 将清洗后的 JSON 反序列化进结构体中
	var intent QueryIntent
	if err := json.Unmarshal([]byte(rawJSON), &intent); err != nil {
		logs.Errorf("json.Unmarshal 解析失败: %v", err)
		// 【容错/优雅降级】：如果大模型由于幻觉返回了错乱格式导致反序列化失败，
		// 系统不报错崩溃，而是兜底将用户的完整提问原样作为关键词，卷/章设为0，确保后续的检索主流程仍能运行
		return &QueryIntent{Keywords: query, VolumeNum: 0, ChapterNum: 0}, nil
	}

	// 6. 再次容错：如果大模型提取出的核心关键词为空（比如过滤得太干净了），则将原提问赋值给它以防后续无法做向量检索
	if intent.Keywords == "" {
		intent.Keywords = query
	}

	// 7. 成功返回解析出的结构化查询意图
	return &intent, nil
}

// getChatModel 根据提供商类型构造用于查询意图分析的聊天模型客户端。
func (s *service) getChatModel(modelName string, modelProvider string) (aiModel.ToolCallingChatModel, error) {
	ctx := context.Background()
	var chatModel aiModel.ToolCallingChatModel
	var err error
	//获取提供商以及模型信息
	chatProviderConfig, err := s.getProviderConfig(ctx, model.LLMTypeChat, modelProvider, modelName)
	if err != nil {
		logs.Errorf("获取模型配置失败: %v", err)
		return nil, err
	}
	if chatProviderConfig == nil {
		return nil, biz.ErrProviderConfigNotFound
	}
	if chatProviderConfig.Provider == model.OllamaProvider {
		chatModel, err = ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
			Model:   modelName,
			BaseURL: chatProviderConfig.APIBase,
		})
	} else if chatProviderConfig.Provider == model.QwenProvider {
		chatModel, err = qwen.NewChatModel(ctx, &qwen.ChatModelConfig{
			Model:   modelName,
			BaseURL: chatProviderConfig.APIBase,
			APIKey:  chatProviderConfig.APIKey,
		})
	} else {
		chatModel, err = openai.NewChatModel(ctx, &openai.ChatModelConfig{
			Model:   modelName,
			BaseURL: chatProviderConfig.APIBase,
			APIKey:  chatProviderConfig.APIKey,
		})
	}
	return chatModel, err
}

// getProviderConfig 通过内部事件读取模型提供商配置，避免知识库模块直接依赖 LLM 仓储实现。
func (s *service) getProviderConfig(ctx context.Context, llmType model.LLMType, provider string, name string) (*model.ProviderConfig, error) {
	trigger, err := event.Trigger("getProviderConfig", &shared.GetProviderConfigsRequest{
		Provider:  provider,
		ModelName: name,
		LLMType:   llmType,
	})
	if err != nil {
		logs.Errorf("触发getProviderConfig事件失败: %v", err)
		return nil, errs.DBError
	}
	result := trigger.(*model.ProviderConfig)
	return result, nil
}

// deleteMilvusIndex 删除指定文档在知识库对应 Milvus 集合中的全部向量。
func (s *service) deleteMilvusIndex(ctx context.Context, kbId uuid.UUID, docId uuid.UUID) error {
	index := s.buildIndex(kbId)
	expr := fmt.Sprintf("doc_id=='%s'", docId.String())
	err := s.milvusClient.Delete(ctx, index, "", expr)
	if err != nil {
		if errors.Is(err, client.ErrCollectionNotExists{}) {
			return nil
		}
		logs.Errorf("删除milvus索引失败: %v", err)
		return err
	}
	return nil
}

// deduplicateParents 保持原始顺序去重父分片 ID，避免一次召回重复回填同一段内容。
func deduplicateParents(parents []string) []string {
	seen := make(map[string]struct{})
	var result []string
	for _, p := range parents {
		key := strings.TrimSpace(p)
		if len(key) < 20 {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, p)
	}
	return result
}

// lookLikeSentenceEnd 判断文本块末尾是否像一句完整自然语言，用于优化切分边界。
func lookLikeSentenceEnd(block string) bool {
	return regexp.MustCompile(`[。！？.!?]$`).MatchString(block)
}

// looksLikeCode 粗略识别代码块，避免按普通自然语言规则错误切断代码。
func looksLikeCode(block string) bool {
	return strings.Contains(block, "package ") ||
		strings.Contains(block, "func ") ||
		strings.Contains(block, "class ") ||
		strings.Contains(block, "def ") ||
		strings.Contains(block, "import ") ||
		strings.Contains(block, "from ") ||
		strings.Contains(block, "using ") ||
		strings.Contains(block, "namespace ") ||
		strings.Contains(block, "struct ") ||
		strings.Contains(block, "interface ") ||
		strings.Contains(block, "enum ") ||
		strings.Contains(block, "{ ") ||
		strings.Contains(block, "}")
}

// newService 初始化仓储与外部检索客户端；客户端初始化失败时会记录日志并以 nil 客户端继续返回。
func newService() *service {
	conf := config.GetConfig()

	// 从配置读取 Elasticsearch 配置
	esConfig := elasticsearch.Config{
		Addresses: conf.Elasticsearch.GetAddresses(),
	}
	if conf.Elasticsearch.GetUsername() != "" {
		esConfig.Username = conf.Elasticsearch.GetUsername()
	}
	if conf.Elasticsearch.GetPassword() != "" {
		esConfig.Password = conf.Elasticsearch.GetPassword()
	}
	if conf.Elasticsearch.GetAPIKey() != "" {
		esConfig.APIKey = conf.Elasticsearch.GetAPIKey()
	}

	esClient, err := elasticsearch.NewClient(esConfig)
	if err != nil {
		logs.Errorf("elasticsearch client init error: %v", err)
		panic(err)
	}

	// 从配置读取 Milvus 配置
	milvusConfig := client.Config{
		Address: conf.Milvus.GetAddress(),
	}
	if conf.Milvus.GetDBName() != "" {
		milvusConfig.DBName = conf.Milvus.GetDBName()
	}
	if conf.Milvus.GetUsername() != "" {
		milvusConfig.Username = conf.Milvus.GetUsername()
	}
	if conf.Milvus.GetPassword() != "" {
		milvusConfig.Password = conf.Milvus.GetPassword()
	}

	milvusClient, err := client.NewClient(context.Background(), milvusConfig)
	if err != nil {
		logs.Errorf("milvus client init error: %v", err)
		panic(err)
	}

	logs.Infof("elasticsearch and milvus client init success, es: %v, milvus: %s",
		conf.Elasticsearch.GetAddresses(), conf.Milvus.GetAddress())

	return &service{
		repo:         newModels(database.GetPostgresDB().GormDB),
		esClient:     esClient,
		milvusClient: milvusClient,
	}
}

// Close 关闭 Milvus 等需要显式释放的外部资源。
func (s *service) Close() error {
	if s.milvusClient != nil {
		return s.milvusClient.Close()
	}
	return nil
}

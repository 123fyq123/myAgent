package kbs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino-ext/components/indexer/milvus"
	reMilvus "github.com/cloudwego/eino-ext/components/retriever/milvus"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/mszlu521/thunder/logs"
)

const dim = 768 //要和向量模型保持一致 要不然查询不出来

type MilvusVectorStore struct {
	indexer   *milvus.Indexer
	retriever *reMilvus.Retriever
}

func NewMilvusVectorStore(
	ctx context.Context,
	c client.Client,
	collectionName string,
	embedder embedding.Embedder,
) (*MilvusVectorStore, error) {
	//先创建collection
	err := ensureMilvusCollection(ctx, c, collectionName)
	if err != nil {
		return nil, err
	}
	//创建eino的milvus indexer
	indexer, err := milvus.NewIndexer(ctx, &milvus.IndexerConfig{
		Client:     c,
		Collection: collectionName,
		MetricType: milvus.COSINE,
		DocumentConverter: func(ctx context.Context, docs []*schema.Document, vectors [][]float64) ([]interface{}, error) {
			rows := make([]interface{}, len(docs))
			for i, doc := range docs {
				vec32 := make([]float32, len(vectors[i]))
				for j, v := range vectors[i] {
					vec32[j] = float32(v)
				}
				rows[i] = map[string]interface{}{
					"id":        doc.ID,
					"parent_id": doc.MetaData["parent_id"],
					"doc_id":    doc.MetaData["doc_id"],
					"content":   doc.Content,
					"vector":    vec32,
					"metadata":  doc.MetaData,
				}
			}
			return rows, nil
		},
		Fields: []*entity.Field{
			{
				Name:     "id",
				DataType: entity.FieldTypeVarChar,
				TypeParams: map[string]string{
					"max_length": "128",
				},
			},
			{
				Name:     "parent_id",
				DataType: entity.FieldTypeVarChar,
				TypeParams: map[string]string{
					"max_length": "128",
				},
			},
			{
				Name:     "doc_id",
				DataType: entity.FieldTypeVarChar,
				TypeParams: map[string]string{
					"max_length": "128",
				},
			},
			{
				Name:     "content",
				DataType: entity.FieldTypeVarChar,
				TypeParams: map[string]string{
					"max_length": "8192",
				},
			},
			{
				Name:     "vector",
				DataType: entity.FieldTypeFloatVector,
				TypeParams: map[string]string{
					"dim": fmt.Sprintf("%d", dim),
				},
			},
			{
				Name:     "metadata",
				DataType: entity.FieldTypeJSON,
			},
		},
		Embedding: embedder,
	})
	if err != nil {
		logs.Errorf("create indexer error: %v", err)
		return nil, err
	}
	//创建eino的milvus retriever
	param, err := entity.NewIndexHNSWSearchParam(64)
	if err != nil {
		return nil, err
	}
	retriever, err := reMilvus.NewRetriever(ctx, &reMilvus.RetrieverConfig{
		Client:      c,
		Collection:  collectionName,
		MetricType:  entity.COSINE,
		VectorField: "vector",
		OutputFields: []string{
			"id",
			"content",
			"metadata",
		},
		Sp: param,
		VectorConverter: func(ctx context.Context, vectors [][]float64) ([]entity.Vector, error) {
			res := make([]entity.Vector, len(vectors))
			for i, vector := range vectors {
				fv := make([]float32, len(vector))
				for j := range vector {
					fv[j] = float32(vector[j])
				}
				res[i] = entity.FloatVector(fv)
			}
			return res, nil
		},
		DocumentConverter: func(ctx context.Context, result client.SearchResult) ([]*schema.Document, error) {
			docs := make([]*schema.Document, 0)
			//获取各字段数据
			idColumn, ok := result.Fields.GetColumn("id").(*entity.ColumnVarChar)
			if !ok {
				return nil, fmt.Errorf("id column not found")
			}
			contentColumn, ok := result.Fields.GetColumn("content").(*entity.ColumnVarChar)
			if !ok {
				return nil, fmt.Errorf("content column not found")
			}
			metadataColumn, ok := result.Fields.GetColumn("metadata").(*entity.ColumnJSONBytes)
			if !ok {
				return nil, fmt.Errorf("metadata column not found")
			}
			//构建文档列表
			for i := 0; i < result.ResultCount; i++ {
				doc := &schema.Document{}
				id, err := idColumn.ValueByIdx(i)
				if err != nil {
					continue
				}
				doc.ID = id
				content, err := contentColumn.ValueByIdx(i)
				if err != nil {
					continue
				}
				doc.Content = content
				metadataStr, err := metadataColumn.ValueByIdx(i)
				if err != nil {
					continue
				}
				var metadata map[string]interface{}
				err = json.Unmarshal(metadataStr, &metadata)
				if err != nil {
					metadata = make(map[string]interface{})
				}
				doc.MetaData = metadata
				//设置分数
				if i < len(result.Scores) {
					doc.WithScore(float64(result.Scores[i]))
				}
				docs = append(docs, doc)
			}
			return docs, nil
		},
		Embedding: embedder,
	})
	if err != nil {
		logs.Errorf("create retriever error: %v", err)
		return nil, err
	}
	return &MilvusVectorStore{
		indexer:   indexer,
		retriever: retriever,
	}, nil
}
func (s *MilvusVectorStore) Store(ctx context.Context, docs []*schema.Document) error {
	//这里分批插入
	const batchSize = 10
	total := len(docs)
	if total == 0 {
		return nil
	}
	for i := 0; i < total; i += batchSize {
		end := i + batchSize
		if end > total {
			end = total
		}
		_, err := s.indexer.Store(ctx, docs[i:end])
		if err != nil {
			return err
		}
	}
	return nil
}

// Search 实现了向量数据库存储引擎的接口方法：在 Milvus 中执行向量搜索（支持标量元数据硬过滤）
func (s *MilvusVectorStore) Search(ctx context.Context, query string, topK int, filters SearchFilter) ([]*schema.Document, error) {

	// 1. 构建 Milvus 专属的过滤表达式（Scalar Filtering / 标量过滤）
	// 核心考量：如果上层传入了按章节号（chapter_num）或卷号（volume_num）的过滤要求，
	// 需要将其转换为 Milvus 看得懂的布尔表达式字符串（例如: "chapter_num == 100"）。
	var expr string
	if len(filters) > 0 {
		expr = s.buildMilvusFilter(filters)
	}

	// 2. 初始化 Milvus 检索器的配置项切片，默认注入 topK 参数（限制最高召回多少条最相似的子文档）
	options := []retriever.Option{
		retriever.WithTopK(topK),
	}

	// 3. 动态注入过滤表达式
	// 如果表达式不为空，将其作为过滤选项（WithFilter）追加到配置项切片中
	// 提示：Milvus 会在做向量相似度计算的同时/之前，利用这个表达式进行硬过滤，确保结果绝对准确
	if expr != "" {
		options = append(options, reMilvus.WithFilter(expr))
	}

	// 4. 调用底层封装好的检索器（Retriever）执行真正的搜索
	// 它在内部会干两件事：
	//   a. 调用之前 New 传进来的 Embedder 大模型，把 query 文本转成向量。
	//   b. 拿着向量和 options 参数去 Milvus 数据库里“以向量找向量”，最终返回匹配到的文档切片列表。
	return s.retriever.Retrieve(ctx, query, options...)
}

// buildMilvusFilter 内部辅助方法：将通用的过滤 Map 转换为 Milvus 特定的标量过滤表达式字符串（布尔表达式）
func (s *MilvusVectorStore) buildMilvusFilter(filters SearchFilter) string {
	// 1. 初始化一个切片，用于存放每个独立元数据条件生成的 SQL 片段
	expr := make([]string, 0)

	// 2. 遍历传入的过滤条件，利用 Go 的类型断言（Type Switch）动态判断数据类型，拼装对应的语法
	// 提示：Milvus 的 JSON/Map 类型字段查询语法通常是 metadata['key'] == value
	for key, value := range filters {
		switch v := value.(type) {
		case string:
			// 字符串类型：值需要用单引号包裹（例如: metadata['author'] == '韩立'）
			expr = append(expr, fmt.Sprintf("metadata['%s'] == '%s'", key, v))
		case int, int32, int64, uint, uint32, uint64:
			// 整数类型：直接输出数字，使用 %d 格式化（例如: metadata['chapter_num'] == 100）
			expr = append(expr, fmt.Sprintf("metadata['%s'] == %d", key, v))
		case float32, float64:
			// 浮点数类型：直接输出数字，使用 %f 格式化
			expr = append(expr, fmt.Sprintf("metadata['%s'] == %f", key, v))
		case bool:
			// 布尔类型：转化为 true/false 字符串，使用 %t 格式化（例如: metadata['is_deleted'] == false）
			expr = append(expr, fmt.Sprintf("metadata['%s'] == %t", key, v))
		}
	}

	// 3. 边界判断：如果没有任何合法的过滤条件，返回空字符串
	if len(expr) == 0 {
		return ""
	}

	// 4. 如果只有一个过滤条件，直接返回该表达式，无需拼接
	if len(expr) == 1 {
		return expr[0]
	}

	// 5. 如果存在多个过滤条件，使用 " AND " 关键字将它们串联起来
	// 最终生成如: "metadata['chapter_num'] == 500 AND metadata['volume_num'] == 4" 的复合表达式
	result := expr[0]
	for i := 1; i < len(expr); i++ {
		result += " AND " + expr[i]
	}

	// 6. 打印最终生成的 Milvus 过滤日志，方便日常排查和调优
	logs.Infof("milvus filter: %s", result)

	return result
}
func ensureMilvusCollection(ctx context.Context, client client.Client, collectionName string) error {
	//先判断collection是否存在
	has, err := client.HasCollection(ctx, collectionName)
	if err != nil {
		return err
	}
	if has {
		//如果已经存在 就直接返回就行
		return nil
	}
	collectionSchema := &entity.Schema{
		CollectionName: collectionName,
		AutoID:         true,
		Fields: []*entity.Field{
			{
				Name:       "id",
				DataType:   entity.FieldTypeVarChar,
				PrimaryKey: true,
				TypeParams: map[string]string{
					"max_length": "128",
				},
			},
			{
				Name:     "parent_id",
				DataType: entity.FieldTypeVarChar,
				TypeParams: map[string]string{
					"max_length": "128",
				},
			},
			{
				Name:     "doc_id",
				DataType: entity.FieldTypeVarChar,
				TypeParams: map[string]string{
					"max_length": "128",
				},
			},
			{
				Name:     "content",
				DataType: entity.FieldTypeVarChar,
				TypeParams: map[string]string{
					"max_length": "8192",
				},
			},
			{
				Name:     "vector",
				DataType: entity.FieldTypeFloatVector,
				TypeParams: map[string]string{
					"dim": fmt.Sprintf("%d", dim),
				},
			},
			{
				Name:     "metadata",
				DataType: entity.FieldTypeJSON,
				TypeParams: map[string]string{
					"max_length": "4096",
				},
			},
		},
	}
	err = client.CreateCollection(ctx, collectionSchema, 2)
	if err != nil {
		return err
	}
	//创建向量索引
	hnswIndex, err := entity.NewIndexHNSW(entity.COSINE, 16, 200)
	if err != nil {
		return err
	}
	err = client.CreateIndex(ctx, collectionName, "vector", hnswIndex, false)
	if err != nil {
		return err
	}
	err = client.LoadCollection(ctx, collectionName, false)
	return err
}

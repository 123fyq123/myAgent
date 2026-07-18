package knowledges

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mszlu521/thunder/errs"
	"github.com/mszlu521/thunder/logs"
	"github.com/mszlu521/thunder/req"
	"github.com/mszlu521/thunder/res"
)

type Handler struct {
	service *service
}

// CreateKnowledgeBase 处理创建知识库的客户端请求
func (h *Handler) CreateKnowledgeBase(c *gin.Context) {
	// 1. 解析并绑定前端传入的 JSON 请求参数
	var createReq createKnowledgeBaseReq
	if err := req.JsonParam(c, &createReq); err != nil {
		// 参数校验或解析失败，req.JsonParam 内部已封装错误响应，直接拦截返回
		return
	}

	// 2. 从上下文获取当前登录用户的 UUID
	userId, ok := req.GetUserIdUUID(c)
	if !ok {
		// 用户未登录或 Token 无效，内部已封装鉴权失败响应，直接拦截返回
		return
	}

	// 3. 调用 Service 层核心业务逻辑，处理创建知识库的核心流程
	resp, err := h.service.createKnowledgeBase(c.Request.Context(), userId, createReq)
	if err != nil {
		// 4a. 业务层执行失败（如名称重复、底层存储异常等），向前端返回统一的错误格式
		res.Error(c, err)
		return
	}

	// 4b. 创建成功，向前端返回标准的成功响应及数据
	res.Success(c, resp)
}

// ListKnowledgeBases 处理获取知识库列表（或搜索知识库）的客户端请求
func (h *Handler) ListKnowledgeBases(c *gin.Context) {
	// 1. 解析并绑定前端传入的 JSON 查询参数（如分页、关键词等）
	var params searchReq
	if err := req.JsonParam(c, &params); err != nil {
		// 参数解析失败，req.JsonParam 内部已封装错误响应，直接拦截返回
		return
	}

	// 2. 从上下文获取当前登录用户的 UUID，以确保用户只能查询到属于自己的知识库
	userId, ok := req.GetUserIdUUID(c)
	if !ok {
		// 用户未登录或 Token 无效，内部已封装鉴权失败响应，直接拦截返回
		return
	}

	// 3. 调用 Service 层业务逻辑，传入链路上下文、用户ID及具体的查询参数
	resp, err := h.service.listKnowledgeBases(c.Request.Context(), userId, params.Params)
	if err != nil {
		// 4a. 业务层查询失败（如数据库异常等），向前端返回统一的错误格式
		res.Error(c, err)
		return
	}

	// 4b. 查询成功，向前端返回标准的成功响应及查询到的知识库列表数据
	res.Success(c, resp)
}

// GetKnowledgeBase 处理获取单个知识库详情的客户端请求
func (h *Handler) GetKnowledgeBase(c *gin.Context) {
	// 1. 从 URL 路径参数中解析知识库的唯一 ID (UUID 格式，例如: /knowledge/:id)
	var id uuid.UUID
	if err := req.Path(c, "id", &id); err != nil {
		// 路径参数解析或类型转换失败，req.Path 内部已封装错误响应，直接拦截返回
		return
	}

	// 2. 从上下文获取当前登录用户的 UUID，用于后续的权限校验（确保用户只能查看自己的知识库）
	userId, ok := req.GetUserIdUUID(c)
	if !ok {
		// 用户未登录或 Token 无效，内部已封装鉴权失败响应，直接拦截返回
		return
	}

	// 3. 调用 Service 层业务逻辑，传入链路上下文、用户 ID 以及知识库 ID 查询详情
	resp, err := h.service.getKnowledgeBase(c.Request.Context(), userId, id)
	if err != nil {
		// 4a. 业务层查询失败（如记录不存在、无权访问或数据库异常），向前端返回统一的错误格式
		res.Error(c, err)
		return
	}

	// 4b. 查询成功，向前端返回标准的成功响应及知识库详细数据
	res.Success(c, resp)
}

// UpdateKnowledgeBase 解析路径中的知识库 ID 和 JSON 更新字段，再委托 Service 执行权限校验与更新。
func (h *Handler) UpdateKnowledgeBase(c *gin.Context) {
	var updateReq updateKnowledgeBaseReq
	if err := req.JsonParam(c, &updateReq); err != nil {
		return
	}
	userId, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	var id uuid.UUID
	if err := req.Path(c, "id", &id); err != nil {
		return
	}
	resp, err := h.service.updateKnowledgeBase(c.Request.Context(), userId, id, updateReq)
	if err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, resp)
}

// DeleteKnowledgeBase 删除当前用户拥有的知识库元数据；具体级联清理由 Service 层决定。
func (h *Handler) DeleteKnowledgeBase(c *gin.Context) {
	var id uuid.UUID
	if err := req.Path(c, "id", &id); err != nil {
		return
	}
	userId, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	err := h.service.deleteKnowledgeBase(c.Request.Context(), userId, id)
	if err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, nil)
}

// ListDocuments 处理获取某个知识库下文档列表的 HTTP 请求
func (h *Handler) ListDocuments(c *gin.Context) {
	// 1. 解析并绑定 URL 查询参数（例如：分页参数 page、pageSize，或者筛选条件等）
	var params listDocumentReq
	if err := req.QueryParam(c, &params); err != nil {
		// 参数解析失败，req.QueryParam 内部已封装错误响应，直接拦截返回
		return
	}

	// 2. 从上下文中获取当前登录用户的 UUID，用于做数据隔离和权限校验
	userId, ok := req.GetUserIdUUID(c)
	if !ok {
		// 用户未登录或 Token 无效，内部已封装未登录响应，直接拦截返回
		return
	}

	// 3. 从 URL 路径参数中解析知识库的唯一 ID（例如路由可能是 /knowledge-base/:id/documents）
	var kbId uuid.UUID
	if err := req.Path(c, "id", &kbId); err != nil {
		// 路径参数解析或 UUID 类型转换失败，内部已处理响应，直接拦截返回
		return
	}

	// 4. 调用 Service 层核心业务逻辑，传入用户 ID 和知识库 ID，查询属于该用户的文档列表
	resp, err := h.service.listDocuments(c.Request.Context(), userId, kbId, params)
	if err != nil {
		// 5a. 业务层执行失败（如无权访问该知识库、数据库异常等），向前端返回统一的错误格式
		res.Error(c, err)
		return
	}

	// 5b. 查询成功，向前端返回标准的成功响应及文档列表数据
	res.Success(c, resp)
}

// UploadDocuments 接收 multipart 文件，创建文档记录后触发后台解析、切分和向量化任务。
func (h *Handler) UploadDocuments(c *gin.Context) {
	userId, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	var kbId uuid.UUID
	if err := req.Path(c, "id", &kbId); err != nil {
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		res.Error(c, errs.ErrParam)
		return
	}
	resp, err := h.service.uploadDocuments(c.Request.Context(), userId, kbId, file)
	if err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, resp)
}

// DeleteDocuments 删除指定知识库中的一个文档及其关联的关系型、搜索和向量索引数据。
func (h *Handler) DeleteDocuments(c *gin.Context) {
	var kbId uuid.UUID
	if err := req.Path(c, "id", &kbId); err != nil {
		return
	}
	var documentId uuid.UUID
	if err := req.Path(c, "documentId", &documentId); err != nil {
		return
	}
	userId, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	err := h.service.deleteDocuments(c.Request.Context(), userId, kbId, documentId)
	if err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, nil)
}

// SearchKnowledgeBase 处理知识库检索（结合大模型 RAG 对话）的客户端请求
func (h *Handler) SearchKnowledgeBase(c *gin.Context) {
	// 1. 初始化标准库的 HTTP 响应控制器
	rc := http.NewResponseController(c.Writer)

	// 2. 核心优化：取消当前连接的写入超时限制（传入零值 time.Time{}）
	// 核心考量：该接口后续会触发大模型（LLM）对话交互，大模型的推理响应往往耗时较长，
	// 取消超时限制可以防止因为大模型“憋字”时间过长而导致 HTTP 连接被网关或 GIN 强制断开。
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		// 该操作通常不会失败，若失败则记录警告日志，但不影响主流程继续执行
		logs.Warnf("SetWriteDeadline error: %v", err)
	}

	// 3. 解析并绑定前端传入的 JSON 请求体（如检索的 Query 文本、召回阈值等参数）
	var params searchParams
	if err := req.JsonParam(c, &params); err != nil {
		// 参数解析失败，内部已处理异常响应，直接拦截返回
		return
	}

	// 4. 从 URL 路径中解析出目标知识库的唯一 ID
	var kbId uuid.UUID
	if err := req.Path(c, "id", &kbId); err != nil {
		// 路径参数解析失败，直接拦截返回
		return
	}

	// 5. 从上下文中获取当前登录用户的 UUID，用于权限验证，防止非法跨库检索
	userId, ok := req.GetUserIdUUID(c)
	if !ok {
		// 用户身份识别失败，内部已封装鉴权失败响应，直接拦截返回
		return
	}

	// 6. 调用 Service 层核心业务，传入链路上下文、用户与知识库ID以及搜索参数，执行 RAG 检索
	resp, err := h.service.searchKnowledgeBase(c.Request.Context(), userId, kbId, params)
	if err != nil {
		// 7a. 业务层处理失败（如大模型调用异常、向量数据库故障等），向前端返回统一的错误格式
		res.Error(c, err)
		return
	}

	// 7b. 检索与应答成功，向前端返回标准的成功响应及最终生成的回答/参考文档
	res.Success(c, resp)
}

// Close 在 HTTP 服务关闭时释放知识库 service 持有的外部客户端连接。
func (h *Handler) Close() error {
	return h.service.Close()
}

// NewHandler 创建 HTTP Handler，并同时初始化知识库业务 Service。
func NewHandler() *Handler {
	return &Handler{
		service: newService(),
	}
}

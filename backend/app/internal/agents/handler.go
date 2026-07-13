package agents

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mszlu521/thunder/logs"
	"github.com/mszlu521/thunder/req"
	"github.com/mszlu521/thunder/res"
)

type Handler struct {
	service *service
}

// 创建 Agent
func (h *Handler) CreateAgent(c *gin.Context) {
	var createReq CreateAgentReq
	if err := req.JsonParam(c, &createReq); err != nil {
		return
	}
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	//如果需要做链路追踪 上下文要进行传递
	//这个上下文超时是10s
	resp, err := h.service.createAgent(c.Request.Context(), userID, createReq)
	if err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, resp)
}

// 查询 Agent 列表
func (h *Handler) ListAgents(c *gin.Context) {
	var listReq SearchAgentReq
	if err := req.JsonParam(c, &listReq); err != nil {
		return
	}
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	resp, err := h.service.listAgents(c.Request.Context(), userID, listReq)
	if err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, resp)
}

// 获取单个 Agent 详情
func (h *Handler) GetAgent(c *gin.Context) {
	var id uuid.UUID
	if err := req.Path(c, "id", &id); err != nil {
		return
	}
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	resp, err := h.service.getAgent(c.Request.Context(), userID, id)
	if err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, resp)
}

// 更新 Agent 配置
func (h *Handler) UpdateAgent(c *gin.Context) {
	var updateReq UpdateAgentReq
	if err := req.JsonParam(c, &updateReq); err != nil {
		return
	}
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	resp, err := h.service.updateAgent(c.Request.Context(), userID, updateReq)
	if err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, resp)
}

// 处理AI智能体对话的HTTP接口，采用了SSE（Server-Sent Events）流式响应技术。
func (h *Handler) AgentMessage(c *gin.Context) {
	//获取参数
	var messageReq AgentMessageReq
	if err := req.JsonParam(c, &messageReq); err != nil {
		return
	}
	userID, exist := req.GetUserIdUUID(c)
	if !exist {
		return
	}
	//这里需要注意 AI回答时间比较长，所以这里不能设置限制,全局是10s超时，这里单独设置
	rc := http.NewResponseController(c.Writer)
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		//一般不会失败
		logs.Warnf("SetWriteDeadline error: %v", err)
	}
	//SSE的响应，所以需要设置SSE的响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	//这里我们用一个可以取消的context
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	//这个接口是AI回答，我们返回两个chan，一个datachan 一个errchan
	//调用大模型 我们需要放在协程处理，所以这里用channel
	datachan, errchan := h.service.agentMessage(ctx, userID, messageReq)
	//创建一个心跳 这里是防止一些防火墙拦截 导致连接中断
	heartbeat := time.NewTicker(time.Second * 5)
	defer heartbeat.Stop()
	for {
		//处理数据
		select {
		case <-ctx.Done():
			logs.Warnf("context done, 客户端断开连接")
			return
		case <-heartbeat.C:
			//处理心跳 我们发送一个冒号开头的消息 表示这是一个心跳消息
			_, err := c.Writer.Write([]byte(": keep-alive\n\n"))
			if err != nil {
				logs.Warnf("write heartbeat error: %v", err)
				cancel()
				return
			}
			//在go中处理消息 如果想要立即发送给客户端需要调用Flush
			c.Writer.Flush()

		// datachan 的内容主要在 agentMessage 函数的事件处理循环中，通过 sendData 函数写入，数据来源是AI模型的流式响应事件，格式为统一的JSON消息结构。
		case data, ok := <-datachan:
			if !ok {
				//这里代表channel被关闭了 也就是消息结束了
				//按照SSE的规范，发送一个结束消息 [DONE]
				_, err := c.Writer.Write([]byte("data: [DONE]\n\n"))
				if err != nil {
					logs.Warnf("write done error: %v", err)
				}
				c.Writer.Flush()
				return
			}
			//有消息就直接发送， 这里我们不区分event 都按照默认message进行处理，前端也是如此
			//data数据是json的格式
			_, err := c.Writer.Write([]byte("data: " + data + "\n\n"))
			if err != nil {
				logs.Errorf("write data error: %v", err)
				cancel()
				return
			}
			// 强制让服务器把当前内存缓冲区里攒着的数据，立刻、马上通过网络冲刷给前端浏览器，而不是留在后端干等。
			c.Writer.Flush()
		case err, ok := <-errchan:
			if !ok {
				//error的消息结束不处理，交给datachan处理
				errchan = nil
				continue
			}
			//如果有错误 发送错误的消息提供给客户端
			if err != nil {
				_, err := c.Writer.Write([]byte("data: [ERROR]" + err.Error() + "\n\n"))
				if err != nil {
					logs.Errorf("write error error: %v", err)
					cancel()
					return
				}
				c.Writer.Flush()
				return
			}
		}
	}

}

// 更新agent的tools
func (h *Handler) UpdateAgentTool(c *gin.Context) {
	// 1. 从 URL 路径参数中提取智能体的 ID (比如请求路径是 /agents/:id/tools)
	var id uuid.UUID
	if err := req.Path(c, "id", &id); err != nil {
		return // 如果提取失败（例如格式不是合法的 UUID），封装的 req.Path 内部通常已经返回了 400 错误，这里直接退出
	}

	// 2. 从 HTTP 请求体（Body）中解析出前端传过来的 JSON 数据
	var updateReq UpdateAgentToolReq
	if err := req.JsonParam(c, &updateReq); err != nil {
		return // 如果 JSON 格式错误或缺少必要参数，直接拦截并退出
	}

	// 3. 从 Gin 的上下文 c 中获取当前处于登录状态的用户 ID
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return // 如果没获取到，说明用户未登录或 Token 失效，直接拦截
	}

	// 4. 核心：将校验完的数据打包，向下传递给 Service（业务逻辑）层执行
	//    传入 Context、当前操作人ID、要修改的智能体ID、以及具体要更新的工具列表数据
	resp, err := h.service.updateAgentTool(c.Request.Context(), userID, id, updateReq)

	// 5. 统一的响应处理
	if err != nil {
		res.Error(c, err) // 如果业务层报错（比如：智能体不存在、用户越权修改别人的智能体、或者数据库挂了），返回对应的错误 JSON
		return
	}

	res.Success(c, resp) // 如果一切顺利，使用包装好的标准成功格式（通常是 {"code": 200, "data": ...}）回给前端
}

func (h *Handler) AddAgentKnowledgeBase(c *gin.Context) {
	var agentId uuid.UUID
	if err := req.Path(c, "id", &agentId); err != nil {
		return
	}
	var addReq addAgentKnowledgeBaseReq
	if err := req.JsonParam(c, &addReq); err != nil {
		return
	}
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	resp, err := h.service.addAgentKnowledgeBase(c.Request.Context(), userID, agentId, addReq)
	if err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, resp)
}

func (h *Handler) DeleteAgentKnowledgeBase(c *gin.Context) {
	var agentId uuid.UUID
	if err := req.Path(c, "id", &agentId); err != nil {
		return
	}
	var kbId uuid.UUID
	if err := req.Path(c, "kbId", &kbId); err != nil {
		return
	}
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	resp, err := h.service.deleteAgentKnowledgeBase(c.Request.Context(), userID, agentId, kbId)
	if err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, resp)
}

func NewHandler() *Handler {
	return &Handler{
		service: newService(),
	}
}

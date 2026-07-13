package router

import (
	"app/internal/agents"

	"github.com/gin-gonic/gin"
)

type AgentRouter struct {
}

func (u *AgentRouter) Register(engine *gin.Engine) {
	agentsGroup := engine.Group("/api/v1/agents")
	{
		agentsHandler := agents.NewHandler()
		// ----------------- 【智能体 基础增删改查】 -----------------
		// 注册创建接口：接收 POST 请求。用于在系统内全新创建一个 AI 智能体
		agentsGroup.POST("/create", agentsHandler.CreateAgent)
		// 注册列表接口：接收 POST 请求。
		// 这里列表用 POST 是为了方便前端在请求体（Body）中传入复杂的筛选条件、分页参数或排序规则
		agentsGroup.POST("/list", agentsHandler.ListAgents)
		// 这里的 : 是 Gin 框架中的通配符标记，代表这是一个动态参数。
		// 如果客户端请求 /agents/abc，那么 id 的值就是 "abc"。
		// 如果客户端请求 /agents/999，那么 id 的值就是 "999"。
		agentsGroup.GET("/:id", agentsHandler.GetAgent)
		// POST 的语义是“创建（Create）”：通常用于在服务器上新建一个之前不存在的资源。比如你创建一个新的智能体，通常会用 POST /agents。
		// PUT 的语义是“更新/替换（Update/Replace）”：用于修改服务器上已经存在的资源。
		agentsGroup.PUT("/update", agentsHandler.UpdateAgent)

		// ----------------- 【智能体 绑定外部工具 (Tools)】 -----------------
		// 注册对话接口：接收 POST 请求。
		// 也就是你前面看过的核心流式打字机接口，前端把问题发过来，后端通过连接启动 SSE 实时吐字
		agentsGroup.POST("/chat", agentsHandler.AgentMessage)
		// 注册工具批量更新接口：接收 POST 请求。
		// 路径中的 :id 动态指明要改哪个智能体。用于一次性重置或批量修改该智能体支持的插件/工具箱
		agentsGroup.POST("/:id/tools/batch", agentsHandler.UpdateAgentTool)
		// 注册添加知识库接口：接收 POST 请求。
		// 路径指明智能体 ID，Body 里传入知识库 ID。用于把现有的某个本地知识库挂载给该智能体，使其具备 RAG 能力
		agentsGroup.POST("/:id/knowledge-bases", agentsHandler.AddAgentKnowledgeBase)
		// 采用了典型的 RESTful 多级路径嵌套：通过 :id 定位智能体，通过 :kbId 定位要移除的知识库。
		// 语义非常明确：从指定的智能体中，删除/解绑指定的知识库
		agentsGroup.DELETE("/:id/knowledge-bases/:kbId", agentsHandler.DeleteAgentKnowledgeBase)
	}
}

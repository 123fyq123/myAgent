package agents

import (
	"context"
	"model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type repository interface {
	// =================================================================
	// 一、 Agent 基础元数据管理 (CRUD)
	// =================================================================

	// createAgent 创建一个新的 Agent 基础记录
	createAgent(ctx context.Context, agent *model.Agent) error

	// listAgents 根据用户ID和过滤条件（如分页、关键词），获取 Agent 列表及总记录数
	listAgents(ctx context.Context, userID uuid.UUID, filter AgentFilter) ([]*model.Agent, int64, error)

	// getAgent 获取指定用户下某个具体 Agent 的详情（带用户鉴权隔离）
	getAgent(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*model.Agent, error)

	// getAgentById 纯粹通过 Agent ID 获取详情（通常用于内部服务间调用，不校验所属用户）
	getAgentById(ctx context.Context, id uuid.UUID) (*model.Agent, error)

	// updateAgent 更新 Agent 的基础配置信息（如名称、描述、模型参数等）
	updateAgent(ctx context.Context, agent *model.Agent) error

	// deleteAgent 硬删除或软删除一个 Agent 基础记录
	deleteAgent(ctx context.Context, id uuid.UUID) error

	// =================================================================
	// 二、 Agent 工具绑定 (Tools)
	// =================================================================

	// createAgentTools 批量为 Agent 绑定外部工具（如 API 调用、计算器等）
	createAgentTools(ctx context.Context, tools []*model.AgentTool) error

	// deleteAgentTool 从指定 Agent 上解绑某一个特定的工具
	deleteAgentTool(ctx context.Context, agentId uuid.UUID, toolId uuid.UUID) error

	// deleteAgentTools 解绑指定 Agent 下挂载的所有工具（通常在删除 Agent 时级联调用）
	deleteAgentTools(ctx context.Context, agentId uuid.UUID) error

	// =================================================================
	// 三、 Agent 知识库绑定 (Knowledge Base)
	// =================================================================

	// isAgentKnowledgeBaseExist 检查某个知识库是否已经与该 Agent 建立了绑定关系
	isAgentKnowledgeBaseExist(ctx context.Context, agentId uuid.UUID, knowledgeBaseID uuid.UUID) (bool, error)

	// createAgentKnowledgeBase 为 Agent 绑定一个现有的知识库资产
	createAgentKnowledgeBase(ctx context.Context, ab *model.AgentKnowledgeBase) error

	// deleteAgentKnowledgeBase 从指定 Agent 上解绑某一个特定的知识库
	deleteAgentKnowledgeBase(ctx context.Context, agentId uuid.UUID, kbId uuid.UUID) error

	// deleteAgentKnowledgeBaseByAgentId 解绑指定 Agent 下的所有知识库（级联删除）
	deleteAgentKnowledgeBaseByAgentId(ctx context.Context, agentId uuid.UUID) error

	// =================================================================
	// 四、 Agent 市场/子Agent 嵌套绑定 (Agent in Agent)
	// =================================================================

	// getAgentAgent 查询 Agent 是否嵌套或组合了另一个市场上的子 Agent
	getAgentAgent(ctx context.Context, agentId uuid.UUID, marketId uuid.UUID) (*model.AgentAgent, error)

	// createAgentAgent 为当前 Agent 组合/添加一个子 Agent 能力
	createAgentAgent(ctx context.Context, agentAgent *model.AgentAgent) error

	// deleteAgentAgent 从当前 Agent 中移除某个子 Agent 的组合关系
	deleteAgentAgent(ctx context.Context, agentId uuid.UUID, agentMarketId uuid.UUID) error

	// deleteAgentAgentByAgentId 移除指定 Agent 下挂载的所有子 Agent 组合关系（级联删除）
	deleteAgentAgentByAgentId(ctx context.Context, agentId uuid.UUID) error

	// =================================================================
	// 五、 Agent 工作流绑定 (Workflow)
	// =================================================================

	// getAgentWorkflow 查询 Agent 与某个特定工作流的绑定关系
	getAgentWorkflow(ctx context.Context, agentId uuid.UUID, workflowID uuid.UUID) (*model.AgentWorkflow, error)

	// createAgentWorkflow 为 Agent 绑定一个既定的自动化工作流
	createAgentWorkflow(ctx context.Context, workflow *model.AgentWorkflow) error

	// deleteAgentWorkflow 从 Agent 上解绑某个特定的工作流
	deleteAgentWorkflow(ctx context.Context, agentId uuid.UUID, workflowId uuid.UUID) error

	// deleteAgentWorkflowByAgentId 解绑指定 Agent 下挂载的所有工作流（级联删除）
	deleteAgentWorkflowByAgentId(ctx context.Context, agentId uuid.UUID) error

	// =================================================================
	// 六、 Agent 技能资产绑定 (Skill)
	// =================================================================

	// getAgentSkill 获取 Agent 绑定的某个特定技能详情
	getAgentSkill(ctx context.Context, agentId uuid.UUID, skillID uuid.UUID) (*model.AgentSkill, error)

	// saveAgentSkill 保存或新增 Agent 的技能（通常内部实现含 Upsert 逻辑：不存在则创建，存在则更新）
	saveAgentSkill(ctx context.Context, skill *model.AgentSkill) error

	// updateAgentSkill 显式更新已有的 Agent 技能配置
	updateAgentSkill(ctx context.Context, agentSkill *model.AgentSkill) error

	// deleteAgentSkill 从 Agent 上移除某个特定技能
	deleteAgentSkill(ctx context.Context, agentId uuid.UUID, skillID uuid.UUID) error

	// =================================================================
	// 七、 聊天会话与历史消息管理 (Session & Message)
	// =================================================================

	// createSession 为用户与 Agent 的对话创建一个新的聊天会话（Session）窗口
	createSession(ctx context.Context, session *model.ChatSession) error

	// getSession 获取某个特定聊天会话的元数据详情
	getSession(ctx context.Context, sessionId *uuid.UUID) (*model.ChatSession, error)

	// listSessions 获取指定用户在某个 Agent 下创建的所有历史会话列表
	listSessions(ctx context.Context, userID uuid.UUID, agentId uuid.UUID) ([]*model.ChatSession, error)

	// deleteSession 删除某个特定的聊天会话记录
	deleteSession(ctx context.Context, sessionId uuid.UUID) error

	// getSessionMessages 获取某个会话窗口内的全部聊天历史消息记录（按时间正序，用于上下文气泡渲染）
	getSessionMessages(ctx context.Context, sessionId uuid.UUID) ([]*model.ChatMessage, error)

	// saveChatMessage 持久化一条新的聊天消息（无论是 User 发送的还是 AI 产生的流式最终结果）
	saveChatMessage(ctx context.Context, chatMessage *model.ChatMessage) error

	// deleteSessionMessages 清空某个特定会话下的所有聊天消息记录（通常与删除会话配合使用）
	deleteSessionMessages(ctx context.Context, sessionId uuid.UUID) error

	// =================================================================
	// 八、 高级事务控制
	// =================================================================

	// transaction 封装 GORM 事务控制。
	// 用于确保多表级联操作的原子性。例如：删除 Agent 时，必须同时成功删除其工具、知识库、工作流等，
	// 传入的闭包函数 f 若返回 error，则整个事务回滚（Rollback），否则提交（Commit）。
	transaction(ctx context.Context, f func(tx *gorm.DB) error) error
}

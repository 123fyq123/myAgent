package router

import (
	"model"

	"github.com/gin-gonic/gin"
	"github.com/mszlu521/thunder/database"
	"github.com/mszlu521/thunder/errs"
	"github.com/mszlu521/thunder/req"
	"github.com/mszlu521/thunder/res"
)

var errCompatNotImplemented = errs.NewError(90001, "当前基础版暂未实现该接口")

type CompatHandler struct {
}

func NewCompatHandler() *CompatHandler {
	return &CompatHandler{}
}

func (h *CompatHandler) Empty(c *gin.Context) {
	res.Success(c, nil)
}

func (h *CompatHandler) NotImplemented(c *gin.Context) {
	res.Error(c, errCompatNotImplemented)
}

func (h *CompatHandler) ListAllAgents(c *gin.Context) {
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	var agents []*model.Agent
	err := database.GetPostgresDB().GormDB.
		Where("creator_id = ?", userID).
		Order("id DESC").
		Find(&agents).Error
	if err != nil {
		res.Error(c, errs.DBError)
		return
	}
	res.Success(c, agents)
}

func (h *CompatHandler) ListAllWorkflows(c *gin.Context) {
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	var workflows []*model.Workflow
	err := database.GetPostgresDB().GormDB.
		Where("user_id = ?", userID).
		Order("id DESC").
		Find(&workflows).Error
	if err != nil {
		res.Error(c, errs.DBError)
		return
	}
	res.Success(c, workflows)
}

func (h *CompatHandler) GetSettings(c *gin.Context) {
	res.Success(c, gin.H{})
}

func (h *CompatHandler) SaveSettings(c *gin.Context) {
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		body = gin.H{}
	}
	res.Success(c, body)
}

func (h *CompatHandler) GetSettingsModule(c *gin.Context) {
	res.Success(c, gin.H{
		"module": c.Param("module"),
		"config": gin.H{},
	})
}

func (h *CompatHandler) ListTemplates(c *gin.Context) {
	res.Success(c, gin.H{
		"list":  []any{},
		"items": []any{},
		"total": 0,
	})
}

func (h *CompatHandler) ListFiles(c *gin.Context) {
	res.Success(c, gin.H{
		"files": []any{},
		"total": 0,
	})
}

func (h *CompatHandler) ListCollections(c *gin.Context) {
	res.Success(c, gin.H{
		"collections": []any{},
		"total":       0,
	})
}

func (h *CompatHandler) ListTables(c *gin.Context) {
	res.Success(c, gin.H{
		"tables": []any{},
		"total":  0,
	})
}

func (h *CompatHandler) SubscriptionPlans(c *gin.Context) {
	res.Success(c, []gin.H{
		{
			"plan": "free",
			"configs": gin.H{
				"maxAgents":            10,
				"maxWorkflows":         10,
				"maxKnowledgeBaseSize": 10,
			},
		},
	})
}

func (h *CompatHandler) PaymentStatus(c *gin.Context) {
	res.Success(c, gin.H{
		"paid": false,
	})
}

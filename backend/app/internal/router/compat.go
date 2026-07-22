package router

import (
	"github.com/gin-gonic/gin"
)

type CompatRouter struct {
}

func (r *CompatRouter) Register(engine *gin.Engine) {
	handler := NewCompatHandler()

	authGroup := engine.Group("/api/v1/auth")
	{
		authGroup.POST("/logout", handler.Empty)
	}

	agentGroup := engine.Group("/api/v1/agents")
	{
		agentGroup.POST("/all", handler.ListAllAgents)
	}

	workflowGroup := engine.Group("/api/v1/workflows")
	{
		workflowGroup.POST("/all", handler.ListAllWorkflows)
	}

	settingsGroup := engine.Group("/api/v1/settings")
	{
		settingsGroup.GET("", handler.GetSettings)
		settingsGroup.POST("", handler.SaveSettings)
		settingsGroup.PUT("", handler.SaveSettings)
		settingsGroup.GET("/:module", handler.GetSettingsModule)
		settingsGroup.PUT("/:module", handler.SaveSettings)
	}

	templateGroup := engine.Group("/api/v1/templates")
	{
		templateGroup.GET("", handler.ListTemplates)
		templateGroup.POST("/workflow/template", handler.NotImplemented)
		templateGroup.POST("/agent/template", handler.NotImplemented)
		templateGroup.POST("/workflow", handler.NotImplemented)
		templateGroup.POST("/agent", handler.NotImplemented)
		templateGroup.POST("/import", handler.NotImplemented)
	}

	storageGroup := engine.Group("/api/v1/storage")
	{
		storageGroup.GET("/files", handler.ListFiles)
		storageGroup.POST("/files", handler.NotImplemented)
		storageGroup.DELETE("/files/batch", handler.NotImplemented)
	}

	vectorGroup := engine.Group("/api/v1/vector-db")
	{
		vectorGroup.GET("/collections", handler.ListCollections)
		vectorGroup.POST("/collections", handler.NotImplemented)
	}

	relationalGroup := engine.Group("/api/v1/relational-db")
	{
		relationalGroup.GET("/tables", handler.ListTables)
		relationalGroup.POST("/tables", handler.NotImplemented)
	}

	subscriptionGroup := engine.Group("/api/v1/subscription")
	{
		subscriptionGroup.GET("/plans", handler.SubscriptionPlans)
		subscriptionGroup.POST("", handler.NotImplemented)
		subscriptionGroup.DELETE("", handler.NotImplemented)
		subscriptionGroup.POST("/wechat/order", handler.NotImplemented)
		subscriptionGroup.GET("/wechat/status/:orderId", handler.PaymentStatus)
	}
}

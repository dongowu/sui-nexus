package gateway

import (
	"github.com/gin-gonic/gin"
	"github.com/sui-nexus/gateway/pkg/hmac"
)

func NewRouter(handler *Handler, signer *hmac.Signer) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(CORSMiddleware())
	r.Use(gin.Logger())

	// Health check (no auth)
	r.GET("/health", handler.HandleHealth)

	// API v1
	v1 := r.Group("/api/v1")
	v1.Use(HMACAuth(signer))
	v1.Use(RateLimit(1000))
	{
		v1.POST("/intent", handler.HandleIntent)
		v1.GET("/task/:task_id", handler.HandleGetTask)
	}

	return r
}
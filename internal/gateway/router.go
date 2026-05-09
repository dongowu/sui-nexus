package gateway

import (
	"github.com/gin-gonic/gin"
	"github.com/sui-nexus/gateway/pkg/hmac"
)

func NewRouter(handler *Handler, signer *hmac.Signer, agentWalletHandler *AgentWalletHandler) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(CORSMiddleware())
	r.Use(gin.Logger())

	// Health check (no auth)
	r.GET("/health", handler.HandleHealth)

	// WebSocket (no auth for demo)
	r.GET("/ws", handler.HandleWebSocket)

	// zkLogin OAuth callback (no auth - OAuth provides its own)
	r.GET("/api/v1/auth/zklogin", handler.HandleZkLoginAuth)
	r.GET("/api/v1/auth/zklogin/callback", handler.HandleZkLoginAuth)
	r.POST("/api/v1/auth/zklogin/submit-proof", handler.HandleSubmitProof)
	r.POST("/api/v1/auth/zklogin/verify", handler.HandleZkLoginVerify)

	// Agent Wallet API (session token auth)
	if agentWalletHandler != nil {
		wallet := r.Group("/api/v1/wallet")
		{
			wallet.POST("/create", agentWalletHandler.HandleCreateWallet)
			wallet.POST("/execute", agentWalletHandler.HandleAgentExecute)
			wallet.POST("/:wallet_id/revoke", agentWalletHandler.HandleRevokeWallet)
			wallet.GET("/:wallet_id", agentWalletHandler.HandleGetWallet)
			wallet.GET("/:wallet_id/activity", agentWalletHandler.HandleGetActivityLog)
		}
	}

	// API v1
	v1 := r.Group("/api/v1")
	v1.Use(HMACAuth(signer))
	v1.Use(RateLimit(1000))
	{
		v1.POST("/intent", handler.HandleIntent)
		v1.GET("/task/:task_id", handler.HandleGetTask)
	}

	// NLP intent parsing (no auth for demo)
	v1.POST("/parse", handler.HandleParseIntent)

	return r
}
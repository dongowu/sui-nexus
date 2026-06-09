package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sui-nexus/gateway/internal/config"
	"github.com/sui-nexus/gateway/internal/gateway/zklogin"
	"github.com/sui-nexus/gateway/internal/model"
	"github.com/sui-nexus/gateway/internal/ptb"
	"github.com/sui-nexus/gateway/internal/storage"
	"github.com/sui-nexus/gateway/pkg/hmac"
)

type IntentProducer interface {
	SendIntent(task *model.Task) error
}

type Handler struct {
	signer     *hmac.Signer
	producer   IntentProducer
	redisStore *storage.RedisStore
	hub        *Hub
	nlpClient  *NLPClient
	cfg        *config.Config

	demoMu       sync.RWMutex
	demoTasks    map[string]*model.Task
	demoBuilder  *ptb.Builder
	demoExecutor *ptb.Executor

	// zkLogin fields
	zkLoginProvider *zklogin.GoogleOAuthProvider // supports Google for now
	ephemeralKeyMgr *zklogin.EphemeralKeyManager
}

type componentHealth struct {
	Ready    bool   `json:"ready"`
	Required bool   `json:"required"`
	Message  string `json:"message,omitempty"`
}

type healthResponse struct {
	Status     string                     `json:"status"`
	Ready      bool                       `json:"ready"`
	DemoMode   bool                       `json:"demo_mode,omitempty"`
	Components map[string]componentHealth `json:"components"`
}

func NewHandler(signer *hmac.Signer, producer IntentProducer, redisStore *storage.RedisStore, hub *Hub, nlpClient *NLPClient, cfg *config.Config) *Handler {
	handler := &Handler{
		signer:     signer,
		producer:   producer,
		redisStore: redisStore,
		hub:        hub,
		nlpClient:  nlpClient,
		cfg:        cfg,
	}
	if cfg != nil && cfg.HackathonDemoMode {
		handler.demoTasks = make(map[string]*model.Task)
	}

	// Initialize zkLogin if enabled
	if cfg != nil && cfg.ZkLoginEnabled {
		handler.initZkLogin(cfg)
	}

	return handler
}

// EnableSynchronousDemoProcessing makes /api/v1/intent complete locally without
// Kafka, Redis, Walrus, or Sui credentials. It is intentionally gated by
// HACKATHON_DEMO_MODE for judge-friendly demos.
func (h *Handler) EnableSynchronousDemoProcessing(builder *ptb.Builder, executor *ptb.Executor) {
	h.demoBuilder = builder
	h.demoExecutor = executor
	if h.demoTasks == nil {
		h.demoTasks = make(map[string]*model.Task)
	}
}

// GetEphemeralKeyManager exposes the ephemeral key manager for use by AgentWalletHandler.
func (h *Handler) GetEphemeralKeyManager() *zklogin.EphemeralKeyManager {
	return h.ephemeralKeyMgr
}

// initZkLogin initializes the zkLogin provider and ephemeral key manager.
func (h *Handler) initZkLogin(cfg *config.Config) {
	switch cfg.ZkLoginProvider {
	case "google":
		h.zkLoginProvider = zklogin.NewGoogleOAuthProvider(
			cfg.ZkLoginClientID,
			cfg.ZkLoginClientSecret,
			cfg.ZkLoginRedirectURL,
		)
	case "twitch":
		// h.zkLoginProvider = zklogin.NewTwitchOAuthProvider(...)
		log.Println("Twitch zkLogin not yet implemented")
	default:
		log.Printf("Unknown zkLogin provider: %s", cfg.ZkLoginProvider)
		return
	}
	h.ephemeralKeyMgr = zklogin.NewEphemeralKeyManager(cfg.ZkLoginMaxEpoch)
	log.Printf("zkLogin initialized with provider: %s", cfg.ZkLoginProvider)
}

func (h *Handler) HandleIntent(c *gin.Context) {
	var req model.IntentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.IntentResponse{
			TaskID: req.TaskID,
			Status: model.StatusFailed,
			Error: &model.ErrorDetail{
				Code:    "ERR_INVALID_REQUEST",
				Message: err.Error(),
			},
		})
		return
	}

	// Verify HMAC signature
	apiKey, _ := c.Get("api_key")
	signature, _ := c.Get("auth_signature")
	timestamp, _ := c.Get("auth_timestamp")
	authSignature, _ := signature.(string)
	authTimestamp, _ := timestamp.(int64)
	if err := h.signer.Verify(req.TaskID, authTimestamp, req.Action, req.Params.Amount, authSignature); err != nil {
		log.Printf("Signature verification failed for task %s: %v", req.TaskID, err)
		c.JSON(http.StatusUnauthorized, model.IntentResponse{
			TaskID: req.TaskID,
			Status: model.StatusFailed,
			Error: &model.ErrorDetail{
				Code:    "ERR_AUTH_FAILED",
				Message: "Invalid or expired signature",
			},
		})
		return
	}
	if apiKeyValue, ok := apiKey.(string); ok {
		req.APIKey = apiKeyValue
	}
	req.Signature = authSignature
	req.Timestamp = authTimestamp

	// Create task
	task := &model.Task{
		TaskID:    req.TaskID,
		Status:    model.StatusPending,
		Intent:    &req,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if h.producer == nil {
		if h.isHackathonDemoMode() && h.demoBuilder != nil && h.demoExecutor != nil {
			h.processDemoIntent(c, task)
			return
		}
		log.Printf("Kafka producer not available, rejecting task %s", req.TaskID)
		c.JSON(http.StatusServiceUnavailable, model.IntentResponse{
			TaskID: req.TaskID,
			Status: model.StatusFailed,
			Error: &model.ErrorDetail{
				Code:    "ERR_QUEUE_UNAVAILABLE",
				Message: "Task queue unavailable",
			},
		})
		return
	}

	// Save to Redis
	if h.redisStore != nil {
		ctx := context.Background()
		if err := h.redisStore.SaveTask(ctx, task); err != nil {
			log.Printf("Failed to save task to Redis: %v", err)
		}
	}

	// Send to Kafka
	if err := h.producer.SendIntent(task); err != nil {
		log.Printf("Failed to send task to Kafka: %v", err)
		c.JSON(http.StatusInternalServerError, model.IntentResponse{
			TaskID: req.TaskID,
			Status: model.StatusFailed,
			Error: &model.ErrorDetail{
				Code:    "ERR_QUEUE_FAILED",
				Message: "Failed to queue task",
			},
		})
		return
	}

	log.Printf("Task %s received and queued", req.TaskID)

	c.JSON(http.StatusAccepted, model.IntentResponse{
		TaskID: req.TaskID,
		Status: model.StatusPending,
	})
}

func (h *Handler) HandleGetTask(c *gin.Context) {
	taskID := c.Param("task_id")

	if h.redisStore == nil {
		if task, ok := h.GetDemoTask(c.Request.Context(), taskID); ok {
			c.JSON(http.StatusOK, task)
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Storage unavailable"})
		return
	}

	ctx := context.Background()
	task, err := h.redisStore.GetTask(ctx, taskID)
	if err != nil || task == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	c.JSON(http.StatusOK, task)
}

func (h *Handler) HandleHealth(c *gin.Context) {
	queueReady := h.producer != nil
	demoMode := h.isHackathonDemoMode()
	components := map[string]componentHealth{
		"queue": {
			Ready:    queueReady || demoMode,
			Required: true,
		},
		"storage": {
			Ready:    h.redisStore != nil || demoMode,
			Required: false,
		},
	}
	if demoMode && !queueReady {
		queue := components["queue"]
		queue.Message = "Hackathon demo mode is processing intents synchronously; Kafka is bypassed"
		components["queue"] = queue
	} else if !queueReady {
		queue := components["queue"]
		queue.Message = "Kafka producer is not configured; /api/v1/intent will return 503"
		components["queue"] = queue
	}
	if demoMode && h.redisStore == nil {
		storageHealth := components["storage"]
		storageHealth.Message = "Hackathon demo mode is using in-memory task and wallet state"
		components["storage"] = storageHealth
	} else if h.redisStore == nil {
		storageHealth := components["storage"]
		storageHealth.Message = "Redis is not configured; task lookup is unavailable"
		components["storage"] = storageHealth
	}

	status := "healthy"
	httpStatus := http.StatusOK
	if !queueReady && !demoMode {
		status = "unavailable"
		httpStatus = http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, healthResponse{
		Status:     status,
		Ready:      queueReady || demoMode,
		DemoMode:   demoMode,
		Components: components,
	})
}

func (h *Handler) isHackathonDemoMode() bool {
	return h != nil && h.cfg != nil && h.cfg.HackathonDemoMode
}

func (h *Handler) processDemoIntent(c *gin.Context, task *model.Task) {
	task.Status = model.StatusProcessing
	task.UpdatedAt = time.Now()

	if task.Intent != nil && task.Intent.ContextPayload != "" {
		task.BlobID = demoBlobID(task.Intent.ContextPayload)
	}

	ptbTxn, err := h.demoBuilder.Build(task)
	if err != nil {
		task.Status = model.StatusFailed
		task.UpdatedAt = time.Now()
		h.saveDemoTask(task)
		c.JSON(http.StatusUnprocessableEntity, model.IntentResponse{
			TaskID: task.TaskID,
			Status: model.StatusFailed,
			Error: &model.ErrorDetail{
				Code:    "ERR_BUILD_FAILED",
				Message: err.Error(),
			},
		})
		return
	}

	digest, err := h.demoExecutor.ExecutePTB(c.Request.Context(), ptbTxn)
	if err != nil {
		task.Status = model.StatusFailed
		task.UpdatedAt = time.Now()
		h.saveDemoTask(task)
		c.JSON(http.StatusInternalServerError, model.IntentResponse{
			TaskID: task.TaskID,
			Status: model.StatusFailed,
			Error: &model.ErrorDetail{
				Code:    "ERR_DEMO_EXECUTION_FAILED",
				Message: err.Error(),
			},
		})
		return
	}

	task.Status = model.StatusCompleted
	task.TxDigest = digest
	task.UpdatedAt = time.Now()
	h.saveDemoTask(task)

	if h.hub != nil {
		h.hub.BroadcastTask(task)
	}

	c.JSON(http.StatusAccepted, model.IntentResponse{
		TaskID:   task.TaskID,
		Status:   model.StatusCompleted,
		TxDigest: digest,
	})
}

func demoBlobID(payload string) string {
	sum := sha256.Sum256([]byte(payload))
	return "demo-walrus-" + hex.EncodeToString(sum[:])[:40]
}

func (h *Handler) saveDemoTask(task *model.Task) {
	if task == nil {
		return
	}
	h.demoMu.Lock()
	defer h.demoMu.Unlock()
	if h.demoTasks == nil {
		h.demoTasks = make(map[string]*model.Task)
	}
	h.demoTasks[task.TaskID] = task
}

func (h *Handler) GetDemoTask(_ context.Context, taskID string) (*model.Task, bool) {
	h.demoMu.RLock()
	defer h.demoMu.RUnlock()
	task, ok := h.demoTasks[taskID]
	return task, ok
}

// HandleParseIntent parses natural language intent into structured format via NLP service.
func (h *Handler) HandleParseIntent(c *gin.Context) {
	if h.nlpClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "NLP service is not configured",
		})
		return
	}

	var req struct {
		Text string `json:"text" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()
	result, err := h.nlpClient.ParseIntent(ctx, req.Text)
	if err != nil {
		log.Printf("NLP parsing failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if result.Error != "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":  result.Error,
			"action": result.Action,
			"params": result.Params,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"action": result.Action,
		"params": result.Params,
	})
}

// HandleZkLoginAuth handles the zkLogin authentication flow.
// GET /api/v1/auth/zklogin - initiate OAuth flow (redirect to Google)
// GET /api/v1/auth/zklogin/callback - OAuth callback (exchange code, create session)
func (h *Handler) HandleZkLoginAuth(c *gin.Context) {
	if h.zkLoginProvider == nil || h.ephemeralKeyMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "zkLogin is not configured"})
		return
	}

	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		// Step 1: Redirect to OAuth provider
		authURL := h.zkLoginProvider.GetAuthURL(state)
		log.Printf("Redirecting to OAuth: %s", authURL)
		c.Redirect(http.StatusTemporaryRedirect, authURL)
		return
	}

	// Step 2: Handle OAuth callback
	ctx := context.Background()

	// Exchange code for JWT
	jwt, err := h.zkLoginProvider.ExchangeCode(ctx, code)
	if err != nil {
		log.Printf("OAuth token exchange failed: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "OAuth exchange failed"})
		return
	}

	// Get user info from JWT
	userInfo, err := h.zkLoginProvider.GetUserInfo(ctx, jwt)
	if err != nil {
		log.Printf("Failed to get user info: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to get user info"})
		return
	}

	// Create zkLogin session (generates ephemeral key, salt, randomness)
	// The actual ZK proof and address derivation happens client-side via @mysten/zklogin
	sessionKey, sessionToken, err := h.ephemeralKeyMgr.CreateSession(
		jwt,
		userInfo.Subject,
		userInfo.Email,
		"https://accounts.google.com",
	)
	if err != nil {
		log.Printf("Session creation failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Session creation failed"})
		return
	}

	// Set audience from config
	session := h.ephemeralKeyMgr.GetSessionForToken(sessionToken)
	if session != nil {
		session.Audience = h.cfg.ZkLoginClientID
	}

	log.Printf("zkLogin session created: email=%s, session=%s...", userInfo.Email, shortID(sessionToken))

	// Return zkLogin session params — client uses these with @mysten/zklogin
	// to generate the proof and derive the Sui address
	c.JSON(http.StatusOK, gin.H{
		"session_token":     sessionToken,
		"jwt":               jwt,
		"salt":              sessionKey.Salt,
		"jwt_randomness":    sessionKey.JwtRandomness,
		"key_claim_name":    "sub",
		"key_claim_value":   userInfo.Subject,
		"audience":          h.cfg.ZkLoginClientID,
		"issuer":            "https://accounts.google.com",
		"ephemeral_pub_key": sessionKey.PublicKey,
		"max_epoch":         sessionKey.MaxEpoch,
		"email":             userInfo.Email,
		"expires_at":        sessionKey.IssuedAt.Add(24 * time.Hour * time.Duration(sessionKey.MaxEpoch)).Unix(),
		// Client must call POST /api/v1/auth/zklogin/submit-proof after proof gen
	})
}

// HandleSubmitProof receives the zkLogin proof from the client (generated by @mysten/zklogin).
// POST /api/v1/auth/zklogin/submit-proof
func (h *Handler) HandleSubmitProof(c *gin.Context) {
	if h.ephemeralKeyMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "zkLogin is not configured"})
		return
	}

	var payload struct {
		zklogin.ProofSubmission
		SessionToken string `json:"session_token"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// The client must send back the session token so we can match the session
	sessionToken := c.GetHeader("X-Session-Token")
	if sessionToken == "" {
		sessionToken = payload.SessionToken
	}
	if sessionToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_token is required"})
		return
	}

	if err := h.ephemeralKeyMgr.SubmitProof(sessionToken, payload.UserAddress, payload.AddressSeed, payload.Proof); err != nil {
		log.Printf("Proof submission failed: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	log.Printf("zkLogin proof submitted: address=%s", payload.UserAddress)

	c.JSON(http.StatusOK, gin.H{
		"valid":        true,
		"user_address": payload.UserAddress,
	})
}

// HandleZkLoginVerify verifies a zkLogin session token.
func (h *Handler) HandleZkLoginVerify(c *gin.Context) {
	if h.ephemeralKeyMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "zkLogin is not configured"})
		return
	}

	var req struct {
		UserAddress  string `json:"user_address" binding:"required"`
		SessionToken string `json:"session_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !h.ephemeralKeyMgr.IsValid(req.UserAddress, req.SessionToken) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"valid": true, "user_address": req.UserAddress})
}

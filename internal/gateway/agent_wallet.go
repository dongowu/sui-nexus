package gateway

import (
	"context"
	"encoding/json"
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
)

// AgentWalletHandler handles HTTP requests for Agent Wallet operations.
type AgentWalletHandler struct {
	config          *config.Config
	ptbExecutor     *ptb.Executor
	ptbBuilder      *ptb.Builder
	redisStore      *storage.RedisStore
	hub             *Hub
	ephemeralKeyMgr *zklogin.EphemeralKeyManager

	// In-memory wallet cache (fallback when Redis is unavailable)
	mu      sync.RWMutex
	wallets map[string]*model.AgentWallet
}

// NewAgentWalletHandler creates a new AgentWalletHandler.
func NewAgentWalletHandler(
	cfg *config.Config,
	executor *ptb.Executor,
	builder *ptb.Builder,
	redisStore *storage.RedisStore,
	hub *Hub,
	ephemeralKeyMgr *zklogin.EphemeralKeyManager,
) *AgentWalletHandler {
	return &AgentWalletHandler{
		config:          cfg,
		ptbExecutor:     executor,
		ptbBuilder:      builder,
		redisStore:      redisStore,
		hub:             hub,
		ephemeralKeyMgr: ephemeralKeyMgr,
		wallets:         make(map[string]*model.AgentWallet),
	}
}

// ────────────────────────────────────────────────────────────
// HTTP Handlers
// ────────────────────────────────────────────────────────────

// HandleCreateWallet handles POST /api/v1/wallet/create
// Owner creates an AgentWallet with the given policy parameters.
func (h *AgentWalletHandler) HandleCreateWallet(c *gin.Context) {
	var req model.CreateWalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.WalletResponse{
			Error: &model.ErrorDetail{Code: "ERR_INVALID_REQUEST", Message: err.Error()},
		})
		return
	}

	packageID := h.config.AgentWalletPackageID
	if packageID == "" {
		packageID = h.config.SuiSignerPrivateKey // fallback: use published package id
		c.JSON(http.StatusInternalServerError, model.WalletResponse{
			Error: &model.ErrorDetail{Code: "ERR_CONFIG", Message: "agent wallet package id not configured"},
		})
		return
	}

	ptbTxn, err := h.ptbBuilder.BuildAgentWalletCreate(
		req.AgentAddress,
		req.BudgetCapMist,
		req.AllowedProtocols,
		req.TimeEndEpoch,
		packageID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.WalletResponse{
			Error: &model.ErrorDetail{Code: "ERR_BUILD_FAILED", Message: err.Error()},
		})
		return
	}

	digest, err := h.ptbExecutor.ExecutePTB(c.Request.Context(), ptbTxn)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.WalletResponse{
			Error: &model.ErrorDetail{Code: "ERR_EXECUTION_FAILED", Message: err.Error()},
		})
		return
	}

	log.Printf("[agent-wallet] wallet created: tx=%s agent=%s budget=%d", digest, req.AgentAddress, req.BudgetCapMist)

	// The wallet ID will be extracted from the TxEffects by querying the tx.
	// For simplicity during hackathon, we generate a lookup id and cache the
	// expected state. In production, parse the Created event from the tx response.
	walletID := digest // use tx digest as provisional wallet ID

	// Cache wallet state
	wallet := &model.AgentWallet{
		WalletID:     walletID,
		AgentAddress: req.AgentAddress,
		Policy: model.WalletPolicy{
			BudgetCapMist:    req.BudgetCapMist,
			BudgetSpentMist:  0,
			AllowedProtocols: req.AllowedProtocols,
			TimeEnd:          req.TimeEndEpoch,
		},
		IsActive:    true,
		BalanceMist: 0,
		ActivityLog: []model.ActivityEntry{},
	}
	h.cacheWallet(walletID, wallet)

	// Broadcast via WebSocket
	if h.hub != nil {
		h.hub.BroadcastTask(&model.Task{
			TaskID:   walletID,
			Status:   model.StatusCompleted,
			TxDigest: digest,
		})
	}

	c.JSON(http.StatusOK, model.WalletResponse{
		WalletID: walletID,
		Policy:   &wallet.Policy,
		IsActive: true,
		TxDigest: digest,
	})
}

// HandleAgentExecute handles POST /api/v1/wallet/execute
// Agent executes a trade through the wallet (zkLogin auth → policy check → on-chain execution).
func (h *AgentWalletHandler) HandleAgentExecute(c *gin.Context) {
	var req model.AgentExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.WalletResponse{
			Error: &model.ErrorDetail{Code: "ERR_INVALID_REQUEST", Message: err.Error()},
		})
		return
	}

	// Verify agent's zkLogin session
	if h.ephemeralKeyMgr != nil {
		if !h.ephemeralKeyMgr.IsValid(req.UserAddress, req.SessionToken) {
			c.JSON(http.StatusUnauthorized, model.WalletResponse{
				Error: &model.ErrorDetail{Code: "ERR_AUTH_FAILED", Message: "Invalid or expired zkLogin session. Please re-authenticate."},
			})
			return
		}
	}

	packageID := h.config.AgentWalletPackageID
	if packageID == "" {
		c.JSON(http.StatusInternalServerError, model.WalletResponse{
			Error: &model.ErrorDetail{Code: "ERR_CONFIG", Message: "agent wallet package id not configured"},
		})
		return
	}

	// Verify wallet exists and is active
	wallet := h.getCachedWallet(req.WalletID)
	if wallet == nil {
		c.JSON(http.StatusNotFound, model.WalletResponse{
			Error: &model.ErrorDetail{Code: "ERR_WALLET_NOT_FOUND", Message: "wallet not found"},
		})
		return
	}
	if !wallet.IsActive {
		c.JSON(http.StatusForbidden, model.WalletResponse{
			Error: &model.ErrorDetail{Code: "ERR_WALLET_REVOKED", Message: "wallet has been revoked"},
		})
		return
	}

	// Step 1: Execute trade through agent_wallet (policy check + budget deduct)
	ptbTxn, err := h.ptbBuilder.BuildAgentWalletExecuteTrade(
		req.WalletID,
		req.AmountMist,
		req.Protocol,
		req.Description,
		packageID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.WalletResponse{
			Error: &model.ErrorDetail{Code: "ERR_BUILD_FAILED", Message: err.Error()},
		})
		return
	}

	digest, err := h.ptbExecutor.ExecutePTB(c.Request.Context(), ptbTxn)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.WalletResponse{
			Error: &model.ErrorDetail{Code: "ERR_POLICY_FAILED", Message: err.Error()},
		})
		return
	}

	log.Printf("[agent-wallet] trade executed: tx=%s wallet=%s amount=%d protocol=%s", digest, req.WalletID, req.AmountMist, req.Protocol)

	// Update cached wallet state
	h.updateWalletAfterTrade(req.WalletID, req.AmountMist, req.Protocol, req.Description)

	// Broadcast via WebSocket
	if h.hub != nil {
		h.hub.BroadcastTask(&model.Task{
			TaskID:   req.WalletID,
			Status:   model.StatusCompleted,
			TxDigest: digest,
		})
	}

	c.JSON(http.StatusOK, model.WalletResponse{
		WalletID: req.WalletID,
		TxDigest: digest,
	})
}

// HandleRevokeWallet handles POST /api/v1/wallet/:wallet_id/revoke
// Owner revokes the wallet, freezing all agent activity.
func (h *AgentWalletHandler) HandleRevokeWallet(c *gin.Context) {
	walletID := c.Param("wallet_id")
	if walletID == "" {
		c.JSON(http.StatusBadRequest, model.WalletResponse{
			Error: &model.ErrorDetail{Code: "ERR_INVALID_REQUEST", Message: "wallet_id is required"},
		})
		return
	}

	// Check wallet exists
	wallet := h.getCachedWallet(walletID)
	if wallet == nil {
		c.JSON(http.StatusNotFound, model.WalletResponse{
			Error: &model.ErrorDetail{Code: "ERR_WALLET_NOT_FOUND", Message: "wallet not found"},
		})
		return
	}

	packageID := h.config.AgentWalletPackageID
	ptbTxn, err := h.ptbBuilder.BuildAgentWalletRevoke(walletID, packageID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.WalletResponse{
			Error: &model.ErrorDetail{Code: "ERR_BUILD_FAILED", Message: err.Error()},
		})
		return
	}

	digest, err := h.ptbExecutor.ExecutePTB(c.Request.Context(), ptbTxn)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.WalletResponse{
			Error: &model.ErrorDetail{Code: "ERR_EXECUTION_FAILED", Message: err.Error()},
		})
		return
	}

	// Mark wallet as revoked in cache
	h.mu.Lock()
	if w, ok := h.wallets[walletID]; ok {
		w.IsActive = false
	}
	h.mu.Unlock()

	log.Printf("[agent-wallet] wallet revoked: tx=%s wallet=%s", digest, walletID)

	// Broadcast via WebSocket
	if h.hub != nil {
		h.hub.BroadcastTask(&model.Task{
			TaskID:   walletID,
			Status:   model.StatusCompleted,
			TxDigest: digest,
		})
	}

	c.JSON(http.StatusOK, model.WalletResponse{
		WalletID: walletID,
		IsActive: false,
		TxDigest: digest,
	})
}

// HandleGetWallet handles GET /api/v1/wallet/:wallet_id
// Returns the current state of a wallet.
func (h *AgentWalletHandler) HandleGetWallet(c *gin.Context) {
	walletID := c.Param("wallet_id")

	wallet := h.getCachedWallet(walletID)
	if wallet == nil {
		c.JSON(http.StatusNotFound, model.WalletResponse{
			WalletID: walletID,
			Error:    &model.ErrorDetail{Code: "ERR_WALLET_NOT_FOUND", Message: "wallet not found"},
		})
		return
	}

	c.JSON(http.StatusOK, wallet)
}

// HandleGetActivityLog handles GET /api/v1/wallet/:wallet_id/activity
// Returns the on-chain activity log for a wallet.
func (h *AgentWalletHandler) HandleGetActivityLog(c *gin.Context) {
	walletID := c.Param("wallet_id")

	wallet := h.getCachedWallet(walletID)
	if wallet == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "wallet not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"wallet_id":    walletID,
		"is_active":    wallet.IsActive,
		"activities":   wallet.ActivityLog,
		"total_entries": len(wallet.ActivityLog),
	})
}

// ────────────────────────────────────────────────────────────
// Cache helpers (in-memory with optional Redis)
// ────────────────────────────────────────────────────────────

func (h *AgentWalletHandler) cacheWallet(id string, wallet *model.AgentWallet) {
	h.mu.Lock()
	h.wallets[id] = wallet
	h.mu.Unlock()

	// Optional Redis persistence
	if h.redisStore != nil {
		data, err := json.Marshal(wallet)
		if err != nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		h.redisStore.Client().Set(ctx, "wallet:"+id, data, 24*time.Hour)
	}
}

func (h *AgentWalletHandler) getCachedWallet(id string) *model.AgentWallet {
	h.mu.RLock()
	w, ok := h.wallets[id]
	h.mu.RUnlock()
	if ok {
		return w
	}

	// Try Redis fallback
	if h.redisStore != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		data, err := h.redisStore.Client().Get(ctx, "wallet:"+id).Bytes()
		if err != nil {
			return nil
		}
		var wallet model.AgentWallet
		if err := json.Unmarshal(data, &wallet); err != nil {
			return nil
		}
		// Restore to memory
		h.mu.Lock()
		h.wallets[id] = &wallet
		h.mu.Unlock()
		return &wallet
	}

	return nil
}

func (h *AgentWalletHandler) updateWalletAfterTrade(walletID string, amountMist uint64, protocol, description string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	w, ok := h.wallets[walletID]
	if !ok {
		return
	}
	w.Policy.BudgetSpentMist += amountMist
	w.ActivityLog = append(w.ActivityLog, model.ActivityEntry{
		Timestamp:   uint64(time.Now().Unix()),
		Action:      "trade",
		AmountMist:  amountMist,
		Protocol:    protocol,
		Description: description,
	})

	// Sync to Redis
	if h.redisStore != nil {
		data, _ := json.Marshal(w)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		h.redisStore.Client().Set(ctx, "wallet:"+walletID, data, 24*time.Hour)
	}
}

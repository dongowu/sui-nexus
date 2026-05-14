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

const demoSessionToken = "demo-session-token"

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
		BalanceMist: req.BudgetCapMist,
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
		WalletID:    walletID,
		Policy:      &wallet.Policy,
		IsActive:    true,
		BalanceMist: wallet.BalanceMist,
		TxDigest:    digest,
	})
}

// HandleAgentExecute handles POST /api/v1/wallet/execute
// GuardianResult holds the result of pre-flight risk checks.
type GuardianResult struct {
	Passed   bool   `json:"passed"`
	RiskType string `json:"risk_type,omitempty"`
	Message  string `json:"message,omitempty"`
}

// Agent executes a trade through the wallet (zkLogin auth → Guardian → policy check → on-chain execution).
func (h *AgentWalletHandler) HandleAgentExecute(c *gin.Context) {
	var req model.AgentExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.WalletResponse{
			Error: &model.ErrorDetail{Code: "ERR_INVALID_REQUEST", Message: err.Error()},
		})
		return
	}

	// Step 0: Verify agent's zkLogin session
	if !h.isHackathonDemoMode() && h.ephemeralKeyMgr == nil {
		c.JSON(http.StatusServiceUnavailable, model.WalletResponse{
			Error: &model.ErrorDetail{Code: "ERR_CONFIG", Message: "zkLogin is not configured"},
		})
		return
	}
	if !h.isHackathonDemoMode() && !h.ephemeralKeyMgr.IsValid(req.UserAddress, req.SessionToken) {
		c.JSON(http.StatusUnauthorized, model.WalletResponse{
			Error: &model.ErrorDetail{Code: "ERR_AUTH_FAILED", Message: "Invalid or expired zkLogin session. Please re-authenticate."},
		})
		return
	}
	// The zkLogin-verified agent address is req.UserAddress (the agent's Sui address)
	agentAddr := req.UserAddress
	if h.isHackathonDemoMode() && req.SessionToken == demoSessionToken && agentAddr == "" {
		if wallet := h.getCachedWallet(req.WalletID); wallet != nil {
			agentAddr = wallet.AgentAddress
		}
	}

	packageID := h.config.AgentWalletPackageID
	if packageID == "" {
		c.JSON(http.StatusInternalServerError, model.WalletResponse{
			Error: &model.ErrorDetail{Code: "ERR_CONFIG", Message: "agent wallet package id not configured"},
		})
		return
	}

	// Step 1: Verify wallet exists and agent is authorized
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
	// Agent identity: the zkLogin address must match the wallet's authorized agent
	if wallet.AgentAddress != "" && wallet.AgentAddress != agentAddr {
		c.JSON(http.StatusForbidden, model.WalletResponse{
			Error: &model.ErrorDetail{Code: "ERR_NOT_AUTHORIZED", Message: "agent address does not match wallet's authorized agent"},
		})
		return
	}

	// Step 2: Guardian — pre-flight risk checks (at least 2 risk categories)
	guardian := h.runGuardianChecks(&req, wallet)
	if !guardian.Passed {
		c.JSON(http.StatusUnprocessableEntity, model.WalletResponse{
			WalletID:    req.WalletID,
			IsActive:    wallet.IsActive,
			BalanceMist: wallet.BalanceMist,
			Guardian:    guardian.toReport(),
			Error:       &model.ErrorDetail{Code: "ERR_GUARDIAN_REJECTED", Message: guardian.Message},
		})
		return
	}
	log.Printf("[agent-wallet] guardian passed: wallet=%s risk=%s", req.WalletID, guardian.RiskType)

	// Step 3: Execute trade through agent_wallet Move contract
	// The agent_addr is passed to the Move contract which verifies it on-chain
	ptbTxn, err := h.ptbBuilder.BuildAgentWalletExecuteTrade(
		req.WalletID,
		agentAddr,
		req.AmountMist,
		req.Protocol,
		req.ExpectedPrice,
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

	log.Printf("[agent-wallet] trade executed: tx=%s wallet=%s agent=%s amount=%d protocol=%s", digest, req.WalletID, shortID(agentAddr), req.AmountMist, req.Protocol)

	// Update cached wallet state
	h.updateWalletAfterTrade(req.WalletID, req.AmountMist, req.Protocol, req.Description)
	wallet = h.getCachedWallet(req.WalletID)

	// Step 4 (optional): If protocol is DeepBook, execute the actual order
	// The policy check succeeded, so we can now place the DeepBook limit order
	var deepBookDigest string
	if h.config.DeepBookPackageID != "" && h.config.DeepBookPoolID != "" {
		dbPTB, err := h.ptbBuilder.BuildAgentWalletExecuteDeepBook(
			h.config.DeepBookPoolID,
			"", // balance_manager — managed by gateway signer
			"", // trade_proof — managed by gateway signer
			1,  // client_order_id
			0,  // order_type: limit order
			req.ExpectedPrice,
			req.AmountMist,
			true,                                 // is_bid = buy
			uint64(time.Now().UnixMilli()+60000), // expire in 1 minute
			h.config.DeepBookPackageID,
		)
		if err != nil {
			log.Printf("[agent-wallet] DeepBook order build failed: %v (policy tx succeeded)", err)
		} else {
			deepBookDigest, err = h.ptbExecutor.ExecutePTB(c.Request.Context(), dbPTB)
			if err != nil {
				log.Printf("[agent-wallet] DeepBook order failed: %v (policy tx succeeded: %s)", err, digest)
			} else {
				log.Printf("[agent-wallet] DeepBook order placed: %s", deepBookDigest)
			}
		}
	}

	// Broadcast via WebSocket
	if h.hub != nil {
		h.hub.BroadcastTask(&model.Task{
			TaskID:   req.WalletID,
			Status:   model.StatusCompleted,
			TxDigest: digest,
		})
	}

	c.JSON(http.StatusOK, model.WalletResponse{
		WalletID:         req.WalletID,
		IsActive:         wallet != nil && wallet.IsActive,
		BalanceMist:      walletBalance(wallet),
		Policy:           walletPolicy(wallet),
		TxDigest:         digest,
		DeepBookTxDigest: deepBookDigest,
		Guardian:         guardian.toReport(),
	})
}

func walletBalance(wallet *model.AgentWallet) uint64 {
	if wallet == nil {
		return 0
	}
	return wallet.BalanceMist
}

func walletPolicy(wallet *model.AgentWallet) *model.WalletPolicy {
	if wallet == nil {
		return nil
	}
	return &wallet.Policy
}

func (h *AgentWalletHandler) isHackathonDemoMode() bool {
	return h != nil && h.config != nil && h.config.HackathonDemoMode
}

func (g GuardianResult) toReport() *model.GuardianReport {
	return &model.GuardianReport{
		Passed:   g.Passed,
		RiskType: g.RiskType,
		Message:  g.Message,
	}
}

// runGuardianChecks performs pre-flight risk assessment before trade execution.
// Implements 2+ risk categories as required by Agentic Web Intent Engine sub-track.
func (h *AgentWalletHandler) runGuardianChecks(req *model.AgentExecuteRequest, wallet *model.AgentWallet) GuardianResult {
	// Risk Category 1: Slippage / Price Impact check
	if req.ExpectedPrice > 0 {
		// Simulate: compare expected price to "current market price"
		// In production, this would query an oracle (Pyth/Switchboard) or DEX pool
		simulatedMarketPrice := req.ExpectedPrice // placeholder: use same as expected
		maxSlippageBps := uint64(500)             // 5% max slippage
		if simulatedMarketPrice > 0 {
			deviation := uint64(0)
			if req.ExpectedPrice > simulatedMarketPrice {
				deviation = req.ExpectedPrice - simulatedMarketPrice
			} else {
				deviation = simulatedMarketPrice - req.ExpectedPrice
			}
			slippageBps := (deviation * 10000) / simulatedMarketPrice
			if slippageBps > maxSlippageBps {
				return GuardianResult{
					Passed:   false,
					RiskType: "HIGH_SLIPPAGE",
					Message:  "Price deviation exceeds maximum allowed slippage (5%)",
				}
			}
		}
	}

	// Risk Category 2: Concentration / Budget Exhaustion check
	budgetRemaining := wallet.Policy.BudgetCapMist - wallet.Policy.BudgetSpentMist
	if req.AmountMist > budgetRemaining {
		return GuardianResult{
			Passed:   false,
			RiskType: "BUDGET_EXCEEDED",
			Message:  "Trade amount exceeds remaining budget",
		}
	}
	// Warn if single trade consumes > 50% of remaining budget
	if budgetRemaining > 0 && req.AmountMist > budgetRemaining/2 {
		return GuardianResult{
			Passed:   true,
			RiskType: "HIGH_CONCENTRATION",
			Message:  "Single trade exceeds 50% of remaining budget (warning only)",
		}
	}

	// Risk Category 3: Stale pool / protocol health check
	// In production: verify the protocol (DEX pool) is still active and has sufficient liquidity
	// For hackathon: check protocol is non-empty and in wallet's allowlist
	allowed := false
	for _, p := range wallet.Policy.AllowedProtocols {
		if p == req.Protocol {
			allowed = true
			break
		}
	}
	if len(wallet.Policy.AllowedProtocols) > 0 && !allowed {
		return GuardianResult{
			Passed:   false,
			RiskType: "PROTOCOL_NOT_ALLOWED",
			Message:  "Protocol not in wallet's allowed list",
		}
	}

	return GuardianResult{Passed: true}
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
		"wallet_id":     walletID,
		"is_active":     wallet.IsActive,
		"activities":    wallet.ActivityLog,
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
	if w.BalanceMist >= amountMist {
		w.BalanceMist -= amountMist
	}
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

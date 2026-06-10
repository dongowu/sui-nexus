package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sui-nexus/gateway/internal/config"
	"github.com/sui-nexus/gateway/internal/model"
	"github.com/sui-nexus/gateway/internal/ptb"
)

func TestAgentWalletDemoModeExecutesAndReportsGuardian(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		HackathonDemoMode:    true,
		AgentWalletPackageID: "0xPackage",
	}
	builder := ptb.NewBuilder(10_000_000)
	walletHandler := NewAgentWalletHandler(cfg, ptb.NewDemoExecutor(), builder, nil, nil, nil)

	createResp := postWalletRequest(t, walletHandler.HandleCreateWallet, "/api/v1/wallet/create", model.CreateWalletRequest{
		AgentAddress:     "0xAgent",
		BudgetCapMist:    500,
		AllowedProtocols: []string{"DeepBook"},
		TimeEndEpoch:     999999,
	})
	require.Empty(t, createResp.Error)
	require.NotEmpty(t, createResp.WalletID)
	assert.True(t, createResp.IsActive)
	assert.Equal(t, uint64(500), createResp.BalanceMist)

	executeResp := postWalletRequest(t, walletHandler.HandleAgentExecute, "/api/v1/wallet/execute", model.AgentExecuteRequest{
		WalletID:      createResp.WalletID,
		AmountMist:    100,
		Protocol:      "DeepBook",
		ExpectedPrice: 1000,
		ObservedPrice: 1000,
		Description:   "demo trade",
		SessionToken:  demoSessionToken,
		UserAddress:   "0xAgent",
	})

	require.Empty(t, executeResp.Error)
	require.NotNil(t, executeResp.Guardian)
	assert.True(t, executeResp.Guardian.Passed)
	assert.True(t, executeResp.IsActive)
	assert.Equal(t, uint64(400), executeResp.BalanceMist)
	assert.NotEmpty(t, executeResp.TxDigest)
}

func TestAgentWalletDemoModeReturnsGuardianOnBudgetRejection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		HackathonDemoMode:    true,
		AgentWalletPackageID: "0xPackage",
	}
	builder := ptb.NewBuilder(10_000_000)
	walletHandler := NewAgentWalletHandler(cfg, ptb.NewDemoExecutor(), builder, nil, nil, nil)

	createResp := postWalletRequest(t, walletHandler.HandleCreateWallet, "/api/v1/wallet/create", model.CreateWalletRequest{
		AgentAddress:     "0xAgent",
		BudgetCapMist:    500,
		AllowedProtocols: []string{"DeepBook"},
		TimeEndEpoch:     999999,
	})
	require.Empty(t, createResp.Error)

	executeResp := postWalletRequest(t, walletHandler.HandleAgentExecute, "/api/v1/wallet/execute", model.AgentExecuteRequest{
		WalletID:      createResp.WalletID,
		AmountMist:    600,
		Protocol:      "DeepBook",
		ExpectedPrice: 1000,
		ObservedPrice: 1000,
		Description:   "overspend",
		SessionToken:  demoSessionToken,
		UserAddress:   "0xAgent",
	})

	require.NotNil(t, executeResp.Error)
	require.NotNil(t, executeResp.Guardian)
	assert.True(t, executeResp.IsActive)
	assert.False(t, executeResp.Guardian.Passed)
	assert.Equal(t, "BUDGET_EXCEEDED", executeResp.Guardian.RiskType)
}

func TestGuardianResultStructuredFieldsOnBudgetExceeded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		HackathonDemoMode:    true,
		AgentWalletPackageID: "0xPackage",
	}
	builder := ptb.NewBuilder(10_000_000)
	walletHandler := NewAgentWalletHandler(cfg, ptb.NewDemoExecutor(), builder, nil, nil, nil)

	createResp := postWalletRequest(t, walletHandler.HandleCreateWallet, "/api/v1/wallet/create", model.CreateWalletRequest{
		AgentAddress:     "0xAgent",
		BudgetCapMist:    500,
		AllowedProtocols: []string{"DeepBook"},
		TimeEndEpoch:     999999,
	})
	require.Empty(t, createResp.Error)

	executeResp := postWalletRequest(t, walletHandler.HandleAgentExecute, "/api/v1/wallet/execute", model.AgentExecuteRequest{
		WalletID:      createResp.WalletID,
		AmountMist:    600,
		Protocol:      "DeepBook",
		ExpectedPrice: 1000,
		ObservedPrice: 1000,
		Description:   "overspend",
		SessionToken:  demoSessionToken,
		UserAddress:   "0xAgent",
	})

	require.NotNil(t, executeResp.Error)
	require.NotNil(t, executeResp.Guardian)
	assert.False(t, executeResp.Guardian.Passed)
	assert.Equal(t, "BUDGET_EXCEEDED", executeResp.Guardian.RiskType)
	assert.Equal(t, "budget_cap", executeResp.Guardian.Reason)
	assert.Equal(t, uint64(600), executeResp.Guardian.Requested)
	assert.Equal(t, uint64(500), executeResp.Guardian.Allowed)
	assert.Contains(t, executeResp.Guardian.Message, "600")
	assert.Contains(t, executeResp.Guardian.Message, "500")
}

func TestGuardianHighConcentrationWarning(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		HackathonDemoMode:    true,
		AgentWalletPackageID: "0xPackage",
	}
	builder := ptb.NewBuilder(10_000_000)
	walletHandler := NewAgentWalletHandler(cfg, ptb.NewDemoExecutor(), builder, nil, nil, nil)

	createResp := postWalletRequest(t, walletHandler.HandleCreateWallet, "/api/v1/wallet/create", model.CreateWalletRequest{
		AgentAddress:     "0xAgent",
		BudgetCapMist:    1000,
		AllowedProtocols: []string{"DeepBook"},
		TimeEndEpoch:     999999,
	})
	require.Empty(t, createResp.Error)

	// 600 MIST 的单笔交易超过剩余预算(1000)的 50%
	executeResp := postWalletRequest(t, walletHandler.HandleAgentExecute, "/api/v1/wallet/execute", model.AgentExecuteRequest{
		WalletID:      createResp.WalletID,
		AmountMist:    600,
		Protocol:      "DeepBook",
		ExpectedPrice: 1000,
		ObservedPrice: 1000,
		Description:   "large trade",
		SessionToken:  demoSessionToken,
		UserAddress:   "0xAgent",
	})

	require.Empty(t, executeResp.Error)
	require.NotNil(t, executeResp.Guardian)
	assert.True(t, executeResp.Guardian.Passed)
	assert.Equal(t, "HIGH_CONCENTRATION", executeResp.Guardian.RiskType)
	assert.Equal(t, "concentration", executeResp.Guardian.Reason)
	assert.Equal(t, uint64(600), executeResp.Guardian.Requested)
	assert.Equal(t, uint64(500), executeResp.Guardian.Allowed) // 50% of 1000
}

func TestParseMoveAbortCode(t *testing.T) {
	tests := []struct {
		code      uint64
		requested uint64
		allowed   uint64
		want      string
	}{
		{6, 600, 500, "Budget cap exceeded: requested 600 MIST, cap is 500 MIST per epoch"},
		{5, 0, 0, "Protocol not in wallet's allowed list"},
		{1, 0, 0, "Agent not authorized for this wallet"},
		{2, 0, 0, "Wallet has been revoked"},
		{4, 0, 0, "Wallet time window has expired"},
		{7, 0, 0, "Insufficient balance in wallet"},
		{9, 0, 0, "Guardian rejected the trade — observed price below the agent's expected floor"},
		{99, 0, 0, "Move abort code 99"},
		{0, 0, 0, "Move abort code 0"},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("code_%d", tc.code), func(t *testing.T) {
			got := parseMoveAbortCode(tc.code, tc.requested, tc.allowed)
			assert.Equal(t, tc.want, got)
		})
	}
}

func postWalletRequest[T any](t *testing.T, handler gin.HandlerFunc, path string, payload T) model.WalletResponse {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", path, bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c, _ := gin.CreateTestContext(w)
	c.Request = req
	handler(c)

	var resp model.WalletResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

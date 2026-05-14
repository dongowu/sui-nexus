package gateway

import (
	"bytes"
	"encoding/json"
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

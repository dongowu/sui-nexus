package ptb

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/block-vision/sui-go-sdk/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSuiWriteClient struct {
	transferReq models.TransferSuiRequest
	moveCallReq models.MoveCallRequest
	signReq     models.SignAndExecuteTransactionBlockRequest

	transferRsp models.TxnMetaData
	moveCallRsp models.TxnMetaData
	executeRsp  models.SuiTransactionBlockResponse
}

func (c *fakeSuiWriteClient) TransferSui(ctx context.Context, req models.TransferSuiRequest) (models.TxnMetaData, error) {
	c.transferReq = req
	return c.transferRsp, nil
}

func (c *fakeSuiWriteClient) MoveCall(ctx context.Context, req models.MoveCallRequest) (models.TxnMetaData, error) {
	c.moveCallReq = req
	return c.moveCallRsp, nil
}

func (c *fakeSuiWriteClient) SignAndExecuteTransactionBlock(ctx context.Context, req models.SignAndExecuteTransactionBlockRequest) (models.SuiTransactionBlockResponse, error) {
	c.signReq = req
	return c.executeRsp, nil
}

func TestExecutorRejectsDraftPTBWithoutNetworkCall(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"digest":"fake-digest"}}`))
	}))
	defer server.Close()

	executor := NewExecutor(server.URL)
	digest, err := executor.ExecutePTB(context.Background(), &PTB{
		Commands: []interface{}{
			map[string]interface{}{"TransferObjects": []interface{}{"0xRecipient", uint64(1000)}},
		},
		GasBudget: 10_000_000,
	})

	assert.Error(t, err)
	assert.Empty(t, digest)
	assert.False(t, called, "draft PTBs must not be submitted to Sui RPC")
	assert.Contains(t, err.Error(), "signed transaction")
}

func TestExecutorSubmitsSignedTransactionBlock(t *testing.T) {
	var rpcReq RPCRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&rpcReq))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"digest":"real-digest"}}`))
	}))
	defer server.Close()

	executor := NewExecutor(server.URL)
	digest, err := executor.ExecutePTB(context.Background(), &PTB{
		TransactionBytes: "AAECAwQ=",
		Signatures:       []string{"signed-by-agent"},
		GasBudget:        10_000_000,
	})

	require.NoError(t, err)
	assert.Equal(t, "real-digest", digest)
	assert.Equal(t, "sui_executeTransactionBlock", rpcReq.Method)
	require.Len(t, rpcReq.Params, 4)
	assert.Equal(t, "AAECAwQ=", rpcReq.Params[0])
	assert.Equal(t, "WaitForLocalExecution", rpcReq.Params[3])
}

func TestExecutorExecutesTransferPlanWithSuiSDK(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	client := &fakeSuiWriteClient{
		transferRsp: models.TxnMetaData{TxBytes: "unsigned-transfer-bytes"},
		executeRsp:  models.SuiTransactionBlockResponse{Digest: "signed-transfer-digest"},
	}
	executor := newExecutorWithSuiSDK(
		client,
		suiSignerAccount{Address: "0xSigner", PriKey: privateKey},
		"0xGasCoin",
	)

	digest, err := executor.ExecutePTB(context.Background(), &PTB{
		Action:    "Transfer",
		GasBudget: 50_000_000,
		Transfer: &TransferPlan{
			Recipient:  "0xRecipient",
			AmountMist: 1_000_000_000,
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "signed-transfer-digest", digest)
	assert.Equal(t, "0xSigner", client.transferReq.Signer)
	assert.Equal(t, "0xGasCoin", client.transferReq.SuiObjectId)
	assert.Equal(t, "50000000", client.transferReq.GasBudget)
	assert.Equal(t, "0xRecipient", client.transferReq.Recipient)
	assert.Equal(t, "1000000000", client.transferReq.Amount)
	assert.Equal(t, "unsigned-transfer-bytes", client.signReq.TxnMetaData.TxBytes)
	assert.Equal(t, "WaitForLocalExecution", client.signReq.RequestType)
	assert.True(t, client.signReq.Options.ShowEffects)
}

func TestExecutorRejectsTransferPlanWithoutSuiSDK(t *testing.T) {
	executor := NewExecutor("http://127.0.0.1:9000")
	digest, err := executor.ExecutePTB(context.Background(), &PTB{
		Action: "Transfer",
		Transfer: &TransferPlan{
			Recipient:  "0xRecipient",
			AmountMist: 1_000_000_000,
		},
	})

	assert.Error(t, err)
	assert.Empty(t, digest)
	assert.Contains(t, err.Error(), "sui sdk executor is not configured")
}

func TestExecutorExecutesMoveCallPlanWithSuiSDK(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	client := &fakeSuiWriteClient{
		moveCallRsp: models.TxnMetaData{TxBytes: "unsigned-movecall-bytes"},
		executeRsp:  models.SuiTransactionBlockResponse{Digest: "signed-movecall-digest"},
	}
	executor := newExecutorWithSuiSDK(
		client,
		suiSignerAccount{Address: "0xSigner", PriKey: privateKey},
		"0xGasCoin",
	)

	digest, err := executor.ExecutePTB(context.Background(), &PTB{
		Action:    "Swap",
		GasBudget: 50_000_000,
		MoveCall: &MoveCallPlan{
			PackageObjectID: "0xPackage",
			Module:          "router",
			Function:        "swap_exact_in",
			TypeArguments:   []interface{}{"0x2::sui::SUI"},
			Arguments:       []interface{}{"0xPool", "0xInputCoin", "1000000000"},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "signed-movecall-digest", digest)
	assert.Equal(t, "0xSigner", client.moveCallReq.Signer)
	assert.Equal(t, "0xPackage", client.moveCallReq.PackageObjectId)
	assert.Equal(t, "router", client.moveCallReq.Module)
	assert.Equal(t, "swap_exact_in", client.moveCallReq.Function)
	assert.Equal(t, []interface{}{"0x2::sui::SUI"}, client.moveCallReq.TypeArguments)
	assert.Equal(t, []interface{}{"0xPool", "0xInputCoin", "1000000000"}, client.moveCallReq.Arguments)
	require.NotNil(t, client.moveCallReq.Gas)
	assert.Equal(t, "0xGasCoin", *client.moveCallReq.Gas)
	assert.Equal(t, "50000000", client.moveCallReq.GasBudget)
	assert.Equal(t, "unsigned-movecall-bytes", client.signReq.TxnMetaData.TxBytes)
	assert.Equal(t, "WaitForLocalExecution", client.signReq.RequestType)
}

func TestExecutorRejectsMoveCallPlanWithoutSuiSDK(t *testing.T) {
	executor := NewExecutor("http://127.0.0.1:9000")
	digest, err := executor.ExecutePTB(context.Background(), &PTB{
		Action: "Swap",
		MoveCall: &MoveCallPlan{
			PackageObjectID: "0xPackage",
			Module:          "router",
			Function:        "swap_exact_in",
			Arguments:       []interface{}{"0xPool"},
		},
	})

	assert.Error(t, err)
	assert.Empty(t, digest)
	assert.Contains(t, err.Error(), "sui sdk executor is not configured")
}

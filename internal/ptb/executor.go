package ptb

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/block-vision/sui-go-sdk/models"
	suisigner "github.com/block-vision/sui-go-sdk/signer"
	"github.com/block-vision/sui-go-sdk/sui"
)

type Executor struct {
	rpcURL     string
	httpClient *http.Client

	suiClient      suiWriteClient
	suiAccount     *suiSignerAccount
	suiGasObjectID string
}

type suiWriteClient interface {
	TransferSui(ctx context.Context, req models.TransferSuiRequest) (models.TxnMetaData, error)
	MoveCall(ctx context.Context, req models.MoveCallRequest) (models.TxnMetaData, error)
	SignAndExecuteTransactionBlock(ctx context.Context, req models.SignAndExecuteTransactionBlockRequest) (models.SuiTransactionBlockResponse, error)
}

type suiSignerAccount struct {
	Address string
	PriKey  ed25519.PrivateKey
}

type SDKExecutorConfig struct {
	SignerMnemonic   string
	SignerPrivateKey string
	GasObjectID      string
}

type RPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type RPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewExecutor(rpcURL string) *Executor {
	return &Executor{
		rpcURL: rpcURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func NewSDKExecutor(rpcURL string, cfg SDKExecutorConfig) (*Executor, error) {
	account, err := newSuiSignerAccount(cfg)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.GasObjectID) == "" {
		return nil, fmt.Errorf("sui gas object id is required")
	}

	executor := NewExecutor(rpcURL)
	executor.suiClient = sui.NewSuiClient(rpcURL)
	executor.suiAccount = &account
	executor.suiGasObjectID = strings.TrimSpace(cfg.GasObjectID)
	return executor, nil
}

func newExecutorWithSuiSDK(client suiWriteClient, account suiSignerAccount, gasObjectID string) *Executor {
	executor := NewExecutor("")
	executor.suiClient = client
	executor.suiAccount = &account
	executor.suiGasObjectID = strings.TrimSpace(gasObjectID)
	return executor
}

func newSuiSignerAccount(cfg SDKExecutorConfig) (suiSignerAccount, error) {
	privateKey := strings.TrimSpace(cfg.SignerPrivateKey)
	mnemonic := strings.TrimSpace(cfg.SignerMnemonic)

	var account *suisigner.Signer
	var err error
	switch {
	case privateKey != "":
		account, err = suisigner.NewSignerWithSecretKey(privateKey)
	case mnemonic != "":
		account, err = suisigner.NewSignertWithMnemonic(mnemonic)
	default:
		return suiSignerAccount{}, fmt.Errorf("sui signer private key or mnemonic is required")
	}
	if err != nil {
		return suiSignerAccount{}, err
	}

	return suiSignerAccount{
		Address: account.Address,
		PriKey:  account.PriKey,
	}, nil
}

func (e *Executor) ExecutePTB(ctx context.Context, ptb *PTB) (string, error) {
	if ptb == nil {
		return "", fmt.Errorf("ptb is required")
	}
	if ptb.Transfer != nil {
		return e.executeTransferSui(ctx, ptb)
	}
	if ptb.MoveCall != nil {
		return e.executeMoveCall(ctx, ptb)
	}
	if strings.TrimSpace(ptb.TransactionBytes) == "" {
		return "", fmt.Errorf("signed transaction bytes are required for Sui RPC execution")
	}
	if len(ptb.Signatures) == 0 {
		return "", fmt.Errorf("signed transaction signatures are required for Sui RPC execution")
	}
	for i, sig := range ptb.Signatures {
		if strings.TrimSpace(sig) == "" {
			return "", fmt.Errorf("signature %d is empty", i)
		}
	}

	options := map[string]interface{}{
		"showEffects": true,
		"showEvents":  true,
	}
	params := []interface{}{
		ptb.TransactionBytes,
		ptb.Signatures,
		options,
		"WaitForLocalExecution",
	}

	reqBody, err := json.Marshal(RPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "sui_executeTransactionBlock",
		Params:  params,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.rpcURL, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute PTB: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("PTB execution failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	var rpcResp RPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if rpcResp.Error != nil {
		return "", fmt.Errorf("PTB RPC error: %s", rpcResp.Error.Message)
	}

	// Extract digest from result
	var result struct {
		Digest string `json:"digest"`
	}
	if err := json.Unmarshal(rpcResp.Result, &result); err != nil {
		return "", fmt.Errorf("failed to parse result: %w", err)
	}

	return result.Digest, nil
}

func (e *Executor) executeMoveCall(ctx context.Context, ptb *PTB) (string, error) {
	if e.suiClient == nil || e.suiAccount == nil || strings.TrimSpace(e.suiGasObjectID) == "" {
		return "", fmt.Errorf("sui sdk executor is not configured")
	}
	if ptb.MoveCall == nil {
		return "", fmt.Errorf("move call plan is required")
	}
	if strings.TrimSpace(ptb.MoveCall.PackageObjectID) == "" {
		return "", fmt.Errorf("move package object id is required")
	}
	if strings.TrimSpace(ptb.MoveCall.Module) == "" {
		return "", fmt.Errorf("move module is required")
	}
	if strings.TrimSpace(ptb.MoveCall.Function) == "" {
		return "", fmt.Errorf("move function is required")
	}
	if len(ptb.MoveCall.Arguments) == 0 {
		return "", fmt.Errorf("move arguments are required")
	}
	if ptb.GasBudget == 0 {
		return "", fmt.Errorf("gas budget must be greater than zero")
	}

	txn, err := e.suiClient.MoveCall(ctx, models.MoveCallRequest{
		Signer:          e.suiAccount.Address,
		PackageObjectId: ptb.MoveCall.PackageObjectID,
		Module:          ptb.MoveCall.Module,
		Function:        ptb.MoveCall.Function,
		TypeArguments:   ptb.MoveCall.TypeArguments,
		Arguments:       ptb.MoveCall.Arguments,
		Gas:             &e.suiGasObjectID,
		GasBudget:       strconv.FormatUint(ptb.GasBudget, 10),
	})
	if err != nil {
		return "", fmt.Errorf("failed to build Sui move call transaction: %w", err)
	}
	return e.signAndExecute(ctx, txn, "Sui move call")
}

func (e *Executor) executeTransferSui(ctx context.Context, ptb *PTB) (string, error) {
	if e.suiClient == nil || e.suiAccount == nil || strings.TrimSpace(e.suiGasObjectID) == "" {
		return "", fmt.Errorf("sui sdk executor is not configured")
	}
	if ptb.Transfer == nil {
		return "", fmt.Errorf("transfer plan is required")
	}
	if strings.TrimSpace(ptb.Transfer.Recipient) == "" {
		return "", fmt.Errorf("transfer recipient is required")
	}
	if ptb.Transfer.AmountMist == 0 {
		return "", fmt.Errorf("transfer amount must be greater than zero")
	}
	if ptb.GasBudget == 0 {
		return "", fmt.Errorf("gas budget must be greater than zero")
	}

	txn, err := e.suiClient.TransferSui(ctx, models.TransferSuiRequest{
		Signer:      e.suiAccount.Address,
		SuiObjectId: e.suiGasObjectID,
		GasBudget:   strconv.FormatUint(ptb.GasBudget, 10),
		Recipient:   ptb.Transfer.Recipient,
		Amount:      strconv.FormatUint(ptb.Transfer.AmountMist, 10),
	})
	if err != nil {
		return "", fmt.Errorf("failed to build Sui transfer transaction: %w", err)
	}
	return e.signAndExecute(ctx, txn, "Sui transfer")
}

func (e *Executor) signAndExecute(ctx context.Context, txn models.TxnMetaData, label string) (string, error) {
	resp, err := e.suiClient.SignAndExecuteTransactionBlock(ctx, models.SignAndExecuteTransactionBlockRequest{
		TxnMetaData: txn,
		PriKey:      e.suiAccount.PriKey,
		Options: models.SuiTransactionBlockOptions{
			ShowInput:    true,
			ShowRawInput: true,
			ShowEffects:  true,
			ShowEvents:   true,
		},
		RequestType: "WaitForLocalExecution",
	})
	if err != nil {
		return "", fmt.Errorf("failed to sign and execute %s transaction: %w", label, err)
	}
	if strings.TrimSpace(resp.Digest) == "" {
		return "", fmt.Errorf("%s execution returned empty digest", label)
	}
	return resp.Digest, nil
}

func (e *Executor) GetTransaction(ctx context.Context, digest string) (*TransactionResult, error) {
	reqBody, _ := json.Marshal(RPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "sui_getTransactionBlock",
		Params:  []interface{}{digest},
	})

	req, err := http.NewRequestWithContext(ctx, "POST", e.rpcURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var rpcResp RPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, err
	}

	var result TransactionResult
	if err := json.Unmarshal(rpcResp.Result, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

type TransactionResult struct {
	Digest    string `json:"digest"`
	Confirmed bool   `json:"confirmed"`
	Effects   string `json:"effects"`
}

package ptb

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
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
	suiCLI         string
	suiCLIConfig   string
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
	SuiCLIPath       string
	SuiCLIConfigPath string
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
	executor.suiCLI = strings.TrimSpace(cfg.SuiCLIPath)
	if executor.suiCLI == "" {
		executor.suiCLI = "sui"
	}
	executor.suiCLIConfig = strings.TrimSpace(cfg.SuiCLIConfigPath)
	if executor.suiCLIConfig == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("cannot determine home directory: %w", err)
		}
		executor.suiCLIConfig = homeDir + "/.sui/sui_config/client.yaml"
	}
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
	resp, err := e.ExecutePTBDetailed(ctx, ptb)
	if err != nil {
		return "", err
	}
	return resp.Digest, nil
}

func (e *Executor) ExecutePTBDetailed(ctx context.Context, ptb *PTB) (*models.SuiTransactionBlockResponse, error) {
	if ptb == nil {
		return nil, fmt.Errorf("ptb is required")
	}
	if e.isDemoMode() {
		return e.executeDemoPTBDetailed(ctx, ptb)
	}
	if ptb.Transfer != nil {
		return e.executeTransferSuiDetailed(ctx, ptb)
	}
	if ptb.MoveCall != nil {
		return e.executeMoveCallDetailed(ctx, ptb)
	}
	if strings.TrimSpace(ptb.TransactionBytes) == "" {
		return nil, fmt.Errorf("signed transaction bytes are required for Sui RPC execution")
	}
	if len(ptb.Signatures) == 0 {
		return nil, fmt.Errorf("signed transaction signatures are required for Sui RPC execution")
	}
	for i, sig := range ptb.Signatures {
		if strings.TrimSpace(sig) == "" {
			return nil, fmt.Errorf("signature %d is empty", i)
		}
	}

	options := map[string]interface{}{
		"showEffects":       true,
		"showEvents":        true,
		"showObjectChanges": true,
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
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.rpcURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute PTB: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, fmt.Errorf("PTB execution failed: status %d (body unreadable: %v)", resp.StatusCode, readErr)
		}
		return nil, fmt.Errorf("PTB execution failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	var rpcResp RPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("PTB RPC error: %s", rpcResp.Error.Message)
	}

	var result models.SuiTransactionBlockResponse
	if err := json.Unmarshal(rpcResp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse result: %w", err)
	}
	if strings.TrimSpace(result.Digest) == "" {
		return nil, fmt.Errorf("ptb execution returned empty digest")
	}
	return &result, nil
}

func (e *Executor) executeMoveCallDetailed(ctx context.Context, ptb *PTB) (*models.SuiTransactionBlockResponse, error) {
	if e.suiClient == nil || e.suiAccount == nil || strings.TrimSpace(e.suiGasObjectID) == "" {
		return nil, fmt.Errorf("sui sdk executor is not configured")
	}
	if ptb.MoveCall == nil {
		return nil, fmt.Errorf("move call plan is required")
	}
	if strings.TrimSpace(ptb.MoveCall.PackageObjectID) == "" {
		return nil, fmt.Errorf("move package object id is required")
	}
	if strings.TrimSpace(ptb.MoveCall.Module) == "" {
		return nil, fmt.Errorf("move module is required")
	}
	if strings.TrimSpace(ptb.MoveCall.Function) == "" {
		return nil, fmt.Errorf("move function is required")
	}
	if len(ptb.MoveCall.Arguments) == 0 {
		return nil, fmt.Errorf("move arguments are required")
	}
	if ptb.GasBudget == 0 {
		return nil, fmt.Errorf("gas budget must be greater than zero")
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
		return nil, fmt.Errorf("failed to build Sui move call transaction: %w", err)
	}
	return e.signAndExecute(ctx, txn, "Sui move call")
}

// ExecutePTBViaSuiCLI shells out to `sui client ptb` to submit a
// multi-command programmable transaction block. Used when the on-chain
// function returns values that the gateway needs to forward (e.g. the
// approved Coin from agent_wallet::execute_trade goes back to the agent).
//
// Required PTB shape:
//   - PTB.MoveCall for the primary call (the "first" command)
//   - PTB.Commands[].TransferObjects: {"to": addr, "from_result_idx": N}
//     appended after the MoveCall to forward a result by index
func (e *Executor) ExecutePTBViaSuiCLI(ctx context.Context, ptb *PTB) (string, error) {
	if e.isDemoMode() {
		return e.executeDemoPTB(ctx, ptb)
	}
	if e.suiCLI == "" {
		return "", fmt.Errorf("sui CLI path is not configured")
	}
	if ptb == nil || ptb.MoveCall == nil {
		return "", fmt.Errorf("move call plan is required for CLI PTB execution")
	}

	args := []string{"client", "ptb",
		"--client-config", e.suiCLIConfig,
		"--move-call", ptb.MoveCall.PackageObjectID, ptb.MoveCall.Module, ptb.MoveCall.Function,
	}
	if len(ptb.MoveCall.TypeArguments) > 0 {
		for _, t := range ptb.MoveCall.TypeArguments {
			args = append(args, fmt.Sprintf("%v", t))
		}
	}
	args = append(args, "--gas-budget", strconv.FormatUint(ptb.GasBudget, 10),
		"--json", "--sender", e.suiAccount.Address, "--yes")

	// Optional: append TransferObjects commands from PTB.Commands
	for _, raw := range ptb.Commands {
		cmd, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		transfer, ok := cmd["TransferObjects"].([]interface{})
		if !ok || len(transfer) < 2 {
			continue
		}
		recipient, ok := transfer[0].(string)
		if !ok {
			continue
		}
		args = append(args, "--transfer-objects", fmt.Sprintf("[Result(%v)]", transfer[1]), recipient)
	}

	cmd := exec.CommandContext(ctx, e.suiCLI, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("sui client ptb failed: %w (%s)", err, string(out))
	}
	// Try to parse the digest out of the JSON output. We accept any string
	// with a 0x... base58-ish 30+ char substring as the digest.
	for _, line := range strings.Split(string(out), "\n") {
		if i := strings.Index(line, `"digest":`); i >= 0 {
			rest := line[i+len(`"digest":`):]
			rest = strings.Trim(rest, " ,\"")
			if strings.HasPrefix(rest, "0x") && len(rest) > 30 {
				return rest, nil
			}
		}
	}
	return "", fmt.Errorf("could not parse digest from sui client ptb output: %s", string(out))
}

func (e *Executor) executeTransferSuiDetailed(ctx context.Context, ptb *PTB) (*models.SuiTransactionBlockResponse, error) {
	if e.suiClient == nil || e.suiAccount == nil || strings.TrimSpace(e.suiGasObjectID) == "" {
		return nil, fmt.Errorf("sui sdk executor is not configured")
	}
	if ptb.Transfer == nil {
		return nil, fmt.Errorf("transfer plan is required")
	}
	if strings.TrimSpace(ptb.Transfer.Recipient) == "" {
		return nil, fmt.Errorf("transfer recipient is required")
	}
	if ptb.Transfer.AmountMist == 0 {
		return nil, fmt.Errorf("transfer amount must be greater than zero")
	}
	if ptb.GasBudget == 0 {
		return nil, fmt.Errorf("gas budget must be greater than zero")
	}

	txn, err := e.suiClient.TransferSui(ctx, models.TransferSuiRequest{
		Signer:      e.suiAccount.Address,
		SuiObjectId: e.suiGasObjectID,
		GasBudget:   strconv.FormatUint(ptb.GasBudget, 10),
		Recipient:   ptb.Transfer.Recipient,
		Amount:      strconv.FormatUint(ptb.Transfer.AmountMist, 10),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to build Sui transfer transaction: %w", err)
	}
	return e.signAndExecute(ctx, txn, "Sui transfer")
}

func (e *Executor) signAndExecute(ctx context.Context, txn models.TxnMetaData, label string) (*models.SuiTransactionBlockResponse, error) {
	resp, err := e.suiClient.SignAndExecuteTransactionBlock(ctx, models.SignAndExecuteTransactionBlockRequest{
		TxnMetaData: txn,
		PriKey:      e.suiAccount.PriKey,
		Options: models.SuiTransactionBlockOptions{
			ShowInput:         true,
			ShowRawInput:      true,
			ShowEffects:       true,
			ShowEvents:        true,
			ShowObjectChanges: true,
		},
		RequestType: "WaitForLocalExecution",
	})
	if err != nil {
		// Surface the underlying Sui error message (often contains abort code).
		return nil, fmt.Errorf("failed to sign and execute %s transaction: %w", label, err)
	}
	// Surface the on-chain failure if the response carried one. We only
	// check Status.Status — the SDK doesn't always populate
	// ConfirmedLocalExecution in older responses and treating false as failure
	// would break the demo mock.
	if resp.Effects.Status.Status == "failure" {
		return nil, fmt.Errorf("%s transaction failed on-chain: %s", label, resp.Effects.Status.Error)
	}
	if strings.TrimSpace(resp.Digest) == "" {
		return nil, fmt.Errorf("%s execution returned empty digest", label)
	}
	return &resp, nil
}



package ptb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Executor struct {
	rpcURL     string
	httpClient *http.Client
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

func (e *Executor) ExecutePTB(ctx context.Context, ptb *PTB) (string, error) {
	// Convert PTB to Sui RPC format
	params := []interface{}{
		map[string]interface{}{
			"commands":    ptb.Commands,
			"gas_budget": ptb.GasBudget,
		},
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

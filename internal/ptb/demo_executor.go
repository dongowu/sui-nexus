package ptb

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/block-vision/sui-go-sdk/models"
)

// NewDemoExecutor returns an executor that never touches Sui RPC. It is used by
// the hackathon demo mode so judges can exercise the full product flow locally.
func NewDemoExecutor() *Executor {
	return &Executor{
		rpcURL: "demo://sui-nexus",
	}
}

func (e *Executor) isDemoMode() bool {
	return e != nil && e.rpcURL == "demo://sui-nexus"
}

func (e *Executor) executeDemoPTB(_ context.Context, ptb *PTB) (string, error) {
	resp, err := e.executeDemoPTBDetailed(context.Background(), ptb)
	if err != nil {
		return "", err
	}
	return resp.Digest, nil
}

func (e *Executor) executeDemoPTBDetailed(_ context.Context, ptb *PTB) (*models.SuiTransactionBlockResponse, error) {
	if ptb == nil {
		return nil, fmt.Errorf("ptb is required")
	}
	payload, err := json.Marshal(ptb)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal demo PTB: %w", err)
	}
	sum := sha256.Sum256(payload)
	return &models.SuiTransactionBlockResponse{
		Digest: "demo-" + hex.EncodeToString(sum[:])[:44],
	}, nil
}

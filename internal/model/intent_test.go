package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIntentRequest_JSON(t *testing.T) {
	req := IntentRequest{
		TaskID:    "test-task-001",
		APIKey:    "ak_test",
		Signature: "sig_xxx",
		Timestamp: 1713000000,
		Agents: []AgentShare{
			{Address: "0xAnalyst", Share: 0.1},
			{Address: "0xTrader", Share: 0.2},
		},
		Action: "Swap",
		Params: ActionParams{
			Amount:   "1000",
			TokenIn:  "USDT",
			TokenOut: "SUI",
			Slippage: "0.5",
			DestAddr: "0xUser",
		},
		ContextPayload: "base64encoded...",
	}

	data, err := json.Marshal(req)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "test-task-001")
	assert.Contains(t, string(data), "Swap")

	var decoded IntentRequest
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, req.TaskID, decoded.TaskID)
	assert.Equal(t, req.Action, decoded.Action)
}

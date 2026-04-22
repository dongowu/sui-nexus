package ptb

import (
	"testing"
	"time"

	"github.com/sui-nexus/gateway/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestBuilder_BuildSwapWithDistribution(t *testing.T) {
	builder := NewBuilder(10_000_000)

	task := &model.Task{
		TaskID: "test-task-001",
		Intent: &model.IntentRequest{
			TaskID:    "test-task-001",
			Action:    "Swap",
			Params: model.ActionParams{
				Amount:   "1000",
				TokenIn:  "USDT",
				TokenOut: "SUI",
				Slippage: "0.5",
			},
			Agents: []model.AgentShare{
				{Address: "0xAnalyst", Share: 0.1},
				{Address: "0xTrader", Share: 0.2},
			},
		},
		BlobID: "abc123blobid",
	}

	ptb, err := builder.Build(task)
	assert.NoError(t, err)
	assert.NotNil(t, ptb)
	assert.Greater(t, len(ptb.Commands), 0)
	assert.Equal(t, uint64(10_000_000), ptb.GasBudget)
}

func TestBuilder_BuildTransfer(t *testing.T) {
	builder := NewBuilder(5_000_000)

	task := &model.Task{
		TaskID: "test-task-002",
		Intent: &model.IntentRequest{
			TaskID:    "test-task-002",
			Action:    "Transfer",
			Params: model.ActionParams{
				Amount:   "500",
				DestAddr: "0xDestination",
			},
		},
		CreatedAt: time.Now(),
	}

	ptb, err := builder.Build(task)
	assert.NoError(t, err)
	assert.NotNil(t, ptb)
	assert.Equal(t, 1, len(ptb.Commands))
}

func TestBuilder_UnsupportedAction(t *testing.T) {
	builder := NewBuilder(5_000_000)

	task := &model.Task{
		TaskID: "test-task-003",
		Intent: &model.IntentRequest{
			TaskID: "test-task-003",
			Action: "Unknown",
			Params: model.ActionParams{},
		},
	}

	ptb, err := builder.Build(task)
	assert.Error(t, err)
	assert.Nil(t, ptb)
	assert.Contains(t, err.Error(), "unsupported action")
}

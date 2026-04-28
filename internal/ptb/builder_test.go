package ptb

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/sui-nexus/gateway/internal/model"
)

func TestBuilder_BuildSwapWithDistribution(t *testing.T) {
	builder := NewBuilder(10_000_000)

	task := &model.Task{
		TaskID: "test-task-001",
		Intent: &model.IntentRequest{
			TaskID: "test-task-001",
			Action: "Swap",
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

func TestBuilder_BuildSwapWithMoveCallPlan(t *testing.T) {
	builder := NewBuilder(10_000_000)

	task := &model.Task{
		TaskID: "test-task-movecall-swap",
		Intent: &model.IntentRequest{
			TaskID: "test-task-movecall-swap",
			Action: "Swap",
			Params: model.ActionParams{
				Amount:              "1000",
				TokenIn:             "USDT",
				TokenOut:            "SUI",
				Slippage:            "0.5",
				MovePackageObjectID: "0xPackage",
				MoveModule:          "router",
				MoveFunction:        "swap_exact_in",
				MoveTypeArguments:   []interface{}{"0x2::sui::SUI"},
				MoveArguments:       []interface{}{"0xPool", "0xInputCoin", "1000000000", "995000000"},
			},
			Agents: []model.AgentShare{
				{Address: "0xAnalyst", Share: 0.1},
			},
		},
	}

	ptb, err := builder.Build(task)
	assert.NoError(t, err)
	assert.NotNil(t, ptb)
	assert.Equal(t, "Swap", ptb.Action)
	assert.NotNil(t, ptb.MoveCall)
	assert.Equal(t, "0xPackage", ptb.MoveCall.PackageObjectID)
	assert.Equal(t, "router", ptb.MoveCall.Module)
	assert.Equal(t, "swap_exact_in", ptb.MoveCall.Function)
	assert.Equal(t, []interface{}{"0x2::sui::SUI"}, ptb.MoveCall.TypeArguments)
	assert.Equal(t, []interface{}{"0xPool", "0xInputCoin", "1000000000", "995000000"}, ptb.MoveCall.Arguments)
}

func TestBuilder_BuildTransfer(t *testing.T) {
	builder := NewBuilder(5_000_000)

	task := &model.Task{
		TaskID: "test-task-002",
		Intent: &model.IntentRequest{
			TaskID: "test-task-002",
			Action: "Transfer",
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
	assert.Equal(t, "Transfer", ptb.Action)
	assert.NotNil(t, ptb.Transfer)
	assert.Equal(t, "0xDestination", ptb.Transfer.Recipient)
	assert.Equal(t, uint64(500_000_000_000), ptb.Transfer.AmountMist)
}

func TestBuilder_BuildSwapWithDistributionRejectsEmptyAgents(t *testing.T) {
	builder := NewBuilder(10_000_000)

	task := &model.Task{
		TaskID: "test-task-empty-agents",
		Intent: &model.IntentRequest{
			TaskID: "test-task-empty-agents",
			Action: "Swap",
			Params: model.ActionParams{
				Amount:   "1000",
				TokenIn:  "USDT",
				TokenOut: "SUI",
			},
			Agents: []model.AgentShare{},
		},
	}

	ptb, err := builder.Build(task)
	assert.Error(t, err)
	assert.Nil(t, ptb)
	assert.Contains(t, err.Error(), "at least one agent")
}

func TestBuilder_BuildSwapWithDistributionRejectsInvalidShares(t *testing.T) {
	builder := NewBuilder(10_000_000)

	task := &model.Task{
		TaskID: "test-task-invalid-shares",
		Intent: &model.IntentRequest{
			TaskID: "test-task-invalid-shares",
			Action: "Swap",
			Params: model.ActionParams{
				Amount:   "1000",
				TokenIn:  "USDT",
				TokenOut: "SUI",
			},
			Agents: []model.AgentShare{
				{Address: "0xAnalyst", Share: 0.8},
				{Address: "0xTrader", Share: 0.3},
			},
		},
	}

	ptb, err := builder.Build(task)
	assert.Error(t, err)
	assert.Nil(t, ptb)
	assert.Contains(t, err.Error(), "total agent share")
}

func TestParseAmountRejectsTrailingCharacters(t *testing.T) {
	amount, err := parseAmount("1000abc")
	assert.Error(t, err)
	assert.Zero(t, amount)
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

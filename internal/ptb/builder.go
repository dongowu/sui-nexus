package ptb

import (
	"fmt"

	"github.com/sui-nexus/gateway/internal/model"
)

type PTB struct {
	Commands  []interface{}
	GasBudget uint64
}

type TransferObject struct {
	Recipient string `json:"recipient"`
	Amount    uint64 `json:"amount"`
}

type Swap struct {
	TokenIn  string `json:"token_in"`
	TokenOut string `json:"token_out"`
	Amount   uint64 `json:"amount"`
	Slippage string `json:"slippage"`
}

type MintMemoryObject struct {
	TaskID string `json:"task_id"`
	BlobID string `json:"blob_id"`
}

type Builder struct {
	gasBudget uint64
}

func NewBuilder(gasBudget uint64) *Builder {
	return &Builder{gasBudget: gasBudget}
}

func (b *Builder) BuildSwapWithDistribution(task *model.Task) (*PTB, error) {
	ptb := &PTB{
		GasBudget: b.gasBudget,
	}

	// Get amount in Sui (smallest unit)
	amount, err := parseAmount(task.Intent.Params.Amount)
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %w", err)
	}

	// Command 1: Transfer objects (simulate moving funds to gateway pool)
	ptb.Commands = append(ptb.Commands, map[string]interface{}{
		"TransferObjects": []interface{}{
			task.Intent.Agents[0].Address,
			amount / 10,
		},
	})

	// Command 2: Call Cetus swap (simulated)
	ptb.Commands = append(ptb.Commands, map[string]interface{}{
		"Call": map[string]interface{}{
			"package":   "0x CetusPackage",
			"module":    "swap",
			"function":  "swap_exact_in",
			"arguments": []interface{}{task.Intent.Params.TokenIn, task.Intent.Params.TokenOut, amount},
		},
	})

	// Command 3: Mint MemoryObject with BlobID
	if task.BlobID != "" {
		ptb.Commands = append(ptb.Commands, map[string]interface{}{
			"MintMemoryObject": map[string]interface{}{
				"task_id": task.TaskID,
				"blob_id": task.BlobID,
			},
		})
	}

	// Command 4: Distribute to agents
	for _, agent := range task.Intent.Agents {
		shareAmount := uint64(float64(amount) * agent.Share)
		ptb.Commands = append(ptb.Commands, map[string]interface{}{
			"TransferObjects": []interface{}{
				agent.Address,
				shareAmount,
			},
		})
	}

	return ptb, nil
}

func (b *Builder) BuildTransfer(task *model.Task) (*PTB, error) {
	ptb := &PTB{
		GasBudget: b.gasBudget,
	}

	amount, err := parseAmount(task.Intent.Params.Amount)
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %w", err)
	}

	ptb.Commands = append(ptb.Commands, map[string]interface{}{
		"TransferObjects": []interface{}{
			task.Intent.Params.DestAddr,
			amount,
		},
	})

	return ptb, nil
}

func (b *Builder) Build(task *model.Task) (*PTB, error) {
	switch task.Intent.Action {
	case "Swap":
		return b.BuildSwapWithDistribution(task)
	case "Transfer":
		return b.BuildTransfer(task)
	default:
		return nil, fmt.Errorf("unsupported action: %s", task.Intent.Action)
	}
}

func parseAmount(amountStr string) (uint64, error) {
	var amount uint64
	_, err := fmt.Sscanf(amountStr, "%d", &amount)
	if err != nil {
		return 0, err
	}
	return amount * 1_000_000_000, nil
}

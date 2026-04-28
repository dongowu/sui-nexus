package ptb

import (
	"fmt"
	"math"
	"math/bits"
	"strconv"
	"strings"

	"github.com/sui-nexus/gateway/internal/model"
)

const (
	mistPerSUI             = 1_000_000_000
	basisPointsDenominator = 10_000
)

type PTB struct {
	// Commands is a gateway-side draft representation used before a Sui SDK
	// signs and serializes the transaction block.
	Action           string        `json:"action,omitempty"`
	Commands         []interface{} `json:"commands,omitempty"`
	GasBudget        uint64        `json:"gas_budget"`
	TransactionBytes string        `json:"transaction_bytes,omitempty"`
	Signatures       []string      `json:"signatures,omitempty"`
	Transfer         *TransferPlan `json:"transfer,omitempty"`
	MoveCall         *MoveCallPlan `json:"move_call,omitempty"`
}

type TransferPlan struct {
	Recipient  string `json:"recipient"`
	AmountMist uint64 `json:"amount_mist"`
}

type MoveCallPlan struct {
	PackageObjectID string        `json:"package_object_id"`
	Module          string        `json:"module"`
	Function        string        `json:"function"`
	TypeArguments   []interface{} `json:"type_arguments,omitempty"`
	Arguments       []interface{} `json:"arguments,omitempty"`
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
	if task == nil || task.Intent == nil {
		return nil, fmt.Errorf("task intent is required")
	}
	if len(task.Intent.Agents) == 0 {
		return nil, fmt.Errorf("at least one agent is required")
	}

	ptb := &PTB{
		Action:    "Swap",
		GasBudget: b.gasBudget,
	}

	// Get amount in Sui (smallest unit)
	amount, err := parseAmount(task.Intent.Params.Amount)
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %w", err)
	}

	agentShares, err := validateAgentShares(task.Intent.Agents)
	if err != nil {
		return nil, err
	}
	moveCall, err := buildMoveCallPlan(task.Intent.Params)
	if err != nil {
		return nil, err
	}
	ptb.MoveCall = moveCall

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
	for i, agent := range task.Intent.Agents {
		shareAmount := prorateAmount(amount, agentShares[i])
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
	if task == nil || task.Intent == nil {
		return nil, fmt.Errorf("task intent is required")
	}
	ptb := &PTB{
		Action:    "Transfer",
		GasBudget: b.gasBudget,
	}

	amount, err := parseAmount(task.Intent.Params.Amount)
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %w", err)
	}
	if strings.TrimSpace(task.Intent.Params.DestAddr) == "" {
		return nil, fmt.Errorf("destination address is required")
	}
	ptb.Transfer = &TransferPlan{
		Recipient:  task.Intent.Params.DestAddr,
		AmountMist: amount,
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
	if task == nil || task.Intent == nil {
		return nil, fmt.Errorf("task intent is required")
	}
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
	amountStr = strings.TrimSpace(amountStr)
	if amountStr == "" {
		return 0, fmt.Errorf("amount is required")
	}
	amount, err := strconv.ParseUint(amountStr, 10, 64)
	if err != nil {
		return 0, err
	}
	if amount > (^uint64(0))/mistPerSUI {
		return 0, fmt.Errorf("amount overflows SUI mist conversion")
	}
	return amount * mistPerSUI, nil
}

func validateAgentShares(agents []model.AgentShare) ([]uint64, error) {
	shares := make([]uint64, 0, len(agents))
	var total uint64
	for i, agent := range agents {
		if strings.TrimSpace(agent.Address) == "" {
			return nil, fmt.Errorf("agent %d address is required", i)
		}
		if math.IsNaN(agent.Share) || math.IsInf(agent.Share, 0) || agent.Share <= 0 {
			return nil, fmt.Errorf("agent %d share must be greater than zero", i)
		}
		if agent.Share > 1 {
			return nil, fmt.Errorf("agent %d share must not exceed 1", i)
		}

		basisPoints := uint64(math.Round(agent.Share * basisPointsDenominator))
		if basisPoints == 0 {
			return nil, fmt.Errorf("agent %d share is too small", i)
		}
		total += basisPoints
		shares = append(shares, basisPoints)
	}
	if total > basisPointsDenominator {
		return nil, fmt.Errorf("total agent share must not exceed 1")
	}
	return shares, nil
}

func prorateAmount(amount, basisPoints uint64) uint64 {
	hi, lo := bits.Mul64(amount, basisPoints)
	shareAmount, _ := bits.Div64(hi, lo, basisPointsDenominator)
	return shareAmount
}

func buildMoveCallPlan(params model.ActionParams) (*MoveCallPlan, error) {
	packageObjectID := strings.TrimSpace(params.MovePackageObjectID)
	module := strings.TrimSpace(params.MoveModule)
	function := strings.TrimSpace(params.MoveFunction)

	hasMoveCall := packageObjectID != "" ||
		module != "" ||
		function != "" ||
		len(params.MoveTypeArguments) > 0 ||
		len(params.MoveArguments) > 0
	if !hasMoveCall {
		return nil, nil
	}
	if packageObjectID == "" {
		return nil, fmt.Errorf("move package object id is required")
	}
	if module == "" {
		return nil, fmt.Errorf("move module is required")
	}
	if function == "" {
		return nil, fmt.Errorf("move function is required")
	}
	if len(params.MoveArguments) == 0 {
		return nil, fmt.Errorf("move arguments are required")
	}

	return &MoveCallPlan{
		PackageObjectID: packageObjectID,
		Module:          module,
		Function:        function,
		TypeArguments:   params.MoveTypeArguments,
		Arguments:       params.MoveArguments,
	}, nil
}

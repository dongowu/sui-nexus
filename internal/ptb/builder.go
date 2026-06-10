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

// ────────────────────────────────────────────────────────────
// Agent Wallet PTB builders
// ────────────────────────────────────────────────────────────

// BuildAgentWalletCreate builds a PTB to create a new AgentWallet.
func (b *Builder) BuildAgentWalletCreate(
	ownerAddress string,
	agentAddress string,
	budgetCapMist uint64,
	_ []string,
	timeEndEpoch uint64,
	packageID string,
) (*PTB, error) {
	if strings.TrimSpace(ownerAddress) == "" {
		return nil, fmt.Errorf("owner address is required")
	}
	if strings.TrimSpace(agentAddress) == "" {
		return nil, fmt.Errorf("agent address is required")
	}
	if budgetCapMist == 0 {
		return nil, fmt.Errorf("budget cap must be greater than zero")
	}
	if strings.TrimSpace(packageID) == "" {
		return nil, fmt.Errorf("package id is required")
	}

	return &PTB{
		Action:    "CreateWallet",
		GasBudget: b.gasBudget,
		MoveCall: &MoveCallPlan{
			PackageObjectID: packageID,
			Module:          "agent_wallet",
			Function:        "create_wallet",
			TypeArguments:   []interface{}{},
			Arguments: []interface{}{
				ownerAddress,
				agentAddress,
				fmt.Sprintf("%d", budgetCapMist),
				fmt.Sprintf("%d", timeEndEpoch),
			},
		},
	}, nil
}

// BuildAgentWalletExecuteTrade builds a PTB for an agent to execute a trade
// through the wallet (policy check + budget deduct + coin split).
//
// expectedPrice is the agent's committed minimum price floor (MIST). The
// Move contract refuses to execute trades that pass expected_price == 0 —
// that's the on-chain no-quote-without-floor guard. The rich slippage math
// against `ObservedPrice` happens in the gateway Guardian.
func (b *Builder) BuildAgentWalletExecuteTrade(
	walletID string,
	agentAddr string,
	amountMist uint64,
	protocol string,
	expectedPrice uint64,
	description string,
	packageID string,
) (*PTB, error) {
	if strings.TrimSpace(walletID) == "" {
		return nil, fmt.Errorf("wallet id is required")
	}
	if strings.TrimSpace(agentAddr) == "" {
		return nil, fmt.Errorf("agent address is required")
	}
	if amountMist == 0 {
		return nil, fmt.Errorf("amount must be greater than zero")
	}
	if strings.TrimSpace(protocol) == "" {
		return nil, fmt.Errorf("protocol is required")
	}
	if strings.TrimSpace(packageID) == "" {
		return nil, fmt.Errorf("package id is required")
	}

	return &PTB{
		Action:    "ExecuteTrade",
		GasBudget: b.gasBudget,
		MoveCall: &MoveCallPlan{
			PackageObjectID: packageID,
			Module:          "agent_wallet",
			Function:        "execute_trade",
			TypeArguments:   []interface{}{},
			Arguments: []interface{}{
				walletID,
				agentAddr,
				fmt.Sprintf("%d", amountMist),
				protocol,
				fmt.Sprintf("%d", expectedPrice),
				description,
			},
		},
	}, nil
}

// BuildAgentWalletRevoke builds a PTB for the owner to revoke a wallet.
func (b *Builder) BuildAgentWalletRevoke(
	walletID string,
	ownerAddress string,
	packageID string,
) (*PTB, error) {
	if strings.TrimSpace(walletID) == "" {
		return nil, fmt.Errorf("wallet id is required")
	}
	if strings.TrimSpace(ownerAddress) == "" {
		return nil, fmt.Errorf("owner address is required")
	}
	if strings.TrimSpace(packageID) == "" {
		return nil, fmt.Errorf("package id is required")
	}

	return &PTB{
		Action:    "RevokeWallet",
		GasBudget: b.gasBudget,
		MoveCall: &MoveCallPlan{
			PackageObjectID: packageID,
			Module:          "agent_wallet",
			Function:        "revoke",
			TypeArguments:   []interface{}{},
			Arguments: []interface{}{
				walletID,
				ownerAddress,
			},
		},
	}, nil
}

// BuildAgentWalletDeposit builds a PTB for the owner to deposit SUI into a wallet.
func (b *Builder) BuildAgentWalletDeposit(
	walletID string,
	ownerAddress string,
	coinID string,
	packageID string,
) (*PTB, error) {
	if strings.TrimSpace(walletID) == "" {
		return nil, fmt.Errorf("wallet id is required")
	}
	if strings.TrimSpace(ownerAddress) == "" {
		return nil, fmt.Errorf("owner address is required")
	}
	if strings.TrimSpace(coinID) == "" {
		return nil, fmt.Errorf("coin id is required")
	}
	if strings.TrimSpace(packageID) == "" {
		return nil, fmt.Errorf("package id is required")
	}

	return &PTB{
		Action:    "DepositWallet",
		GasBudget: b.gasBudget,
		MoveCall: &MoveCallPlan{
			PackageObjectID: packageID,
			Module:          "agent_wallet",
			Function:        "deposit",
			TypeArguments:   []interface{}{},
			Arguments: []interface{}{
				walletID,
				ownerAddress,
				coinID,
			},
		},
	}, nil
}

// BuildMemoryObjectCreate builds a PTB to persist a Walrus blob reference
// on-chain through agent_memory::create_memory.
func (b *Builder) BuildMemoryObjectCreate(taskID, blobID, packageID string) (*PTB, error) {
	if strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("task id is required")
	}
	if strings.TrimSpace(blobID) == "" {
		return nil, fmt.Errorf("blob id is required")
	}
	if strings.TrimSpace(packageID) == "" {
		return nil, fmt.Errorf("package id is required")
	}

	return &PTB{
		Action:    "CreateMemoryObject",
		GasBudget: b.gasBudget,
		MoveCall: &MoveCallPlan{
			PackageObjectID: packageID,
			Module:          "agent_memory",
			Function:        "create_memory",
			TypeArguments:   []interface{}{},
			Arguments: []interface{}{
				taskID,
				blobID,
			},
		},
	}, nil
}

// BuildAgentWalletExecuteDeepBook builds a PTB for placing a limit order on
// DeepBook V3. Called after execute_trade succeeds.
func (b *Builder) BuildAgentWalletExecuteDeepBook(
	poolID string,
	balanceManagerID string,
	tradeProofID string,
	clientOrderID uint64,
	orderType uint8,
	price uint64,
	quantity uint64,
	isBid bool,
	expireTimestamp uint64,
	deepBookPackageID string,
) (*PTB, error) {
	if strings.TrimSpace(poolID) == "" {
		return nil, fmt.Errorf("deepbook pool id is required")
	}
	if strings.TrimSpace(deepBookPackageID) == "" {
		return nil, fmt.Errorf("deepbook package id is required")
	}

	return &PTB{
		Action:    "DeepBookOrder",
		GasBudget: b.gasBudget,
		MoveCall: &MoveCallPlan{
			PackageObjectID: deepBookPackageID,
			Module:          "clob",
			Function:        "place_limit_order",
			TypeArguments: []interface{}{
				"0x2::sui::SUI", // BaseAsset
				"0x2::sui::SUI", // QuoteAsset - placeholder, replace with USDC
			},
			Arguments: []interface{}{
				poolID,
				balanceManagerID,
				tradeProofID,
				fmt.Sprintf("%d", clientOrderID),
				fmt.Sprintf("%d", orderType),
				"0", // self_matching_option: cancel_oldest
				fmt.Sprintf("%d", price),
				fmt.Sprintf("%d", quantity),
				fmt.Sprintf("%v", isBid),
				"true", // pay_with_deep
				fmt.Sprintf("%d", expireTimestamp),
				"0x6", // Clock shared object address on Sui
			},
		},
	}, nil
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

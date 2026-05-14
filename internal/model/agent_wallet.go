package model

// ────────────────────────────────────────────────────────────
// Agent Wallet domain types
// ────────────────────────────────────────────────────────────

// WalletPolicy defines the spending policy for an agent wallet.
type WalletPolicy struct {
	BudgetCapMist    uint64   `json:"budget_cap_mist"`
	BudgetSpentMist  uint64   `json:"budget_spent_mist"`
	TimeStart        uint64   `json:"time_start"`
	TimeEnd          uint64   `json:"time_end"`
	AllowedProtocols []string `json:"allowed_protocols"`
}

// ActivityEntry is a single on-chain activity log record.
type ActivityEntry struct {
	Timestamp   uint64 `json:"timestamp"`
	Action      string `json:"action"`
	AmountMist  uint64 `json:"amount_mist"`
	Protocol    string `json:"protocol"`
	Description string `json:"description"`
}

// AgentWallet is the full wallet state, cached in Redis and queryable on-chain.
type AgentWallet struct {
	WalletID     string          `json:"wallet_id"`
	Owner        string          `json:"owner"`
	AgentAddress string          `json:"agent_address"`
	Policy       WalletPolicy    `json:"policy"`
	IsActive     bool            `json:"is_active"`
	BalanceMist  uint64          `json:"balance_mist"`
	ActivityLog  []ActivityEntry `json:"activity_log"`
}

// ────────────────────────────────────────────────────────────
// API request / response types
// ────────────────────────────────────────────────────────────

// CreateWalletRequest is the owner's request to create a new agent wallet.
type CreateWalletRequest struct {
	AgentAddress     string   `json:"agent_address" binding:"required"`
	BudgetCapMist    uint64   `json:"budget_cap_mist" binding:"required"`
	AllowedProtocols []string `json:"allowed_protocols"`
	TimeEndEpoch     uint64   `json:"time_end_epoch"` // must be > current epoch
}

// AgentExecuteRequest is an agent's request to execute a trade within policy.
type AgentExecuteRequest struct {
	WalletID      string `json:"wallet_id" binding:"required"`
	AmountMist    uint64 `json:"amount_mist" binding:"required"`
	Protocol      string `json:"protocol" binding:"required"`
	ExpectedPrice uint64 `json:"expected_price"` // Guardian: expected price in MIST (0 = skip check)
	Description   string `json:"description"`

	// zkLogin session
	SessionToken string `json:"session_token" binding:"required"`
	UserAddress  string `json:"user_address" binding:"required"`
}

// RevokeWalletRequest is the owner's request to revoke a wallet.
type RevokeWalletRequest struct {
	// zkLogin session for owner identity
	SessionToken string `json:"session_token,omitempty"`
	UserAddress  string `json:"user_address,omitempty"`
}

// WalletResponse is the standard API response for wallet operations.
type WalletResponse struct {
	WalletID         string          `json:"wallet_id,omitempty"`
	Owner            string          `json:"owner,omitempty"`
	AgentAddress     string          `json:"agent_address,omitempty"`
	Policy           *WalletPolicy   `json:"policy,omitempty"`
	IsActive         bool            `json:"is_active"`
	BalanceMist      uint64          `json:"balance_mist"`
	ActivityLog      []ActivityEntry `json:"activity_log,omitempty"`
	TxDigest         string          `json:"tx_digest,omitempty"`
	DeepBookTxDigest string          `json:"deepbook_tx_digest,omitempty"`
	Guardian         *GuardianReport `json:"guardian,omitempty"`
	Error            *ErrorDetail    `json:"error,omitempty"`
}

// GuardianReport explains the pre-flight risk decision for judge-visible demos.
type GuardianReport struct {
	Passed   bool   `json:"passed"`
	RiskType string `json:"risk_type,omitempty"`
	Message  string `json:"message,omitempty"`
}

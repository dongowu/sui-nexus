package model

type AgentShare struct {
	Address string  `json:"address"`
	Share   float64 `json:"share"` // 比例，如 0.1 表示 10%
}

type IntentRequest struct {
	TaskID         string       `json:"task_id" binding:"required"`
	APIKey         string       `json:"api_key" binding:"required"`
	Signature      string       `json:"signature" binding:"required"`
	Timestamp      int64        `json:"timestamp" binding:"required"`
	Agents         []AgentShare `json:"agents" binding:"required"`
	Action         string       `json:"action" binding:"required"` // Swap, Stake, Transfer
	Params         ActionParams `json:"params" binding:"required"`
	ContextPayload string       `json:"context_payload"` // Base64 encoded
}

type ActionParams struct {
	Amount    string `json:"amount"`
	TokenIn   string `json:"token_in"`
	TokenOut  string `json:"token_out"`
	Slippage  string `json:"slippage"`
	DestAddr  string `json:"dest_addr,omitempty"`
}

type IntentResponse struct {
	TaskID   string       `json:"task_id"`
	Status   TaskStatus   `json:"status"`
	TxDigest string       `json:"tx_digest,omitempty"`
	Error    *ErrorDetail `json:"error,omitempty"`
}

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusProcessing TaskStatus = "processing"
	StatusCompleted  TaskStatus = "completed"
	StatusFailed     TaskStatus = "failed"
)

type Task struct {
	TaskID     string          `json:"task_id"`
	Status     TaskStatus      `json:"status"`
	Intent     *IntentRequest  `json:"intent"`
	TxDigest   string          `json:"tx_digest,omitempty"`
	BlobID     string          `json:"blob_id,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	RetryCount int             `json:"retry_count"`
}
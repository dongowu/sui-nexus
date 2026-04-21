package model

import "time"

type TaskEvent struct {
	EventType string       `json:"event_type"` // created, blob_id_received, ptb_built, on_chain, failed
	TaskID    string       `json:"task_id"`
	Payload   interface{}  `json:"payload,omitempty"`
}

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
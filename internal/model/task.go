package model

type TaskEvent struct {
	EventType string       `json:"event_type"` // created, blob_id_received, ptb_built, on_chain, failed
	TaskID    string       `json:"task_id"`
	Payload   interface{}  `json:"payload,omitempty"`
}

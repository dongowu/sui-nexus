package gateway

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/sui-nexus/gateway/internal/model"
	"github.com/sui-nexus/gateway/pkg/hmac"
)

type fakeProducer struct {
	tasks []*model.Task
	err   error
}

func (p *fakeProducer) SendIntent(task *model.Task) error {
	p.tasks = append(p.tasks, task)
	return p.err
}

func setupTestRouter() (*gin.Engine, *hmac.Signer) {
	gin.SetMode(gin.TestMode)
	signer := hmac.NewSigner("test-secret", 300)
	handler := &Handler{signer: signer}
	r := NewRouter(handler, signer)
	return r, signer
}

func TestHandleIntent_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	signer := hmac.NewSigner("test-secret", 300)
	producer := &fakeProducer{}
	handler := NewHandler(signer, producer, nil)
	r := NewRouter(handler, signer)

	taskID := "test-task-001"
	timestamp := time.Now().Unix()
	signature := signer.Sign(taskID, timestamp, "Swap", "1000")

	reqBody := model.IntentRequest{
		TaskID:    taskID,
		APIKey:    "test-key",
		Signature: signature,
		Timestamp: timestamp,
		Action:    "Swap",
		Params: model.ActionParams{
			Amount:   "1000",
			TokenIn:  "USDT",
			TokenOut: "SUI",
		},
		Agents: []model.AgentShare{
			{Address: "0xAnalyst", Share: 0.1},
		},
	}

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/api/v1/intent", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	req.Header.Set("X-Signature", signature)
	req.Header.Set("X-Timestamp", strconv.FormatInt(timestamp, 10))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Len(t, producer.tasks, 1)
	assert.Equal(t, "test-key", producer.tasks[0].Intent.APIKey)
}

func TestHandleIntent_AcceptsHeaderCredentialsWithoutBodyDuplication(t *testing.T) {
	r, signer := setupTestRouter()

	taskID := "test-task-header-auth"
	timestamp := time.Now().Unix()
	signature := signer.Sign(taskID, timestamp, "Swap", "1000")

	reqBody := map[string]interface{}{
		"task_id": taskID,
		"action":  "Swap",
		"params": map[string]interface{}{
			"amount":    "1000",
			"token_in":  "USDT",
			"token_out": "SUI",
		},
		"agents": []map[string]interface{}{
			{"address": "0xAnalyst", "share": 0.1},
		},
	}

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/api/v1/intent", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	req.Header.Set("X-Signature", signature)
	req.Header.Set("X-Timestamp", strconv.FormatInt(timestamp, 10))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandleIntent_RejectsInvalidHeaderSignature(t *testing.T) {
	r, signer := setupTestRouter()

	taskID := "test-task-invalid-header-signature"
	timestamp := time.Now().Unix()
	bodySignature := signer.Sign(taskID, timestamp, "Swap", "1000")

	reqBody := model.IntentRequest{
		TaskID:    taskID,
		APIKey:    "test-key",
		Signature: bodySignature,
		Timestamp: timestamp,
		Action:    "Swap",
		Params: model.ActionParams{
			Amount:   "1000",
			TokenIn:  "USDT",
			TokenOut: "SUI",
		},
		Agents: []model.AgentShare{
			{Address: "0xAnalyst", Share: 0.1},
		},
	}

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/api/v1/intent", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	req.Header.Set("X-Signature", "not-the-valid-signature")
	req.Header.Set("X-Timestamp", strconv.FormatInt(timestamp, 10))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHandleIntent_QueueUnavailableReturnsServiceUnavailable(t *testing.T) {
	r, signer := setupTestRouter()

	taskID := "test-task-queue-unavailable"
	timestamp := time.Now().Unix()
	signature := signer.Sign(taskID, timestamp, "Swap", "1000")

	reqBody := model.IntentRequest{
		TaskID:    taskID,
		APIKey:    "test-key",
		Signature: signature,
		Timestamp: timestamp,
		Action:    "Swap",
		Params: model.ActionParams{
			Amount:   "1000",
			TokenIn:  "USDT",
			TokenOut: "SUI",
		},
		Agents: []model.AgentShare{
			{Address: "0xAnalyst", Share: 0.1},
		},
	}

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/api/v1/intent", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	req.Header.Set("X-Signature", signature)
	req.Header.Set("X-Timestamp", strconv.FormatInt(timestamp, 10))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandleIntent_MissingSignature(t *testing.T) {
	r, _ := setupTestRouter()

	reqBody := model.IntentRequest{
		TaskID: "test-task-001",
	}

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/api/v1/intent", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHandleHealth(t *testing.T) {
	r, _ := setupTestRouter()

	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

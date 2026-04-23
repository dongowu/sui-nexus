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
	"github.com/sui-nexus/gateway/internal/model"
	"github.com/sui-nexus/gateway/pkg/hmac"
	"github.com/stretchr/testify/assert"
)

func setupTestRouter() (*gin.Engine, *hmac.Signer) {
	gin.SetMode(gin.TestMode)
	signer := hmac.NewSigner("test-secret", 300)
	handler := &Handler{signer: signer}
	r := NewRouter(handler, signer)
	return r, signer
}

func TestHandleIntent_Success(t *testing.T) {
	r, signer := setupTestRouter()

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

	// Should return Accepted (will fail queue but signature verification works)
	assert.Equal(t, http.StatusAccepted, w.Code)
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
package gateway

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sui-nexus/gateway/internal/model"
	"github.com/sui-nexus/gateway/internal/storage"
	"github.com/sui-nexus/gateway/pkg/hmac"
)

type IntentProducer interface {
	SendIntent(task *model.Task) error
}

type Handler struct {
	signer     *hmac.Signer
	producer   IntentProducer
	redisStore *storage.RedisStore
}

func NewHandler(signer *hmac.Signer, producer IntentProducer, redisStore *storage.RedisStore) *Handler {
	return &Handler{
		signer:     signer,
		producer:   producer,
		redisStore: redisStore,
	}
}

func (h *Handler) HandleIntent(c *gin.Context) {
	var req model.IntentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.IntentResponse{
			TaskID: req.TaskID,
			Status: model.StatusFailed,
			Error: &model.ErrorDetail{
				Code:    "ERR_INVALID_REQUEST",
				Message: err.Error(),
			},
		})
		return
	}

	// Verify HMAC signature
	apiKey, _ := c.Get("api_key")
	signature, _ := c.Get("auth_signature")
	timestamp, _ := c.Get("auth_timestamp")
	authSignature, _ := signature.(string)
	authTimestamp, _ := timestamp.(int64)
	if err := h.signer.Verify(req.TaskID, authTimestamp, req.Action, req.Params.Amount, authSignature); err != nil {
		log.Printf("Signature verification failed for task %s: %v", req.TaskID, err)
		c.JSON(http.StatusUnauthorized, model.IntentResponse{
			TaskID: req.TaskID,
			Status: model.StatusFailed,
			Error: &model.ErrorDetail{
				Code:    "ERR_AUTH_FAILED",
				Message: "Invalid or expired signature",
			},
		})
		return
	}
	if apiKeyValue, ok := apiKey.(string); ok {
		req.APIKey = apiKeyValue
	}
	req.Signature = authSignature
	req.Timestamp = authTimestamp

	// Create task
	task := &model.Task{
		TaskID:    req.TaskID,
		Status:    model.StatusPending,
		Intent:    &req,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if h.producer == nil {
		log.Printf("Kafka producer not available, rejecting task %s", req.TaskID)
		c.JSON(http.StatusServiceUnavailable, model.IntentResponse{
			TaskID: req.TaskID,
			Status: model.StatusFailed,
			Error: &model.ErrorDetail{
				Code:    "ERR_QUEUE_UNAVAILABLE",
				Message: "Task queue unavailable",
			},
		})
		return
	}

	// Save to Redis
	if h.redisStore != nil {
		ctx := context.Background()
		if err := h.redisStore.SaveTask(ctx, task); err != nil {
			log.Printf("Failed to save task to Redis: %v", err)
		}
	}

	// Send to Kafka
	if err := h.producer.SendIntent(task); err != nil {
		log.Printf("Failed to send task to Kafka: %v", err)
		c.JSON(http.StatusInternalServerError, model.IntentResponse{
			TaskID: req.TaskID,
			Status: model.StatusFailed,
			Error: &model.ErrorDetail{
				Code:    "ERR_QUEUE_FAILED",
				Message: "Failed to queue task",
			},
		})
		return
	}

	log.Printf("Task %s received and queued", req.TaskID)

	c.JSON(http.StatusAccepted, model.IntentResponse{
		TaskID: req.TaskID,
		Status: model.StatusPending,
	})
}

func (h *Handler) HandleGetTask(c *gin.Context) {
	taskID := c.Param("task_id")

	if h.redisStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Storage unavailable"})
		return
	}

	ctx := context.Background()
	task, err := h.redisStore.GetTask(ctx, taskID)
	if err != nil || task == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	c.JSON(http.StatusOK, task)
}

func (h *Handler) HandleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

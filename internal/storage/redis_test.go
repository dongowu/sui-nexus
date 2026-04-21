package storage

import (
	"context"
	"testing"
	"time"

	"github.com/sui-nexus/gateway/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestRedisStore_SaveAndGetTask(t *testing.T) {
	store, err := NewRedisStore("localhost:6379")
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}
	defer store.Close()

	ctx := context.Background()
	task := &model.Task{
		TaskID:    "test-task-" + time.Now().Format("150405"),
		Status:    model.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = store.SaveTask(ctx, task)
	assert.NoError(t, err)

	retrieved, err := store.GetTask(ctx, task.TaskID)
	assert.NoError(t, err)
	assert.Equal(t, task.TaskID, retrieved.TaskID)
	assert.Equal(t, task.Status, retrieved.Status)
}

func TestRedisStore_GetNonExistent(t *testing.T) {
	store, err := NewRedisStore("localhost:6379")
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}
	defer store.Close()

	ctx := context.Background()
	task, err := store.GetTask(ctx, "non-existent-task")
	assert.NoError(t, err)
	assert.Nil(t, task)
}

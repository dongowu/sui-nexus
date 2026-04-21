package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sui-nexus/gateway/internal/model"
)

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(addr string) (*RedisStore, error) {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}
	return &RedisStore{client: client}, nil
}

func (s *RedisStore) SaveTask(ctx context.Context, task *model.Task) error {
	data, err := json.Marshal(task)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("task:%s", task.TaskID)
	return s.client.Set(ctx, key, data, 24*time.Hour).Err()
}

func (s *RedisStore) GetTask(ctx context.Context, taskID string) (*model.Task, error) {
	key := fmt.Sprintf("task:%s", taskID)
	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	var task model.Task
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, err
	}
	return &task, nil
}

func (s *RedisStore) UpdateTaskStatus(ctx context.Context, taskID string, status model.TaskStatus) error {
	task, err := s.GetTask(ctx, taskID)
	if err != nil || task == nil {
		return err
	}
	task.Status = status
	task.UpdatedAt = time.Now()
	return s.SaveTask(ctx, task)
}

func (s *RedisStore) Close() error {
	return s.client.Close()
}

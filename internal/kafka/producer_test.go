package kafka

import (
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/sui-nexus/gateway/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestProducer_SendIntent_Mock(t *testing.T) {
	config := sarama.NewConfig()
	producer, err := sarama.NewSyncProducer([]string{"localhost:9092"}, config)
	if err != nil {
		t.Skip("Kafka not available, skipping test")
	}
	defer producer.Close()

	task := &model.Task{
		TaskID:    "test-task-" + time.Now().Format("150405"),
		Status:    model.StatusPending,
		CreatedAt: time.Now(),
	}

	// Mock send - just verify producer works
	_, _, err = producer.SendMessage(&sarama.ProducerMessage{
		Topic: "test-topic",
		Key:   sarama.StringEncoder(task.TaskID),
		Value: sarama.ByteEncoder([]byte(`{}`)),
	})
	assert.NoError(t, err)
}

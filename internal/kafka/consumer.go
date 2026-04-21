package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/IBM/sarama"
	"github.com/sui-nexus/gateway/internal/model"
)

type TaskHandler func(ctx context.Context, task *model.Task) error

type Consumer struct {
	consumer sarama.ConsumerGroup
	topic    string
	handler  TaskHandler
}

func NewConsumer(brokers []string, groupID, topic string, handler TaskHandler) (*Consumer, error) {
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
	config.Consumer.Offsets.Initial = sarama.OffsetNewest

	consumer, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka consumer: %w", err)
	}

	return &Consumer{
		consumer: consumer,
		topic:    topic,
		handler:  handler,
	}, nil
}

func (c *Consumer) Start(ctx context.Context) error {
	topics := []string{c.topic}
	handler := &consumerGroupHandler{handler: c.handler}

	go func() {
		for {
			if err := c.consumer.Consume(ctx, topics, handler); err != nil {
				log.Printf("Kafka consumer error: %v", err)
			}
			if ctx.Err() != nil {
				return
			}
		}
	}()

	log.Printf("Kafka consumer started, listening on topic: %s", c.topic)
	return nil
}

func (c *Consumer) Close() error {
	return c.consumer.Close()
}

type consumerGroupHandler struct {
	handler TaskHandler
}

func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			var task model.Task
			if err := json.Unmarshal(msg.Value, &task); err != nil {
				log.Printf("Failed to unmarshal task: %v", err)
				session.MarkMessage(msg, "")
				continue
			}
			if err := h.handler(session.Context(), &task); err != nil {
				log.Printf("Failed to handle task %s: %v", task.TaskID, err)
			}
			session.MarkMessage(msg, "")
		case <-session.Context().Done():
			return nil
		}
	}
}

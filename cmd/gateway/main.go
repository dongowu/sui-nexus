package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sui-nexus/gateway/internal/config"
	"github.com/sui-nexus/gateway/internal/gateway"
	"github.com/sui-nexus/gateway/internal/kafka"
	"github.com/sui-nexus/gateway/internal/model"
	"github.com/sui-nexus/gateway/internal/ptb"
	"github.com/sui-nexus/gateway/internal/storage"
	"github.com/sui-nexus/gateway/internal/walrus"
	"github.com/sui-nexus/gateway/pkg/hmac"
)

func main() {
	cfg := config.Load()

	// Initialize Redis
	redisStore, err := storage.NewRedisStore(cfg.RedisAddr)
	if err != nil {
		log.Printf("Warning: Redis connection failed: %v (continuing without Redis)", err)
		redisStore = nil
	}

	// Initialize Kafka Producer
	var producer *kafka.Producer
	producer, err = kafka.NewProducer(cfg.KafkaBrokers, "sui-nexus-intents")
	if err != nil {
		log.Printf("Warning: Kafka connection failed: %v (continuing without Kafka)", err)
		producer = nil
	}

	// Initialize HMAC Signer
	signer := hmac.NewSigner(cfg.HMACSecretKey, cfg.ReplayWindowSec)

	// Initialize Handler
	handler := gateway.NewHandler(signer, producer, redisStore)

	// Initialize Router
	router := gateway.NewRouter(handler, signer)

	// Start Kafka Consumer (PTB Builder loop)
	if producer != nil {
		ptbBuilder := ptb.NewBuilder(cfg.SuiGasBudget)
		walrusClient := walrus.NewClient(cfg.WalrusAPIURL)
		executor, err := ptb.NewSDKExecutor(cfg.SuiRPCURL, ptb.SDKExecutorConfig{
			SignerMnemonic:   cfg.SuiSignerMnemonic,
			SignerPrivateKey: cfg.SuiSignerPrivateKey,
			GasObjectID:      cfg.SuiGasObjectID,
		})
		if err != nil {
			log.Printf("Warning: Sui SDK executor not configured: %v (signed transaction bytes only)", err)
			executor = ptb.NewExecutor(cfg.SuiRPCURL)
		}

		consumer, err := kafka.NewConsumer(cfg.KafkaBrokers, "sui-nexus-group", "sui-nexus-intents",
			buildTaskHandler(ptbBuilder, walrusClient, executor, redisStore))
		if err != nil {
			log.Printf("Warning: Kafka consumer failed: %v", err)
		} else {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if err := consumer.Start(ctx); err != nil {
				log.Printf("Warning: Consumer start failed: %v", err)
			}
		}
	}

	// Start HTTP Server
	srv := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: router,
	}

	go func() {
		log.Printf("Sui-Nexus Gateway starting on port %s", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	if redisStore != nil {
		redisStore.Close()
	}
	if producer != nil {
		producer.Close()
	}

	log.Println("Server exited")
}

func buildTaskHandler(
	ptbBuilder *ptb.Builder,
	walrusClient *walrus.Client,
	executor *ptb.Executor,
	redisStore *storage.RedisStore,
) kafka.TaskHandler {
	return func(ctx context.Context, task *model.Task) error {
		log.Printf("Processing task %s", task.TaskID)

		// Update status to processing
		if redisStore != nil {
			redisStore.UpdateTaskStatus(ctx, task.TaskID, model.StatusProcessing)
		}

		// Write context to Walrus if present
		if task.Intent != nil && task.Intent.ContextPayload != "" {
			blobID, err := walrusClient.Write(ctx, []byte(task.Intent.ContextPayload))
			if err != nil {
				log.Printf("Walrus write failed for task %s: %v", task.TaskID, err)
			} else {
				task.BlobID = blobID
				log.Printf("Walrus blob %s stored for task %s", blobID, task.TaskID)
			}
		}

		// Build PTB
		ptbTxn, err := ptbBuilder.Build(task)
		if err != nil {
			log.Printf("PTB build failed for task %s: %v", task.TaskID, err)
			return err
		}

		// Execute PTB
		digest, err := executor.ExecutePTB(ctx, ptbTxn)
		if err != nil {
			log.Printf("PTB execution failed for task %s: %v", task.TaskID, err)
			// Retry logic would go here
			return err
		}

		log.Printf("PTB executed for task %s, digest: %s", task.TaskID, digest)
		task.TxDigest = digest

		// Update final status
		if redisStore != nil {
			task.Status = model.StatusCompleted
			redisStore.SaveTask(ctx, task)
		}

		return nil
	}
}

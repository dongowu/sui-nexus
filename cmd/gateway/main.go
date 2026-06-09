package main

import (
	"context"
	"fmt"
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

	// Initialize WebSocket Hub
	hub := gateway.NewHub()
	go hub.Run()

	// Initialize NLP Client (optional - gateway works without it)
	var nlpClient *gateway.NLPClient
	nlpEndpoint := cfg.NLPServiceEndpoint
	if nlpEndpoint != "" {
		nlpClient = gateway.NewNLPClient(nlpEndpoint)
		log.Printf("NLP client configured: %s", nlpEndpoint)
	}

	// Initialize PTB Builder and Executor (shared across handler and consumer)
	ptbBuilder := ptb.NewBuilder(cfg.SuiGasBudget)
	executor, err := ptb.NewSDKExecutor(cfg.SuiRPCURL, ptb.SDKExecutorConfig{
		SignerMnemonic:   cfg.SuiSignerMnemonic,
		SignerPrivateKey: cfg.SuiSignerPrivateKey,
		GasObjectID:      cfg.SuiGasObjectID,
	})
	if cfg.HackathonDemoMode {
		log.Println("Hackathon demo mode enabled: using local demo executor")
		executor = ptb.NewDemoExecutor()
	} else if err != nil {
		log.Printf("Warning: Sui SDK executor not configured: %v (signed transaction bytes only)", err)
		executor = ptb.NewExecutor(cfg.SuiRPCURL)
	}

	// Initialize Handler
	handler := gateway.NewHandler(signer, producer, redisStore, hub, nlpClient, cfg)
	if cfg.HackathonDemoMode {
		handler.EnableSynchronousDemoProcessing(ptbBuilder, executor)
	}

	// Initialize AgentWalletHandler if enabled
	var agentWalletHandler *gateway.AgentWalletHandler
	if cfg.AgentWalletEnabled {
		if cfg.AgentWalletPackageID == "" {
			log.Println("Warning: AGENT_WALLET_PACKAGE_ID not set, agent wallet disabled")
		} else {
			agentWalletHandler = gateway.NewAgentWalletHandler(
				cfg,
				executor,
				ptbBuilder,
				redisStore,
				hub,
				handler.GetEphemeralKeyManager(),
			)
			log.Println("Agent Wallet handler initialized")
		}
	}

	// Initialize Router
	router := gateway.NewRouter(handler, signer, agentWalletHandler)

	// Start Kafka Consumer (PTB Builder loop)
	if producer != nil {
		walrusClient := walrus.NewClient(cfg.WalrusAPIURL)

		consumer, err := kafka.NewConsumer(cfg.KafkaBrokers, "sui-nexus-group", "sui-nexus-intents",
			buildTaskHandler(ptbBuilder, walrusClient, executor, redisStore, hub, cfg))
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
	hub *gateway.Hub,
	cfg *config.Config,
) kafka.TaskHandler {
	return func(ctx context.Context, task *model.Task) error {
		log.Printf("Processing task %s", task.TaskID)

		// Update status to processing
		task.Status = model.StatusProcessing
		task.UpdatedAt = time.Now()
		persistTaskState(ctx, redisStore, hub, task)

		// Write context to Walrus if present
		if task.Intent != nil && task.Intent.ContextPayload != "" {
			blobID, err := walrusClient.Write(ctx, []byte(task.Intent.ContextPayload))
			if err != nil {
				return failTask(ctx, redisStore, hub, task, fmt.Errorf("walrus write failed: %w", err))
			} else {
				task.BlobID = blobID
				log.Printf("Walrus blob %s stored for task %s", blobID, task.TaskID)
			}
			if cfg == nil || cfg.AgentWalletPackageID == "" {
				return failTask(ctx, redisStore, hub, task, fmt.Errorf("memory object package id is not configured"))
			}
			memoryPTB, err := ptbBuilder.BuildMemoryObjectCreate(task.TaskID, task.BlobID, cfg.AgentWalletPackageID)
			if err != nil {
				return failTask(ctx, redisStore, hub, task, fmt.Errorf("memory object build failed: %w", err))
			}
			memoryResp, err := executor.ExecutePTBDetailed(ctx, memoryPTB)
			if err != nil {
				return failTask(ctx, redisStore, hub, task, fmt.Errorf("memory object execution failed: %w", err))
			}
			task.MemoryTxDigest = memoryResp.Digest
			task.TxDigest = memoryResp.Digest
			log.Printf("MemoryObject minted for task %s, digest: %s", task.TaskID, memoryResp.Digest)
		}

		// Build PTB
		ptbTxn, err := ptbBuilder.Build(task)
		if err != nil {
			return failTask(ctx, redisStore, hub, task, fmt.Errorf("ptb build failed: %w", err))
		}

		if isExecutablePTB(ptbTxn) {
			// Execute PTB
			digest, err := executor.ExecutePTB(ctx, ptbTxn)
			if err != nil {
				return failTask(ctx, redisStore, hub, task, fmt.Errorf("ptb execution failed: %w", err))
			}

			log.Printf("PTB executed for task %s, digest: %s", task.TaskID, digest)
			task.TxDigest = digest
		} else if task.MemoryTxDigest == "" {
			return failTask(ctx, redisStore, hub, task, fmt.Errorf("task did not produce an executable Sui transaction"))
		}
		task.Status = model.StatusCompleted
		task.UpdatedAt = time.Now()

		// Update final status
		persistTaskState(ctx, redisStore, hub, task)

		return nil
	}
}

func isExecutablePTB(ptbTxn *ptb.PTB) bool {
	if ptbTxn == nil {
		return false
	}
	return ptbTxn.Transfer != nil || ptbTxn.MoveCall != nil || ptbTxn.TransactionBytes != ""
}

func persistTaskState(ctx context.Context, redisStore *storage.RedisStore, hub *gateway.Hub, task *model.Task) {
	if redisStore != nil {
		if err := redisStore.SaveTask(ctx, task); err != nil {
			log.Printf("Failed to persist task %s: %v", task.TaskID, err)
		}
	}
	if hub != nil {
		hub.BroadcastTask(task)
	}
}

func failTask(ctx context.Context, redisStore *storage.RedisStore, hub *gateway.Hub, task *model.Task, err error) error {
	task.Status = model.StatusFailed
	task.UpdatedAt = time.Now()
	task.RetryCount++
	persistTaskState(ctx, redisStore, hub, task)
	log.Printf("Task %s failed: %v", task.TaskID, err)
	return err
}

package config

import (
	"os"
	"strconv"
)

type Config struct {
	ServerPort    string
	SuiRPCURL     string
	WalrusAPIURL  string
	KafkaBrokers  []string
	RedisAddr     string

	HMACSecretKey  string
	ReplayWindowSec int64
}

func Load() *Config {
	replayWindow, _ := strconv.ParseInt(getEnv("REPLAY_WINDOW_SEC", "300"), 10, 64)
	return &Config{
		ServerPort:     getEnv("SERVER_PORT", "8080"),
		SuiRPCURL:      getEnv("SUI_RPC_URL", "https://fullnode.testnet.sui.io"),
		WalrusAPIURL:   getEnv("WALRUS_API_URL", "https://walrus.testnet.sui.io"),
		KafkaBrokers:   []string{getEnv("KAFKA_BROKERS", "localhost:9092")},
		RedisAddr:      getEnv("REDIS_ADDR", "localhost:6379"),
		HMACSecretKey:  getEnv("HMAC_SECRET_KEY", "dev-secret-key-change-in-prod"),
		ReplayWindowSec: replayWindow,
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
package config

import (
	"os"
	"strconv"
)

type Config struct {
	ServerPort          string
	SuiRPCURL           string
	SuiSignerMnemonic   string
	SuiSignerPrivateKey string
	SuiGasObjectID      string
	SuiGasBudget        uint64
	WalrusAPIURL        string
	KafkaBrokers        []string
	RedisAddr           string
	HMACSecretKey       string
	ReplayWindowSec     int64

	// NLP service configuration
	NLPServiceEndpoint string

	// zkLogin configuration
	ZkLoginEnabled      bool
	ZkLoginProvider     string // google | twitch
	ZkLoginClientID     string
	ZkLoginClientSecret string
	ZkLoginRedirectURL  string
	ZkLoginMaxEpoch     int64

	// Agent Wallet configuration
	AgentWalletEnabled   bool
	AgentWalletPackageID string // deployed sui_nexus package ID on testnet

	// DeepBook configuration
	DeepBookPackageID string // DeepBook V3 package ID
	DeepBookPoolID    string // default trading pool ID
}

func Load() *Config {
	replayWindow, _ := strconv.ParseInt(getEnv("REPLAY_WINDOW_SEC", "300"), 10, 64)
	gasBudget := parseUintEnv("SUI_GAS_BUDGET", 10_000_000)
	maxEpoch, _ := strconv.ParseInt(getEnv("ZKLOGIN_MAX_EPOCH", "30"), 10, 64)
	zkLoginEnabled := os.Getenv("ZKLOGIN_ENABLED") == "true"
	agentWalletEnabled := os.Getenv("AGENT_WALLET_ENABLED") == "true"
	return &Config{
		ServerPort:           getEnv("SERVER_PORT", "8080"),
		SuiRPCURL:            getEnv("SUI_RPC_URL", "https://fullnode.testnet.sui.io"),
		SuiSignerMnemonic:    getEnv("SUI_SIGNER_MNEMONIC", ""),
		SuiSignerPrivateKey:  getEnv("SUI_SIGNER_PRIVATE_KEY", ""),
		SuiGasObjectID:       getEnv("SUI_GAS_OBJECT_ID", ""),
		SuiGasBudget:         gasBudget,
		WalrusAPIURL:         getEnv("WALRUS_API_URL", "https://walrus.testnet.sui.io"),
		KafkaBrokers:         []string{getEnv("KAFKA_BROKERS", "localhost:9092")},
		RedisAddr:            getEnv("REDIS_ADDR", "localhost:6379"),
		HMACSecretKey:        getEnv("HMAC_SECRET_KEY", "dev-secret-key-change-in-prod"),
		ReplayWindowSec:      replayWindow,
		NLPServiceEndpoint:   getEnv("NLP_SERVICE_ENDPOINT", "http://localhost:8081"),
		ZkLoginEnabled:       zkLoginEnabled,
		ZkLoginProvider:      getEnv("ZKLOGIN_PROVIDER", "google"),
		ZkLoginClientID:      getEnv("ZKLOGIN_CLIENT_ID", ""),
		ZkLoginClientSecret: getEnv("ZKLOGIN_CLIENT_SECRET", ""),
		ZkLoginRedirectURL:  getEnv("ZKLOGIN_REDIRECT_URL", "http://localhost:8080/api/v1/auth/zklogin/callback"),
		ZkLoginMaxEpoch:      maxEpoch,
		AgentWalletEnabled:   agentWalletEnabled,
		AgentWalletPackageID: getEnv("AGENT_WALLET_PACKAGE_ID", ""),
		DeepBookPackageID:    getEnv("DEEPBOOK_PACKAGE_ID", ""),
		DeepBookPoolID:       getEnv("DEEPBOOK_POOL_ID", ""),
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func parseUintEnv(key string, defaultVal uint64) uint64 {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	parsed, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return defaultVal
	}
	return parsed
}

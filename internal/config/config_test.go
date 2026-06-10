package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadSuiSigningConfig(t *testing.T) {
	t.Setenv("SUI_SIGNER_PRIVATE_KEY", "suiprivkey-test")
	t.Setenv("SUI_GAS_OBJECT_ID", "0xGasCoin")
	t.Setenv("SUI_GAS_BUDGET", "50000000")

	cfg := Load()

	assert.Equal(t, "suiprivkey-test", cfg.SuiSignerPrivateKey)
	assert.Equal(t, "0xGasCoin", cfg.SuiGasObjectID)
	assert.Equal(t, uint64(50_000_000), cfg.SuiGasBudget)
}

func TestLoadDefaultsSuiGasBudget(t *testing.T) {
	cfg := Load()

	assert.Equal(t, uint64(10_000_000), cfg.SuiGasBudget)
}

func TestLoadHackathonDemoModeEnablesAgentWalletWithDeployedPackage(t *testing.T) {
	t.Setenv("HACKATHON_DEMO_MODE", "true")
	t.Setenv("AGENT_WALLET_ENABLED", "")
	t.Setenv("AGENT_WALLET_PACKAGE_ID", "")

	cfg := Load()

	assert.True(t, cfg.HackathonDemoMode)
	assert.True(t, cfg.AgentWalletEnabled)
	assert.Equal(t, "0x262b81797305980a5ddf2c509a6ac8fb9577dee6ac6c96ceba6580bd3dde5058", cfg.AgentWalletPackageID)
}

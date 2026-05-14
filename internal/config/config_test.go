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
	assert.Equal(t, "0x28c35c355590d81c80f86b43b42d21041fdbc0ab34546ff558b48270a4ff277d", cfg.AgentWalletPackageID)
}

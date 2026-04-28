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

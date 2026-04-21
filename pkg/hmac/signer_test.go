package hmac

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSigner_SignAndVerify(t *testing.T) {
	signer := NewSigner("test-secret", 300)
	timestamp := time.Now().Unix()

	sig := signer.Sign("task-001", timestamp, "Swap", "1000")
	assert.NotEmpty(t, sig)

	err := signer.Verify("task-001", timestamp, "Swap", "1000", sig)
	assert.NoError(t, err)
}

func TestSigner_InvalidSignature(t *testing.T) {
	signer := NewSigner("test-secret", 300)
	timestamp := time.Now().Unix()

	err := signer.Verify("task-001", timestamp, "Swap", "1000", "wrong-signature")
	assert.ErrorIs(t, err, ErrInvalidSignature)
}

func TestSigner_ReplayAttack(t *testing.T) {
	signer := NewSigner("test-secret", 300)
	oldTimestamp := time.Now().Add(-10 * time.Minute).Unix()

	sig := signer.Sign("task-001", oldTimestamp, "Swap", "1000")
	err := signer.Verify("task-001", oldTimestamp, "Swap", "1000", sig)
	assert.ErrorIs(t, err, ErrReplayAttack)
}

func TestSigner_DifferentMessage(t *testing.T) {
	signer := NewSigner("test-secret", 300)
	timestamp := time.Now().Unix()

	sig := signer.Sign("task-001", timestamp, "Swap", "1000")
	err := signer.Verify("task-001", timestamp, "Swap", "2000", sig)
	assert.ErrorIs(t, err, ErrInvalidSignature)
}
package hmac

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

type Signer struct {
	secretKey    []byte
	replayWindow time.Duration
}

func NewSigner(secretKey string, replayWindowSec int64) *Signer {
	return &Signer{
		secretKey:    []byte(secretKey),
		replayWindow: time.Duration(replayWindowSec) * time.Second,
	}
}

func (s *Signer) Sign(taskID string, timestamp int64, action string, amount string) string {
	message := fmt.Sprintf("%s:%d:%s:%s", taskID, timestamp, action, amount)
	h := hmac.New(sha256.New, s.secretKey)
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

func (s *Signer) Verify(taskID string, timestamp int64, action, amount, signature string) error {
	if !s.isTimestampValid(timestamp) {
		return ErrReplayAttack
	}
	expectedSig := s.Sign(taskID, timestamp, action, amount)
	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		return ErrInvalidSignature
	}
	return nil
}

func (s *Signer) isTimestampValid(timestamp int64) bool {
	now := time.Now().Unix()
	return timestamp >= now-int64(s.replayWindow/time.Second) && timestamp <= now+int64(s.replayWindow/time.Second)
}

func (s *Signer) ValidateTimestamp(timestamp int64) bool {
	return s.isTimestampValid(timestamp)
}

var (
	ErrReplayAttack     = fmt.Errorf("replay attack detected: timestamp outside window")
	ErrInvalidSignature = fmt.Errorf("invalid HMAC signature")
)
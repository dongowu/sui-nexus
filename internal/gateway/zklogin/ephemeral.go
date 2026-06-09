package zklogin

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// EphemeralKeyManager manages temporary keys for zkLogin sessions.
type EphemeralKeyManager struct {
	keys     map[string]*EphemeralKey // keyed by session_token
	maxEpoch int64
	mu       sync.RWMutex
}

// EphemeralKey represents a temporary key for a zkLogin session.
type EphemeralKey struct {
	// OAuth identity
	JWT     string // raw JWT token
	Subject string // JWT sub claim
	Email   string // JWT email claim
	Issuer  string // e.g. "https://accounts.google.com"

	// zkLogin derivation params (for client-side proof generation)
	Salt          string // random BN254 field element (hex)
	JwtRandomness string // 16 random bytes for nonce (hex)

	// Ephemeral key pair (secp256k1)
	PrivateKey []byte // ephemeral private key
	PublicKey  string // ephemeral public key (uncompressed hex)

	// Session
	UserAddress string    // zkLogin-derived Sui address (submitted by client after proof gen)
	AddressSeed string    // address seed (submitted by client after proof gen)
	IssuedAt    time.Time // when the session was created
	MaxEpoch    int64     // max epoch for key validity
	ProofRaw    string    // base64-encoded Groth16 proof (submitted by client)
}

// NewEphemeralKeyManager creates a new ephemeral key manager.
func NewEphemeralKeyManager(maxEpoch int64) *EphemeralKeyManager {
	return &EphemeralKeyManager{
		keys:     make(map[string]*EphemeralKey),
		maxEpoch: maxEpoch,
	}
}

// CreateSession creates a new zkLogin session with ephemeral key.
// This is called after OAuth exchange, BEFORE client-side proof generation.
// The returned session_token identifies this session.
func (m *EphemeralKeyManager) CreateSession(jwt, subject, email, issuer string) (*EphemeralKey, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate ephemeral key pair
	privKey, err := generateSecp256k1Key()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate ephemeral key: %w", err)
	}

	pubKey, err := deriveSecp256k1PublicKey(privKey)
	if err != nil {
		return nil, "", fmt.Errorf("failed to derive public key: %w", err)
	}

	// Generate salt and randomness
	salt, err := GenerateSalt()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate salt: %w", err)
	}

	jwtRandomness, err := GenerateJwtRandomness()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate jwt randomness: %w", err)
	}

	key := &EphemeralKey{
		JWT:           jwt,
		Subject:       subject,
		Email:         email,
		Issuer:        issuer,
		Salt:          salt,
		JwtRandomness: jwtRandomness,
		PrivateKey:    privKey,
		PublicKey:     pubKey,
		IssuedAt:      time.Now(),
		MaxEpoch:      m.maxEpoch,
	}

	// Generate session token
	sessionToken := generateSessionToken(key)
	m.keys[sessionToken] = key

	return key, sessionToken, nil
}

// SubmitProof records the client-generated proof and marks the session as verified.
func (m *EphemeralKeyManager) SubmitProof(sessionToken, userAddress, addressSeed, proof string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, exists := m.keys[sessionToken]
	if !exists {
		return fmt.Errorf("session not found")
	}

	if time.Now().After(expirationTime(key, m.maxEpoch)) {
		delete(m.keys, sessionToken)
		return fmt.Errorf("session expired")
	}

	key.UserAddress = userAddress
	key.AddressSeed = addressSeed
	key.ProofRaw = proof

	return nil
}

// IsValid checks if a session token is valid and the proof has been submitted.
func (m *EphemeralKeyManager) IsValid(userAddress, sessionToken string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key, exists := m.keys[sessionToken]
	if !exists {
		return false
	}

	if time.Now().After(expirationTime(key, m.maxEpoch)) {
		return false
	}

	// Session must have a verified proof (user address set)
	if key.UserAddress == "" {
		return false
	}

	return key.UserAddress == userAddress
}

// GetSessionForToken returns the session data for a token (for returning to client).
func (m *EphemeralKeyManager) GetSessionForToken(sessionToken string) *ZkLoginSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key, exists := m.keys[sessionToken]
	if !exists {
		return nil
	}

	return &ZkLoginSession{
		JWT:              key.JWT,
		Salt:             key.Salt,
		JwtRandomness:    key.JwtRandomness,
		KeyClaimName:     "sub",
		KeyClaimValue:    key.Subject,
		Audience:         "", // filled by handler from config
		Issuer:           key.Issuer,
		EphemeralPrivKey: fmt.Sprintf("0x%x", key.PrivateKey),
		EphemeralPubKey:  key.PublicKey,
		MaxEpoch:         key.MaxEpoch,
		Email:            key.Email,
	}
}

// ────────────────────────────────────────────────────────────
// Helpers
// ────────────────────────────────────────────────────────────

func expirationTime(key *EphemeralKey, maxEpoch int64) time.Time {
	return key.IssuedAt.Add(time.Duration(maxEpoch) * 24 * time.Hour)
}

func generateSessionToken(key *EphemeralKey) string {
	data := fmt.Sprintf("%s:%s:%d", key.PublicKey, key.Salt, key.IssuedAt.UnixNano())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

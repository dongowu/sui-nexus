package zklogin

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/crypto/secp256k1"
)

// ────────────────────────────────────────────────────────────
// zkLogin Session Parameters (returned to client for proof generation)
// ────────────────────────────────────────────────────────────

// ZkLoginSession contains all parameters the client needs to generate
// a zkLogin proof using @mysten/zklogin.
//
// The client-side flow (NOT in Go):
//   1. Use @mysten/zklogin to compute:
//      address_seed = Poseidon_BN254(kc_name_F, kc_value_F, aud_F, Poseidon_BN254(salt))
//      address = Blake2b_256(0x05 || iss_len || iss_bytes || address_seed)
//   2. Generate Groth16 ZK proof proving knowledge of salt, sub, aud, iss
//   3. Return { user_address, address_seed, proof } to the gateway
type ZkLoginSession struct {
	// OAuth result
	JWT string `json:"jwt"`

	// zkLogin derivation inputs (pass to @mysten/zklogin)
	Salt          string `json:"salt"`           // random BN254 field element (hex)
	JwtRandomness string `json:"jwt_randomness"` // 16 random bytes for nonce (hex)
	KeyClaimName  string `json:"key_claim_name"` // always "sub"
	KeyClaimValue string `json:"key_claim_value"` // JWT sub claim value
	Audience      string `json:"audience"`       // OAuth client ID
	Issuer        string `json:"issuer"`         // e.g. "https://accounts.google.com"

	// Ephemeral key pair (generated server-side)
	EphemeralPrivKey string `json:"ephemeral_priv_key"` // secp256k1 private key (hex)
	EphemeralPubKey  string `json:"ephemeral_pub_key"`  // secp256k1 uncompressed public key (hex)

	// Session metadata
	MaxEpoch  int64  `json:"max_epoch"`  // max epoch for key validity
	Email     string `json:"email"`      // from JWT
}

// ProofSubmission is what the client returns after generating a proof with @mysten/zklogin.
type ProofSubmission struct {
	UserAddress  string `json:"user_address" binding:"required"`
	AddressSeed  string `json:"address_seed" binding:"required"`
	Proof        string `json:"proof"`          // base64-encoded Groth16 proof
	EphemeralPubKey string `json:"ephemeral_pub_key"` // must match server's key
}

// ────────────────────────────────────────────────────────────
// Server-side key generation (real crypto — secp256k1)
// ────────────────────────────────────────────────────────────

// generateSecp256k1Key generates a new secp256k1 private key.
func generateSecp256k1Key() ([]byte, error) {
	key, err := ecdsa.GenerateKey(secp256k1.S256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	return key.D.Bytes(), nil
}

// deriveSecp256k1PublicKey derives the public key from a secp256k1 private key.
// Returns the uncompressed public key (04 || x || y).
func deriveSecp256k1PublicKey(prv []byte) (string, error) {
	d := new(big.Int).SetBytes(prv)
	x, y := secp256k1.S256().ScalarBaseMult(d.Bytes())
	if x == nil || y == nil {
		return "", fmt.Errorf("failed to derive secp256k1 public key")
	}

	// Uncompressed format: 04 || x || y
	pubBytes := append([]byte{0x04}, append(x.Bytes(), y.Bytes()...)...)
	return "0x" + fmt.Sprintf("%x", pubBytes), nil
}

// GenerateSalt creates a random 22-byte salt for zkLogin address derivation.
// The salt is a random BN254 field element that unlinks the OAuth identity
// from the on-chain address.
func GenerateSalt() (string, error) {
	b := make([]byte, 22)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}
	return "0x" + fmt.Sprintf("%x", b), nil
}

// GenerateJwtRandomness creates 16 random bytes for the JWT nonce.
func GenerateJwtRandomness() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate jwt randomness: %w", err)
	}
	return "0x" + fmt.Sprintf("%x", b), nil
}

// ────────────────────────────────────────────────────────────
// JWT utilities
// ────────────────────────────────────────────────────────────

// ParseJWT parses a JWT and extracts the claims.
func ParseJWT(jwt string) (map[string]interface{}, error) {
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format: expected 3 parts, got %d", len(parts))
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, fmt.Errorf("failed to parse JWT claims: %w", err)
	}

	return claims, nil
}

// ValidateJWTClaims validates the essential JWT claims.
func ValidateJWTClaims(claims map[string]interface{}, expectedIssuer, expectedAudience, expectedNonce string) error {
	iss, ok := claims["iss"].(string)
	if !ok || iss != expectedIssuer {
		return fmt.Errorf("invalid issuer: expected %s, got %v", expectedIssuer, claims["iss"])
	}

	aud, ok := claims["aud"]
	if !ok {
		return fmt.Errorf("missing audience claim")
	}
	switch v := aud.(type) {
	case string:
		if v != expectedAudience {
			return fmt.Errorf("invalid audience: expected %s, got %s", expectedAudience, v)
		}
	case []interface{}:
		found := false
		for _, a := range v {
			if a == expectedAudience {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("audience %s not found in token", expectedAudience)
		}
	}

	if expectedNonce != "" {
		nonce, ok := claims["nonce"].(string)
		if !ok || nonce != expectedNonce {
			return fmt.Errorf("invalid nonce")
		}
	}

	return nil
}

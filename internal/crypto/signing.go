package crypto

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/khaar-ai/BotNet/pkg/types"
)

// CreateCanonicalPayload creates the canonical message payload for signing
// Format: "authorID|content|timestamp"
func CreateCanonicalPayload(message *types.Message) string {
	timestamp := message.Timestamp.Unix()
	return fmt.Sprintf("%s|%s|%d", message.AuthorID, message.Content.Text, timestamp)
}

// SignMessage signs a message with the agent's private key
func SignMessage(message *types.Message, privateKey ed25519.PrivateKey) error {
	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now()
	}

	// Create canonical payload
	payload := CreateCanonicalPayload(message)

	// Sign the payload
	signature := ed25519.Sign(privateKey, []byte(payload))

	// Encode signature as base64
	message.Signature = base64.StdEncoding.EncodeToString(signature)

	return nil
}

// VerifyMessageSignature verifies a message signature using the author's public key
func VerifyMessageSignature(message *types.Message, publicKey ed25519.PublicKey) error {
	if message.Signature == "" {
		return fmt.Errorf("message has no signature")
	}

	// Decode signature from base64
	signature, err := base64.StdEncoding.DecodeString(message.Signature)
	if err != nil {
		return fmt.Errorf("failed to decode message signature: %v", err)
	}

	// Create canonical payload
	payload := CreateCanonicalPayload(message)

	// Verify signature
	if !ed25519.Verify(publicKey, []byte(payload), signature) {
		return fmt.Errorf("invalid message signature")
	}

	return nil
}

// ValidateMessageSignature is a wrapper that includes additional security checks
func ValidateMessageSignature(message *types.Message, publicKeyBase64 string) error {
	// Check for empty or nil message
	if message == nil {
		return fmt.Errorf("message is nil")
	}

	// Check required fields
	if message.AuthorID == "" {
		return fmt.Errorf("message missing author ID")
	}

	if message.Signature == "" {
		return fmt.Errorf("message missing signature")
	}

	// Check timestamp is reasonable (not too old or in future)
	now := time.Now()
	maxAge := 24 * time.Hour
	maxFuture := 5 * time.Minute

	if now.Sub(message.Timestamp) > maxAge {
		return fmt.Errorf("message timestamp too old (max age: %v)", maxAge)
	}

	if message.Timestamp.Sub(now) > maxFuture {
		return fmt.Errorf("message timestamp too far in future (max: %v)", maxFuture)
	}

	// Decode public key
	publicKey, err := PublicKeyFromBase64(publicKeyBase64)
	if err != nil {
		return fmt.Errorf("failed to decode public key: %v", err)
	}

	// Verify signature
	return VerifyMessageSignature(message, publicKey)
}
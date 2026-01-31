package crypto

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/khaar-ai/BotNet/pkg/types"
)

// CreateManifestSignaturePayload creates the canonical payload for node manifest signing
// This ensures consistent signing across all nodes
func CreateManifestSignaturePayload(manifest *types.NodeManifest) ([]byte, error) {
	// Create a copy of the manifest without the signature field for signing
	signingManifest := &types.NodeManifest{
		NodeID:       manifest.NodeID,
		Version:      manifest.Version,
		PublicKey:    manifest.PublicKey,
		Endpoints:    manifest.Endpoints,
		Capabilities: manifest.Capabilities,
		RateLimit:    manifest.RateLimit,
		UpdatedAt:    manifest.UpdatedAt,
		// Deliberately exclude Signature field
	}

	// Marshal to JSON in a canonical format
	payload, err := json.Marshal(signingManifest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal manifest for signing: %v", err)
	}

	return payload, nil
}

// SignNodeManifest signs a node manifest with the node's private key
func SignNodeManifest(manifest *types.NodeManifest, privateKey ed25519.PrivateKey) error {
	// Set updated timestamp
	manifest.UpdatedAt = time.Now()

	// Create canonical payload for signing
	payload, err := CreateManifestSignaturePayload(manifest)
	if err != nil {
		return fmt.Errorf("failed to create signing payload: %v", err)
	}

	// Sign the payload
	signature := ed25519.Sign(privateKey, payload)

	// Encode signature as base64 and store in manifest
	manifest.Signature = base64.StdEncoding.EncodeToString(signature)

	return nil
}

// VerifyNodeManifestSignature verifies a node manifest signature
func VerifyNodeManifestSignature(manifest *types.NodeManifest) error {
	if manifest.Signature == "" {
		return fmt.Errorf("manifest has no signature")
	}

	// Extract public key from manifest
	if manifest.PublicKey == "" {
		return fmt.Errorf("manifest has no public key")
	}

	// Parse the public key - expect format "ed25519:base64"
	var publicKeyBase64 string
	if len(manifest.PublicKey) > 8 && manifest.PublicKey[:8] == "ed25519:" {
		publicKeyBase64 = manifest.PublicKey[8:]
	} else {
		// Assume it's just base64 without prefix (for compatibility)
		publicKeyBase64 = manifest.PublicKey
	}

	// Decode public key
	publicKey, err := PublicKeyFromBase64(publicKeyBase64)
	if err != nil {
		return fmt.Errorf("failed to decode public key from manifest: %v", err)
	}

	// Decode signature
	signature, err := base64.StdEncoding.DecodeString(manifest.Signature)
	if err != nil {
		return fmt.Errorf("failed to decode manifest signature: %v", err)
	}

	// Create canonical payload (same as signing)
	payload, err := CreateManifestSignaturePayload(manifest)
	if err != nil {
		return fmt.Errorf("failed to create verification payload: %v", err)
	}

	// Verify signature
	if !ed25519.Verify(publicKey, payload, signature) {
		return fmt.Errorf("invalid manifest signature")
	}

	return nil
}

// ValidateNodeManifestWithTimestamp adds additional security checks for manifest validation
func ValidateNodeManifestWithTimestamp(manifest *types.NodeManifest) error {
	// Check for required fields
	if manifest == nil {
		return fmt.Errorf("manifest is nil")
	}

	if manifest.NodeID == "" {
		return fmt.Errorf("manifest missing node ID")
	}

	if manifest.PublicKey == "" {
		return fmt.Errorf("manifest missing public key")
	}

	if manifest.Signature == "" {
		return fmt.Errorf("manifest missing signature")
	}

	// Check timestamp is reasonable (not too old or in future)
	now := time.Now()
	maxAge := 7 * 24 * time.Hour  // Manifests valid for up to 1 week
	maxFuture := 10 * time.Minute // Allow small clock skew

	if now.Sub(manifest.UpdatedAt) > maxAge {
		return fmt.Errorf("manifest timestamp too old (max age: %v)", maxAge)
	}

	if manifest.UpdatedAt.Sub(now) > maxFuture {
		return fmt.Errorf("manifest timestamp too far in future (max: %v)", maxFuture)
	}

	// Verify cryptographic signature
	if err := VerifyNodeManifestSignature(manifest); err != nil {
		return fmt.Errorf("signature verification failed: %v", err)
	}

	return nil
}

// FormatNodePublicKey formats a public key for storage in manifest
func FormatNodePublicKey(publicKeyBase64 string) string {
	return fmt.Sprintf("ed25519:%s", publicKeyBase64)
}
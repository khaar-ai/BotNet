package tests

import (
	"os"
	"testing"
	"time"

	"github.com/khaar-ai/BotNet/internal/crypto"
	"github.com/khaar-ai/BotNet/pkg/types"
)

func TestKeyGeneration(t *testing.T) {
	// Test Ed25519 keypair generation
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	if len(keyPair.PublicKey) != 32 {
		t.Errorf("Invalid public key length: expected 32, got %d", len(keyPair.PublicKey))
	}

	if len(keyPair.PrivateKey) != 64 {
		t.Errorf("Invalid private key length: expected 64, got %d", len(keyPair.PrivateKey))
	}

	// Test base64 encoding/decoding
	publicKeyBase64 := keyPair.PublicKeyToBase64()
	privateKeyBase64 := keyPair.PrivateKeyToBase64()

	if publicKeyBase64 == "" || privateKeyBase64 == "" {
		t.Error("Base64 encoding failed")
	}

	// Test decoding
	decodedPublic, err := crypto.PublicKeyFromBase64(publicKeyBase64)
	if err != nil {
		t.Errorf("Failed to decode public key: %v", err)
	}

	decodedPrivate, err := crypto.PrivateKeyFromBase64(privateKeyBase64)
	if err != nil {
		t.Errorf("Failed to decode private key: %v", err)
	}

	// Verify decoded keys match original
	if string(decodedPublic) != string(keyPair.PublicKey) {
		t.Error("Decoded public key doesn't match original")
	}

	if string(decodedPrivate) != string(keyPair.PrivateKey) {
		t.Error("Decoded private key doesn't match original")
	}

	t.Log("✅ Key generation and encoding tests passed")
}

func TestMessageSigning(t *testing.T) {
	// Generate test keypair
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Create test message
	message := &types.Message{
		ID:       "test-message-1",
		Type:     "post",
		AuthorID: "test-agent-1",
		Content: types.MessageContent{
			Text: "This is a test message for signature verification",
		},
		Timestamp: time.Now(),
	}

	// Sign the message
	err = crypto.SignMessage(message, keyPair.PrivateKey)
	if err != nil {
		t.Fatalf("Failed to sign message: %v", err)
	}

	if message.Signature == "" {
		t.Error("Message signature is empty after signing")
	}

	t.Logf("Message signed successfully: %s", message.Signature[:20]+"...")

	// Verify the signature
	err = crypto.VerifyMessageSignature(message, keyPair.PublicKey)
	if err != nil {
		t.Errorf("Failed to verify message signature: %v", err)
	}

	t.Log("✅ Message signing and verification tests passed")
}

func TestSignatureVerificationFailures(t *testing.T) {
	// Generate two different keypairs
	keyPair1, _ := crypto.GenerateKeyPair()
	keyPair2, _ := crypto.GenerateKeyPair()

	// Create and sign message with first keypair
	message := &types.Message{
		ID:       "test-message-2",
		Type:     "post",
		AuthorID: "test-agent-2",
		Content: types.MessageContent{
			Text: "This message will be verified with wrong key",
		},
		Timestamp: time.Now(),
	}

	crypto.SignMessage(message, keyPair1.PrivateKey)

	// Try to verify with second keypair (should fail)
	err := crypto.VerifyMessageSignature(message, keyPair2.PublicKey)
	if err == nil {
		t.Error("Signature verification should have failed with wrong public key")
	}

	// Test with tampered message content
	originalContent := message.Content.Text
	message.Content.Text = "This content has been tampered with"

	err = crypto.VerifyMessageSignature(message, keyPair1.PublicKey)
	if err == nil {
		t.Error("Signature verification should have failed with tampered content")
	}

	// Restore original content
	message.Content.Text = originalContent

	// Test with tampered signature
	message.Signature = "dGFtcGVyZWRfc2lnbmF0dXJl" // Base64 encoded "tampered_signature"

	err = crypto.VerifyMessageSignature(message, keyPair1.PublicKey)
	if err == nil {
		t.Error("Signature verification should have failed with tampered signature")
	}

	t.Log("✅ Signature verification failure tests passed")
}

func TestValidateMessageSignature(t *testing.T) {
	keyPair, _ := crypto.GenerateKeyPair()
	publicKeyBase64 := keyPair.PublicKeyToBase64()

	// Test with valid message
	validMessage := &types.Message{
		ID:       "valid-message",
		Type:     "post",
		AuthorID: "test-agent",
		Content: types.MessageContent{
			Text: "Valid test message",
		},
		Timestamp: time.Now(),
	}

	crypto.SignMessage(validMessage, keyPair.PrivateKey)

	err := crypto.ValidateMessageSignature(validMessage, publicKeyBase64)
	if err != nil {
		t.Errorf("Valid message failed validation: %v", err)
	}

	// Test with nil message
	err = crypto.ValidateMessageSignature(nil, publicKeyBase64)
	if err == nil {
		t.Error("Should fail validation for nil message")
	}

	// Test with missing author ID
	invalidMessage := &types.Message{
		AuthorID: "",
		Content:  types.MessageContent{Text: "Test"},
		Timestamp: time.Now(),
	}
	err = crypto.ValidateMessageSignature(invalidMessage, publicKeyBase64)
	if err == nil {
		t.Error("Should fail validation for missing author ID")
	}

	// Test with old timestamp
	oldMessage := &types.Message{
		AuthorID: "test-agent",
		Content:  types.MessageContent{Text: "Old message"},
		Timestamp: time.Now().Add(-25 * time.Hour), // 25 hours ago
	}
	crypto.SignMessage(oldMessage, keyPair.PrivateKey)
	err = crypto.ValidateMessageSignature(oldMessage, publicKeyBase64)
	if err == nil {
		t.Error("Should fail validation for old timestamp")
	}

	// Test with future timestamp
	futureMessage := &types.Message{
		AuthorID: "test-agent",
		Content:  types.MessageContent{Text: "Future message"},
		Timestamp: time.Now().Add(10 * time.Minute), // 10 minutes in future
	}
	crypto.SignMessage(futureMessage, keyPair.PrivateKey)
	err = crypto.ValidateMessageSignature(futureMessage, publicKeyBase64)
	if err == nil {
		t.Error("Should fail validation for future timestamp")
	}

	t.Log("✅ Message validation tests passed")
}

func TestAgentKeyStore(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "botnet-test-keys")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Initialize key store
	keyStore, err := crypto.NewAgentKeyStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create key store: %v", err)
	}

	agentID := "test-agent-keystore"

	// Test key generation and storage
	keyPair, err := keyStore.GenerateAndStoreKeyPair(agentID)
	if err != nil {
		t.Fatalf("Failed to generate and store keypair: %v", err)
	}

	// Test key retrieval
	privateKey, err := keyStore.GetPrivateKey(agentID)
	if err != nil {
		t.Errorf("Failed to get private key: %v", err)
	}

	if string(privateKey) != string(keyPair.PrivateKey) {
		t.Error("Retrieved private key doesn't match generated key")
	}

	publicKey, err := keyStore.GetPublicKey(agentID)
	if err != nil {
		t.Errorf("Failed to get public key: %v", err)
	}

	if publicKey != keyPair.PublicKeyToBase64() {
		t.Error("Retrieved public key doesn't match generated key")
	}

	// Test HasKey
	if !keyStore.HasKey(agentID) {
		t.Error("HasKey should return true for existing agent")
	}

	if keyStore.HasKey("non-existent-agent") {
		t.Error("HasKey should return false for non-existent agent")
	}

	// Test ListAgents
	agents, err := keyStore.ListAgents()
	if err != nil {
		t.Errorf("Failed to list agents: %v", err)
	}

	if len(agents) != 1 || agents[0] != agentID {
		t.Error("ListAgents returned incorrect results")
	}

	// Test duplicate key generation (should fail)
	_, err = keyStore.GenerateAndStoreKeyPair(agentID)
	if err == nil {
		t.Error("Should fail to generate duplicate keypair")
	}

	// Test key deletion
	err = keyStore.DeleteKey(agentID)
	if err != nil {
		t.Errorf("Failed to delete key: %v", err)
	}

	if keyStore.HasKey(agentID) {
		t.Error("Key should be deleted")
	}

	t.Log("✅ Agent key store tests passed")
}

func TestPublicKeyCache(t *testing.T) {
	cache := crypto.NewPublicKeyCache(1 * time.Hour)

	agentID := "test-agent-cache"
	publicKey := "test-public-key-base64"
	nodeID := "test-node"

	// Test cache miss
	_, found := cache.Get(agentID)
	if found {
		t.Error("Should not find key in empty cache")
	}

	// Test cache set and get
	cache.Set(agentID, publicKey, nodeID)
	retrievedKey, found := cache.Get(agentID)
	if !found {
		t.Error("Should find key in cache after setting")
	}

	if retrievedKey != publicKey {
		t.Error("Retrieved key doesn't match set key")
	}

	// Test cache size
	if cache.Size() != 1 {
		t.Error("Cache size should be 1")
	}

	// Test cache delete
	cache.Delete(agentID)
	_, found = cache.Get(agentID)
	if found {
		t.Error("Should not find key after deletion")
	}

	// Test cache clear
	cache.Set(agentID, publicKey, nodeID)
	cache.Set("another-agent", "another-key", nodeID)
	cache.Clear()

	if cache.Size() != 0 {
		t.Error("Cache should be empty after clear")
	}

	t.Log("✅ Public key cache tests passed")
}

func TestCanonicalPayloadCreation(t *testing.T) {
	message := &types.Message{
		AuthorID: "test-agent",
		Content: types.MessageContent{
			Text: "Test message content",
		},
		Timestamp: time.Unix(1640995200, 0), // Fixed timestamp for consistent testing
	}

	expectedPayload := "test-agent|Test message content|1640995200"
	actualPayload := crypto.CreateCanonicalPayload(message)

	if actualPayload != expectedPayload {
		t.Errorf("Canonical payload mismatch. Expected: %s, Got: %s", expectedPayload, actualPayload)
	}

	t.Log("✅ Canonical payload creation test passed")
}

func TestEndToEndMessageAuthentication(t *testing.T) {
	// This test simulates the complete flow:
	// 1. Agent registration with key generation
	// 2. Message creation and signing
	// 3. Message federation and signature verification

	// Setup temporary key store
	tempDir, _ := os.MkdirTemp("", "botnet-e2e-test")
	defer os.RemoveAll(tempDir)

	keyStore, _ := crypto.NewAgentKeyStore(tempDir)

	// Simulate agent registration
	agentID := "e2e-test-agent"
	_, err := keyStore.GenerateAndStoreKeyPair(agentID)
	if err != nil {
		t.Fatalf("Failed to register agent: %v", err)
	}

	// Simulate message creation and signing (sender side)
	message := &types.Message{
		ID:       "e2e-test-message",
		Type:     "post",
		AuthorID: agentID,
		Content: types.MessageContent{
			Text: "End-to-end test message",
		},
		Timestamp: time.Now(),
	}

	// Sign message (this happens in PostMessage)
	privateKey, _ := keyStore.GetPrivateKey(agentID)
	err = crypto.SignMessage(message, privateKey)
	if err != nil {
		t.Fatalf("Failed to sign message: %v", err)
	}

	// Simulate message reception and verification (receiver side)
	publicKeyBase64, _ := keyStore.GetPublicKey(agentID)
	err = crypto.ValidateMessageSignature(message, publicKeyBase64)
	if err != nil {
		t.Fatalf("Failed to verify received message: %v", err)
	}

	t.Log("✅ End-to-end message authentication test passed")
}

// Run all tests
func TestCryptographicSystem(t *testing.T) {
	t.Run("KeyGeneration", TestKeyGeneration)
	t.Run("MessageSigning", TestMessageSigning)
	t.Run("SignatureVerificationFailures", TestSignatureVerificationFailures)
	t.Run("ValidateMessageSignature", TestValidateMessageSignature)
	t.Run("AgentKeyStore", TestAgentKeyStore)
	t.Run("PublicKeyCache", TestPublicKeyCache)
	t.Run("CanonicalPayloadCreation", TestCanonicalPayloadCreation)
	t.Run("EndToEndMessageAuthentication", TestEndToEndMessageAuthentication)

	t.Log("\n🎉 ALL CRYPTOGRAPHIC TESTS PASSED!")
	t.Log("✅ Ed25519 keypair generation working")
	t.Log("✅ Message signing/verification working")
	t.Log("✅ Signature validation with security checks working")
	t.Log("✅ Key storage and management working")
	t.Log("✅ Public key caching working")
	t.Log("✅ End-to-end message authentication working")
}
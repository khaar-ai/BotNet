package tests

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/khaar-ai/BotNet/internal/crypto"
	"github.com/khaar-ai/BotNet/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeKeyGeneration(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := ioutil.TempDir("", "botnet_node_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	nodeID := "test-node-001"

	// Test 1: Generate new keys
	nodeKeyStore, err := crypto.NewNodeKeyStore(tmpDir, nodeID)
	require.NoError(t, err)

	keyPair1, err := nodeKeyStore.InitializeOrLoadKeys()
	require.NoError(t, err)
	assert.NotNil(t, keyPair1.PublicKey)
	assert.NotNil(t, keyPair1.PrivateKey)

	publicKey1 := keyPair1.PublicKeyToBase64()

	// Test 2: Load existing keys (should be same as generated)
	keyPair2, err := nodeKeyStore.InitializeOrLoadKeys()
	require.NoError(t, err)

	publicKey2 := keyPair2.PublicKeyToBase64()
	assert.Equal(t, publicKey1, publicKey2, "Public keys should be identical when loading existing keypair")

	// Test 3: Key file should exist with correct permissions
	keyFilePath := filepath.Join(tmpDir, "node_keys", "node.key")
	info, err := os.Stat(keyFilePath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm(), "Key file should have 0600 permissions")

	// Test 4: Key file should have correct structure
	keyData, err := ioutil.ReadFile(keyFilePath)
	require.NoError(t, err)

	var storedKey struct {
		NodeID     string `json:"node_id"`
		PublicKey  string `json:"public_key"`
		PrivateKey string `json:"private_key"`
		CreatedAt  int64  `json:"created_at"`
	}
	err = json.Unmarshal(keyData, &storedKey)
	require.NoError(t, err)

	assert.Equal(t, nodeID, storedKey.NodeID)
	assert.Equal(t, publicKey1, storedKey.PublicKey)
	assert.NotEmpty(t, storedKey.PrivateKey)
	assert.Greater(t, storedKey.CreatedAt, int64(0))

	t.Logf("✅ Node key generation test passed. Public key: ed25519:%s", publicKey1[:16]+"...")
}

func TestNodeManifestSigning(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := ioutil.TempDir("", "botnet_manifest_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	nodeID := "test-node-manifest"

	// Initialize node keys
	nodeKeyStore, err := crypto.NewNodeKeyStore(tmpDir, nodeID)
	require.NoError(t, err)

	keyPair, err := nodeKeyStore.InitializeOrLoadKeys()
	require.NoError(t, err)

	// Create a test manifest
	manifest := &types.NodeManifest{
		NodeID:    nodeID,
		Version:   "1.0.0",
		PublicKey: crypto.FormatNodePublicKey(keyPair.PublicKeyToBase64()),
		Endpoints: types.NodeEndpoints{
			Federation: "https://test.example.com/federation",
			API:        "https://test.example.com/api/v1",
			WebUI:      "https://test.example.com/",
		},
		Capabilities: []string{"messaging", "agents", "federation"},
		RateLimit: types.RateLimitInfo{
			MessagesPerHour:   1000,
			FederationPerHour: 100,
		},
		UpdatedAt: time.Now(),
	}

	// Test 1: Sign the manifest
	err = crypto.SignNodeManifest(manifest, keyPair.PrivateKey)
	require.NoError(t, err)
	assert.NotEmpty(t, manifest.Signature, "Manifest should have a signature after signing")

	// Test 2: Verify the signature
	err = crypto.VerifyNodeManifestSignature(manifest)
	assert.NoError(t, err, "Signature verification should succeed for valid signature")

	// Test 3: Tamper with manifest and verify it fails
	originalNodeID := manifest.NodeID
	manifest.NodeID = "tampered-node-id"
	err = crypto.VerifyNodeManifestSignature(manifest)
	assert.Error(t, err, "Signature verification should fail for tampered manifest")

	// Restore original data
	manifest.NodeID = originalNodeID

	// Test 4: Full validation with timestamp checks
	err = crypto.ValidateNodeManifestWithTimestamp(manifest)
	assert.NoError(t, err, "Full manifest validation should succeed")

	// Test 5: Test with old timestamp (should fail)
	oldManifest := *manifest
	oldManifest.UpdatedAt = time.Now().Add(-8 * 24 * time.Hour) // 8 days old
	err = crypto.ValidateNodeManifestWithTimestamp(&oldManifest)
	assert.Error(t, err, "Validation should fail for manifests that are too old")

	// Test 6: Test with future timestamp (should fail)
	futureManifest := *manifest
	futureManifest.UpdatedAt = time.Now().Add(15 * time.Minute) // 15 minutes in future
	err = crypto.ValidateNodeManifestWithTimestamp(&futureManifest)
	assert.Error(t, err, "Validation should fail for manifests with timestamps too far in future")

	t.Logf("✅ Node manifest signing test passed. Signature: %s", manifest.Signature[:16]+"...")
}

func TestAgentMessageSigning(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := ioutil.TempDir("", "botnet_agent_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Initialize agent key store
	agentKeyStore, err := crypto.NewAgentKeyStore(filepath.Join(tmpDir, "keys"))
	require.NoError(t, err)

	agentID := "test-agent-001"

	// Test 1: Generate agent keys
	keyPair, err := agentKeyStore.GenerateAndStoreKeyPair(agentID)
	require.NoError(t, err)

	// Test 2: Create and sign a message
	message := &types.Message{
		Type:     "post",
		AuthorID: agentID,
		Content: types.MessageContent{
			Text: "Hello, BotNet! This is a test message from the cryptographic infrastructure.",
		},
		Timestamp: time.Now(),
	}

	// Sign the message
	err = crypto.SignMessage(message, keyPair.PrivateKey)
	require.NoError(t, err)
	assert.NotEmpty(t, message.Signature, "Message should have a signature after signing")

	// Test 3: Verify the message signature
	err = crypto.VerifyMessageSignature(message, keyPair.PublicKey)
	assert.NoError(t, err, "Message signature verification should succeed")

	// Test 4: Test with tampered content (should fail)
	originalContent := message.Content.Text
	message.Content.Text = "This content has been tampered with!"
	err = crypto.VerifyMessageSignature(message, keyPair.PublicKey)
	assert.Error(t, err, "Signature verification should fail for tampered message")

	// Restore original content
	message.Content.Text = originalContent

	// Test 5: Full message validation
	publicKeyBase64 := keyPair.PublicKeyToBase64()
	err = crypto.ValidateMessageSignature(message, publicKeyBase64)
	assert.NoError(t, err, "Full message validation should succeed")

	// Test 6: Test with old timestamp (should fail)
	oldMessage := *message
	oldMessage.Timestamp = time.Now().Add(-25 * time.Hour) // 25 hours old
	err = crypto.ValidateMessageSignature(&oldMessage, publicKeyBase64)
	assert.Error(t, err, "Validation should fail for messages that are too old")

	t.Logf("✅ Agent message signing test passed. Agent: %s, Signature: %s", agentID, message.Signature[:16]+"...")
}

func TestPublicKeyCaching(t *testing.T) {
	// Test 1: Create public key cache
	cache := crypto.NewPublicKeyCache(5 * time.Second) // Short TTL for testing
	assert.NotNil(t, cache)

	agentID := "test-agent-cache"
	nodeID := "test-node-cache"

	// Generate a test key
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)
	publicKeyBase64 := keyPair.PublicKeyToBase64()

	// Test 2: Cache a public key
	cache.Set(agentID, publicKeyBase64, nodeID)
	assert.Equal(t, 1, cache.Size())

	// Test 3: Retrieve cached key
	cachedKey, found := cache.Get(agentID)
	assert.True(t, found, "Should find cached key")
	assert.Equal(t, publicKeyBase64, cachedKey, "Cached key should match original")

	// Test 4: Test cache expiration
	time.Sleep(6 * time.Second) // Wait for TTL to expire
	_, found = cache.Get(agentID)
	assert.False(t, found, "Expired key should not be found")

	// Test 5: Cache miss
	_, found = cache.Get("nonexistent-agent")
	assert.False(t, found, "Nonexistent key should not be found")

	t.Logf("✅ Public key caching test passed")
}

func TestCryptographicIntegration(t *testing.T) {
	// This test simulates the full cryptographic workflow between two nodes
	
	// Create temporary directories for two test nodes
	tmpDir1, err := ioutil.TempDir("", "botnet_node1_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir1)

	tmpDir2, err := ioutil.TempDir("", "botnet_node2_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir2)

	// Initialize two nodes
	node1ID := "test-node-001"
	node2ID := "test-node-002"

	node1KeyStore, err := crypto.NewNodeKeyStore(tmpDir1, node1ID)
	require.NoError(t, err)

	node2KeyStore, err := crypto.NewNodeKeyStore(tmpDir2, node2ID)
	require.NoError(t, err)

	node1Keys, err := node1KeyStore.InitializeOrLoadKeys()
	require.NoError(t, err)

	node2Keys, err := node2KeyStore.InitializeOrLoadKeys()
	require.NoError(t, err)

	// Test 1: Create and sign manifests for both nodes
	manifest1 := &types.NodeManifest{
		NodeID:    node1ID,
		Version:   "1.0.0",
		PublicKey: crypto.FormatNodePublicKey(node1Keys.PublicKeyToBase64()),
		Endpoints: types.NodeEndpoints{
			Federation: "https://node1.example.com/federation",
			API:        "https://node1.example.com/api/v1",
			WebUI:      "https://node1.example.com/",
		},
		Capabilities: []string{"messaging", "agents"},
		RateLimit: types.RateLimitInfo{MessagesPerHour: 1000, FederationPerHour: 100},
		UpdatedAt: time.Now(),
	}

	err = crypto.SignNodeManifest(manifest1, node1Keys.PrivateKey)
	require.NoError(t, err)

	// Test 2: Node 2 verifies Node 1's manifest
	err = crypto.ValidateNodeManifestWithTimestamp(manifest1)
	assert.NoError(t, err, "Node 2 should be able to verify Node 1's manifest")

	// Test 3: Create agent on Node 1
	agent1KeyStore, err := crypto.NewAgentKeyStore(filepath.Join(tmpDir1, "agent_keys"))
	require.NoError(t, err)

	agent1ID := "agent-alice"
	agent1Keys, err := agent1KeyStore.GenerateAndStoreKeyPair(agent1ID)
	require.NoError(t, err)

	// Test 4: Agent sends message from Node 1
	message := &types.Message{
		Type:     "post",
		AuthorID: agent1ID,
		Content: types.MessageContent{
			Text: "Cross-node cryptographic test message",
		},
		Timestamp: time.Now(),
	}

	err = crypto.SignMessage(message, agent1Keys.PrivateKey)
	require.NoError(t, err)

	// Test 5: Node 2 receives and validates the message
	// (In real scenario, Node 2 would fetch agent's public key from Node 1)
	agent1PublicKeyBase64 := agent1Keys.PublicKeyToBase64()
	err = crypto.ValidateMessageSignature(message, agent1PublicKeyBase64)
	assert.NoError(t, err, "Node 2 should be able to validate message from Node 1's agent")

	t.Logf("✅ Cryptographic integration test passed")
	t.Logf("Node 1: %s, Public Key: ed25519:%s", node1ID, node1Keys.PublicKeyToBase64()[:16]+"...")
	t.Logf("Node 2: %s, Public Key: ed25519:%s", node2ID, node2Keys.PublicKeyToBase64()[:16]+"...")
	t.Logf("Agent: %s, Message Signature: %s", agent1ID, message.Signature[:16]+"...")
}
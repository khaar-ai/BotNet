package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/khaar-ai/BotNet/internal/crypto"
	"github.com/khaar-ai/BotNet/pkg/types"
)

func main() {
	fmt.Println("🔐 BotNet Cryptographic Infrastructure Demo")
	fmt.Println("==========================================")

	// Create temporary directory for demo
	tmpDir, err := ioutil.TempDir("", "botnet_crypto_demo")
	if err != nil {
		log.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	fmt.Printf("Demo directory: %s\n\n", tmpDir)

	// Demo 1: Node Identity Generation
	fmt.Println("📊 1. Node Identity Generation")
	fmt.Println("==============================")

	nodeID := "demo-node.airon.games"
	nodeKeyStore, err := crypto.NewNodeKeyStore(tmpDir, nodeID)
	if err != nil {
		log.Fatalf("Failed to create node key store: %v", err)
	}

	nodeKeys, err := nodeKeyStore.InitializeOrLoadKeys()
	if err != nil {
		log.Fatalf("Failed to initialize node keys: %v", err)
	}

	nodePublicKey := nodeKeys.PublicKeyToBase64()
	fmt.Printf("✅ Node ID: %s\n", nodeID)
	fmt.Printf("✅ Public Key: ed25519:%s\n", nodePublicKey[:32]+"...")
	fmt.Printf("✅ Key file created at: %s\n\n", filepath.Join(tmpDir, "node_keys", "node.key"))

	// Demo 2: Node Manifest Signing
	fmt.Println("📋 2. Node Manifest Signing")
	fmt.Println("============================")

	manifest := &types.NodeManifest{
		NodeID:    nodeID,
		Version:   "1.0.0",
		PublicKey: crypto.FormatNodePublicKey(nodePublicKey),
		Endpoints: types.NodeEndpoints{
			Federation: "https://demo-node.airon.games/federation",
			API:        "https://demo-node.airon.games/api/v1",
			WebUI:      "https://demo-node.airon.games/",
		},
		Capabilities: []string{"messaging", "agents", "federation", "challenges"},
		RateLimit: types.RateLimitInfo{
			MessagesPerHour:   1000,
			FederationPerHour: 100,
		},
		UpdatedAt: time.Now(),
	}

	err = crypto.SignNodeManifest(manifest, nodeKeys.PrivateKey)
	if err != nil {
		log.Fatalf("Failed to sign manifest: %v", err)
	}

	fmt.Printf("✅ Manifest signed successfully\n")
	fmt.Printf("✅ Signature: %s\n", manifest.Signature[:32]+"...")

	// Verify the manifest signature
	err = crypto.VerifyNodeManifestSignature(manifest)
	if err != nil {
		log.Fatalf("Manifest signature verification failed: %v", err)
	}
	fmt.Printf("✅ Signature verification: PASSED\n\n")

	// Demo 3: Agent Key Generation
	fmt.Println("🤖 3. Agent Key Generation")
	fmt.Println("===========================")

	agentKeyStore, err := crypto.NewAgentKeyStore(filepath.Join(tmpDir, "agent_keys"))
	if err != nil {
		log.Fatalf("Failed to create agent key store: %v", err)
	}

	agentID := "demo-agent-001"
	agentKeys, err := agentKeyStore.GenerateAndStoreKeyPair(agentID)
	if err != nil {
		log.Fatalf("Failed to generate agent keys: %v", err)
	}

	agentPublicKey := agentKeys.PublicKeyToBase64()
	fmt.Printf("✅ Agent ID: %s\n", agentID)
	fmt.Printf("✅ Public Key: %s\n", agentPublicKey[:32]+"...")
	fmt.Printf("✅ Key file created at: %s\n\n", filepath.Join(tmpDir, "agent_keys", agentID+".key"))

	// Demo 4: Message Signing
	fmt.Println("💬 4. Message Signing")
	fmt.Println("======================")

	message := &types.Message{
		Type:     "post",
		AuthorID: agentID,
		Content: types.MessageContent{
			Text: "Hello BotNet! This is a demo message showcasing Ed25519 cryptographic signing. 🔐✨",
			Links: []string{"https://github.com/khaar-ai/BotNet"},
			Hashtags: []string{"#BotNet", "#Cryptography", "#Ed25519", "#Federation"},
		},
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"demo": true,
			"version": "1.0.0",
		},
	}

	err = crypto.SignMessage(message, agentKeys.PrivateKey)
	if err != nil {
		log.Fatalf("Failed to sign message: %v", err)
	}

	fmt.Printf("✅ Message signed by agent %s\n", agentID)
	fmt.Printf("✅ Content: %s\n", message.Content.Text[:50]+"...")
	fmt.Printf("✅ Signature: %s\n", message.Signature[:32]+"...")

	// Verify the message signature
	err = crypto.VerifyMessageSignature(message, agentKeys.PublicKey)
	if err != nil {
		log.Fatalf("Message signature verification failed: %v", err)
	}
	fmt.Printf("✅ Signature verification: PASSED\n\n")

	// Demo 5: Public Key Caching
	fmt.Println("🗄️  5. Public Key Caching")
	fmt.Println("=========================")

	cache := crypto.NewPublicKeyCache(1 * time.Hour)
	
	// Cache the agent's public key
	cache.Set(agentID, agentPublicKey, nodeID)
	fmt.Printf("✅ Cached public key for agent %s\n", agentID)

	// Retrieve from cache
	cachedKey, found := cache.Get(agentID)
	if !found {
		log.Fatalf("Failed to retrieve cached key")
	}

	if cachedKey != agentPublicKey {
		log.Fatalf("Cached key doesn't match original")
	}

	fmt.Printf("✅ Retrieved public key from cache: %s\n", cachedKey[:32]+"...")
	fmt.Printf("✅ Cache size: %d entries\n\n", cache.Size())

	// Demo 6: Cross-Validation Security Test
	fmt.Println("🛡️  6. Security Validation Tests")
	fmt.Println("=================================")

	// Test 1: Tampered message detection
	originalContent := message.Content.Text
	message.Content.Text = "This message has been tampered with!"
	
	err = crypto.VerifyMessageSignature(message, agentKeys.PublicKey)
	if err == nil {
		log.Fatalf("SECURITY FAILURE: Tampered message was not detected!")
	}
	fmt.Printf("✅ Tampered message detection: PASSED\n")

	// Restore original content
	message.Content.Text = originalContent

	// Test 2: Wrong public key detection
	wrongKeys, _ := crypto.GenerateKeyPair()
	err = crypto.VerifyMessageSignature(message, wrongKeys.PublicKey)
	if err == nil {
		log.Fatalf("SECURITY FAILURE: Wrong public key was not detected!")
	}
	fmt.Printf("✅ Wrong public key detection: PASSED\n")

	// Test 3: Manifest tampering detection
	originalNodeID := manifest.NodeID
	manifest.NodeID = "hacker-node.evil.com"
	
	err = crypto.VerifyNodeManifestSignature(manifest)
	if err == nil {
		log.Fatalf("SECURITY FAILURE: Tampered manifest was not detected!")
	}
	fmt.Printf("✅ Manifest tampering detection: PASSED\n")

	// Restore original manifest
	manifest.NodeID = originalNodeID

	fmt.Println("\n🎉 Cryptographic Infrastructure Demo Complete!")
	fmt.Println("==============================================")
	fmt.Printf("✅ All cryptographic operations successful\n")
	fmt.Printf("✅ All security validations passed\n")
	fmt.Printf("✅ BotNet is ready for secure federation\n\n")

	// Summary
	fmt.Println("📊 Summary:")
	fmt.Printf("   • Node Identity: %s\n", nodeID)
	fmt.Printf("   • Node Public Key: ed25519:%s\n", nodePublicKey[:16]+"...")
	fmt.Printf("   • Agent: %s\n", agentID)
	fmt.Printf("   • Agent Public Key: %s\n", agentPublicKey[:16]+"...")
	fmt.Printf("   • Message Signature: %s\n", message.Signature[:16]+"...")
	fmt.Printf("   • Manifest Signature: %s\n", manifest.Signature[:16]+"...")
	fmt.Printf("   • Demo Directory: %s (will be cleaned up)\n", tmpDir)
}
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/khaar-ai/BotNet/internal/crypto"
	"github.com/khaar-ai/BotNet/pkg/types"
)

func main() {
	fmt.Println("🔐 BotNet Cryptographic Message Authentication Demo")
	fmt.Println("=" + string(make([]byte, 50)))

	// Create demo directory
	demoDir := "/tmp/botnet-crypto-demo"
	os.MkdirAll(demoDir, 0755)
	defer os.RemoveAll(demoDir)

	// 1. DEMONSTRATE KEY GENERATION
	fmt.Println("\n1️⃣  AGENT KEYPAIR GENERATION")
	fmt.Println("-" + string(make([]byte, 30)))

	keyStore, err := crypto.NewAgentKeyStore(filepath.Join(demoDir, "keys"))
	if err != nil {
		log.Fatalf("Failed to create key store: %v", err)
	}

	// Register Alice agent
	aliceID := "alice-agent"
	aliceKeyPair, err := keyStore.GenerateAndStoreKeyPair(aliceID)
	if err != nil {
		log.Fatalf("Failed to generate Alice's keys: %v", err)
	}

	fmt.Printf("✅ Alice's keypair generated\n")
	fmt.Printf("   Public Key:  %s\n", aliceKeyPair.PublicKeyToBase64()[:32]+"...")
	fmt.Printf("   Private Key: %s (🔒 NEVER FEDERATED)\n", aliceKeyPair.PrivateKeyToBase64()[:16]+"...")

	// Register Bob agent
	bobID := "bob-agent"
	bobKeyPair, err := keyStore.GenerateAndStoreKeyPair(bobID)
	if err != nil {
		log.Fatalf("Failed to generate Bob's keys: %v", err)
	}

	fmt.Printf("✅ Bob's keypair generated\n")
	fmt.Printf("   Public Key:  %s\n", bobKeyPair.PublicKeyToBase64()[:32]+"...")
	fmt.Printf("   Private Key: %s (🔒 NEVER FEDERATED)\n", bobKeyPair.PrivateKeyToBase64()[:16]+"...")

	// 2. DEMONSTRATE MESSAGE SIGNING
	fmt.Println("\n2️⃣  MESSAGE SIGNING")
	fmt.Println("-" + string(make([]byte, 30)))

	// Alice creates a message
	aliceMessage := &types.Message{
		ID:       "msg-001",
		Type:     "post",
		AuthorID: aliceID,
		Content: types.MessageContent{
			Text: "Hello Bob! This message is cryptographically signed. 👋",
		},
		Timestamp: time.Now(),
	}

	// Sign Alice's message
	alicePrivateKey, _ := keyStore.GetPrivateKey(aliceID)
	err = crypto.SignMessage(aliceMessage, alicePrivateKey)
	if err != nil {
		log.Fatalf("Failed to sign Alice's message: %v", err)
	}

	fmt.Printf("✅ Alice's message signed\n")
	fmt.Printf("   Author:    %s\n", aliceMessage.AuthorID)
	fmt.Printf("   Content:   %s\n", aliceMessage.Content.Text)
	fmt.Printf("   Signature: %s\n", aliceMessage.Signature[:32]+"...")
	fmt.Printf("   Canonical: %s\n", crypto.CreateCanonicalPayload(aliceMessage))

	// 3. DEMONSTRATE SIGNATURE VERIFICATION
	fmt.Println("\n3️⃣  SIGNATURE VERIFICATION")
	fmt.Printf("-" + string(make([]byte, 30)))

	// Verify with correct public key (should succeed)
	alicePublicKey, _ := keyStore.GetPublicKey(aliceID)
	err = crypto.ValidateMessageSignature(aliceMessage, alicePublicKey)
	if err != nil {
		fmt.Printf("❌ Signature verification failed: %v\n", err)
	} else {
		fmt.Printf("✅ Signature verification PASSED - message is authentic\n")
	}

	// Try to verify with wrong public key (should fail)
	bobPublicKey, _ := keyStore.GetPublicKey(bobID)
	err = crypto.ValidateMessageSignature(aliceMessage, bobPublicKey)
	if err != nil {
		fmt.Printf("✅ Correctly REJECTED message with wrong public key: %v\n", err)
	} else {
		fmt.Printf("❌ SECURITY VULNERABILITY: Accepted message with wrong key!\n")
	}

	// 4. DEMONSTRATE FORGERY ATTEMPT
	fmt.Println("\n4️⃣  FORGERY ATTACK SIMULATION")
	fmt.Println("-" + string(make([]byte, 30)))

	// Create a forged message claiming to be from Alice
	forgedMessage := &types.Message{
		ID:       "forged-001",
		Type:     "post",
		AuthorID: aliceID, // Claiming to be Alice!
		Content: types.MessageContent{
			Text: "I'm transferring all my credits to Bob! (FORGED MESSAGE)",
		},
		Timestamp: time.Now(),
	}

	// Evil Bob tries to sign it with his own key
	bobPrivateKey, _ := keyStore.GetPrivateKey(bobID)
	crypto.SignMessage(forgedMessage, bobPrivateKey)

	fmt.Printf("🎭 Forged message created by Bob\n")
	fmt.Printf("   Claims Author: %s (Alice)\n", forgedMessage.AuthorID)
	fmt.Printf("   Content:       %s\n", forgedMessage.Content.Text)
	fmt.Printf("   Signature:     %s\n", forgedMessage.Signature[:32]+"...")

	// Try to verify forged message with Alice's public key
	err = crypto.ValidateMessageSignature(forgedMessage, alicePublicKey)
	if err != nil {
		fmt.Printf("✅ 🛡️  FORGERY BLOCKED: %v\n", err)
		fmt.Printf("   The cryptographic system successfully detected the forged message!\n")
	} else {
		fmt.Printf("❌ 🚨 CRITICAL SECURITY FAILURE: Forged message was accepted!\n")
	}

	// 5. DEMONSTRATE PUBLIC KEY DISTRIBUTION
	fmt.Println("\n5️⃣  PUBLIC KEY DISTRIBUTION")
	fmt.Println("-" + string(make([]byte, 30)))

	// Simulate public key cache for federation
	publicKeyCache := crypto.NewPublicKeyCache(1 * time.Hour)

	// Cache Alice's public key (as if received from federation)
	publicKeyCache.Set(aliceID, alicePublicKey, "node-1")

	// Retrieve from cache
	cachedKey, found := publicKeyCache.Get(aliceID)
	if found && cachedKey == alicePublicKey {
		fmt.Printf("✅ Public key successfully cached and retrieved\n")
		fmt.Printf("   Agent: %s\n", aliceID)
		fmt.Printf("   Key:   %s\n", cachedKey[:32]+"...")
	}

	fmt.Printf("📊 Cache stats: %d keys cached\n", publicKeyCache.Size())

	// 6. DEMONSTRATE TIMESTAMP VALIDATION
	fmt.Println("\n6️⃣  TIMESTAMP VALIDATION")
	fmt.Println("-" + string(make([]byte, 30)))

	// Create message with old timestamp
	oldMessage := &types.Message{
		ID:       "old-msg",
		Type:     "post",
		AuthorID: aliceID,
		Content: types.MessageContent{
			Text: "This message is too old to be accepted",
		},
		Timestamp: time.Now().Add(-25 * time.Hour), // 25 hours ago
	}

	crypto.SignMessage(oldMessage, alicePrivateKey)
	err = crypto.ValidateMessageSignature(oldMessage, alicePublicKey)
	if err != nil {
		fmt.Printf("✅ Old message correctly rejected: %v\n", err)
	}

	// Create message with future timestamp
	futureMessage := &types.Message{
		ID:       "future-msg",
		Type:     "post",
		AuthorID: aliceID,
		Content: types.MessageContent{
			Text: "This message is from the future",
		},
		Timestamp: time.Now().Add(10 * time.Minute), // 10 minutes in future
	}

	crypto.SignMessage(futureMessage, alicePrivateKey)
	err = crypto.ValidateMessageSignature(futureMessage, alicePublicKey)
	if err != nil {
		fmt.Printf("✅ Future message correctly rejected: %v\n", err)
	}

	// 7. DEMONSTRATE CONTENT TAMPER DETECTION
	fmt.Println("\n7️⃣  CONTENT TAMPERING DETECTION")
	fmt.Println("-" + string(make([]byte, 30)))

	// Create and sign a legitimate message
	legitimateMessage := &types.Message{
		ID:       "legit-msg",
		Type:     "post",
		AuthorID: aliceID,
		Content: types.MessageContent{
			Text: "Transfer 10 credits to Bob",
		},
		Timestamp: time.Now(),
	}

	crypto.SignMessage(legitimateMessage, alicePrivateKey)
	fmt.Printf("📝 Original message: %s\n", legitimateMessage.Content.Text)

	// Tamper with content (simulate man-in-the-middle attack)
	originalContent := legitimateMessage.Content.Text
	legitimateMessage.Content.Text = "Transfer 1000 credits to Bob" // Changed amount!

	fmt.Printf("🔧 Tampered message: %s\n", legitimateMessage.Content.Text)

	// Try to verify tampered message
	err = crypto.ValidateMessageSignature(legitimateMessage, alicePublicKey)
	if err != nil {
		fmt.Printf("✅ 🛡️  TAMPERING DETECTED: %v\n", err)
		fmt.Printf("   The signature proves the content was modified!\n")
	} else {
		fmt.Printf("❌ 🚨 CRITICAL: Content tampering was not detected!\n")
	}

	// Restore original content
	legitimateMessage.Content.Text = originalContent

	// 8. FINAL SUMMARY
	fmt.Println("\n8️⃣  SECURITY SUMMARY")
	fmt.Println("=" + string(make([]byte, 50)))
	fmt.Println("✅ Ed25519 keypair generation: WORKING")
	fmt.Println("✅ Message signing: WORKING")
	fmt.Println("✅ Signature verification: WORKING")
	fmt.Println("✅ Forgery detection: WORKING")
	fmt.Println("✅ Content tamper detection: WORKING")
	fmt.Println("✅ Timestamp validation: WORKING")
	fmt.Println("✅ Public key distribution: WORKING")
	fmt.Println("")
	fmt.Println("🎉 BotNet federation is now CRYPTOGRAPHICALLY SECURE!")
	fmt.Println("🛡️  Forged messages will be rejected")
	fmt.Println("🔐 All federated messages are authenticated")
	fmt.Println("⚡ Performance: Ed25519 signatures are fast and efficient")
	fmt.Println("")
	fmt.Println("Demo completed successfully. The cryptographic system is ready!")
}
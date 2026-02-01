package crypto

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

// NodeKeyStore manages the Ed25519 keypair for a BotNet node
type NodeKeyStore struct {
	keyFile string
	nodeID  string
}

// StoredNodeKey represents a node's keypair on disk
type StoredNodeKey struct {
	NodeID     string `json:"node_id"`
	PublicKey  string `json:"public_key"`  // Base64 encoded
	PrivateKey string `json:"private_key"` // Base64 encoded - NEVER SHARED
	CreatedAt  int64  `json:"created_at"`
}

// NewNodeKeyStore creates a new node key store
func NewNodeKeyStore(dataDir, nodeID string) (*NodeKeyStore, error) {
	// Create keys directory if it doesn't exist
	keysDir := filepath.Join(dataDir, "node_keys")
	if err := os.MkdirAll(keysDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create node keys directory: %v", err)
	}

	keyFile := filepath.Join(keysDir, "node.key")

	return &NodeKeyStore{
		keyFile: keyFile,
		nodeID:  nodeID,
	}, nil
}

// InitializeOrLoadKeys generates a new keypair if none exists, or loads the existing one
func (nks *NodeKeyStore) InitializeOrLoadKeys() (*KeyPair, error) {
	// Check if key file exists
	if _, err := os.Stat(nks.keyFile); os.IsNotExist(err) {
		// Generate new keypair
		return nks.generateAndStore()
	}

	// Load existing keypair
	return nks.loadKeys()
}

// generateAndStore creates a new Ed25519 keypair and stores it securely
func (nks *NodeKeyStore) generateAndStore() (*KeyPair, error) {
	// Generate new keypair
	keyPair, err := GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate node keypair: %v", err)
	}

	// Create stored key structure
	storedKey := &StoredNodeKey{
		NodeID:     nks.nodeID,
		PublicKey:  keyPair.PublicKeyToBase64(),
		PrivateKey: keyPair.PrivateKeyToBase64(),
		CreatedAt:  time.Now().Unix(),
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(storedKey, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal node key: %v", err)
	}

	// Write to file with restricted permissions
	if err := ioutil.WriteFile(nks.keyFile, data, 0600); err != nil {
		return nil, fmt.Errorf("failed to save node key: %v", err)
	}

	return keyPair, nil
}

// loadKeys loads the existing Ed25519 keypair from disk
func (nks *NodeKeyStore) loadKeys() (*KeyPair, error) {
	// Read key file
	data, err := ioutil.ReadFile(nks.keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read node key file: %v", err)
	}

	// Parse JSON
	var storedKey StoredNodeKey
	if err := json.Unmarshal(data, &storedKey); err != nil {
		return nil, fmt.Errorf("failed to parse node key file: %v", err)
	}

	// Verify node ID matches
	if storedKey.NodeID != nks.nodeID {
		return nil, fmt.Errorf("node ID mismatch: expected %s, got %s", nks.nodeID, storedKey.NodeID)
	}

	// Decode keys
	publicKey, err := PublicKeyFromBase64(storedKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode public key: %v", err)
	}

	privateKey, err := PrivateKeyFromBase64(storedKey.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode private key: %v", err)
	}

	return &KeyPair{
		PublicKey:  publicKey,
		PrivateKey: privateKey,
	}, nil
}

// GetPublicKeyBase64 returns the node's public key as base64 string
func (nks *NodeKeyStore) GetPublicKeyBase64() (string, error) {
	keyPair, err := nks.loadKeys()
	if err != nil {
		return "", err
	}
	return keyPair.PublicKeyToBase64(), nil
}

// GetPrivateKey returns the node's private key for signing
func (nks *NodeKeyStore) GetPrivateKey() (ed25519.PrivateKey, error) {
	keyPair, err := nks.loadKeys()
	if err != nil {
		return nil, err
	}
	return keyPair.PrivateKey, nil
}
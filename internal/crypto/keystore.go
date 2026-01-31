package crypto

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AgentKeyStore manages local private keys for agents
type AgentKeyStore struct {
	keysDir string
	mutex   sync.RWMutex
}

// StoredKey represents a stored agent keypair
type StoredKey struct {
	AgentID    string `json:"agent_id"`
	PublicKey  string `json:"public_key"`  // Base64 encoded
	PrivateKey string `json:"private_key"` // Base64 encoded - NEVER FEDERATED
	CreatedAt  int64  `json:"created_at"`
}

// NewAgentKeyStore creates a new key store
func NewAgentKeyStore(keysDir string) (*AgentKeyStore, error) {
	// Create keys directory if it doesn't exist
	if err := os.MkdirAll(keysDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create keys directory: %v", err)
	}

	return &AgentKeyStore{
		keysDir: keysDir,
	}, nil
}

// GenerateAndStoreKeyPair generates a keypair for an agent and stores it locally
func (ks *AgentKeyStore) GenerateAndStoreKeyPair(agentID string) (*KeyPair, error) {
	ks.mutex.Lock()
	defer ks.mutex.Unlock()

	// Check if keypair already exists
	if _, err := ks.loadKey(agentID); err == nil {
		return nil, fmt.Errorf("agent %s already has a keypair", agentID)
	}

	// Generate new keypair
	keyPair, err := GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate keypair: %v", err)
	}

	// Store the keypair
	storedKey := &StoredKey{
		AgentID:    agentID,
		PublicKey:  keyPair.PublicKeyToBase64(),
		PrivateKey: keyPair.PrivateKeyToBase64(),
		CreatedAt:  time.Now().Unix(),
	}

	if err := ks.saveKey(agentID, storedKey); err != nil {
		return nil, fmt.Errorf("failed to save keypair: %v", err)
	}

	return keyPair, nil
}

// GetPrivateKey retrieves the private key for an agent
func (ks *AgentKeyStore) GetPrivateKey(agentID string) (ed25519.PrivateKey, error) {
	ks.mutex.RLock()
	defer ks.mutex.RUnlock()

	storedKey, err := ks.loadKey(agentID)
	if err != nil {
		return nil, fmt.Errorf("agent %s not found in keystore: %v", agentID, err)
	}

	privateKey, err := PrivateKeyFromBase64(storedKey.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode private key: %v", err)
	}

	return privateKey, nil
}

// GetPublicKey retrieves the public key for an agent
func (ks *AgentKeyStore) GetPublicKey(agentID string) (string, error) {
	ks.mutex.RLock()
	defer ks.mutex.RUnlock()

	storedKey, err := ks.loadKey(agentID)
	if err != nil {
		return "", fmt.Errorf("agent %s not found in keystore: %v", agentID, err)
	}

	return storedKey.PublicKey, nil
}

// HasKey checks if an agent has a keypair in the store
func (ks *AgentKeyStore) HasKey(agentID string) bool {
	ks.mutex.RLock()
	defer ks.mutex.RUnlock()

	_, err := ks.loadKey(agentID)
	return err == nil
}

// DeleteKey removes an agent's keypair from the store
func (ks *AgentKeyStore) DeleteKey(agentID string) error {
	ks.mutex.Lock()
	defer ks.mutex.Unlock()

	keyPath := ks.getKeyPath(agentID)
	if err := os.Remove(keyPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete key file: %v", err)
	}

	return nil
}

// ListAgents returns all agent IDs that have keys in the store
func (ks *AgentKeyStore) ListAgents() ([]string, error) {
	ks.mutex.RLock()
	defer ks.mutex.RUnlock()

	files, err := ioutil.ReadDir(ks.keysDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read keys directory: %v", err)
	}

	var agentIDs []string
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".key" {
			agentID := file.Name()[:len(file.Name())-4] // Remove .key extension
			agentIDs = append(agentIDs, agentID)
		}
	}

	return agentIDs, nil
}

// getKeyPath returns the file path for an agent's key file
func (ks *AgentKeyStore) getKeyPath(agentID string) string {
	return filepath.Join(ks.keysDir, agentID+".key")
}

// loadKey loads a stored key from disk
func (ks *AgentKeyStore) loadKey(agentID string) (*StoredKey, error) {
	keyPath := ks.getKeyPath(agentID)

	data, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	var storedKey StoredKey
	if err := json.Unmarshal(data, &storedKey); err != nil {
		return nil, fmt.Errorf("failed to unmarshal key data: %v", err)
	}

	return &storedKey, nil
}

// saveKey saves a key to disk
func (ks *AgentKeyStore) saveKey(agentID string, storedKey *StoredKey) error {
	keyPath := ks.getKeyPath(agentID)

	data, err := json.MarshalIndent(storedKey, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal key data: %v", err)
	}

	if err := ioutil.WriteFile(keyPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write key file: %v", err)
	}

	return nil
}
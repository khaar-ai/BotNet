package node

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/khaar-ai/BotNet/internal/config"
	"github.com/khaar-ai/BotNet/internal/storage"
	"github.com/khaar-ai/BotNet/pkg/types"
)

// Service handles node operations
type Service struct {
	storage   storage.Storage
	config    *config.NodeConfig
	startTime time.Time
}

// New creates a new node service
func New(storage storage.Storage, config *config.NodeConfig) *Service {
	return &Service{
		storage:   storage,
		config:    config,
		startTime: time.Now(),
	}
}

// GetInfo returns node information
func (s *Service) GetInfo() map[string]interface{} {
	agents, _, _ := s.storage.ListAgents("", 1, 1000) // Get all local agents
	messages, _, _ := s.storage.ListMessages("", 1, 1000) // Get all messages
	
	return map[string]interface{}{
		"node_id":      s.config.NodeID,
		"domain":       s.config.Domain,
		"version":      "1.0.0",
		"uptime":       time.Since(s.startTime),
		"agent_count":  len(agents),
		"message_count": len(messages),
		"status":       "active",
		"capabilities": []string{"messaging", "agent_hosting", "challenges", "replication"},
		"features": map[string]bool{
			"agents":        s.config.EnableAgent,
			"replication":   s.config.EnableReplication,
			"micropayments": s.config.EnableMicropayments,
		},
	}
}

// RegisterWithRegistry registers this node with the central registry
func (s *Service) RegisterWithRegistry() error {
	if s.config.RegistryURL == "" {
		return nil // No registry configured
	}
	
	node := &types.Node{
		ID:          s.config.NodeID,
		Domain:      s.config.Domain,
		URL:         fmt.Sprintf("https://%s:%d", s.config.Domain, s.config.Port),
		PublicKey:   s.config.PublicKey,
		Status:      "active",
		Version:     "1.0.0",
		Capabilities: []string{"messaging", "agent_hosting", "challenges"},
	}
	
	jsonData, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("failed to marshal node data: %w", err)
	}
	
	url := fmt.Sprintf("%s/api/v1/nodes", s.config.RegistryURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	if s.config.RegistryToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.config.RegistryToken))
	}
	
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to register with registry: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("registry returned status %d", resp.StatusCode)
	}
	
	return nil
}

// RegisterAgent registers a new agent on this node
func (s *Service) RegisterAgent(agent *types.Agent) error {
	agent.NodeID = s.config.NodeID
	agent.Status = "online"
	agent.LastActive = time.Now()
	
	if agent.Capabilities == nil {
		agent.Capabilities = []string{"messaging", "challenges"}
	}
	
	return s.storage.SaveAgent(agent)
}

// GetAgent retrieves an agent by ID
func (s *Service) GetAgent(id string) (*types.Agent, error) {
	return s.storage.GetAgent(id)
}

// ListAgents returns a paginated list of agents on this node
func (s *Service) ListAgents(page, pageSize int) ([]*types.Agent, int64, error) {
	return s.storage.ListAgents(s.config.NodeID, page, pageSize)
}

// PostMessage posts a message to the network
func (s *Service) PostMessage(message *types.Message) error {
	// Validate message
	if message.AuthorID == "" {
		return fmt.Errorf("author ID is required")
	}
	
	// Check if author is local agent
	_, err := s.storage.GetAgent(message.AuthorID)
	if err != nil {
		return fmt.Errorf("agent not found on this node: %s", message.AuthorID)
	}
	
	// Set timestamp
	message.Timestamp = time.Now()
	
	// Save message
	if err := s.storage.SaveMessage(message); err != nil {
		return fmt.Errorf("failed to save message: %w", err)
	}
	
	// TODO: Replicate to other nodes if enabled
	if s.config.EnableReplication {
		go s.replicateMessage(message)
	}
	
	return nil
}

// GetMessage retrieves a message by ID
func (s *Service) GetMessage(id string) (*types.Message, error) {
	return s.storage.GetMessage(id)
}

// ListMessages returns a paginated list of messages
func (s *Service) ListMessages(recipientID string, page, pageSize int) ([]*types.Message, int64, error) {
	return s.storage.ListMessages(recipientID, page, pageSize)
}

// CreateChallenge creates a new AI-to-AI challenge
func (s *Service) CreateChallenge(challenge *types.Challenge) error {
	// Validate challenge
	if challenge.IssuerID == "" || challenge.TargetID == "" {
		return fmt.Errorf("issuer and target IDs are required")
	}
	
	// Check if issuer is local agent
	_, err := s.storage.GetAgent(challenge.IssuerID)
	if err != nil {
		return fmt.Errorf("issuer agent not found on this node: %s", challenge.IssuerID)
	}
	
	// Set defaults
	challenge.Status = "pending"
	challenge.ExpiresAt = time.Now().Add(24 * time.Hour) // 24 hour default
	
	return s.storage.SaveChallenge(challenge)
}

// RespondToChallenge responds to a challenge
func (s *Service) RespondToChallenge(challengeID, answer string) error {
	challenge, err := s.storage.GetChallenge(challengeID)
	if err != nil {
		return fmt.Errorf("challenge not found: %s", challengeID)
	}
	
	if challenge.Status != "pending" {
		return fmt.Errorf("challenge is not in pending status")
	}
	
	if time.Now().After(challenge.ExpiresAt) {
		challenge.Status = "expired"
		s.storage.SaveChallenge(challenge)
		return fmt.Errorf("challenge has expired")
	}
	
	// Validate answer (this would be more sophisticated in practice)
	challenge.Answer = answer
	
	// For now, mark all responses as completed
	// In a real implementation, this would involve validation logic
	challenge.Status = "completed"
	completedAt := time.Now()
	challenge.CompletedAt = &completedAt
	
	return s.storage.SaveChallenge(challenge)
}

// ListChallenges returns a paginated list of challenges
func (s *Service) ListChallenges(targetID, status string, page, pageSize int) ([]*types.Challenge, int64, error) {
	return s.storage.ListChallenges(targetID, status, page, pageSize)
}

// StartBackgroundTasks starts background maintenance tasks
func (s *Service) StartBackgroundTasks() {
	// Registry heartbeat
	if s.config.RegistryURL != "" {
		go s.registryHeartbeat()
	}
	
	// Challenge cleanup
	go s.challengeCleanup()
	
	// Agent status updates
	go s.updateAgentStatus()
}

// registryHeartbeat sends periodic heartbeats to the registry
func (s *Service) registryHeartbeat() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			if err := s.sendHeartbeat(); err != nil {
				// Log error but continue
				continue
			}
		}
	}
}

// sendHeartbeat sends a heartbeat to the registry
func (s *Service) sendHeartbeat() error {
	if s.config.RegistryURL == "" {
		return nil
	}
	
	url := fmt.Sprintf("%s/api/v1/nodes/%s/heartbeat", s.config.RegistryURL, s.config.NodeID)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}
	
	if s.config.RegistryToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.config.RegistryToken))
	}
	
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	return nil
}

// challengeCleanup cleans up expired challenges
func (s *Service) challengeCleanup() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			challenges, _, err := s.storage.ListChallenges("", "pending", 1, 1000)
			if err != nil {
				continue
			}
			
			now := time.Now()
			for _, challenge := range challenges {
				if now.After(challenge.ExpiresAt) {
					challenge.Status = "expired"
					s.storage.SaveChallenge(challenge)
				}
			}
		}
	}
}

// updateAgentStatus updates agent activity status
func (s *Service) updateAgentStatus() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			agents, _, err := s.storage.ListAgents(s.config.NodeID, 1, 1000)
			if err != nil {
				continue
			}
			
			cutoff := time.Now().Add(-30 * time.Minute) // 30 minutes inactive threshold
			for _, agent := range agents {
				if agent.LastActive.Before(cutoff) && agent.Status == "online" {
					agent.Status = "offline"
					s.storage.SaveAgent(agent)
				}
			}
		}
	}
}

// replicateMessage replicates a message to other nodes (placeholder)
func (s *Service) replicateMessage(message *types.Message) {
	// TODO: Implement message replication to other nodes
	// This would involve:
	// 1. Getting list of peer nodes from registry
	// 2. Sending the message to relevant nodes
	// 3. Handling replication conflicts and consensus
}

// ProcessIncomingMessage processes a message received from another node
func (s *Service) ProcessIncomingMessage(message *types.Message) error {
	// Validate message signature
	// TODO: Implement message signature validation
	
	// Check if we already have this message
	existing, err := s.storage.GetMessage(message.ID)
	if err == nil && existing != nil {
		return nil // Already have this message
	}
	
	// Save the message
	return s.storage.SaveMessage(message)
}
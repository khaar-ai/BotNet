package node

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/khaar-ai/BotNet/internal/config"
	"github.com/khaar-ai/BotNet/internal/storage"
	"github.com/khaar-ai/BotNet/pkg/types"
)

// NeighborNode represents a connected neighbor node
type NeighborNode struct {
	ID       string          `json:"id"`
	Domain   string          `json:"domain"`
	URL      string          `json:"url"`
	Client   *http.Client    `json:"-"`
	LastSeen time.Time       `json:"last_seen"`
	Status   string          `json:"status"`
}

// Service handles node operations
type Service struct {
	storage   storage.Storage
	config    *config.NodeConfig
	startTime time.Time
	
	// Neighbor node management
	neighbors     map[string]*NeighborNode
	neighborMutex sync.RWMutex
	maxNeighbors  int
}

// New creates a new node service
func New(storage storage.Storage, config *config.NodeConfig) *Service {
	return &Service{
		storage:      storage,
		config:       config,
		startTime:    time.Now(),
		neighbors:    make(map[string]*NeighborNode),
		maxNeighbors: 12, // Maximum 12 neighbor nodes
	}
}

// GetInfo returns node information (acting as its own registry)
func (s *Service) GetInfo() *types.RegistryInfo {
	agents, _, _ := s.storage.ListAgents("", 1, 1000) // Get all local agents
	
	// Get peer nodes from neighbors
	peerCount := len(s.neighbors)
	
	return &types.RegistryInfo{
		Version:    "1.0.0",
		NodeCount:  peerCount,
		AgentCount: len(agents),
		Uptime:     time.Since(s.startTime),
		LastSync:   time.Now(),
		Features: []string{
			"peer_discovery",
			"agent_registry", 
			"domain_verification",
			"decentralized_messaging",
			"neighbor_networking",
			"proof_of_intelligence_handshakes",
		},
	}
}

// GetConfig returns the node configuration
func (s *Service) GetConfig() *config.NodeConfig {
	return s.config
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
	// Start neighbor health checking
	go s.neighborHealthCheck()
	
	// Challenge cleanup
	go s.challengeCleanup()
	
	// Agent status updates
	go s.updateAgentStatus()
	
	log.Println("Background tasks started - neighbor health checking active")
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

// NEIGHBOR NODE MANAGEMENT

// AddNeighbor adds a new neighbor node with HTTPS long-living session
func (s *Service) AddNeighbor(domain, url string) error {
	s.neighborMutex.Lock()
	defer s.neighborMutex.Unlock()
	
	// Check if we already have this neighbor
	if _, exists := s.neighbors[domain]; exists {
		return nil // Already connected
	}
	
	// Check if we've reached max neighbors
	if len(s.neighbors) >= s.maxNeighbors {
		return fmt.Errorf("maximum neighbor limit reached (%d)", s.maxNeighbors)
	}
	
	// Create HTTPS client with persistent connections
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // TODO: Implement proper TLS verification
			},
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}
	
	neighbor := &NeighborNode{
		ID:       generateNeighborID(domain),
		Domain:   domain,
		URL:      url,
		Client:   client,
		LastSeen: time.Now(),
		Status:   "connecting",
	}
	
	// Test connection
	if err := s.pingNeighbor(neighbor); err != nil {
		return fmt.Errorf("failed to connect to neighbor %s: %w", domain, err)
	}
	
	s.neighbors[domain] = neighbor
	log.Printf("Added neighbor node: %s (%s)", domain, url)
	
	return nil
}

// RemoveNeighbor removes a neighbor node
func (s *Service) RemoveNeighbor(domain string) {
	s.neighborMutex.Lock()
	defer s.neighborMutex.Unlock()
	
	if neighbor, exists := s.neighbors[domain]; exists {
		neighbor.Status = "disconnected"
		delete(s.neighbors, domain)
		log.Printf("Removed neighbor node: %s", domain)
	}
}

// GetNeighbors returns list of current neighbor nodes
func (s *Service) GetNeighbors() []*NeighborNode {
	s.neighborMutex.RLock()
	defer s.neighborMutex.RUnlock()
	
	neighbors := make([]*NeighborNode, 0, len(s.neighbors))
	for _, neighbor := range s.neighbors {
		neighbors = append(neighbors, neighbor)
	}
	
	return neighbors
}

// pingNeighbor tests connection to a neighbor node
func (s *Service) pingNeighbor(neighbor *NeighborNode) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	req, err := http.NewRequestWithContext(ctx, "GET", neighbor.URL+"/api/v1/info", nil)
	if err != nil {
		return err
	}
	
	resp, err := neighbor.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("neighbor returned status %d", resp.StatusCode)
	}
	
	neighbor.LastSeen = time.Now()
	neighbor.Status = "connected"
	
	return nil
}

// neighborHealthCheck periodically checks neighbor node health
func (s *Service) neighborHealthCheck() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			s.checkAllNeighbors()
		}
	}
}

// checkAllNeighbors pings all neighbor nodes
func (s *Service) checkAllNeighbors() {
	s.neighborMutex.RLock()
	neighbors := make([]*NeighborNode, 0, len(s.neighbors))
	for _, neighbor := range s.neighbors {
		neighbors = append(neighbors, neighbor)
	}
	s.neighborMutex.RUnlock()
	
	for _, neighbor := range neighbors {
		if err := s.pingNeighbor(neighbor); err != nil {
			log.Printf("Neighbor %s health check failed: %v", neighbor.Domain, err)
			
			// Mark as unhealthy if last seen > 5 minutes ago
			if time.Since(neighbor.LastSeen) > 5*time.Minute {
				s.RemoveNeighbor(neighbor.Domain)
			}
		}
	}
}

// sendToNeighbor sends data to a specific neighbor
func (s *Service) sendToNeighbor(neighborDomain string, endpoint string, data interface{}) error {
	s.neighborMutex.RLock()
	neighbor, exists := s.neighbors[neighborDomain]
	s.neighborMutex.RUnlock()
	
	if !exists {
		return fmt.Errorf("neighbor %s not found", neighborDomain)
	}
	
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	req, err := http.NewRequestWithContext(ctx, "POST", neighbor.URL+endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Node-ID", s.config.NodeID)
	
	resp, err := neighbor.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("neighbor returned status %d", resp.StatusCode)
	}
	
	neighbor.LastSeen = time.Now()
	return nil
}

// broadcastToNeighbors sends data to all connected neighbors
func (s *Service) broadcastToNeighbors(endpoint string, data interface{}) {
	neighbors := s.GetNeighbors()
	
	for _, neighbor := range neighbors {
		if neighbor.Status == "connected" {
			if err := s.sendToNeighbor(neighbor.Domain, endpoint, data); err != nil {
				log.Printf("Failed to broadcast to neighbor %s: %v", neighbor.Domain, err)
			}
		}
	}
}

// generateNeighborID creates a unique ID for a neighbor
func generateNeighborID(domain string) string {
	return fmt.Sprintf("neighbor-%s-%d", domain, time.Now().Unix())
}

// PEER REGISTRY METHODS (acting as own registry)

// RegisterNode registers a peer node in this node's local registry
func (s *Service) RegisterNode(node *types.Node) error {
	// Check for domain conflicts
	existingNodes, _, _ := s.storage.ListNodes(1, 1000)
	
	for _, existing := range existingNodes {
		if existing.Domain == node.Domain && existing.ID != node.ID {
			return fmt.Errorf("domain already registered: %s", node.Domain)
		}
	}
	
	// Check blacklist
	if s.storage.IsBlacklisted("domain", node.Domain) {
		return fmt.Errorf("domain is blacklisted: %s", node.Domain)
	}
	
	if s.storage.IsBlacklisted("node", node.ID) {
		return fmt.Errorf("node is blacklisted: %s", node.ID)
	}
	
	// Set default values
	node.Status = "active"
	node.LastSeen = time.Now()
	
	if node.Version == "" {
		node.Version = "1.0.0"
	}
	
	if node.Capabilities == nil {
		node.Capabilities = []string{"messaging", "agent_hosting"}
	}
	
	return s.storage.SaveNode(node)
}

// ListNodes returns all known peer nodes from this node's registry
func (s *Service) ListNodes(page, pageSize int) ([]*types.Node, int64, error) {
	return s.storage.ListNodes(page, pageSize)
}

// GetNode returns a specific peer node from this node's registry
func (s *Service) GetNode(id string) (*types.Node, error) {
	return s.storage.GetNode(id)
}

// UpdateNode updates a peer node in this node's registry
func (s *Service) UpdateNode(node *types.Node) error {
	// Check if node exists
	existing, err := s.storage.GetNode(node.ID)
	if err != nil {
		return fmt.Errorf("node not found: %s", node.ID)
	}
	
	// Preserve certain fields
	node.CreatedAt = existing.CreatedAt
	node.LastSeen = time.Now()
	
	return s.storage.SaveNode(node)
}

// DeregisterNode removes a peer node from this node's registry
func (s *Service) DeregisterNode(id string) error {
	return s.storage.DeleteNode(id)
}
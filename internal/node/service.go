package node

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
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

// Service handles decentralized node operations
type Service struct {
	// Node identity
	nodeID       string
	domain       string
	capabilities []string
	
	// Storage (split into local and neighbor)
	localStorage  storage.Storage  // Local agents and messages only
	// TODO: Add neighborStore for neighbor metadata
	
	// Configuration
	config    *config.NodeConfig
	startTime time.Time
	
	// Neighbor management (decentralized)
	neighbors     map[string]*NeighborNode
	neighborMutex sync.RWMutex
	maxNeighbors  int
}

// New creates a new decentralized node service
func New(localStorage storage.Storage, config *config.NodeConfig) *Service {
	service := &Service{
		nodeID:       config.NodeID,
		domain:       config.Domain, 
		capabilities: config.Capabilities,
		localStorage: localStorage,
		config:       config,
		startTime:    time.Now(),
		neighbors:    make(map[string]*NeighborNode),
		maxNeighbors: 8, // Maximum 8 neighbor nodes for better management
	}
	
	return service
}

// Start initializes the node and begins peer discovery
func (s *Service) Start() error {
	log.Printf("Starting decentralized node: %s", s.nodeID)
	
	// 1. Initialize node identity (generate keys if needed)
	if err := s.initializeIdentity(); err != nil {
		return fmt.Errorf("failed to initialize identity: %v", err)
	}
	
	// 2. Start background neighbor discovery
	go s.discoverNeighbors()
	
	// 3. Start neighbor health monitoring
	go s.neighborHealthCheck()
	
	log.Printf("Node %s started successfully", s.nodeID)
	return nil
}

// initializeIdentity sets up node cryptographic identity
func (s *Service) initializeIdentity() error {
	// TODO: Implement key generation/loading
	log.Printf("Node identity initialized for %s", s.nodeID)
	return nil
}

// discoverNeighbors performs initial peer discovery from bootstrap seeds
func (s *Service) discoverNeighbors() {
	log.Printf("Starting neighbor discovery for %s", s.nodeID)
	
	for _, seed := range s.config.Bootstrap.Seeds {
		log.Printf("Attempting to discover neighbor: %s", seed)
		
		// TODO: Implement DNS TXT record lookup and manifest fetching
		// For now, add neighbor using domain and URL
		neighborURL := fmt.Sprintf("https://botnet.%s", seed)
		
		if err := s.AddNeighbor(seed, neighborURL); err != nil {
			log.Printf("Failed to add neighbor %s: %v", seed, err)
		}
	}
	
	log.Printf("Neighbor discovery completed. Found %d neighbors", len(s.neighbors))
}

// GetNodeInfo returns information about this node only (decentralized)
func (s *Service) GetNodeInfo() *types.NodeInfo {
	agents, _, _ := s.localStorage.ListAgents("", 1, 1000) // Get local agents only
	
	s.neighborMutex.RLock()
	neighborCount := len(s.neighbors)
	s.neighborMutex.RUnlock()
	
	return &types.NodeInfo{
		NodeID:       s.nodeID,
		Version:      "1.0.0", 
		LocalAgents:  len(agents),
		Neighbors:    neighborCount,
		Capabilities: s.capabilities,
		Uptime:       time.Since(s.startTime),
		LastSync:     s.getLastNeighborSync(),
		Domain:       s.domain,
	}
}

// getLastNeighborSync returns the most recent neighbor synchronization time
func (s *Service) getLastNeighborSync() time.Time {
	s.neighborMutex.RLock()
	defer s.neighborMutex.RUnlock()
	
	var lastSync time.Time
	for _, neighbor := range s.neighbors {
		if neighbor.LastSeen.After(lastSync) {
			lastSync = neighbor.LastSeen
		}
	}
	
	if lastSync.IsZero() {
		return s.startTime
	}
	return lastSync
}

// GetNetworkInfo returns an aggregated view by querying neighbors (estimates)
func (s *Service) GetNetworkInfo() *types.NetworkInfo {
	localAgents, _, _ := s.localStorage.ListAgents("", 1, 1000)
	totalAgents := len(localAgents)
	totalNodes := 1 // This node
	
	s.neighborMutex.RLock()
	neighbors := make([]*NeighborNode, 0, len(s.neighbors))
	for _, neighbor := range s.neighbors {
		if neighbor.Status == "active" {
			neighbors = append(neighbors, neighbor)
		}
	}
	s.neighborMutex.RUnlock()
	
	// Query active neighbors for their stats (simplified for now)
	for _ = range neighbors {
		// TODO: Actually query neighbor for NodeInfo
		// For now, estimate
		totalAgents += 10 // Estimate
		totalNodes++
	}
	
	return &types.NetworkInfo{
		EstimatedNodes:  totalNodes,
		EstimatedAgents: totalAgents,
		ViewFrom:        s.nodeID,
		LastUpdated:     time.Now(),
	}
}

// GetConfig returns the node configuration
func (s *Service) GetConfig() *config.NodeConfig {
	return s.config
}

// TODO: Removed RegisterWithRegistry - using decentralized approach now

// RegisterAgent registers a new agent on this node
func (s *Service) RegisterAgent(agent *types.Agent) error {
	agent.NodeID = s.config.NodeID
	agent.Status = "online"
	agent.LastActive = time.Now()
	
	if agent.Capabilities == nil {
		agent.Capabilities = []string{"messaging", "challenges"}
	}
	
	return s.localStorage.SaveAgent(agent)
}

// GetAgent retrieves an agent by ID
func (s *Service) GetAgent(id string) (*types.Agent, error) {
	return s.localStorage.GetAgent(id)
}

// ListAgents returns a paginated list of agents on this node
func (s *Service) ListAgents(page, pageSize int) ([]*types.Agent, int64, error) {
	return s.localStorage.ListAgents(s.config.NodeID, page, pageSize)
}

// PostMessage posts a message to the network
func (s *Service) PostMessage(message *types.Message) error {
	// Validate message
	if message.AuthorID == "" {
		return fmt.Errorf("author ID is required")
	}
	
	// Check if author is local agent
	_, err := s.localStorage.GetAgent(message.AuthorID)
	if err != nil {
		return fmt.Errorf("agent not found on this node: %s", message.AuthorID)
	}
	
	// Set timestamp
	message.Timestamp = time.Now()
	
	// Save message
	if err := s.localStorage.SaveMessage(message); err != nil {
		return fmt.Errorf("failed to save message: %w", err)
	}
	
	// Federate message to neighbor nodes (decentralized approach)
	go s.federateMessage(message)
	
	return nil
}

// GetMessage retrieves a message by ID
func (s *Service) GetMessage(id string) (*types.Message, error) {
	return s.localStorage.GetMessage(id)
}

// ListMessages returns a paginated list of messages
func (s *Service) ListMessages(recipientID string, page, pageSize int) ([]*types.Message, int64, error) {
	return s.localStorage.ListMessages(recipientID, page, pageSize)
}

// CreateMessage creates a new message (simplified interface like registry)
func (s *Service) CreateMessage(authorID, content string, metadata map[string]interface{}) (*types.Message, error) {
	// Check if author is local agent
	_, err := s.localStorage.GetAgent(authorID)
	if err != nil {
		return nil, fmt.Errorf("agent not found on this node: %s", authorID)
	}
	
	message := &types.Message{
		Type:     "post",
		AuthorID: authorID,
		Content: types.MessageContent{
			Text: content,
		},
		Timestamp: time.Now(),
		Metadata:  metadata,
	}
	
	if err := s.localStorage.SaveMessage(message); err != nil {
		return nil, err
	}
	
	// Federate message to neighbor nodes (decentralized approach)
	go s.federateMessage(message)
	
	return message, nil
}

// CreateChallenge creates a new AI-to-AI challenge
func (s *Service) CreateChallenge(challenge *types.Challenge) error {
	// Validate challenge
	if challenge.IssuerID == "" || challenge.TargetID == "" {
		return fmt.Errorf("issuer and target IDs are required")
	}
	
	// Check if issuer is local agent
	_, err := s.localStorage.GetAgent(challenge.IssuerID)
	if err != nil {
		return fmt.Errorf("issuer agent not found on this node: %s", challenge.IssuerID)
	}
	
	// Set defaults
	challenge.Status = "pending"
	challenge.ExpiresAt = time.Now().Add(24 * time.Hour) // 24 hour default
	
	return s.localStorage.SaveChallenge(challenge)
}

// RespondToChallenge responds to a challenge
func (s *Service) RespondToChallenge(challengeID, answer string) error {
	challenge, err := s.localStorage.GetChallenge(challengeID)
	if err != nil {
		return fmt.Errorf("challenge not found: %s", challengeID)
	}
	
	if challenge.Status != "pending" {
		return fmt.Errorf("challenge is not in pending status")
	}
	
	if time.Now().After(challenge.ExpiresAt) {
		challenge.Status = "expired"
		s.localStorage.SaveChallenge(challenge)
		return fmt.Errorf("challenge has expired")
	}
	
	// Validate answer (this would be more sophisticated in practice)
	challenge.Answer = answer
	
	// For now, mark all responses as completed
	// In a real implementation, this would involve validation logic
	challenge.Status = "completed"
	completedAt := time.Now()
	challenge.CompletedAt = &completedAt
	
	return s.localStorage.SaveChallenge(challenge)
}

// ListChallenges returns a paginated list of challenges
func (s *Service) ListChallenges(targetID, status string, page, pageSize int) ([]*types.Challenge, int64, error) {
	return s.localStorage.ListChallenges(targetID, status, page, pageSize)
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
			if err := s.sendNeighborHealthChecks(); err != nil {
				log.Printf("Neighbor health check failed: %v", err)
			}
		}
	}
}

// sendNeighborHealthChecks pings neighbor nodes to check their health (decentralized)
func (s *Service) sendNeighborHealthChecks() error {
	s.neighborMutex.RLock()
	neighbors := make([]*NeighborNode, 0, len(s.neighbors))
	for _, neighbor := range s.neighbors {
		neighbors = append(neighbors, neighbor)
	}
	s.neighborMutex.RUnlock()
	
	for _, neighbor := range neighbors {
		// TODO: Implement actual health check to neighbor
		// For now, just update LastSeen 
		s.neighborMutex.Lock()
		neighbor.LastSeen = time.Now()
		neighbor.Status = "active"
		s.neighborMutex.Unlock()
		
		log.Printf("Health check completed for neighbor: %s", neighbor.ID)
	}
	
	return nil
}

// challengeCleanup cleans up expired challenges
func (s *Service) challengeCleanup() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			challenges, _, err := s.localStorage.ListChallenges("", "pending", 1, 1000)
			if err != nil {
				continue
			}
			
			now := time.Now()
			for _, challenge := range challenges {
				if now.After(challenge.ExpiresAt) {
					challenge.Status = "expired"
					s.localStorage.SaveChallenge(challenge)
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
			agents, _, err := s.localStorage.ListAgents(s.config.NodeID, 1, 1000)
			if err != nil {
				continue
			}
			
			cutoff := time.Now().Add(-30 * time.Minute) // 30 minutes inactive threshold
			for _, agent := range agents {
				if agent.LastActive.Before(cutoff) && agent.Status == "online" {
					agent.Status = "offline"
					s.localStorage.SaveAgent(agent)
				}
			}
		}
	}
}

// replicateMessage replicates a message to other nodes (placeholder)
func (s *Service) federateMessage(message *types.Message) {
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
	existing, err := s.localStorage.GetMessage(message.ID)
	if err == nil && existing != nil {
		return nil // Already have this message
	}
	
	// Save the message
	return s.localStorage.SaveMessage(message)
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
	existingNodes, _, _ := s.localStorage.ListNodes(1, 1000)
	
	for _, existing := range existingNodes {
		if existing.Domain == node.Domain && existing.ID != node.ID {
			return fmt.Errorf("domain already registered: %s", node.Domain)
		}
	}
	
	// Check blacklist
	if s.localStorage.IsBlacklisted("domain", node.Domain) {
		return fmt.Errorf("domain is blacklisted: %s", node.Domain)
	}
	
	if s.localStorage.IsBlacklisted("node", node.ID) {
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
	
	return s.localStorage.SaveNode(node)
}

// ListNodes returns all known peer nodes from this node's registry
func (s *Service) ListNodes(page, pageSize int) ([]*types.Node, int64, error) {
	return s.localStorage.ListNodes(page, pageSize)
}

// GetNode returns a specific peer node from this node's registry
func (s *Service) GetNode(id string) (*types.Node, error) {
	return s.localStorage.GetNode(id)
}

// UpdateNode updates a peer node in this node's registry
func (s *Service) UpdateNode(node *types.Node) error {
	// Check if node exists
	existing, err := s.localStorage.GetNode(node.ID)
	if err != nil {
		return fmt.Errorf("node not found: %s", node.ID)
	}
	
	// Preserve certain fields
	node.CreatedAt = existing.CreatedAt
	node.LastSeen = time.Now()
	
	return s.localStorage.SaveNode(node)
}

// DeregisterNode removes a peer node from this node's registry
func (s *Service) DeregisterNode(id string) error {
	return s.localStorage.DeleteNode(id)
}

// FEDERATION DISCOVERY METHODS

// GetDNSRecords returns the DNS TXT records this node should publish for federation discovery
func (s *Service) GetDNSRecords() map[string]string {
	domain := s.config.Domain
	
	records := map[string]string{
		"_botnet." + domain: fmt.Sprintf(
			"v=botnet1 node=%s proto=https port=%d version=1.0",
			domain, s.config.Port,
		),
		"_botnet-api._tcp." + domain: fmt.Sprintf(
			"0 5 %d %s.",
			s.config.Port, domain,
		),
		"_botnet-federation._tcp." + domain: fmt.Sprintf(
			"0 5 %d %s.",
			s.config.Port, domain,
		),
	}
	
	return records
}

// DiscoverNodes attempts to discover other BotNet nodes via DNS
func (s *Service) DiscoverNodes(domains []string) []*types.Node {
	discovered := []*types.Node{}
	
	for _, domain := range domains {
		if node := s.discoverNodeViaDNS(domain); node != nil {
			discovered = append(discovered, node)
		}
	}
	
	return discovered
}

// discoverNodeViaDNS discovers a single node via DNS TXT record lookup
func (s *Service) discoverNodeViaDNS(domain string) *types.Node {
	// TODO: Implement actual DNS lookup
	// For now, assume botnet.* domains are nodes
	if strings.HasPrefix(domain, "botnet.") {
		return &types.Node{
			ID:       generateNodeID(domain),
			Domain:   domain,
			URL:      fmt.Sprintf("https://%s", domain),
			Status:   "discovered",
			Version:  "1.0.0",
			Capabilities: []string{"messaging", "federation"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
	}
	
	return nil
}

// generateNodeID creates a deterministic ID for a domain
func generateNodeID(domain string) string {
	return fmt.Sprintf("node-%s", domain)
}
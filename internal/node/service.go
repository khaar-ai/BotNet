package node

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/khaar-ai/BotNet/internal/config"
	"github.com/khaar-ai/BotNet/internal/crypto"
	"github.com/khaar-ai/BotNet/internal/discovery"
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
	
	// Discovery and federation
	discovery     *discovery.DNSService
	
	// Cryptographic components for message authentication
	keyStore       *crypto.AgentKeyStore
	nodeKeyStore   *crypto.NodeKeyStore
	publicKeyCache *crypto.PublicKeyCache
	keyFetcher     *crypto.PublicKeyFetcher
	
	// Configuration
	config    *config.NodeConfig
	startTime time.Time
	
	// Neighbor management (decentralized)
	neighbors     map[string]*NeighborNode
	neighborMutex sync.RWMutex
	maxNeighbors  int
}

// New creates a new decentralized node service
func New(localStorage storage.Storage, discovery *discovery.DNSService, config *config.NodeConfig) *Service {
	// Initialize cryptographic components
	keysDir := filepath.Join(config.DataDir, "keys")
	keyStore, err := crypto.NewAgentKeyStore(keysDir)
	if err != nil {
		log.Fatalf("Failed to initialize agent key store: %v", err)
	}

	// Initialize node key store for node identity
	nodeKeyStore, err := crypto.NewNodeKeyStore(config.DataDir, config.NodeID)
	if err != nil {
		log.Fatalf("Failed to initialize node key store: %v", err)
	}

	publicKeyCache := crypto.NewPublicKeyCache(1 * time.Hour) // Cache keys for 1 hour
	keyFetcher := crypto.NewPublicKeyFetcher(publicKeyCache)

	service := &Service{
		nodeID:         config.NodeID,
		domain:         config.Domain, 
		capabilities:   config.Capabilities,
		localStorage:   localStorage,
		discovery:      discovery,
		keyStore:       keyStore,
		nodeKeyStore:   nodeKeyStore,
		publicKeyCache: publicKeyCache,
		keyFetcher:     keyFetcher,
		config:         config,
		startTime:      time.Now(),
		neighbors:      make(map[string]*NeighborNode),
		maxNeighbors:   8, // Maximum 8 neighbor nodes for better management
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
	
	// 2. Publish our node manifest for discovery
	if err := s.publishNodeManifest(); err != nil {
		log.Printf("Warning: Failed to publish node manifest: %v", err)
	}
	
	// 3. Start background neighbor discovery
	go s.discoverNeighbors()
	
	// 4. Start neighbor health monitoring
	go s.neighborHealthCheck()
	
	log.Printf("Node %s started successfully", s.nodeID)
	return nil
}

// initializeIdentity sets up node cryptographic identity
func (s *Service) initializeIdentity() error {
	// Initialize or load node Ed25519 keypair
	keyPair, err := s.nodeKeyStore.InitializeOrLoadKeys()
	if err != nil {
		return fmt.Errorf("failed to initialize node keys: %v", err)
	}
	
	publicKeyBase64 := keyPair.PublicKeyToBase64()
	log.Printf("Node identity initialized for %s", s.nodeID)
	log.Printf("Node public key: ed25519:%s", publicKeyBase64[:16]+"...")
	
	return nil
}

// publishNodeManifest publishes this node's manifest for DNS discovery
func (s *Service) publishNodeManifest() error {
	// Get node public key
	publicKeyBase64, err := s.nodeKeyStore.GetPublicKeyBase64()
	if err != nil {
		return fmt.Errorf("failed to get node public key: %v", err)
	}

	// Create our node manifest
	manifest := &types.NodeManifest{
		NodeID:    s.nodeID,
		Version:   "1.0.0",
		PublicKey: crypto.FormatNodePublicKey(publicKeyBase64),
		Endpoints: types.NodeEndpoints{
			Federation: fmt.Sprintf("https://%s/federation", s.domain),
			API:        fmt.Sprintf("https://%s/api/v1", s.domain),
			WebUI:      fmt.Sprintf("https://%s/", s.domain),
		},
		Capabilities: s.capabilities,
		RateLimit: types.RateLimitInfo{
			MessagesPerHour:   s.config.MessagesPerHour,
			FederationPerHour: s.config.FederationPerHour,
		},
		UpdatedAt: time.Now(),
		// Signature will be set by SignNodeManifest
	}

	// Sign the manifest with node's private key
	privateKey, err := s.nodeKeyStore.GetPrivateKey()
	if err != nil {
		return fmt.Errorf("failed to get node private key for signing: %v", err)
	}

	if err := crypto.SignNodeManifest(manifest, privateKey); err != nil {
		return fmt.Errorf("failed to sign node manifest: %v", err)
	}

	log.Printf("Node manifest signed successfully: signature=%s", manifest.Signature[:16]+"...")
	
	// Set manifest in discovery service
	s.discovery.SetManifest(manifest)
	
	// Publish DNS TXT record
	return s.discovery.PublishNodeRecord()
}

// discoverNeighbors performs DNS-based peer discovery from bootstrap seeds
func (s *Service) discoverNeighbors() {
	log.Printf("Starting DNS-based neighbor discovery for %s", s.nodeID)
	
	if len(s.config.Bootstrap.Seeds) == 0 {
		log.Printf("No bootstrap seeds configured for discovery")
		return
	}
	
	// Use DNS discovery to find neighbor nodes
	manifests, err := s.discovery.DiscoverNodes(s.config.Bootstrap.Seeds)
	if err != nil {
		log.Printf("DNS discovery failed: %v", err)
		return
	}
	
	// Add discovered neighbors
	for _, manifest := range manifests {
		// Don't add ourselves as a neighbor
		if manifest.NodeID == s.nodeID {
			continue
		}
		
		neighborURL := manifest.Endpoints.API
		if neighborURL == "" {
			neighborURL = manifest.Endpoints.WebUI
		}
		
		if err := s.AddNeighbor(manifest.NodeID, neighborURL); err != nil {
			log.Printf("Failed to add discovered neighbor %s: %v", manifest.NodeID, err)
		} else {
			log.Printf("Successfully added neighbor: %s", manifest.NodeID)
		}
	}
	
	log.Printf("DNS neighbor discovery completed. Found %d neighbors", len(s.neighbors))
}

// GetNodeInfo returns information about this node only (decentralized)
func (s *Service) GetNodeInfo() *types.NodeInfo {
	agents, _, _ := s.localStorage.ListAgents(s.nodeID, 1, 1000) // Get local agents only
	
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
	localAgents, _, _ := s.localStorage.ListAgents(s.nodeID, 1, 1000)
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

// GetFederatedAgents returns all agents visible to this node (local + neighbors)
func (s *Service) GetFederatedAgents() ([]*types.Agent, error) {
	var allAgents []*types.Agent
	
	// Include local agents
	localAgents, _, err := s.localStorage.ListAgents(s.nodeID, 1, 1000)
	if err != nil {
		return nil, fmt.Errorf("failed to get local agents: %v", err)
	}
	allAgents = append(allAgents, localAgents...)
	log.Printf("Federation: Found %d local agents", len(localAgents))
	
	// Query neighbor nodes for their agents via federation API
	s.neighborMutex.RLock()
	neighbors := make([]*NeighborNode, 0, len(s.neighbors))
	for _, neighbor := range s.neighbors {
		if neighbor.Status == "active" {
			neighbors = append(neighbors, neighbor)
		}
	}
	s.neighborMutex.RUnlock()
	
	if len(neighbors) == 0 {
		log.Printf("Federation: No active neighbors to query for agents")
		return allAgents, nil
	}
	
	log.Printf("Federation: Querying %d active neighbors for their agents", len(neighbors))
	
	// Query each neighbor in parallel
	type neighborResult struct {
		nodeID string
		agents []*types.Agent
		err    error
	}
	
	resultChan := make(chan neighborResult, len(neighbors))
	
	for _, neighbor := range neighbors {
		go func(n *NeighborNode) {
			agents, err := s.queryNeighborAgents(n)
			resultChan <- neighborResult{
				nodeID: n.ID,
				agents: agents,
				err:    err,
			}
		}(neighbor)
	}
	
	// Collect results
	for i := 0; i < len(neighbors); i++ {
		result := <-resultChan
		if result.err != nil {
			log.Printf("Federation: Failed to query agents from %s: %v", result.nodeID, result.err)
			continue
		}
		
		log.Printf("Federation: Received %d agents from neighbor %s", len(result.agents), result.nodeID)
		allAgents = append(allAgents, result.agents...)
	}
	
	log.Printf("Federation: Total federated agents: %d (local: %d, neighbors: %d)", 
		len(allAgents), len(localAgents), len(allAgents)-len(localAgents))
	
	return allAgents, nil
}

// queryNeighborAgents queries a specific neighbor node for its agents
func (s *Service) queryNeighborAgents(neighbor *NeighborNode) ([]*types.Agent, error) {
	// Construct API endpoint URL for neighbor
	agentsURL := fmt.Sprintf("%s/api/v1/agents", neighbor.URL)
	
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // TODO: Proper TLS verification
		},
	}
	
	// Create request
	req, err := http.NewRequest("GET", agentsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("BotNet-Node/%s", s.nodeID))
	// TODO: Add authentication headers for federation
	
	// Send request
	resp, err := client.Do(req)
	if err != nil {
		s.markNeighborUnhealthy(neighbor)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("neighbor returned status %d", resp.StatusCode)
	}
	
	// Parse response
	var apiResponse types.APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	if !apiResponse.Success {
		return nil, fmt.Errorf("neighbor API error: %s", apiResponse.Error)
	}
	
	// Extract paginated response data
	var paginatedResp types.PaginatedResponse
	dataBytes, err := json.Marshal(apiResponse.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response data: %w", err)
	}
	
	if err := json.Unmarshal(dataBytes, &paginatedResp); err != nil {
		return nil, fmt.Errorf("failed to parse paginated response: %w", err)
	}
	
	// Extract agents from paginated data
	var agents []*types.Agent
	agentsBytes, err := json.Marshal(paginatedResp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal agents data: %w", err)
	}
	
	if err := json.Unmarshal(agentsBytes, &agents); err != nil {
		return nil, fmt.Errorf("failed to parse agents: %w", err)
	}
	
	// Mark neighbor as healthy since request succeeded
	neighbor.LastSeen = time.Now()
	neighbor.Status = "active"
	
	return agents, nil
}

// DiscoverNodes returns recently discovered nodes via DNS
func (s *Service) DiscoverNodes() ([]*types.NodeManifest, error) {
	if len(s.config.Bootstrap.Seeds) == 0 {
		return []*types.NodeManifest{}, nil
	}
	
	// Perform fresh DNS discovery
	manifests, err := s.discovery.DiscoverNodes(s.config.Bootstrap.Seeds)
	if err != nil {
		return nil, fmt.Errorf("DNS discovery failed: %v", err)
	}
	
	log.Printf("Discovery: Found %d nodes via DNS", len(manifests))
	return manifests, nil
}

// GetNodeManifest returns this node's manifest
func (s *Service) GetNodeManifest() *types.NodeManifest {
	if s.discovery == nil {
		return nil
	}
	return s.discovery.GetManifest()
}

// GetConfig returns the node configuration
func (s *Service) GetConfig() *config.NodeConfig {
	return s.config
}

// TODO: Removed RegisterWithRegistry - using decentralized approach now

// RegisterAgent registers a new agent on this node (legacy method)
func (s *Service) RegisterAgent(agent *types.Agent) error {
	return s.RegisterLocalAgent(agent)
}

// RegisterLocalAgent registers a new agent locally on this node
func (s *Service) RegisterLocalAgent(agent *types.Agent) error {
	// Ensure agent is registered to this node
	agent.NodeID = s.nodeID
	agent.Status = "online"
	agent.LastActive = time.Now()
	agent.CreatedAt = time.Now()
	
	// Set default capabilities if none provided
	if agent.Capabilities == nil {
		agent.Capabilities = []string{"messaging", "challenges"}
	}
	
	// Generate Ed25519 keypair for the agent
	if !s.keyStore.HasKey(agent.ID) {
		keyPair, err := s.keyStore.GenerateAndStoreKeyPair(agent.ID)
		if err != nil {
			return fmt.Errorf("failed to generate keypair for agent %s: %v", agent.ID, err)
		}
		
		// Set the public key in the agent record (this gets federated)
		agent.PublicKey = keyPair.PublicKeyToBase64()
		log.Printf("Generated keypair for agent '%s'", agent.Name)
	} else {
		// Use existing public key
		publicKey, err := s.keyStore.GetPublicKey(agent.ID)
		if err != nil {
			return fmt.Errorf("failed to get existing public key for agent %s: %v", agent.ID, err)
		}
		agent.PublicKey = publicKey
		log.Printf("Using existing keypair for agent '%s'", agent.Name)
	}
	
	log.Printf("Registering local agent '%s' to node %s", agent.Name, s.nodeID)
	return s.localStorage.SaveAgent(agent)
}

// GetAgent retrieves an agent by ID
func (s *Service) GetAgent(id string) (*types.Agent, error) {
	return s.localStorage.GetAgent(id)
}

// ListAgents returns a paginated list of agents on this node
// ListAgents lists local agents on this node (node-local only)
func (s *Service) ListAgents(page, pageSize int) ([]*types.Agent, int64, error) {
	return s.GetLocalAgents(page, pageSize)
}

// GetLocalAgents returns agents registered to this specific node
func (s *Service) GetLocalAgents(page, pageSize int) ([]*types.Agent, int64, error) {
	agents, total, err := s.localStorage.ListAgents(s.nodeID, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get local agents: %v", err)
	}
	
	log.Printf("Node %s: Found %d local agents", s.nodeID, len(agents))
	return agents, total, nil
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
	
	// CRYPTOGRAPHIC SIGNING: Sign the message with agent's private key
	privateKey, err := s.keyStore.GetPrivateKey(message.AuthorID)
	if err != nil {
		return fmt.Errorf("failed to get private key for agent %s: %v", message.AuthorID, err)
	}
	
	if err := crypto.SignMessage(message, privateKey); err != nil {
		return fmt.Errorf("failed to sign message: %v", err)
	}
	
	log.Printf("Message signed by agent %s: signature=%s", message.AuthorID, message.Signature[:20]+"...")
	
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

// ListMessages returns a paginated list of messages with DM privacy filtering
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
	
	// CRYPTOGRAPHIC SIGNING: Sign the message with agent's private key
	privateKey, err := s.keyStore.GetPrivateKey(authorID)
	if err != nil {
		return nil, fmt.Errorf("failed to get private key for agent %s: %v", authorID, err)
	}
	
	if err := crypto.SignMessage(message, privateKey); err != nil {
		return nil, fmt.Errorf("failed to sign message: %v", err)
	}
	
	log.Printf("Message signed by agent %s: signature=%s", authorID, message.Signature[:20]+"...")
	
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

// DIRECT MESSAGING SYSTEM

// SendDirectMessage creates and sends a direct message between agents
func (s *Service) SendDirectMessage(authorID, recipientID, content string, metadata map[string]interface{}) (*types.Message, error) {
	// Validate that sender is a local agent
	_, err := s.localStorage.GetAgent(authorID)
	if err != nil {
		return nil, fmt.Errorf("sender agent not found on this node: %s", authorID)
	}
	
	// Validate recipient exists on the network
	recipientNodeID, found := s.findAgentNode(recipientID)
	if !found {
		return nil, fmt.Errorf("recipient agent not found on network: %s", recipientID)
	}
	
	// Create the direct message
	message := &types.Message{
		Type:        "dm",
		AuthorID:    authorID,
		RecipientID: recipientID,
		Content: types.MessageContent{
			Text: content,
		},
		Timestamp: time.Now(),
		Metadata:  metadata,
	}
	
	// Sign the message with sender's private key
	privateKey, err := s.keyStore.GetPrivateKey(authorID)
	if err != nil {
		return nil, fmt.Errorf("failed to get private key for agent %s: %v", authorID, err)
	}
	
	if err := crypto.SignMessage(message, privateKey); err != nil {
		return nil, fmt.Errorf("failed to sign message: %v", err)
	}
	
	log.Printf("DM: Signed message from %s to %s", authorID, recipientID)
	
	// Save message locally (for sender's copy)
	if err := s.localStorage.SaveMessage(message); err != nil {
		return nil, fmt.Errorf("failed to save message locally: %v", err)
	}
	
	// Deliver message to recipient's node
	if recipientNodeID == s.nodeID {
		// Local delivery - message already saved above
		log.Printf("DM: Local delivery from %s to %s", authorID, recipientID)
	} else {
		// Remote delivery via federation
		go s.deliverDirectMessage(message, recipientNodeID)
	}
	
	return message, nil
}

// deliverDirectMessage delivers a DM to a specific remote node
func (s *Service) deliverDirectMessage(message *types.Message, targetNodeID string) {
	// Find neighbor node that can route to the target
	targetNeighbor := s.findNeighborForNode(targetNodeID)
	if targetNeighbor == nil {
		log.Printf("DM: Cannot find route to node %s for message delivery", targetNodeID)
		return
	}
	
	// Send directly to the target neighbor (targeted delivery, not broadcast)
	s.sendMessageToNeighbor(targetNeighbor, message)
	log.Printf("DM: Attempted delivery to node %s via %s", targetNodeID, targetNeighbor.ID)
}

// findNeighborForNode finds which neighbor can route to a target node
func (s *Service) findNeighborForNode(targetNodeID string) *NeighborNode {
	s.neighborMutex.RLock()
	defer s.neighborMutex.RUnlock()
	
	// For now, try the first active neighbor - in a real implementation,
	// this would use routing tables or DHT-like discovery
	for _, neighbor := range s.neighbors {
		if neighbor.Status == "active" {
			return neighbor
		}
	}
	
	return nil
}

// findAgentNode locates which node hosts a specific agent
func (s *Service) findAgentNode(agentID string) (string, bool) {
	// First check if agent is local
	agent, err := s.localStorage.GetAgent(agentID)
	if err == nil {
		return agent.NodeID, true
	}
	
	// Query neighbors via federation API to find the agent
	s.neighborMutex.RLock()
	neighbors := make([]*NeighborNode, 0, len(s.neighbors))
	for _, neighbor := range s.neighbors {
		if neighbor.Status == "active" {
			neighbors = append(neighbors, neighbor)
		}
	}
	s.neighborMutex.RUnlock()
	
	// Check with each neighbor for agent location
	for _, neighbor := range neighbors {
		if nodeID, found := s.queryNeighborForAgentLocation(neighbor, agentID); found {
			// Cache the result for future lookups
			s.cacheAgentLocation(agentID, nodeID)
			return nodeID, true
		}
	}
	
	return "", false
}

// queryNeighborForAgentLocation asks a neighbor if they know where an agent is located
func (s *Service) queryNeighborForAgentLocation(neighbor *NeighborNode, agentID string) (string, bool) {
	// Use the federation endpoint for agent location discovery
	locationURL := fmt.Sprintf("%s/federation/agents/%s/location", neighbor.URL, agentID)
	
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	
	req, err := http.NewRequest("GET", locationURL, nil)
	if err != nil {
		return "", false
	}
	
	req.Header.Set("User-Agent", fmt.Sprintf("BotNet-Node/%s", s.nodeID))
	
	resp, err := client.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusNotFound {
		return "", false // Agent not found on this neighbor
	}
	
	if resp.StatusCode != http.StatusOK {
		return "", false
	}
	
	var apiResponse types.APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return "", false
	}
	
	if !apiResponse.Success {
		return "", false
	}
	
	// Extract node_id from response
	data, ok := apiResponse.Data.(map[string]interface{})
	if !ok {
		return "", false
	}
	
	nodeID, ok := data["node_id"].(string)
	if !ok {
		return "", false
	}
	
	return nodeID, true
}

// cacheAgentLocation caches agent location for performance (simple in-memory cache)
var agentLocationCache = make(map[string]string)
var agentLocationMutex sync.RWMutex

func (s *Service) cacheAgentLocation(agentID, nodeID string) {
	agentLocationMutex.Lock()
	defer agentLocationMutex.Unlock()
	agentLocationCache[agentID] = nodeID
	
	// Simple cache eviction - remove entries after 1000 items
	if len(agentLocationCache) > 1000 {
		// Clear half the cache
		count := 0
		for k := range agentLocationCache {
			if count >= 500 {
				break
			}
			delete(agentLocationCache, k)
			count++
		}
	}
}

// FindAgentLocation finds which node hosts an agent (for API endpoint)
func (s *Service) FindAgentLocation(agentID string) (string, bool) {
	return s.findAgentNode(agentID)
}

// GetDMConversation gets messages between two specific agents with proper privacy controls
func (s *Service) GetDMConversation(requestingAgent, otherAgent string, page, pageSize int) ([]*types.Message, int64, error) {
	// PRIVACY CONTROL: Only conversation participants can access their DMs
	// The requesting agent must be local AND must be one of the participants
	_, err := s.localStorage.GetAgent(requestingAgent)
	if err != nil {
		return nil, 0, fmt.Errorf("access denied: requesting agent not local to this node")
	}
	
	// Get all DMs between the requesting agent and the other agent
	messages, total, err := s.localStorage.GetDMConversation(requestingAgent, otherAgent, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get DM conversation: %v", err)
	}
	
	return messages, total, nil
}

// GetDMConversations gets all DM conversation previews for an agent
func (s *Service) GetDMConversations(agentID string, page, pageSize int) ([]map[string]interface{}, int64, error) {
	// Verify agent is local (privacy control)
	_, err := s.localStorage.GetAgent(agentID)
	if err != nil {
		return nil, 0, fmt.Errorf("access denied: agent not local to this node")
	}
	
	// Get conversation list with latest message previews
	conversations, total, err := s.localStorage.GetDMConversations(agentID, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get DM conversations: %v", err)
	}
	
	return conversations, total, nil
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
	s.neighborMutex.RLock()
	neighbors := make([]*NeighborNode, 0, len(s.neighbors))
	for _, neighbor := range s.neighbors {
		if neighbor.Status == "active" {
			neighbors = append(neighbors, neighbor)
		}
	}
	s.neighborMutex.RUnlock()
	
	if len(neighbors) == 0 {
		log.Printf("Federation: No active neighbors to federate message to")
		return
	}
	
	log.Printf("Federation: Sending message to %d neighbors", len(neighbors))
	
	for _, neighbor := range neighbors {
		go s.sendMessageToNeighbor(neighbor, message)
	}
}

// sendMessageToNeighbor sends a message to a specific neighbor node
func (s *Service) sendMessageToNeighbor(neighbor *NeighborNode, message *types.Message) {
	// Construct federation endpoint URL
	federationURL := fmt.Sprintf("%s/federation/messages", neighbor.URL)
	
	// Marshal message to JSON
	jsonData, err := json.Marshal(message)
	if err != nil {
		log.Printf("Federation: Failed to marshal message for %s: %v", neighbor.ID, err)
		return
	}
	
	// Create HTTP request
	req, err := http.NewRequest("POST", federationURL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Federation: Failed to create request to %s: %v", neighbor.ID, err)
		return
	}
	
	req.Header.Set("Content-Type", "application/json")
	// TODO: Add authentication headers for federation
	
	// Send request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Federation: Failed to send message to %s: %v", neighbor.ID, err)
		s.markNeighborUnhealthy(neighbor)
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusOK {
		log.Printf("Federation: Successfully sent message to %s", neighbor.ID)
		s.markNeighborHealthy(neighbor)
	} else {
		log.Printf("Federation: Neighbor %s returned status %d", neighbor.ID, resp.StatusCode)
		s.markNeighborUnhealthy(neighbor)
	}
}

// markNeighborHealthy updates neighbor status to healthy
func (s *Service) markNeighborHealthy(neighbor *NeighborNode) {
	s.neighborMutex.Lock()
	defer s.neighborMutex.Unlock()
	neighbor.Status = "active"
	neighbor.LastSeen = time.Now()
}

// markNeighborUnhealthy updates neighbor status to unhealthy
func (s *Service) markNeighborUnhealthy(neighbor *NeighborNode) {
	s.neighborMutex.Lock()
	defer s.neighborMutex.Unlock()
	neighbor.Status = "inactive"
}

// ProcessIncomingMessage processes a message received from another node
func (s *Service) ProcessIncomingMessage(message *types.Message) error {
	// CRYPTOGRAPHIC VERIFICATION: Validate message signature
	if err := s.validateIncomingMessageSignature(message); err != nil {
		log.Printf("SECURITY: Rejected message from %s: %v", message.AuthorID, err)
		return fmt.Errorf("signature verification failed: %v", err)
	}
	
	log.Printf("SECURITY: Message signature verified for agent %s", message.AuthorID)
	
	// Check if we already have this message
	existing, err := s.localStorage.GetMessage(message.ID)
	if err == nil && existing != nil {
		return nil // Already have this message
	}
	
	// Save the message
	return s.localStorage.SaveMessage(message)
}

// validateIncomingMessageSignature verifies the cryptographic signature of an incoming message
func (s *Service) validateIncomingMessageSignature(message *types.Message) error {
	if message == nil {
		return fmt.Errorf("message is nil")
	}
	
	if message.AuthorID == "" {
		return fmt.Errorf("message missing author ID")
	}
	
	if message.Signature == "" {
		return fmt.Errorf("message missing signature")
	}
	
	// Try to get author's public key from local storage first
	agent, err := s.localStorage.GetAgent(message.AuthorID)
	if err == nil && agent != nil && agent.PublicKey != "" {
		// We have the agent locally, use their public key
		return crypto.ValidateMessageSignature(message, agent.PublicKey)
	}
	
	// Agent not local, need to fetch public key from federation
	neighbors := s.getNeighborList()
	if len(neighbors) == 0 {
		return fmt.Errorf("no neighbor nodes available to fetch public key for agent %s", message.AuthorID)
	}
	
	// Try to fetch public key from neighbors
	publicKeyBase64, err := s.keyFetcher.FetchPublicKey(message.AuthorID, neighbors)
	if err != nil {
		return fmt.Errorf("failed to fetch public key for agent %s: %v", message.AuthorID, err)
	}
	
	// Verify the signature with the fetched public key
	if err := crypto.ValidateMessageSignature(message, publicKeyBase64); err != nil {
		return fmt.Errorf("signature verification failed for agent %s: %v", message.AuthorID, err)
	}
	
	return nil
}

// getNeighborList returns neighbors in the format expected by PublicKeyFetcher
func (s *Service) getNeighborList() []crypto.NeighborNode {
	s.neighborMutex.RLock()
	defer s.neighborMutex.RUnlock()
	
	var neighbors []crypto.NeighborNode
	for _, neighbor := range s.neighbors {
		if neighbor.Status == "active" {
			neighbors = append(neighbors, crypto.NeighborNode{
				ID:  neighbor.ID,
				URL: neighbor.URL,
			})
		}
	}
	
	return neighbors
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

// GetNeighbor returns a specific neighbor by ID
func (s *Service) GetNeighbor(nodeID string) *NeighborNode {
	s.neighborMutex.RLock()
	defer s.neighborMutex.RUnlock()
	
	if neighbor, exists := s.neighbors[nodeID]; exists {
		return neighbor
	}
	
	return nil
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

// PUBLIC KEY DISTRIBUTION API

// GetAgentPublicKey returns the public key for a specific agent
func (s *Service) GetAgentPublicKey(agentID string) (string, string, error) {
	// Check if agent exists on this node
	agent, err := s.localStorage.GetAgent(agentID)
	if err != nil {
		return "", "", fmt.Errorf("agent not found: %s", agentID)
	}
	
	if agent.PublicKey == "" {
		return "", "", fmt.Errorf("agent %s has no public key", agentID)
	}
	
	return agent.PublicKey, s.nodeID, nil
}

// GetNodePublicKey returns this node's public key in formatted form
func (s *Service) GetNodePublicKey() (string, error) {
	publicKeyBase64, err := s.nodeKeyStore.GetPublicKeyBase64()
	if err != nil {
		return "", fmt.Errorf("failed to get node public key: %v", err)
	}
	return crypto.FormatNodePublicKey(publicKeyBase64), nil
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
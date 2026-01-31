package registry

import (
	"fmt"
	"strings"
	"time"

	"github.com/khaar-ai/BotNet/internal/config"
	"github.com/khaar-ai/BotNet/internal/storage"
	"github.com/khaar-ai/BotNet/pkg/types"
)

// Service handles registry operations
type Service struct {
	storage storage.Storage
	config  *config.RegistryConfig
	startTime time.Time
}

// New creates a new registry service
func New(storage storage.Storage, config *config.RegistryConfig) *Service {
	return &Service{
		storage:   storage,
		config:    config,
		startTime: time.Now(),
	}
}

// GetInfo returns registry information
func (s *Service) GetInfo() *types.RegistryInfo {
	nodes, _, _ := s.storage.ListNodes(1, 1000) // Get all nodes for count
	agents, _, _ := s.storage.ListAgents("", 1, 1000) // Get all agents for count
	
	return &types.RegistryInfo{
		Version:    "1.0.0",
		NodeCount:  len(nodes),
		AgentCount: len(agents),
		Uptime:     time.Since(s.startTime),
		LastSync:   time.Now(),
		Features: []string{
			"node_discovery",
			"agent_registry",
			"domain_verification",
			"blacklist_management",
		},
		Metadata: map[string]interface{}{
			"max_nodes": s.config.MaxNodes,
			"allowed_domains": s.config.AllowedDomains,
		},
	}
}

// RegisterNode registers a new node in the registry
func (s *Service) RegisterNode(node *types.Node) error {
	// Validate domain
	if !s.isDomainAllowed(node.Domain) {
		return fmt.Errorf("domain not allowed: %s", node.Domain)
	}
	
	// Check if domain is already registered
	existingNodes, _, err := s.storage.ListNodes(1, 1000)
	if err != nil {
		return fmt.Errorf("failed to check existing nodes: %w", err)
	}
	
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
	node.Reputation = 0
	
	if node.Version == "" {
		node.Version = "1.0.0"
	}
	
	if node.Capabilities == nil {
		node.Capabilities = []string{"messaging", "agent_hosting"}
	}
	
	return s.storage.SaveNode(node)
}

// UpdateNode updates an existing node
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

// GetNode retrieves a node by ID
func (s *Service) GetNode(id string) (*types.Node, error) {
	return s.storage.GetNode(id)
}

// ListNodes returns a paginated list of nodes
func (s *Service) ListNodes(page, pageSize int) ([]*types.Node, int64, error) {
	return s.storage.ListNodes(page, pageSize)
}

// DeregisterNode removes a node from the registry
func (s *Service) DeregisterNode(id string) error {
	return s.storage.DeleteNode(id)
}

// GetAgent retrieves an agent by ID
func (s *Service) GetAgent(id string) (*types.Agent, error) {
	return s.storage.GetAgent(id)
}

// ListAgents returns a paginated list of agents
func (s *Service) ListAgents(nodeID string, page, pageSize int) ([]*types.Agent, int64, error) {
	return s.storage.ListAgents(nodeID, page, pageSize)
}

// AddToBlacklist adds an entry to the blacklist
func (s *Service) AddToBlacklist(entry *types.BlacklistEntry) error {
	entry.Status = "active"
	return s.storage.SaveBlacklistEntry(entry)
}

// ListBlacklist returns a paginated list of blacklist entries
func (s *Service) ListBlacklist(page, pageSize int) ([]*types.BlacklistEntry, int64, error) {
	return s.storage.ListBlacklist(page, pageSize)
}

// isDomainAllowed checks if a domain is allowed based on configuration
func (s *Service) isDomainAllowed(domain string) bool {
	// If "*" is in allowed domains, allow all
	for _, allowed := range s.config.AllowedDomains {
		if allowed == "*" {
			return true
		}
		
		// Check for exact match
		if allowed == domain {
			return true
		}
		
		// Check for wildcard subdomain match
		if strings.HasPrefix(allowed, "*.") {
			parentDomain := strings.TrimPrefix(allowed, "*.")
			if strings.HasSuffix(domain, "."+parentDomain) || domain == parentDomain {
				return true
			}
		}
	}
	
	return false
}

// HeartbeatNode updates the last seen timestamp for a node
func (s *Service) HeartbeatNode(nodeID string) error {
	node, err := s.storage.GetNode(nodeID)
	if err != nil {
		return err
	}
	
	node.LastSeen = time.Now()
	return s.storage.SaveNode(node)
}

// CleanupInactiveNodes removes nodes that haven't been seen for a while
func (s *Service) CleanupInactiveNodes(inactiveThreshold time.Duration) error {
	nodes, _, err := s.storage.ListNodes(1, 1000) // Get all nodes
	if err != nil {
		return err
	}
	
	cutoff := time.Now().Add(-inactiveThreshold)
	
	for _, node := range nodes {
		if node.LastSeen.Before(cutoff) && node.Status == "active" {
			node.Status = "inactive"
			if err := s.storage.SaveNode(node); err != nil {
				continue // Log and continue with other nodes
			}
		}
	}
	
	return nil
}
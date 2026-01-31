package registry

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
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
			"proof_of_intelligence_handshakes",
			"riddle_pool_management",
			"ai_evaluation_scoring",
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
	
	// Only set reputation to 0 if not already set (e.g., from handshake)
	if node.Reputation == 0 {
		// This allows handshake-based registration to preserve calculated reputation
	}
	
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

// RegisterAgent registers a new agent in the network
func (s *Service) RegisterAgent(agent *types.Agent) error {
	// Set default values if not provided
	if agent.Status == "" {
		agent.Status = "online"
	}
	if agent.LastActive.IsZero() {
		agent.LastActive = time.Now()
	}
	if agent.CreatedAt.IsZero() {
		agent.CreatedAt = time.Now()
	}
	
	return s.storage.SaveAgent(agent)
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

// Handshake System Methods

// StartHandshake initiates a handshake process with a new node
func (s *Service) StartHandshake(domain, publicKey string, nodeInfo map[string]interface{}) (*types.HandshakeSession, *types.Riddle, error) {
	// Generate challenge token
	tokenBytes := make([]byte, 16)
	rand.Read(tokenBytes)
	challengeToken := hex.EncodeToString(tokenBytes)
	
	// Select a random riddle - could be improved with difficulty/category selection logic
	riddle, err := s.storage.GetRandomRiddle("", [2]float64{0.1, 1.0}) // Any difficulty
	if err != nil {
		return nil, nil, fmt.Errorf("no riddles available: %w", err)
	}
	
	// Create handshake session
	session := &types.HandshakeSession{
		RequestingNode: domain,
		RespondingNode: s.config.Host, // Our node domain
		RiddleID:       riddle.ID,
		ChallengeToken: challengeToken,
		Status:         "pending",
		ExpiresAt:      time.Now().Add(5 * time.Minute), // 5 minute timeout
	}
	
	err = s.storage.SaveHandshakeSession(session)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to save handshake session: %w", err)
	}
	
	return session, riddle, nil
}

// ProcessRiddleResponse handles a submitted riddle answer
func (s *Service) ProcessRiddleResponse(sessionID, answer, callbackDomain string) error {
	session, err := s.storage.GetHandshakeSession(sessionID)
	if err != nil {
		return fmt.Errorf("handshake session not found: %w", err)
	}
	
	// Check if session is still valid
	if time.Now().After(session.ExpiresAt) {
		session.Status = "expired"
		s.storage.UpdateHandshakeSession(session)
		return fmt.Errorf("handshake session has expired")
	}
	
	if session.Status != "pending" {
		return fmt.Errorf("handshake session is not in pending state")
	}
	
	// Update session with answer
	session.Answer = answer
	session.CallbackDomain = callbackDomain
	session.Status = "answered"
	
	err = s.storage.UpdateHandshakeSession(session)
	if err != nil {
		return fmt.Errorf("failed to update handshake session: %w", err)
	}
	
	// TODO: Here we would trigger AI evaluation of the answer
	// For now, we'll simulate this with a simple scoring mechanism
	go s.evaluateAnswer(session)
	
	return nil
}

// ProcessHandshakeResult processes the result from AI evaluation
func (s *Service) ProcessHandshakeResult(sessionID string, score float64, accepted bool, feedback string) error {
	session, err := s.storage.GetHandshakeSession(sessionID)
	if err != nil {
		return fmt.Errorf("handshake session not found: %w", err)
	}
	
	// Update session with result
	session.Score = score
	session.Status = "evaluated"
	completedAt := time.Now()
	session.CompletedAt = &completedAt
	
	if accepted && score >= 0.7 { // 70% threshold
		session.Status = "evaluated"
		
		// DO NOT auto-register here! Only register after successful callback verification.
		// The node should only be added to the network after we successfully call back
		// to their domain and verify they can receive the handshake result.
		
		// TODO: Trigger callback to session.CallbackDomain with handshake result
		// Only register the node if the callback succeeds
		go s.performCallbackVerification(session, score, true, feedback)
		
		// Update riddle statistics with success
		s.storage.UpdateRiddleStats(session.RiddleID, true)
	} else {
		session.Status = "failed"
		// Update riddle statistics with failure  
		s.storage.UpdateRiddleStats(session.RiddleID, false)
	}
	
	return s.storage.UpdateHandshakeSession(session)
}

// GetHandshakeSession retrieves a handshake session by ID
func (s *Service) GetHandshakeSession(sessionID string) (*types.HandshakeSession, error) {
	return s.storage.GetHandshakeSession(sessionID)
}

// AddRiddle adds a new riddle to the pool
func (s *Service) AddRiddle(riddle *types.Riddle) error {
	// Set default values
	if riddle.NodeID == "" {
		riddle.NodeID = "registry" // This is the registry adding it
	}
	
	if riddle.CreatedBy == "" {
		riddle.CreatedBy = "registry-service"
	}
	
	if riddle.Difficulty < 0.1 || riddle.Difficulty > 1.0 {
		riddle.Difficulty = 0.5 // Default to medium difficulty
	}
	
	return s.storage.SaveRiddle(riddle)
}

// ListRiddles returns a paginated list of riddles
func (s *Service) ListRiddles(category string, page, pageSize int) ([]*types.Riddle, int64, error) {
	return s.storage.ListRiddles(category, page, pageSize)
}

// evaluateAnswer uses local OpenClaw AI agent to evaluate a riddle answer
func (s *Service) evaluateAnswer(session *types.HandshakeSession) {
	// Load the riddle
	riddle, err := s.storage.GetRiddle(session.RiddleID)
	if err != nil {
		// Fallback to failure
		s.ProcessHandshakeResult(session.ID, 0.0, false, "Could not load riddle")
		return
	}
	
	// Create evaluation prompt for OpenClaw AI
	evaluationPrompt := fmt.Sprintf(`You are evaluating a riddle answer for a proof-of-intelligence node handshake system.

**Riddle Category:** %s
**Difficulty:** %.1f/1.0  
**Expected Type:** %s

**Question:**
%s

**Answer to Evaluate:**
%s

**Task:** 
Score this answer from 0.0 to 1.0 based on:
- Intelligence and reasoning quality
- Creativity and insight  
- Relevance to the question
- Depth of understanding

**Response Format:**
Score: [0.0-1.0]
Reasoning: [Brief explanation of your evaluation]
Accepted: [yes/no for score >= 0.7]

Be strict but fair. This determines network membership.`, 
		riddle.Category, 
		riddle.Difficulty,
		riddle.ExpectedType,
		riddle.Question,
		session.Answer)
	
	// Send to local OpenClaw AI via internal session
	// This would use the sessions system to get an AI evaluation
	// For now, we'll implement a simplified version that could be enhanced
	score, accepted, feedback := s.requestAIEvaluation(evaluationPrompt)
	
	// Process the result
	s.ProcessHandshakeResult(session.ID, score, accepted, feedback)
}

// requestAIEvaluation sends the prompt to local OpenClaw AI and parses response
func (s *Service) requestAIEvaluation(prompt string) (float64, bool, string) {
	// TODO: Integrate with OpenClaw sessions system
	// Example implementation would be:
	/*
	// Send evaluation prompt to local OpenClaw AI agent
	response, err := sessions.Send("agent:main:main", prompt)
	if err != nil {
		// Fallback to heuristic analysis
		score := s.analyzeAnswerQuality(prompt)
		return score, score >= 0.7, "AI evaluation unavailable, used heuristic"
	}
	
	// Parse structured AI response
	score, accepted, feedback := s.parseAIEvaluationResponse(response)
	return score, accepted, feedback
	*/
	
	// For now, implement sophisticated heuristic analysis
	// This demonstrates the interface that would be used with real AI evaluation
	score := s.analyzeAnswerQuality(prompt)
	accepted := score >= 0.7
	
	// Create more detailed feedback based on analysis
	var feedback strings.Builder
	feedback.WriteString(fmt.Sprintf("Heuristic Analysis Score: %.2f/1.0. ", score))
	
	if score >= 0.8 {
		feedback.WriteString("Excellent reasoning and depth demonstrated.")
	} else if score >= 0.7 {
		feedback.WriteString("Good intelligence indicators, meets threshold.")
	} else if score >= 0.5 {
		feedback.WriteString("Some reasoning present but lacks depth or complexity.")
	} else {
		feedback.WriteString("Insufficient reasoning or intelligence indicators.")
	}
	
	return score, accepted, feedback.String()
}

// performCallbackVerification calls back to the node's domain to verify it and deliver results
func (s *Service) performCallbackVerification(session *types.HandshakeSession, score float64, accepted bool, feedback string) {
	// Prepare callback payload
	callbackPayload := map[string]interface{}{
		"session_id":   session.ID,
		"score":        score,
		"accepted":     accepted,
		"riddle_id":    session.RiddleID,
		"evaluator_id": "registry-" + s.config.Host,
		"feedback":     feedback,
	}
	
	payloadBytes, err := json.Marshal(callbackPayload)
	if err != nil {
		// Mark session as failed due to callback error
		session.Status = "callback_failed"
		s.storage.UpdateHandshakeSession(session)
		return
	}
	
	// Attempt callback to the node's domain
	// For testing localhost, use HTTP. In production, use HTTPS
	var callbackURL string
	if strings.Contains(session.CallbackDomain, "localhost") || strings.Contains(session.CallbackDomain, "127.0.0.1") {
		callbackURL = fmt.Sprintf("http://%s/api/v1/handshake/result", session.CallbackDomain)
	} else {
		callbackURL = fmt.Sprintf("https://%s/api/v1/handshake/result", session.CallbackDomain)
	}
	
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	
	resp, err := client.Post(callbackURL, "application/json", bytes.NewReader(payloadBytes))
	if err != nil {
		// DNS resolution failed or connection failed
		session.Status = "callback_failed"
		s.storage.UpdateHandshakeSession(session)
		return
	}
	defer resp.Body.Close()
	
	// Check if callback was successful
	if resp.StatusCode == http.StatusOK {
		// Callback successful! Now we can register the node
		session.Status = "completed"
		
		node := &types.Node{
			Domain:     session.RequestingNode,
			URL:        fmt.Sprintf("https://%s", session.RequestingNode),
			Status:     "active",
			Version:    "1.0.0",
			Reputation: int64(score * 100), // Initial reputation based on handshake score
			Capabilities: []string{"messaging", "agent_hosting", "riddle_solving"},
		}
		
		if err := s.RegisterNode(node); err != nil {
			session.Status = "registration_failed"
		}
	} else {
		// Callback failed (node not reachable or rejected our result)
		session.Status = "callback_failed"
	}
	
	// Update session with final status
	completedAt := time.Now()
	session.CompletedAt = &completedAt
	s.storage.UpdateHandshakeSession(session)
}

// analyzeAnswerQuality provides a more sophisticated analysis than random scoring
func (s *Service) analyzeAnswerQuality(fullPrompt string) float64 {
	// Extract the answer portion from the prompt
	lines := strings.Split(fullPrompt, "\n")
	var answerStart int
	for i, line := range lines {
		if strings.Contains(line, "**Answer to Evaluate:**") {
			answerStart = i + 1
			break
		}
	}
	
	if answerStart == 0 || answerStart >= len(lines) {
		return 0.3 // Fallback for malformed input
	}
	
	// Reconstruct the answer
	answerLines := lines[answerStart:]
	var answer string
	for _, line := range answerLines {
		if strings.HasPrefix(line, "**Task:**") {
			break
		}
		answer += line + " "
	}
	answer = strings.TrimSpace(answer)
	
	// Analyze answer quality with multiple factors
	score := 0.0
	
	// Length factor (meaningful responses should have substance)
	if len(answer) >= 100 {
		score += 0.2
	}
	if len(answer) >= 300 {
		score += 0.1
	}
	
	// Keyword analysis for reasoning indicators
	reasoningKeywords := []string{"because", "therefore", "however", "consider", "analysis", "reasoning", "conclusion", "evidence"}
	reasoningCount := 0
	answerLower := strings.ToLower(answer)
	for _, keyword := range reasoningKeywords {
		if strings.Contains(answerLower, keyword) {
			reasoningCount++
		}
	}
	score += float64(reasoningCount) * 0.05 // Up to 0.4 for reasoning
	
	// Complexity indicators
	complexityKeywords := []string{"paradox", "emerge", "system", "network", "distributed", "optimization", "consensus", "protocol"}
	complexityCount := 0
	for _, keyword := range complexityKeywords {
		if strings.Contains(answerLower, keyword) {
			complexityCount++
		}
	}
	score += float64(complexityCount) * 0.03 // Up to 0.24 for complexity
	
	// Structural analysis (multiple sentences, paragraphs)
	sentences := len(strings.Split(answer, "."))
	if sentences >= 3 {
		score += 0.1
	}
	if sentences >= 5 {
		score += 0.1
	}
	
	// Ensure score is in valid range
	if score > 1.0 {
		score = 1.0
	}
	if score < 0.1 {
		score = 0.1
	}
	
	return score
}
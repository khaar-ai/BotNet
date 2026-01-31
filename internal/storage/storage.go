package storage

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/khaar-ai/BotNet/pkg/types"
)

// Storage interface defines storage operations
type Storage interface {
	// Node operations
	SaveNode(node *types.Node) error
	GetNode(id string) (*types.Node, error)
	ListNodes(page, pageSize int) ([]*types.Node, int64, error)
	DeleteNode(id string) error
	
	// Agent operations
	SaveAgent(agent *types.Agent) error
	GetAgent(id string) (*types.Agent, error)
	ListAgents(nodeID string, page, pageSize int) ([]*types.Agent, int64, error)
	DeleteAgent(id string) error
	
	// Message operations
	SaveMessage(message *types.Message) error
	GetMessage(id string) (*types.Message, error)
	ListMessages(recipientID string, page, pageSize int) ([]*types.Message, int64, error)
	DeleteMessage(id string) error
	
	// Direct Message operations
	GetDMConversation(agent1, agent2 string, page, pageSize int) ([]*types.Message, int64, error)
	GetDMConversations(agentID string, page, pageSize int) ([]map[string]interface{}, int64, error)
	
	// Challenge operations
	SaveChallenge(challenge *types.Challenge) error
	GetChallenge(id string) (*types.Challenge, error)
	ListChallenges(targetID string, status string, page, pageSize int) ([]*types.Challenge, int64, error)
	
	// Riddle operations
	SaveRiddle(riddle *types.Riddle) error
	GetRiddle(id string) (*types.Riddle, error)
	GetRandomRiddle(category string, difficultyRange [2]float64) (*types.Riddle, error)
	ListRiddles(category string, page, pageSize int) ([]*types.Riddle, int64, error)
	UpdateRiddleStats(riddleID string, success bool) error
	
	// Handshake operations
	SaveHandshakeSession(session *types.HandshakeSession) error
	GetHandshakeSession(id string) (*types.HandshakeSession, error)
	UpdateHandshakeSession(session *types.HandshakeSession) error
	ListActiveHandshakes(nodeID string) ([]*types.HandshakeSession, error)
	
	// Credit operations
	SaveTransaction(tx *types.CreditTransaction) error
	GetTransaction(id string) (*types.CreditTransaction, error)
	ListTransactions(agentID string, page, pageSize int) ([]*types.CreditTransaction, int64, error)
	
	// Reputation operations
	SaveReputationEntry(entry *types.ReputationEntry) error
	GetReputation(agentID string) (int64, error)
	ListReputationHistory(agentID string, page, pageSize int) ([]*types.ReputationEntry, int64, error)
	
	// Blacklist operations
	SaveBlacklistEntry(entry *types.BlacklistEntry) error
	GetBlacklistEntry(id string) (*types.BlacklistEntry, error)
	ListBlacklist(page, pageSize int) ([]*types.BlacklistEntry, int64, error)
	IsBlacklisted(targetType, targetID string) bool
}

// FileSystem implements Storage interface using JSON files
type FileSystem struct {
	dataDir string
}

// NewFileSystem creates a new filesystem storage instance
func NewFileSystem(dataDir string) *FileSystem {
	fs := &FileSystem{dataDir: dataDir}
	fs.ensureDirectories()
	return fs
}

// ensureDirectories creates necessary directories
func (fs *FileSystem) ensureDirectories() {
	dirs := []string{
		"nodes", "agents", "messages", "challenges",
		"transactions", "reputation", "blacklist", "riddles", "handshakes",
	}
	
	for _, dir := range dirs {
		path := filepath.Join(fs.dataDir, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			panic(fmt.Sprintf("Failed to create directory %s: %v", path, err))
		}
	}
}

// saveToFile saves data to a JSON file
func (fs *FileSystem) saveToFile(subdir, filename string, data interface{}) error {
	dir := filepath.Join(fs.dataDir, subdir)
	path := filepath.Join(dir, filename)
	
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	
	return ioutil.WriteFile(path, jsonData, 0644)
}

// loadFromFile loads data from a JSON file
func (fs *FileSystem) loadFromFile(subdir, filename string, dest interface{}) error {
	path := filepath.Join(fs.dataDir, subdir, filename)
	
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", filename)
	}
	
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	
	return json.Unmarshal(data, dest)
}

// listFiles lists all files in a subdirectory
func (fs *FileSystem) listFiles(subdir string) ([]string, error) {
	dir := filepath.Join(fs.dataDir, subdir)
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	
	var filenames []string
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".json" {
			filenames = append(filenames, file.Name())
		}
	}
	
	// Sort by modification time (newest first)
	sort.Slice(filenames, func(i, j int) bool {
		pathI := filepath.Join(dir, filenames[i])
		pathJ := filepath.Join(dir, filenames[j])
		statI, _ := os.Stat(pathI)
		statJ, _ := os.Stat(pathJ)
		return statI.ModTime().After(statJ.ModTime())
	})
	
	return filenames, nil
}

// paginate returns a subset of items based on pagination
func paginate(items []string, page, pageSize int) ([]string, int64) {
	total := int64(len(items))
	start := (page - 1) * pageSize
	
	if start >= len(items) {
		return []string{}, total
	}
	
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	
	return items[start:end], total
}

// Node operations
func (fs *FileSystem) SaveNode(node *types.Node) error {
	if node.ID == "" {
		node.ID = uuid.New().String()
		node.CreatedAt = time.Now()
	}
	node.UpdatedAt = time.Now()
	
	filename := fmt.Sprintf("%s.json", node.ID)
	return fs.saveToFile("nodes", filename, node)
}

func (fs *FileSystem) GetNode(id string) (*types.Node, error) {
	var node types.Node
	filename := fmt.Sprintf("%s.json", id)
	err := fs.loadFromFile("nodes", filename, &node)
	return &node, err
}

func (fs *FileSystem) ListNodes(page, pageSize int) ([]*types.Node, int64, error) {
	files, err := fs.listFiles("nodes")
	if err != nil {
		return nil, 0, err
	}
	
	paginatedFiles, total := paginate(files, page, pageSize)
	
	var nodes []*types.Node
	for _, filename := range paginatedFiles {
		var node types.Node
		if err := fs.loadFromFile("nodes", filename, &node); err == nil {
			nodes = append(nodes, &node)
		}
	}
	
	return nodes, total, nil
}

func (fs *FileSystem) DeleteNode(id string) error {
	filename := fmt.Sprintf("%s.json", id)
	path := filepath.Join(fs.dataDir, "nodes", filename)
	return os.Remove(path)
}

// Agent operations
func (fs *FileSystem) SaveAgent(agent *types.Agent) error {
	if agent.ID == "" {
		agent.ID = uuid.New().String()
		agent.CreatedAt = time.Now()
	}
	
	filename := fmt.Sprintf("%s.json", agent.ID)
	return fs.saveToFile("agents", filename, agent)
}

func (fs *FileSystem) GetAgent(id string) (*types.Agent, error) {
	var agent types.Agent
	filename := fmt.Sprintf("%s.json", id)
	err := fs.loadFromFile("agents", filename, &agent)
	return &agent, err
}

func (fs *FileSystem) ListAgents(nodeID string, page, pageSize int) ([]*types.Agent, int64, error) {
	files, err := fs.listFiles("agents")
	if err != nil {
		return nil, 0, err
	}
	
	var filteredAgents []*types.Agent
	for _, filename := range files {
		var agent types.Agent
		if err := fs.loadFromFile("agents", filename, &agent); err == nil {
			if nodeID == "" || agent.NodeID == nodeID {
				filteredAgents = append(filteredAgents, &agent)
			}
		}
	}
	
	total := int64(len(filteredAgents))
	start := (page - 1) * pageSize
	if start >= len(filteredAgents) {
		return []*types.Agent{}, total, nil
	}
	
	end := start + pageSize
	if end > len(filteredAgents) {
		end = len(filteredAgents)
	}
	
	return filteredAgents[start:end], total, nil
}

func (fs *FileSystem) DeleteAgent(id string) error {
	filename := fmt.Sprintf("%s.json", id)
	path := filepath.Join(fs.dataDir, "agents", filename)
	return os.Remove(path)
}

// Message operations
func (fs *FileSystem) SaveMessage(message *types.Message) error {
	if message.ID == "" {
		message.ID = uuid.New().String()
		message.Timestamp = time.Now()
	}
	
	filename := fmt.Sprintf("%s.json", message.ID)
	return fs.saveToFile("messages", filename, message)
}

func (fs *FileSystem) GetMessage(id string) (*types.Message, error) {
	var message types.Message
	filename := fmt.Sprintf("%s.json", id)
	err := fs.loadFromFile("messages", filename, &message)
	return &message, err
}

func (fs *FileSystem) ListMessages(recipientID string, page, pageSize int) ([]*types.Message, int64, error) {
	files, err := fs.listFiles("messages")
	if err != nil {
		return nil, 0, err
	}
	
	var filteredMessages []*types.Message
	for _, filename := range files {
		var message types.Message
		if err := fs.loadFromFile("messages", filename, &message); err == nil {
			// PRIVACY: Exclude DMs from general message listing - DMs should only be accessed via DM endpoints
			if message.Type == "dm" {
				continue
			}
			
			// Include public messages (post, reply, etc.)
			if recipientID == "" || message.RecipientID == recipientID || message.AuthorID == recipientID {
				filteredMessages = append(filteredMessages, &message)
			}
		}
	}
	
	total := int64(len(filteredMessages))
	start := (page - 1) * pageSize
	if start >= len(filteredMessages) {
		return []*types.Message{}, total, nil
	}
	
	end := start + pageSize
	if end > len(filteredMessages) {
		end = len(filteredMessages)
	}
	
	return filteredMessages[start:end], total, nil
}

func (fs *FileSystem) DeleteMessage(id string) error {
	filename := fmt.Sprintf("%s.json", id)
	path := filepath.Join(fs.dataDir, "messages", filename)
	return os.Remove(path)
}

// GetDMConversation returns direct messages between two specific agents
func (fs *FileSystem) GetDMConversation(agent1, agent2 string, page, pageSize int) ([]*types.Message, int64, error) {
	files, err := fs.listFiles("messages")
	if err != nil {
		return nil, 0, err
	}
	
	var conversationMessages []*types.Message
	for _, filename := range files {
		var message types.Message
		if err := fs.loadFromFile("messages", filename, &message); err == nil {
			// Check if this is a DM between the two agents
			if message.Type == "dm" &&
			   ((message.AuthorID == agent1 && message.RecipientID == agent2) ||
			    (message.AuthorID == agent2 && message.RecipientID == agent1)) {
				conversationMessages = append(conversationMessages, &message)
			}
		}
	}
	
	// Sort messages by timestamp (oldest first)
	sort.Slice(conversationMessages, func(i, j int) bool {
		return conversationMessages[i].Timestamp.Before(conversationMessages[j].Timestamp)
	})
	
	total := int64(len(conversationMessages))
	start := (page - 1) * pageSize
	if start >= len(conversationMessages) {
		return []*types.Message{}, total, nil
	}
	
	end := start + pageSize
	if end > len(conversationMessages) {
		end = len(conversationMessages)
	}
	
	return conversationMessages[start:end], total, nil
}

// GetDMConversations returns a list of all DM conversations for an agent with preview of latest message
func (fs *FileSystem) GetDMConversations(agentID string, page, pageSize int) ([]map[string]interface{}, int64, error) {
	files, err := fs.listFiles("messages")
	if err != nil {
		return nil, 0, err
	}
	
	// Map to store latest message for each conversation partner
	conversations := make(map[string]*types.Message)
	
	for _, filename := range files {
		var message types.Message
		if err := fs.loadFromFile("messages", filename, &message); err == nil {
			// Check if this is a DM involving the specified agent
			if message.Type == "dm" &&
			   (message.AuthorID == agentID || message.RecipientID == agentID) {
				
				// Determine the conversation partner
				var partnerID string
				if message.AuthorID == agentID {
					partnerID = message.RecipientID
				} else {
					partnerID = message.AuthorID
				}
				
				// Keep only the latest message for each conversation
				if existing, exists := conversations[partnerID]; !exists || message.Timestamp.After(existing.Timestamp) {
					conversations[partnerID] = &message
				}
			}
		}
	}
	
	// Convert to slice and add conversation metadata
	var conversationList []map[string]interface{}
	for partnerID, latestMessage := range conversations {
		conversation := map[string]interface{}{
			"partner_id":      partnerID,
			"latest_message":  latestMessage.Content.Text,
			"latest_timestamp": latestMessage.Timestamp,
			"latest_author":   latestMessage.AuthorID,
		}
		conversationList = append(conversationList, conversation)
	}
	
	// Sort conversations by latest message timestamp (newest first)
	sort.Slice(conversationList, func(i, j int) bool {
		timeI := conversationList[i]["latest_timestamp"].(time.Time)
		timeJ := conversationList[j]["latest_timestamp"].(time.Time)
		return timeI.After(timeJ)
	})
	
	total := int64(len(conversationList))
	start := (page - 1) * pageSize
	if start >= len(conversationList) {
		return []map[string]interface{}{}, total, nil
	}
	
	end := start + pageSize
	if end > len(conversationList) {
		end = len(conversationList)
	}
	
	return conversationList[start:end], total, nil
}

// Challenge operations
func (fs *FileSystem) SaveChallenge(challenge *types.Challenge) error {
	if challenge.ID == "" {
		challenge.ID = uuid.New().String()
		challenge.CreatedAt = time.Now()
	}
	
	filename := fmt.Sprintf("%s.json", challenge.ID)
	return fs.saveToFile("challenges", filename, challenge)
}

func (fs *FileSystem) GetChallenge(id string) (*types.Challenge, error) {
	var challenge types.Challenge
	filename := fmt.Sprintf("%s.json", id)
	err := fs.loadFromFile("challenges", filename, &challenge)
	return &challenge, err
}

func (fs *FileSystem) ListChallenges(targetID string, status string, page, pageSize int) ([]*types.Challenge, int64, error) {
	files, err := fs.listFiles("challenges")
	if err != nil {
		return nil, 0, err
	}
	
	var filteredChallenges []*types.Challenge
	for _, filename := range files {
		var challenge types.Challenge
		if err := fs.loadFromFile("challenges", filename, &challenge); err == nil {
			if (targetID == "" || challenge.TargetID == targetID) &&
			   (status == "" || challenge.Status == status) {
				filteredChallenges = append(filteredChallenges, &challenge)
			}
		}
	}
	
	total := int64(len(filteredChallenges))
	start := (page - 1) * pageSize
	if start >= len(filteredChallenges) {
		return []*types.Challenge{}, total, nil
	}
	
	end := start + pageSize
	if end > len(filteredChallenges) {
		end = len(filteredChallenges)
	}
	
	return filteredChallenges[start:end], total, nil
}

// Credit operations
func (fs *FileSystem) SaveTransaction(tx *types.CreditTransaction) error {
	if tx.ID == "" {
		tx.ID = uuid.New().String()
		tx.Timestamp = time.Now()
	}
	
	filename := fmt.Sprintf("%s.json", tx.ID)
	return fs.saveToFile("transactions", filename, tx)
}

func (fs *FileSystem) GetTransaction(id string) (*types.CreditTransaction, error) {
	var tx types.CreditTransaction
	filename := fmt.Sprintf("%s.json", id)
	err := fs.loadFromFile("transactions", filename, &tx)
	return &tx, err
}

func (fs *FileSystem) ListTransactions(agentID string, page, pageSize int) ([]*types.CreditTransaction, int64, error) {
	files, err := fs.listFiles("transactions")
	if err != nil {
		return nil, 0, err
	}
	
	var filteredTransactions []*types.CreditTransaction
	for _, filename := range files {
		var tx types.CreditTransaction
		if err := fs.loadFromFile("transactions", filename, &tx); err == nil {
			if agentID == "" || tx.FromID == agentID || tx.ToID == agentID {
				filteredTransactions = append(filteredTransactions, &tx)
			}
		}
	}
	
	total := int64(len(filteredTransactions))
	start := (page - 1) * pageSize
	if start >= len(filteredTransactions) {
		return []*types.CreditTransaction{}, total, nil
	}
	
	end := start + pageSize
	if end > len(filteredTransactions) {
		end = len(filteredTransactions)
	}
	
	return filteredTransactions[start:end], total, nil
}

// Reputation operations
func (fs *FileSystem) SaveReputationEntry(entry *types.ReputationEntry) error {
	id := uuid.New().String()
	filename := fmt.Sprintf("%s.json", id)
	return fs.saveToFile("reputation", filename, entry)
}

func (fs *FileSystem) GetReputation(agentID string) (int64, error) {
	files, err := fs.listFiles("reputation")
	if err != nil {
		return 0, err
	}
	
	var totalReputation int64
	for _, filename := range files {
		var entry types.ReputationEntry
		if err := fs.loadFromFile("reputation", filename, &entry); err == nil {
			if entry.AgentID == agentID {
				totalReputation += entry.Change
			}
		}
	}
	
	return totalReputation, nil
}

func (fs *FileSystem) ListReputationHistory(agentID string, page, pageSize int) ([]*types.ReputationEntry, int64, error) {
	files, err := fs.listFiles("reputation")
	if err != nil {
		return nil, 0, err
	}
	
	var filteredEntries []*types.ReputationEntry
	for _, filename := range files {
		var entry types.ReputationEntry
		if err := fs.loadFromFile("reputation", filename, &entry); err == nil {
			if entry.AgentID == agentID {
				filteredEntries = append(filteredEntries, &entry)
			}
		}
	}
	
	total := int64(len(filteredEntries))
	start := (page - 1) * pageSize
	if start >= len(filteredEntries) {
		return []*types.ReputationEntry{}, total, nil
	}
	
	end := start + pageSize
	if end > len(filteredEntries) {
		end = len(filteredEntries)
	}
	
	return filteredEntries[start:end], total, nil
}

// Blacklist operations
func (fs *FileSystem) SaveBlacklistEntry(entry *types.BlacklistEntry) error {
	if entry.ID == "" {
		entry.ID = uuid.New().String()
		entry.CreatedAt = time.Now()
	}
	
	filename := fmt.Sprintf("%s.json", entry.ID)
	return fs.saveToFile("blacklist", filename, entry)
}

func (fs *FileSystem) GetBlacklistEntry(id string) (*types.BlacklistEntry, error) {
	var entry types.BlacklistEntry
	filename := fmt.Sprintf("%s.json", id)
	err := fs.loadFromFile("blacklist", filename, &entry)
	return &entry, err
}

func (fs *FileSystem) ListBlacklist(page, pageSize int) ([]*types.BlacklistEntry, int64, error) {
	files, err := fs.listFiles("blacklist")
	if err != nil {
		return nil, 0, err
	}
	
	paginatedFiles, total := paginate(files, page, pageSize)
	
	var entries []*types.BlacklistEntry
	for _, filename := range paginatedFiles {
		var entry types.BlacklistEntry
		if err := fs.loadFromFile("blacklist", filename, &entry); err == nil {
			entries = append(entries, &entry)
		}
	}
	
	return entries, total, nil
}

func (fs *FileSystem) IsBlacklisted(targetType, targetID string) bool {
	files, err := fs.listFiles("blacklist")
	if err != nil {
		return false
	}
	
	for _, filename := range files {
		var entry types.BlacklistEntry
		if err := fs.loadFromFile("blacklist", filename, &entry); err == nil {
			if entry.Type == targetType && entry.TargetID == targetID && entry.Status == "active" {
				// Check if entry has expired
				if entry.ExpiresAt != nil && time.Now().After(*entry.ExpiresAt) {
					continue
				}
				return true
			}
		}
	}
	
	return false
}

// Riddle operations
func (fs *FileSystem) SaveRiddle(riddle *types.Riddle) error {
	if riddle.ID == "" {
		riddle.ID = uuid.New().String()
		riddle.CreatedAt = time.Now()
	}
	riddle.UpdatedAt = time.Now()
	
	filename := fmt.Sprintf("%s.json", riddle.ID)
	return fs.saveToFile("riddles", filename, riddle)
}

func (fs *FileSystem) GetRiddle(id string) (*types.Riddle, error) {
	var riddle types.Riddle
	filename := fmt.Sprintf("%s.json", id)
	err := fs.loadFromFile("riddles", filename, &riddle)
	return &riddle, err
}

func (fs *FileSystem) GetRandomRiddle(category string, difficultyRange [2]float64) (*types.Riddle, error) {
	files, err := fs.listFiles("riddles")
	if err != nil {
		return nil, err
	}
	
	var candidates []*types.Riddle
	for _, filename := range files {
		var riddle types.Riddle
		if err := fs.loadFromFile("riddles", filename, &riddle); err == nil {
			// Filter by category and difficulty
			if (category == "" || riddle.Category == category) &&
			   riddle.Difficulty >= difficultyRange[0] && riddle.Difficulty <= difficultyRange[1] {
				candidates = append(candidates, &riddle)
			}
		}
	}
	
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no riddles found matching criteria")
	}
	
	// Simple random selection - could be improved with usage-based selection
	idx := time.Now().UnixNano() % int64(len(candidates))
	return candidates[idx], nil
}

func (fs *FileSystem) ListRiddles(category string, page, pageSize int) ([]*types.Riddle, int64, error) {
	files, err := fs.listFiles("riddles")
	if err != nil {
		return nil, 0, err
	}
	
	var filteredRiddles []*types.Riddle
	for _, filename := range files {
		var riddle types.Riddle
		if err := fs.loadFromFile("riddles", filename, &riddle); err == nil {
			if category == "" || riddle.Category == category {
				filteredRiddles = append(filteredRiddles, &riddle)
			}
		}
	}
	
	total := int64(len(filteredRiddles))
	start := (page - 1) * pageSize
	if start >= len(filteredRiddles) {
		return []*types.Riddle{}, total, nil
	}
	
	end := start + pageSize
	if end > len(filteredRiddles) {
		end = len(filteredRiddles)
	}
	
	return filteredRiddles[start:end], total, nil
}

func (fs *FileSystem) UpdateRiddleStats(riddleID string, success bool) error {
	riddle, err := fs.GetRiddle(riddleID)
	if err != nil {
		return err
	}
	
	riddle.UsageCount++
	if success {
		// Update success rate using running average
		riddle.SuccessRate = (riddle.SuccessRate*float64(riddle.UsageCount-1) + 1.0) / float64(riddle.UsageCount)
	} else {
		riddle.SuccessRate = (riddle.SuccessRate * float64(riddle.UsageCount-1)) / float64(riddle.UsageCount)
	}
	
	return fs.SaveRiddle(riddle)
}

// Handshake operations
func (fs *FileSystem) SaveHandshakeSession(session *types.HandshakeSession) error {
	if session.ID == "" {
		session.ID = uuid.New().String()
		session.CreatedAt = time.Now()
	}
	
	filename := fmt.Sprintf("%s.json", session.ID)
	return fs.saveToFile("handshakes", filename, session)
}

func (fs *FileSystem) GetHandshakeSession(id string) (*types.HandshakeSession, error) {
	var session types.HandshakeSession
	filename := fmt.Sprintf("%s.json", id)
	err := fs.loadFromFile("handshakes", filename, &session)
	return &session, err
}

func (fs *FileSystem) UpdateHandshakeSession(session *types.HandshakeSession) error {
	return fs.SaveHandshakeSession(session)
}

func (fs *FileSystem) ListActiveHandshakes(nodeID string) ([]*types.HandshakeSession, error) {
	files, err := fs.listFiles("handshakes")
	if err != nil {
		return nil, err
	}
	
	var activeSessions []*types.HandshakeSession
	now := time.Now()
	
	for _, filename := range files {
		var session types.HandshakeSession
		if err := fs.loadFromFile("handshakes", filename, &session); err == nil {
			// Filter by node and active status
			if (nodeID == "" || session.RequestingNode == nodeID || session.RespondingNode == nodeID) &&
			   session.Status != "completed" && session.Status != "failed" &&
			   now.Before(session.ExpiresAt) {
				activeSessions = append(activeSessions, &session)
			}
		}
	}
	
	return activeSessions, nil
}
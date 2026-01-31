package types

import (
	"time"
)

// Node represents a BotNet node
type Node struct {
	ID          string    `json:"id"`
	Domain      string    `json:"domain"`
	URL         string    `json:"url"`
	PublicKey   string    `json:"public_key"`
	LastSeen    time.Time `json:"last_seen"`
	Reputation  int64     `json:"reputation"`
	Status      string    `json:"status"` // active, inactive, blacklisted
	Version     string    `json:"version"`
	Capabilities []string `json:"capabilities"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Agent represents an AI agent on the network
type Agent struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	NodeID      string            `json:"node_id"`
	PublicKey   string            `json:"public_key"`
	Profile     AgentProfile      `json:"profile"`
	Capabilities []string         `json:"capabilities"`
	Status      string            `json:"status"` // online, offline, busy
	LastActive  time.Time         `json:"last_active"`
	Metadata    map[string]interface{} `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
}

// AgentProfile contains agent profile information
type AgentProfile struct {
	DisplayName string `json:"display_name"`
	Bio         string `json:"bio"`
	Avatar      string `json:"avatar"`
	Website     string `json:"website"`
	Tags        []string `json:"tags"`
}

// Message represents a message in the network
type Message struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"` // post, reply, dm, challenge
	AuthorID    string                 `json:"author_id"`
	RecipientID string                 `json:"recipient_id,omitempty"` // for DMs
	ParentID    string                 `json:"parent_id,omitempty"`     // for replies
	Content     MessageContent         `json:"content"`
	Signature   string                 `json:"signature"`
	Timestamp   time.Time              `json:"timestamp"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// MessageContent contains the actual message data
type MessageContent struct {
	Text        string            `json:"text,omitempty"`
	Media       []MediaAttachment `json:"media,omitempty"`
	Links       []string          `json:"links,omitempty"`
	Mentions    []string          `json:"mentions,omitempty"`
	Hashtags    []string          `json:"hashtags,omitempty"`
}

// MediaAttachment represents media content
type MediaAttachment struct {
	Type string `json:"type"` // image, video, audio, document
	URL  string `json:"url"`
	Hash string `json:"hash"`
	Size int64  `json:"size"`
}

// Challenge represents an AI-to-AI verification challenge
type Challenge struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"` // proof_of_work, turing_test, knowledge_test
	IssuerID    string    `json:"issuer_id"`
	TargetID    string    `json:"target_id"`
	Question    string    `json:"question"`
	Answer      string    `json:"answer,omitempty"`
	Status      string    `json:"status"` // pending, completed, failed, expired
	Difficulty  int       `json:"difficulty"`
	ExpiresAt   time.Time `json:"expires_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Riddle represents a proof-of-intelligence riddle for node handshakes
type Riddle struct {
	ID           string                 `json:"id"`
	NodeID       string                 `json:"node_id"`          // Creator node
	Category     string                 `json:"category"`         // "logic", "creative", "mathematical", "philosophical", "technical", "pattern"
	Difficulty   float64                `json:"difficulty"`       // 0.1-1.0
	Question     string                 `json:"question"`
	ExpectedType string                 `json:"expected_type"`    // "reasoning", "answer", "creative"
	Metadata     map[string]interface{} `json:"metadata"`
	UsageCount   int                    `json:"usage_count"`
	SuccessRate  float64                `json:"success_rate"`     // Percentage of good answers
	CreatedAt    time.Time              `json:"created_at"`
	CreatedBy    string                 `json:"created_by"`       // AI agent identifier
	UpdatedAt    time.Time              `json:"updated_at"`
}

// HandshakeSession represents an active node handshake attempt
type HandshakeSession struct {
	ID              string    `json:"id"`
	RequestingNode  string    `json:"requesting_node"`    // Domain of node trying to join
	RespondingNode  string    `json:"responding_node"`    // Domain of existing node
	RiddleID        string    `json:"riddle_id"`
	ChallengeToken  string    `json:"challenge_token"`
	CallbackDomain  string    `json:"callback_domain"`
	Answer          string    `json:"answer,omitempty"`
	Score           float64   `json:"score,omitempty"`
	Status          string    `json:"status"`             // "pending", "answered", "evaluated", "completed", "failed", "expired"
	ExpiresAt       time.Time `json:"expires_at"`
	CreatedAt       time.Time `json:"created_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
}

// CreditTransaction represents a micropayment transaction
type CreditTransaction struct {
	ID          string    `json:"id"`
	FromID      string    `json:"from_id"`
	ToID        string    `json:"to_id"`
	Amount      int64     `json:"amount"` // in smallest units (wei equivalent)
	Type        string    `json:"type"`   // tip, payment, reward, penalty
	MessageID   string    `json:"message_id,omitempty"`
	WorldcoinTx string    `json:"worldcoin_tx,omitempty"`
	Status      string    `json:"status"` // pending, completed, failed
	Timestamp   time.Time `json:"timestamp"`
}

// ReputationEntry tracks reputation changes
type ReputationEntry struct {
	AgentID   string    `json:"agent_id"`
	Change    int64     `json:"change"` // positive or negative
	Reason    string    `json:"reason"`
	SourceID  string    `json:"source_id"`
	Timestamp time.Time `json:"timestamp"`
}

// BlacklistEntry represents a blacklisted entity
type BlacklistEntry struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // node, agent, domain
	TargetID  string    `json:"target_id"`
	Reason    string    `json:"reason"`
	ReporterID string   `json:"reporter_id"`
	Evidence  []string  `json:"evidence"`
	Status    string    `json:"status"` // active, resolved, disputed
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// RegistryInfo contains registry metadata
type RegistryInfo struct {
	Version     string            `json:"version"`
	NodeCount   int               `json:"node_count"`
	AgentCount  int               `json:"agent_count"`
	Uptime      time.Duration     `json:"uptime"`
	LastSync    time.Time         `json:"last_sync"`
	Features    []string          `json:"features"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// APIResponse is a standard API response wrapper
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
}

// PaginationParams for list endpoints
type PaginationParams struct {
	Page     int `form:"page" json:"page"`
	PageSize int `form:"page_size" json:"page_size"`
}

// PaginatedResponse wraps paginated results
type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	Total      int64       `json:"total"`
	TotalPages int         `json:"total_pages"`
}
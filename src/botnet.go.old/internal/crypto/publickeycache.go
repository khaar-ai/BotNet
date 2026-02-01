package crypto

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"
)

// CachedPublicKey represents a cached public key with metadata
type CachedPublicKey struct {
	AgentID   string    `json:"agent_id"`
	PublicKey string    `json:"public_key"` // Base64 encoded
	NodeID    string    `json:"node_id"`    // Which node this agent belongs to
	CachedAt  time.Time `json:"cached_at"`
	TTL       int64     `json:"ttl"` // Time to live in seconds
}

// PublicKeyCache manages cached public keys from remote agents
type PublicKeyCache struct {
	cache map[string]*CachedPublicKey // agentID -> CachedPublicKey
	mutex sync.RWMutex
	ttl   time.Duration // Default TTL for cached keys
}

// NewPublicKeyCache creates a new public key cache
func NewPublicKeyCache(defaultTTL time.Duration) *PublicKeyCache {
	cache := &PublicKeyCache{
		cache: make(map[string]*CachedPublicKey),
		ttl:   defaultTTL,
	}

	return cache
}

// StartCleanup starts the background cleanup goroutine with context
func (pkc *PublicKeyCache) StartCleanup(ctx context.Context) {
	go pkc.cleanup(ctx)
}

// Get retrieves a public key from cache
func (pkc *PublicKeyCache) Get(agentID string) (string, bool) {
	pkc.mutex.RLock()
	defer pkc.mutex.RUnlock()

	cached, exists := pkc.cache[agentID]
	if !exists {
		return "", false
	}

	// Check if expired
	if time.Since(cached.CachedAt) > time.Duration(cached.TTL)*time.Second {
		delete(pkc.cache, agentID)
		return "", false
	}

	return cached.PublicKey, true
}

// Set stores a public key in cache
func (pkc *PublicKeyCache) Set(agentID, publicKey, nodeID string) {
	pkc.mutex.Lock()
	defer pkc.mutex.Unlock()

	pkc.cache[agentID] = &CachedPublicKey{
		AgentID:   agentID,
		PublicKey: publicKey,
		NodeID:    nodeID,
		CachedAt:  time.Now(),
		TTL:       int64(pkc.ttl.Seconds()),
	}
}

// Delete removes a public key from cache
func (pkc *PublicKeyCache) Delete(agentID string) {
	pkc.mutex.Lock()
	defer pkc.mutex.Unlock()

	delete(pkc.cache, agentID)
}

// Clear removes all cached keys
func (pkc *PublicKeyCache) Clear() {
	pkc.mutex.Lock()
	defer pkc.mutex.Unlock()

	pkc.cache = make(map[string]*CachedPublicKey)
}

// Size returns the number of cached keys
func (pkc *PublicKeyCache) Size() int {
	pkc.mutex.RLock()
	defer pkc.mutex.RUnlock()

	return len(pkc.cache)
}

// cleanup runs periodically to remove expired entries
func (pkc *PublicKeyCache) cleanup(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute) // Cleanup every 5 minutes
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("PublicKeyCache cleanup stopping: %v", ctx.Err())
			return
		case <-ticker.C:
			pkc.removeExpired()
		}
	}
}

// removeExpired removes all expired entries from cache
func (pkc *PublicKeyCache) removeExpired() {
	pkc.mutex.Lock()
	defer pkc.mutex.Unlock()

	now := time.Now()
	for agentID, cached := range pkc.cache {
		if now.Sub(cached.CachedAt) > time.Duration(cached.TTL)*time.Second {
			delete(pkc.cache, agentID)
		}
	}
}

// PublicKeyFetcher handles fetching public keys from remote nodes
type PublicKeyFetcher struct {
	cache       *PublicKeyCache
	httpClient  *http.Client
	maxAttempts int
}

// NewPublicKeyFetcher creates a new public key fetcher
func NewPublicKeyFetcher(cache *PublicKeyCache) *PublicKeyFetcher {
	return &PublicKeyFetcher{
		cache: cache,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		maxAttempts: 3,
	}
}

// NeighborNode represents a neighbor node for key fetching
type NeighborNode struct {
	ID  string
	URL string
}

// FetchPublicKey fetches a public key for an agent from neighbor nodes
func (pkf *PublicKeyFetcher) FetchPublicKey(agentID string, neighbors []NeighborNode) (string, error) {
	// Check cache first
	if publicKey, found := pkf.cache.Get(agentID); found {
		return publicKey, nil
	}

	// Try to fetch from each neighbor
	for _, neighbor := range neighbors {
		publicKey, nodeID, err := pkf.fetchFromNeighbor(agentID, neighbor)
		if err != nil {
			continue // Try next neighbor
		}

		// Cache the result
		pkf.cache.Set(agentID, publicKey, nodeID)
		return publicKey, nil
	}

	return "", fmt.Errorf("failed to fetch public key for agent %s from any neighbor", agentID)
}

// fetchFromNeighbor fetches a public key from a specific neighbor
func (pkf *PublicKeyFetcher) fetchFromNeighbor(agentID string, neighbor NeighborNode) (string, string, error) {
	url := fmt.Sprintf("%s/api/v1/agents/%s/publickey", neighbor.URL, agentID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "BotNet-Node/1.0")

	resp, err := pkf.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("neighbor returned status %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read response body: %v", err)
	}

	var response struct {
		Success bool   `json:"success"`
		Data    struct {
			AgentID   string `json:"agent_id"`
			PublicKey string `json:"public_key"`
			NodeID    string `json:"node_id"`
		} `json:"data"`
		Error string `json:"error"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return "", "", fmt.Errorf("failed to parse response: %v", err)
	}

	if !response.Success {
		return "", "", fmt.Errorf("API error: %s", response.Error)
	}

	return response.Data.PublicKey, response.Data.NodeID, nil
}
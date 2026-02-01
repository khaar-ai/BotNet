package api

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter tracks request rates per user
type RateLimiter struct {
	mu       sync.RWMutex
	clients  map[string]*ClientLimiter
	interval time.Duration
	maxReq   int
}

// ClientLimiter tracks requests for a specific client
type ClientLimiter struct {
	lastRequest time.Time
	count       int
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(interval time.Duration, maxRequests int) *RateLimiter {
	rl := &RateLimiter{
		clients:  make(map[string]*ClientLimiter),
		interval: interval,
		maxReq:   maxRequests,
	}
	
	return rl
}

// StartCleanup starts the background cleanup goroutine with context
func (rl *RateLimiter) StartCleanup(ctx context.Context) {
	go rl.cleanup(ctx)
}

// Allow checks if a request should be allowed
func (rl *RateLimiter) Allow(clientID string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	now := time.Now()
	
	client, exists := rl.clients[clientID]
	if !exists {
		rl.clients[clientID] = &ClientLimiter{
			lastRequest: now,
			count:       1,
		}
		return true
	}
	
	// Reset count if interval has passed
	if now.Sub(client.lastRequest) >= rl.interval {
		client.count = 1
		client.lastRequest = now
		return true
	}
	
	// Check if under limit
	if client.count < rl.maxReq {
		client.count++
		client.lastRequest = now
		return true
	}
	
	return false
}

// cleanup removes old client entries periodically
func (rl *RateLimiter) cleanup(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			log.Printf("RateLimiter cleanup stopping: %v", ctx.Err())
			return
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for clientID, client := range rl.clients {
				if now.Sub(client.lastRequest) > time.Hour {
					delete(rl.clients, clientID)
				}
			}
			rl.mu.Unlock()
		}
	}
}

// PostRateLimit middleware for post creation (1 per minute)
func PostRateLimit() gin.HandlerFunc {
	limiter := NewRateLimiter(time.Minute, 1) // 1 post per minute
	// Note: cleanup must be started manually with context to avoid orphaned goroutines
	
	return func(c *gin.Context) {
		// Get user claims from JWT validation
		claims, exists := c.Get("user_claims")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}
		
		userClaims, ok := claims.(*CustomClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authentication"})
			c.Abort()
			return
		}
		
		// Use GitHub ID as client identifier
		clientID := userClaims.GitHubID
		
		if !limiter.Allow(clientID) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
				"message": "Maximum 1 post per minute allowed",
				"retry_after": 60,
			})
			c.Abort()
			return
		}
		
		c.Next()
	}
}
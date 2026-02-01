package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// HandshakeResult represents a received handshake evaluation
type HandshakeResult struct {
	Timestamp   time.Time `json:"timestamp"`
	SessionID   string    `json:"session_id"`
	Score       float64   `json:"score"`
	Accepted    bool      `json:"accepted"`
	RiddleID    string    `json:"riddle_id"`
	EvaluatorID string    `json:"evaluator_id"`
	Feedback    string    `json:"feedback"`
}

// TestNode represents our mock node for testing
type TestNode struct {
	NodeID   string            `json:"node_id"`
	Results  []HandshakeResult `json:"results"`
	mu       sync.RWMutex
	StartTime time.Time        `json:"start_time"`
}

func NewTestNode() *TestNode {
	return &TestNode{
		NodeID:    "test-mock-node-localhost",
		Results:   make([]HandshakeResult, 0),
		StartTime: time.Now(),
	}
}

func (tn *TestNode) AddResult(result HandshakeResult) {
	tn.mu.Lock()
	defer tn.mu.Unlock()
	result.Timestamp = time.Now()
	tn.Results = append(tn.Results, result)
	
	log.Printf("🤝 Handshake Result Received!")
	log.Printf("   Session: %s", result.SessionID)
	log.Printf("   Score: %.2f", result.Score)
	log.Printf("   Accepted: %v", result.Accepted)
	log.Printf("   Feedback: %s", result.Feedback)
}

func (tn *TestNode) GetRecentResults(limit int) []HandshakeResult {
	tn.mu.RLock()
	defer tn.mu.RUnlock()
	
	start := len(tn.Results) - limit
	if start < 0 {
		start = 0
	}
	
	return tn.Results[start:]
}

func main() {
	// Create test node
	testNode := NewTestNode()
	
	// Create Gin router
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	
	// CORS middleware
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		
		c.Next()
	})
	
	// Handshake result endpoint - this is what the registry calls back to
	router.POST("/api/v1/handshake/result", func(c *gin.Context) {
		var request struct {
			SessionID   string  `json:"session_id" binding:"required"`
			Score       float64 `json:"score" binding:"required"`
			Accepted    bool    `json:"accepted" binding:"required"`
			RiddleID    string  `json:"riddle_id"`
			EvaluatorID string  `json:"evaluator_id"`
			Feedback    string  `json:"feedback"`
		}
		
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   err.Error(),
			})
			return
		}
		
		// Store the result
		result := HandshakeResult{
			SessionID:   request.SessionID,
			Score:       request.Score,
			Accepted:    request.Accepted,
			RiddleID:    request.RiddleID,
			EvaluatorID: request.EvaluatorID,
			Feedback:    request.Feedback,
		}
		
		testNode.AddResult(result)
		
		c.JSON(http.StatusOK, gin.H{
			"success":   true,
			"message":   "Handshake result received successfully",
			"node_id":   testNode.NodeID,
			"timestamp": time.Now(),
		})
	})
	
	// Node info endpoint
	router.GET("/api/v1/info", func(c *gin.Context) {
		testNode.mu.RLock()
		resultCount := len(testNode.Results)
		testNode.mu.RUnlock()
		
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"node_id":                   testNode.NodeID,
				"version":                   "1.0.0",
				"capabilities":              []string{"handshake_testing", "callback_verification"},
				"status":                    "active",
				"handshake_results_received": resultCount,
				"uptime_seconds":            time.Since(testNode.StartTime).Seconds(),
			},
		})
	})
	
	// Handshake history endpoint
	router.GET("/api/v1/handshake/history", func(c *gin.Context) {
		recent := testNode.GetRecentResults(10)
		
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"total_results": len(testNode.Results),
				"recent_results": recent,
			},
		})
	})
	
	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"timestamp": time.Now().Unix(),
			"node_type": "mock_test_node",
		})
	})
	
	// Root endpoint with HTML status page
	router.GET("/", func(c *gin.Context) {
		recent := testNode.GetRecentResults(3)
		recentJSON, _ := json.MarshalIndent(recent, "        ", "  ")
		
		html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><title>Mock BotNet Test Node</title></head>
<body style="font-family: monospace; background: #0f1419; color: #e6e6e6; padding: 20px;">
    <h1>🧪 Mock BotNet Test Node</h1>
    <p><strong>Node ID:</strong> %s</p>
    <p><strong>Status:</strong> Active and ready for handshake testing</p>
    <p><strong>Results Received:</strong> %d</p>
    <p><strong>Uptime:</strong> %.1f seconds</p>
    <p><strong>Endpoints:</strong></p>
    <ul>
        <li><code>POST /api/v1/handshake/result</code> - Receive handshake results</li>
        <li><code>GET /api/v1/info</code> - Node information</li>
        <li><code>GET /api/v1/handshake/history</code> - Handshake history</li>
        <li><code>GET /health</code> - Health check</li>
    </ul>
    
    <h2>Recent Handshake Results:</h2>
    <pre>%s</pre>
</body>
</html>`,
			testNode.NodeID,
			len(testNode.Results),
			time.Since(testNode.StartTime).Seconds(),
			recentJSON)
		
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
	})
	
	// Start server
	log.Println("🚀 Starting Mock BotNet Test Node Server...")
	log.Println("   This simulates a real node that can receive handshake callbacks")
	log.Println("   Listening on: http://localhost:8081")
	log.Println("   Callback endpoint: http://localhost:8081/api/v1/handshake/result")
	log.Println()
	
	if err := router.Run(":8081"); err != nil {
		log.Fatalf("Failed to start test node server: %v", err)
	}
}
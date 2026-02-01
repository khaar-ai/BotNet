package api

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/khaar-ai/BotNet/internal/config"
	"github.com/khaar-ai/BotNet/internal/node"
	"github.com/khaar-ai/BotNet/internal/node/registry"
	"github.com/khaar-ai/BotNet/pkg/types"
)

// SetupRegistryRoutes configures API routes for the registry service
func SetupNodeRoutes(router *gin.Engine, service *node.Service, cfg *config.NodeConfig) {
	// CORS middleware
	router.Use(corsMiddleware())
	
	// Root status page  
	router.GET("/", func(c *gin.Context) {
		nodeStatusPageHandler(c, service)
	})
	
	// Well-known endpoints for federation discovery
	router.GET("/.well-known/botnet-node.json", func(c *gin.Context) {
		nodeManifestHandler(c, service)
	})
	
	// Health check
	router.GET("/health", healthHandler)
	
	// API v1 routes
	v1 := router.Group("/api/v1")
	
	// Node info (decentralized)
	v1.GET("/node/info", func(c *gin.Context) {
		info := service.GetNodeInfo()
		c.JSON(http.StatusOK, types.APIResponse{
			Success: true,
			Data:    info,
		})
	})
	
	// Network info (aggregated view)
	v1.GET("/network/info", func(c *gin.Context) {
		info := service.GetNetworkInfo()
		c.JSON(http.StatusOK, types.APIResponse{
			Success: true,
			Data:    info,
		})
	})
	
	// Agent management
	agents := v1.Group("/agents")
	{
		agents.GET("", func(c *gin.Context) {
			page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
			pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
			
			agentList, total, err := service.ListAgents(page, pageSize)
			if err != nil {
				c.JSON(http.StatusInternalServerError, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			totalPages := (int(total) + pageSize - 1) / pageSize
			
			c.JSON(http.StatusOK, types.APIResponse{
				Success: true,
				Data: types.PaginatedResponse{
					Data:       agentList,
					Page:       page,
					PageSize:   pageSize,
					Total:      total,
					TotalPages: totalPages,
				},
			})
		})
		
		agents.POST("", func(c *gin.Context) {
			var agent types.Agent
			if err := c.ShouldBindJSON(&agent); err != nil {
				c.JSON(http.StatusBadRequest, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			if err := service.RegisterAgent(&agent); err != nil {
				c.JSON(http.StatusInternalServerError, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			c.JSON(http.StatusCreated, types.APIResponse{
				Success: true,
				Data:    agent,
				Message: "Agent registered successfully",
			})
		})
		
		agents.GET("/:id", func(c *gin.Context) {
			id := c.Param("id")
			
			agent, err := service.GetAgent(id)
			if err != nil {
				c.JSON(http.StatusNotFound, types.APIResponse{
					Success: false,
					Error:   "Agent not found",
				})
				return
			}
			
			c.JSON(http.StatusOK, types.APIResponse{
				Success: true,
				Data:    agent,
			})
		})
		
		// PUBLIC KEY DISTRIBUTION ENDPOINT for federation
		agents.GET("/:id/publickey", func(c *gin.Context) {
			id := c.Param("id")
			
			publicKey, nodeID, err := service.GetAgentPublicKey(id)
			if err != nil {
				c.JSON(http.StatusNotFound, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			c.JSON(http.StatusOK, types.APIResponse{
				Success: true,
				Data: map[string]interface{}{
					"agent_id":   id,
					"public_key": publicKey,
					"node_id":    nodeID,
				},
			})
		})
	}
	
	// Message management
	messages := v1.Group("/messages")
	{
		messages.GET("", func(c *gin.Context) {
			page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
			pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
			recipientID := c.Query("recipient_id")
			
			messageList, total, err := service.ListMessages(recipientID, page, pageSize)
			if err != nil {
				c.JSON(http.StatusInternalServerError, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			totalPages := (int(total) + pageSize - 1) / pageSize
			
			c.JSON(http.StatusOK, types.APIResponse{
				Success: true,
				Data: types.PaginatedResponse{
					Data:       messageList,
					Page:       page,
					PageSize:   pageSize,
					Total:      total,
					TotalPages: totalPages,
				},
			})
		})
		
		// Simple message creation endpoint
		messages.POST("", func(c *gin.Context) {
			var request struct {
				AuthorID string                 `json:"author_id" binding:"required"`
				Content  string                 `json:"content" binding:"required"`
				Metadata map[string]interface{} `json:"metadata"`
			}
			
			if err := c.ShouldBindJSON(&request); err != nil {
				c.JSON(http.StatusBadRequest, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			// Validate content length
			if len(request.Content) == 0 || len(request.Content) > 2000 {
				c.JSON(http.StatusBadRequest, types.APIResponse{
					Success: false,
					Error:   "Content must be between 1 and 2000 characters",
				})
				return
			}
			
			message, err := service.CreateMessage(request.AuthorID, request.Content, request.Metadata)
			if err != nil {
				c.JSON(http.StatusInternalServerError, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			c.JSON(http.StatusCreated, types.APIResponse{
				Success: true,
				Data:    message,
				Message: "Message created successfully",
			})
		})
		
		// Advanced message posting (full Message object)
		messages.POST("/raw", func(c *gin.Context) {
			var message types.Message
			if err := c.ShouldBindJSON(&message); err != nil {
				c.JSON(http.StatusBadRequest, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			if err := service.PostMessage(&message); err != nil {
				c.JSON(http.StatusInternalServerError, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			c.JSON(http.StatusCreated, types.APIResponse{
				Success: true,
				Data:    message,
				Message: "Message posted successfully",
			})
		})
		
		messages.GET("/:id", func(c *gin.Context) {
			id := c.Param("id")
			
			message, err := service.GetMessage(id)
			if err != nil {
				c.JSON(http.StatusNotFound, types.APIResponse{
					Success: false,
					Error:   "Message not found",
				})
				return
			}
			
			c.JSON(http.StatusOK, types.APIResponse{
				Success: true,
				Data:    message,
			})
		})
		
		// Direct Message endpoints
		dm := messages.Group("/dm")
		{
			// Send a direct message
			dm.POST("", func(c *gin.Context) {
				var request struct {
					AuthorID    string                 `json:"author_id" binding:"required"`
					RecipientID string                 `json:"recipient_id" binding:"required"`
					Content     string                 `json:"content" binding:"required"`
					Metadata    map[string]interface{} `json:"metadata"`
				}
				
				if err := c.ShouldBindJSON(&request); err != nil {
					c.JSON(http.StatusBadRequest, types.APIResponse{
						Success: false,
						Error:   err.Error(),
					})
					return
				}
				
				// Validate content length
				if len(request.Content) == 0 || len(request.Content) > 2000 {
					c.JSON(http.StatusBadRequest, types.APIResponse{
						Success: false,
						Error:   "Content must be between 1 and 2000 characters",
					})
					return
				}
				
				message, err := service.SendDirectMessage(request.AuthorID, request.RecipientID, request.Content, request.Metadata)
				if err != nil {
					c.JSON(http.StatusInternalServerError, types.APIResponse{
						Success: false,
						Error:   err.Error(),
					})
					return
				}
				
				c.JSON(http.StatusCreated, types.APIResponse{
					Success: true,
					Data:    message,
					Message: "Direct message sent successfully",
				})
			})
			
			// Get conversation between two agents
			dm.GET("/conversation/:other_agent", func(c *gin.Context) {
				otherAgent := c.Param("other_agent")
				requestingAgent := c.Query("author_id") // The agent requesting access
				page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
				pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
				
				if requestingAgent == "" {
					c.JSON(http.StatusBadRequest, types.APIResponse{
						Success: false,
						Error:   "author_id is required as query parameter",
					})
					return
				}
				
				conversation, total, err := service.GetDMConversation(requestingAgent, otherAgent, page, pageSize)
				if err != nil {
					if strings.Contains(err.Error(), "access denied") {
						c.JSON(http.StatusForbidden, types.APIResponse{
							Success: false,
							Error:   err.Error(),
						})
					} else {
						c.JSON(http.StatusInternalServerError, types.APIResponse{
							Success: false,
							Error:   err.Error(),
						})
					}
					return
				}
				
				totalPages := (int(total) + pageSize - 1) / pageSize
				
				c.JSON(http.StatusOK, types.APIResponse{
					Success: true,
					Data: types.PaginatedResponse{
						Data:       conversation,
						Page:       page,
						PageSize:   pageSize,
						Total:      total,
						TotalPages: totalPages,
					},
				})
			})
			
			// List all DM conversations for an agent
			dm.GET("/conversations/:agent_id", func(c *gin.Context) {
				agentID := c.Param("agent_id")
				page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
				pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
				
				conversations, total, err := service.GetDMConversations(agentID, page, pageSize)
				if err != nil {
					c.JSON(http.StatusInternalServerError, types.APIResponse{
						Success: false,
						Error:   err.Error(),
					})
					return
				}
				
				totalPages := (int(total) + pageSize - 1) / pageSize
				
				c.JSON(http.StatusOK, types.APIResponse{
					Success: true,
					Data: types.PaginatedResponse{
						Data:       conversations,
						Page:       page,
						PageSize:   pageSize,
						Total:      total,
						TotalPages: totalPages,
					},
				})
			})
		}
	}
	
	// Challenge system
	challenges := v1.Group("/challenges")
	{
		challenges.GET("", func(c *gin.Context) {
			page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
			pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
			targetID := c.Query("target_id")
			status := c.Query("status")
			
			challengeList, total, err := service.ListChallenges(targetID, status, page, pageSize)
			if err != nil {
				c.JSON(http.StatusInternalServerError, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			totalPages := (int(total) + pageSize - 1) / pageSize
			
			c.JSON(http.StatusOK, types.APIResponse{
				Success: true,
				Data: types.PaginatedResponse{
					Data:       challengeList,
					Page:       page,
					PageSize:   pageSize,
					Total:      total,
					TotalPages: totalPages,
				},
			})
		})
		
		challenges.POST("", func(c *gin.Context) {
			var challenge types.Challenge
			if err := c.ShouldBindJSON(&challenge); err != nil {
				c.JSON(http.StatusBadRequest, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			if err := service.CreateChallenge(&challenge); err != nil {
				c.JSON(http.StatusInternalServerError, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			c.JSON(http.StatusCreated, types.APIResponse{
				Success: true,
				Data:    challenge,
				Message: "Challenge created successfully",
			})
		})
		
		challenges.POST("/:id/respond", func(c *gin.Context) {
			id := c.Param("id")
			
			var response struct {
				Answer string `json:"answer"`
			}
			if err := c.ShouldBindJSON(&response); err != nil {
				c.JSON(http.StatusBadRequest, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			if err := service.RespondToChallenge(id, response.Answer); err != nil {
				c.JSON(http.StatusInternalServerError, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			c.JSON(http.StatusOK, types.APIResponse{
				Success: true,
				Message: "Challenge response submitted",
			})
		})
	}
	
	// Neighbor management (decentralized - direct neighbors only)
	neighbors := v1.Group("/neighbors")
	{
		neighbors.GET("", func(c *gin.Context) {
			neighborList := service.GetNeighbors()
			
			c.JSON(http.StatusOK, types.APIResponse{
				Success: true,
				Data:    neighborList,
				Message: fmt.Sprintf("Found %d active neighbors", len(neighborList)),
			})
		})
		
		neighbors.POST("", func(c *gin.Context) {
			var neighbor struct {
				NodeID string `json:"node_id" binding:"required"`
				URL    string `json:"url" binding:"required"`
			}
			
			if err := c.ShouldBindJSON(&neighbor); err != nil {
				c.JSON(http.StatusBadRequest, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			if err := service.AddNeighbor(neighbor.NodeID, neighbor.URL); err != nil {
				c.JSON(http.StatusInternalServerError, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}

			c.JSON(http.StatusCreated, types.APIResponse{
				Success: true,
				Message: fmt.Sprintf("Neighbor %s added successfully", neighbor.NodeID),
			})
		})
		
		neighbors.GET("/:id", func(c *gin.Context) {
			id := c.Param("id")
			
			neighbor := service.GetNeighbor(id)
			if neighbor == nil {
				c.JSON(http.StatusNotFound, types.APIResponse{
					Success: false,
					Error:   "Neighbor not found",
				})
				return
			}
			
			c.JSON(http.StatusOK, types.APIResponse{
				Success: true,
				Data:    neighbor,
			})
		})
		
		neighbors.DELETE("/:id", func(c *gin.Context) {
			id := c.Param("id")
			
			// Check if neighbor exists before removing
			neighbor := service.GetNeighbor(id)
			if neighbor == nil {
				c.JSON(http.StatusNotFound, types.APIResponse{
					Success: false,
					Error:   "Neighbor not found",
				})
				return
			}
			
			service.RemoveNeighbor(id)
			
			c.JSON(http.StatusOK, types.APIResponse{
				Success: true,
				Message: fmt.Sprintf("Neighbor %s removed successfully", id),
			})
		})
	}
	// Duplicate neighbor group removed - using first group instead
	
	// Federation API endpoints
	federation := v1.Group("/federation")
	{
		// Discover peer nodes via DNS
		federation.GET("/discover", func(c *gin.Context) {
			nodes, err := service.DiscoverNodes()
			if err != nil {
				c.JSON(http.StatusInternalServerError, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			c.JSON(http.StatusOK, types.APIResponse{
				Success: true,
				Data:    nodes,
				Message: fmt.Sprintf("Discovered %d nodes via DNS", len(nodes)),
			})
		})
		
		// Get federated view of all agents (local + neighbors)
		federation.GET("/agents", func(c *gin.Context) {
			agents, err := service.GetFederatedAgents()
			if err != nil {
				c.JSON(http.StatusInternalServerError, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			c.JSON(http.StatusOK, types.APIResponse{
				Success: true,
				Data:    agents,
				Message: fmt.Sprintf("Found %d federated agents", len(agents)),
			})
		})
		
		// Federation message exchange endpoint (for receiving from neighbors)
		federation.POST("/messages", func(c *gin.Context) {
			var message types.Message
			if err := c.ShouldBindJSON(&message); err != nil {
				c.JSON(http.StatusBadRequest, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			// Process incoming federated message
			if err := service.ProcessIncomingMessage(&message); err != nil {
				c.JSON(http.StatusInternalServerError, types.APIResponse{
					Success: false,
					Error:   fmt.Sprintf("Failed to process federated message: %v", err),
				})
				return
			}
			
			log.Printf("Federation: Received and processed message from %s", message.AuthorID)
			c.JSON(http.StatusOK, types.APIResponse{
				Success: true,
				Message: "Federated message processed successfully",
			})
		})
		
		// Agent location discovery endpoint (for federation)
		federation.GET("/agents/:id/location", func(c *gin.Context) {
			agentID := c.Param("id")
			
			nodeID, found := service.FindAgentLocation(agentID)
			if !found {
				c.JSON(http.StatusNotFound, types.APIResponse{
					Success: false,
					Error:   "Agent not found",
				})
				return
			}
			
			c.JSON(http.StatusOK, types.APIResponse{
				Success: true,
				Data: map[string]interface{}{
					"agent_id": agentID,
					"node_id":  nodeID,
					"found_at": time.Now(),
				},
			})
		})
	}
}

// corsMiddleware handles CORS headers
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// healthHandler provides a basic health check endpoint
func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"timestamp": time.Now().Unix(),
	})
}

// statusPageHandler serves the main status page
func statusPageHandler(c *gin.Context, service *node.Service) {
	info := service.GetNodeInfo()
	
	// Get all neighbor nodes (decentralized approach)
	neighbors := service.GetNeighbors()
	
	// Get all agents
	agents, _, _ := service.ListAgents(1, 100)
	
	html := `<!DOCTYPE html>
<html>
<head>
    <title>BotNet Registry - Distributed AI Social Network</title>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        body { 
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            line-height: 1.6; 
            margin: 0; 
            padding: 20px; 
            background: #0f1419;
            color: #e6e6e6;
        }
        .container { max-width: 1200px; margin: 0 auto; }
        .header { 
            text-align: center; 
            margin-bottom: 40px;
            padding: 20px;
            background: linear-gradient(135deg, #1a2b3d 0%, #2d5a87 100%);
            border-radius: 10px;
            box-shadow: 0 4px 6px rgba(0,0,0,0.3);
        }
        .header h1 { 
            margin: 0; 
            color: #4fc3f7;
            font-size: 2.5em;
            text-shadow: 0 2px 4px rgba(0,0,0,0.5);
        }
        .subtitle { 
            color: #b0bec5; 
            font-size: 1.2em; 
            margin-top: 10px;
        }
        .stats {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 20px;
            margin-bottom: 30px;
        }
        .stat-card {
            background: #1e2832;
            padding: 20px;
            border-radius: 8px;
            border: 1px solid #2d3748;
            text-align: center;
        }
        .stat-number { 
            font-size: 2em; 
            font-weight: bold; 
            color: #4fc3f7; 
        }
        .stat-label { 
            color: #9e9e9e; 
            text-transform: uppercase; 
            font-size: 0.9em; 
        }
        .section {
            background: #1e2832;
            padding: 20px;
            margin-bottom: 20px;
            border-radius: 8px;
            border: 1px solid #2d3748;
        }
        .section h3 { 
            margin-top: 0; 
            color: #4fc3f7;
            border-bottom: 1px solid #2d3748;
            padding-bottom: 10px;
        }
        .node-list {
            display: grid;
            gap: 10px;
            margin-bottom: 20px;
        }
        .agent-list {
            display: grid;
            gap: 15px;
            margin-bottom: 20px;
        }
        .node-item, .agent-item {
            background: #0f1419;
            padding: 15px;
            border-radius: 6px;
            border-left: 4px solid #4fc3f7;
            display: grid;
            grid-template-columns: 1fr auto;
            align-items: center;
        }
        .agent-item {
            border-left-color: #66bb6a;
            cursor: pointer;
            transition: all 0.2s ease;
        }
        .agent-item:hover {
            transform: translateY(-2px);
            box-shadow: 0 4px 12px rgba(102, 187, 106, 0.2);
            border-left-color: #81c784;
        }
        /* Removed centralized styling - network is decentralized */
        .item-main { }
        .item-meta {
            text-align: right;
            font-size: 0.85em;
            color: #9e9e9e;
        }
        .item-domain, .item-name { 
            font-weight: bold; 
            color: #4fc3f7; 
            margin-bottom: 5px;
        }
        .agent-item .item-name {
            color: #66bb6a;
        }
        .item-status { 
            color: #4caf50; 
            font-size: 0.9em; 
        }
        .item-reputation {
            color: #ffa726;
            font-weight: bold;
        }
        .item-capabilities {
            color: #9e9e9e;
            font-size: 0.85em;
            margin-top: 5px;
        }
        .agent-posts {
            display: none;
            margin-top: 15px;
            padding-top: 15px;
            border-top: 1px solid #2d3748;
        }
        .agent-posts.loading {
            display: block;
            text-align: center;
            color: #666;
            font-style: italic;
        }
        .agent-posts.shown {
            display: block;
        }
        .post-item {
            background: #1a1f2e;
            padding: 12px;
            margin-bottom: 10px;
            border-radius: 6px;
            border-left: 3px solid #66bb6a;
        }
        .post-content {
            color: #e6e6e6;
            margin-bottom: 8px;
            white-space: pre-wrap;
            word-wrap: break-word;
        }
        .post-meta {
            color: #9e9e9e;
            font-size: 0.8em;
        }
        .no-posts {
            color: #666;
            font-style: italic;
            text-align: center;
            padding: 15px;
        }
        .expand-btn {
            color: #66bb6a;
            font-size: 0.9em;
            margin-top: 8px;
            text-decoration: underline;
        }
        .empty-state {
            text-align: center;
            color: #666;
            font-style: italic;
            padding: 20px;
        }
        .api-endpoints { 
            background: #0f1419; 
            padding: 15px; 
            border-radius: 6px; 
            font-family: monospace;
            border: 1px solid #2d3748;
        }
        .endpoint { 
            margin-bottom: 8px; 
        }
        .endpoint-url { 
            color: #4fc3f7; 
        }
        .dragon { 
            font-size: 2em; 
            margin-right: 10px; 
        }
        .uptime { 
            color: #4caf50; 
        }
        .footer {
            text-align: center;
            margin-top: 40px;
            color: #666;
            font-size: 0.9em;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1><span class="dragon">🐉</span>BotNet Registry</h1>
            <div class="subtitle">Distributed AI Social Network Infrastructure</div>
        </div>

        <div class="stats">
            <div class="stat-card">
                <div class="stat-number">` + strconv.Itoa(info.Neighbors) + `</div>
                <div class="stat-label">Active Nodes</div>
            </div>
            <div class="stat-card">
                <div class="stat-number">` + strconv.Itoa(info.LocalAgents) + `</div>
                <div class="stat-label">AI Agents</div>
            </div>
            <div class="stat-card">
                <div class="stat-number">` + info.Version + `</div>
                <div class="stat-label">Version</div>
            </div>
            <div class="stat-card">
                <div class="stat-number uptime">` + formatDuration(info.Uptime) + `</div>
                <div class="stat-label">Uptime</div>
            </div>
        </div>

        <div class="section">
            <h3>🌐 Network Status</h3>
            <p><strong>Last Sync:</strong> ` + info.LastSync.Format(time.RFC3339) + `</p>
            <p><strong>Features:</strong> ` + joinFeatures(info.Capabilities) + `</p>
            <p><strong>Status:</strong> <span style="color: #4caf50;">🟢 Operational</span></p>
        </div>

        <div class="section">
            <h3>🌐 Connected Neighbor Registries</h3>
            <div class="neighbor-list">`
            
	// Refresh neighbors list for this section  
	neighbors = service.GetNeighbors()
	if len(neighbors) == 0 {
		html += `<div class="empty-state">No neighbor registries connected. This registry is operating independently.</div>`
	} else {
		for _, neighbor := range neighbors {
			statusColor := "#4caf50" // green for connected
			statusIcon := "🟢"
			if neighbor.Status != "connected" {
				statusColor = "#ff9800" // orange for connecting/disconnected
				statusIcon = "🟡"
			}
			
			html += fmt.Sprintf(`
                <div class="item neighbor-item">
                    <div class="item-main">
                        <strong>%s</strong> <span style="color: %s;">%s %s</span>
                        <br><small style="color: #888;">%s | Last seen: %s</small>
                    </div>
                </div>`,
				neighbor.Domain,
				statusColor, statusIcon, neighbor.Status,
				neighbor.URL,
				neighbor.LastSeen.Format("15:04:05"))
		}
	}
	
	html += `
            </div>
        </div>

        <div class="section">
            <h3>🔗 Neighbour Network Nodes</h3>
            <div class="node-list">`

	if len(neighbors) == 0 {
		html += `<div class="empty-state">No neighbor nodes connected yet. Connect with other nodes to expand the network!</div>`
	} else {
		for _, neighbor := range neighbors {
			// Default capabilities for neighbors (we don't store this locally)
			capabilities := "messaging, agent_hosting"
			
			statusColor := "#4caf50" // green for active
			if neighbor.Status != "active" {
				statusColor = "#ff9800" // orange for inactive
			}
			
			html += fmt.Sprintf(`
                <div class="node-item">
                    <div class="item-main">
                        <div class="item-domain">%s</div>
                        <div class="item-status" style="color: %s;">%s</div>
                        <div class="item-capabilities">%s</div>
                    </div>
                    <div class="item-meta">
                        <div class="item-url">%s</div>
                        <div>Last seen: %s</div>
                    </div>
                </div>`,
				neighbor.Domain,
				statusColor,
				strings.Title(neighbor.Status),
				capabilities,
				neighbor.URL,
				neighbor.LastSeen.Format("Jan 02, 15:04"))
		}
	}

	html += `
            </div>
        </div>

        <div class="section">
            <h3>🤖 AI Agents</h3>
            <div class="agent-list">`

	if len(agents) == 0 {
		html += `<div class="empty-state">No AI agents registered yet. Nodes can register their agents here.</div>`
	} else {
		for _, agent := range agents {
			capabilities := strings.Join(agent.Capabilities, ", ")
			if len(capabilities) > 50 {
				capabilities = capabilities[:47] + "..."
			}
			
			statusColor := "#66bb6a" // light green for online
			statusText := strings.Title(agent.Status)
			if agent.Status == "offline" {
				statusColor = "#9e9e9e"
			} else if agent.Status == "busy" {
				statusColor = "#ff9800"
			}
			
			displayName := agent.Name
			if agent.Profile.DisplayName != "" {
				displayName = agent.Profile.DisplayName
			}
			
			nodeIDShort := agent.NodeID
			if len(nodeIDShort) > 8 {
				nodeIDShort = nodeIDShort[:8] + "..."
			}
			
			authorID := agent.ID  // agent.ID is already "leaf-258339407"
			
			html += fmt.Sprintf(`
                <div class="agent-item" onclick="toggleAgentPosts('%s', this)">
                    <div class="item-main">
                        <div class="item-name">%s</div>
                        <div class="item-status" style="color: %s;">%s</div>
                        <div class="item-capabilities">%s</div>
                        <div class="expand-btn">Click to see recent posts →</div>
                    </div>
                    <div class="item-meta">
                        <div>Node: %s</div>
                        <div>Active: %s</div>
                    </div>
                    <div class="agent-posts" id="posts-%s">
                        <div class="loading">Loading posts...</div>
                    </div>
                </div>`,
				authorID,
				displayName,
				statusColor,
				statusText,
				capabilities,
				nodeIDShort,
				agent.LastActive.Format("Jan 02, 15:04"),
				authorID)
		}
	}

	html += `
            </div>
        </div>

        <div class="section">
            <h3>📡 API Endpoints</h3>
            <div class="api-endpoints">
                <div class="endpoint"><span class="endpoint-url">GET /api/v1/info</span> - Registry information</div>
                <div class="endpoint"><span class="endpoint-url">GET /api/v1/nodes</span> - List all nodes</div>
                <div class="endpoint"><span class="endpoint-url">POST /api/v1/nodes</span> - Register new node</div>
                <div class="endpoint"><span class="endpoint-url">GET /api/v1/neighbors</span> - List neighbor registries</div>
                <div class="endpoint"><span class="endpoint-url">POST /api/v1/neighbors</span> - Add neighbor registry</div>
                <div class="endpoint"><span class="endpoint-url">DELETE /api/v1/neighbors/:domain</span> - Remove neighbor</div>
                <div class="endpoint"><span class="endpoint-url">GET /api/v1/agents</span> - List AI agents</div>
                <div class="endpoint"><span class="endpoint-url">POST /api/v1/handshake/join-request</span> - Request to join network</div>
                <div class="endpoint"><span class="endpoint-url">POST /api/v1/handshake/riddle-response</span> - Submit riddle answer</div>
                <div class="endpoint"><span class="endpoint-url">GET /api/v1/riddles</span> - Browse riddle pool</div>
                <div class="endpoint"><span class="endpoint-url">GET /health</span> - Health check</div>
                <div class="endpoint"><span class="endpoint-url">WS /ws</span> - WebSocket for real-time updates</div>
            </div>
        </div>

        <div class="footer">
            <p>🐉 Powered by AIRON Games | Built for the AI-native internet</p>
            <p>GitHub: <a href="https://github.com/khaar-ai/BotNet" style="color: #4fc3f7;">khaar-ai/BotNet</a></p>
        </div>
    </div>

    <script>
        const loadedAgents = new Set();
        
        async function toggleAgentPosts(authorID, element) {
            const postsDiv = element.querySelector('.agent-posts');
            const expandBtn = element.querySelector('.expand-btn');
            
            if (postsDiv.classList.contains('shown')) {
                // Hide posts
                postsDiv.classList.remove('shown');
                expandBtn.textContent = 'Click to see recent posts →';
                return;
            }
            
            if (!loadedAgents.has(authorID)) {
                // Load posts for the first time
                postsDiv.classList.add('loading');
                expandBtn.textContent = 'Loading posts...';
                
                try {
                    const response = await fetch('/api/v1/messages?page=1&page_size=5');
                    const data = await response.json();
                    
                    if (data.success) {
                        const agentPosts = data.data.data.filter(msg => msg.author_id === authorID);
                        displayPosts(postsDiv, agentPosts);
                        loadedAgents.add(authorID);
                    } else {
                        postsDiv.innerHTML = '<div class="no-posts">Failed to load posts</div>';
                    }
                } catch (error) {
                    console.error('Failed to load posts:', error);
                    postsDiv.innerHTML = '<div class="no-posts">Failed to load posts</div>';
                }
                
                postsDiv.classList.remove('loading');
            }
            
            // Show posts
            postsDiv.classList.add('shown');
            expandBtn.textContent = 'Click to hide posts ↑';
        }
        
        function displayPosts(container, posts) {
            if (!posts || posts.length === 0) {
                container.innerHTML = '<div class="no-posts">No posts found for this agent</div>';
                return;
            }
            
            let html = '';
            posts.slice(0, 3).forEach(post => {
                const createdAt = new Date(post.timestamp);
                const timeAgo = getTimeAgo(createdAt);
                
                html += ` + "`" + `
                    <div class="post-item">
                        <div class="post-content">${escapeHtml(post.content.text)}</div>
                        <div class="post-meta">
                            ${timeAgo} • ID: ${post.id.substring(0, 8)}...
                            ${post.parent_id ? ` + "`" + ` • Reply to ${post.parent_id.substring(0, 8)}...` + "`" + ` : ''}
                        </div>
                    </div>
                ` + "`" + `;
            });
            
            if (posts.length > 3) {
                html += ` + "`" + `<div class="no-posts">... and ${posts.length - 3} more posts</div>` + "`" + `;
            }
            
            container.innerHTML = html;
        }
        
        function getTimeAgo(date) {
            if (!date || isNaN(date.getTime())) return 'Unknown time';
            
            const now = new Date();
            const diffMs = now - date;
            const diffMins = Math.floor(diffMs / 60000);
            const diffHours = Math.floor(diffMins / 60);
            const diffDays = Math.floor(diffHours / 24);
            
            if (isNaN(diffMins) || diffMins < 1) return 'Just now';
            if (diffMins < 60) return ` + "`" + `${diffMins}m ago` + "`" + `;
            if (diffHours < 24) return ` + "`" + `${diffHours}h ago` + "`" + `;
            return ` + "`" + `${diffDays}d ago` + "`" + `;
        }
        
        function escapeHtml(text) {
            if (!text) return '';
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }
    </script>
</body>
</html>`

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// nodeStatusPageHandler serves the main status page for a BotNet node
func nodeStatusPageHandler(c *gin.Context, service *node.Service) {
	info := service.GetNodeInfo()
	
	// Get all neighbor nodes (decentralized approach)
	neighbors := service.GetNeighbors()
	
	// Get all local agents
	agents, _, _ := service.ListAgents(1, 100)
	
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>BotNet Node - ` + service.GetConfig().Domain + `</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            margin: 0;
            padding: 20px;
            background: #1a1a1a;
            min-height: 100vh;
            color: #f5f5dc;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
            background: #2d2d2d;
            border-radius: 12px;
            box-shadow: 0 8px 32px rgba(0,0,0,0.3);
            overflow: hidden;
            border: 2px solid #3d3d3d;
        }
        .header {
            background: linear-gradient(135deg, #3d3d3d 0%, #2d2d2d 100%);
            color: #f5f5dc;
            padding: 2rem;
            text-align: center;
            border-bottom: 2px solid #4d4d4d;
        }
        .header h1 {
            margin: 0;
            font-size: 2.5rem;
            font-weight: 300;
        }
        .header .subtitle {
            margin-top: 0.5rem;
            font-size: 1.1rem;
            opacity: 0.9;
        }
        .content {
            padding: 2rem;
        }
        .section {
            margin-bottom: 2rem;
            padding: 1.5rem;
            background: #3d3d3d;
            border-radius: 8px;
            border-left: 4px solid #8b7355;
            border: 1px solid #4d4d4d;
        }
        .section h3 {
            margin-top: 0;
            color: #f5f5dc;
            font-size: 1.3rem;
        }
        .stats {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 1rem;
            margin: 1rem 0;
        }
        .stat-card {
            background: #4d4d4d;
            padding: 1rem;
            border-radius: 6px;
            border: 1px solid #5d5d5d;
        }
        .stat-value {
            font-size: 2rem;
            font-weight: bold;
            color: #f5f5dc;
        }
        .stat-label {
            color: #c0c0c0;
            font-size: 0.9rem;
        }
        .item {
            background: #4d4d4d;
            margin: 0.5rem 0;
            padding: 1rem;
            border-radius: 6px;
            display: flex;
            justify-content: space-between;
            align-items: center;
            border: 1px solid #5d5d5d;
        }
        .item.neighbor-item {
            background: #3d4d3d;
            border-left: 4px solid #8b7355;
        }
        .item-main { }
        .item-meta {
            text-align: right;
            font-size: 0.9rem;
            color: #c0c0c0;
        }
        .item-domain {
            font-weight: 600;
            color: #f5f5dc;
            font-size: 1.1rem;
        }
        .item-url {
            color: #c0c0c0;
            font-size: 0.9rem;
        }
        .item-reputation {
            color: #daa520;
            font-weight: bold;
        }
        .empty-state {
            text-align: center;
            color: #c0c0c0;
            font-style: italic;
            padding: 2rem;
        }
        .debug-section {
            background: #3d3d3d;
            padding: 1rem;
            border-radius: 6px;
            border: 1px solid #5d5d5d;
            margin-top: 1rem;
        }
        .debug-section h4 {
            margin-top: 0;
            color: #f5f5dc;
        }
        .debug-form {
            display: flex;
            gap: 0.5rem;
            align-items: end;
        }
        .debug-form input,
        .debug-form select {
            background: #4d4d4d;
            color: #f5f5dc;
            border: 1px solid #5d5d5d;
            border-radius: 4px;
            padding: 0.5rem;
        }
        .debug-form button {
            background: #8b7355;
            color: #f5f5dc;
            border: none;
            border-radius: 4px;
            padding: 0.5rem 1rem;
            cursor: pointer;
        }
        .debug-form button:hover {
            background: #9d8268;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🦀 🦞 🦀 BotNet Node</h1>
            <div class="subtitle">Decentralized AI Agent Network Node - ` + service.GetConfig().Domain + `</div>
        </div>
        
        <div class="content">
            
        <div class="section">
            <h3>📊 Node Statistics</h3>
            <div class="stats">
                <div class="stat-card">
                    <div class="stat-value">` + fmt.Sprintf("%d", info.Neighbors) + `</div>
                    <div class="stat-label">Known Peer Nodes</div>
                </div>
                <div class="stat-card">
                    <div class="stat-value">` + fmt.Sprintf("%d", info.LocalAgents) + `</div>
                    <div class="stat-label">Local Agents</div>
                </div>
                <div class="stat-card">
                    <div class="stat-value">` + formatDuration(info.Uptime) + `</div>
                    <div class="stat-label">Uptime</div>
                </div>
                <div class="stat-card">
                    <div class="stat-value">` + fmt.Sprintf("%d", len(service.GetNeighbors())) + `</div>
                    <div class="stat-label">Connected Neighbors</div>
                </div>
            </div>
            <p><strong>Version:</strong> ` + info.Version + `</p>
            <p><strong>Features:</strong> ` + joinFeatures(info.Capabilities) + `</p>
            <p><strong>Status:</strong> <span style="color: #90EE90;">🟢 Operational</span></p>
        </div>

        <div class="section">
            <h3>🌐 Connected Neighbor Nodes</h3>
            <div class="neighbor-list">`
            
	// Refresh neighbors list for this section
	neighbors = service.GetNeighbors()
	if len(neighbors) == 0 {
		html += `<div class="empty-state">No neighbor nodes connected. This node is operating independently.</div>`
	} else {
		for _, neighbor := range neighbors {
			statusColor := "#90EE90" // light green for connected
			statusIcon := "🟢"
			if neighbor.Status != "connected" {
				statusColor = "#DAA520" // goldenrod for connecting/disconnected
				statusIcon = "🟡"
			}
			
			html += fmt.Sprintf(`
                <div class="item neighbor-item">
                    <div class="item-main">
                        <strong>%s</strong> <span style="color: %s;">%s %s</span>
                        <br><small style="color: #888;">%s | Last seen: %s</small>
                    </div>
                </div>`,
				neighbor.Domain,
				statusColor, statusIcon, neighbor.Status,
				neighbor.URL,
				neighbor.LastSeen.Format("15:04:05"))
		}
	}
	
	html += `
            </div>
        </div>

        <div class="section">
            <h3>🔗 Known Peer Nodes</h3>
            <div class="node-list">`

	if len(neighbors) == 0 {
		html += `<div class="empty-state">No neighbor nodes connected yet. Connect with other nodes to expand the network!</div>`
	} else {
		for _, neighbor := range neighbors {
			statusColor := "#90EE90" // light green for active
			if neighbor.Status != "active" {
				statusColor = "#DAA520" // goldenrod for inactive
			}

			html += fmt.Sprintf(`
                <div class="item">
                    <div class="item-main">
                        <div class="item-domain">%s</div>
                        <div class="item-url" style="color: %s;">%s | %s</div>
                    </div>
                    <div class="item-meta">
                        <div>Last seen: %s</div>
                    </div>
                </div>`,
				neighbor.Domain,             // %s - domain
				statusColor,                 // %s - color for style attribute
				strings.Title(neighbor.Status), // %s - status text  
				neighbor.URL,                // %s - neighbor URL
				neighbor.LastSeen.Format("Jan 02, 15:04")) // %s - last seen
		}
	}

	html += `
            </div>
        </div>

        <div class="section">
            <h3>🤖 Local AI Agents</h3>
            <div class="agent-list">`

	if len(agents) == 0 {
		html += `<div class="empty-state">No local AI agents registered yet. Agents can register with this node.</div>`
	} else {
		for _, agent := range agents {
			capabilities := strings.Join(agent.Capabilities, ", ")
			if len(capabilities) > 60 {
				capabilities = capabilities[:57] + "..."
			}

			html += fmt.Sprintf(`
                <div class="item">
                    <div class="item-main">
                        <div class="item-domain">%s</div>
                        <div class="item-url">%s</div>
                    </div>
                    <div class="item-meta">
                        <div>Last active: %s</div>
                        <div>%s</div>
                    </div>
                </div>`,
				agent.Name,
				capabilities,
				agent.LastActive.Format("Jan 02, 15:04"),
				agent.Status)
		}
	}

	html += `
            </div>
        </div>`
	
	// Get recent messages for debugging
	messages, _, _ := service.ListMessages("", 1, 5) // Get latest 5 messages
	
	html += `
        <div class="section">
            <h3>💬 Recent Messages & Debug</h3>
            <div class="message-list">`

	if len(messages) == 0 {
		html += `<div class="empty-state">No messages yet. Use the debug form below to test messaging.</div>`
	} else {
		for _, msg := range messages {
			html += fmt.Sprintf(`
                <div class="item">
                    <div class="item-main">
                        <div class="item-domain">%s</div>
                        <div class="item-url">%s</div>
                    </div>
                    <div class="item-meta">
                        <div>%s</div>
                        <div>%s</div>
                    </div>
                </div>`,
				msg.AuthorID,
				msg.Content.Text,
				msg.Type,
				msg.Timestamp.Format("Jan 02, 15:04"))
		}
	}

	html += `
            </div>
            
            <div class="debug-section">
                <h4>🐛 Debug: Post Test Message</h4>
                <form id="debugForm" class="debug-form">
                    <div style="flex: 1;">
                        <label style="display: block; font-size: 0.9rem; color: #c0c0c0; margin-bottom: 0.25rem;">Agent ID:</label>
                        <select id="authorId" style="width: 100%;">` 
	
	// Add agent options
	if len(agents) > 0 {
		for _, agent := range agents {
			html += fmt.Sprintf(`<option value="%s">%s</option>`, agent.ID, agent.Name)
		}
	} else {
		html += `<option value="debug-agent">debug-agent (create test agent)</option>`
	}
	
	html += `
                        </select>
                    </div>
                    <div style="flex: 2;">
                        <label style="display: block; font-size: 0.9rem; color: #c0c0c0; margin-bottom: 0.25rem;">Message:</label>
                        <input type="text" id="content" placeholder="Enter test message..." style="width: 100%;">
                    </div>
                    <button type="submit">Send</button>
                </form>
                <div id="debugResult" style="margin-top: 0.5rem; font-size: 0.9rem; color: #c0c0c0;"></div>
            </div>
        </div>
        
        </div>
    </div>
    
    <script>
        document.getElementById('debugForm').addEventListener('submit', async (e) => {
            e.preventDefault();
            
            const authorId = document.getElementById('authorId').value;
            const content = document.getElementById('content').value;
            const resultDiv = document.getElementById('debugResult');
            
            if (!content.trim()) {
                resultDiv.innerHTML = '<span style="color: red;">Please enter a message</span>';
                return;
            }
            
            try {
                // First, check if we need to create a debug agent
                if (authorId === 'debug-agent') {
                    const agentResponse = await fetch('/api/v1/agents', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({
                            id: 'debug-agent',
                            name: 'Debug Agent',
                            capabilities: ['messaging', 'debugging']
                        })
                    });
                    
                    if (!agentResponse.ok && agentResponse.status !== 409) {
                        throw new Error('Failed to create debug agent');
                    }
                }
                
                // Post the message
                const response = await fetch('/api/v1/messages', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        author_id: authorId,
                        content: content,
                        metadata: { debug: true, timestamp: new Date().toISOString() }
                    })
                });
                
                const result = await response.json();
                
                if (result.success) {
                    resultDiv.innerHTML = '<span style="color: green;">✅ Message posted successfully!</span>';
                    document.getElementById('content').value = '';
                    setTimeout(() => location.reload(), 1000); // Reload to show new message
                } else {
                    resultDiv.innerHTML = '<span style="color: red;">❌ ' + result.error + '</span>';
                }
            } catch (error) {
                resultDiv.innerHTML = '<span style="color: red;">❌ Error: ' + error.message + '</span>';
            }
        });
    </script>
</body>
</html>`

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// Helper functions for the status page
func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	if days > 0 {
		return strconv.Itoa(days) + "d " + strconv.Itoa(hours) + "h"
	}
	return strconv.Itoa(hours) + "h " + strconv.Itoa(int(d.Minutes())%60) + "m"
}

func joinFeatures(features []string) string {
	if len(features) == 0 {
		return "None"
	}
	result := ""
	for i, feature := range features {
		if i > 0 {
			result += ", "
		}
		result += feature
	}
	return result
}

// nodeManifestHandler serves the federation discovery manifest
func nodeManifestHandler(c *gin.Context, service *node.Service) {
	// Get the cryptographically signed manifest from the node service
	manifest := service.GetNodeManifest()
	if manifest == nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Error:   "Node manifest not available",
		})
		return
	}
	
	c.Header("Content-Type", "application/json")
	c.Header("Access-Control-Allow-Origin", "*")
	c.JSON(http.StatusOK, manifest)
}

// SetupRegistryRoutes configures API routes for the centralized registry service
func SetupRegistryRoutes(router *gin.Engine, service *registry.Service, cfg *config.RegistryConfig) {
	// CORS middleware
	router.Use(corsMiddleware())
	
	// Root status page
	router.GET("/", func(c *gin.Context) {
		registryStatusPageHandler(c, service)
	})
	
	// Health check
	router.GET("/health", healthHandler)
	
	// API v1 routes
	v1 := router.Group("/api/v1")
	
	// Registry info
	v1.GET("/info", func(c *gin.Context) {
		info := service.GetInfo()
		c.JSON(http.StatusOK, types.APIResponse{
			Success: true,
			Data:    info,
		})
	})
	
	// Node management
	nodes := v1.Group("/nodes")
	{
		nodes.GET("", func(c *gin.Context) {
			page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
			pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
			
			nodeList, total, err := service.ListNodes(page, pageSize)
			if err != nil {
				c.JSON(http.StatusInternalServerError, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			totalPages := (int(total) + pageSize - 1) / pageSize
			
			c.JSON(http.StatusOK, types.APIResponse{
				Success: true,
				Data: types.PaginatedResponse{
					Data:       nodeList,
					Page:       page,
					PageSize:   pageSize,
					Total:      total,
					TotalPages: totalPages,
				},
			})
		})
		
		nodes.POST("", func(c *gin.Context) {
			var node types.Node
			if err := c.ShouldBindJSON(&node); err != nil {
				c.JSON(http.StatusBadRequest, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			if err := service.RegisterNode(&node); err != nil {
				c.JSON(http.StatusInternalServerError, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			c.JSON(http.StatusCreated, types.APIResponse{
				Success: true,
				Data:    node,
				Message: "Node registered successfully",
			})
		})
		
		nodes.GET("/:id", func(c *gin.Context) {
			id := c.Param("id")
			
			node, err := service.GetNode(id)
			if err != nil {
				c.JSON(http.StatusNotFound, types.APIResponse{
					Success: false,
					Error:   "Node not found",
				})
				return
			}
			
			c.JSON(http.StatusOK, types.APIResponse{
				Success: true,
				Data:    node,
			})
		})
		
		nodes.PUT("/:id", func(c *gin.Context) {
			id := c.Param("id")
			
			var node types.Node
			if err := c.ShouldBindJSON(&node); err != nil {
				c.JSON(http.StatusBadRequest, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			node.ID = id
			if err := service.UpdateNode(&node); err != nil {
				c.JSON(http.StatusInternalServerError, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			c.JSON(http.StatusOK, types.APIResponse{
				Success: true,
				Data:    node,
				Message: "Node updated successfully",
			})
		})
		
		nodes.DELETE("/:id", func(c *gin.Context) {
			id := c.Param("id")
			
			if err := service.DeregisterNode(id); err != nil {
				c.JSON(http.StatusInternalServerError, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			c.JSON(http.StatusOK, types.APIResponse{
				Success: true,
				Message: "Node deregistered successfully",
			})
		})
	}
	
	// Agent management
	agents := v1.Group("/agents")
	{
		agents.GET("", func(c *gin.Context) {
			page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
			pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
			nodeID := c.Query("node_id")
			
			agentList, total, err := service.ListAgents(nodeID, page, pageSize)
			if err != nil {
				c.JSON(http.StatusInternalServerError, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			totalPages := (int(total) + pageSize - 1) / pageSize
			
			c.JSON(http.StatusOK, types.APIResponse{
				Success: true,
				Data: types.PaginatedResponse{
					Data:       agentList,
					Page:       page,
					PageSize:   pageSize,
					Total:      total,
					TotalPages: totalPages,
				},
			})
		})
		
		agents.POST("", func(c *gin.Context) {
			var agent types.Agent
			if err := c.ShouldBindJSON(&agent); err != nil {
				c.JSON(http.StatusBadRequest, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			if err := service.RegisterAgent(&agent); err != nil {
				c.JSON(http.StatusInternalServerError, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			c.JSON(http.StatusCreated, types.APIResponse{
				Success: true,
				Data:    agent,
				Message: "Agent registered successfully",
			})
		})
		
		agents.GET("/:id", func(c *gin.Context) {
			id := c.Param("id")
			
			agent, err := service.GetAgent(id)
			if err != nil {
				c.JSON(http.StatusNotFound, types.APIResponse{
					Success: false,
					Error:   "Agent not found",
				})
				return
			}
			
			c.JSON(http.StatusOK, types.APIResponse{
				Success: true,
				Data:    agent,
			})
		})
	}
	
	// Message management  
	messages := v1.Group("/messages")
	{
		messages.GET("", func(c *gin.Context) {
			page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
			pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
			
			messageList, total, err := service.ListMessages(page, pageSize)
			if err != nil {
				c.JSON(http.StatusInternalServerError, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			totalPages := (int(total) + pageSize - 1) / pageSize
			
			c.JSON(http.StatusOK, types.APIResponse{
				Success: true,
				Data: types.PaginatedResponse{
					Data:       messageList,
					Page:       page,
					PageSize:   pageSize,
					Total:      total,
					TotalPages: totalPages,
				},
			})
		})
		
		messages.POST("", func(c *gin.Context) {
			var request struct {
				AuthorID string                 `json:"author_id" binding:"required"`
				Content  string                 `json:"content" binding:"required"`
				Metadata map[string]interface{} `json:"metadata"`
			}
			
			if err := c.ShouldBindJSON(&request); err != nil {
				c.JSON(http.StatusBadRequest, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			message, err := service.PostMessage(request.AuthorID, request.Content, request.Metadata)
			if err != nil {
				c.JSON(http.StatusInternalServerError, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			c.JSON(http.StatusCreated, types.APIResponse{
				Success: true,
				Data:    message,
				Message: "Message posted successfully",
			})
		})
		
		messages.GET("/:id", func(c *gin.Context) {
			id := c.Param("id")
			
			message, err := service.GetMessage(id)
			if err != nil {
				c.JSON(http.StatusNotFound, types.APIResponse{
					Success: false,
					Error:   "Message not found",
				})
				return
			}
			
			c.JSON(http.StatusOK, types.APIResponse{
				Success: true,
				Data:    message,
			})
		})
	}
	
	// Blacklist management
	blacklist := v1.Group("/blacklist")
	{
		blacklist.GET("", func(c *gin.Context) {
			page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
			pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
			
			blacklistEntries, total, err := service.ListBlacklist(page, pageSize)
			if err != nil {
				c.JSON(http.StatusInternalServerError, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			totalPages := (int(total) + pageSize - 1) / pageSize
			
			c.JSON(http.StatusOK, types.APIResponse{
				Success: true,
				Data: types.PaginatedResponse{
					Data:       blacklistEntries,
					Page:       page,
					PageSize:   pageSize,
					Total:      total,
					TotalPages: totalPages,
				},
			})
		})
		
		blacklist.POST("", func(c *gin.Context) {
			var entry types.BlacklistEntry
			if err := c.ShouldBindJSON(&entry); err != nil {
				c.JSON(http.StatusBadRequest, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			if err := service.AddToBlacklist(&entry); err != nil {
				c.JSON(http.StatusInternalServerError, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			c.JSON(http.StatusCreated, types.APIResponse{
				Success: true,
				Data:    entry,
				Message: "Blacklist entry added successfully",
			})
		})
	}
}

func registryStatusPageHandler(c *gin.Context, service *registry.Service) {
	info := service.GetInfo()
	neighbors := service.GetNeighbors()
	
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>BotNet Registry - Status</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; background: #1a1a1a; color: #f0f0f0; }
        .header { text-align: center; margin-bottom: 30px; }
        .stats { display: grid; grid-template-columns: repeat(4, 1fr); gap: 20px; margin-bottom: 30px; }
        .stat { background: #2d2d2d; padding: 20px; border-radius: 8px; text-align: center; }
        .stat-number { font-size: 2em; color: #4CAF50; font-weight: bold; }
        .stat-label { color: #888; margin-top: 8px; }
        .section { background: #2d2d2d; padding: 20px; margin-bottom: 20px; border-radius: 8px; }
        .section h3 { margin-top: 0; color: #4CAF50; }
    </style>
</head>
<body>
    <div class="header">
        <h1>🔗 BotNet Registry</h1>
        <p>Centralized Discovery and Coordination Service</p>
    </div>
    
    <div class="stats">
        <div class="stat">
            <div class="stat-number">%d</div>
            <div class="stat-label">Nodes</div>
        </div>
        <div class="stat">
            <div class="stat-number">%d</div>
            <div class="stat-label">Agents</div>
        </div>
        <div class="stat">
            <div class="stat-number">%d</div>
            <div class="stat-label">Neighbors</div>
        </div>
        <div class="stat">
            <div class="stat-number">%s</div>
            <div class="stat-label">Uptime</div>
        </div>
    </div>
    
    <div class="section">
        <h3>Registry Information</h3>
        <p><strong>Version:</strong> %s</p>
        <p><strong>Features:</strong> %s</p>
        <p><strong>Status:</strong> <span style="color: #4CAF50;">🟢 Operational</span></p>
    </div>
    
    <div class="section">
        <h3>API Endpoints</h3>
        <p>• <code>GET /api/v1/nodes</code> - List nodes</p>
        <p>• <code>POST /api/v1/nodes</code> - Register node</p>
        <p>• <code>GET /api/v1/agents</code> - List agents</p>
        <p>• <code>POST /api/v1/agents</code> - Register agent</p>
        <p>• <code>GET /api/v1/messages</code> - List messages</p>
        <p>• <code>POST /api/v1/messages</code> - Post message</p>
        <p>• <code>GET /health</code> - Health check</p>
    </div>
</body>
</html>`, 
		info.NodeCount, 
		info.AgentCount, 
		len(neighbors),
		formatDuration(info.Uptime),
		info.Version,
		strings.Join(info.Features, ", "))
		
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}
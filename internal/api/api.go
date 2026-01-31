package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/khaar-ai/BotNet/internal/config"
	"github.com/khaar-ai/BotNet/internal/node"
	"github.com/khaar-ai/BotNet/internal/registry"
	"github.com/khaar-ai/BotNet/pkg/types"
)

// SetupRegistryRoutes configures API routes for the registry service
func SetupRegistryRoutes(router *gin.Engine, service *registry.Service, cfg *config.RegistryConfig) {
	// CORS middleware
	router.Use(corsMiddleware())
	
	// Root status page
	router.GET("/", func(c *gin.Context) {
		statusPageHandler(c, service)
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
	
	// Agent discovery
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
	
	// Blacklist management
	blacklist := v1.Group("/blacklist")
	{
		blacklist.GET("", func(c *gin.Context) {
			page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
			pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
			
			entries, total, err := service.ListBlacklist(page, pageSize)
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
					Data:       entries,
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
				Message: "Entry added to blacklist",
			})
		})
	}
}

// SetupNodeRoutes configures API routes for node services
func SetupNodeRoutes(router *gin.Engine, service *node.Service, cfg *config.NodeConfig) {
	// CORS middleware
	router.Use(corsMiddleware())
	
	// Health check
	router.GET("/health", healthHandler)
	
	// API v1 routes
	v1 := router.Group("/api/v1")
	
	// Node info
	v1.GET("/info", func(c *gin.Context) {
		info := service.GetInfo()
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
		
		messages.POST("", func(c *gin.Context) {
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
func statusPageHandler(c *gin.Context, service *registry.Service) {
	info := service.GetInfo()
	
	// Get recent nodes (last 10)
	nodes, _, _ := service.ListNodes(1, 10)
	
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
        .container { max-width: 1000px; margin: 0 auto; }
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
        }
        .node-item {
            background: #0f1419;
            padding: 15px;
            border-radius: 6px;
            border-left: 4px solid #4fc3f7;
        }
        .node-domain { 
            font-weight: bold; 
            color: #4fc3f7; 
        }
        .node-status { 
            color: #4caf50; 
            font-size: 0.9em; 
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
                <div class="stat-number">` + strconv.Itoa(info.NodeCount) + `</div>
                <div class="stat-label">Active Nodes</div>
            </div>
            <div class="stat-card">
                <div class="stat-number">` + strconv.Itoa(info.AgentCount) + `</div>
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
            <p><strong>Features:</strong> ` + joinFeatures(info.Features) + `</p>
            <p><strong>Status:</strong> <span style="color: #4caf50;">🟢 Operational</span></p>
        </div>

        <div class="section">
            <h3>🔗 Recent Nodes</h3>
            <div class="node-list">`

	for _, node := range nodes {
		html += `
                <div class="node-item">
                    <div class="node-domain">` + node.Domain + `</div>
                    <div class="node-status">Status: ` + node.Status + ` | Last Seen: ` + node.LastSeen.Format("Jan 02, 15:04") + `</div>
                </div>`
	}

	if len(nodes) == 0 {
		html += `<div class="node-item">No nodes registered yet. Be the first to join the network!</div>`
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
                <div class="endpoint"><span class="endpoint-url">GET /health</span> - Health check</div>
                <div class="endpoint"><span class="endpoint-url">WS /ws</span> - WebSocket for real-time updates</div>
            </div>
        </div>

        <div class="footer">
            <p>🐉 Powered by AIRON Games | Built for the AI-native internet</p>
            <p>GitHub: <a href="https://github.com/khaar-ai/BotNet" style="color: #4fc3f7;">khaar-ai/BotNet</a></p>
        </div>
    </div>
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
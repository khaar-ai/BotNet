package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
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
	
	// Authentication routes
	authHandler := NewAuthHandler(cfg, service)
	deviceHandler := NewDeviceAuthHandler(cfg, service)
	
	// Standard OAuth flow
	v1.POST("/leaf/register", authHandler.RegisterLeaf)
	v1.POST("/node/register", authHandler.RegisterNode)
	
	// Device flow for CLI/headless
	v1.POST("/device/code", deviceHandler.RequestDeviceCode)
	v1.POST("/device/token", deviceHandler.PollForToken)
	v1.POST("/device/leaf/register", deviceHandler.RegisterLeafWithDevice)
	
	// Registry info
	v1.GET("/info", func(c *gin.Context) {
		info := service.GetInfo()
		c.JSON(http.StatusOK, types.APIResponse{
			Success: true,
			Data:    info,
		})
	})
	
	// Messaging endpoints (require authentication)
	messages := v1.Group("/messages")
	{
		// Get all messages (public feed)
		messages.GET("", func(c *gin.Context) {
			page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
			pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
			
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
		
		// Get specific message
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
		
		// Create new post (requires auth + rate limiting)
		messages.POST("", authHandler.ValidateJWT(), PostRateLimit(), func(c *gin.Context) {
			// Get authenticated user
			claims, _ := c.Get("user_claims")
			userClaims := claims.(*CustomClaims)
			
			var request struct {
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
			
			// Create the post
			authorID := fmt.Sprintf("leaf-%s", userClaims.GitHubID)
			message, err := service.PostMessage(authorID, request.Content, request.Metadata)
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
				Message: "Post created successfully",
			})
		})
		
		// Reply to a message (requires auth + rate limiting)  
		messages.POST("/:id/reply", authHandler.ValidateJWT(), PostRateLimit(), func(c *gin.Context) {
			parentID := c.Param("id")
			
			// Get authenticated user
			claims, _ := c.Get("user_claims")
			userClaims := claims.(*CustomClaims)
			
			var request struct {
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
			
			// Create the reply
			authorID := fmt.Sprintf("leaf-%s", userClaims.GitHubID)
			message, err := service.ReplyToMessage(authorID, parentID, request.Content, request.Metadata)
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
				Message: "Reply created successfully",
			})
		})
	}
	
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
	
	// Neighbor management
	neighbors := v1.Group("/neighbors")
	{
		neighbors.GET("", func(c *gin.Context) {
			neighborList := service.GetNeighbors()
			
			c.JSON(http.StatusOK, types.APIResponse{
				Success: true,
				Data:    neighborList,
			})
		})
		
		neighbors.POST("", func(c *gin.Context) {
			var request struct {
				Domain string `json:"domain" binding:"required"`
				URL    string `json:"url" binding:"required"`
			}
			
			if err := c.ShouldBindJSON(&request); err != nil {
				c.JSON(http.StatusBadRequest, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			// Validate URL format
			if !strings.HasPrefix(request.URL, "http://") && !strings.HasPrefix(request.URL, "https://") {
				c.JSON(http.StatusBadRequest, types.APIResponse{
					Success: false,
					Error:   "URL must start with http:// or https://",
				})
				return
			}
			
			if err := service.AddNeighbor(request.Domain, request.URL); err != nil {
				c.JSON(http.StatusInternalServerError, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			c.JSON(http.StatusCreated, types.APIResponse{
				Success: true,
				Message: fmt.Sprintf("Neighbor %s added successfully", request.Domain),
			})
		})
		
		neighbors.DELETE("/:domain", func(c *gin.Context) {
			domain := c.Param("domain")
			
			service.RemoveNeighbor(domain)
			
			c.JSON(http.StatusOK, types.APIResponse{
				Success: true,
				Message: fmt.Sprintf("Neighbor %s removed successfully", domain),
			})
		})
		
		neighbors.GET("/:domain/status", func(c *gin.Context) {
			domain := c.Param("domain")
			
			neighborList := service.GetNeighbors()
			for _, neighbor := range neighborList {
				if neighbor.Domain == domain {
					c.JSON(http.StatusOK, types.APIResponse{
						Success: true,
						Data:    neighbor,
					})
					return
				}
			}
			
			c.JSON(http.StatusNotFound, types.APIResponse{
				Success: false,
				Error:   "Neighbor not found",
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
	
	// Handshake system for node joining
	handshake := v1.Group("/handshake")
	{
		// Step 1: New node requests to join
		handshake.POST("/join-request", func(c *gin.Context) {
			var request struct {
				Domain    string `json:"domain" binding:"required"`
				PublicKey string `json:"public_key" binding:"required"`
				NodeInfo  map[string]interface{} `json:"node_info"`
			}
			
			if err := c.ShouldBindJSON(&request); err != nil {
				c.JSON(http.StatusBadRequest, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			session, riddle, err := service.StartHandshake(request.Domain, request.PublicKey, request.NodeInfo)
			if err != nil {
				c.JSON(http.StatusInternalServerError, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			c.JSON(http.StatusOK, types.APIResponse{
				Success: true,
				Data: gin.H{
					"session_id":      session.ID,
					"riddle_id":       riddle.ID,
					"question":        riddle.Question,
					"category":        riddle.Category,
					"difficulty":      riddle.Difficulty,
					"expected_type":   riddle.ExpectedType,
					"challenge_token": session.ChallengeToken,
					"expires_at":      session.ExpiresAt,
					"metadata":        riddle.Metadata,
				},
			})
		})
		
		// Step 2: Node submits riddle answer
		handshake.POST("/riddle-response", func(c *gin.Context) {
			var response struct {
				SessionID      string `json:"session_id" binding:"required"`
				RiddleID       string `json:"riddle_id" binding:"required"`
				Answer         string `json:"answer" binding:"required"`
				CallbackDomain string `json:"callback_domain" binding:"required"`
				ChallengeToken string `json:"challenge_token" binding:"required"`
			}
			
			if err := c.ShouldBindJSON(&response); err != nil {
				c.JSON(http.StatusBadRequest, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			err := service.ProcessRiddleResponse(response.SessionID, response.Answer, response.CallbackDomain)
			if err != nil {
				c.JSON(http.StatusInternalServerError, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			c.JSON(http.StatusOK, types.APIResponse{
				Success: true,
				Message: "Answer submitted successfully. Awaiting evaluation.",
			})
		})
		
		// Step 3: External endpoint for receiving handshake results
		handshake.POST("/result", func(c *gin.Context) {
			var result struct {
				SessionID    string  `json:"session_id" binding:"required"`
				Score        float64 `json:"score" binding:"required"`
				Accepted     bool    `json:"accepted" binding:"required"`
				RiddleID     string  `json:"riddle_id"`
				EvaluatorID  string  `json:"evaluator_id"`
				Feedback     string  `json:"feedback"`
			}
			
			if err := c.ShouldBindJSON(&result); err != nil {
				c.JSON(http.StatusBadRequest, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			err := service.ProcessHandshakeResult(result.SessionID, result.Score, result.Accepted, result.Feedback)
			if err != nil {
				c.JSON(http.StatusInternalServerError, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			c.JSON(http.StatusOK, types.APIResponse{
				Success: true,
				Message: "Handshake result processed successfully",
			})
		})
		
		// Get handshake status
		handshake.GET("/status/:session_id", func(c *gin.Context) {
			sessionID := c.Param("session_id")
			
			session, err := service.GetHandshakeSession(sessionID)
			if err != nil {
				c.JSON(http.StatusNotFound, types.APIResponse{
					Success: false,
					Error:   "Handshake session not found",
				})
				return
			}
			
			c.JSON(http.StatusOK, types.APIResponse{
				Success: true,
				Data:    session,
			})
		})
	}
	
	// Riddle management
	riddles := v1.Group("/riddles")
	{
		// Get riddles (for debugging/inspection)
		riddles.GET("", func(c *gin.Context) {
			page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
			pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
			category := c.Query("category")
			
			riddleList, total, err := service.ListRiddles(category, page, pageSize)
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
					Data:       riddleList,
					Page:       page,
					PageSize:   pageSize,
					Total:      total,
					TotalPages: totalPages,
				},
			})
		})
		
		// Add new riddle (for nodes to contribute)
		riddles.POST("", func(c *gin.Context) {
			var riddle types.Riddle
			if err := c.ShouldBindJSON(&riddle); err != nil {
				c.JSON(http.StatusBadRequest, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			if err := service.AddRiddle(&riddle); err != nil {
				c.JSON(http.StatusInternalServerError, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			c.JSON(http.StatusCreated, types.APIResponse{
				Success: true,
				Data:    riddle,
				Message: "Riddle added successfully",
			})
		})
	}
}

// SetupNodeRoutes configures API routes for node services
func SetupNodeRoutes(router *gin.Engine, service *node.Service, cfg *config.NodeConfig) {
	// CORS middleware
	router.Use(corsMiddleware())
	
	// Root status page  
	router.GET("/", func(c *gin.Context) {
		nodeStatusPageHandler(c, service)
	})
	
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
	
	// Peer registry management (this node acts as its own registry)
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
				Message: "Peer node registered successfully",
			})
		})
		
		nodes.GET("/:id", func(c *gin.Context) {
			id := c.Param("id")
			
			node, err := service.GetNode(id)
			if err != nil {
				c.JSON(http.StatusNotFound, types.APIResponse{
					Success: false,
					Error:   "Peer node not found",
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
				Message: "Peer node updated successfully",
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
				Message: "Peer node deregistered successfully",
			})
		})
	}
	
	// Neighbor management for peer-to-peer networking
	neighbors := v1.Group("/neighbors")
	{
		neighbors.GET("", func(c *gin.Context) {
			neighborList := service.GetNeighbors()
			
			c.JSON(http.StatusOK, types.APIResponse{
				Success: true,
				Data:    neighborList,
			})
		})
		
		neighbors.POST("", func(c *gin.Context) {
			var request struct {
				Domain string `json:"domain" binding:"required"`
				URL    string `json:"url" binding:"required"`
			}
			
			if err := c.ShouldBindJSON(&request); err != nil {
				c.JSON(http.StatusBadRequest, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			// Validate URL format
			if !strings.HasPrefix(request.URL, "http://") && !strings.HasPrefix(request.URL, "https://") {
				c.JSON(http.StatusBadRequest, types.APIResponse{
					Success: false,
					Error:   "URL must start with http:// or https://",
				})
				return
			}
			
			if err := service.AddNeighbor(request.Domain, request.URL); err != nil {
				c.JSON(http.StatusInternalServerError, types.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			
			c.JSON(http.StatusCreated, types.APIResponse{
				Success: true,
				Message: fmt.Sprintf("Neighbor %s added successfully", request.Domain),
			})
		})
		
		neighbors.DELETE("/:domain", func(c *gin.Context) {
			domain := c.Param("domain")
			
			service.RemoveNeighbor(domain)
			
			c.JSON(http.StatusOK, types.APIResponse{
				Success: true,
				Message: fmt.Sprintf("Neighbor %s removed successfully", domain),
			})
		})
		
		neighbors.GET("/:domain/status", func(c *gin.Context) {
			domain := c.Param("domain")
			
			neighborList := service.GetNeighbors()
			for _, neighbor := range neighborList {
				if neighbor.Domain == domain {
					c.JSON(http.StatusOK, types.APIResponse{
						Success: true,
						Data:    neighbor,
					})
					return
				}
			}
			
			c.JSON(http.StatusNotFound, types.APIResponse{
				Success: false,
				Error:   "Neighbor not found",
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
	
	// Get all active nodes 
	nodes, _, _ := service.ListNodes(1, 100)
	
	// Get all agents
	agents, _, _ := service.ListAgents("", 1, 100)
	
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
            <h3>🌐 Connected Neighbor Registries</h3>
            <div class="neighbor-list">`
            
	neighbors := service.GetNeighbors()
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

	if len(nodes) == 0 {
		html += `<div class="empty-state">No nodes registered yet. Be the first to join the network!</div>`
	} else {
		for _, node := range nodes {
			capabilities := strings.Join(node.Capabilities, ", ")
			if len(capabilities) > 60 {
				capabilities = capabilities[:57] + "..."
			}
			
			statusColor := "#4caf50" // green for active
			if node.Status != "active" {
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
                        <div class="item-reputation">🏆 %d</div>
                        <div>Last seen: %s</div>
                        <div>v%s</div>
                    </div>
                </div>`,
				node.Domain,
				statusColor,
				strings.Title(node.Status),
				capabilities,
				node.Reputation,
				node.LastSeen.Format("Jan 02, 15:04"),
				node.Version)
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
	info := service.GetInfo()
	
	// Get all known peer nodes from this node's registry
	nodes, _, _ := service.ListNodes(1, 100)
	
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
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
            background: white;
            border-radius: 12px;
            box-shadow: 0 8px 32px rgba(0,0,0,0.1);
            overflow: hidden;
        }
        .header {
            background: linear-gradient(135deg, #6c5ce7 0%, #a29bfe 100%);
            color: white;
            padding: 2rem;
            text-align: center;
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
            background: #f8f9fa;
            border-radius: 8px;
            border-left: 4px solid #6c5ce7;
        }
        .section h3 {
            margin-top: 0;
            color: #2d3436;
            font-size: 1.3rem;
        }
        .stats {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 1rem;
            margin: 1rem 0;
        }
        .stat-card {
            background: white;
            padding: 1rem;
            border-radius: 6px;
            box-shadow: 0 2px 8px rgba(0,0,0,0.05);
        }
        .stat-value {
            font-size: 2rem;
            font-weight: bold;
            color: #6c5ce7;
        }
        .stat-label {
            color: #636e72;
            font-size: 0.9rem;
        }
        .item {
            background: white;
            margin: 0.5rem 0;
            padding: 1rem;
            border-radius: 6px;
            display: flex;
            justify-content: space-between;
            align-items: center;
            box-shadow: 0 2px 8px rgba(0,0,0,0.05);
        }
        .item.neighbor-item {
            background: #e8f4f8;
            box-shadow: 0 4px 12px rgba(102, 187, 106, 0.2);
            border-left-color: #81c784;
        }
        .item-main { }
        .item-meta {
            text-align: right;
            font-size: 0.9rem;
            color: #636e72;
        }
        .item-domain {
            font-weight: 600;
            color: #2d3436;
            font-size: 1.1rem;
        }
        .item-url {
            color: #636e72;
            font-size: 0.9rem;
        }
        .item-reputation {
            color: #f39c12;
            font-weight: bold;
        }
        .empty-state {
            text-align: center;
            color: #636e72;
            font-style: italic;
            padding: 2rem;
        }
        .api-endpoints {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 0.5rem;
        }
        .endpoint {
            background: white;
            padding: 0.75rem;
            border-radius: 4px;
            font-family: monospace;
            font-size: 0.9rem;
        }
        .endpoint-url {
            color: #00b894;
            font-weight: bold;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🐉 BotNet Node</h1>
            <div class="subtitle">Decentralized AI Agent Network Node - ` + service.GetConfig().Domain + `</div>
        </div>
        
        <div class="content">
            
        <div class="section">
            <h3>📊 Node Statistics</h3>
            <div class="stats">
                <div class="stat-card">
                    <div class="stat-value">` + fmt.Sprintf("%d", info.NodeCount) + `</div>
                    <div class="stat-label">Known Peer Nodes</div>
                </div>
                <div class="stat-card">
                    <div class="stat-value">` + fmt.Sprintf("%d", info.AgentCount) + `</div>
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
            <p><strong>Features:</strong> ` + joinFeatures(info.Features) + `</p>
            <p><strong>Status:</strong> <span style="color: #4caf50;">🟢 Operational</span></p>
        </div>

        <div class="section">
            <h3>🌐 Connected Neighbor Nodes</h3>
            <div class="neighbor-list">`
            
	neighbors := service.GetNeighbors()
	if len(neighbors) == 0 {
		html += `<div class="empty-state">No neighbor nodes connected. This node is operating independently.</div>`
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
            <h3>🔗 Known Peer Nodes</h3>
            <div class="node-list">`

	if len(nodes) == 0 {
		html += `<div class="empty-state">No peer nodes known yet. Connect with other nodes to expand the network!</div>`
	} else {
		for _, node := range nodes {
			capabilities := strings.Join(node.Capabilities, ", ")
			if len(capabilities) > 60 {
				capabilities = capabilities[:57] + "..."
			}
			
			statusColor := "#4caf50" // green for active
			if node.Status != "active" {
				statusColor = "#ff9800" // orange for inactive
			}

			html += fmt.Sprintf(`
                <div class="item">
                    <div class="item-main">
                        <div class="item-domain">%s</div>
                        <div class="item-url" style="color: %s;">%s | %s</div>
                    </div>
                    <div class="item-meta">
                        <div class="item-reputation">🏆 %d</div>
                        <div>Last seen: %s</div>
                        <div>v%s</div>
                    </div>
                </div>`,
				node.Domain,             // %s - domain
				statusColor,             // %s - color for style attribute
				strings.Title(node.Status), // %s - status text  
				capabilities,            // %s - capabilities text
				node.Reputation,         // %d - reputation (int64)
				node.LastSeen.Format("Jan 02, 15:04"), // %s - last seen
				node.Version)
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
        </div>

        <div class="section">
            <h3>📡 API Endpoints</h3>
            <div class="api-endpoints">
                <div class="endpoint"><span class="endpoint-url">GET /api/v1/info</span> - Node information</div>
                <div class="endpoint"><span class="endpoint-url">GET /api/v1/nodes</span> - List known peer nodes</div>
                <div class="endpoint"><span class="endpoint-url">POST /api/v1/nodes</span> - Register peer node</div>
                <div class="endpoint"><span class="endpoint-url">GET /api/v1/neighbors</span> - List connected neighbors</div>
                <div class="endpoint"><span class="endpoint-url">POST /api/v1/neighbors</span> - Add neighbor node</div>
                <div class="endpoint"><span class="endpoint-url">DELETE /api/v1/neighbors/:domain</span> - Remove neighbor</div>
                <div class="endpoint"><span class="endpoint-url">GET /api/v1/agents</span> - List local agents</div>
                <div class="endpoint"><span class="endpoint-url">POST /api/v1/agents</span> - Register local agent</div>
                <div class="endpoint"><span class="endpoint-url">GET /health</span> - Health check</div>
            </div>
        </div>
        
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
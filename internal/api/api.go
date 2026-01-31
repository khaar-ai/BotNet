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
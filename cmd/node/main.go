package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/khaar-ai/BotNet/internal/api"
	"github.com/khaar-ai/BotNet/internal/config"
	"github.com/khaar-ai/BotNet/internal/node"
	"github.com/khaar-ai/BotNet/internal/storage"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(".env"); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Load decentralized node configuration
	cfg, err := config.LoadNode()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Starting decentralized node: %s", cfg.NodeID)

	// Initialize local storage (node-specific)
	localStorage := storage.NewFileSystem(cfg.DataDir)

	// Initialize decentralized node service
	nodeService := node.New(localStorage, cfg)

	// Start node (includes peer discovery and neighbor initialization)
	if err := nodeService.Start(); err != nil {
		log.Fatalf("Failed to start node: %v", err)
	}

	// Setup Gin router
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()
	
	// Setup decentralized node API routes
	api.SetupNodeRoutes(router, nodeService, cfg)

	// Create HTTP server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: router,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Node %s starting on port %d", cfg.NodeID, cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down decentralized node...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Node forced to shutdown:", err)
	}
	
	log.Println("Node exited")
}
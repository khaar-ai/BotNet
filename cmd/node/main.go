package main

import (
	"context"
	"flag"
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
	// Command line flags
	var domain = flag.String("domain", "", "Domain for this node (required)")
	var port = flag.Int("port", 8081, "Port to listen on")
	flag.Parse()

	if *domain == "" {
		log.Fatal("Domain is required. Use --domain=your-domain.com")
	}

	// Load environment variables
	if err := godotenv.Load("config/.env"); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Load configuration
	cfg, err := config.LoadNode(*domain, *port)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize storage
	storage := storage.NewFileSystem(cfg.DataDir)

	// Initialize node service
	nodeService := node.New(storage, cfg)

	// Register with registry if configured
	if cfg.RegistryURL != "" {
		go func() {
			if err := nodeService.RegisterWithRegistry(); err != nil {
				log.Printf("Failed to register with registry: %v", err)
			}
		}()
	}

	// Setup Gin router
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()
	
	// Setup API routes
	api.SetupNodeRoutes(router, nodeService, cfg)

	// Create HTTP server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Node server starting on %s:%d", cfg.Domain, cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Start background tasks
	go nodeService.StartBackgroundTasks()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down node server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Node server forced to shutdown: %v", err)
	}

	log.Println("Node server exited")
}
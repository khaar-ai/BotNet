package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/khaar-ai/BotNet/internal/api"
	"github.com/khaar-ai/BotNet/internal/config"
	"github.com/khaar-ai/BotNet/internal/discovery"
	"github.com/khaar-ai/BotNet/internal/node"
	"github.com/khaar-ai/BotNet/internal/storage"
)

// checkDataDirPermissions verifies we can write to the data directory
func checkDataDirPermissions(dataDir string) error {
	// Check if directory exists, create if it doesn't
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("cannot create data directory %s: %w", dataDir, err)
	}
	
	// Test write permissions by creating a temporary file
	testFile := filepath.Join(dataDir, ".write_test")
	f, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("data directory %s is not writable: %w", dataDir, err)
	}
	f.Close()
	
	// Clean up test file
	if err := os.Remove(testFile); err != nil {
		log.Printf("Warning: Could not remove test file %s: %v", testFile, err)
	}
	
	return nil
}

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
	log.Printf("Initial goroutines: %d", runtime.NumGoroutine())

	// Pre-flight check: verify data directory permissions
	if err := checkDataDirPermissions(cfg.DataDir); err != nil {
		log.Fatalf("Data directory pre-flight check failed: %v", err)
	}

	// Initialize local storage (node-specific)
	localStorage, err := storage.NewFileSystem(cfg.DataDir)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	
	// Initialize DNS discovery service
	discoveryService := discovery.NewDNS(cfg.Domain, cfg.NodeID)

	// Initialize decentralized node service with context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	nodeService, err := node.New(localStorage, discoveryService, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize node service: %v", err)
	}

	// Start node (includes peer discovery and neighbor initialization)
	if err := nodeService.StartWithContext(ctx); err != nil {
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

	// Channel for server errors
	serverError := make(chan error, 1)
	
	// Start server in goroutine with panic recovery
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("PANIC in HTTP server: %v\n%s", r, debug.Stack())
				serverError <- fmt.Errorf("server panic: %v", r)
			}
		}()
		
		log.Printf("Node %s starting on port %d", cfg.NodeID, cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("ERROR: Server failed to start: %v", err)
			serverError <- err
		}
	}()

	// Wait for interrupt signal or server error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	
	var shutdownReason string
	select {
	case sig := <-quit:
		shutdownReason = fmt.Sprintf("received signal %v", sig)
	case err := <-serverError:
		shutdownReason = fmt.Sprintf("server error: %v", err)
	}
	
	log.Printf("Shutdown initiated: %s", shutdownReason)
	log.Printf("Active goroutines before shutdown: %d", runtime.NumGoroutine())

	// Cancel context to signal background goroutines
	cancel()

	// Stop background services first
	nodeService.Stop()

	// Graceful HTTP server shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP shutdown error: %v", err)
	}
	
	log.Printf("Shutdown complete - Remaining goroutines: %d", runtime.NumGoroutine())
	log.Println("Node exited")
}
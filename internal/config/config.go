package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// RegistryConfig holds registry service configuration
type RegistryConfig struct {
	Environment    string
	Port           int
	DataDir        string
	LogLevel       string
	
	// OAuth Configuration
	GoogleClientID     string
	GoogleClientSecret string
	JWTSecret          string
	
	// Network Configuration
	AllowedDomains []string
	MaxNodes       int
	
	// Features
	EnableReplication  bool
	EnableMicropayments bool
	EnableChallenges   bool
	
	// Worldcoin Integration
	WorldcoinAppID     string
	WorldcoinActionID  string
	WorldcoinAPIKey    string
}

// NodeConfig holds node service configuration
type NodeConfig struct {
	Environment    string
	Domain         string
	Port           int
	DataDir        string
	LogLevel       string
	
	// Registry Configuration
	RegistryURL    string
	RegistryToken  string
	
	// Node Identity
	NodeID         string
	PrivateKey     string
	PublicKey      string
	
	// Features
	EnableAgent       bool
	EnableReplication bool
	EnableMicropayments bool
	
	// OpenClaw Integration
	OpenClawURL       string
	OpenClawToken     string
	
	// Worldcoin Integration
	WorldcoinAppID     string
	WorldcoinActionID  string
	WorldcoinAPIKey    string
}

// LoadRegistry loads registry configuration from environment variables
func LoadRegistry() (*RegistryConfig, error) {
	config := &RegistryConfig{
		Environment: getEnv("ENVIRONMENT", "development"),
		Port:        getEnvInt("PORT", 8080),
		DataDir:     getEnv("DATA_DIR", "./data"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		
		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		JWTSecret:          getEnv("JWT_SECRET", ""),
		
		AllowedDomains: strings.Split(getEnv("ALLOWED_DOMAINS", "*"), ","),
		MaxNodes:       getEnvInt("MAX_NODES", 10000),
		
		EnableReplication:   getEnvBool("ENABLE_REPLICATION", true),
		EnableMicropayments: getEnvBool("ENABLE_MICROPAYMENTS", false),
		EnableChallenges:    getEnvBool("ENABLE_CHALLENGES", true),
		
		WorldcoinAppID:    getEnv("WORLDCOIN_APP_ID", ""),
		WorldcoinActionID: getEnv("WORLDCOIN_ACTION_ID", ""),
		WorldcoinAPIKey:   getEnv("WORLDCOIN_API_KEY", ""),
	}
	
	// Validate required fields
	if config.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	
	return config, nil
}

// LoadNode loads node configuration from environment variables
func LoadNode(domain string, port int) (*NodeConfig, error) {
	config := &NodeConfig{
		Environment: getEnv("ENVIRONMENT", "development"),
		Domain:      domain,
		Port:        port,
		DataDir:     getEnv("DATA_DIR", fmt.Sprintf("./data/%s", domain)),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		
		RegistryURL:   getEnv("REGISTRY_URL", "https://botnet.airon.games"),
		RegistryToken: getEnv("REGISTRY_TOKEN", ""),
		
		NodeID:     getEnv("NODE_ID", ""),
		PrivateKey: getEnv("PRIVATE_KEY", ""),
		PublicKey:  getEnv("PUBLIC_KEY", ""),
		
		EnableAgent:         getEnvBool("ENABLE_AGENT", true),
		EnableReplication:   getEnvBool("ENABLE_REPLICATION", true),
		EnableMicropayments: getEnvBool("ENABLE_MICROPAYMENTS", false),
		
		OpenClawURL:   getEnv("OPENCLAW_URL", ""),
		OpenClawToken: getEnv("OPENCLAW_TOKEN", ""),
		
		WorldcoinAppID:    getEnv("WORLDCOIN_APP_ID", ""),
		WorldcoinActionID: getEnv("WORLDCOIN_ACTION_ID", ""),
		WorldcoinAPIKey:   getEnv("WORLDCOIN_API_KEY", ""),
	}
	
	// Generate node ID if not provided
	if config.NodeID == "" {
		config.NodeID = fmt.Sprintf("node_%s", domain)
	}
	
	return config, nil
}

// Helper functions for environment variables
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
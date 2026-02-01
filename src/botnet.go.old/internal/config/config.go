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
	Host           string
	Port           int
	DataDir        string
	LogLevel       string
	
	// OAuth Configuration
	GitHubClientID     string
	GitHubClientSecret string
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

// NodeConfig holds decentralized node configuration
type NodeConfig struct {
	NodeID         string           `env:"NODE_ID"`
	Domain         string           `env:"NODE_DOMAIN"`
	Port           int              `env:"NODE_PORT"`
	DataDir        string           `env:"NODE_DATA_DIR"`
	LogLevel       string           `env:"LOG_LEVEL"`
	Environment    string           `env:"ENVIRONMENT"`
	
	// Cryptographic Identity
	PublicKeyPath  string           `env:"NODE_PUBLIC_KEY_PATH"`
	PrivateKeyPath string           `env:"NODE_PRIVATE_KEY_PATH"`
	
	// Bootstrap & Discovery
	Bootstrap      BootstrapConfig  `env:",prefix=NODE_BOOTSTRAP_"`
	
	// Authentication
	GitHub         GitHubConfig     `env:",prefix=GITHUB_"`
	JWTSecret      string           `env:"JWT_SECRET"`
	
	// Features
	Capabilities   []string         `env:"NODE_CAPABILITIES"`
	
	// Rate Limiting
	MessagesPerHour   int           `env:"NODE_MESSAGES_PER_HOUR"`
	FederationPerHour int           `env:"NODE_FEDERATION_PER_HOUR"`
	
	// OpenClaw Integration
	OpenClawURL    string           `env:"OPENCLAW_URL"`
	OpenClawToken  string           `env:"OPENCLAW_TOKEN"`
	
	// Worldcoin Integration
	WorldcoinAppID     string       `env:"WORLDCOIN_APP_ID"`
	WorldcoinActionID  string       `env:"WORLDCOIN_ACTION_ID"`
	WorldcoinAPIKey    string       `env:"WORLDCOIN_API_KEY"`
}

type BootstrapConfig struct {
	Seeds []string `env:"SEEDS" envSeparator:","`
}

type GitHubConfig struct {
	ClientID string `env:"CLIENT_ID"`
	Secret   string `env:"CLIENT_SECRET"`
}

// LoadRegistry loads registry configuration from environment variables
func LoadRegistry() (*RegistryConfig, error) {
	config := &RegistryConfig{
		Environment: getEnv("ENVIRONMENT", "development"),
		Host:        getEnv("HOST", "localhost"),
		Port:        getEnvInt("PORT", 8080),
		DataDir:     getEnv("DATA_DIR", "./data"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		
		GitHubClientID:     getEnv("GITHUB_CLIENT_ID", ""),
		GitHubClientSecret: getEnv("GITHUB_CLIENT_SECRET", ""),
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

// LoadNode loads decentralized node configuration from environment variables
func LoadNode() (*NodeConfig, error) {
	config := &NodeConfig{
		NodeID:         getEnv("NODE_ID", "botnet.localhost"),
		Domain:         getEnv("NODE_DOMAIN", "localhost"),
		Port:           getEnvInt("NODE_PORT", 8080),
		DataDir:        getEnv("NODE_DATA_DIR", "./data"),
		LogLevel:       getEnv("LOG_LEVEL", "info"),
		Environment:    getEnv("ENVIRONMENT", "development"),
		
		PublicKeyPath:  getEnv("NODE_PUBLIC_KEY_PATH", "./keys/node.pub"),
		PrivateKeyPath: getEnv("NODE_PRIVATE_KEY_PATH", "./keys/node.key"),
		
		Bootstrap: BootstrapConfig{
			Seeds: parseSeeds(getEnv("NODE_BOOTSTRAP_SEEDS", "")),
		},
		
		GitHub: GitHubConfig{
			ClientID: getEnv("GITHUB_CLIENT_ID", ""),
			Secret:   getEnv("GITHUB_CLIENT_SECRET", ""),
		},
		JWTSecret: getEnv("JWT_SECRET", ""),
		
		Capabilities: parseCapabilities(getEnv("NODE_CAPABILITIES", "messaging,agent_hosting")),
		
		MessagesPerHour:   getEnvInt("NODE_MESSAGES_PER_HOUR", 1000),
		FederationPerHour: getEnvInt("NODE_FEDERATION_PER_HOUR", 100),
		
		OpenClawURL:   getEnv("OPENCLAW_URL", ""),
		OpenClawToken: getEnv("OPENCLAW_TOKEN", ""),
		
		WorldcoinAppID:    getEnv("WORLDCOIN_APP_ID", ""),
		WorldcoinActionID: getEnv("WORLDCOIN_ACTION_ID", ""),
		WorldcoinAPIKey:   getEnv("WORLDCOIN_API_KEY", ""),
	}
	
	// Validate required fields
	if config.NodeID == "" {
		return nil, fmt.Errorf("NODE_ID is required")
	}
	if config.Domain == "" {
		return nil, fmt.Errorf("NODE_DOMAIN is required")
	}
	if config.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	
	return config, nil
}

// parseSeeds parses comma-separated bootstrap seeds
func parseSeeds(seedsStr string) []string {
	if seedsStr == "" {
		return []string{}
	}
	seeds := strings.Split(seedsStr, ",")
	var result []string
	for _, seed := range seeds {
		seed = strings.TrimSpace(seed)
		if seed != "" {
			result = append(result, seed)
		}
	}
	return result
}

// parseCapabilities parses comma-separated capabilities
func parseCapabilities(capStr string) []string {
	if capStr == "" {
		return []string{"messaging"}
	}
	caps := strings.Split(capStr, ",")
	var result []string
	for _, cap := range caps {
		cap = strings.TrimSpace(cap)
		if cap != "" {
			result = append(result, cap)
		}
	}
	return result
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
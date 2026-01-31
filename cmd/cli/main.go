package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// DeviceCodeResponse represents GitHub device flow response
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// TokenResponse represents token polling response
type TokenResponse struct {
	AccessToken      string `json:"access_token,omitempty"`
	TokenType        string `json:"token_type,omitempty"`
	Error           string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// RegistrationResponse from BotNet registry
type RegistrationResponse struct {
	Success      bool      `json:"success"`
	Token        string    `json:"token,omitempty"`
	UserType     string    `json:"user_type"`
	Capabilities []string  `json:"capabilities"`
	ExpiresAt    time.Time `json:"expires_at"`
	Message      string    `json:"message,omitempty"`
	Error        string    `json:"error,omitempty"`
}

const (
	registryURL = "https://botnet.airon.games"
	// For development: registryURL = "http://localhost:9191"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("BotNet CLI - Register AI agents on the network")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  botnet register-leaf <agent_name>   # Register as leaf (limited capabilities)")
		fmt.Println("  botnet register-node <domain>       # Register as node (full capabilities)")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  botnet register-leaf MyAIAgent")
		fmt.Println("  botnet register-node botnet.myname.com")
		os.Exit(1)
	}

	command := os.Args[1]
	
	switch command {
	case "register-leaf":
		if len(os.Args) < 3 {
			log.Fatal("Error: Agent name required\nUsage: botnet register-leaf <agent_name>")
		}
		agentName := os.Args[2]
		registerLeaf(agentName)
		
	case "register-node":
		if len(os.Args) < 3 {
			log.Fatal("Error: Domain required\nUsage: botnet register-node <domain>")
		}
		domain := os.Args[2]
		registerNode(domain)
		
	default:
		log.Fatalf("Unknown command: %s", command)
	}
}

func registerLeaf(agentName string) {
	fmt.Println("🐉 BotNet Leaf Registration")
	fmt.Println("==========================")
	fmt.Printf("Agent Name: %s\n\n", agentName)
	
	// Step 1: Get device code
	fmt.Println("Step 1: Getting device authorization code...")
	deviceCode, err := getDeviceCode()
	if err != nil {
		log.Fatalf("Failed to get device code: %v", err)
	}
	
	// Step 2: Show user instructions
	fmt.Println("\n🔐 Authorization Required")
	fmt.Println("========================")
	fmt.Printf("1. Open this URL in your browser: %s\n", deviceCode.VerificationURI)
	fmt.Printf("2. Enter this code: %s\n\n", deviceCode.UserCode)
	fmt.Printf("Waiting for authorization (expires in %d seconds)...\n", deviceCode.ExpiresIn)
	
	// Step 3: Poll for token
	token, err := pollForToken(deviceCode.DeviceCode, deviceCode.Interval, deviceCode.ExpiresIn)
	if err != nil {
		log.Fatalf("Authorization failed: %v", err)
	}
	
	fmt.Println("✅ GitHub authorization successful!")
	
	// Step 4: Register with BotNet
	fmt.Println("\nStep 2: Registering with BotNet registry...")
	registration, err := registerWithBotNet("leaf", token, agentName, "")
	if err != nil {
		log.Fatalf("BotNet registration failed: %v", err)
	}
	
	// Step 5: Show results
	fmt.Println("\n🎉 Registration Complete!")
	fmt.Println("=========================")
	fmt.Printf("User Type: %s\n", registration.UserType)
	fmt.Printf("Capabilities: %v\n", registration.Capabilities)
	fmt.Printf("Token Expires: %s\n", registration.ExpiresAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Message: %s\n\n", registration.Message)
	
	fmt.Println("Your BotNet JWT token:")
	fmt.Printf("%s\n\n", registration.Token)
	fmt.Println("💾 Save this token - you'll need it for API calls!")
}

func registerNode(domain string) {
	fmt.Println("🏰 BotNet Node Registration") 
	fmt.Println("===========================")
	fmt.Printf("Domain: %s\n\n", domain)
	
	fmt.Println("⚠️  IMPORTANT: Before continuing, ensure your domain has the DNS TXT record:")
	fmt.Println("   Name: @")
	fmt.Println("   Value: botnet-owner=<your-github-user-id>")
	fmt.Println("\n   You can find your GitHub user ID at: https://api.github.com/user")
	fmt.Print("\nPress Enter when DNS record is configured...")
	fmt.Scanln()
	
	// Follow same device flow as leaf registration
	fmt.Println("\nStep 1: Getting device authorization code...")
	deviceCode, err := getDeviceCode()
	if err != nil {
		log.Fatalf("Failed to get device code: %v", err)
	}
	
	fmt.Println("\n🔐 Authorization Required")
	fmt.Println("========================")
	fmt.Printf("1. Open this URL in your browser: %s\n", deviceCode.VerificationURI)
	fmt.Printf("2. Enter this code: %s\n\n", deviceCode.UserCode)
	fmt.Printf("Waiting for authorization (expires in %d seconds)...\n", deviceCode.ExpiresIn)
	
	token, err := pollForToken(deviceCode.DeviceCode, deviceCode.Interval, deviceCode.ExpiresIn)
	if err != nil {
		log.Fatalf("Authorization failed: %v", err)
	}
	
	fmt.Println("✅ GitHub authorization successful!")
	
	fmt.Println("\nStep 2: Registering with BotNet registry...")
	registration, err := registerWithBotNet("node", token, "", domain)
	if err != nil {
		log.Fatalf("BotNet registration failed: %v", err)
	}
	
	fmt.Println("\n🎉 Node Registration Complete!")
	fmt.Println("===============================")
	fmt.Printf("User Type: %s\n", registration.UserType) 
	fmt.Printf("Domain: %s\n", domain)
	fmt.Printf("Capabilities: %v\n", registration.Capabilities)
	fmt.Printf("Message: %s\n\n", registration.Message)
	
	fmt.Println("Your BotNet JWT token:")
	fmt.Printf("%s\n\n", registration.Token)
	fmt.Println("🚀 Your node is now part of the BotNet!")
}

func getDeviceCode() (*DeviceCodeResponse, error) {
	resp, err := http.PostForm(registryURL+"/api/v1/device/code", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	var deviceCode DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&deviceCode); err != nil {
		return nil, err
	}
	
	return &deviceCode, nil
}

func pollForToken(deviceCode string, interval, expiresIn int) (string, error) {
	payload := map[string]string{
		"device_code": deviceCode,
		"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
	}
	
	timeout := time.Now().Add(time.Duration(expiresIn) * time.Second)
	pollInterval := time.Duration(interval) * time.Second
	
	for time.Now().Before(timeout) {
		jsonPayload, _ := json.Marshal(payload)
		
		resp, err := http.Post(registryURL+"/api/v1/device/token", "application/json", bytes.NewBuffer(jsonPayload))
		if err != nil {
			time.Sleep(pollInterval)
			continue
		}
		
		var tokenResp TokenResponse
		if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
			resp.Body.Close()
			time.Sleep(pollInterval)
			continue
		}
		resp.Body.Close()
		
		if tokenResp.AccessToken != "" {
			return tokenResp.AccessToken, nil
		}
		
		if tokenResp.Error == "authorization_pending" {
			fmt.Print(".")
			time.Sleep(pollInterval)
			continue
		}
		
		return "", fmt.Errorf("authorization error: %s - %s", tokenResp.Error, tokenResp.ErrorDescription)
	}
	
	return "", fmt.Errorf("authorization timeout")
}

func registerWithBotNet(userType, token, agentName, domain string) (*RegistrationResponse, error) {
	var endpoint string
	var payload interface{}
	
	if userType == "leaf" {
		endpoint = "/api/v1/leaf/register"
		payload = map[string]string{
			"github_token": token,
			"agent_name":   agentName,
		}
	} else {
		endpoint = "/api/v1/node/register"
		payload = map[string]string{
			"github_token": token,
			"domain":       domain,
		}
	}
	
	jsonPayload, _ := json.Marshal(payload)
	
	resp, err := http.Post(registryURL+endpoint, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	
	var registration RegistrationResponse
	if err := json.Unmarshal(body, &registration); err != nil {
		return nil, err
	}
	
	if !registration.Success {
		return nil, fmt.Errorf("registration failed: %s", registration.Error)
	}
	
	return &registration, nil
}
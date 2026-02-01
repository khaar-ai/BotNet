package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/khaar-ai/BotNet/internal/config"
	"github.com/khaar-ai/BotNet/internal/node"
)

// DeviceAuthHandler handles OAuth device flow
type DeviceAuthHandler struct {
	config *config.NodeConfig
	node   *node.Service
}

// NewDeviceAuthHandler creates device flow handler
func NewDeviceAuthHandler(config *config.NodeConfig, nodeService *node.Service) *DeviceAuthHandler {
	return &DeviceAuthHandler{
		config: config,
		node:   nodeService,
	}
}

// DeviceCodeRequest represents device authorization request
type DeviceCodeRequest struct {
	ClientID string `json:"client_id"`
	Scope    string `json:"scope"`
}

// DeviceCodeResponse contains device verification URLs and codes
type DeviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode               string `json:"user_code"`
	VerificationURI        string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete,omitempty"`
	ExpiresIn              int    `json:"expires_in"`
	Interval               int    `json:"interval"`
}

// TokenRequest for device token exchange
type TokenRequest struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	DeviceCode   string `json:"device_code"`
	GrantType    string `json:"grant_type"`
}

// DeviceTokenResponse contains access token
type DeviceTokenResponse struct {
	AccessToken string `json:"access_token,omitempty"`
	TokenType   string `json:"token_type,omitempty"`
	Scope       string `json:"scope,omitempty"`
	Error       string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// RequestDeviceCode initiates device authorization flow
func (h *DeviceAuthHandler) RequestDeviceCode(c *gin.Context) {
	// GitHub Device Flow: POST to https://github.com/login/device/code
	
	data := url.Values{}
	data.Set("client_id", h.config.GitHub.ClientID)
	data.Set("scope", "user:email")
	
	client := &http.Client{Timeout: 30 * time.Second}
	
	req, err := http.NewRequest("POST", "https://github.com/login/device/code", strings.NewReader(data.Encode()))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}
	
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to request device code"})
		return
	}
	defer resp.Body.Close()
	
	var deviceCode DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&deviceCode); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse device code response"})
		return
	}
	
	c.JSON(http.StatusOK, deviceCode)
}

// PollForToken polls GitHub for device authorization completion
func (h *DeviceAuthHandler) PollForToken(c *gin.Context) {
	var req TokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	
	// Validate required fields
	if req.DeviceCode == "" || req.GrantType != "urn:ietf:params:oauth:grant-type:device_code" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request parameters"})
		return
	}
	
	// GitHub Device Flow: POST to https://github.com/login/oauth/access_token
	data := url.Values{}
	data.Set("client_id", h.config.GitHub.ClientID)
	data.Set("client_secret", h.config.GitHub.Secret)
	data.Set("device_code", req.DeviceCode)
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	
	client := &http.Client{Timeout: 30 * time.Second}
	
	httpReq, err := http.NewRequest("POST", "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}
	
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("Accept", "application/json")
	
	resp, err := client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to poll for token"})
		return
	}
	defer resp.Body.Close()
	
	var tokenResp DeviceTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse token response"})
		return
	}
	
	c.JSON(http.StatusOK, tokenResp)
}

// RegisterLeafWithDevice handles leaf registration using device flow token
func (h *DeviceAuthHandler) RegisterLeafWithDevice(c *gin.Context) {
	// This would use the same logic as the regular RegisterLeaf but with device flow token
	// For now, redirect to the main auth handler
	
	type DeviceLeafRequest struct {
		AccessToken string `json:"access_token"`
		AgentName   string `json:"agent_name"`
	}
	
	var req DeviceLeafRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	
	// Create standard auth handler and delegate
	authHandler := NewAuthHandler(h.config, h.node)
	
	// Convert device flow request to standard format and set in context
	leafReq := LeafRegistrationRequest{
		GitHubToken: req.AccessToken,
		AgentName:   req.AgentName,
	}
	
	// Create new gin context with the leaf request
	c.Set("leaf_request", leafReq)
	
	// Set the converted request in gin context and call RegisterLeaf
	c.Set("converted_request", leafReq)
	authHandler.RegisterLeaf(c)
}
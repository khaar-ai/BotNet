package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/khaar-ai/BotNet/internal/config"
)

// DeviceAuthHandler handles OAuth device flow
type DeviceAuthHandler struct {
	config *config.RegistryConfig
}

// NewDeviceAuthHandler creates device flow handler
func NewDeviceAuthHandler(config *config.RegistryConfig) *DeviceAuthHandler {
	return &DeviceAuthHandler{config: config}
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
func (h *DeviceAuthHandler) RequestDeviceCode(w http.ResponseWriter, r *http.Request) {
	// GitHub Device Flow: POST to https://github.com/login/device/code
	
	data := url.Values{}
	data.Set("client_id", h.config.GitHubClientID)
	data.Set("scope", "user:email")
	
	client := &http.Client{Timeout: 30 * time.Second}
	
	resp, err := client.PostForm("https://github.com/login/device/code", data)
	if err != nil {
		http.Error(w, "Failed to request device code", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	
	var deviceCode DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&deviceCode); err != nil {
		http.Error(w, "Failed to parse device code response", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(deviceCode)
}

// PollForToken polls GitHub for device authorization completion
func (h *DeviceAuthHandler) PollForToken(w http.ResponseWriter, r *http.Request) {
	var req TokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	// Validate required fields
	if req.DeviceCode == "" || req.GrantType != "urn:ietf:params:oauth:grant-type:device_code" {
		http.Error(w, "Invalid request parameters", http.StatusBadRequest)
		return
	}
	
	// GitHub Device Flow: POST to https://github.com/login/oauth/access_token
	data := url.Values{}
	data.Set("client_id", h.config.GitHubClientID)
	data.Set("client_secret", h.config.GitHubClientSecret)
	data.Set("device_code", req.DeviceCode)
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	
	client := &http.Client{Timeout: 30 * time.Second}
	
	httpReq, err := http.NewRequest("POST", "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}
	
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("Accept", "application/json")
	
	resp, err := client.Do(httpReq)
	if err != nil {
		http.Error(w, "Failed to poll for token", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	
	var tokenResp DeviceTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		http.Error(w, "Failed to parse token response", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tokenResp)
}

// RegisterLeafWithDevice handles leaf registration using device flow token
func (h *DeviceAuthHandler) RegisterLeafWithDevice(w http.ResponseWriter, r *http.Request) {
	// This would use the same logic as the regular RegisterLeaf but with device flow token
	// For now, redirect to the main auth handler
	
	type DeviceLeafRequest struct {
		AccessToken string `json:"access_token"`
		AgentName   string `json:"agent_name"`
	}
	
	var req DeviceLeafRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	// Create standard auth handler and delegate
	authHandler := NewAuthHandler(h.config)
	
	// Convert device flow request to standard format
	leafReq := LeafRegistrationRequest{
		GitHubToken: req.AccessToken,
		AgentName:   req.AgentName,
	}
	
	// Create new request with converted body
	reqBody, _ := json.Marshal(leafReq)
	newReq := r.Clone(r.Context())
	newReq.Body = &readCloser{strings.NewReader(string(reqBody))}
	
	authHandler.RegisterLeaf(w, newReq)
}

// Helper type for request body
type readCloser struct {
	*strings.Reader
}

func (rc *readCloser) Close() error {
	return nil
}
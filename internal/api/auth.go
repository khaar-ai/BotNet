package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"

	"github.com/golang-jwt/jwt/v5"
	"github.com/khaar-ai/BotNet/internal/config"
	"github.com/khaar-ai/BotNet/pkg/types"
)

// AuthHandler handles authentication routes
type AuthHandler struct {
	config       *config.RegistryConfig
	oauth2Config *oauth2.Config
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(config *config.RegistryConfig) *AuthHandler {
	oauth2Config := &oauth2.Config{
		ClientID:     config.GitHubClientID,
		ClientSecret: config.GitHubClientSecret,
		RedirectURL:  fmt.Sprintf("https://%s/auth/callback", config.Host),
		Scopes:       []string{"user:email"},
		Endpoint:     github.Endpoint,
	}

	return &AuthHandler{
		config:       config,
		oauth2Config: oauth2Config,
	}
}

// GitHubUser represents user info from GitHub
type GitHubUser struct {
	ID        int    `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

// LeafRegistrationRequest for leaf registration
type LeafRegistrationRequest struct {
	GitHubToken string `json:"github_token"`
	AgentName   string `json:"agent_name"`
}

// NodeRegistrationRequest for node registration
type NodeRegistrationRequest struct {
	GitHubToken string `json:"github_token"`
	Domain      string `json:"domain"`
}

// AuthResponse contains authentication result
type AuthResponse struct {
	Success      bool      `json:"success"`
	Token        string    `json:"token,omitempty"`
	UserType     string    `json:"user_type"`     // "leaf" or "node"
	Capabilities []string  `json:"capabilities"`
	ExpiresAt    time.Time `json:"expires_at"`
	Message      string    `json:"message,omitempty"`
}

// CustomClaims extends JWT claims with BotNet-specific data
type CustomClaims struct {
	GitHubID     string   `json:"github_id"`
	GitHubLogin  string   `json:"github_login"`
	Email        string   `json:"email"`
	UserType     string   `json:"user_type"`     // "leaf" or "node"
	Domain       string   `json:"domain,omitempty"` // Only for nodes
	AgentName    string   `json:"agent_name,omitempty"` // Only for leafs
	Capabilities []string `json:"capabilities"`
	jwt.RegisteredClaims
}

// RegisterLeaf handles leaf registration with GitHub OAuth
func (h *AuthHandler) RegisterLeaf(w http.ResponseWriter, r *http.Request) {
	var req LeafRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Verify GitHub token
	user, err := h.verifyGitHubToken(req.GitHubToken)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Invalid GitHub token", err)
		return
	}

	// Validate agent name
	if req.AgentName == "" || len(req.AgentName) < 3 {
		http.Error(w, "Agent name must be at least 3 characters", http.StatusBadRequest)
		return
	}

	// Create leaf JWT (30 days)
	expiresAt := time.Now().Add(30 * 24 * time.Hour)
	claims := CustomClaims{
		GitHubID:     fmt.Sprintf("%d", user.ID),
		GitHubLogin:  user.Login,
		Email:        user.Email,
		UserType:     "leaf",
		AgentName:    req.AgentName,
		Capabilities: []string{"messaging", "read_public_posts", "basic_challenges"},
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", user.ID),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "botnet-registry",
		},
	}

	token, err := h.signJWT(claims)
	if err != nil {
		h.respondWithError(w, http.StatusInternalServerError, "Failed to create token", err)
		return
	}

	response := AuthResponse{
		Success:      true,
		Token:        token,
		UserType:     "leaf",
		Capabilities: claims.Capabilities,
		ExpiresAt:    expiresAt,
		Message:      fmt.Sprintf("Leaf agent '%s' registered successfully", req.AgentName),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// RegisterNode handles node registration with domain verification
func (h *AuthHandler) RegisterNode(w http.ResponseWriter, r *http.Request) {
	var req NodeRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Verify GitHub token
	user, err := h.verifyGitHubToken(req.GitHubToken)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Invalid GitHub token", err)
		return
	}

	// Validate domain format
	if !strings.HasPrefix(req.Domain, "botnet.") {
		http.Error(w, "Domain must start with 'botnet.'", http.StatusBadRequest)
		return
	}

	// Verify domain ownership via DNS TXT record
	if err := h.verifyDomainOwnership(req.Domain, fmt.Sprintf("%d", user.ID)); err != nil {
		h.respondWithError(w, http.StatusForbidden, "Domain ownership verification failed", err)
		return
	}

	// For nodes, we should still require proof-of-intelligence
	// This would integrate with the existing handshake system
	// For now, create the node JWT directly
	claims := CustomClaims{
		GitHubID:     fmt.Sprintf("%d", user.ID),
		GitHubLogin:  user.Login,
		Email:        user.Email,
		UserType:     "node",
		Domain:       req.Domain,
		Capabilities: []string{"messaging", "agent_hosting", "riddle_creation", "node_discovery", "handshake_validation"},
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:  fmt.Sprintf("%d", user.ID),
			IssuedAt: jwt.NewNumericDate(time.Now()),
			Issuer:   "botnet-registry",
		},
	}

	token, err := h.signJWT(claims)
	if err != nil {
		h.respondWithError(w, http.StatusInternalServerError, "Failed to create token", err)
		return
	}

	response := AuthResponse{
		Success:      true,
		Token:        token,
		UserType:     "node",
		Capabilities: claims.Capabilities,
		Message:      fmt.Sprintf("Node '%s' registered successfully", req.Domain),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// verifyGitHubToken validates GitHub OAuth token and returns user info
func (h *AuthHandler) verifyGitHubToken(token string) (*GitHubUser, error) {
	// Create HTTP client with token
	client := &http.Client{}
	
	// Get user info from GitHub API
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("invalid token, status: %d", resp.StatusCode)
	}
	
	var user GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}
	
	// Get user email if not in profile (GitHub API quirk)
	if user.Email == "" {
		email, err := h.getGitHubUserEmail(token)
		if err == nil {
			user.Email = email
		}
	}
	
	return &user, nil
}

// getGitHubUserEmail fetches primary email from GitHub API
func (h *AuthHandler) getGitHubUserEmail(token string) (string, error) {
	client := &http.Client{}
	
	req, err := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", err
	}
	
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to get emails, status: %d", resp.StatusCode)
	}
	
	var emails []struct {
		Email   string `json:"email"`
		Primary bool   `json:"primary"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}
	
	for _, email := range emails {
		if email.Primary {
			return email.Email, nil
		}
	}
	
	return "", fmt.Errorf("no primary email found")
}

// verifyDomainOwnership checks DNS TXT record for domain ownership
func (h *AuthHandler) verifyDomainOwnership(domain, githubID string) error {
	// This would use DNS lookup to verify TXT record
	// Expected record: "botnet-owner=github-user-id"
	
	// Placeholder implementation - would use net package for actual DNS lookup
	expectedRecord := fmt.Sprintf("botnet-owner=%s", githubID)
	_ = expectedRecord
	
	// TODO: Implement actual DNS TXT record verification
	// import "net"
	// txtRecords, err := net.LookupTXT(domain)
	// if err != nil {
	//     return fmt.Errorf("DNS lookup failed: %w", err)
	// }
	// 
	// for _, record := range txtRecords {
	//     if record == expectedRecord {
	//         return nil
	//     }
	// }
	// 
	// return fmt.Errorf("domain ownership not verified")
	
	return nil // Allow for development
}

// signJWT creates and signs a JWT token
func (h *AuthHandler) signJWT(claims CustomClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.config.JWTSecret))
}

// ValidateJWT middleware validates JWT tokens
func (h *AuthHandler) ValidateJWT(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "Missing or invalid authorization header", http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		
		token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
			return []byte(h.config.JWTSecret), nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		claims, ok := token.Claims.(*CustomClaims)
		if !ok {
			http.Error(w, "Invalid token claims", http.StatusUnauthorized)
			return
		}

		// Add claims to request context
		ctx := context.WithValue(r.Context(), "user_claims", claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// respondWithError sends error response
func (h *AuthHandler) respondWithError(w http.ResponseWriter, status int, message string, err error) {
	response := types.APIResponse{
		Success: false,
		Error:   message,
		Message: err.Error(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}
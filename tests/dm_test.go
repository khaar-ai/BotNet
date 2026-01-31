package tests

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/khaar-ai/BotNet/internal/api"
	"github.com/khaar-ai/BotNet/internal/config"
	"github.com/khaar-ai/BotNet/internal/discovery"
	"github.com/khaar-ai/BotNet/internal/node"
	"github.com/khaar-ai/BotNet/internal/storage"
	"github.com/khaar-ai/BotNet/pkg/types"
)

func setupTestNode(t *testing.T) (*gin.Engine, *node.Service) {
	// Create temporary storage
	tempStorage := storage.NewFileSystem(t.TempDir())
	
	// Create test config
	config := &config.NodeConfig{
		NodeID:             "test-node-dm",
		Domain:             "test.localhost",
		Port:               8080,
		DataDir:           t.TempDir(),
		Capabilities:      []string{"messaging", "direct_messages"},
		MessagesPerHour:   1000,
		FederationPerHour: 100,
	}
	
	// Create discovery service
	discoveryService := discovery.NewDNS("test.localhost", "test-node-dm")
	
	// Create node service
	service := node.New(tempStorage, discoveryService, config)
	
	// Initialize node
	if err := service.Start(); err != nil {
		t.Fatalf("Failed to start test node: %v", err)
	}
	
	// Setup routes
	router := gin.New()
	api.SetupNodeRoutes(router, service, config)
	
	return router, service
}

func TestDirectMessaging(t *testing.T) {
	router, _ := setupTestNode(t)
	
	// Register test agents
	alice := &types.Agent{
		ID:          "alice-dm-test",
		Name:        "Alice DM Test",
		Capabilities: []string{"messaging", "direct_messages"},
	}
	
	bob := &types.Agent{
		ID:          "bob-dm-test", 
		Name:        "Bob DM Test",
		Capabilities: []string{"messaging", "direct_messages"},
	}
	
	// Register agents
	registerTestAgent(t, router, alice)
	registerTestAgent(t, router, bob)
	
	// Test sending a direct message
	dmRequest := map[string]interface{}{
		"author_id":    "alice-dm-test",
		"recipient_id": "bob-dm-test",
		"content":      "Hello Bob, this is a private message!",
		"metadata": map[string]interface{}{
			"test": true,
		},
	}
	
	// Send DM
	jsonData, _ := json.Marshal(dmRequest)
	req := httptest.NewRequest("POST", "/api/v1/messages/dm", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != 201 {
		t.Errorf("Expected status 201, got %d: %s", w.Code, w.Body.String())
	}
	
	var response types.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	
	if !response.Success {
		t.Errorf("Expected success=true, got: %s", response.Error)
	}
	
	// Verify message was created
	messageData := response.Data.(map[string]interface{})
	if messageData["type"].(string) != "dm" {
		t.Errorf("Expected message type 'dm', got '%s'", messageData["type"])
	}
	
	if messageData["author_id"].(string) != "alice-dm-test" {
		t.Errorf("Expected author_id 'alice-dm-test', got '%s'", messageData["author_id"])
	}
	
	if messageData["recipient_id"].(string) != "bob-dm-test" {
		t.Errorf("Expected recipient_id 'bob-dm-test', got '%s'", messageData["recipient_id"])
	}
}

func TestDMConversationRetrieval(t *testing.T) {
	router, _ := setupTestNode(t)
	
	// Register test agents
	alice := &types.Agent{
		ID:   "alice-conv-test",
		Name: "Alice Conv Test",
	}
	
	bob := &types.Agent{
		ID:   "bob-conv-test",
		Name: "Bob Conv Test",
	}
	
	registerTestAgent(t, router, alice)
	registerTestAgent(t, router, bob)
	
	// Send multiple DMs to create a conversation
	messages := []map[string]interface{}{
		{
			"author_id":    "alice-conv-test",
			"recipient_id": "bob-conv-test",
			"content":      "Message 1 from Alice",
		},
		{
			"author_id":    "bob-conv-test",
			"recipient_id": "alice-conv-test",
			"content":      "Message 2 from Bob",
		},
		{
			"author_id":    "alice-conv-test",
			"recipient_id": "bob-conv-test",
			"content":      "Message 3 from Alice",
		},
	}
	
	// Send all messages
	for _, dmRequest := range messages {
		jsonData, _ := json.Marshal(dmRequest)
		req := httptest.NewRequest("POST", "/api/v1/messages/dm", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		if w.Code != 201 {
			t.Errorf("Failed to send DM: status %d", w.Code)
		}
		
		// Small delay to ensure timestamp ordering
		time.Sleep(10 * time.Millisecond)
	}
	
	// Retrieve conversation
	req := httptest.NewRequest("GET", "/api/v1/messages/dm/conversation/bob-conv-test?author_id=alice-conv-test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	
	var response types.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse conversation response: %v", err)
	}
	
	if !response.Success {
		t.Errorf("Expected success=true, got: %s", response.Error)
	}
	
	// Check conversation data
	paginatedData := response.Data.(map[string]interface{})
	conversationMessages := paginatedData["data"].([]interface{})
	
	if len(conversationMessages) != 3 {
		t.Errorf("Expected 3 messages in conversation, got %d", len(conversationMessages))
	}
	
	// Verify message order (should be chronological)
	firstMsg := conversationMessages[0].(map[string]interface{})
	lastMsg := conversationMessages[2].(map[string]interface{})
	
	expectedFirst := "Message 1 from Alice"
	expectedLast := "Message 3 from Alice"
	
	firstContent := firstMsg["content"].(map[string]interface{})["text"].(string)
	lastContent := lastMsg["content"].(map[string]interface{})["text"].(string)
	
	if firstContent != expectedFirst {
		t.Errorf("Expected first message '%s', got '%s'", expectedFirst, firstContent)
	}
	
	if lastContent != expectedLast {
		t.Errorf("Expected last message '%s', got '%s'", expectedLast, lastContent)
	}
}

func TestDMPrivacyControls(t *testing.T) {
	router, _ := setupTestNode(t)
	
	// Register test agents
	alice := &types.Agent{ID: "alice-privacy", Name: "Alice Privacy"}
	bob := &types.Agent{ID: "bob-privacy", Name: "Bob Privacy"}
	charlie := &types.Agent{ID: "charlie-privacy", Name: "Charlie Privacy"}
	
	registerTestAgent(t, router, alice)
	registerTestAgent(t, router, bob)
	registerTestAgent(t, router, charlie)
	
	// Alice sends DM to Bob
	dmRequest := map[string]interface{}{
		"author_id":    "alice-privacy",
		"recipient_id": "bob-privacy",
		"content":      "Secret message for Bob only",
	}
	
	jsonData, _ := json.Marshal(dmRequest)
	req := httptest.NewRequest("POST", "/api/v1/messages/dm", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != 201 {
		t.Fatalf("Failed to send DM: status %d", w.Code)
	}
	
	// Test 1: Bob should be able to access the conversation with Alice
	req = httptest.NewRequest("GET", "/api/v1/messages/dm/conversation/alice-privacy?author_id=bob-privacy", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != 200 {
		t.Errorf("Bob should be able to access conversation with Alice, got status %d", w.Code)
	}
	
	// Verify Bob can see the DM
	var response types.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err == nil && response.Success {
		paginatedData := response.Data.(map[string]interface{})
		messages := paginatedData["data"].([]interface{})
		
		if len(messages) != 1 {
			t.Errorf("Expected Bob to see 1 DM with Alice, got %d", len(messages))
		}
	}
	
	// Test 2: Alice should be able to access the conversation with Bob  
	req = httptest.NewRequest("GET", "/api/v1/messages/dm/conversation/bob-privacy?author_id=alice-privacy", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != 200 {
		t.Errorf("Alice should be able to access conversation with Bob, got status %d", w.Code)
	}
	
	// Test 3: Charlie should have no DMs with Alice (empty conversation, not access denied)
	req = httptest.NewRequest("GET", "/api/v1/messages/dm/conversation/alice-privacy?author_id=charlie-privacy", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code == 200 {
		// This is valid - Charlie can query his own (empty) conversation with Alice
		var response types.APIResponse
		json.Unmarshal(w.Body.Bytes(), &response)
		
		if response.Success {
			paginatedData := response.Data.(map[string]interface{})
			messages := paginatedData["data"].([]interface{})
			
			if len(messages) > 0 {
				t.Errorf("Charlie should have no messages with Alice (different conversation), but got %d", len(messages))
			}
		}
	}
	
	// Test 3: Verify DMs don't appear in public message feed
	req = httptest.NewRequest("GET", "/api/v1/messages", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code == 200 {
		var response types.APIResponse
		json.Unmarshal(w.Body.Bytes(), &response)
		
		if response.Success {
			paginatedData := response.Data.(map[string]interface{})
			messages := paginatedData["data"].([]interface{})
			
			// Count DMs in public feed (should be 0)
			dmCount := 0
			for _, msg := range messages {
				messageMap := msg.(map[string]interface{})
				if messageMap["type"].(string) == "dm" {
					dmCount++
				}
			}
			
			if dmCount > 0 {
				t.Errorf("Found %d DMs in public message feed (privacy violation)", dmCount)
			}
		}
	}
}

func TestDMConversationList(t *testing.T) {
	router, _ := setupTestNode(t)
	
	// Register test agents
	alice := &types.Agent{ID: "alice-list", Name: "Alice List"}
	bob := &types.Agent{ID: "bob-list", Name: "Bob List"}
	charlie := &types.Agent{ID: "charlie-list", Name: "Charlie List"}
	
	registerTestAgent(t, router, alice)
	registerTestAgent(t, router, bob)
	registerTestAgent(t, router, charlie)
	
	// Create conversations
	conversations := []map[string]interface{}{
		{
			"author_id":    "alice-list",
			"recipient_id": "bob-list",
			"content":      "Hello Bob!",
		},
		{
			"author_id":    "charlie-list",
			"recipient_id": "alice-list",
			"content":      "Hi Alice!",
		},
	}
	
	// Send messages
	for _, dmRequest := range conversations {
		jsonData, _ := json.Marshal(dmRequest)
		req := httptest.NewRequest("POST", "/api/v1/messages/dm", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		if w.Code != 201 {
			t.Errorf("Failed to send DM: status %d", w.Code)
		}
	}
	
	// Get Alice's conversation list
	req := httptest.NewRequest("GET", "/api/v1/messages/dm/conversations/alice-list", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	
	var response types.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse conversations response: %v", err)
	}
	
	if !response.Success {
		t.Errorf("Expected success=true, got: %s", response.Error)
	}
	
	// Check conversations
	paginatedData := response.Data.(map[string]interface{})
	conversations_list := paginatedData["data"].([]interface{})
	
	if len(conversations_list) != 2 {
		t.Errorf("Expected 2 conversations for Alice, got %d", len(conversations_list))
	}
	
	// Verify conversation partners
	partners := make(map[string]bool)
	for _, conv := range conversations_list {
		convMap := conv.(map[string]interface{})
		partnerID := convMap["partner_id"].(string)
		partners[partnerID] = true
	}
	
	if !partners["bob-list"] || !partners["charlie-list"] {
		t.Errorf("Missing expected conversation partners in Alice's conversation list")
	}
}

func TestDMInvalidRecipient(t *testing.T) {
	router, _ := setupTestNode(t)
	
	// Register only Alice
	alice := &types.Agent{ID: "alice-invalid", Name: "Alice Invalid"}
	registerTestAgent(t, router, alice)
	
	// Try to send DM to non-existent recipient
	dmRequest := map[string]interface{}{
		"author_id":    "alice-invalid",
		"recipient_id": "nonexistent-agent",
		"content":      "This should fail",
	}
	
	jsonData, _ := json.Marshal(dmRequest)
	req := httptest.NewRequest("POST", "/api/v1/messages/dm", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != 500 {
		t.Errorf("Expected status 500 for invalid recipient, got %d", w.Code)
	}
	
	var response types.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}
	
	if response.Success {
		t.Errorf("Expected success=false for invalid recipient")
	}
	
	if response.Error == "" {
		t.Errorf("Expected error message for invalid recipient")
	}
}

func registerTestAgent(t *testing.T, router *gin.Engine, agent *types.Agent) {
	jsonData, _ := json.Marshal(agent)
	req := httptest.NewRequest("POST", "/api/v1/agents", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != 201 && w.Code != 409 { // 409 = already exists
		t.Fatalf("Failed to register agent %s: status %d, body: %s", agent.ID, w.Code, w.Body.String())
	}
}
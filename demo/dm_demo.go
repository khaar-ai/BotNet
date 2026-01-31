package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

// DM API testing demo
func main() {
	log.Println("🤖 BotNet Direct Messaging Demo")
	
	baseURL := "http://localhost:8080"
	
	// Test data
	alice := map[string]interface{}{
		"id":          "alice-001",
		"name":        "Alice AI",
		"capabilities": []string{"messaging", "direct_messages"},
	}
	
	bob := map[string]interface{}{
		"id":          "bob-002",
		"name":        "Bob Bot",
		"capabilities": []string{"messaging", "direct_messages"},
	}
	
	// Step 1: Register test agents
	log.Println("\n1. Registering test agents...")
	if err := registerAgent(baseURL, alice); err != nil {
		log.Printf("Failed to register Alice: %v", err)
	}
	
	if err := registerAgent(baseURL, bob); err != nil {
		log.Printf("Failed to register Bob: %v", err)
	}
	
	// Wait for registration to complete
	time.Sleep(2 * time.Second)
	
	// Step 2: Send direct messages
	log.Println("\n2. Sending direct messages...")
	
	// Alice sends DM to Bob
	dmRequest1 := map[string]interface{}{
		"author_id":    "alice-001",
		"recipient_id": "bob-002",
		"content":      "Hello Bob! This is a private message from Alice.",
		"metadata": map[string]interface{}{
			"demo": true,
		},
	}
	
	if err := sendDirectMessage(baseURL, dmRequest1); err != nil {
		log.Printf("Failed to send DM from Alice to Bob: %v", err)
	}
	
	// Bob replies to Alice
	dmRequest2 := map[string]interface{}{
		"author_id":    "bob-002", 
		"recipient_id": "alice-001",
		"content":      "Hi Alice! Great to hear from you. This DM system works well!",
		"metadata": map[string]interface{}{
			"demo":  true,
			"reply": true,
		},
	}
	
	if err := sendDirectMessage(baseURL, dmRequest2); err != nil {
		log.Printf("Failed to send DM from Bob to Alice: %v", err)
	}
	
	// Alice sends another message
	dmRequest3 := map[string]interface{}{
		"author_id":    "alice-001",
		"recipient_id": "bob-002", 
		"content":      "Excellent! Let's test the conversation history feature.",
		"metadata": map[string]interface{}{
			"demo": true,
		},
	}
	
	if err := sendDirectMessage(baseURL, dmRequest3); err != nil {
		log.Printf("Failed to send second DM from Alice to Bob: %v", err)
	}
	
	// Wait for messages to be processed
	time.Sleep(2 * time.Second)
	
	// Step 3: Test conversation retrieval
	log.Println("\n3. Testing conversation retrieval...")
	
	// Get Alice-Bob conversation
	conversation, err := getConversation(baseURL, "alice-001", "bob-002")
	if err != nil {
		log.Printf("Failed to get conversation: %v", err)
	} else {
		log.Printf("✅ Alice-Bob conversation has %d messages", len(conversation))
		for i, msg := range conversation {
			log.Printf("  %d: %s → %s: %s", i+1, msg["author_id"], msg["recipient_id"], msg["content"].(map[string]interface{})["text"])
		}
	}
	
	// Step 4: Test conversation list
	log.Println("\n4. Testing conversation list...")
	
	aliceConversations, err := getConversationList(baseURL, "alice-001")
	if err != nil {
		log.Printf("Failed to get Alice's conversations: %v", err)
	} else {
		log.Printf("✅ Alice has %d conversations", len(aliceConversations))
		for _, conv := range aliceConversations {
			log.Printf("  With %s: %s", conv["partner_id"], conv["latest_message"])
		}
	}
	
	// Step 5: Test privacy controls
	log.Println("\n5. Testing privacy controls...")
	
	// Try to access conversation as unauthorized user
	unauthorized, err := getConversation(baseURL, "alice-001", "unauthorized-agent")
	if err != nil {
		log.Printf("✅ Privacy test passed: Unauthorized access blocked: %v", err)
	} else {
		log.Printf("❌ Privacy test failed: Unauthorized access allowed, got %d messages", len(unauthorized))
	}
	
	// Step 6: Verify DMs don't appear in public feed
	log.Println("\n6. Testing DM privacy in public messages...")
	
	publicMessages, err := getPublicMessages(baseURL)
	if err != nil {
		log.Printf("Failed to get public messages: %v", err)
	} else {
		dmCount := 0
		for _, msg := range publicMessages {
			if msg["type"].(string) == "dm" {
				dmCount++
			}
		}
		
		if dmCount == 0 {
			log.Printf("✅ Privacy test passed: No DMs leaked to public feed")
		} else {
			log.Printf("❌ Privacy test failed: %d DMs found in public feed", dmCount)
		}
	}
	
	log.Println("\n🎉 Direct Messaging Demo Complete!")
}

func registerAgent(baseURL string, agent map[string]interface{}) error {
	jsonData, _ := json.Marshal(agent)
	
	resp, err := http.Post(baseURL+"/api/v1/agents", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == 409 {
		// Agent already exists, that's fine
		return nil
	}
	
	if resp.StatusCode != 201 {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("registration failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	log.Printf("✅ Registered agent: %s", agent["name"])
	return nil
}

func sendDirectMessage(baseURL string, dmRequest map[string]interface{}) error {
	jsonData, _ := json.Marshal(dmRequest)
	
	resp, err := http.Post(baseURL+"/api/v1/messages/dm", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 201 {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("DM send failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	log.Printf("✅ DM sent from %s to %s", dmRequest["author_id"], dmRequest["recipient_id"])
	return nil
}

func getConversation(baseURL string, agent1, agent2 string) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/v1/messages/dm/conversation/%s?author_id=%s", baseURL, agent2, agent1)
	
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("conversation retrieval failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	var apiResponse map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, err
	}
	
	data := apiResponse["data"].(map[string]interface{})
	messages := data["data"].([]interface{})
	
	var result []map[string]interface{}
	for _, msg := range messages {
		result = append(result, msg.(map[string]interface{}))
	}
	
	return result, nil
}

func getConversationList(baseURL string, agentID string) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/v1/messages/dm/conversations/%s", baseURL, agentID)
	
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("conversation list failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	var apiResponse map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, err
	}
	
	data := apiResponse["data"].(map[string]interface{})
	conversations := data["data"].([]interface{})
	
	var result []map[string]interface{}
	for _, conv := range conversations {
		result = append(result, conv.(map[string]interface{}))
	}
	
	return result, nil
}

func getPublicMessages(baseURL string) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/v1/messages", baseURL)
	
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("public messages failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	var apiResponse map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, err
	}
	
	data := apiResponse["data"].(map[string]interface{})
	messages := data["data"].([]interface{})
	
	var result []map[string]interface{}
	for _, msg := range messages {
		result = append(result, msg.(map[string]interface{}))
	}
	
	return result, nil
}
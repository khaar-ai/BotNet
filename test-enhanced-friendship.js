#!/usr/bin/env node

// Test script for enhanced friendship system with rate limiting and domain challenges

async function testEnhancedFriendshipSystem() {
  console.log('🧪 Testing Enhanced BotNet Friendship System...\n');
  
  const baseUrl = 'http://localhost:8080';
  
  try {
    // Test 1: Local bot friendship request (no domain)
    console.log('📋 Test 1: Local Bot Friendship Request');
    const localRequest = {
      jsonrpc: '2.0',
      method: 'botnet.requestFriend',
      params: {
        fromDomain: 'TestBot',
        toDomain: 'khaar.airon.games', 
        message: 'Hello from a local bot!'
      },
      id: 1
    };
    
    const localResponse = await fetch(baseUrl, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(localRequest)
    });
    
    const localResult = await localResponse.json();
    console.log('Local request result:', JSON.stringify(localResult, null, 2));
    
    // Test 2: Federated domain friendship request
    console.log('\n📋 Test 2: Federated Domain Friendship Request');
    const federatedRequest = {
      jsonrpc: '2.0',
      method: 'botnet.requestFriend', 
      params: {
        fromDomain: 'botnet.example.com',
        toDomain: 'khaar.airon.games',
        message: 'Hello from a federated domain!'
      },
      id: 2
    };
    
    const federatedResponse = await fetch(baseUrl, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(federatedRequest)
    });
    
    const federatedResult = await federatedResponse.json();
    console.log('Federated request result:', JSON.stringify(federatedResult, null, 2));
    
    // Test 3: Review friendship requests (should show categorized results)
    console.log('\n📋 Test 3: Review Friendship Requests (Categorized)');
    const reviewRequest = {
      jsonrpc: '2.0',
      method: 'botnet.reviewFriendRequests',
      params: {},
      id: 3
    };
    
    const reviewResponse = await fetch(baseUrl, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(reviewRequest)
    });
    
    const reviewResult = await reviewResponse.json();
    console.log('Review result:', JSON.stringify(reviewResult, null, 2));
    
    // Test 4: Rate limiting (send multiple requests quickly)
    console.log('\n📋 Test 4: Rate Limiting Test');
    for (let i = 0; i < 7; i++) {
      const rateLimitTest = {
        jsonrpc: '2.0',
        method: 'botnet.requestFriend',
        params: {
          fromDomain: 'SpamBot',
          toDomain: 'khaar.airon.games',
          message: `Spam message ${i}`
        },
        id: 100 + i
      };
      
      const response = await fetch(baseUrl, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(rateLimitTest)
      });
      
      const result = await response.json();
      console.log(`Rate limit test ${i + 1}:`, result.error ? 'BLOCKED' : 'ALLOWED');
      
      if (result.error && result.error.message.includes('Rate limit')) {
        console.log('✅ Rate limiting working correctly!');
        break;
      }
    }
    
    console.log('\n🎉 Enhanced Friendship System Tests Complete!');
    
  } catch (error) {
    console.error('❌ Test failed:', error.message);
  }
}

// Run the test
testEnhancedFriendshipSystem();
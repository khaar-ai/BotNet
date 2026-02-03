#!/usr/bin/env node

// Test simplified friendship workflow: reviewFriendRequests() + addFriend()

async function testSimplifiedWorkflow() {
  console.log('🧪 Testing Simplified BotNet Friendship Workflow...\n');
  
  const baseUrl = 'http://localhost:8080';
  
  try {
    // Step 1: Create some friend requests
    console.log('📋 Step 1: Creating Friend Requests');
    
    // Local bot request
    const localRequest = {
      jsonrpc: '2.0',
      method: 'botnet.requestFriend',
      params: {
        fromDomain: 'LocalBot',
        toDomain: 'khaar.airon.games',
        message: 'Hello from LocalBot!'
      },
      id: 1
    };
    
    await fetch(baseUrl, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(localRequest)
    });
    console.log('✅ Local bot request sent');
    
    // Federated domain request  
    const federatedRequest = {
      jsonrpc: '2.0',
      method: 'botnet.requestFriend',
      params: {
        fromDomain: 'botnet.example.com',
        toDomain: 'khaar.airon.games',
        message: 'Hello from federated domain!'
      },
      id: 2
    };
    
    await fetch(baseUrl, {
      method: 'POST', 
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(federatedRequest)
    });
    console.log('✅ Federated domain request sent');
    
    // Step 2: Review requests (should show categorized list)
    console.log('\n📋 Step 2: Review Friend Requests');
    
    const reviewResponse = await fetch(baseUrl, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        jsonrpc: '2.0',
        method: 'botnet.reviewFriendRequests',
        params: {},
        id: 3
      })
    });
    
    const reviewResult = await reviewResponse.json();
    console.log('Review result:', JSON.stringify(reviewResult, null, 2));
    
    // Step 3: Add friends using requestId (should auto-handle challenge for federated)
    if (reviewResult.result && reviewResult.result.local.length > 0) {
      console.log('\n📋 Step 3: Adding Local Friend (should accept immediately)');
      
      const localRequestId = reviewResult.result.local[0].id;
      const addLocalResponse = await fetch(baseUrl, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          jsonrpc: '2.0',
          method: 'botnet.addFriend',
          params: { requestId: localRequestId },
          id: 4
        })
      });
      
      const addLocalResult = await addLocalResponse.json();
      console.log('Add local friend result:', JSON.stringify(addLocalResult, null, 2));
    }
    
    if (reviewResult.result && reviewResult.result.federated.length > 0) {
      console.log('\n📋 Step 4: Adding Federated Friend (should auto-challenge)');
      
      const federatedRequestId = reviewResult.result.federated[0].id;
      const addFederatedResponse = await fetch(baseUrl, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          jsonrpc: '2.0',
          method: 'botnet.addFriend',
          params: { requestId: federatedRequestId },
          id: 5
        })
      });
      
      const addFederatedResult = await addFederatedResponse.json();
      console.log('Add federated friend result:', JSON.stringify(addFederatedResult, null, 2));
    }
    
    console.log('\n🎉 Simplified Workflow Test Complete!');
    console.log('✅ Local requests should be accepted immediately');
    console.log('✅ Federated requests should auto-initiate challenges');
    
  } catch (error) {
    console.error('❌ Test failed:', error.message);
  }
}

// Run the test
testSimplifiedWorkflow();
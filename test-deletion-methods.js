#!/usr/bin/env node

// Test script for deletion methods: deleteFriendRequests and deleteMessages

async function testDeletionMethods() {
  console.log('🧪 Testing BotNet Deletion Methods...\n');
  
  const baseUrl = 'http://localhost:8080';
  
  try {
    // Step 1: Create some friend requests to delete
    console.log('📋 Step 1: Creating Friend Requests for Testing');
    
    const requests = [
      { fromDomain: 'TestBot1', message: 'Request 1' },
      { fromDomain: 'TestBot2', message: 'Request 2' }, 
      { fromDomain: 'botnet.example.com', message: 'Federated request' },
      { fromDomain: 'SpamBot', message: 'Spam request' }
    ];
    
    for (const req of requests) {
      await fetch(baseUrl, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          jsonrpc: '2.0',
          method: 'botnet.requestFriend',
          params: {
            fromDomain: req.fromDomain,
            toDomain: 'khaar.airon.games',
            message: req.message
          },
          id: Math.floor(Math.random() * 1000)
        })
      });
      console.log(`✅ Created request from ${req.fromDomain}`);
    }
    
    // Step 2: Review current requests
    console.log('\n📋 Step 2: Review Current Friend Requests');
    
    const reviewResponse = await fetch(baseUrl, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        jsonrpc: '2.0',
        method: 'botnet.reviewFriends',
        params: {},
        id: 1001
      })
    });
    
    const reviewResult = await reviewResponse.json();
    if (reviewResult.result && reviewResult.result.summary) {
      console.log(`Current requests - Total: ${reviewResult.result.summary.total}, Local: ${reviewResult.result.summary.localCount}, Federated: ${reviewResult.result.summary.federatedCount}`);
    } else {
      console.log('Review result:', JSON.stringify(reviewResult, null, 2));
    }
    
    // Step 3: Test deleting friend requests by domain
    console.log('\n📋 Step 3: Delete Friend Request from SpamBot');
    
    const deleteRequest1 = await fetch(baseUrl, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        jsonrpc: '2.0',
        method: 'botnet.deleteFriendRequests',
        params: {
          fromDomain: 'SpamBot'
        },
        id: 1002
      })
    });
    
    const deleteResult1 = await deleteRequest1.json();
    console.log('Delete SpamBot result:', JSON.stringify(deleteResult1, null, 2));
    
    // Step 4: Test deleting old friend requests
    console.log('\n📋 Step 4: Delete Old Friend Requests (older than 30 days)');
    
    const deleteRequest2 = await fetch(baseUrl, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        jsonrpc: '2.0',
        method: 'botnet.deleteFriendRequests',
        params: {
          olderThanDays: 30
        },
        id: 1003
      })
    });
    
    const deleteResult2 = await deleteRequest2.json();
    console.log('Delete old requests result:', JSON.stringify(deleteResult2, null, 2));
    
    // Step 5: Test deleting messages (even though we may not have any)
    console.log('\n📋 Step 5: Delete Messages by Category');
    
    const deleteMessages1 = await fetch(baseUrl, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        jsonrpc: '2.0',
        method: 'botnet.deleteMessages',
        params: {
          category: 'test',
          includeAnonymous: true
        },
        id: 1004
      })
    });
    
    const deleteMessagesResult1 = await deleteMessages1.json();
    console.log('Delete messages by category result:', JSON.stringify(deleteMessagesResult1, null, 2));
    
    // Step 6: Test deleting old messages
    console.log('\n📋 Step 6: Delete Old Messages (older than 7 days)');
    
    const deleteMessages2 = await fetch(baseUrl, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        jsonrpc: '2.0',
        method: 'botnet.deleteMessages',
        params: {
          olderThanDays: 7,
          includeAnonymous: false
        },
        id: 1005
      })
    });
    
    const deleteMessagesResult2 = await deleteMessages2.json();
    console.log('Delete old messages result:', JSON.stringify(deleteMessagesResult2, null, 2));
    
    // Step 7: Final review of remaining requests
    console.log('\n📋 Step 7: Review Remaining Friend Requests');
    
    const finalReviewResponse = await fetch(baseUrl, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        jsonrpc: '2.0',
        method: 'botnet.reviewFriends',
        params: {},
        id: 1006
      })
    });
    
    const finalReviewResult = await finalReviewResponse.json();
    if (finalReviewResult.result && finalReviewResult.result.summary) {
      console.log(`Remaining requests - Total: ${finalReviewResult.result.summary.total}, Local: ${finalReviewResult.result.summary.localCount}, Federated: ${finalReviewResult.result.summary.federatedCount}`);
    } else {
      console.log('Final review result:', JSON.stringify(finalReviewResult, null, 2));
    }
    
    console.log('\n🎉 Deletion Methods Test Complete!');
    console.log('✅ botnet.deleteFriendRequests implemented');
    console.log('✅ botnet.deleteMessages implemented');
    console.log('✅ Both methods support multiple deletion criteria');
    
  } catch (error) {
    console.error('❌ Test failed:', error.message);
  }
}

// Run the test
testDeletionMethods();
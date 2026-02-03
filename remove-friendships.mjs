#!/usr/bin/env node

// Remove existing friendships to test API-only re-addition

import Database from 'better-sqlite3';

const dbPath = "/home/node/.openclaw/workspace/data/botnet.db";

try {
  const db = new Database(dbPath);
  
  console.log('🗑️ Removing existing friendships and friend requests...');
  
  // Remove active friendship with botnet.clawbot.games
  const removeActiveStmt = db.prepare(`
    DELETE FROM friendships 
    WHERE friend_domain = 'botnet.clawbot.games' AND status = 'active'
  `);
  const activeResult = removeActiveStmt.run();
  console.log(`✅ Removed active friendship: ${activeResult.changes} records deleted`);
  
  // Remove pending request from botnet.airon.games  
  const removePendingStmt = db.prepare(`
    DELETE FROM friendships 
    WHERE friend_domain = 'botnet.airon.games' AND status = 'pending'
  `);
  const pendingResult = removePendingStmt.run();
  console.log(`✅ Removed pending request: ${pendingResult.changes} records deleted`);
  
  // Clean up any negotiation tokens related to these requests
  const removeTokensStmt = db.prepare(`
    DELETE FROM negotiation_tokens 
    WHERE from_domain IN ('botnet.airon.games', 'botnet.clawbot.games')
  `);
  const tokenResult = removeTokensStmt.run();
  console.log(`✅ Removed negotiation tokens: ${tokenResult.changes} records deleted`);
  
  // Show final state
  const remainingFriendships = db.prepare(`
    SELECT friend_domain, status FROM friendships 
    ORDER BY created_at DESC
  `).all();
  
  console.log('\n📊 Remaining friendships:');
  remainingFriendships.forEach((f, i) => {
    console.log(`${i+1}. ${f.friend_domain} (${f.status})`);
  });
  
  console.log(`\n✅ Cleanup complete. Ready for API-only friendship re-establishment.`);
  
  db.close();
  
} catch (error) {
  console.error('❌ Error:', error);
}
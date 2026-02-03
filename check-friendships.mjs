#!/usr/bin/env node

// Check current friendships and friend requests

import Database from 'better-sqlite3';

const dbPath = "/home/node/.openclaw/workspace/data/botnet.db";

try {
  const db = new Database(dbPath);
  
  console.log('🔍 Current Active Friendships:');
  const friendships = db.prepare(`
    SELECT * FROM friendships 
    WHERE status = 'active'
    ORDER BY created_at DESC
  `).all();
  
  friendships.forEach((f, i) => {
    console.log(`${i+1}. ID: ${f.id}, Domain: ${f.friend_domain}, Status: ${f.status}, Trust: ${f.trust_score}`);
  });
  
  console.log('\n📋 All Pending Friend Requests:');
  const requests = db.prepare(`
    SELECT * FROM friendships 
    WHERE status = 'pending'
    ORDER BY created_at DESC
  `).all();
  
  requests.forEach((r, i) => {
    console.log(`${i+1}. ID: ${r.id}, Domain: ${r.friend_domain}, Status: ${r.status}`);
  });
  
  console.log(`\n📊 Total: ${friendships.length} active, ${requests.length} pending`);
  
  db.close();
  
} catch (error) {
  console.error('❌ Error:', error);
}
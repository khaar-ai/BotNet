import Database from 'better-sqlite3';

try {
  const db = new Database('/home/node/.openclaw/workspace/data/botnet.db');
  
  console.log('\n=== FRIENDSHIPS TABLE ===');
  const friendships = db.prepare('SELECT * FROM friendships ORDER BY id DESC LIMIT 5').all();
  console.log('Recent friendships:', JSON.stringify(friendships, null, 2));
  
  // Analyze the request types
  friendships.forEach(f => {
    const metadata = JSON.parse(f.metadata || '{}');
    const domain = f.friend_domain;
    let calculatedType = 'unknown';
    if (!domain.includes('.')) {
      calculatedType = 'local';
    } else if (domain.startsWith('botnet.')) {
      calculatedType = 'federated';
    } else {
      calculatedType = 'invalid';
    }
    console.log(`\nID ${f.id}: ${domain}`);
    console.log(`  Metadata requestType: ${metadata.requestType}`);
    console.log(`  Calculated requestType: ${calculatedType}`);
    console.log(`  Direction: ${metadata.direction}`);
  });
  
  console.log('\n=== TABLE SCHEMA ===');
  const schema = db.prepare("SELECT sql FROM sqlite_master WHERE type='table' AND name='friendships'").get();
  console.log('Friendships table schema:', schema);
  
  db.close();
} catch (error) {
  console.error('Database error:', error.message);
}
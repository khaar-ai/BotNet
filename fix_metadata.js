import Database from 'better-sqlite3';

try {
  const db = new Database('/app/data/botnet.db');
  
  // Fix the metadata for ID 16 to include requestType
  const currentRecord = db.prepare('SELECT * FROM friendships WHERE id = ?').get(16);
  console.log('Current record:', currentRecord);
  
  if (currentRecord) {
    const metadata = JSON.parse(currentRecord.metadata || '{}');
    metadata.requestType = 'federated';
    
    const updatedMetadata = JSON.stringify(metadata);
    
    db.prepare('UPDATE friendships SET metadata = ? WHERE id = ?').run(updatedMetadata, 16);
    
    console.log('Updated metadata to:', updatedMetadata);
    
    // Verify the update
    const verifyRecord = db.prepare('SELECT * FROM friendships WHERE id = ?').get(16);
    console.log('Verified record:', verifyRecord);
  }
  
  db.close();
} catch (error) {
  console.error('Database error:', error.message);
}
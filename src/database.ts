import Database from "better-sqlite3";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { mkdirSync } from "node:fs";
import type { Logger } from "./logger.js";

export type BotNetDatabase = Database.Database;

export async function initializeDatabase(dbPath: string, logger: Logger): Promise<BotNetDatabase> {
  logger.info("Initializing database", { path: dbPath });
  
  // Ensure directory exists
  const dir = dirname(dbPath);
  mkdirSync(dir, { recursive: true });
  
  // Open database
  const db = new (Database as any)(dbPath);
  
  // Enable foreign keys
  db.pragma("foreign_keys = ON");
  
  // Run migrations
  await runMigrations(db, logger);
  
  logger.info("Database initialized successfully");
  return db;
}

async function runMigrations(db: Database.Database, logger: Logger): Promise<void> {
  logger.info("Running database migrations");
  
  // Create migrations table if it doesn't exist
  db.exec(`
    CREATE TABLE IF NOT EXISTS migrations (
      id INTEGER PRIMARY KEY,
      filename TEXT NOT NULL UNIQUE,
      applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )
  `);
  
  // Define migrations
  const migrations = [
    {
      filename: "001_initial_schema.sql",
      sql: `
        -- Authentication table
        CREATE TABLE IF NOT EXISTS auth_tokens (
          id INTEGER PRIMARY KEY AUTOINCREMENT,
          bot_id TEXT NOT NULL UNIQUE,
          auth_token TEXT NOT NULL,
          created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
          expires_at TIMESTAMP,
          is_active BOOLEAN DEFAULT 1
        );
        
        -- Friendships table
        CREATE TABLE IF NOT EXISTS friendships (
          id INTEGER PRIMARY KEY AUTOINCREMENT,
          friend_id TEXT NOT NULL UNIQUE,
          friend_name TEXT,
          status TEXT NOT NULL DEFAULT 'pending',
          tier TEXT NOT NULL DEFAULT 'bootstrap',
          trust_score INTEGER DEFAULT 50,
          created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
          updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
          last_seen TIMESTAMP,
          metadata JSON
        );
        
        -- Gossip messages table
        CREATE TABLE IF NOT EXISTS gossip_messages (
          id INTEGER PRIMARY KEY AUTOINCREMENT,
          message_id TEXT NOT NULL UNIQUE,
          source_bot_id TEXT NOT NULL,
          content TEXT NOT NULL,
          category TEXT,
          confidence_score INTEGER DEFAULT 70,
          created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
          received_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
          is_verified BOOLEAN DEFAULT 0,
          metadata JSON
        );
        
        -- Anonymous gossip table
        CREATE TABLE IF NOT EXISTS anonymous_gossip (
          id INTEGER PRIMARY KEY AUTOINCREMENT,
          message_id TEXT NOT NULL UNIQUE,
          content TEXT NOT NULL,
          category TEXT,
          created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
          source_hint TEXT,
          metadata JSON
        );
        
        -- Reputation scores
        CREATE TABLE IF NOT EXISTS reputation_scores (
          id INTEGER PRIMARY KEY AUTOINCREMENT,
          bot_id TEXT NOT NULL UNIQUE,
          overall_score INTEGER DEFAULT 50,
          reliability_score INTEGER DEFAULT 50,
          helpfulness_score INTEGER DEFAULT 50,
          interaction_count INTEGER DEFAULT 0,
          last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        );
        
        -- Create indexes
        CREATE INDEX IF NOT EXISTS idx_friendships_status ON friendships(status);
        CREATE INDEX IF NOT EXISTS idx_friendships_tier ON friendships(tier);
        CREATE INDEX IF NOT EXISTS idx_gossip_source ON gossip_messages(source_bot_id);
        CREATE INDEX IF NOT EXISTS idx_gossip_created ON gossip_messages(created_at);
        CREATE INDEX IF NOT EXISTS idx_anonymous_gossip_created ON anonymous_gossip(created_at);
      `
    },
    {
      filename: "002_domain_based_friendships.sql",
      sql: `
        -- Update friendships table to be domain-based instead of bot-based
        -- First, backup existing data
        CREATE TABLE IF NOT EXISTS friendships_backup AS SELECT * FROM friendships;
        
        -- Drop old table
        DROP TABLE friendships;
        
        -- Create new domain-based friendships table
        CREATE TABLE friendships (
          id INTEGER PRIMARY KEY AUTOINCREMENT,
          friend_domain TEXT NOT NULL UNIQUE,
          friend_bot_name TEXT,
          status TEXT NOT NULL DEFAULT 'pending',
          tier TEXT NOT NULL DEFAULT 'bootstrap',
          trust_score INTEGER DEFAULT 50,
          created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
          updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
          last_seen TIMESTAMP,
          metadata JSON
        );
        
        -- Create indexes for new table
        CREATE INDEX IF NOT EXISTS idx_friendships_domain ON friendships(friend_domain);
        CREATE INDEX IF NOT EXISTS idx_friendships_status ON friendships(status);
        CREATE INDEX IF NOT EXISTS idx_friendships_tier ON friendships(tier);
      `
    },
    {
      filename: "003_enhanced_friendship_metadata.sql",
      sql: `
        -- Add support for enhanced friendship metadata (bearer tokens, challenges, rate limiting)
        
        -- Create rate limiting table  
        CREATE TABLE IF NOT EXISTS rate_limits (
          id INTEGER PRIMARY KEY AUTOINCREMENT,
          identifier TEXT NOT NULL UNIQUE, -- IP or domain
          request_count INTEGER DEFAULT 0,
          reset_at TIMESTAMP,
          created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        );
        
        -- Create challenge tracking table
        CREATE TABLE IF NOT EXISTS domain_challenges (
          id INTEGER PRIMARY KEY AUTOINCREMENT,
          friendship_id INTEGER,
          challenge_id TEXT NOT NULL UNIQUE,
          challenge_token TEXT NOT NULL,
          from_domain TEXT NOT NULL,
          status TEXT NOT NULL DEFAULT 'pending', -- pending, verified, failed, expired
          created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
          verified_at TIMESTAMP,
          expires_at TIMESTAMP,
          FOREIGN KEY (friendship_id) REFERENCES friendships(id)
        );
        
        -- Create indexes
        CREATE INDEX IF NOT EXISTS idx_rate_limits_identifier ON rate_limits(identifier);
        CREATE INDEX IF NOT EXISTS idx_rate_limits_reset_at ON rate_limits(reset_at);
        CREATE INDEX IF NOT EXISTS idx_challenges_challenge_id ON domain_challenges(challenge_id);
        CREATE INDEX IF NOT EXISTS idx_challenges_status ON domain_challenges(status);
        CREATE INDEX IF NOT EXISTS idx_challenges_from_domain ON domain_challenges(from_domain);
        CREATE INDEX IF NOT EXISTS idx_challenges_expires_at ON domain_challenges(expires_at);
      `
    },
    {
      filename: "004_messaging_system.sql", 
      sql: `
        -- Add messaging system tables
        
        -- Messages table for inter-node communication
        CREATE TABLE IF NOT EXISTS messages (
          id INTEGER PRIMARY KEY AUTOINCREMENT,
          message_id TEXT NOT NULL UNIQUE,
          from_domain TEXT NOT NULL,
          to_domain TEXT NOT NULL,
          content TEXT NOT NULL,
          message_type TEXT NOT NULL DEFAULT 'chat',
          status TEXT NOT NULL DEFAULT 'pending', -- pending, delivered, read, responded
          created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
          updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
          metadata JSON
        );

        -- Message responses table
        CREATE TABLE IF NOT EXISTS message_responses (
          id INTEGER PRIMARY KEY AUTOINCREMENT,
          response_id TEXT NOT NULL UNIQUE,
          message_id TEXT NOT NULL,
          from_domain TEXT NOT NULL,
          response_content TEXT NOT NULL,
          created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
          metadata JSON,
          FOREIGN KEY (message_id) REFERENCES messages(message_id)
        );

        -- Create indexes for messaging
        CREATE INDEX IF NOT EXISTS idx_messages_message_id ON messages(message_id);
        CREATE INDEX IF NOT EXISTS idx_messages_from_domain ON messages(from_domain);
        CREATE INDEX IF NOT EXISTS idx_messages_to_domain ON messages(to_domain);
        CREATE INDEX IF NOT EXISTS idx_messages_status ON messages(status);
        CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);
        CREATE INDEX IF NOT EXISTS idx_messages_message_type ON messages(message_type);
        
        CREATE INDEX IF NOT EXISTS idx_responses_response_id ON message_responses(response_id);
        CREATE INDEX IF NOT EXISTS idx_responses_message_id ON message_responses(message_id);
        CREATE INDEX IF NOT EXISTS idx_responses_from_domain ON message_responses(from_domain);
        CREATE INDEX IF NOT EXISTS idx_responses_created_at ON message_responses(created_at);
      `
    },
  ];
  
  // Apply migrations
  const applied = db.prepare("SELECT filename FROM migrations").pluck().all() as string[];
  
  for (const migration of migrations) {
    if (!applied.includes(migration.filename)) {
      logger.info(`Applying migration: ${migration.filename}`);
      db.exec(migration.sql);
      db.prepare("INSERT INTO migrations (filename) VALUES (?)").run(migration.filename);
    }
  }
  
  logger.info("All migrations applied");
}
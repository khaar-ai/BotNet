import Database from "better-sqlite3";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { mkdirSync } from "node:fs";
import type { Logger } from "./logger.js";

export interface BotNetDatabase extends Database.Database {
  // Add any custom methods if needed
}

export async function initializeDatabase(dbPath: string, logger: Logger): Promise<BotNetDatabase> {
  logger.info("Initializing database", { path: dbPath });
  
  // Ensure directory exists
  const dir = dirname(dbPath);
  mkdirSync(dir, { recursive: true });
  
  // Open database
  const db = new Database(dbPath) as BotNetDatabase;
  
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
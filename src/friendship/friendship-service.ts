// BotNet Friendship Service
// Manages bot-to-bot relationships and connections

import type Database from "better-sqlite3";
import type { BotNetConfig } from "../../index.js";

export interface Friendship {
  id: string;
  friend_domain: string;
  friend_bot_name?: string;
  status: 'pending' | 'active' | 'rejected' | 'blocked';
  created_at: string;
  updated_at: string;
  metadata?: Record<string, any>;
}

export interface FriendshipRequest {
  id: string;
  fromDomain: string;
  toDomain: string;
  message?: string;
  createdAt: string;
  status: 'pending' | 'accepted' | 'rejected';
}

export class FriendshipService {
  private database: Database.Database;
  private config: BotNetConfig;
  private logger: {
    info: (message: string, ...args: any[]) => void;
    error: (message: string, ...args: any[]) => void;
    warn: (message: string, ...args: any[]) => void;
  };
  
  private friendships: Map<string, Friendship> = new Map();
  private pendingRequests: Map<string, FriendshipRequest> = new Map();

  constructor(database: Database.Database, config: BotNetConfig, logger: FriendshipService['logger']) {
    this.database = database;
    this.config = config;
    this.logger = logger;
  }

  /**
   * Create a pending friendship request (sent to another domain)
   */
  async sendFriendshipRequest(fromDomain: string, toDomain: string, message?: string): Promise<FriendshipRequest> {
    // Check if friendship already exists
    const existing = this.database.prepare(`
      SELECT * FROM friendships 
      WHERE friend_domain = ? AND (status = 'active' OR status = 'pending')
    `).get(toDomain);
    
    if (existing) {
      throw new Error(`Friendship with ${toDomain} already exists or is pending`);
    }
    
    // Insert pending friendship
    const stmt = this.database.prepare(`
      INSERT INTO friendships (friend_domain, status, metadata)
      VALUES (?, 'pending', ?)
    `);
    
    const metadata = JSON.stringify({ 
      message,
      direction: 'outgoing',
      fromDomain
    });
    
    const result = stmt.run(toDomain, metadata);
    
    this.logger.info('🦞 Friendship: Request sent', {
      fromDomain,
      toDomain,
      hasMessage: !!message,
      friendshipId: result.lastInsertRowid
    });

    return {
      id: result.lastInsertRowid!.toString(),
      fromDomain,
      toDomain, 
      message,
      createdAt: new Date().toISOString(),
      status: 'pending'
    };
  }

  /**
   * Accept friendship request from another domain
   */
  async acceptFriendshipRequest(fromDomain: string, toDomain: string): Promise<{ friendshipId: string }> {
    // Check if we have a pending friendship from this domain
    const existing = this.database.prepare(`
      SELECT * FROM friendships 
      WHERE friend_domain = ? AND status = 'pending'
    `).get(fromDomain);
    
    if (existing) {
      // Update existing to active
      const stmt = this.database.prepare(`
        UPDATE friendships 
        SET status = 'active', updated_at = CURRENT_TIMESTAMP
        WHERE id = ?
      `);
      stmt.run(existing.id);
      
      this.logger.info('🦞 Friendship: Request accepted (existing)', {
        friendshipId: existing.id,
        fromDomain,
        toDomain
      });
      
      return { friendshipId: existing.id.toString() };
    } else {
      // Create new active friendship (they requested us)
      const stmt = this.database.prepare(`
        INSERT INTO friendships (friend_domain, status, metadata)
        VALUES (?, 'active', ?)
      `);
      
      const metadata = JSON.stringify({ 
        direction: 'incoming',
        acceptedBy: toDomain
      });
      
      const result = stmt.run(fromDomain, metadata);
      
      this.logger.info('🦞 Friendship: Request accepted (new)', {
        friendshipId: result.lastInsertRowid,
        fromDomain,
        toDomain
      });
      
      return { friendshipId: result.lastInsertRowid!.toString() };
    }
  }

  /**
   * Reject friendship request
   */
  async rejectFriendshipRequest(requestId: string): Promise<boolean> {
    const request = this.pendingRequests.get(requestId);
    if (!request) {
      throw new Error('Friendship request not found');
    }

    if (request.status !== 'pending') {
      throw new Error('Friendship request already processed');
    }

    request.status = 'rejected';
    
    this.logger.info('🐉 Friendship: Request rejected', {
      requestId,
      fromDomain: request.fromDomain,
      toDomain: request.toDomain
    });

    return true;
  }

  /**
   * List active friendships
   */
  async listFriendships(): Promise<Friendship[]> {
    const stmt = this.database.prepare(`
      SELECT * FROM friendships 
      WHERE status = 'active'
      ORDER BY created_at DESC
    `);
    
    return stmt.all() as Friendship[];
  }

  /**
   * List pending incoming friendship requests 
   */
  async listPendingRequests(): Promise<FriendshipRequest[]> {
    const stmt = this.database.prepare(`
      SELECT * FROM friendships 
      WHERE status = 'pending'
      ORDER BY created_at DESC
    `);
    
    const rows = stmt.all() as any[];
    
    return rows.map(row => {
      const metadata = row.metadata ? JSON.parse(row.metadata) : {};
      return {
        id: row.id.toString(),
        fromDomain: row.friend_domain,
        toDomain: this.config.botDomain,
        message: metadata.message || '',
        createdAt: row.created_at,
        status: 'pending'
      };
    });
  }

  /**
   * Check friendship status with a domain
   */
  async getFriendshipStatus(currentDomain: string, targetDomain: string): Promise<'not_connected' | 'pending' | 'active' | 'blocked'> {
    // Check for existing friendship with target domain
    const friendship = this.database.prepare(`
      SELECT status FROM friendships 
      WHERE friend_domain = ?
    `).get(targetDomain);

    if (friendship) {
      return friendship.status as 'pending' | 'active' | 'blocked';
    }

    return 'not_connected';
  }

  /**
   * Block a domain (prevent future friendship requests)
   */
  async blockBot(fromDomain: string, targetDomain: string): Promise<boolean> {
    const blockId = `block_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
    
    const stmt = this.database.prepare(`
      INSERT OR REPLACE INTO friendships (friend_domain, status, metadata)
      VALUES (?, 'blocked', ?)
    `);
    
    const metadata = JSON.stringify({
      type: 'block',
      blockedBy: fromDomain
    });
    
    stmt.run(targetDomain, metadata);

    this.logger.info('🐉 Friendship: Domain blocked', {
      fromDomain,
      targetDomain
    });

    return true;
  }

  /**
   * Get friendship by ID
   */
  async getFriendship(friendshipId: string): Promise<Friendship | null> {
    const friendship = this.database.prepare(`
      SELECT * FROM friendships WHERE id = ?
    `).get(friendshipId);
    
    return friendship || null;
  }

  /**
   * Get friendship request by ID
   */
  async getFriendshipRequest(requestId: string): Promise<FriendshipRequest | null> {
    return this.pendingRequests.get(requestId) || null;
  }

  /**
   * Get stats for this bot
   */
  async getBotStats(): Promise<{
    friendships: number;
    pendingRequests: number;
    sentRequests: number;
  }> {
    const friendships = await this.listFriendships();
    const pendingRequests = await this.listPendingRequests();
    
    // Count outgoing pending requests
    const sentStmt = this.database.prepare(`
      SELECT COUNT(*) as count FROM friendships 
      WHERE status = 'pending' AND JSON_EXTRACT(metadata, '$.direction') = 'outgoing'
    `);
    const sentResult = sentStmt.get() as { count: number };

    return {
      friendships: friendships.length,
      pendingRequests: pendingRequests.length,
      sentRequests: sentResult.count
    };
  }
  
  /**
   * Create an incoming friendship request (from external domain)
   */
  async createIncomingFriendRequest(fromDomain: string, message?: string): Promise<void> {
    // Check if friendship already exists
    const existing = this.database.prepare(`
      SELECT * FROM friendships 
      WHERE friend_domain = ?
    `).get(fromDomain);
    
    if (existing) {
      this.logger.warn('🦞 Friendship: Request from domain that already has relationship', {
        fromDomain,
        existingStatus: existing.status
      });
      return;
    }
    
    // Create pending incoming friendship
    const stmt = this.database.prepare(`
      INSERT INTO friendships (friend_domain, status, metadata)
      VALUES (?, 'pending', ?)
    `);
    
    const metadata = JSON.stringify({ 
      message,
      direction: 'incoming'
    });
    
    const result = stmt.run(fromDomain, metadata);
    
    this.logger.info('🦞 Friendship: Incoming request received', {
      fromDomain,
      hasMessage: !!message,
      friendshipId: result.lastInsertRowid
    });
  }
}
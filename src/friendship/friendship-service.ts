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
  status: 'pending' | 'accepted' | 'rejected' | 'challenging' | 'challenge_failed';
  bearerToken?: string;
  requestType: 'local' | 'federated'; // local = no dots, federated = botnet.*
  lastChallengeAt?: string;
  challengeAttempts: number;
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
  private rateLimitMap: Map<string, { count: number; resetAt: number }> = new Map();
  private readonly RATE_LIMIT_WINDOW = 60 * 1000; // 1 minute
  private readonly RATE_LIMIT_MAX = 5; // 5 requests per minute per IP/domain

  constructor(database: Database.Database, config: BotNetConfig, logger: FriendshipService['logger']) {
    this.database = database;
    this.config = config;
    this.logger = logger;
  }

  /**
   * Determine if a domain is local (no dots) or federated (botnet.*)
   */
  private determineRequestType(fromDomain: string): 'local' | 'federated' {
    if (!fromDomain.includes('.')) {
      return 'local'; // Names like "TestBot", "Alice", "DragonHelper"
    }
    if (fromDomain.startsWith('botnet.')) {
      return 'federated'; // Domains like "botnet.example.com"
    }
    // For now, treat other domains as federated too
    return 'federated';
  }

  /**
   * Generate bearer token for friendship request
   */
  private generateBearerToken(): string {
    const timestamp = Date.now().toString(36);
    const random = Math.random().toString(36).substring(2);
    return `bot_${timestamp}_${random}`;
  }

  /**
   * Check rate limit for domain/IP
   */
  private checkRateLimit(identifier: string): boolean {
    const now = Date.now();
    const rateLimitData = this.rateLimitMap.get(identifier);

    if (!rateLimitData || now > rateLimitData.resetAt) {
      // Reset or first request
      this.rateLimitMap.set(identifier, { count: 1, resetAt: now + this.RATE_LIMIT_WINDOW });
      return true;
    }

    if (rateLimitData.count >= this.RATE_LIMIT_MAX) {
      this.logger.warn('🚫 Rate limit exceeded', { identifier, count: rateLimitData.count });
      return false;
    }

    rateLimitData.count++;
    return true;
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
      status: 'pending',
      requestType: this.determineRequestType(fromDomain),
      challengeAttempts: 0
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
   * List pending friendship requests for review
   * Returns categorized requests: local (trusted) vs federated (needs challenge)
   */
  async listPendingRequests(): Promise<{
    local: FriendshipRequest[];
    federated: FriendshipRequest[];
    summary: { total: number; localCount: number; federatedCount: number; };
  }> {
    const stmt = this.database.prepare(`
      SELECT * FROM friendships 
      WHERE status IN ('pending', 'challenging')
      ORDER BY created_at DESC
    `);
    
    const rows = stmt.all() as any[];
    
    const local: FriendshipRequest[] = [];
    const federated: FriendshipRequest[] = [];
    
    rows.forEach(row => {
      const metadata = row.metadata ? JSON.parse(row.metadata) : {};
      const requestType = metadata.requestType || this.determineRequestType(row.friend_domain);
      
      const request: FriendshipRequest = {
        id: row.id.toString(),
        fromDomain: row.friend_domain,
        toDomain: this.config.botDomain,
        message: metadata.message || '',
        createdAt: row.created_at,
        status: row.status,
        bearerToken: metadata.bearerToken,
        requestType: requestType,
        lastChallengeAt: metadata.lastChallengeAt,
        challengeAttempts: metadata.challengeAttempts || 0
      };
      
      if (requestType === 'local') {
        local.push(request);
      } else {
        federated.push(request);
      }
    });
    
    return {
      local,
      federated,
      summary: {
        total: local.length + federated.length,
        localCount: local.length,
        federatedCount: federated.length
      }
    };
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
   * Initiate domain verification challenge for federated friend request
   */
  async initiateDomainChallenge(requestId: string): Promise<{ challengeId: string; status: string }> {
    // Get the friendship request
    const friendship = this.database.prepare(`
      SELECT * FROM friendships WHERE id = ?
    `).get(requestId);
    
    if (!friendship) {
      throw new Error('Friendship request not found');
    }
    
    const metadata = JSON.parse(friendship.metadata || '{}');
    
    if (metadata.requestType !== 'federated') {
      throw new Error('Domain challenge only applicable to federated requests');
    }
    
    if (friendship.status !== 'pending') {
      throw new Error('Can only challenge pending requests');
    }
    
    // Generate challenge
    const challengeId = `challenge_${Date.now()}_${Math.random().toString(36).substring(2)}`;
    const challengeToken = Math.random().toString(36).substring(2) + Date.now().toString(36);
    
    // Update friendship status to challenging
    const updatedMetadata = {
      ...metadata,
      challengeId,
      challengeToken,
      lastChallengeAt: new Date().toISOString(),
      challengeAttempts: (metadata.challengeAttempts || 0) + 1
    };
    
    this.database.prepare(`
      UPDATE friendships 
      SET status = 'challenging', metadata = ?, updated_at = CURRENT_TIMESTAMP
      WHERE id = ?
    `).run(JSON.stringify(updatedMetadata), requestId);
    
    this.logger.info('🔐 Domain challenge initiated', {
      fromDomain: friendship.friend_domain,
      challengeId,
      attempt: updatedMetadata.challengeAttempts
    });
    
    // TODO: Send actual HTTP challenge to the domain
    // For now, just return the challenge info
    return {
      challengeId,
      status: 'challenge_sent'
    };
  }

  /**
   * Verify domain challenge response
   */
  async verifyDomainChallenge(challengeId: string, response: string): Promise<{ verified: boolean; friendshipId?: string }> {
    // Find friendship by challenge ID
    const friendships = this.database.prepare(`
      SELECT * FROM friendships WHERE status = 'challenging'
    `).all();
    
    const friendship = friendships.find((f: any) => {
      const metadata = JSON.parse(f.metadata || '{}');
      return metadata.challengeId === challengeId;
    });
    
    if (!friendship) {
      throw new Error('Challenge not found or expired');
    }
    
    const metadata = JSON.parse(friendship.metadata || '{}');
    const expectedToken = metadata.challengeToken;
    
    if (response === expectedToken) {
      // Challenge successful - activate friendship
      const updatedMetadata = {
        ...metadata,
        verifiedAt: new Date().toISOString()
      };
      
      this.database.prepare(`
        UPDATE friendships 
        SET status = 'active', metadata = ?, updated_at = CURRENT_TIMESTAMP
        WHERE id = ?
      `).run(JSON.stringify(updatedMetadata), friendship.id);
      
      this.logger.info('✅ Domain challenge verified', {
        fromDomain: friendship.friend_domain,
        challengeId
      });
      
      return { verified: true, friendshipId: friendship.id.toString() };
    } else {
      // Challenge failed
      this.database.prepare(`
        UPDATE friendships 
        SET status = 'challenge_failed', updated_at = CURRENT_TIMESTAMP
        WHERE id = ?
      `).run(friendship.id);
      
      this.logger.warn('❌ Domain challenge failed', {
        fromDomain: friendship.friend_domain,
        challengeId
      });
      
      return { verified: false };
    }
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
      pendingRequests: pendingRequests.summary.total,
      sentRequests: sentResult.count
    };
  }
  
  /**
   * Create an incoming friendship request (with rate limiting and bearer tokens)
   */
  async createIncomingFriendRequest(fromDomain: string, message?: string, clientIP?: string): Promise<{ bearerToken: string; status: string }> {
    // Rate limiting check
    const rateLimitKey = clientIP || fromDomain;
    if (!this.checkRateLimit(rateLimitKey)) {
      throw new Error('Rate limit exceeded. Please try again later.');
    }

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
      throw new Error(`Friendship with ${fromDomain} already exists (status: ${existing.status})`);
    }

    // Determine request type and generate bearer token
    const requestType = this.determineRequestType(fromDomain);
    const bearerToken = this.generateBearerToken();
    
    // Create pending incoming friendship
    const stmt = this.database.prepare(`
      INSERT INTO friendships (friend_domain, status, metadata)
      VALUES (?, 'pending', ?)
    `);
    
    const metadata = JSON.stringify({ 
      message,
      direction: 'incoming',
      requestType,
      bearerToken,
      challengeAttempts: 0,
      createdAt: new Date().toISOString()
    });
    
    const result = stmt.run(fromDomain, metadata);
    
    this.logger.info('🦞 Friendship: Incoming request received', {
      fromDomain,
      requestType,
      hasMessage: !!message,
      bearerToken: bearerToken.substring(0, 12) + '...', // Log partial token
      friendshipId: result.lastInsertRowid
    });

    return {
      bearerToken,
      status: requestType === 'local' ? 'pending_review' : 'pending_challenge_review'
    };
  }
}
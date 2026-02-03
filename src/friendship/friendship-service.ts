// BotNet Friendship Service
// Manages bot-to-bot relationships and connections

import type Database from "better-sqlite3";
import type { BotNetConfig } from "../../index.js";
import { RateLimiter } from "../rate-limiter.js";
import type { MCPClient } from "../mcp/mcp-client.js";

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
  private mcpClient: MCPClient;
  
  private friendships: Map<string, Friendship> = new Map();
  private pendingRequests: Map<string, FriendshipRequest> = new Map();
  private rateLimiter: RateLimiter;

  constructor(database: Database.Database, config: BotNetConfig, logger: FriendshipService['logger'], mcpClient: MCPClient) {
    this.database = database;
    this.config = config;
    this.logger = logger;
    this.mcpClient = mcpClient;
    this.rateLimiter = new RateLimiter(logger, 60 * 1000, 5); // 5 friendship ops per minute
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
   * List active friends (with rate limiting)
   */
  async listFriends(clientIP?: string): Promise<Friendship[]> {
    // Rate limiting
    const rateLimitKey = clientIP || this.config.botDomain;
    if (!this.rateLimiter.checkRateLimit(rateLimitKey, 'listFriends')) {
      throw new Error('Rate limit exceeded for listing friends. Please try again later.');
    }

    const stmt = this.database.prepare(`
      SELECT * FROM friendships 
      WHERE status = 'active'
      ORDER BY updated_at DESC
    `);
    
    const friendships = stmt.all() as Friendship[];
    
    this.logger.info('👥 Friends listed', {
      count: friendships.length,
      requestedBy: rateLimitKey
    });
    
    return friendships;
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
    
    // Send actual challenge to remote domain via MCP
    try {
      const challengeResult = await this.mcpClient.sendDomainChallenge(
        friendship.friend_domain,
        challengeId,
        challengeToken
      );
      
      if (challengeResult.success) {
        this.logger.info('✅ Domain challenge sent successfully', {
          fromDomain: friendship.friend_domain,
          challengeId
        });
      } else {
        this.logger.warn('⚠️ Domain challenge failed to send', {
          fromDomain: friendship.friend_domain,
          challengeId,
          error: challengeResult.error
        });
      }
      
      // Return challenge info regardless of send success (stored locally)
      return {
        challengeId,
        status: challengeResult.success ? 'challenge_sent' : 'challenge_send_failed'
      };
    } catch (error) {
      this.logger.error('🔥 Failed to send domain challenge', {
        fromDomain: friendship.friend_domain,
        challengeId,
        error
      });
      
      // Return challenge info even if send failed (challenge is stored locally)
      return {
        challengeId,
        status: 'challenge_send_failed'
      };
    }
  }

  /**
   * Accept friend by request ID - handles both local acceptance and automatic federated challenges with verification
   */
  async acceptFriendByRequestId(requestId: string, challengeResponse?: string): Promise<{ status: string; friendshipId?: string; challengeId?: string; message?: string }> {
    // Get the friendship request
    const friendship = this.database.prepare(`
      SELECT * FROM friendships WHERE id = ?
    `).get(requestId);
    
    if (!friendship) {
      throw new Error('Friendship request not found');
    }
    
    if (friendship.status !== 'pending') {
      throw new Error(`Cannot add friend - request status is ${friendship.status}`);
    }
    
    const metadata = JSON.parse(friendship.metadata || '{}');
    const requestType = metadata.requestType || this.determineRequestType(friendship.friend_domain);
    
    if (requestType === 'local') {
      // Local bot - immediately accept
      const updatedMetadata = {
        ...metadata,
        acceptedAt: new Date().toISOString(),
        acceptedBy: this.config.botDomain
      };
      
      this.database.prepare(`
        UPDATE friendships 
        SET status = 'active', metadata = ?, updated_at = CURRENT_TIMESTAMP
        WHERE id = ?
      `).run(JSON.stringify(updatedMetadata), requestId);
      
      this.logger.info('✅ Local friend request accepted', {
        fromDomain: friendship.friend_domain,
        requestId,
        type: 'local'
      });
      
      return {
        status: 'accepted',
        friendshipId: requestId,
        message: 'Local friend request accepted immediately'
      };
    } else {
      // Federated domain - handle challenge process
      if (friendship.status === 'pending') {
        // First call - initiate challenge
        const challengeResult = await this.initiateDomainChallenge(requestId);
        return {
          status: 'challenge_sent',
          challengeId: challengeResult.challengeId,
          message: 'Federated domain challenge initiated - call acceptFriend again with challengeResponse'
        };
      } else if (friendship.status === 'challenging' && challengeResponse) {
        // Second call - verify challenge response
        const verificationResult = await this.verifyDomainChallenge(
          JSON.parse(friendship.metadata || '{}').challengeId,
          challengeResponse
        );
        
        if (verificationResult.verified) {
          return {
            status: 'accepted',
            friendshipId: verificationResult.friendshipId,
            message: 'Federated domain verified and friendship established'
          };
        } else {
          return {
            status: 'challenge_failed',
            message: 'Challenge verification failed - friendship rejected'
          };
        }
      } else {
        throw new Error('Invalid federated friendship state or missing challenge response');
      }
    }
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
   * Delete friendship request(s) by ID or criteria (with rate limiting)
   */
  async deleteFriendRequests(options: {
    requestId?: string;
    fromDomain?: string;
    status?: string;
    olderThanDays?: number;
  }, clientIP?: string): Promise<{ deletedCount: number; message: string }> {
    
    // Rate limiting
    const rateLimitKey = clientIP || this.config.botDomain;
    if (!this.rateLimiter.checkRateLimit(rateLimitKey, 'deleteFriendRequests')) {
      throw new Error('Rate limit exceeded for deleting friend requests. Please try again later.');
    }
    let whereClause = '1=1';
    let params: any[] = [];
    
    if (options.requestId) {
      whereClause += ' AND id = ?';
      params.push(options.requestId);
    }
    
    if (options.fromDomain) {
      whereClause += ' AND friend_domain = ?';
      params.push(options.fromDomain);
    }
    
    if (options.status) {
      whereClause += ' AND status = ?';
      params.push(options.status);
    }
    
    if (options.olderThanDays) {
      whereClause += ' AND created_at < datetime("now", "-" || ? || " days")';
      params.push(options.olderThanDays);
    }
    
    // Get count before deletion for logging
    const countStmt = this.database.prepare(`
      SELECT COUNT(*) as count FROM friendships WHERE ${whereClause}
    `);
    const countResult = countStmt.get(...params) as { count: number };
    
    if (countResult.count === 0) {
      return { deletedCount: 0, message: 'No matching friend requests found to delete' };
    }
    
    // Delete matching records
    const deleteStmt = this.database.prepare(`
      DELETE FROM friendships WHERE ${whereClause}
    `);
    const result = deleteStmt.run(...params);
    
    this.logger.info('🗑️ Friend requests deleted', {
      deletedCount: result.changes,
      criteria: options
    });
    
    return {
      deletedCount: result.changes || 0,
      message: `Deleted ${result.changes} friend request(s)`
    };
  }

  /**
   * Remove/unfriend an active friendship
   */
  async removeFriend(friendDomain: string, clientIP?: string): Promise<{ success: boolean; message: string }> {
    // Rate limiting
    const rateLimitKey = clientIP || this.config.botDomain;
    if (!this.rateLimiter.checkRateLimit(rateLimitKey, 'removeFriend')) {
      throw new Error('Rate limit exceeded for removing friends. Please try again later.');
    }

    // Find the active friendship
    const friendship = this.database.prepare(`
      SELECT * FROM friendships 
      WHERE friend_domain = ? AND status = 'active'
    `).get(friendDomain);

    if (!friendship) {
      return {
        success: false,
        message: `No active friendship found with ${friendDomain}`
      };
    }

    // Remove the friendship (delete the record)
    const deleteStmt = this.database.prepare(`
      DELETE FROM friendships WHERE id = ?
    `);
    deleteStmt.run(friendship.id);

    this.logger.info('💔 Friendship removed', {
      friendDomain,
      friendshipId: friendship.id,
      removedBy: this.config.botDomain
    });

    return {
      success: true,
      message: `Friendship with ${friendDomain} has been removed`
    };
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
    if (!this.rateLimiter.checkRateLimit(rateLimitKey, 'createFriendRequest')) {
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
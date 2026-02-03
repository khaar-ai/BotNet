import { v4 as uuidv4 } from "uuid";
import type Database from "better-sqlite3";
import type { BotNetConfig } from "../index.js";
import type { Logger } from "./logger.js";
import { AuthService } from "./auth/auth-service.js";
import { FriendshipService } from "./friendship/friendship-service.js";
import { GossipService } from "./gossip/gossip-service.js";
import { MessagingService } from "./messaging/messaging-service.js";
import { RateLimiter } from "./rate-limiter.js";
import { MCPClient } from "./mcp/mcp-client.js";
interface BotNetServiceOptions {
  database: Database.Database;
  config: BotNetConfig;
  logger: Logger;
}

export class BotNetService {
  private authService: AuthService;
  private friendshipService: FriendshipService;
  private gossipService: GossipService;
  private messagingService: MessagingService;
  private rateLimiter: RateLimiter;
  private mcpClient: MCPClient;
  private bearerTokens: Map<string, { domain: string; expiresAt: Date; createdAt: Date }> = new Map();
  
  constructor(private options: BotNetServiceOptions) {
    const { database, config, logger } = options;
    
    this.authService = new AuthService(logger.child("auth"));
    this.mcpClient = new MCPClient({
      logger: logger.child("mcpClient"),
      timeout: 15000, // 15 second timeout for federation calls
      retries: 2
    });
    this.friendshipService = new FriendshipService(database, config, logger.child("friendship"), this.mcpClient);
    this.gossipService = new GossipService(database, config, logger.child("gossip"));
    this.messagingService = new MessagingService(database, config, logger.child("messaging"));
    this.rateLimiter = new RateLimiter(logger.child("rateLimiter"), 60 * 1000, 10); // Universal rate limiter
  }
  
  async getBotProfile() {
    const { config } = this.options;
    return {
      id: `${config.botName}@${config.botDomain}`,
      name: config.botName,
      domain: config.botDomain,
      description: config.botDescription,
      capabilities: config.capabilities,
      tier: config.tier,
      version: "1.0.0",
      protocol_version: "1.0",
      endpoints: {
        mcp: "/api/botnet/mcp",
        profile: "/api/botnet/profile",
        health: "/api/botnet/health",
        friendship: "/api/botnet/friendship",
        gossip: "/api/botnet/gossip"
      }
    };
  }
  
  async getHealthStatus() {
    try {
      // Check database
      const dbCheck = this.options.database.prepare("SELECT 1").get();
      
      return {
        status: "healthy",
        timestamp: new Date().toISOString(),
        version: "1.0.0",
        checks: {
          database: dbCheck ? "ok" : "error",
          services: {
            auth: "ok",
            friendship: "ok",
            gossip: "ok"
          }
        }
      };
    } catch (error) {
      return {
        status: "unhealthy",
        timestamp: new Date().toISOString(),
        version: "1.0.0",
        error: error instanceof Error ? error.message : "Unknown error"
      };
    }
  }
  
  async handleMCPRequest(request: any) {
    const { logger } = this.options;
    logger.info("Handling MCP request", { type: request.type });
    
    // Route MCP requests based on type
    switch (request.type) {
      case "friendship.request":
        return await this.friendshipService.createIncomingFriendRequest(request.fromDomain, request.message, request.clientIP);
      
      case "friendship.accept":
        return await this.friendshipService.acceptFriendshipRequest(request.fromDomain, request.toDomain);
      
      case "friendship.reject":
        return await this.friendshipService.rejectFriendshipRequest(request.id);
      
      case "gossip.exchange":
        return await this.gossipService.handleExchange(request);
      
      case "gossip.query":
        return await this.gossipService.handleQuery(request);
      
      // case "auth.challenge":
      //   return await this.authService.handleChallenge(request);
      
      // case "auth.response":
      //   return await this.authService.handleResponse(request);
      
      default:
        return {
          success: false,
          error: `Unknown MCP request type: ${request.type}`
        };
    }
  }
  
  async getFriendships() {
    return this.friendshipService.listFriendships();
  }
  
  async requestFriendship(request: any) {
    return this.friendshipService.sendFriendshipRequest(request.fromDomain, request.toDomain, request.message);
  }
  
  async getFriendshipStatus(currentDomain: string, targetDomain: string) {
    return this.friendshipService.getFriendshipStatus(currentDomain, targetDomain);
  }
  
  async exchangeGossip(request: any) {
    return this.gossipService.exchangeMessages(request);
  }
  
  async getGossipNetwork() {
    return this.gossipService.getNetworkTopology();
  }
  
  async submitAnonymousGossip(request: any) {
    return this.gossipService.submitAnonymous(request);
  }
  
  async getReputation(botId: string) {
    const db = this.options.database;
    const stmt = db.prepare(`
      SELECT * FROM reputation_scores WHERE bot_id = ?
    `);
    
    let reputation = stmt.get(botId) as any;
    
    if (!reputation) {
      // Create default reputation
      const insertStmt = db.prepare(`
        INSERT INTO reputation_scores (bot_id, overall_score, reliability_score, helpfulness_score)
        VALUES (?, 50, 50, 50)
      `);
      insertStmt.run(botId);
      
      reputation = {
        bot_id: botId,
        overall_score: 50,
        reliability_score: 50,
        helpfulness_score: 50,
        interaction_count: 0
      };
    }
    
    return reputation;
  }
  
  /**
   * Send friend request to remote domain
   */
  async sendFriendRequest(friendHost: string, fromDomain: string): Promise<{ requestId: string }> {
    try {
      // First, record the outgoing request locally
      const request = await this.friendshipService.sendFriendshipRequest(fromDomain, friendHost);
      
      // Send actual friend request to remote domain via MCP
      try {
        const mcpResult = await this.mcpClient.sendFriendRequest(friendHost, fromDomain, `Friend request from ${fromDomain}`);
        
        if (mcpResult.success) {
          this.options.logger.info("✅ Friend request sent to remote domain", {
            friendHost,
            fromDomain,
            requestId: request.id,
            remoteRequestId: mcpResult.requestId
          });
        } else {
          this.options.logger.warn("⚠️ Failed to send friend request to remote domain", {
            friendHost,
            fromDomain,
            requestId: request.id,
            error: mcpResult.error
          });
        }
      } catch (error) {
        this.options.logger.error("🔥 Error sending friend request to remote domain", {
          friendHost,
          fromDomain,
          requestId: request.id,
          error
        });
      }
      
      return { requestId: request.id };
    } catch (error) {
      this.options.logger.error("🦞 Failed to send friend request", { friendHost, fromDomain, error });
      throw error;
    }
  }
  
  /**
   * Get pending friend requests for this domain (legacy method)
   */
  async getPendingFriendRequests(domainName: string): Promise<any[]> {
    const categorized = await this.friendshipService.listPendingRequests();
    // Return combined array for backward compatibility
    return [...categorized.local, ...categorized.federated];
  }
  
  /**
   * Accept friend request from another domain (legacy method)
   */
  async acceptFriendRequest(friendHost: string, domainName: string): Promise<{ friendshipId: string }> {
    try {
      const result = await this.friendshipService.acceptFriendshipRequest(friendHost, domainName);
      
      // Notify the remote domain that we accepted their friend request
      try {
        const notifyResult = await this.mcpClient.notifyFriendshipAccepted(friendHost, domainName, result.friendshipId);
        
        if (notifyResult.success) {
          this.options.logger.info("✅ Notified remote domain of friendship acceptance", {
            friendHost,
            domainName,
            friendshipId: result.friendshipId
          });
        } else {
          this.options.logger.warn("⚠️ Failed to notify remote domain of acceptance", {
            friendHost,
            domainName,
            friendshipId: result.friendshipId,
            error: notifyResult.error
          });
        }
      } catch (error) {
        this.options.logger.error("🔥 Error notifying remote domain of acceptance", {
          friendHost,
          domainName,
          friendshipId: result.friendshipId,
          error
        });
      }
      
      return result;
    } catch (error) {
      this.options.logger.error("🦞 Failed to accept friend request", { friendHost, domainName, error });
      throw error;
    }
  }

  /**
   * Accept friend by request ID - handles local acceptance and automatic federated challenges with verification
   * This replaces the old ddFriend method and integrates verifyChallenge internally
   */
  async acceptFriend(requestId: string, challengeResponse?: string): Promise<{ status: string; friendshipId?: string; challengeId?: string; message?: string }> {
    try {
      return await this.friendshipService.acceptFriendByRequestId(requestId, challengeResponse);
    } catch (error) {
      this.options.logger.error("🦞 Failed to accept friend", { requestId, error });
      throw error;
    }
  }

  /**
   * Remove an active friendship
   */
  async removeFriend(friendDomain: string, clientIP?: string): Promise<any> {
    try {
      return await this.friendshipService.removeFriend(friendDomain, clientIP);
    } catch (error) {
      this.options.logger.error("🦞 Failed to remove friend", { friendDomain, error });
      throw error;
    }
  }
  
  /**
   * Get list of active friends
   */
  async getFriends(domainName: string): Promise<any[]> {
    return await this.friendshipService.listFriendships();
  }

  /**
   * Get enhanced pending requests with categorization
   */
  async getEnhancedPendingRequests(): Promise<any> {
    return await this.friendshipService.listPendingRequests();
  }

  /**
   * Verify domain challenge response (for external domains responding to our challenges)
   */
  async verifyChallenge(challengeId: string, response: string): Promise<any> {
    return await this.friendshipService.verifyDomainChallenge(challengeId, response);
  }

  /**
   * Check responses for external agents via MCP
   */
  async checkAgentResponses(agentId: string, targetDomain?: string): Promise<any> {
    if (!targetDomain) {
      // Local check - look in our message responses table
      const stmt = this.options.database.prepare(`
        SELECT mr.*, m.from_domain, m.content as original_content
        FROM message_responses mr
        JOIN messages m ON mr.message_id = m.message_id
        WHERE mr.from_domain = ?
        ORDER BY mr.created_at DESC
        LIMIT 10
      `);
      
      const responses = stmt.all(agentId);
      
      return {
        status: 'success',
        agentId,
        responses: responses.map((resp: any) => ({
          responseId: resp.response_id,
          messageId: resp.message_id,
          originalMessage: resp.original_content,
          response: resp.response_content,
          timestamp: resp.created_at,
          fromDomain: resp.from_domain
        })),
        source: 'local'
      };
    } else {
      // Remote check via MCP client
      try {
        const remoteResult = await this.mcpClient.checkAgentResponses(targetDomain, agentId);
        
        return {
          status: 'success',
          agentId,
          responses: remoteResult.responses,
          source: 'remote',
          targetDomain,
          error: remoteResult.error
        };
      } catch (error) {
        return {
          status: 'error',
          agentId,
          responses: [],
          source: 'remote',
          targetDomain,
          error: error instanceof Error ? error.message : String(error)
        };
      }
    }
  }

  /**
   * Delete friend requests by criteria
   */
  async deleteFriendRequests(criteria: any, clientIP?: string): Promise<any> {
    return await this.friendshipService.deleteFriendRequests(criteria, clientIP);
  }

  /**
   * Delete gossip messages by criteria
   */
  async deleteMessages(criteria: any): Promise<any> {
    return await this.gossipService.deleteMessages(criteria);
  }

  /**
   * List active friends
   */
  async listFriends(clientIP?: string): Promise<any> {
    return await this.friendshipService.listFriends(clientIP);
  }

  /**
   * Send message to another domain/bot
   */
  async sendMessage(toDomain: string, content: string, messageType: string = 'chat', clientIP?: string): Promise<any> {
    return await this.messagingService.sendMessage(toDomain, content, messageType, clientIP);
  }

  /**
   * Review messages (different behavior for local vs federated)
   */
  async reviewMessages(domain?: string, includeResponses: boolean = true, clientIP?: string): Promise<any> {
    return await this.messagingService.reviewMessages(domain, includeResponses, clientIP);
  }

  /**
   * Set response to a received message
   */
  async setResponse(messageId: string, responseContent: string, clientIP?: string): Promise<any> {
    return await this.messagingService.setResponse(messageId, responseContent, clientIP);
  }

  /**
   * Delete messages by criteria (messaging service)
   */
  async deleteMessagingMessages(criteria: any, clientIP?: string): Promise<any> {
    return await this.messagingService.deleteMessages(criteria, clientIP);
  }

  /**
   * Upgrade a local friend to federated status
   */
  async upgradeFriend(localName: string, newDomain: string, clientIP?: string): Promise<any> {
    return await this.friendshipService.upgradeFriend(localName, newDomain, clientIP);
  }

  /**
   * Share gossip with known friends
   */
  async shareGossip(content: string, category: string = 'general', tags: string[] = [], clientIP?: string): Promise<any> {
    return await this.gossipService.shareGossip(content, category, tags, clientIP);
  }

  /**
   * Review gossips and get combined gossip text
   */
  async reviewGossips(limit: number = 20, category?: string, clientIP?: string): Promise<any> {
    return await this.gossipService.reviewGossips(limit, category, clientIP);
  }

  /**
   * Get friendship service (for HTTP server access)
   */
  getFriendshipService() {
    return this.friendshipService;
  }

  /**
   * Get messaging service (for HTTP server access) 
   */
  getMessagingService() {
    return this.messagingService;
  }

  /**
   * Get gossip service (for HTTP server access)
   */
  getGossipService() {
    return this.gossipService;
  }

  /**
   * Check rate limit for federation requests
   */
  checkRateLimit(identifier: string, operation: string = 'federation'): boolean {
    return this.rateLimiter.checkRateLimit(identifier, operation);
  }

  /**
   * MCP Authentication: Login and get Bearer token
   */
  async login(fromDomain: string, challenge?: string): Promise<{ bearerToken: string; expiresAt: Date }> {
    this.options.logger.info('🔑 MCP Login attempt', { fromDomain, hasChallenge: !!challenge });
    
    try {
      // Basic domain validation
      if (!fromDomain || fromDomain.length < 3) {
        throw new Error('Invalid domain format');
      }
      
      // For federated domains (botnet.*), we could implement challenge verification
      // For now, we'll allow any domain to login (rate limited)
      const clientKey = fromDomain;
      if (!this.rateLimiter.checkRateLimit(clientKey, 'login')) {
        throw new Error('Login rate limit exceeded');
      }
      
      // Generate Bearer token (secure random string)
      const tokenBytes = new Uint8Array(32);
      crypto.getRandomValues(tokenBytes);
      const bearerToken = 'mcp_' + Array.from(tokenBytes).map(b => b.toString(16).padStart(2, '0')).join('');
      
      // Token expires in 1 hour
      const expiresAt = new Date(Date.now() + 60 * 60 * 1000);
      
      // Store token
      this.bearerTokens.set(bearerToken, {
        domain: fromDomain,
        expiresAt,
        createdAt: new Date()
      });
      
      // Cleanup expired tokens (garbage collection)
      this.cleanupExpiredTokens();
      
      this.options.logger.info('✅ MCP Login successful', { 
        fromDomain, 
        tokenPrefix: bearerToken.substring(0, 12) + '...',
        expiresAt 
      });
      
      return { bearerToken, expiresAt };
      
    } catch (error) {
      this.options.logger.error('❌ MCP Login failed', { 
        fromDomain, 
        error: error instanceof Error ? error.message : String(error) 
      });
      throw error;
    }
  }

  /**
   * MCP Authentication: Validate Bearer token
   */
  async validateBearerToken(bearerToken: string): Promise<{ valid: boolean; domain?: string; error?: string }> {
    try {
      if (!bearerToken || !bearerToken.startsWith('mcp_')) {
        return { valid: false, error: 'Invalid token format' };
      }
      
      const tokenData = this.bearerTokens.get(bearerToken);
      if (!tokenData) {
        return { valid: false, error: 'Token not found' };
      }
      
      // Check expiration
      if (new Date() > tokenData.expiresAt) {
        this.bearerTokens.delete(bearerToken);
        return { valid: false, error: 'Token expired' };
      }
      
      this.options.logger.info('🔐 Token validation successful', { 
        domain: tokenData.domain,
        tokenPrefix: bearerToken.substring(0, 12) + '...'
      });
      
      return { valid: true, domain: tokenData.domain };
      
    } catch (error) {
      this.options.logger.error('🔐 Token validation error', { 
        error: error instanceof Error ? error.message : String(error) 
      });
      return { valid: false, error: 'Validation error' };
    }
  }

  /**
   * Cleanup expired Bearer tokens
   */
  private cleanupExpiredTokens(): void {
    const now = new Date();
    let cleaned = 0;
    
    for (const [token, data] of this.bearerTokens.entries()) {
      if (now > data.expiresAt) {
        this.bearerTokens.delete(token);
        cleaned++;
      }
    }
    
    if (cleaned > 0) {
      this.options.logger.info('🧹 Cleaned up expired tokens', { cleaned, remaining: this.bearerTokens.size });
    }
  }

  async shutdown() {
    this.options.logger.info("Shutting down BotNet service");
    // Cleanup bearer tokens
    this.bearerTokens.clear();
  }
}
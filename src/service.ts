import { v4 as uuidv4 } from "uuid";
import type Database from "better-sqlite3";
import type { BotNetConfig } from "../index.js";
import type { Logger } from "./logger.js";
import { AuthService } from "./auth/auth-service.js";
import { FriendshipService } from "./friendship/friendship-service.js";
import { GossipService } from "./gossip/gossip-service.js";
interface BotNetServiceOptions {
  database: Database.Database;
  config: BotNetConfig;
  logger: Logger;
}

export class BotNetService {
  private authService: AuthService;
  private friendshipService: FriendshipService;
  private gossipService: GossipService;
  
  constructor(private options: BotNetServiceOptions) {
    const { database, config, logger } = options;
    
    this.authService = new AuthService(logger.child("auth"));
    this.friendshipService = new FriendshipService(database, config, logger.child("friendship"));
    this.gossipService = new GossipService(database, config, logger.child("gossip"));
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
      
      // TODO: Make HTTP request to remote domain to notify them
      // For now, we'll just store it locally
      this.options.logger.info("🦞 Friend request created locally", {
        friendHost,
        fromDomain,
        requestId: request.id
      });
      
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
      
      // TODO: Notify the remote domain that we accepted
      this.options.logger.info("🦞 Friend request accepted", {
        friendHost,
        domainName,
        friendshipId: result.friendshipId
      });
      
      return result;
    } catch (error) {
      this.options.logger.error("🦞 Failed to accept friend request", { friendHost, domainName, error });
      throw error;
    }
  }

  /**
   * Add friend by request ID - handles local acceptance and automatic federated challenges
   */
  async addFriend(requestId: string): Promise<{ status: string; friendshipId?: string; challengeId?: string; message?: string }> {
    try {
      return await this.friendshipService.addFriendByRequestId(requestId);
    } catch (error) {
      this.options.logger.error("🦞 Failed to add friend", { requestId, error });
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
   * Delete friend requests by criteria
   */
  async deleteFriendRequests(criteria: any): Promise<any> {
    return await this.friendshipService.deleteFriendRequests(criteria);
  }

  /**
   * Delete gossip messages by criteria
   */
  async deleteMessages(criteria: any): Promise<any> {
    return await this.gossipService.deleteMessages(criteria);
  }

  async shutdown() {
    this.options.logger.info("Shutting down BotNet service");
    // Cleanup tasks if needed
  }
}
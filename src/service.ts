import { v4 as uuidv4 } from "uuid";
import type Database from "better-sqlite3";
import type { BotNetConfig } from "../index.js";
import type { Logger } from "./logger.js";
import { AuthService } from "./auth/auth-service.js";
import { FriendshipService } from "./friendship/friendship-service.js";
import { GossipService } from "./gossip/gossip-service.js";
import type { OpenClawRuntime } from "openclaw/plugin-sdk";

interface BotNetServiceOptions {
  database: Database.Database;
  config: BotNetConfig;
  logger: Logger;
  runtime: OpenClawRuntime;
}

export class BotNetService {
  private authService: AuthService;
  private friendshipService: FriendshipService;
  private gossipService: GossipService;
  
  constructor(private options: BotNetServiceOptions) {
    const { database, config, logger } = options;
    
    this.authService = new AuthService(database, logger.child("auth"));
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
        return await this.friendshipService.handleFriendshipRequest(request);
      
      case "friendship.accept":
        return await this.friendshipService.acceptFriendship(request);
      
      case "friendship.reject":
        return await this.friendshipService.rejectFriendship(request);
      
      case "gossip.exchange":
        return await this.gossipService.handleExchange(request);
      
      case "gossip.query":
        return await this.gossipService.handleQuery(request);
      
      case "auth.challenge":
        return await this.authService.handleChallenge(request);
      
      case "auth.response":
        return await this.authService.handleResponse(request);
      
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
    return this.friendshipService.createFriendshipRequest(request);
  }
  
  async getFriendshipStatus(friendId: string) {
    return this.friendshipService.getFriendshipStatus(friendId);
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
  
  async shutdown() {
    this.options.logger.info("Shutting down BotNet service");
    // Cleanup tasks if needed
  }
}
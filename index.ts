import type { OpenClawPluginApi } from "openclaw/plugin-sdk";
import http from "http";
import { z } from "zod";
import { Type } from "@sinclair/typebox";
import { createBotNetServer } from "./src/http-server.js";
import { initializeDatabase } from "./src/database.js";
import { BotNetService } from "./src/service.js";
import { TokenService } from "./src/auth/token-service.js";

// Configuration schema
const BotNetConfigSchema = z.object({
  botName: z.string().default("Khaar"),
  botDomain: z.string().default("botnet.airon.games"),
  botDescription: z.string().default("A Dragon BotNet node"),
  capabilities: z.array(z.string()).default(["conversation", "collaboration", "federation"]),
  tier: z.enum(["bootstrap", "standard", "pro", "enterprise"]).default("standard"),
  databasePath: z.string().default("./data/botnet.db"),
  httpPort: z.number().default(8080),
  logLevel: z.enum(["debug", "info", "warn", "error"]).default("info"),
  tokenCleanupIntervalMinutes: z.number().default(30), // Token cleanup frequency
});

export type BotNetConfig = z.infer<typeof BotNetConfigSchema>;

const plugin = {
  id: "botnet",
  name: "BotNet",
  description: "Decentralized bot network protocol with three-tier authentication for secure multi-agent collaboration",
  configSchema: BotNetConfigSchema,
  
  register(api: OpenClawPluginApi) {
    console.log("🐉 BotNet plugin loading with THREE-TIER AUTHENTICATION...");
    
    let httpServer: http.Server | null = null;
    let database: any = null;
    let botnetService: BotNetService | null = null;
    let tokenService: TokenService | null = null;
    let cleanupInterval: NodeJS.Timeout | null = null;
    
    const config = BotNetConfigSchema.parse(api.pluginConfig || {});
    
    // Register background service for in-process HTTP server
    api.registerService({
      id: "botnet-server", 
      start: async () => {
        console.log("🐉 Starting BotNet service with three-tier authentication...");
        
        try {
          // Initialize database  
          database = await initializeDatabase(config.databasePath, api.logger as any);
          console.log("✅ Database initialized with authentication tables");
          
          // Create logger adapter with proper child method
          const loggerAdapter = {
            baseLogger: api.logger,
            debug: (msg: string, meta?: any) => api.logger.debug ? api.logger.debug(meta ? `${msg} ${JSON.stringify(meta)}` : msg) : console.log(`[DEBUG] ${msg}`, meta),
            info: (msg: string, meta?: any) => api.logger.info ? api.logger.info(meta ? `${msg} ${JSON.stringify(meta)}` : msg) : console.log(`[INFO] ${msg}`, meta),
            warn: (msg: string, meta?: any) => api.logger.warn ? api.logger.warn(meta ? `${msg} ${JSON.stringify(meta)}` : msg) : console.log(`[WARN] ${msg}`, meta), 
            error: (msg: string, meta?: any) => api.logger.error ? api.logger.error(meta ? `${msg} ${JSON.stringify(meta)}` : msg) : console.log(`[ERROR] ${msg}`, meta),
            child: (prefix: string) => ({
              baseLogger: api.logger,
              debug: (msg: string, meta?: any) => api.logger.debug ? api.logger.debug(meta ? `[${prefix}] ${msg} ${JSON.stringify(meta)}` : `[${prefix}] ${msg}`) : console.log(`[DEBUG] [${prefix}] ${msg}`, meta),
              info: (msg: string, meta?: any) => api.logger.info ? api.logger.info(meta ? `[${prefix}] ${msg} ${JSON.stringify(meta)}` : `[${prefix}] ${msg}`) : console.log(`[INFO] [${prefix}] ${msg}`, meta),
              warn: (msg: string, meta?: any) => api.logger.warn ? api.logger.warn(meta ? `[${prefix}] ${msg} ${JSON.stringify(meta)}` : `[${prefix}] ${msg}`) : console.log(`[WARN] [${prefix}] ${msg}`, meta),
              error: (msg: string, meta?: any) => api.logger.error ? api.logger.error(meta ? `[${prefix}] ${msg} ${JSON.stringify(meta)}` : `[${prefix}] ${msg}`) : console.log(`[ERROR] [${prefix}] ${msg}`, meta),
              child: (subPrefix: string) => loggerAdapter.child(`${prefix}:${subPrefix}`)
            })
          } as any;
          
          // 🔐 Initialize TokenService (THREE-TIER AUTHENTICATION)
          tokenService = new TokenService(database, loggerAdapter.child('TokenService'));
          console.log("✅ TokenService initialized for three-tier authentication");
          
          // Create BotNet service with database
          botnetService = new BotNetService({
            database,
            config,
            logger: loggerAdapter
          });
          console.log("✅ BotNetService initialized");
          
          // Start token cleanup job
          cleanupInterval = setInterval(async () => {
            try {
              const stats = await tokenService!.cleanupExpiredTokens();
              if (stats.negotiationCleaned > 0 || stats.sessionCleaned > 0) {
                loggerAdapter.info("Token cleanup completed", stats);
              }
            } catch (error) {
              loggerAdapter.error("Token cleanup failed", { error });
            }
          }, config.tokenCleanupIntervalMinutes * 60 * 1000);
          console.log(`✅ Token cleanup scheduled every ${config.tokenCleanupIntervalMinutes} minutes`);

          // 🔐 SECURE: Register Internal Plugin API via Tools
          // These methods are only accessible to OpenClaw internally as tools, not via HTTP
          
          // Helper function to format tool results
          const formatToolResult = (content: string, details?: unknown) => ({
            content: [{ type: "text" as const, text: content }],
            details: details || {}
          });
          
          // 👥 ENHANCED Friendship Management Tools (Updated for Three-Tier Auth)
          api.registerTool({
            name: "botnet_list_friends",
            label: "BotNet List Friends", 
            description: "List all active friendships in the BotNet",
            parameters: Type.Object({}),
            execute: async (toolCallId: string, params: {}, signal?: AbortSignal) => {
              try {
                const friends = await botnetService!.listFriends();
                const tokenStats = await tokenService!.getTokenStatistics();
                return formatToolResult(
                  `Found ${friends.length} active friendships. Authentication stats: ${tokenStats.activeFriendshipCredentials} permanent credentials, ${tokenStats.activeSessionTokens} active sessions.`,
                  { friends, tokenStats }
                );
              } catch (error) {
                const errorMsg = error instanceof Error ? error.message : String(error);
                return formatToolResult(
                  `Error listing friends: ${errorMsg}`,
                  { error: errorMsg }
                );
              }
            }
          });

          api.registerTool({
            name: "botnet_review_friends",
            label: "BotNet Review Friends",
            description: "Review pending friend requests (categorized local vs federated)",
            parameters: Type.Object({}),
            execute: async (toolCallId: string, params: {}, signal?: AbortSignal) => {
              try {
                const requests = await botnetService!.getEnhancedPendingRequests();
                const tokenStats = await tokenService!.getTokenStatistics();
                return formatToolResult(
                  `Pending requests: ${requests.summary.total} total (${requests.summary.localCount} local, ${requests.summary.federatedCount} federated). ${tokenStats.activeNegotiationTokens} active negotiation tokens.`,
                  { requests, tokenStats }
                );
              } catch (error) {
                const errorMsg = error instanceof Error ? error.message : String(error);
                return formatToolResult(
                  `Error reviewing friends: ${errorMsg}`,
                  { error: errorMsg }
                );
              }
            }
          });

          // 🔐 NEW: Authentication Management Tools
          api.registerTool({
            name: "botnet_auth_status",
            label: "BotNet Auth Status",
            description: "Get authentication system status and token statistics",
            parameters: Type.Object({}),
            execute: async (toolCallId: string, params: {}, signal?: AbortSignal) => {
              try {
                const stats = await tokenService!.getTokenStatistics();
                const summary = `Authentication Status:
• Negotiation tokens (friendship establishment): ${stats.activeNegotiationTokens}
• Friendship credentials (permanent passwords): ${stats.activeFriendshipCredentials}
• Session tokens (active communication): ${stats.activeSessionTokens}`;
                
                return formatToolResult(summary, { 
                  stats,
                  authTiers: ['public', 'negotiation', 'session'],
                  cleanup: `Next cleanup in ${config.tokenCleanupIntervalMinutes} minutes`
                });
              } catch (error) {
                const errorMsg = error instanceof Error ? error.message : String(error);
                return formatToolResult(
                  `Error getting auth status: ${errorMsg}`,
                  { error: errorMsg }
                );
              }
            }
          });

          api.registerTool({
            name: "botnet_cleanup_tokens",
            label: "BotNet Cleanup Tokens",
            description: "Manually trigger cleanup of expired authentication tokens",
            parameters: Type.Object({}),
            execute: async (toolCallId: string, params: {}, signal?: AbortSignal) => {
              try {
                const stats = await tokenService!.cleanupExpiredTokens();
                return formatToolResult(
                  `Token cleanup completed: ${stats.negotiationCleaned} negotiation tokens, ${stats.sessionCleaned} session tokens removed.`,
                  stats
                );
              } catch (error) {
                const errorMsg = error instanceof Error ? error.message : String(error);
                return formatToolResult(
                  `Error during token cleanup: ${errorMsg}`,
                  { error: errorMsg }
                );
              }
            }
          });

          // 🤝 UPDATED Friendship Request Tool (Three-Tier Auth)
          api.registerTool({
            name: "botnet_send_friend_request",
            label: "BotNet Send Friend Request",
            description: "Send a friend request to another bot",
            parameters: Type.Object({
              friendDomain: Type.String({ description: "Friend's domain (e.g., 'bot.example.com')" }),
              message: Type.Optional(Type.String({ description: "Optional message with the request" }))
            }),
            execute: async (toolCallId: string, params: { friendDomain: string; message?: string }, signal?: AbortSignal) => {
              try {
                // Send friendship request - this will initiate the three-tier auth flow
                const result = await botnetService!.sendFriendRequest(params.friendDomain, config.botDomain);
                
                return formatToolResult(
                  `Friend request sent to ${params.friendDomain}. Three-tier auth initiated.`,
                  {
                    ...result,
                    nextSteps: [
                      "1. Wait for friendship acceptance",
                      "2. Use negotiation token to check status",
                      "3. Receive permanent password when accepted",
                      "4. Use permanent password to login and get session tokens"
                    ]
                  }
                );
              } catch (error) {
                const errorMsg = error instanceof Error ? error.message : String(error);
                return formatToolResult(
                  `Error sending friend request to ${params.friendDomain}: ${errorMsg}`,
                  { error: errorMsg, friendDomain: params.friendDomain }
                );
              }
            }
          });

          // 🤝 Accept Friend Request Tool (FIXED - No longer placeholder!)
          api.registerTool({
            name: "botnet_accept_friend_request",
            label: "BotNet Accept Friend Request",
            description: "Accept a pending friend request by request ID",
            parameters: Type.Object({
              requestId: Type.String({ description: "Friend request ID to accept" }),
              challengeResponse: Type.Optional(Type.String({ description: "Challenge response for federated domains" }))
            }),
            execute: async (toolCallId: string, params: { requestId: string; challengeResponse?: string }, signal?: AbortSignal) => {
              try {
                const result = await botnetService!.acceptFriend(params.requestId, params.challengeResponse);
                
                return formatToolResult(
                  `Friend request ${params.requestId} accepted successfully!`,
                  {
                    status: result.status,
                    friendshipId: result.friendshipId,
                    message: result.message,
                    nextSteps: result.challengeId ? [
                      `Challenge initiated with ID: ${result.challengeId}`,
                      "Domain verification in progress"
                    ] : [
                      "Friendship is now active",
                      "Cross-domain communication enabled"
                    ]
                  }
                );
              } catch (error) {
                const errorMsg = error instanceof Error ? error.message : String(error);
                return formatToolResult(
                  `Error accepting friend request ${params.requestId}: ${errorMsg}`,
                  { error: errorMsg, requestId: params.requestId }
                );
              }
            }
          });

          // 💬 Enhanced Messaging Tools (Session Token Required)
          api.registerTool({
            name: "botnet_send_message",
            label: "BotNet Send Message",
            description: "Send a message to another bot in the network",
            parameters: Type.Object({
              targetBot: Type.String({ description: "Target bot name or domain" }),
              message: Type.String({ description: "Message content" }),
              category: Type.Optional(Type.String({ description: "Message category (default: 'general')" })),
              anonymous: Type.Optional(Type.Boolean({ description: "Send anonymously (default: false)" }))
            }),
            execute: async (toolCallId: string, params: { targetBot: string; message: string; category?: string; anonymous?: boolean }, signal?: AbortSignal) => {
              try {
                // This will use session tokens internally
                const result = await botnetService!.sendMessage(
                  params.targetBot,
                  params.message,
                  params.category || 'general'
                );
                
                return formatToolResult(
                  `Message sent to ${params.targetBot} using authenticated session.`,
                  result
                );
              } catch (error) {
                const errorMsg = error instanceof Error ? error.message : String(error);
                return formatToolResult(
                  `Error sending message to ${params.targetBot}: ${errorMsg}`,
                  { error: errorMsg, targetBot: params.targetBot }
                );
              }
            }
          });

          // 📡 Updated Gossip Tools
          api.registerTool({
            name: "botnet_review_gossips",
            label: "BotNet Review Gossips",
            description: "Review gossips and get combined readable text with trust scoring",
            parameters: Type.Object({
              limit: Type.Optional(Type.Number({ description: "Number of gossips to review (default: 20)" })),
              category: Type.Optional(Type.String({ description: "Filter by category (optional)" }))
            }),
            execute: async (toolCallId: string, params: { limit?: number; category?: string }, signal?: AbortSignal) => {
              try {
                const result = await botnetService!.reviewGossips(params?.limit || 20, params?.category);
                return formatToolResult(
                  `Reviewed ${result.summary.total} gossips from ${result.summary.sources.length} sources. Combined text:\n\n${result.combinedText}`,
                  result
                );
              } catch (error) {
                const errorMsg = error instanceof Error ? error.message : String(error);
                return formatToolResult(
                  `Error reviewing gossips: ${errorMsg}`,
                  { error: errorMsg }
                );
              }
            }
          });

          api.registerTool({
            name: "botnet_share_gossip",
            label: "BotNet Share Gossip",
            description: "Share gossip with friends - category and tags support",
            parameters: Type.Object({
              content: Type.String({ description: "Gossip content to share" }),
              category: Type.Optional(Type.String({ description: "Gossip category (default: 'general')" })),
              tags: Type.Optional(Type.Array(Type.String(), { description: "Tags for the gossip" }))
            }),
            execute: async (toolCallId: string, params: { content: string; category?: string; tags?: string[] }, signal?: AbortSignal) => {
              try {
                const result = await botnetService!.shareGossip(
                  params.content,
                  params.category || 'general',
                  params.tags || []
                );
                
                return formatToolResult(
                  `Gossip shared with authenticated friends: "${params.content.substring(0, 100)}${params.content.length > 100 ? '...' : ''}"`,
                  result
                );
              } catch (error) {
                const errorMsg = error instanceof Error ? error.message : String(error);
                return formatToolResult(
                  `Error sharing gossip: ${errorMsg}`,
                  { error: errorMsg }
                );
              }
            }
          });

          // 🗑️ Deletion and Cleanup Tools  
          api.registerTool({
            name: "botnet_delete_friend_requests",
            label: "BotNet Delete Friend Requests",
            description: "Delete friend requests with flexible criteria",
            parameters: Type.Object({
              requestId: Type.Optional(Type.String({ description: "Specific request ID" })),
              fromDomain: Type.Optional(Type.String({ description: "Delete all from this domain" })),
              status: Type.Optional(Type.String({ description: "Delete by status (pending, accepted, rejected)" })),
              olderThanDays: Type.Optional(Type.Number({ description: "Delete requests older than N days" }))
            }),
            execute: async (toolCallId: string, params: any, signal?: AbortSignal) => {
              try {
                const result = await botnetService!.deleteFriendRequests(params);
                return formatToolResult(
                  `Deleted ${result.deletedCount} friend requests matching criteria.`,
                  result
                );
              } catch (error) {
                const errorMsg = error instanceof Error ? error.message : String(error);
                return formatToolResult(
                  `Error deleting friend requests: ${errorMsg}`,
                  { error: errorMsg }
                );
              }
            }
          });

          // Create HTTP server with TokenService
          httpServer = createBotNetServer({
            config,
            logger: loggerAdapter,
            botnetService,
            tokenService
          });
          
          // Start HTTP server
          httpServer.listen(config.httpPort, () => {
            loggerAdapter.info(`🐉 BotNet HTTP server started on port ${config.httpPort}`);
            loggerAdapter.info(`🔐 Three-tier authentication system active`);
            loggerAdapter.info(`🌐 Public API: http://localhost:${config.httpPort}/`);
            loggerAdapter.info(`🤖 MCP Endpoint: http://localhost:${config.httpPort}/mcp`);
          });
          
        } catch (error) {
          api.logger.error(`Failed to start BotNet service: ${error instanceof Error ? error.message : error}`);
          throw error;
        }
      },
      
      stop: async () => {
        console.log("🐉 Stopping BotNet service...");
        
        // Stop token cleanup
        if (cleanupInterval) {
          clearInterval(cleanupInterval);
          cleanupInterval = null;
        }
        
        // Close HTTP server
        if (httpServer) {
          httpServer.close();
          httpServer = null;
        }
        
        // Close database
        if (database) {
          database.close();
          database = null;
        }
        
        botnetService = null;
        tokenService = null;
        
        console.log("✅ BotNet service stopped");
      }
    });
    
    console.log("🐉 BotNet plugin registered with three-tier authentication");
  }
};

export default plugin;
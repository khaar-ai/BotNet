import type { OpenClawPluginApi } from "openclaw/plugin-sdk";
import http from "http";
import { z } from "zod";
import { Type } from "@sinclair/typebox";
import { createBotNetServer } from "./src/http-server.js";
import { initializeDatabase } from "./src/database.js";
import { BotNetService } from "./src/service.js";

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
});

export type BotNetConfig = z.infer<typeof BotNetConfigSchema>;

const plugin = {
  id: "botnet",
  name: "BotNet",
  description: "Decentralized bot network protocol for secure multi-agent collaboration",
  configSchema: BotNetConfigSchema,
  
  register(api: OpenClawPluginApi) {
    console.log("🦞 BotNet plugin loading with social networking...");
    
    let httpServer: http.Server | null = null;
    let database: any = null;
    let botnetService: BotNetService | null = null;
    const config = BotNetConfigSchema.parse(api.pluginConfig || {});
    
    // Register background service for in-process HTTP server
    api.registerService({
      id: "botnet-server", 
      start: async () => {
        console.log("🦞 Starting BotNet service with database...");
        
        try {
          // Initialize database  
          database = await initializeDatabase(config.databasePath, api.logger as any);
          
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
          
          // Create BotNet service with database
          botnetService = new BotNetService({
            database,
            config,
            logger: loggerAdapter
          });

          // 🔐 SECURE: Register Internal Plugin API via Tools
          // These methods are only accessible to OpenClaw internally as tools, not via HTTP
          
          // Helper function to format tool results
          const formatToolResult = (content: string, details?: unknown) => ({
            content: [{ type: "text" as const, text: content }],
            details: details || {}
          });
          
          // 👥 Friendship Management Tools
          api.registerTool({
            name: "botnet_list_friends",
            label: "BotNet List Friends", 
            description: "List all active friendships in the BotNet",
            parameters: Type.Object({}),
            execute: async (toolCallId: string, params: {}, signal?: AbortSignal) => {
              try {
                const result = await botnetService!.listFriends();
                return formatToolResult(
                  `Found ${result.length} active friendships`,
                  result
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
                const result = await botnetService!.getEnhancedPendingRequests();
                return formatToolResult(
                  `Pending requests: ${result.summary.total} total (${result.summary.localCount} local, ${result.summary.federatedCount} federated)`,
                  result
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
            name: "botnet_get_health", 
            label: "BotNet Get Health",
            description: "Get BotNet node health status and diagnostics",
            parameters: Type.Object({}),
            execute: async (toolCallId: string, params: {}, signal?: AbortSignal) => {
              try {
                const result = await botnetService!.getHealthStatus();
                return formatToolResult(
                  `BotNet node status: ${result.status}`,
                  result
                );
              } catch (error) {
                const errorMsg = error instanceof Error ? error.message : String(error);
                return formatToolResult(
                  `Error checking health: ${errorMsg}`,
                  { error: errorMsg }
                );
              }
            }
          });

          // 💬 Messaging & Communication Tools
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
                const result = await botnetService!.sendMessage(
                  params.targetBot,
                  params.message,
                  params.category || "general"
                );
                return formatToolResult(
                  `Message sent to ${params.targetBot}: ${result.success ? 'Success' : 'Failed'}`,
                  result
                );
              } catch (error) {
                const errorMsg = error instanceof Error ? error.message : String(error);
                return formatToolResult(
                  `Error sending message: ${errorMsg}`,
                  { error: errorMsg }
                );
              }
            }
          });

          api.registerTool({
            name: "botnet_review_messages",
            label: "BotNet Review Messages",
            description: "Review incoming messages (local vs federated)",
            parameters: Type.Object({
              limit: Type.Optional(Type.Number({ description: "Number of messages to review (default: 10)" })),
              category: Type.Optional(Type.String({ description: "Filter by category" })),
              sourceBot: Type.Optional(Type.String({ description: "Filter by source bot" }))
            }),
            execute: async (toolCallId: string, params: { limit?: number; category?: string; sourceBot?: string }, signal?: AbortSignal) => {
              try {
                const result = await botnetService!.reviewMessages(
                  params.sourceBot,
                  true
                );
                return formatToolResult(
                  `Reviewed ${result.messages.length} messages from ${result.summary.sources} sources`,
                  result
                );
              } catch (error) {
                const errorMsg = error instanceof Error ? error.message : String(error);
                return formatToolResult(
                  `Error reviewing messages: ${errorMsg}`,
                  { error: errorMsg }
                );
              }
            }
          });

          // 🔗 Friend Request Management Tools
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
                const result = await botnetService!.sendFriendRequest(
                  params.friendDomain,
                  config.botDomain
                );
                return formatToolResult(
                  `Friend request sent to ${params.friendDomain}: Success (Request ID: ${result.requestId})`,
                  result
                );
              } catch (error) {
                const errorMsg = error instanceof Error ? error.message : String(error);
                return formatToolResult(
                  `Error sending friend request: ${errorMsg}`,
                  { error: errorMsg }
                );
              }
            }
          });

          api.registerTool({
            name: "botnet_respond_friend_request",
            label: "BotNet Respond to Friend Request", 
            description: "Accept or reject a pending friend request",
            parameters: Type.Object({
              requestId: Type.String({ description: "Friend request ID" }),
              accept: Type.Boolean({ description: "True to accept, false to reject" }),
              message: Type.Optional(Type.String({ description: "Optional response message" }))
            }),
            execute: async (toolCallId: string, params: { requestId: string; accept: boolean; message?: string }, signal?: AbortSignal) => {
              try {
                const result = await botnetService!.acceptFriend(
                  params.requestId,
                  params.message
                );
                const action = params.accept ? 'accepted' : 'rejected';
                return formatToolResult(
                  `Friend request ${action}: ${result.status}`,
                  result
                );
              } catch (error) {
                const errorMsg = error instanceof Error ? error.message : String(error);
                return formatToolResult(
                  `Error responding to friend request: ${errorMsg}`,
                  { error: errorMsg }
                );
              }
            }
          });

          // 🗑️ Data Management Tools
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
            execute: async (toolCallId: string, params: { requestId?: string; fromDomain?: string; status?: string; olderThanDays?: number }, signal?: AbortSignal) => {
              try {
                const result = await botnetService!.deleteFriendRequests(params);
                return formatToolResult(
                  `Deleted ${result.deletedCount} friend requests`,
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

          api.registerTool({
            name: "botnet_delete_messages",
            label: "BotNet Delete Messages",
            description: "Delete messages with flexible criteria",
            parameters: Type.Object({
              messageId: Type.Optional(Type.String({ description: "Specific message ID" })),
              sourceBot: Type.Optional(Type.String({ description: "Delete all from this bot" })),
              category: Type.Optional(Type.String({ description: "Delete by category" })),
              olderThanDays: Type.Optional(Type.Number({ description: "Delete messages older than N days" })),
              includeAnonymous: Type.Optional(Type.Boolean({ description: "Include anonymous messages (default: false)" }))
            }),
            execute: async (toolCallId: string, params: { messageId?: string; sourceBot?: string; category?: string; olderThanDays?: number; includeAnonymous?: boolean }, signal?: AbortSignal) => {
              try {
                const result = await botnetService!.deleteMessages(params);
                return formatToolResult(
                  `Deleted ${result.deletedCount} messages`,
                  result
                );
              } catch (error) {
                const errorMsg = error instanceof Error ? error.message : String(error);
                return formatToolResult(
                  `Error deleting messages: ${errorMsg}`,
                  { error: errorMsg }
                );
              }
            }
          });

          console.log("🔐 BotNet Internal API registered as secure tools (10 comprehensive methods available)");
          
          // Create HTTP server with BotNet service
          httpServer = createBotNetServer({
            config,
            logger: loggerAdapter,
            botnetService
          });
          
          // Start the server and wait for it to be ready
          await new Promise<void>((resolve, reject) => {
            httpServer!.on('error', reject);
            httpServer!.listen(config.httpPort, () => {
              console.log(`🦞 BotNet server started on port ${config.httpPort}`);
              api.logger.info(`🦞 BotNet server started with database: ${config.databasePath}`);
              resolve();
            });
          });
          
        } catch (error) {
          const errorMsg = error instanceof Error ? error.message : String(error);
          console.error("🦞 Failed to start BotNet server:", errorMsg);
          api.logger.error(`🦞 Failed to start BotNet server: ${errorMsg}`);
          throw error;
        }
      },
      
      stop: async () => {
        console.log("🦞 Stopping BotNet service...");
        
        if (httpServer) {
          try {
            // Graceful shutdown of HTTP server
            await new Promise<void>((resolve) => {
              httpServer!.close(() => {
                console.log("🦞 BotNet HTTP server stopped");
                api.logger.info("🦞 BotNet HTTP server stopped gracefully");
                resolve();
              });
            });
          } catch (error) {
            const errorMsg = error instanceof Error ? error.message : String(error);
            api.logger.error(`🦞 Error stopping BotNet server: ${errorMsg}`);
          }
          
          httpServer = null;
        }
        
        // Close database connection
        if (database) {
          try {
            database.close();
            api.logger.info("🦞 Database connection closed");
          } catch (error) {
            const errorMsg = error instanceof Error ? error.message : String(error);
            api.logger.error(`🦞 Error closing database: ${errorMsg}`);
          }
          database = null;
        }
      }
    });
    
    console.log("🦞 BotNet plugin loaded successfully");
    
    return {
      shutdown: async () => {
        console.log("🦞 BotNet plugin shutting down");
        // Service stop will be called automatically by OpenClaw
      }
    };
  },
};

export default plugin;
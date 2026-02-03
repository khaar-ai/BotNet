import type { OpenClawPluginApi } from "openclaw/plugin-sdk";
import http from "http";
import { z } from "zod";
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
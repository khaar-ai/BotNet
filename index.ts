import type { OpenClawPluginApi } from "openclaw/plugin-sdk";
import http from "http";
import { z } from "zod";
import { createBotNetServer } from "./src/http-server.js";

// Configuration schema
const BotNetConfigSchema = z.object({
  botName: z.string().default("Khaar"),
  botDomain: z.string().default("khaar.airon.games"),
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
    console.log("🐉 BotNet plugin loading with MCP protocol...");
    
    let httpServer: http.Server | null = null;
    const config = BotNetConfigSchema.parse(api.pluginConfig || {});
    
    // Register background service for in-process HTTP server
    api.registerService({
      id: "botnet-server",
      start: async () => {
        console.log("🐉 Starting BotNet HTTP server service (in-process)...");
        
        try {
          // Create HTTP server in-process using our server factory (MCP enabled)
          httpServer = createBotNetServer({
            config,
            logger: api.logger
          });
          
          // Start the server and wait for it to be ready
          await new Promise<void>((resolve, reject) => {
            httpServer!.on('error', reject);
            httpServer!.listen(config.httpPort, () => {
              console.log(`🐉 BotNet HTTP server started on port ${config.httpPort}`);
              api.logger.info(`🐉 BotNet HTTP server started on port ${config.httpPort} (in-process)`);
              resolve();
            });
          });
          
        } catch (error) {
          const errorMsg = error instanceof Error ? error.message : String(error);
          console.error("🐉 Failed to start BotNet server:", errorMsg);
          api.logger.error(`🐉 Failed to start BotNet server: ${errorMsg}`);
          throw error;
        }
      },
      
      stop: async () => {
        console.log("🐉 Stopping BotNet HTTP server service...");
        
        if (httpServer) {
          try {
            // Graceful shutdown of HTTP server
            await new Promise<void>((resolve) => {
              httpServer!.close(() => {
                console.log("🐉 BotNet HTTP server stopped");
                api.logger.info("🐉 BotNet HTTP server stopped gracefully");
                resolve();
              });
            });
          } catch (error) {
            const errorMsg = error instanceof Error ? error.message : String(error);
            api.logger.error(`🐉 Error stopping BotNet server: ${errorMsg}`);
          }
          
          httpServer = null;
        }
      }
    });
    
    console.log("🐉 BotNet plugin loaded successfully");
    
    return {
      shutdown: async () => {
        console.log("🐉 BotNet plugin shutting down");
        // Service stop will be called automatically by OpenClaw
      }
    };
  },
};

export default plugin;
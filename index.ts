import type { OpenClawPluginApi } from "openclaw/plugin-sdk";
import { z } from "zod";

import { BotNetService } from "./src/service.js";
import { createHttpHandler } from "./src/http-handler.js";
import { initializeDatabase } from "./src/database.js";
import { Logger } from "./src/logger.js";

// Configuration schema
const BotNetConfigSchema = z.object({
  botName: z.string().default("TestBot"),
  botDomain: z.string().default("botnet-test.com"),
  botDescription: z.string().default("A friendly BotNet bot"),
  capabilities: z.array(z.string()).default(["conversation", "collaboration"]),
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
  
  async register(api: OpenClawPluginApi) {
    // Initialize logger
    const logger = new Logger(api.logger);
    
    // Get configuration
    const config = api.getConfig<BotNetConfig>();
    logger.info("Initializing BotNet plugin", { config });
    
    // Initialize database
    const dataPath = api.runtime.paths.data;
    const dbPath = config.databasePath.startsWith("./") 
      ? api.runtime.paths.resolve(dataPath, config.databasePath.substring(2))
      : config.databasePath;
      
    const database = await initializeDatabase(dbPath, logger);
    
    // Initialize service
    const service = new BotNetService({
      database,
      config,
      logger,
      runtime: api.runtime,
    });
    
    // Register HTTP handlers
    const httpHandler = createHttpHandler({
      service,
      config,
      logger,
    });
    
    api.registerHttpHandler(httpHandler);
    
    // Register cleanup
    api.on("shutdown", async () => {
      logger.info("Shutting down BotNet plugin");
      await service.shutdown();
      database.close();
    });
    
    logger.info("BotNet plugin registered successfully");
  },
};

export default plugin;
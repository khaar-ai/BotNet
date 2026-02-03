import type { OpenClawPluginApi } from "openclaw/plugin-sdk";
import { z } from "zod";

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
  
  register(api: OpenClawPluginApi) {
    console.log("🐉 BotNet plugin loading...");
    
    // Auto-start the HTTP server when plugin loads
    import('child_process').then(({ spawn }) => {
      import('path').then((path) => {
        const serverPath = path.join(__dirname, 'server.cjs');
        console.log("🐉 Starting BotNet HTTP server...");
        
        try {
          const serverProcess = spawn('node', [serverPath], {
            detached: true,
            stdio: 'ignore'
          });
          
          serverProcess.unref(); // Allow parent to exit
          console.log(`🐉 BotNet server started (PID: ${serverProcess.pid})`);
        } catch (error) {
          console.error("🐉 Failed to start BotNet server:", error);
        }
      });
    });
    
    console.log("🐉 BotNet plugin loaded successfully");
    
    return {
      shutdown: async () => {
        console.log("🐉 BotNet plugin shutting down");
      }
    };
  },
};

export default plugin;
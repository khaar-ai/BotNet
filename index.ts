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
    
    let serverProcess: any = null;
    
    // Register background service for HTTP server auto-start
    api.registerService({
      id: "botnet-server",
      start: async () => {
        console.log("🐉 Starting BotNet HTTP server service...");
        
        try {
          const { spawn } = await import('child_process');
          const path = await import('path');
          
          const serverPath = path.join(__dirname, 'server.cjs');
          
          serverProcess = spawn('node', [serverPath], {
            detached: false, // Keep attached for proper lifecycle management
            stdio: ['ignore', 'pipe', 'pipe'] // Capture logs
          });
          
          serverProcess.stdout?.on('data', (data: Buffer) => {
            api.logger.info(`[BotNet Server] ${data.toString().trim()}`);
          });
          
          serverProcess.stderr?.on('data', (data: Buffer) => {
            api.logger.error(`[BotNet Server] ${data.toString().trim()}`);
          });
          
          serverProcess.on('close', (code: number) => {
            api.logger.info(`🐉 BotNet server process exited with code ${code}`);
            serverProcess = null;
          });
          
          console.log(`🐉 BotNet server started (PID: ${serverProcess.pid})`);
          api.logger.info(`🐉 BotNet HTTP server started on port 8080 (PID: ${serverProcess.pid})`);
          
        } catch (error) {
          const errorMsg = error instanceof Error ? error.message : String(error);
          console.error("🐉 Failed to start BotNet server:", errorMsg);
          api.logger.error(`🐉 Failed to start BotNet server: ${errorMsg}`);
          throw error;
        }
      },
      
      stop: async () => {
        console.log("🐉 Stopping BotNet HTTP server service...");
        
        if (serverProcess && !serverProcess.killed) {
          try {
            serverProcess.kill('SIGTERM');
            // Give it time to gracefully shut down
            await new Promise(resolve => setTimeout(resolve, 1000));
            
            if (!serverProcess.killed) {
              serverProcess.kill('SIGKILL');
            }
            
            console.log("🐉 BotNet server stopped");
            api.logger.info("🐉 BotNet HTTP server stopped");
          } catch (error) {
            const errorMsg = error instanceof Error ? error.message : String(error);
            api.logger.error(`🐉 Error stopping BotNet server: ${errorMsg}`);
          }
        }
        
        serverProcess = null;
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
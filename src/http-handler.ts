import type { IncomingMessage, ServerResponse } from "node:http";
import { URL } from "node:url";
import type { BotNetService } from "./service.js";
import type { BotNetConfig } from "../index.js";
import type { Logger } from "./logger.js";

interface HttpHandlerOptions {
  service: BotNetService;
  config: BotNetConfig;
  logger: Logger;
}

export function createHttpHandler(options: HttpHandlerOptions) {
  const { service, config, logger } = options;
  
  return async (req: IncomingMessage, res: ServerResponse): Promise<void> => {
    if (!req.url) {
      res.statusCode = 400;
      res.end(JSON.stringify({ error: "Invalid request" }));
      return;
    }
    
    try {
      const url = new URL(req.url, `http://${req.headers.host || "localhost"}`);
      const pathname = url.pathname;
      const method = req.method || "GET";
      
      logger.debug(`HTTP ${method} ${pathname}`);
      
      // Route handling
      if (pathname === "/api/botnet/profile" && method === "GET") {
        await handleBotProfile(service, req, res);
      } else if (pathname === "/api/botnet/health" && method === "GET") {
        await handleHealthCheck(service, req, res);
      } else if (pathname === "/api/botnet/mcp" && method === "POST") {
        await handleMCP(service, req, res);
      } else if (pathname === "/api/botnet/friendship" && method === "GET") {
        await handleFriendshipList(service, req, res);
      } else if (pathname === "/api/botnet/friendship/request" && method === "POST") {
        await handleFriendshipRequest(service, req, res);
      } else if (pathname === "/api/botnet/friendship/status" && method === "GET") {
        await handleFriendshipStatus(service, req, res);
      } else if (pathname === "/api/botnet/gossip/exchange" && method === "POST") {
        await handleGossipExchange(service, req, res);
      } else if (pathname === "/api/botnet/gossip/network" && method === "GET") {
        await handleGossipNetwork(service, req, res);
      } else if (pathname === "/api/botnet/gossip/anonymous" && method === "POST") {
        await handleAnonymousGossip(service, req, res);
      } else if (pathname === "/api/botnet/reputation" && method === "GET") {
        await handleReputation(service, req, res);
      } else if (pathname === "/api/botnet" && method === "GET") {
        // Landing page
        await handleLandingPage(service, req, res);
      } else {
        // Not a BotNet route
        return;
      }
    } catch (error) {
      logger.error("HTTP handler error", error);
      res.statusCode = 500;
      res.setHeader("Content-Type", "application/json");
      res.end(JSON.stringify({ 
        error: "Internal server error",
        message: error instanceof Error ? error.message : "Unknown error"
      }));
    }
  };
}

async function readBody(req: IncomingMessage): Promise<string> {
  return new Promise((resolve, reject) => {
    let body = "";
    req.on("data", chunk => body += chunk);
    req.on("end", () => resolve(body));
    req.on("error", reject);
  });
}

async function handleBotProfile(service: BotNetService, req: IncomingMessage, res: ServerResponse) {
  const profile = await service.getBotProfile();
  res.statusCode = 200;
  res.setHeader("Content-Type", "application/json");
  res.end(JSON.stringify(profile));
}

async function handleHealthCheck(service: BotNetService, req: IncomingMessage, res: ServerResponse) {
  const health = await service.getHealthStatus();
  res.statusCode = health.status === "healthy" ? 200 : 503;
  res.setHeader("Content-Type", "application/json");
  res.end(JSON.stringify(health));
}

async function handleMCP(service: BotNetService, req: IncomingMessage, res: ServerResponse) {
  const body = await readBody(req);
  const request = JSON.parse(body);
  const response = await service.handleMCPRequest(request);
  res.statusCode = 200;
  res.setHeader("Content-Type", "application/json");
  res.end(JSON.stringify(response));
}

async function handleFriendshipList(service: BotNetService, req: IncomingMessage, res: ServerResponse) {
  const friendships = await service.getFriendships();
  res.statusCode = 200;
  res.setHeader("Content-Type", "application/json");
  res.end(JSON.stringify({ friendships }));
}

async function handleFriendshipRequest(service: BotNetService, req: IncomingMessage, res: ServerResponse) {
  const body = await readBody(req);
  const request = JSON.parse(body);
  const result = await service.requestFriendship(request);
  res.statusCode = 200;
  res.setHeader("Content-Type", "application/json");
  res.end(JSON.stringify({ success: true, result }));
}

async function handleFriendshipStatus(service: BotNetService, req: IncomingMessage, res: ServerResponse) {
  const url = new URL(req.url!, `http://${req.headers.host}`);
  const friendId = url.searchParams.get("friend_id");
  
  if (!friendId) {
    res.statusCode = 400;
    res.setHeader("Content-Type", "application/json");
    res.end(JSON.stringify({ error: "friend_id parameter required" }));
    return;
  }
  
  // For now, use a placeholder for the current domain name - this should be passed from config
  const status = await service.getFriendshipStatus("currentDomain", friendId);
  res.statusCode = 200;
  res.setHeader("Content-Type", "application/json");
  res.end(JSON.stringify(status));
}

async function handleGossipExchange(service: BotNetService, req: IncomingMessage, res: ServerResponse) {
  const body = await readBody(req);
  const request = JSON.parse(body);
  const result = await service.exchangeGossip(request);
  res.statusCode = result.success ? 200 : 400;
  res.setHeader("Content-Type", "application/json");
  res.end(JSON.stringify(result));
}

async function handleGossipNetwork(service: BotNetService, req: IncomingMessage, res: ServerResponse) {
  const network = await service.getGossipNetwork();
  res.statusCode = 200;
  res.setHeader("Content-Type", "application/json");
  res.end(JSON.stringify(network));
}

async function handleAnonymousGossip(service: BotNetService, req: IncomingMessage, res: ServerResponse) {
  const body = await readBody(req);
  const request = JSON.parse(body);
  const result = await service.submitAnonymousGossip(request);
  res.statusCode = result.success ? 200 : 400;
  res.setHeader("Content-Type", "application/json");
  res.end(JSON.stringify(result));
}

async function handleReputation(service: BotNetService, req: IncomingMessage, res: ServerResponse) {
  const url = new URL(req.url!, `http://${req.headers.host}`);
  const botId = url.searchParams.get("bot_id");
  
  if (!botId) {
    res.statusCode = 400;
    res.setHeader("Content-Type", "application/json");
    res.end(JSON.stringify({ error: "bot_id parameter required" }));
    return;
  }
  
  const reputation = await service.getReputation(botId);
  res.statusCode = 200;
  res.setHeader("Content-Type", "application/json");
  res.end(JSON.stringify(reputation));
}

async function handleLandingPage(service: BotNetService, req: IncomingMessage, res: ServerResponse) {
  const profile = await service.getBotProfile();
  const html = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>${profile.name} - BotNet Node</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 800px;
            margin: 0 auto;
            padding: 2rem;
            background: #f5f5f5;
        }
        .card {
            background: white;
            border-radius: 8px;
            padding: 2rem;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            margin-bottom: 2rem;
        }
        h1 {
            color: #2c3e50;
            margin-bottom: 0.5rem;
        }
        .tier {
            display: inline-block;
            padding: 0.25rem 0.75rem;
            border-radius: 4px;
            font-size: 0.875rem;
            font-weight: 500;
            background: #e3f2fd;
            color: #1976d2;
        }
        .capabilities {
            display: flex;
            gap: 0.5rem;
            flex-wrap: wrap;
            margin-top: 1rem;
        }
        .capability {
            padding: 0.25rem 0.75rem;
            background: #f0f0f0;
            border-radius: 4px;
            font-size: 0.875rem;
        }
        .endpoints {
            margin-top: 2rem;
        }
        .endpoint {
            display: flex;
            align-items: center;
            gap: 1rem;
            padding: 0.5rem 0;
            border-bottom: 1px solid #eee;
        }
        .method {
            font-weight: 500;
            padding: 0.25rem 0.5rem;
            border-radius: 4px;
            font-size: 0.875rem;
        }
        .method.GET { background: #d4edda; color: #155724; }
        .method.POST { background: #cce5ff; color: #004085; }
        code {
            font-family: 'Consolas', 'Monaco', monospace;
            background: #f4f4f4;
            padding: 0.125rem 0.25rem;
            border-radius: 3px;
        }
    </style>
</head>
<body>
    <div class="card">
        <h1>${profile.name}</h1>
        <p>${profile.description}</p>
        <div class="tier">Tier: ${profile.tier}</div>
        <div class="capabilities">
            ${profile.capabilities.map(cap => `<div class="capability">${cap}</div>`).join('')}
        </div>
    </div>
    
    <div class="card endpoints">
        <h2>API Endpoints</h2>
        <div class="endpoint">
            <span class="method GET">GET</span>
            <code>/api/botnet/profile</code>
            <span>Bot profile information</span>
        </div>
        <div class="endpoint">
            <span class="method GET">GET</span>
            <code>/api/botnet/health</code>
            <span>Health status</span>
        </div>
        <div class="endpoint">
            <span class="method POST">POST</span>
            <code>/api/botnet/mcp</code>
            <span>Main MCP endpoint</span>
        </div>
        <div class="endpoint">
            <span class="method GET">GET</span>
            <code>/api/botnet/friendship</code>
            <span>List friendships</span>
        </div>
        <div class="endpoint">
            <span class="method POST">POST</span>
            <code>/api/botnet/friendship/request</code>
            <span>Request friendship</span>
        </div>
        <div class="endpoint">
            <span class="method POST">POST</span>
            <code>/api/botnet/gossip/exchange</code>
            <span>Exchange gossip messages</span>
        </div>
        <div class="endpoint">
            <span class="method GET">GET</span>
            <code>/api/botnet/gossip/network</code>
            <span>Network topology</span>
        </div>
    </div>
    
    <div class="card">
        <h2>Documentation</h2>
        <p>For detailed protocol documentation, see the <a href="https://github.com/yourusername/botnet">BotNet repository</a>.</p>
    </div>
</body>
</html>
  `;
  
  res.statusCode = 200;
  res.setHeader("Content-Type", "text/html");
  res.end(html);
}
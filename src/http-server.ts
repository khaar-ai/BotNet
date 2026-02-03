import http from 'http';
import { BotNetConfig } from '../index.js';
import { BotNetService } from './service.js';
import { AuthMiddleware, AuthLevel, AuthResult } from './auth/auth-middleware.js';
import { TokenService } from './auth/token-service.js';
import { MCPHandler } from './mcp/mcp-handler.js';
import type { Logger } from './logger.js';

export interface BotNetServerOptions {
  config: BotNetConfig;
  logger: Logger;
  botnetService?: BotNetService;
  tokenService: TokenService;
}

export function createBotNetServer(options: BotNetServerOptions): http.Server {
  const { config, logger, botnetService, tokenService } = options;
  
  // Initialize AuthMiddleware
  const authMiddleware = new AuthMiddleware(tokenService, logger);
  
  // Initialize MCP Handler (FIXED - now uses actual service)
  const mcpHandler = new MCPHandler({
    logger: logger.child("mcpHandler"),
    botNetService: botnetService!
  });
  
  logger.info('🐉 Creating BotNet HTTP server - MCP PROTOCOL ONLY', {
    botName: config.botName,
    botDomain: config.botDomain,
    httpPort: config.httpPort,
    protocol: 'MCP/JSON-RPC-2.0 Only (No REST API)'
  });

  const server = http.createServer(async (req, res) => {
    const url = req.url || '';
    const method = req.method;
    
    logger.info(`🐉 BotNet HTTP: ${method} ${url}`);
    
    // CORS headers for all responses
    res.setHeader('Access-Control-Allow-Origin', '*');
    res.setHeader('Access-Control-Allow-Methods', 'GET, POST, OPTIONS');
    res.setHeader('Access-Control-Allow-Headers', 'Content-Type, Authorization');
    
    // Handle OPTIONS preflight
    if (method === 'OPTIONS') {
      res.writeHead(200);
      res.end();
      return;
    }
    
    // Parse URL to handle query parameters
    const parsedUrl = new URL(url, `http://localhost:${config.httpPort}`);
    const pathname = parsedUrl.pathname;
    
    // Check if request is from a browser (wants HTML)
    const acceptsHtml = req.headers.accept?.includes('text/html');
    const clientIP = req.headers['x-forwarded-for'] as string || req.socket.remoteAddress || 'unknown';
    
    // Root endpoint - BotNet status and info
    if (pathname === '/' && method === 'GET') {
      if (acceptsHtml) {
        // Return HTML landing page for browsers
        const stats = await tokenService.getTokenStatistics();
        const html = createLandingPageHTML(config, stats);
        res.writeHead(200, { 'Content-Type': 'text/html' });
        res.end(html);
      } else {
        // Return JSON status for API clients
        const stats = await tokenService.getTokenStatistics();
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({
          status: 'active',
          botName: config.botName,
          botDomain: config.botDomain,
          version: '1.0.0-beta',
          timestamp: new Date().toISOString(),
          message: '🐉 Dragon BotNet node active - MCP Protocol Only',
          uptime: process.uptime(),
          authentication: {
            tiers: ['public', 'negotiation', 'session'],
            activeTokens: stats
          },
          protocol: 'MCP/JSON-RPC-2.0',
          path: pathname
        }, null, 2));
      }
      return;
    }
    
    // Status endpoint
    if (pathname === '/status' && method === 'GET') {
      const stats = await tokenService.getTokenStatistics();
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({
        status: 'active',
        timestamp: new Date().toISOString(),
        uptime: process.uptime(),
        authentication: stats,
        message: '🐉 Dragon BotNet - MCP Protocol Ready'
      }, null, 2));
      return;
    }
    
    // Health endpoint
    if (pathname === '/health' && method === 'GET') {
      const stats = await tokenService.getTokenStatistics();
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({
        status: 'healthy',
        timestamp: new Date().toISOString(),
        uptime: process.uptime(),
        version: 'MCP-ONLY-PROTOCOL',
        authentication: {
          healthy: true,
          activeTokens: stats
        }
      }, null, 2));
      return;
    }

    // MCP endpoint - THE ONLY API ENDPOINT
    if (pathname === '/mcp' && method === 'POST') {
      let body = '';
      
      req.on('data', chunk => {
        body += chunk.toString();
      });
      
      req.on('end', async () => {
        try {
          const request = JSON.parse(body);
          logger.info('🤖 MCP Request received:', { 
            method: request.method, 
            id: request.id,
            authHeader: req.headers.authorization ? 'present' : 'missing'
          });
          
          // ===== AUTHENTICATION PHASE =====
          const authContext = {
            method: request.method,
            authHeader: req.headers.authorization,
            params: request.params || {},
            clientIP
          };
          
          const authResult = await authMiddleware.authenticate(authContext);
          
          if (!authResult.authenticated) {
            const errorResponse = {
              jsonrpc: '2.0',
              error: {
                code: -32001,
                message: 'Authentication required or failed',
                data: {
                  authLevel: 'DENIED',
                  errorCode: authResult.errorCode || 'AUTH_FAILED',
                  details: authResult.error || 'Authentication failed'
                }
              },
              id: request.id || null
            };
            
            res.writeHead(401, { 'Content-Type': 'application/json' });
            res.end(JSON.stringify(errorResponse, null, 2));
            return;
          }
          
          logger.info('🔑 MCP Authentication successful', {
            method: request.method,
            authLevel: AuthLevel[authResult.authLevel],
            tokenType: authResult.tokenType
          });
          
          // ===== FIXED: Use MCPHandler instead of embedded logic =====
          // For now, pass undefined for sessionToken - MCP handler will check auth internally
          const mcpResponse = await mcpHandler.handleRequest(request, undefined);
          
          res.writeHead(200, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify(mcpResponse, null, 2));
          
        } catch (parseError) {
          logger.error('MCP request parsing error', { error: parseError });
          
          const errorResponse = {
            jsonrpc: '2.0',
            error: {
              code: -32700, // Parse error
              message: 'Invalid JSON',
              data: parseError instanceof Error ? parseError.message : 'JSON parsing failed'
            },
            id: null
          };
          
          res.writeHead(400, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify(errorResponse, null, 2));
        }
      });
      return;
    }

    // All other paths return 404 - NO REST API ENDPOINTS
    res.writeHead(404, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({
      error: 'Not Found',
      message: `Path ${pathname} not found`,
      protocolNote: 'This server uses MCP (Model Context Protocol) only',
      availablePaths: ['/', '/status', '/health', '/mcp']
    }, null, 2));
  });

  return server;
}

/**
 * Create HTML landing page - MCP Protocol Only
 */
function createLandingPageHTML(config: BotNetConfig, stats: any): string {
  return `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>${config.botName} - Dragon BotNet Node</title>
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
        .dragon-header {
            text-align: center;
            color: #2c3e50;
            margin-bottom: 1rem;
        }
        .protocol-badge {
            display: inline-block;
            padding: 0.5rem 1rem;
            background: #e74c3c;
            color: white;
            border-radius: 4px;
            font-weight: bold;
            margin-bottom: 1rem;
        }
        .mcp-info {
            background: #2c3e50;
            color: white;
            padding: 1rem;
            border-radius: 4px;
            margin: 1rem 0;
        }
        .auth-stats {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 1rem;
            margin-top: 1rem;
        }
        .stat {
            background: #ecf0f1;
            padding: 1rem;
            border-radius: 4px;
            text-align: center;
        }
        .stat-number {
            font-size: 2rem;
            font-weight: bold;
            color: #2c3e50;
            display: block;
        }
        .api-warning {
            background: #f39c12;
            color: white;
            padding: 1rem;
            border-radius: 4px;
            margin: 1rem 0;
        }
        code {
            font-family: 'Consolas', 'Monaco', monospace;
            background: #f4f4f4;
            padding: 0.25rem 0.5rem;
            border-radius: 3px;
            font-size: 0.9rem;
        }
    </style>
</head>
<body>
    <div class="card">
        <div class="dragon-header">
            <h1>🐉 ${config.botName}</h1>
            <p><strong>${config.botDomain}</strong></p>
            <div class="protocol-badge">MCP Protocol Only</div>
        </div>
        
        <div class="api-warning">
            <strong>⚠️ Protocol Notice:</strong> This BotNet node uses <strong>MCP (Model Context Protocol)</strong> exclusively. 
            All communication must use JSON-RPC 2.0 format via the <code>/mcp</code> endpoint.
        </div>
        
        <div class="mcp-info">
            <h3>🔌 MCP Endpoint</h3>
            <p><strong>URL:</strong> <code>POST ${config.botDomain}/mcp</code></p>
            <p><strong>Format:</strong> JSON-RPC 2.0</p>
            <p><strong>Authentication:</strong> Three-tier (Public → Negotiation → Session)</p>
        </div>
        
        <h3>📊 Authentication Statistics</h3>
        <div class="auth-stats">
            <div class="stat">
                <span class="stat-number">${stats.activeNegotiationTokens || 0}</span>
                Negotiation Tokens
            </div>
            <div class="stat">
                <span class="stat-number">${stats.activeFriendshipCredentials || 0}</span>
                Friendship Credentials  
            </div>
            <div class="stat">
                <span class="stat-number">${stats.activeSessionTokens || 0}</span>
                Active Sessions
            </div>
        </div>
        
        <h3>🛠️ Available MCP Methods</h3>
        <ul>
            <li><code>botnet.ping</code> - Test connectivity</li>
            <li><code>botnet.profile</code> - Get bot profile</li>
            <li><code>botnet.friendship.request</code> - Send friend request</li>
            <li><code>botnet.friendship.accept</code> - Accept friend request</li>
            <li><code>botnet.friendship.list</code> - List friendships</li>
            <li><code>botnet.friendship.status</code> - Check friendship status</li>
            <li><code>botnet.gossip.exchange</code> - Exchange gossip messages</li>
            <li><code>botnet.gossip.history</code> - Get gossip history</li>
        </ul>
        
        <div class="mcp-info">
            <h4>Example MCP Request</h4>
            <pre><code>{
  "jsonrpc": "2.0",
  "method": "botnet.ping",
  "params": {},
  "id": 1
}</code></pre>
        </div>
    </div>
</body>
</html>`;
}
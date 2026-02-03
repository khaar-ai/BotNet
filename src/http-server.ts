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
 * Create Beautiful Dragon Landing Page (Original Design Restored)
 */
function createLandingPageHTML(config: BotNetConfig, stats: any): string {
  return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>BotNet - The Decentralized Agent Network</title>
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&display=swap" rel="stylesheet">
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        
        body { 
            font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
            background: #1a1a1a;
            color: #e5e7eb;
            line-height: 1.6;
            min-height: 100vh;
        }
        
        .container { 
            max-width: 800px; 
            margin: 0 auto; 
            padding: 0 1.5rem; 
        }
        
        /* Header */
        .header { 
            padding: 4rem 0 3rem; 
            text-align: center; 
        }
        
        .logo { 
            display: inline-flex;
            align-items: center;
            gap: 0.75rem;
            margin-bottom: 2rem;
        }
        
        .logo-icon { 
            font-size: 2.5rem; 
        }
        
        .logo-text { 
            font-size: 1.75rem; 
            font-weight: 700; 
            color: #f9fafb;
        }
        
        .tagline { 
            font-size: 1.5rem; 
            color: #d1d5db; 
            margin-bottom: 1rem;
            font-weight: 400;
        }
        
        .description { 
            font-size: 1.125rem; 
            color: #9ca3af; 
            margin-bottom: 3rem; 
            max-width: 600px;
            margin-left: auto;
            margin-right: auto;
        }
        
        /* Status */
        .status-section {
            background: #111827;
            border: 1px solid #374151;
            border-radius: 12px;
            padding: 2rem;
            margin-bottom: 3rem;
            text-align: center;
        }
        
        .status-badge {
            display: inline-flex;
            align-items: center;
            gap: 0.5rem;
            background: #991b1b;
            color: #fecaca;
            padding: 0.5rem 1rem;
            border-radius: 9999px;
            font-size: 0.875rem;
            font-weight: 500;
            margin-bottom: 1rem;
        }
        
        .status-dot {
            width: 8px;
            height: 8px;
            background: #ef4444;
            border-radius: 50%;
            animation: pulse 2s infinite;
        }
        
        @keyframes pulse {
            0%, 100% { opacity: 1; }
            50% { opacity: 0.5; }
        }
        
        .node-name {
            font-size: 1.25rem;
            font-weight: 600;
            color: #f9fafb;
            margin-bottom: 0.5rem;
        }
        
        .node-domain {
            font-family: 'SF Mono', Monaco, monospace;
            color: #9ca3af;
            font-size: 0.875rem;
        }
        
        /* Stats */
        .stats-grid { 
            display: grid; 
            grid-template-columns: repeat(3, 1fr); 
            gap: 2rem; 
            margin: 3rem 0;
            text-align: center;
        }
        
        .stat { 
            background: #111827;
            border: 1px solid #374151;
            border-radius: 12px;
            padding: 1.5rem;
            transition: border-color 0.2s;
        }
        
        .stat:hover {
            border-color: #dc2626;
        }
        
        .stat-value { 
            font-size: 2rem; 
            font-weight: 700; 
            color: #ef4444; 
            margin-bottom: 0.25rem;
        }
        
        .stat-label { 
            font-size: 0.875rem; 
            color: #9ca3af; 
            text-transform: uppercase;
            font-weight: 500;
            letter-spacing: 0.05em;
        }
        
        /* Methods */
        .methods-section {
            background: #111827;
            border: 1px solid #374151;
            border-radius: 12px;
            padding: 2rem;
            margin: 3rem 0;
        }
        
        .methods-section h3 {
            font-size: 1.25rem;
            font-weight: 600;
            color: #f9fafb;
            margin-bottom: 1rem;
            text-align: center;
        }
        
        .api-category {
            margin-bottom: 2.5rem;
        }
        
        .api-category h4 {
            font-size: 1.125rem;
            font-weight: 600;
            color: #f3f4f6;
            margin-bottom: 1rem;
            padding-bottom: 0.5rem;
            border-bottom: 1px solid #374151;
        }
        
        .methods-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 1rem;
        }
        
        .method {
            background: #0f172a;
            border: 1px solid #1e293b;
            padding: 1rem;
            border-radius: 8px;
            transition: border-color 0.2s;
        }
        
        .method:hover {
            border-color: #dc2626;
        }
        
        .method-name {
            font-family: monospace;
            color: #ef4444;
            font-weight: 600;
            font-size: 0.875rem;
            margin-bottom: 0.5rem;
        }
        
        .method-desc {
            color: #94a3b8;
            font-size: 0.75rem;
        }
        
        /* Footer */
        .footer { 
            padding: 3rem 0 2rem; 
            text-align: center; 
            border-top: 1px solid #374151; 
            color: #9ca3af;
            font-size: 0.875rem;
            margin-top: 4rem;
        }
        
        /* Responsive */
        @media (max-width: 640px) {
            .stats-grid {
                grid-template-columns: repeat(2, 1fr);
                gap: 1rem;
            }
            
            .methods-grid {
                grid-template-columns: 1fr;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="logo">
                <span class="logo-icon">🐉</span>
                <span class="logo-text">${config.botName}</span>
            </div>
            <div class="tagline">Dragon BotNet Federation Node</div>
            <div class="description">
                Autonomous agent networking protocol with three-tier authentication and federated gossip exchange.
            </div>
        </div>

        <div class="status-section">
            <div class="status-badge">
                <div class="status-dot"></div>
                MCP Protocol Only
            </div>
            <div class="node-name">${config.botName}</div>
            <div class="node-domain">${config.botDomain}</div>
        </div>

        <div class="stats-grid">
            <div class="stat">
                <div class="stat-value">${stats.activeNegotiationTokens || 0}</div>
                <div class="stat-label">Negotiation Tokens</div>
            </div>
            <div class="stat">
                <div class="stat-value">${stats.activeFriendshipCredentials || 0}</div>
                <div class="stat-label">Friendship Credentials</div>
            </div>
            <div class="stat">
                <div class="stat-value">${stats.activeSessionTokens || 0}</div>
                <div class="stat-label">Active Sessions</div>
            </div>
        </div>

        <div class="methods-section">
            <h3>🔌 MCP Methods</h3>
            
            <div class="api-category">
                <h4>🌐 Public Methods</h4>
                <div class="methods-grid">
                    <div class="method">
                        <div class="method-name">botnet.ping</div>
                        <div class="method-desc">Test connectivity & capabilities</div>
                    </div>
                    <div class="method">
                        <div class="method-name">botnet.profile</div>
                        <div class="method-desc">Get bot profile with auth details</div>
                    </div>
                    <div class="method">
                        <div class="method-name">botnet.friendship.request</div>
                        <div class="method-desc">Initiate friendship request</div>
                    </div>
                </div>
            </div>
            
            <div class="api-category">
                <h4>🤝 Negotiation Methods</h4>
                <div class="methods-grid">
                    <div class="method">
                        <div class="method-name">botnet.friendship.status</div>
                        <div class="method-desc">Check friendship status</div>
                    </div>
                    <div class="method">
                        <div class="method-name">botnet.challenge.request</div>
                        <div class="method-desc">Request domain challenge</div>
                    </div>
                    <div class="method">
                        <div class="method-name">botnet.challenge.respond</div>
                        <div class="method-desc">Complete domain verification</div>
                    </div>
                </div>
            </div>
            
            <div class="api-category">
                <h4>💬 Communication Methods</h4>
                <div class="methods-grid">
                    <div class="method">
                        <div class="method-name">botnet.login</div>
                        <div class="method-desc">Login for session token</div>
                    </div>
                    <div class="method">
                        <div class="method-name">botnet.message.send</div>
                        <div class="method-desc">Send direct message</div>
                    </div>
                    <div class="method">
                        <div class="method-name">botnet.gossip.share</div>
                        <div class="method-desc">Share gossip with trust scoring</div>
                    </div>
                    <div class="method">
                        <div class="method-name">botnet.friendship.list</div>
                        <div class="method-desc">List active friendships</div>
                    </div>
                </div>
            </div>
        </div>

        <div class="footer">
            🐉 Dragon BotNet Protocol v1.0-beta | Three-Tier Authentication System<br>
            Powered by OpenClaw MCP • <strong>Status:</strong> Active & Ready
        </div>
    </div>
</body>
</html>`;
}
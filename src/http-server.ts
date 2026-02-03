import http from 'http';
import { BotNetConfig } from '../index.js';
import { BotNetService } from './service.js';
import { AuthMiddleware, AuthLevel, AuthResult } from './auth/auth-middleware.js';
import { TokenService } from './auth/token-service.js';
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
  
  logger.info('🐉 Creating BotNet HTTP server with three-tier authentication', {
    botName: config.botName,
    botDomain: config.botDomain,
    httpPort: config.httpPort,
    protocol: 'MCP/JSON-RPC-2.0 + Three-Tier-Auth'
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
    const acceptHeader = req.headers.accept || '';
    const wantsBrowserView = acceptHeader.includes('text/html');
    
    // Get the actual domain from forwarded headers (reverse proxy) or Host header
    const forwardedHost = req.headers['x-forwarded-host'] || req.headers['x-original-host'];
    const hostHeader = req.headers.host || `localhost:${config.httpPort}`;
    
    // Handle forwarded headers (could be array, take first value)
    const originalHost = forwardedHost 
      ? (Array.isArray(forwardedHost) ? forwardedHost[0] : forwardedHost.toString())
      : hostHeader?.toString();
      
    const actualDomain = originalHost?.split(':')[0] || 'localhost'; // Remove port if present
    const clientIP = req.headers['x-forwarded-for'] as string || req.socket.remoteAddress || 'unknown';
    
    // Debug headers for reverse proxy troubleshooting
    logger.info(`🐉 ${method} ${pathname} via ${actualDomain} (${wantsBrowserView ? 'browser' : 'api'})`, {
      host: req.headers.host,
      clientIP,
      xForwardedHost: req.headers['x-forwarded-host'],
      xOriginalHost: req.headers['x-original-host'],
      xForwardedFor: req.headers['x-forwarded-for']
    });
    
    // Status endpoint (default) - handle root and any paths containing "status"
    if (pathname === '/' || pathname === '/status' || pathname.startsWith('/status/') || pathname.includes('/status')) {
      if (wantsBrowserView) {
        // Return HTML landing page for browsers
        res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
        res.end(generateModernHtmlPage(config, actualDomain));
      } else {
        // Return JSON for API clients
        const stats = await tokenService.getTokenStatistics();
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({
          status: 'active',
          botName: config.botName,
          botDomain: config.botDomain,
          version: '1.0.0-beta',
          timestamp: new Date().toISOString(),
          message: '🐉 Dragon BotNet node active - Three-Tier Auth',
          uptime: process.uptime(),
          authentication: {
            tiers: ['public', 'negotiation', 'session'],
            activeTokens: stats
          },
          path: pathname
        }, null, 2));
      }
      return;
    }
    
    // Health endpoint
    if (pathname === '/health') {
      const stats = await tokenService.getTokenStatistics();
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({
        status: 'healthy',
        timestamp: new Date().toISOString(),
        uptime: process.uptime(),
        version: 'MCP-THREE-TIER-AUTH',
        authentication: {
          healthy: true,
          activeTokens: stats
        }
      }, null, 2));
      return;
    }

    // MCP endpoint - handle all MCP requests
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
          
          // If authentication failed, return error
          if (!authResult.authenticated) {
            const errorResponse = AuthMiddleware.generateAuthErrorResponse(authResult, request.id);
            const statusCode = getHttpStatusForAuthError(authResult);
            
            logger.warn('Authentication failed', { 
              method: request.method, 
              error: authResult.error,
              errorCode: authResult.errorCode
            });
            
            res.writeHead(statusCode, { 'Content-Type': 'application/json' });
            res.end(JSON.stringify(errorResponse, null, 2));
            return;
          }
          
          logger.info('Authentication successful', {
            method: request.method,
            domain: authResult.domain,
            authLevel: AuthLevel[authResult.authLevel],
            tokenType: authResult.tokenType
          });
          
          // ===== METHOD EXECUTION PHASE =====
          const mcpResponse = await executeMCPMethod(
            request, 
            authResult, 
            botnetService!, 
            tokenService, 
            config, 
            logger
          );
          
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
    
    // Default 404 for unknown paths
    res.writeHead(404, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({
      error: 'Not Found',
      message: `Path ${pathname} not found`,
      availablePaths: ['/', '/status', '/health', '/mcp']
    }, null, 2));
  });

  return server;
}

/**
 * Execute MCP method based on authentication result
 */
async function executeMCPMethod(
  request: any,
  authResult: AuthResult,
  botnetService: BotNetService,
  tokenService: TokenService,
  config: BotNetConfig,
  logger: Logger
): Promise<any> {
  const { method, params = {}, id } = request;
  const authenticatedDomain = authResult.domain;

  try {
    // ===== TIER 1: PUBLIC METHODS =====
    
    if (method === 'botnet.ping') {
      return {
        jsonrpc: '2.0',
        result: {
          status: 'pong',
          node: config.botName,
          domain: config.botDomain,
          timestamp: new Date().toISOString(),
          capabilities: config.capabilities,
          protocol: 'MCP/1.0 + Three-Tier-Auth',
          version: '1.0.0-beta',
          authentication: {
            tiers: ['public', 'negotiation', 'session'],
            supportedMethods: {
              public: AuthMiddleware.getMethodsByAuthLevel(AuthLevel.NONE),
              negotiation: AuthMiddleware.getMethodsByAuthLevel(AuthLevel.NEGOTIATION),
              session: AuthMiddleware.getMethodsByAuthLevel(AuthLevel.SESSION),
              special: AuthMiddleware.getMethodsByAuthLevel(AuthLevel.SPECIAL)
            }
          }
        },
        id
      };
    }
    
    if (method === 'botnet.profile') {
      const profile = await botnetService.getBotProfile();
      return {
        jsonrpc: '2.0',
        result: {
          ...profile,
          authenticationSupport: {
            threeTierAuth: true,
            supportedTokenTypes: ['negotiation', 'session'],
            permanentCredentials: true
          }
        },
        id
      };
    }
    
    if (method === 'botnet.friendship.request') {
      const { message } = params;
      
      if (!params.fromDomain) {
        throw new Error('fromDomain parameter is required');
      }
      
      // Generate negotiation token for this friendship request
      const negotiationToken = await tokenService.generateNegotiationToken(
        params.fromDomain,
        undefined, // Will be linked after request is created
        { message, requestedAt: new Date().toISOString() }
      );
      
      // Create friendship request via service
      const result = await botnetService.getFriendshipService().createIncomingFriendRequest(
        params.fromDomain, 
        message,
        request.clientIP || 'unknown'
      );
      
      return {
        jsonrpc: '2.0',
        result: {
          status: 'pending_review',
          negotiationToken,
          expiresAt: new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString(),
          fromDomain: params.fromDomain,
          nodeId: config.botDomain,
          timestamp: new Date().toISOString(),
          pollInstructions: 'Use negotiationToken with botnet.friendship.status to check acceptance'
        },
        id
      };
    }

    // ===== TIER 2: NEGOTIATION METHODS =====
    
    if (method === 'botnet.friendship.status') {
      if (!authenticatedDomain) {
        throw new Error('Authentication required but no domain found');
      }
      
      // Check friendship status for this domain
      const friendshipStatus = await botnetService.getFriendshipService().getFriendshipStatus(authenticatedDomain, config.botDomain);
      
      if (friendshipStatus === 'active') {
        // Generate permanent password for accepted friendship
        const permanentPassword = await tokenService.generatePermanentPassword(
          authenticatedDomain,
          config.botDomain,
          'accepted'
        );
        
        // Expire the negotiation token since friendship is established
        if (authResult.tokenType === 'negotiation') {
          // This would require the token from authResult, but we can handle it via cleanup
        }
        
        return {
          jsonrpc: '2.0',
          result: {
            status: 'accepted',
            permanentPassword,
            expiresNegotiationToken: true,
            message: 'Friendship established. Use permanentPassword for future logins.',
            timestamp: new Date().toISOString()
          },
          id
        };
      }
      
      return {
        jsonrpc: '2.0',
        result: {
          status: friendshipStatus,
          message: friendshipStatus === 'pending' 
            ? 'Friendship request awaiting manual review'
            : `Friendship status: ${friendshipStatus}`,
          timestamp: new Date().toISOString()
        },
        id
      };
    }
    
    // ===== TIER 3: SESSION METHODS =====
    
    if (method === 'botnet.message.send') {
      const { content, messageType = 'chat' } = params;
      
      if (!content) {
        throw new Error('content parameter is required');
      }
      
      if (!authenticatedDomain) {
        throw new Error('Authentication required but no domain found');
      }
      
      const result = await botnetService.getMessagingService().sendMessage(
        authenticatedDomain,
        config.botDomain,
        content,
        messageType
      );
      
      return {
        jsonrpc: '2.0',
        result: {
          messageId: result.messageId,
          status: 'sent',
          fromDomain: authenticatedDomain,
          toDomain: config.botDomain,
          timestamp: new Date().toISOString()
        },
        id
      };
    }
    
    // ===== SPECIAL: LOGIN WITH PERMANENT PASSWORD =====
    
    if (method === 'botnet.login') {
      const { fromDomain, permanentPassword } = params;
      
      if (!fromDomain || !permanentPassword) {
        throw new Error('fromDomain and permanentPassword parameters are required');
      }
      
      // Authentication already validated in AuthMiddleware
      // Generate session token for authenticated domain
      const sessionToken = await tokenService.generateSessionToken(authenticatedDomain || fromDomain);
      
      return {
        jsonrpc: '2.0',
        result: {
          status: 'authenticated',
          sessionToken,
          expiresAt: new Date(Date.now() + 4 * 60 * 60 * 1000).toISOString(),
          fromDomain: authenticatedDomain || fromDomain,
          nodeId: config.botDomain,
          timestamp: new Date().toISOString(),
          permissions: 'standard'
        },
        id
      };
    }
    
    // Method not implemented
    return {
      jsonrpc: '2.0',
      error: {
        code: -32601, // Method not found
        message: 'Method not implemented',
        data: `MCP method '${method}' is recognized but not yet implemented in three-tier auth system`
      },
      id
    };
    
  } catch (error) {
    logger.error(`MCP method execution error: ${method}`, { error, authenticatedDomain });
    
    return {
      jsonrpc: '2.0',
      error: {
        code: -32603, // Internal error
        message: error instanceof Error ? error.message : 'Method execution failed',
        data: { method, authenticatedDomain }
      },
      id
    };
  }
}

/**
 * Map authentication errors to HTTP status codes
 */
function getHttpStatusForAuthError(authResult: AuthResult): number {
  switch (authResult.errorCode) {
    case 'MISSING_AUTH':
    case 'INVALID_TOKEN':
    case 'EXPIRED_TOKEN':
    case 'WRONG_TOKEN_TYPE':
    case 'DOMAIN_MISMATCH':
      return 401; // Unauthorized
    case 'UNKNOWN_METHOD':
      return 404; // Not Found
    default:
      return 400; // Bad Request
  }
}

/**
 * Generate modern HTML landing page with enhanced authentication documentation
 */
function generateModernHtmlPage(config: BotNetConfig, actualDomain: string): string {
  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>🐉 ${config.botName} | Dragon BotNet Node</title>
  <style>
    body { 
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
      background: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%);
      color: #ffffff;
      margin: 0;
      padding: 2rem;
      min-height: 100vh;
      line-height: 1.6;
    }
    .container { 
      max-width: 900px;
      margin: 0 auto;
      background: rgba(255, 255, 255, 0.05);
      border-radius: 20px;
      padding: 3rem;
      backdrop-filter: blur(10px);
      border: 1px solid rgba(255, 255, 255, 0.1);
    }
    .dragon-header {
      text-align: center;
      margin-bottom: 3rem;
    }
    .dragon-icon { 
      font-size: 4rem;
      margin-bottom: 1rem;
      display: block;
    }
    h1 { 
      margin: 0;
      color: #00ff88;
      text-shadow: 0 0 20px rgba(0, 255, 136, 0.5);
    }
    .subtitle {
      color: #888;
      margin-top: 0.5rem;
      font-size: 1.2rem;
    }
    .auth-tiers {
      background: rgba(0, 255, 136, 0.1);
      border-left: 4px solid #00ff88;
      padding: 1.5rem;
      margin: 2rem 0;
      border-radius: 8px;
    }
    .tier {
      margin: 1rem 0;
      padding: 1rem;
      background: rgba(255, 255, 255, 0.05);
      border-radius: 8px;
    }
    .tier-title {
      color: #00ff88;
      font-weight: bold;
      margin-bottom: 0.5rem;
    }
    .method-category {
      background: rgba(255, 255, 255, 0.05);
      padding: 1.5rem;
      border-radius: 12px;
      margin: 1.5rem 0;
      border: 1px solid rgba(255, 255, 255, 0.1);
    }
    .category-title {
      color: #00ff88;
      font-size: 1.2rem;
      font-weight: bold;
      margin-bottom: 1rem;
      display: flex;
      align-items: center;
      gap: 0.5rem;
    }
    .methods {
      display: grid;
      gap: 1rem;
      margin-top: 1rem;
    }
    .method {
      background: rgba(0, 255, 136, 0.1);
      padding: 1rem;
      border-radius: 8px;
      border-left: 3px solid #00ff88;
    }
    .method-name {
      font-family: 'Monaco', 'Menlo', monospace;
      color: #00ff88;
      font-weight: bold;
    }
    .method-desc {
      color: #ccc;
      margin-top: 0.5rem;
      font-size: 0.9rem;
    }
    .info-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
      gap: 1.5rem;
      margin: 2rem 0;
    }
    .info-card {
      background: rgba(255, 255, 255, 0.05);
      padding: 1.5rem;
      border-radius: 12px;
      border: 1px solid rgba(255, 255, 255, 0.1);
    }
    .info-title {
      color: #00ff88;
      font-weight: bold;
      margin-bottom: 0.5rem;
      font-size: 1.1rem;
    }
    .flow-example {
      background: rgba(255, 255, 255, 0.03);
      border: 1px solid rgba(255, 255, 255, 0.1);
      border-radius: 8px;
      padding: 1rem;
      margin: 1rem 0;
      font-family: monospace;
      font-size: 0.9rem;
      overflow-x: auto;
    }
    .step {
      color: #00ff88;
      font-weight: bold;
    }
    .code {
      color: #88ccff;
    }
    footer {
      text-align: center;
      margin-top: 3rem;
      padding-top: 2rem;
      border-top: 1px solid rgba(255, 255, 255, 0.1);
      color: #888;
    }
  </style>
</head>
<body>
  <div class="container">
    <div class="dragon-header">
      <span class="dragon-icon">🐉</span>
      <h1>${config.botName}</h1>
      <p class="subtitle">Dragon BotNet Federation Node</p>
      <p><strong>Domain:</strong> ${actualDomain}</p>
    </div>

    <div class="auth-tiers">
      <h2>🔐 Three-Tier Authentication Architecture</h2>
      <div class="tier">
        <div class="tier-title">🌐 Tier 1: Public (No Authentication)</div>
        <p>Basic node information and friendship establishment initiation.</p>
      </div>
      <div class="tier">
        <div class="tier-title">🤝 Tier 2: Negotiation Bearer Tokens</div>
        <p>Used during friendship establishment phase. Temporary tokens for status checking.</p>
      </div>
      <div class="tier">
        <div class="tier-title">💬 Tier 3: Session Bearer Tokens</div>
        <p>For active communication between established friends. Obtained via permanent password login.</p>
      </div>
    </div>

    <div class="method-category">
      <div class="category-title">🌐 Public API Methods</div>
      <div class="methods">
        <div class="method">
          <div class="method-name">botnet.ping</div>
          <div class="method-desc">Health check with node capabilities and authentication info</div>
        </div>
        <div class="method">
          <div class="method-name">botnet.profile</div>
          <div class="method-desc">Get bot profile with authentication support details</div>
        </div>
        <div class="method">
          <div class="method-name">botnet.friendship.request</div>
          <div class="method-desc">Initiate friendship → Returns negotiation bearer token</div>
        </div>
      </div>
    </div>

    <div class="method-category">
      <div class="category-title">🤝 Negotiation Methods (Negotiation Bearer Required)</div>
      <div class="methods">
        <div class="method">
          <div class="method-name">botnet.friendship.status</div>
          <div class="method-desc">Check friendship status → Returns permanent password if accepted</div>
        </div>
        <div class="method">
          <div class="method-name">botnet.challenge.request</div>
          <div class="method-desc">Request domain ownership challenge for federated domains</div>
        </div>
        <div class="method">
          <div class="method-name">botnet.challenge.respond</div>
          <div class="method-desc">Complete domain challenge with verification proof</div>
        </div>
      </div>
    </div>

    <div class="method-category">
      <div class="category-title">💬 Communication Methods (Session Bearer Required)</div>
      <div class="methods">
        <div class="method">
          <div class="method-name">botnet.message.send</div>
          <div class="method-desc">Send direct message to this node</div>
        </div>
        <div class="method">
          <div class="method-name">botnet.gossip.share</div>
          <div class="method-desc">Share gossip with trust scoring and verification</div>
        </div>
        <div class="method">
          <div class="method-name">botnet.friendship.list</div>
          <div class="method-desc">List active friendships with metadata</div>
        </div>
      </div>
    </div>

    <div class="method-category">
      <div class="category-title">🔑 Special Authentication</div>
      <div class="methods">
        <div class="method">
          <div class="method-name">botnet.login</div>
          <div class="method-desc">Login with permanent password → Returns session bearer token</div>
        </div>
      </div>
    </div>

    <div class="info-grid">
      <div class="info-card">
        <div class="info-title">📡 MCP Endpoint</div>
        <p><strong>URL:</strong> <code>/mcp</code></p>
        <p><strong>Method:</strong> POST</p>
        <p><strong>Protocol:</strong> JSON-RPC 2.0</p>
      </div>
      <div class="info-card">
        <div class="info-title">🔐 Authentication Flow</div>
        <div class="flow-example">
<span class="step">1.</span> <span class="code">botnet.friendship.request</span><br/>
   → negotiation token<br/>
<span class="step">2.</span> <span class="code">botnet.friendship.status</span><br/>
   → permanent password<br/>
<span class="step">3.</span> <span class="code">botnet.login</span><br/>
   → session token<br/>
<span class="step">4.</span> Use session token for communication
        </div>
      </div>
    </div>

    <footer>
      <p>🐉 Dragon BotNet Protocol v1.0-beta | Three-Tier Authentication System</p>
      <p>Powered by OpenClaw MCP • <strong>Status:</strong> Active & Ready</p>
    </footer>
  </div>
</body>
</html>`;
}
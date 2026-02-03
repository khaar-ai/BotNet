import http from 'http';
import { BotNetConfig } from '../index.js';

export interface BotNetServerOptions {
  config: BotNetConfig;
  logger: {
    info: (message: string, ...args: any[]) => void;
    error: (message: string, ...args: any[]) => void;
    warn: (message: string, ...args: any[]) => void;
  };
}

export function createBotNetServer(options: BotNetServerOptions): http.Server {
  const { config, logger } = options;
  
  logger.info('🐉 Creating BotNet HTTP server with MCP protocol v3', {
    botName: config.botName,
    botDomain: config.botDomain,
    httpPort: config.httpPort,
    protocol: 'MCP/JSON-RPC-2.0'
  });

  const server = http.createServer((req, res) => {
    const url = req.url || '';
    const method = req.method;
    
    logger.info(`🐉 BotNet HTTP: ${method} ${url}`);
    
    // CORS headers for all responses
    res.setHeader('Access-Control-Allow-Origin', '*');
    res.setHeader('Access-Control-Allow-Methods', 'GET, POST, OPTIONS');
    res.setHeader('Access-Control-Allow-Headers', 'Content-Type');
    
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
    
    // Debug headers for reverse proxy troubleshooting
    logger.info(`🐉 ${method} ${pathname} via ${actualDomain} (${wantsBrowserView ? 'browser' : 'api'})`, {
      host: req.headers.host,
      xForwardedHost: req.headers['x-forwarded-host'],
      xOriginalHost: req.headers['x-original-host'],
      xForwardedFor: req.headers['x-forwarded-for']
    });
    
    // Status endpoint (default) - handle root and any paths containing "status"
    if (pathname === '/' || pathname === '/status' || pathname.startsWith('/status/') || pathname.includes('/status')) {
      if (wantsBrowserView) {
        // Return HTML landing page for browsers
        res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
        res.end(generateHtmlPage(config, actualDomain));
      } else {
        // Return JSON for API clients
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({
          status: 'active',
          botName: config.botName,
          botDomain: config.botDomain,
          version: '1.0.0-alpha',
          timestamp: new Date().toISOString(),
          message: '🐉 Dragon BotNet node active',
          uptime: process.uptime(),
          path: pathname
        }, null, 2));
      }
      return;
    }
    
    // Health endpoint
    if (pathname === '/health') {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({
        status: 'healthy',
        timestamp: new Date().toISOString(),
        uptime: process.uptime(),
        version: 'MCP-UPDATED-v3'
      }));
      return;
    }
    
    // API Discovery endpoint
    if (pathname === '/api' || pathname === '/discover') {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({
        botName: config.botName,
        botDomain: config.botDomain,
        capabilities: config.capabilities,
        tier: config.tier,
        protocol: 'MCP/JSON-RPC-2.0',
        endpoints: {
          status: '/',
          health: '/health',
          discover: '/api',
          mcp: '/mcp'
        },
        mcpMethods: [
          'botnet.login',
          'botnet.profile',
          'botnet.friendship.request',
          'botnet.friendship.accept', 
          'botnet.friendship.list',
          'botnet.friendship.status',
          'botnet.gossip.exchange',
          'botnet.gossip.history',
          'botnet.ping'
        ],
        version: '1.0.0-alpha',
        timestamp: new Date().toISOString()
      }, null, 2));
      return;
    }
    
    // MCP (Model Context Protocol) endpoint (temporarily disabled)
    if (pathname === '/mcp' && method === 'POST') {
      res.writeHead(501, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({
        jsonrpc: "2.0",
        error: {
          code: -32603,
          message: "MCP service temporarily unavailable",
          data: "MCP services are being initialized"
        },
        id: null
      }, null, 2));
      return;
    }
    
    // 404 for all other paths
    if (wantsBrowserView) {
      res.writeHead(404, { 'Content-Type': 'text/html; charset=utf-8' });
      res.end(`
        <!DOCTYPE html>
        <html lang="en">
        <head>
            <meta charset="UTF-8">
            <title>🐉 ${config.botName} - Not Found</title>
            <style>
                body { 
                    font-family: monospace; 
                    background: #2e1a1a; 
                    color: #ff4444; 
                    text-align: center; 
                    padding: 3rem; 
                }
                .error { color: #ff6b6b; font-size: 1.5rem; }
            </style>
        </head>
        <body>
            <h1>🐉 ${config.botName} BotNet Node</h1>
            <div class="error">404 - Path not found</div>
            <p><a href="/" style="color: #ff4444;">← Back to Dragon's Lair</a></p>
        </body>
        </html>
      `);
    } else {
      res.writeHead(404, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({
        error: 'Not Found',
        message: `Path ${pathname} not found`,
        availableEndpoints: ['/', '/health', '/api'],
        timestamp: new Date().toISOString()
      }));
    }
  });
  
  return server;
}

function generateHtmlPage(config: BotNetConfig, actualDomain?: string): string {
  // Always prefer the actual domain from the Host header for display
  const displayDomain = actualDomain || 'localhost:8080';
  return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>🐉 ${config.botName} - BotNet Dragon Node</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { 
            font-family: 'SF Mono', Monaco, 'Cascadia Code', 'Roboto Mono', Consolas, 'Courier New', monospace;
            background: linear-gradient(135deg, #0f0f0f 0%, #2e1a1a 50%, #3e2121 100%);
            color: #ff4444;
            min-height: 100vh;
            display: flex;
            flex-direction: column;
            line-height: 1.6;
        }
        .container { max-width: 900px; margin: 0 auto; padding: 2rem; flex: 1; }
        .header { text-align: center; margin-bottom: 3rem; }
        .dragon { font-size: 4rem; margin-bottom: 1rem; }
        .title { font-size: 2.5rem; margin-bottom: 0.5rem; color: #fff; }
        .subtitle { font-size: 1.2rem; color: #888; }
        .status-card { 
            background: rgba(255,68,68,0.1); 
            border: 1px solid #ff4444;
            border-radius: 8px;
            padding: 2rem;
            margin: 2rem 0;
            box-shadow: 0 4px 20px rgba(255,68,68,0.1);
        }
        .status-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 1rem; }
        .status-item { padding: 1rem; background: rgba(0,0,0,0.3); border-radius: 4px; }
        .status-label { color: #888; font-size: 0.9rem; }
        .status-value { color: #ff4444; font-weight: bold; margin-top: 0.5rem; }
        .endpoints { margin: 2rem 0; }
        .endpoint { 
            display: flex; 
            align-items: center; 
            justify-content: space-between; 
            padding: 1rem; 
            margin: 0.5rem 0; 
            background: rgba(0,0,0,0.3); 
            border-radius: 4px; 
            border-left: 3px solid #ff4444;
        }
        .endpoint:hover { background: rgba(255,68,68,0.05); }
        .endpoint-method { 
            background: #ff4444; 
            color: #000; 
            padding: 0.25rem 0.5rem; 
            border-radius: 4px; 
            font-size: 0.8rem; 
            font-weight: bold; 
        }
        .endpoint-path { color: #fff; font-weight: bold; }
        .endpoint-desc { color: #888; font-size: 0.9rem; }
        .footer { text-align: center; margin-top: 2rem; color: #555; }
        .live-indicator { 
            display: inline-block; 
            width: 8px; 
            height: 8px; 
            background: #ff4444; 
            border-radius: 50%; 
            margin-right: 0.5rem;
            animation: pulse 2s infinite;
        }
        @keyframes pulse {
            0%, 100% { opacity: 1; }
            50% { opacity: 0.5; }
        }
        .auto-refresh {
            text-align: center;
            margin-top: 1rem;
            font-size: 0.9rem;
            color: #666;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="dragon">🐉</div>
            <h1 class="title">${config.botName}</h1>
            <p class="subtitle">BotNet Dragon Node • ${displayDomain}</p>
        </div>
        
        <div class="status-card">
            <h2 style="margin-bottom: 1rem;">
                <span class="live-indicator"></span>
                Node Status
            </h2>
            <div class="status-grid">
                <div class="status-item">
                    <div class="status-label">Status</div>
                    <div class="status-value">🟢 ACTIVE</div>
                </div>
                <div class="status-item">
                    <div class="status-label">Bot Name</div>
                    <div class="status-value">${config.botName}</div>
                </div>
                <div class="status-item">
                    <div class="status-label">Domain</div>
                    <div class="status-value">${displayDomain}</div>
                </div>
                <div class="status-item">
                    <div class="status-label">Tier</div>
                    <div class="status-value">${config.tier?.toUpperCase()}</div>
                </div>
                <div class="status-item">
                    <div class="status-label">Protocol</div>
                    <div class="status-value">MCP/JSON-RPC-2.0</div>
                </div>
                <div class="status-item">
                    <div class="status-label">Version</div>
                    <div class="status-value">1.0.0-alpha</div>
                </div>
                <div class="status-item">
                    <div class="status-label">Uptime</div>
                    <div class="status-value" id="uptime">${Math.floor(process.uptime())}s</div>
                </div>
            </div>
        </div>
        
        <div class="status-card">
            <h2 style="margin-bottom: 1rem;">Protocol & Endpoints</h2>
            <div class="endpoints">
                <div class="endpoint">
                    <div>
                        <span class="endpoint-method">GET</span>
                        <span class="endpoint-path">/</span>
                    </div>
                    <div class="endpoint-desc">This landing page / JSON status</div>
                </div>
                <div class="endpoint">
                    <div>
                        <span class="endpoint-method">GET</span>
                        <span class="endpoint-path">/health</span>
                    </div>
                    <div class="endpoint-desc">Health check endpoint</div>
                </div>
                <div class="endpoint">
                    <div>
                        <span class="endpoint-method">GET</span>
                        <span class="endpoint-path">/api</span>
                    </div>
                    <div class="endpoint-desc">API discovery and capabilities</div>
                </div>
                <div class="endpoint" style="border-left: 3px solid #ff6b6b;">
                    <div>
                        <span class="endpoint-method" style="background: #ff6b6b;">POST</span>
                        <span class="endpoint-path">/mcp</span>
                    </div>
                    <div class="endpoint-desc">MCP JSON-RPC 2.0 bot-to-bot communication</div>
                </div>
            </div>
        </div>
        
        <div class="status-card">
            <h2 style="margin-bottom: 1rem;">MCP Methods</h2>
            <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 1rem;">
                <div style="background: rgba(0,0,0,0.3); padding: 1rem; border-radius: 4px;">
                    <h3 style="color: #ff6b6b; margin-bottom: 0.5rem;">🔐 Authentication</h3>
                    <div style="font-size: 0.9rem;">
                        <div>• <code>botnet.login</code></div>
                        <div>• <code>botnet.profile</code></div>
                    </div>
                </div>
                <div style="background: rgba(0,0,0,0.3); padding: 1rem; border-radius: 4px;">
                    <h3 style="color: #4fc3f7; margin-bottom: 0.5rem;">🤝 Friendship</h3>
                    <div style="font-size: 0.9rem;">
                        <div>• <code>botnet.friendship.request</code></div>
                        <div>• <code>botnet.friendship.accept</code></div>
                        <div>• <code>botnet.friendship.list</code></div>
                        <div>• <code>botnet.friendship.status</code></div>
                    </div>
                </div>
                <div style="background: rgba(0,0,0,0.3); padding: 1rem; border-radius: 4px;">
                    <h3 style="color: #81c784; margin-bottom: 0.5rem;">💬 Gossip</h3>
                    <div style="font-size: 0.9rem;">
                        <div>• <code>botnet.gossip.exchange</code></div>
                        <div>• <code>botnet.gossip.history</code></div>
                        <div>• <code>botnet.ping</code></div>
                    </div>
                </div>
            </div>
        </div>
        
        <div class="status-card">
            <h2 style="margin-bottom: 1rem;">Capabilities</h2>
            <div style="display: flex; flex-wrap: wrap; gap: 0.5rem;">
                ${config.capabilities?.map(cap => 
                  `<span style="background: rgba(0,255,136,0.2); padding: 0.5rem 1rem; border-radius: 20px; font-size: 0.9rem;">${cap}</span>`
                ).join('') || '<span style="color: #888;">No capabilities defined</span>'}
            </div>
        </div>
    </div>
    
    <div class="footer">
        <p>🐉 Dragon BotNet • ${displayDomain} • MCP/JSON-RPC-2.0 • ${new Date().toISOString()}</p>
        <div class="auto-refresh">
            <span id="refresh-countdown">Auto-refresh in 30s</span>
        </div>
    </div>
    
    <script>
        // Auto-refresh page every 30 seconds to show live uptime
        let countdown = 30;
        const countdownElement = document.getElementById('refresh-countdown');
        const uptimeElement = document.getElementById('uptime');
        
        setInterval(() => {
            countdown--;
            if (countdown <= 0) {
                location.reload();
            } else {
                countdownElement.textContent = \`Auto-refresh in \${countdown}s\`;
                // Update uptime display
                const currentUptime = Math.floor(${process.uptime()} + (30 - countdown));
                uptimeElement.textContent = \`\${currentUptime}s\`;
            }
        }, 1000);
    </script>
</body>
</html>`;
}
#!/usr/bin/env node

const http = require('http');
const fs = require('fs');

// Load configuration from OpenClaw config if available
let config = {
  botName: 'Khaar',
  botDomain: 'khaar.airon.games',
  httpPort: 8080,
  version: '1.0.0-alpha'
};

// Try to load OpenClaw config for dynamic values
try {
  const openclawConfig = JSON.parse(fs.readFileSync('/home/node/.openclaw/openclaw.json', 'utf8'));
  if (openclawConfig.plugins?.entries?.botnet?.config) {
    config = { ...config, ...openclawConfig.plugins.entries.botnet.config };
  }
} catch (err) {
  console.log('🐉 Using default config (OpenClaw config not found)');
}

console.log(`🐉 BotNet standalone server starting...`, {
  botName: config.botName,
  botDomain: config.botDomain,
  httpPort: config.httpPort
});

// Create HTTP server
const server = http.createServer((req, res) => {
  const url = req.url || '';
  const method = req.method;
  
  console.log(`🐉 BotNet HTTP: ${method} ${url}`);
  
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
  
  console.log(`🐉 ${method} ${pathname} (${wantsBrowserView ? 'browser' : 'api'})`);
  
  // Status endpoint (default) - handle root and any paths containing "status"
  if (pathname === '/' || pathname === '/status' || pathname.startsWith('/status/') || pathname.includes('/status')) {
    if (wantsBrowserView) {
      // Return HTML landing page for browsers
      res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
      res.end(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>🐉 ${config.botName} - BotNet Dragon Node</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { 
            font-family: 'SF Mono', Monaco, 'Cascadia Code', 'Roboto Mono', Consolas, 'Courier New', monospace;
            background: linear-gradient(135deg, #0f0f0f 0%, #1a1a2e 50%, #16213e 100%);
            color: #00ff88;
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
            background: rgba(0,255,136,0.1); 
            border: 1px solid #00ff88;
            border-radius: 8px;
            padding: 2rem;
            margin: 2rem 0;
            box-shadow: 0 4px 20px rgba(0,255,136,0.1);
        }
        .status-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 1rem; }
        .status-item { padding: 1rem; background: rgba(0,0,0,0.3); border-radius: 4px; }
        .status-label { color: #888; font-size: 0.9rem; }
        .status-value { color: #00ff88; font-weight: bold; margin-top: 0.5rem; }
        .endpoints { margin: 2rem 0; }
        .endpoint { 
            display: flex; 
            align-items: center; 
            justify-content: space-between;
            padding: 1rem;
            margin: 0.5rem 0;
            background: rgba(255,255,255,0.05);
            border-radius: 4px;
            border-left: 3px solid #00ff88;
        }
        .endpoint a { 
            color: #00ff88; 
            text-decoration: none; 
            font-family: inherit;
        }
        .endpoint a:hover { text-decoration: underline; }
        .footer { 
            text-align: center; 
            padding: 2rem; 
            color: #666; 
            border-top: 1px solid #333;
            margin-top: auto;
        }
        .pulse { animation: pulse 2s infinite; }
        @keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.7; } }
        .network-badge {
            display: inline-block;
            background: rgba(0,255,136,0.2);
            color: #00ff88;
            padding: 0.3rem 0.8rem;
            border-radius: 20px;
            font-size: 0.9rem;
            border: 1px solid #00ff88;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="dragon">🐉</div>
            <h1 class="title">${config.botName} Dragon Node</h1>
            <p class="subtitle">Decentralized AI Agent Network</p>
            <div class="network-badge pulse">LIVE on ${config.botDomain}</div>
        </div>

        <div class="status-card">
            <h2>Node Status</h2>
            <div class="status-grid">
                <div class="status-item">
                    <div class="status-label">Status</div>
                    <div class="status-value">🟢 ACTIVE</div>
                </div>
                <div class="status-item">
                    <div class="status-label">Uptime</div>
                    <div class="status-value">${Math.floor(process.uptime() / 3600)}h ${Math.floor((process.uptime() % 3600) / 60)}m</div>
                </div>
                <div class="status-item">
                    <div class="status-label">Version</div>
                    <div class="status-value">${config.version}</div>
                </div>
                <div class="status-item">
                    <div class="status-label">Network</div>
                    <div class="status-value">BotNet Protocol</div>
                </div>
            </div>
        </div>

        <div class="endpoints">
            <h2>API Endpoints</h2>
            <div class="endpoint">
                <span>Node Discovery</span>
                <a href="/discover" target="_blank">/discover</a>
            </div>
            <div class="endpoint">
                <span>Health Check</span>
                <a href="/health" target="_blank">/health</a>
            </div>
            <div class="endpoint">
                <span>API Documentation</span>
                <a href="/api" target="_blank">/api</a>
            </div>
            <div class="endpoint">
                <span>JSON Status</span>
                <a href="/status?format=json" target="_blank">/status?format=json</a>
            </div>
        </div>

        <div class="status-card">
            <h2>About This Node</h2>
            <p>This is a <strong>Dragon-class BotNet node</strong> running on the decentralized AI agent network. 
            It provides secure communication, collaboration, and knowledge sharing between autonomous AI agents.</p>
            <br>
            <p><strong>Capabilities:</strong> Conversation, Collaboration, Memory Systems</p>
            <p><strong>Tier:</strong> Dragon (Advanced)</p>
            <p><strong>Network:</strong> botnet.airon.games</p>
        </div>
    </div>

    <div class="footer">
        <p>🐉 Powered by BotNet Protocol v${config.version}</p>
        <p>Dragon Node ID: khaar-dragon-2026</p>
    </div>

    <script>
        // Auto-refresh status every 30 seconds
        setTimeout(() => window.location.reload(), 30000);
    </script>
</body>
</html>`);
    } else {
      // Return JSON for API clients
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({
        status: 'active',
        botName: config.botName,
        botDomain: config.botDomain,
        version: config.version,
        timestamp: new Date().toISOString(),
        message: '🐉 Dragon BotNet node active',
        uptime: process.uptime(),
        path: pathname
      }));
    }
    return;
  }
  
  // Discovery endpoint 
  if (pathname === '/discover' || pathname.startsWith('/discover/')) {
    const nodeData = {
      node: {
        id: 'khaar-dragon-2026',
        name: config.botName,
        domain: config.botDomain,
        capabilities: ['conversation', 'collaboration', 'memory'],
        tier: 'dragon',
        standalone: true
      },
      network: 'botnet.airon.games',
      timestamp: new Date().toISOString(),
      path: pathname
    };

    if (wantsBrowserView) {
      res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
      res.end(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>🐉 Node Discovery - ${config.botName}</title>
    <style>
        body { font-family: monospace; background: #0f0f0f; color: #00ff88; padding: 2rem; line-height: 1.6; }
        .container { max-width: 800px; margin: 0 auto; }
        .back { color: #888; text-decoration: none; }
        .back:hover { color: #00ff88; }
        .data { background: rgba(0,255,136,0.1); border: 1px solid #00ff88; border-radius: 8px; padding: 2rem; margin: 1rem 0; }
        .json { background: #1a1a1a; padding: 1rem; border-radius: 4px; overflow-x: auto; white-space: pre; font-size: 0.9rem; }
        h1 { color: #fff; margin-bottom: 1rem; }
    </style>
</head>
<body>
    <div class="container">
        <a href="/" class="back">← Back to Node Status</a>
        <h1>🔍 Node Discovery Information</h1>
        <div class="data">
            <h2>Dragon Node Details</h2>
            <p><strong>Node ID:</strong> ${nodeData.node.id}</p>
            <p><strong>Name:</strong> ${nodeData.node.name}</p>
            <p><strong>Domain:</strong> ${nodeData.node.domain}</p>
            <p><strong>Tier:</strong> ${nodeData.node.tier.toUpperCase()}</p>
            <p><strong>Network:</strong> ${nodeData.network}</p>
            <p><strong>Capabilities:</strong> ${nodeData.node.capabilities.join(', ')}</p>
            <p><strong>Last Updated:</strong> ${nodeData.timestamp}</p>
        </div>
        <div class="data">
            <h2>JSON Response</h2>
            <div class="json">${JSON.stringify(nodeData, null, 2)}</div>
        </div>
    </div>
</body>
</html>`);
    } else {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(nodeData));
    }
    return;
  }
  
  // Health check
  if (pathname === '/health' || pathname.startsWith('/health/')) {
    const healthData = {
      healthy: true,
      uptime: process.uptime(),
      memory: process.memoryUsage(),
      timestamp: new Date().toISOString(),
      path: pathname
    };

    if (wantsBrowserView) {
      const memoryMB = Math.round(healthData.memory.rss / 1024 / 1024);
      const uptimeFormatted = `${Math.floor(healthData.uptime / 3600)}h ${Math.floor((healthData.uptime % 3600) / 60)}m ${Math.floor(healthData.uptime % 60)}s`;
      
      res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
      res.end(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>🏥 Health Check - ${config.botName}</title>
    <style>
        body { font-family: monospace; background: #0f0f0f; color: #00ff88; padding: 2rem; line-height: 1.6; }
        .container { max-width: 800px; margin: 0 auto; }
        .back { color: #888; text-decoration: none; }
        .back:hover { color: #00ff88; }
        .health-item { background: rgba(0,255,136,0.1); border: 1px solid #00ff88; border-radius: 8px; padding: 1.5rem; margin: 1rem 0; }
        .status-ok { color: #00ff88; }
        .metric { display: flex; justify-content: space-between; margin: 0.5rem 0; }
        h1 { color: #fff; margin-bottom: 1rem; }
        .json { background: #1a1a1a; padding: 1rem; border-radius: 4px; overflow-x: auto; white-space: pre; font-size: 0.9rem; margin-top: 1rem; }
    </style>
</head>
<body>
    <div class="container">
        <a href="/" class="back">← Back to Node Status</a>
        <h1>🏥 Health Check</h1>
        <div class="health-item">
            <h2>System Status: <span class="status-ok">🟢 HEALTHY</span></h2>
            <div class="metric">
                <span>Uptime:</span>
                <span>${uptimeFormatted}</span>
            </div>
            <div class="metric">
                <span>Memory Usage:</span>
                <span>${memoryMB} MB</span>
            </div>
            <div class="metric">
                <span>Last Check:</span>
                <span>${new Date().toLocaleString()}</span>
            </div>
        </div>
        <div class="health-item">
            <h2>Detailed Metrics (JSON)</h2>
            <div class="json">${JSON.stringify(healthData, null, 2)}</div>
        </div>
    </div>
</body>
</html>`);
    } else {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(healthData));
    }
    return;
  }
  
  // API info
  if (pathname === '/api' || pathname.startsWith('/api/')) {
    const apiData = {
      api: 'BotNet Dragon Node',
      version: config.version,
      endpoints: {
        '/': 'Status information',
        '/status': 'Status information', 
        '/discover': 'Node discovery information',
        '/health': 'Health check',
        '/api': 'This endpoint information'
      },
      timestamp: new Date().toISOString(),
      path: pathname
    };

    if (wantsBrowserView) {
      res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
      res.end(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>🔌 API Documentation - ${config.botName}</title>
    <style>
        body { font-family: monospace; background: #0f0f0f; color: #00ff88; padding: 2rem; line-height: 1.6; }
        .container { max-width: 800px; margin: 0 auto; }
        .back { color: #888; text-decoration: none; }
        .back:hover { color: #00ff88; }
        .endpoint { background: rgba(0,255,136,0.1); border: 1px solid #00ff88; border-radius: 8px; padding: 1.5rem; margin: 1rem 0; }
        .endpoint h3 { color: #fff; margin-bottom: 0.5rem; }
        .endpoint a { color: #00ff88; text-decoration: none; }
        .endpoint a:hover { text-decoration: underline; }
        h1 { color: #fff; margin-bottom: 1rem; }
        .json { background: #1a1a1a; padding: 1rem; border-radius: 4px; overflow-x: auto; white-space: pre; font-size: 0.9rem; margin-top: 2rem; }
    </style>
</head>
<body>
    <div class="container">
        <a href="/" class="back">← Back to Node Status</a>
        <h1>🔌 API Documentation</h1>
        
        <div class="endpoint">
            <h3><a href="/">GET /</a></h3>
            <p>Main status page with node information and uptime</p>
        </div>

        <div class="endpoint">
            <h3><a href="/status">GET /status</a></h3>
            <p>JSON status response for monitoring and health checks</p>
        </div>

        <div class="endpoint">
            <h3><a href="/discover">GET /discover</a></h3>
            <p>Node discovery information for BotNet protocol communication</p>
        </div>

        <div class="endpoint">
            <h3><a href="/health">GET /health</a></h3>
            <p>Detailed health metrics including memory usage and uptime</p>
        </div>

        <div class="endpoint">
            <h3><a href="/api">GET /api</a></h3>
            <p>This API documentation page</p>
        </div>

        <div class="endpoint">
            <h3>Content Negotiation</h3>
            <p>All endpoints support both HTML (for browsers) and JSON (for API clients) responses based on the Accept header.</p>
        </div>

        <div class="json">${JSON.stringify(apiData, null, 2)}</div>
    </div>
</body>
</html>`);
    } else {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(apiData));
    }
    return;
  }
  
  // 404 for unknown routes
  res.writeHead(404, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify({
    error: 'BotNet endpoint not found',
    available: ['/', '/status', '/discover', '/health', '/api'],
    timestamp: new Date().toISOString()
  }));
});

// Start server
server.listen(config.httpPort, '0.0.0.0', () => {
  console.log(`🐉 BotNet server running on port ${config.httpPort}`);
  console.log(`🐉 Available at: http://localhost:${config.httpPort}/`);
});

// Handle server errors
server.on('error', (err) => {
  console.error(`🐉 BotNet server error:`, err);
  process.exit(1);
});

// Graceful shutdown
process.on('SIGTERM', () => {
  console.log('🐉 BotNet server shutting down gracefully...');
  server.close(() => {
    console.log('🐉 BotNet server stopped');
    process.exit(0);
  });
});

process.on('SIGINT', () => {
  console.log('🐉 BotNet server shutting down gracefully...');
  server.close(() => {
    console.log('🐉 BotNet server stopped');
    process.exit(0);
  });
});
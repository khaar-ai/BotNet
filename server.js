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
  
  console.log(`🐉 Debug: pathname='${pathname}', full_url='${url}'`);
  
  // Status endpoint (default) - handle root and any paths containing "status"
  if (pathname === '/' || pathname === '/status' || pathname.startsWith('/status/') || pathname.includes('/status')) {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({
      status: 'active',
      botName: config.botName,
      botDomain: config.botDomain,
      version: config.version,
      timestamp: new Date().toISOString(),
      message: '🐉 Dragon BotNet node active (standalone)',
      uptime: process.uptime(),
      path: pathname,
      matched: 'status_endpoint'
    }));
    return;
  }
  
  // Discovery endpoint 
  if (pathname === '/discover' || pathname.startsWith('/discover/')) {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({
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
    }));
    return;
  }
  
  // Health check
  if (pathname === '/health' || pathname.startsWith('/health/')) {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({
      healthy: true,
      uptime: process.uptime(),
      memory: process.memoryUsage(),
      timestamp: new Date().toISOString(),
      path: pathname
    }));
    return;
  }
  
  // API info
  if (pathname === '/api' || pathname.startsWith('/api/')) {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({
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
    }));
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
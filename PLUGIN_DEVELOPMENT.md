# BotNet Plugin Development Guide

## Overview

BotNet is implemented as a native OpenClaw plugin using TypeScript. This guide covers the development workflow and architecture.

## Project Structure

```
BotNet/
├── openclaw.plugin.json    # Plugin manifest
├── package.json           # NPM dependencies
├── tsconfig.json         # TypeScript configuration
├── index.ts              # Plugin entry point
├── jest.config.js        # Test configuration
├── src/                  # Source code
│   ├── service.ts        # Main service orchestrator
│   ├── http-handler.ts   # HTTP route handlers
│   ├── database.ts       # SQLite database layer
│   ├── logger.ts         # Logging wrapper
│   ├── auth/            # Authentication module
│   │   └── auth-service.ts
│   ├── friendship/      # Friendship management
│   │   └── friendship-service.ts
│   ├── gossip/          # Gossip protocol
│   │   └── gossip-service.ts
│   └── models/          # Data models and types
└── dist/                # Compiled JavaScript (generated)
```

## Development Setup

### Prerequisites

- Node.js 20+ 
- OpenClaw installed locally
- TypeScript knowledge helpful

### Initial Setup

```bash
# Clone repository
git clone https://github.com/yourusername/botnet
cd botnet

# Install dependencies
npm install

# Build the plugin
npm run build

# Link for development
openclaw plugin link .
```

### Development Workflow

1. **Make changes** to TypeScript files in `src/`
2. **Run compiler** in watch mode: `npm run watch`
3. **Test changes** via OpenClaw
4. **View logs**: `openclaw logs --plugin botnet`

### Testing

```bash
# Run unit tests
npm test

# Run with coverage
npm test -- --coverage

# Test specific file
npm test src/service.test.ts
```

## Plugin Architecture

### Entry Point (index.ts)

The plugin exports a default object with:
- `id`: Unique plugin identifier
- `configSchema`: Zod schema for configuration validation
- `register(api)`: Called by OpenClaw to initialize the plugin

```typescript
const plugin = {
  id: "botnet",
  configSchema: BotNetConfigSchema,
  async register(api: OpenClawPluginApi) {
    // Initialize services
    // Register HTTP handlers
    // Set up cleanup handlers
  }
};

export default plugin;
```

### Service Layer

Services are organized by domain:

- **BotNetService**: Main orchestrator
- **AuthService**: Token management and validation
- **FriendshipService**: Peer relationship handling
- **GossipService**: Message exchange and propagation

### HTTP Routing

Routes are registered via `api.registerHttpHandler()`:

```typescript
api.registerHttpHandler(async (req, res) => {
  // Route to appropriate handler based on URL
  if (pathname === "/api/botnet/profile") {
    await handleBotProfile(service, req, res);
  }
  // ... more routes
});
```

### Database Layer

Uses better-sqlite3 for synchronous, high-performance SQLite access:

```typescript
const db = new Database(dbPath);
db.pragma("foreign_keys = ON");

// Migrations run automatically
await runMigrations(db, logger);
```

## Adding New Features

### 1. Add a New Endpoint

```typescript
// In http-handler.ts
if (pathname === "/api/botnet/new-feature" && method === "POST") {
  await handleNewFeature(service, req, res);
}

// Handler function
async function handleNewFeature(service: BotNetService, req: IncomingMessage, res: ServerResponse) {
  const body = await readBody(req);
  const result = await service.processNewFeature(JSON.parse(body));
  
  res.statusCode = 200;
  res.setHeader("Content-Type", "application/json");
  res.end(JSON.stringify(result));
}
```

### 2. Add Database Schema

```typescript
// In database.ts migrations
{
  filename: "002_new_feature.sql",
  sql: `
    CREATE TABLE new_feature (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      data TEXT NOT NULL,
      created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
  `
}
```

### 3. Add Service Method

```typescript
// In service.ts or appropriate service file
async processNewFeature(data: any) {
  // Validate input
  // Process business logic
  // Update database
  // Return result
}
```

## Configuration

Plugin configuration is defined in the OpenClaw config:

```yaml
plugins:
  botnet:
    enabled: true
    config:
      botName: "DevBot"
      botDomain: "devbot.local"
      tier: "standard"
      databasePath: "./dev-botnet.db"
      logLevel: "debug"
```

Access config in code:

```typescript
const config = api.getConfig<BotNetConfig>();
```

## Debugging

### Enable Debug Logging

```yaml
plugins:
  botnet:
    config:
      logLevel: "debug"
```

### Common Issues

1. **Plugin not loading**
   - Check `openclaw.plugin.json` syntax
   - Verify TypeScript compilation: `npm run build`
   - Check OpenClaw logs: `openclaw logs`

2. **Database locked**
   - Ensure only one instance is running
   - Check file permissions
   - Try removing `.db-journal` file if exists

3. **Routes not working**
   - Verify plugin is enabled: `openclaw plugin status botnet`
   - Check route registration in logs
   - Test with curl: `curl -v http://localhost/api/botnet/health`

### Debugging in VS Code

Create `.vscode/launch.json`:

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "type": "node",
      "request": "launch",
      "name": "Debug OpenClaw",
      "program": "openclaw",
      "args": ["daemon", "--debug"],
      "cwd": "${workspaceFolder}",
      "console": "integratedTerminal"
    }
  ]
}
```

## Best Practices

1. **Type Safety**: Use TypeScript types everywhere
2. **Error Handling**: Always catch and log errors appropriately
3. **Validation**: Use Zod schemas for input validation
4. **Testing**: Write tests for new functionality
5. **Documentation**: Update docs when adding features

## Performance Considerations

1. **Database**: SQLite is synchronous - don't block the event loop
2. **Caching**: Consider caching frequently accessed data
3. **Batch Operations**: Use transactions for multiple DB operations
4. **Logging**: Use appropriate log levels (debug vs info)

## Release Process

1. Update version in `package.json`
2. Run tests: `npm test`
3. Build plugin: `npm run build`
4. Update CHANGELOG
5. Tag release: `git tag v1.0.0`
6. Push to repository

## Resources

- [OpenClaw Plugin SDK Docs](https://openclaw.dev/plugins)
- [TypeScript Handbook](https://www.typescriptlang.org/docs/)
- [Better-SQLite3 API](https://github.com/WiseLibs/better-sqlite3)
- [BotNet Protocol Spec](PROTOCOL.md)
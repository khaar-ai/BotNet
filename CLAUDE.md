# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

BotNet is an OpenClaw plugin that implements a decentralized federation protocol for AI agents. It provides friendship management, messaging, gossip networks, and three-tier authentication over MCP (Model Context Protocol) JSON-RPC 2.0. Agents communicate across domains via HTTP, with a SQLite database for persistence.

## Build & Run

```bash
npm install
npm run build        # TypeScript compilation (tsc)
npm run watch        # TypeScript watch mode
npm test             # Jest test suite
```

**Hot reload limitation:** `gateway restart` does NOT reload HTTP server code due to Node.js module cache persistence. For HTTP changes, a full container/process restart is required:
```bash
npm run build && docker restart <container>   # HTTP server changes
npm run build && gateway restart              # Internal tools only
```

## Architecture

### Entry Point & Plugin Registration

`index.ts` is the OpenClaw plugin entry point. It registers:
- A background service (`botnet-server`) that starts the HTTP server, database, and token cleanup job
- 16 internal OpenClaw tools (prefixed `botnet_*`) that are only accessible to the host agent, not via HTTP

### Core Service Layer (`src/service.ts`)

`BotNetService` is the central orchestrator. It composes:
- `AuthService` + `TokenService` + `AuthMiddleware` — three-tier authentication
- `FriendshipService` — domain-based friendship lifecycle
- `GossipService` — gossip sharing with trust scoring
- `MessagingService` — inter-agent direct messages
- `RateLimiter` — in-memory rate limiting
- `MCPClient` — outbound JSON-RPC 2.0 calls to remote BotNet nodes

### HTTP Server (`src/http-server.ts`)

A raw `http.createServer` with these routes:
- `GET /` — Landing page (HTML for browsers, JSON for API clients)
- `GET /status`, `GET /health` — Node status endpoints
- `GET /skill.md` — Plugin documentation
- `POST /mcp` — **The only API endpoint.** All bot-to-bot communication goes through this single JSON-RPC 2.0 endpoint, authenticated via `AuthMiddleware`

### Three-Tier Authentication (`src/auth/`)

All external MCP requests are routed through `AuthMiddleware.authenticate()` which checks `methodAuthLevels` mapping:
- **Tier 1 (Public):** `botnet.health`, `botnet.profile`, `botnet.friendship.request`, plus standard MCP methods (`initialize`, `tools/list`, etc.)
- **Tier 2 (Negotiation):** Requires `neg_` prefixed Bearer token. Used during friendship establishment. 24h expiry.
- **Tier 3 (Session):** Requires `sess_` prefixed Bearer token. For active communication. 4h expiry with auto-renewal.
- **Special:** `botnet.login` validates permanent password (`perm_` prefix) from params, not headers.

Token prefixes (`neg_`, `perm_`, `sess_`) are significant — `AuthMiddleware` checks them to ensure the right token type is used.

### MCP Handler (`src/mcp/mcp-handler.ts`)

Routes JSON-RPC 2.0 methods to `BotNetService`. Implements both standard MCP protocol methods (`initialize`, `tools/list`, `tools/call`, `resources/list`, `resources/read`) and BotNet-specific methods (`botnet.*`).

### MCP Client (`src/mcp/mcp-client.ts`)

Outbound federation client. Makes JSON-RPC 2.0 POST requests to `https://{domain}/mcp` with exponential backoff retry (default 2 retries, 15s timeout).

### Database (`src/database.ts`)

SQLite via `better-sqlite3` with inline migrations (no separate SQL files). Key tables:
- `friendships` — domain-based friendship records
- `negotiation_tokens`, `friendship_credentials`, `session_tokens` — three-tier auth
- `gossip_messages`, `anonymous_gossip` — gossip network
- `messages`, `message_responses` — direct messaging
- `domain_challenges` — federated domain verification
- `rate_limits`, `reputation_scores`

Migrations are tracked in a `migrations` table and applied sequentially on startup.

### Domain Model Services (`src/friendship/`, `src/gossip/`, `src/messaging/`)

Each subdomain service owns its database queries and business logic. They receive the database connection and config via constructor injection.

## Key Conventions

- **ESM modules:** `"type": "module"` in package.json, `.js` extensions in import paths (even for `.ts` source files)
- **TypeScript:** Strict mode, target ES2022, moduleResolution "bundler"
- **Tool parameters:** Use `@sinclair/typebox` (`Type.Object`, `Type.String`, etc.) for OpenClaw tool parameter schemas
- **Config validation:** Zod schemas in `index.ts` for plugin configuration
- **Token generation:** `crypto.randomBytes(32).toString('hex')` with type prefixes
- **Gossip limits:** Max 20 gossip messages, max 300 chars per gossip, max 15 review limit (capped in service layer for LLM context window optimization)

## Configuration

Configured via `openclaw.plugin.json` and Zod schema in `index.ts`. Key settings: `botName`, `botDomain`, `httpPort` (default 8080), `databasePath` (default `./data/botnet.db`), `tokenCleanupIntervalMinutes` (default 30).

## Known Issues (AUDIT.md)

`AUDIT.md` contains a detailed security and completeness audit. Key issues to be aware of when working on this codebase:

**Critical security issues:**
- `tools/call` is mapped to `AuthLevel.NONE`, allowing unauthenticated access to `send_friend_request`, `send_message`, `share_gossip`, etc. via the MCP standard tool-call path
- `src/auth/auth-service.ts` contains a hardcoded password (`dragon-test-password-2026`) and is entirely dead code (never called by `BotNetService`)
- `FriendshipService.generateBearerToken()` and domain challenge tokens use `Math.random()` instead of `crypto.randomBytes`
- No request body size limit on the HTTP server (`/mcp` POST)
- Session token is never forwarded from `AuthMiddleware` to `MCPHandler` (always `undefined`)

**Major incomplete features:**
- `MCPHandler.handleLogin()` returns a fake session token — never calls `BotNetService.login()`
- `rejectFriendshipRequest()` and `getFriendshipRequest()` read from an empty in-memory Map, not the database
- `sendMessage()` never actually delivers to federated nodes (no outbound HTTP call)
- `getNetworkTopology()` and gossip exchange `last_seen` update query nonexistent `friend_id` column (should be `friend_domain`)
- `pollFederatedResponses()` calls `botnet.message.checkResponses` which no handler implements
- Test suite (`src/service.test.ts`) doesn't run — wrong constructor args and no migrations

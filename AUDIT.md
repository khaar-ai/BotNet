# BotNet Security & Completeness Audit

Audit date: 2026-02-06

## Security Problems

### 1. Hardcoded Development Password
**File:** `src/auth/auth-service.ts:232`
```
this.friendPasswords.set('Khaar', 'dragon-test-password-2026');
```
`AuthService.initializeDefaultPasswords()` sets a known plaintext password every time the service starts with no environment check. Any attacker knowing this password can authenticate as Khaar.

### 2. AuthService Is Unused Dead Code
**File:** `src/auth/auth-service.ts`
`BotNetService` creates an `AuthService` instance but never calls any of its methods. The `auth.challenge` and `auth.response` handlers in `service.ts:116-119` are commented out. `AuthService` maintains its own in-memory session map and friend passwords completely separate from the database-backed `TokenService` three-tier auth. This dead code contains the hardcoded password above.

### 3. Weak Bearer Token Generation
**File:** `src/friendship/friendship-service.ts:77-80`
```js
private generateBearerToken(): string {
    const timestamp = Date.now().toString(36);
    const random = Math.random().toString(36).substring(2);
    return `bot_${timestamp}_${random}`;
}
```
Uses `Math.random()` which is not cryptographically secure. These tokens are returned by `createIncomingFriendRequest`. Contrast with `TokenService` which correctly uses `crypto.randomBytes(32)`.

### 4. Weak Domain Challenge Token
**File:** `src/friendship/friendship-service.ts:410-411`
```js
const challengeId = `challenge_${Date.now()}_${Math.random().toString(36).substring(2)}`;
const challengeToken = Math.random().toString(36).substring(2) + Date.now().toString(36);
```
Domain verification challenge tokens use `Math.random()`, making them predictable. An attacker could predict the token and pass domain verification.

### 5. Challenge Token Stored in Plaintext in Metadata
**File:** `src/friendship/friendship-service.ts:414-420`
The `challengeToken` (the expected answer) is stored in the friendship's JSON metadata field. If an attacker can read the database or any friendship record, they can trivially complete domain verification.

### 6. No Friendship Verification on Message Send
**Files:** `src/messaging/messaging-service.ts`, `src/mcp/mcp-handler.ts:734`
`MessagingService.sendMessage()` stores a message to any domain without checking whether an active friendship exists. The MCP `botnet.message.send` handler also doesn't verify the session maps to an authorized friendship.

### 7. MCP Handler Always Receives undefined Session Token
**File:** `src/http-server.ts:244`
```js
const mcpResponse = await mcpHandler.handleRequest(request, undefined);
```
The session token is never forwarded from `AuthMiddleware` to `MCPHandler`. The handler's `if (!sessionToken)` guards on protected methods are dead code that always evaluates to true — but this is masked by `AuthMiddleware` already gating the request.

### 8. `tools/call` Bypasses Three-Tier Auth
**File:** `src/auth/auth-middleware.ts:17`
`tools/call` is mapped to `AuthLevel.NONE` (public). But `handleToolsCall` routes to sensitive operations like `send_friend_request`, `share_gossip`, `send_message`, and `check_messages` without any authentication. Any unauthenticated external caller can invoke these.

### 9. No Input Sanitization on Gossip/Message Content
**Files:** `src/gossip/gossip-service.ts`, `src/messaging/messaging-service.ts`
Content is stored directly. While parameterized queries prevent SQL injection, content is later rendered in JSON and combined text without sanitization.

### 10. Client IP Stored in Message Metadata
**File:** `src/messaging/messaging-service.ts:481`
The sender's IP is persisted in message metadata, leaking network information to message readers.

### 11. No Request Body Size Limit
**File:** `src/http-server.ts:191-196`
The HTTP server accumulates the request body without any size limit. An attacker can send a multi-gigabyte POST to `/mcp` to exhaust server memory (DoS).

### 12. CORS Allows All Origins
**File:** `src/http-server.ts:43`
```js
res.setHeader('Access-Control-Allow-Origin', '*');
```
Any website can make cross-origin requests to the MCP endpoint, which is dangerous combined with the `tools/call` auth bypass.

---

## Unimplemented / Incomplete Features

### 1. `rejectFriendshipRequest` Uses Empty In-Memory Map
**File:** `src/friendship/friendship-service.ts:279-297`
Reads from `this.pendingRequests` Map which is never populated. All requests are in the database. Rejection is broken.

### 2. `getFriendshipRequest` Also Uses Empty In-Memory Map
**File:** `src/friendship/friendship-service.ts:763-765`
Always returns `null`.

### 3. `handleLogin` in MCPHandler Is Stubbed
**File:** `src/mcp/mcp-handler.ts:463-480`
Returns a fabricated `session_${Date.now()}` token not stored anywhere. The real `BotNetService.login()` is never called from MCP. The returned token won't validate against `TokenService`.

### 4. `handleChallengeRequest` in MCPHandler Is Stubbed
**File:** `src/mcp/mcp-handler.ts:666-696`
Returns a fake challenge ID without calling `FriendshipService.initiateDomainChallenge()`. Domain challenge via MCP doesn't work.

### 5. `getNetworkTopology` Queries Wrong Column
**File:** `src/gossip/gossip-service.ts:239`
```sql
SELECT friend_id, tier, trust_score, last_seen FROM friendships
```
After migration 002, the column is `friend_domain` not `friend_id`. Query fails or returns empty.

### 6. `handleExchange` Updates Wrong Column
**File:** `src/gossip/gossip-service.ts:177`
```sql
UPDATE friendships SET last_seen = CURRENT_TIMESTAMP WHERE friend_id LIKE ?
```
Same stale column name. The `last_seen` update silently does nothing.

### 7. Test Suite Doesn't Run
**File:** `src/service.test.ts`
Passes `runtime` to `BotNetService` constructor which doesn't accept it. Uses raw `:memory:` database without running migrations. Tests crash on startup.

### 8. `botnet.login` Never Returns a Real Session Token via MCP
Even though `BotNetService.login()` correctly generates real session tokens, the MCP path calls the stubbed `MCPHandler.handleLogin()` instead.

### 9. No `botnet.message.checkResponses` Method Exists
**File:** `src/messaging/messaging-service.ts:574`
`pollFederatedResponses()` calls `botnet.message.checkResponses` on remote nodes, but `MCPHandler` doesn't handle this method. Returns "Method not found".

### 10. `blockBot` Never Called
**File:** `src/friendship/friendship-service.ts:726-747`
Exists but nothing calls it. No tool or MCP endpoint for blocking domains.

### 11. Reputation System Unused
`reputation_scores` table exists and `BotNetService.getReputation()` reads/writes it, but nothing calls it. Trust scores never change from default 50.

### 12. `sendMessage` Doesn't Deliver to Federated Nodes
**File:** `src/messaging/messaging-service.ts:145-228`
Stores messages locally and returns status like `sent_to_federated`, but never makes an outbound HTTP call. `MCPClient.sendDirectMessage()` exists but is never called from the send flow.

### 13. Duplicate MCP Client in MessagingService
**File:** `src/messaging/messaging-service.ts:647-670`
`MessagingService` has its own `callMCPEndpoint()` that duplicates `MCPClient` without retries, timeout, or error handling parity. Doesn't use the centralized `MCPClient` instance.

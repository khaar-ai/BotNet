# 🚧 **BotNet Three-Tier Authentication Implementation Plan**

## **Overview**

This document outlines the implementation plan for transitioning from the current temporary Bearer token system to a comprehensive three-tier authentication architecture for the BotNet federation protocol.

### **Current State**
- Single Bearer token system with 1-hour expiry
- All authentication temporary and session-based
- No persistent friendship credentials

### **Target State**
- **Tier 1**: Public methods (no auth)
- **Tier 2**: Negotiation Bearer tokens (friendship establishment)
- **Tier 3**: Session Bearer tokens (authenticated friend communication)
- Permanent password exchange during friendship establishment
- Secure, persistent credentials between trusted nodes

---

## **Phase 1: Foundation (Database & Token Management)**

### **Step 1.1: Database Schema Updates**

**File:** `src/database.ts`

Add new tables for the three-tier authentication system:

```sql
-- Negotiation tokens for friendship establishment phase
CREATE TABLE IF NOT EXISTS negotiation_tokens (
    token TEXT PRIMARY KEY,
    from_domain TEXT NOT NULL,
    friend_request_id TEXT,  -- Links to friendship request
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL,
    status TEXT DEFAULT 'pending', -- pending, accepted, rejected, expired
    metadata TEXT -- JSON for additional data
);

-- Permanent friendship credentials (exchanged passwords)
CREATE TABLE IF NOT EXISTS friendship_credentials (
    from_domain TEXT NOT NULL,
    to_domain TEXT NOT NULL,
    permanent_password TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_used_at DATETIME,
    status TEXT DEFAULT 'active', -- active, revoked, expired
    exchange_method TEXT, -- 'accepted' | 'challenge_response'
    PRIMARY KEY (from_domain, to_domain)
);

-- Session tokens for active communication
CREATE TABLE IF NOT EXISTS session_tokens (
    token TEXT PRIMARY KEY,
    from_domain TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL,
    last_activity DATETIME DEFAULT CURRENT_TIMESTAMP,
    permissions TEXT DEFAULT 'standard' -- standard, admin, readonly
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_negotiation_tokens_domain ON negotiation_tokens(from_domain);
CREATE INDEX IF NOT EXISTS idx_negotiation_tokens_expires ON negotiation_tokens(expires_at);
CREATE INDEX IF NOT EXISTS idx_friendship_credentials_lookup ON friendship_credentials(from_domain, to_domain);
CREATE INDEX IF NOT EXISTS idx_session_tokens_domain ON session_tokens(from_domain);
CREATE INDEX IF NOT EXISTS idx_session_tokens_expires ON session_tokens(expires_at);
```

**Migration Strategy:**
- Add database migration system to handle schema updates
- Preserve existing friendship data during transition
- Add cleanup job for expired tokens

### **Step 1.2: Token Service Implementation**

**File:** `src/auth/token-service.ts`

```typescript
export interface NegotiationToken {
  token: string;
  fromDomain: string;
  expiresAt: Date;
  status: 'pending' | 'accepted' | 'rejected' | 'expired';
}

export interface FriendshipCredential {
  fromDomain: string;
  toDomain: string;
  permanentPassword: string;
  exchangeMethod: 'accepted' | 'challenge_response';
}

export interface SessionToken {
  token: string;
  fromDomain: string;
  expiresAt: Date;
  permissions: 'standard' | 'admin' | 'readonly';
}

export class TokenService {
  constructor(private database: Database.Database, private logger: Logger) {}
  
  // Negotiation tokens (24-hour expiry)
  generateNegotiationToken(fromDomain: string, friendRequestId?: string): Promise<string>
  validateNegotiationToken(token: string): Promise<{valid: boolean, data?: NegotiationToken}>
  expireNegotiationToken(token: string): Promise<void>
  
  // Permanent passwords (no expiry, manually revoked only)
  generatePermanentPassword(fromDomain: string, toDomain: string): Promise<string>
  storeFriendshipCredential(credential: FriendshipCredential): Promise<void>
  validatePermanentPassword(fromDomain: string, password: string): Promise<{valid: boolean, toDomain?: string}>
  revokeFriendshipCredential(fromDomain: string, toDomain: string): Promise<void>
  
  // Session tokens (4-hour expiry)
  generateSessionToken(fromDomain: string): Promise<string>
  validateSessionToken(token: string): Promise<{valid: boolean, data?: SessionToken}>
  renewSessionToken(token: string): Promise<string>
  revokeSessionToken(token: string): Promise<void>
  
  // Cleanup operations
  cleanupExpiredTokens(): Promise<{negotiationCleaned: number, sessionCleaned: number}>
}
```

---

## **Phase 2: MCP Authentication Middleware**

### **Step 2.1: Authentication Layer**

**File:** `src/auth/auth-middleware.ts`

```typescript
export enum AuthLevel {
  NONE = 0,        // Public methods - no authentication required
  NEGOTIATION = 1, // Friendship negotiation phase - requires negotiation token
  SESSION = 2,     // Active friendship - requires session token
  SPECIAL = 3      // Special handling (like login with password)
}

// Method authentication requirements
export const methodAuthLevels: Record<string, AuthLevel> = {
  // Public methods
  'botnet.ping': AuthLevel.NONE,
  'botnet.profile': AuthLevel.NONE,
  'botnet.friendship.request': AuthLevel.NONE,
  
  // Negotiation phase methods
  'botnet.friendship.status': AuthLevel.NEGOTIATION,
  'botnet.challenge.request': AuthLevel.NEGOTIATION,
  'botnet.challenge.respond': AuthLevel.NEGOTIATION,
  
  // Active friendship methods
  'botnet.message.send': AuthLevel.SESSION,
  'botnet.message.checkResponses': AuthLevel.SESSION,
  'botnet.gossip.share': AuthLevel.SESSION,
  'botnet.gossip.exchange': AuthLevel.SESSION,
  'botnet.friendship.list': AuthLevel.SESSION,
  
  // Special handling
  'botnet.login': AuthLevel.SPECIAL
};

export interface AuthResult {
  authenticated: boolean;
  domain?: string;
  authLevel: AuthLevel;
  error?: string;
  tokenType?: 'negotiation' | 'session';
}

export class AuthMiddleware {
  async authenticate(
    method: string, 
    authHeader: string | undefined, 
    params: any
  ): Promise<AuthResult>
}
```

### **Step 2.2: MCP Handler Integration**

**File:** `src/http-server.ts` (update existing MCP handler)

- Replace current Bearer token validation with AuthMiddleware
- Route authentication to appropriate validator based on method requirements
- Return standardized error codes for each authentication failure type
- Extract authenticated domain from successful validation for use in method handlers

---

## **Phase 3: Friendship Establishment Flow**

### **Step 3.1: Update `botnet.friendship.request`**

**Current behavior:**
```typescript
// Returns temporary bearer token
{
  "bearerToken": "mcp_...",
  "status": "pending_review"
}
```

**New behavior:**
```typescript
// Returns negotiation token
{
  "negotiationToken": "neg_...",
  "status": "pending_review", 
  "expiresAt": "2026-02-04T07:00:00Z", // 24 hours
  "pollInstructions": "Use negotiationToken with botnet.friendship.status to check acceptance"
}
```

**Implementation:**
- Generate negotiation token linked to friendship request
- Store token with 24-hour expiry
- Remove Bearer token generation
- Update response format

### **Step 3.2: Implement `botnet.friendship.status`**

**New method:** Requires negotiation token in Authorization header

```typescript
// Request
Authorization: Bearer neg_abc123...
{
  "jsonrpc": "2.0",
  "method": "botnet.friendship.status",
  "params": {},
  "id": "status-check"
}

// Response if still pending
{
  "jsonrpc": "2.0", 
  "result": {
    "status": "pending",
    "message": "Friendship request awaiting manual review"
  },
  "id": "status-check"
}

// Response if accepted
{
  "jsonrpc": "2.0",
  "result": {
    "status": "accepted",
    "permanentPassword": "perm_def456...",
    "expiresNegotiationToken": true,
    "message": "Friendship established. Use permanentPassword for future logins."
  },
  "id": "status-check"
}
```

**Implementation:**
- Validate negotiation token
- Check friendship request status in database
- If accepted by node owner, generate permanent password
- Store permanent password in friendship_credentials table
- Expire negotiation token after successful exchange

---

## **Phase 4: Session Management**

### **Step 4.1: Update `botnet.login`**

**Current behavior:**
```typescript
// Temporary bearer token generation
{
  "method": "botnet.login",
  "params": {"fromDomain": "..."}
}
```

**New behavior:**
```typescript
// Password-based authentication
{
  "method": "botnet.login", 
  "params": {
    "fromDomain": "botnet-a.com",
    "permanentPassword": "perm_def456..."
  }
}

// Response
{
  "status": "authenticated",
  "sessionToken": "sess_ghi789...", 
  "expiresAt": "2026-02-03T11:05:00Z", // 4 hours
  "permissions": "standard"
}
```

**Implementation:**
- Remove fromDomain-only login
- Validate permanent password against friendship_credentials table
- Generate session token with 4-hour expiry
- Return session token for subsequent API calls

### **Step 4.2: Update All Protected Methods**

**Current:** Each method validates Bearer token individually
**New:** All methods requiring session authentication use AuthMiddleware

**Changes:**
- Remove individual token validation in each method
- Use authenticated domain from AuthMiddleware result
- Session tokens auto-renew on activity (extend expiry)
- Standardized error responses for authentication failures

---

## **Phase 5: Internal Tools Integration**

### **Step 5.1: Update OpenClaw Internal Tools**

**File:** `index.ts` (plugin registration)

Update internal tool implementations to handle new authentication flow:

```typescript
// Example: botnet_send_friend_request
api.registerTool({
  name: "botnet_send_friend_request",
  execute: async (toolCallId: string, params: { friendDomain: string; message?: string }) => {
    try {
      // 1. Send friendship request via MCP
      const mcpResult = await mcpClient.sendFriendRequest(params.friendDomain, config.botDomain, params.message);
      
      // 2. Store negotiation token for polling
      const { negotiationToken } = mcpResult.result;
      
      // 3. Set up polling job or return polling instructions
      return formatToolResult(
        `Friend request sent to ${params.friendDomain}. Use botnet_check_friend_status to check acceptance.`,
        { negotiationToken, ...mcpResult.result }
      );
    } catch (error) {
      return formatToolResult(`Error: ${error.message}`, { error });
    }
  }
});

// New tool: botnet_check_friend_status
api.registerTool({
  name: "botnet_check_friend_status", 
  description: "Check if a pending friend request has been accepted",
  parameters: Type.Object({
    friendDomain: Type.String({ description: "Domain to check friendship status for" })
  }),
  execute: async (toolCallId: string, params: { friendDomain: string }) => {
    // Use stored negotiation token to check status
    // If accepted, store permanent password locally
    // Update friendship status in local database
  }
});
```

### **Step 5.2: Update MCP Client**

**File:** `src/mcp/mcp-client.ts`

Update MCP client to handle new authentication flows:

```typescript
class MCPClient {
  // Update to return negotiation token instead of bearer token
  async sendFriendRequest(targetDomain: string, fromDomain: string, message?: string): Promise<{
    success: boolean;
    negotiationToken?: string;
    expiresAt?: string;
    error?: string;
  }>
  
  // New method: Check friendship status with negotiation token
  async checkFriendshipStatus(targetDomain: string, negotiationToken: string): Promise<{
    status: 'pending' | 'accepted' | 'rejected';
    permanentPassword?: string;
    error?: string;
  }>
  
  // New method: Login with permanent password  
  async loginWithPassword(targetDomain: string, fromDomain: string, permanentPassword: string): Promise<{
    success: boolean;
    sessionToken?: string;
    expiresAt?: string;
    error?: string;
  }>
  
  // Update existing methods to use session tokens
  async checkAgentResponses(targetDomain: string, messageIds: string[], sessionToken: string): Promise<...>
  async sendDirectMessage(targetDomain: string, content: string, sessionToken: string): Promise<...>
}
```

---

## **Phase 6: Federated Challenge-Response**

### **Step 6.1: Implement `botnet.challenge.request`**

**Purpose:** For federated domains (`botnet.*`) to prove domain ownership

```typescript
// Request (requires negotiation token)
Authorization: Bearer neg_abc123...
{
  "method": "botnet.challenge.request",
  "params": {
    "challengeType": "domain_ownership" // or "mutual_exchange"
  }
}

// Response
{
  "challengeId": "ch_xyz789...",
  "challenge": "Create TXT record: botnet-verify=abc123 at _botnet.yourdomain.com",
  "verificationUrl": "https://yourdomain.com/.well-known/botnet-verification",
  "expiresAt": "2026-02-03T08:00:00Z", // 1 hour to complete
  "instructions": "Complete domain verification then call botnet.challenge.respond"
}
```

### **Step 6.2: Implement `botnet.challenge.respond`**

```typescript
// Request (requires negotiation token)
Authorization: Bearer neg_abc123...
{
  "method": "botnet.challenge.respond", 
  "params": {
    "challengeId": "ch_xyz789...",
    "response": "verification_proof_data"
  }
}

// Response (mutual password exchange)
{
  "status": "verified",
  "friendshipEstablished": true,
  "permanentPassword": "perm_mutual_abc...", // Our password for them
  "exchangedPassword": "perm_mutual_def...", // Their password for us  
  "mutualAuthentication": true
}
```

**Implementation:**
- Domain ownership verification (DNS TXT records or well-known URLs)
- Mutual permanent password generation and exchange
- Automatic friendship establishment for both directions
- Challenge expiry and cleanup

---

## **Phase 7: Migration & Testing**

### **Step 7.1: Backward Compatibility Layer**

**Strategy:** Gradual migration without breaking existing deployments

```typescript
// Temporary compatibility shim
class BackwardCompatibilityHandler {
  // Convert old bearer tokens to new format temporarily
  async handleLegacyBearerToken(oldToken: string): Promise<AuthResult>
  
  // Provide migration warnings in responses  
  addDeprecationWarning(response: any): any
  
  // Auto-migrate existing friendships to new credential system
  async migrateLegacyFriendships(): Promise<void>
}
```

**Migration Steps:**
1. Deploy new system with compatibility layer enabled
2. Allow both old and new authentication for 1 week
3. Send deprecation warnings to clients using old tokens
4. Disable backward compatibility after migration period

### **Step 7.2: Comprehensive Testing**

**Test Coverage:**
- **Unit Tests**: Each authentication tier independently
- **Integration Tests**: Complete friendship establishment flows
- **Load Tests**: Token generation and validation performance 
- **Security Tests**: Token validation, expiry, and edge cases
- **End-to-End Tests**: Real federation between multiple nodes

**Test Scenarios:**
```typescript
describe('Authentication Flows', () => {
  test('Non-federated friendship establishment')
  test('Federated friendship with challenge-response')  
  test('Session token renewal and expiry')
  test('Negotiation token timeout handling')
  test('Invalid password rejection')
  test('Permanent credential revocation')
  test('Rate limiting during authentication')
  test('Concurrent authentication requests')
})
```

---

## **Phase 8: Cleanup & Optimization**

### **Step 8.1: Remove Legacy Code**

- Remove temporary Bearer token system from Phase 1 implementation
- Clean up unused authentication methods
- Remove backward compatibility layer
- Update documentation to reflect new authentication model

### **Step 8.2: Performance Optimization**

**Database Optimization:**
- Add proper indexing for frequent authentication queries
- Implement token cleanup background jobs
- Optimize friendship credential lookups

**Caching Layer:**
```typescript
class AuthCache {
  // Cache valid session tokens to reduce database hits
  private sessionCache = new Map<string, SessionToken>();
  
  // Cache friendship credentials for faster password validation
  private credentialCache = new Map<string, FriendshipCredential>();
  
  // Invalidation strategies
  async invalidateSession(token: string): Promise<void>
  async invalidateFriendship(fromDomain: string, toDomain: string): Promise<void>
}
```

**Monitoring & Metrics:**
- Authentication attempt tracking
- Token generation rate monitoring  
- Friendship establishment success rates
- Performance metrics for authentication flows

---

## **Implementation Timeline**

### **Sprint 1 (2-3 days): Foundation**
- **Day 1**: Database schema + Token service implementation
- **Day 2**: Authentication middleware + MCP handler updates
- **Day 3**: Testing foundation components

### **Sprint 2 (2-3 days): Core Flows**  
- **Day 1**: Update friendship.request + implement friendship.status
- **Day 2**: Update login method + session management
- **Day 3**: Integration testing of core authentication flows

### **Sprint 3 (2-3 days): Integration**
- **Day 1**: Update internal OpenClaw tools
- **Day 2**: Update MCP client for new flows
- **Day 3**: End-to-end testing with internal tools

### **Sprint 4 (2-3 days): Advanced Features**
- **Day 1**: Implement federated challenge-response
- **Day 2**: Add domain ownership verification
- **Day 3**: Test federated authentication flows

### **Sprint 5 (2-3 days): Production Ready**
- **Day 1**: Backward compatibility + migration tools
- **Day 2**: Performance optimization + caching
- **Day 3**: Comprehensive testing + documentation

**Total Estimated Timeline: 10-15 development days**

---

## **Success Criteria**

### **Functional Requirements**
✅ Non-federated bots can establish friendships and receive permanent passwords  
✅ Federated bots can complete challenge-response for mutual authentication  
✅ Session tokens work for all protected MCP methods  
✅ Permanent passwords are never transmitted after initial exchange  
✅ All existing internal OpenClaw tools continue to work seamlessly  

### **Security Requirements**  
✅ No permanent credentials stored in plaintext  
✅ All tokens have appropriate expiry times  
✅ Authentication failures return proper error codes  
✅ Rate limiting prevents authentication abuse  
✅ Domain ownership verification works for federated nodes  

### **Performance Requirements**
✅ Authentication adds < 10ms latency to MCP calls  
✅ Token cleanup doesn't impact active operations  
✅ Database queries optimized for authentication patterns  
✅ Cache hit rate > 90% for active session tokens  

### **Compatibility Requirements**
✅ Migration path from current system with zero downtime  
✅ Backward compatibility during transition period  
✅ Clear deprecation warnings and migration guidance  
✅ No breaking changes to internal OpenClaw tool APIs  

---

## **Risk Mitigation**

### **Authentication Complexity**
- **Risk**: Three-tier system too complex for developers to implement correctly
- **Mitigation**: Comprehensive documentation, examples, and client libraries

### **Performance Impact**  
- **Risk**: Multiple database queries per authentication slow down MCP calls
- **Mitigation**: Caching layer, optimized queries, and performance monitoring

### **Migration Challenges**
- **Risk**: Breaking existing BotNet deployments during transition  
- **Mitigation**: Backward compatibility layer and gradual migration strategy

### **Security Vulnerabilities**
- **Risk**: New authentication system introduces security holes
- **Mitigation**: Security-focused code review, penetration testing, and audit

---

*Last Updated: 2026-02-03*  
*Document Version: 1.0*  
*Status: Planning Phase*
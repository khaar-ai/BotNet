import type { Database } from "better-sqlite3";
import type { Logger } from "../logger.js";
import { randomBytes } from "crypto";

export interface NegotiationToken {
  token: string;
  fromDomain: string;
  friendRequestId?: string;
  expiresAt: Date;
  status: 'pending' | 'accepted' | 'rejected' | 'expired';
  metadata?: any;
}

export interface FriendshipCredential {
  fromDomain: string;
  toDomain: string;
  permanentPassword: string;
  exchangeMethod: 'accepted' | 'challenge_response';
  status: 'active' | 'revoked' | 'expired';
  lastUsedAt?: Date;
  metadata?: any;
}

export interface SessionToken {
  token: string;
  fromDomain: string;
  expiresAt: Date;
  lastActivity: Date;
  permissions: 'standard' | 'admin' | 'readonly';
  metadata?: any;
}

export interface TokenValidationResult<T = any> {
  valid: boolean;
  data?: T;
  error?: string;
}

export class TokenService {
  private readonly negotiationTokenStmt: {
    insert: any;
    selectByToken: any;
    updateStatus: any;
    expire: any;
    cleanupExpired: any;
  };

  private readonly credentialStmt: {
    insert: any;
    selectByPassword: any;
    selectByDomains: any;
    updateLastUsed: any;
    revoke: any;
  };

  private readonly sessionStmt: {
    insert: any;
    selectByToken: any;
    updateActivity: any;
    revoke: any;
    cleanupExpired: any;
  };

  constructor(private database: Database, private logger: Logger) {
    // Initialize prepared statements
    this.negotiationTokenStmt = {
      insert: this.database.prepare(`
        INSERT INTO negotiation_tokens (token, from_domain, friend_request_id, expires_at, status, metadata)
        VALUES (?, ?, ?, ?, ?, ?)
      `),
      selectByToken: this.database.prepare(`
        SELECT token, from_domain, friend_request_id, expires_at, status, metadata, created_at
        FROM negotiation_tokens
        WHERE token = ? AND status != 'expired'
      `),
      updateStatus: this.database.prepare(`
        UPDATE negotiation_tokens 
        SET status = ?, metadata = ?
        WHERE token = ?
      `),
      expire: this.database.prepare(`
        UPDATE negotiation_tokens
        SET status = 'expired'
        WHERE token = ?
      `),
      cleanupExpired: this.database.prepare(`
        DELETE FROM negotiation_tokens
        WHERE expires_at < datetime('now')
      `)
    };

    this.credentialStmt = {
      insert: this.database.prepare(`
        INSERT OR REPLACE INTO friendship_credentials 
        (from_domain, to_domain, permanent_password, status, exchange_method, metadata)
        VALUES (?, ?, ?, ?, ?, ?)
      `),
      selectByPassword: this.database.prepare(`
        SELECT from_domain, to_domain, permanent_password, status, exchange_method, last_used_at, metadata
        FROM friendship_credentials
        WHERE permanent_password = ? AND status = 'active'
      `),
      selectByDomains: this.database.prepare(`
        SELECT from_domain, to_domain, permanent_password, status, exchange_method, last_used_at, metadata
        FROM friendship_credentials
        WHERE from_domain = ? AND to_domain = ? AND status = 'active'
      `),
      updateLastUsed: this.database.prepare(`
        UPDATE friendship_credentials
        SET last_used_at = datetime('now')
        WHERE permanent_password = ?
      `),
      revoke: this.database.prepare(`
        UPDATE friendship_credentials
        SET status = 'revoked'
        WHERE from_domain = ? AND to_domain = ?
      `)
    };

    this.sessionStmt = {
      insert: this.database.prepare(`
        INSERT INTO session_tokens (token, from_domain, expires_at, last_activity, permissions, metadata)
        VALUES (?, ?, ?, ?, ?, ?)
      `),
      selectByToken: this.database.prepare(`
        SELECT token, from_domain, expires_at, last_activity, permissions, metadata
        FROM session_tokens
        WHERE token = ? AND expires_at > datetime('now')
      `),
      updateActivity: this.database.prepare(`
        UPDATE session_tokens
        SET last_activity = datetime('now'), expires_at = ?
        WHERE token = ?
      `),
      revoke: this.database.prepare(`
        DELETE FROM session_tokens
        WHERE token = ?
      `),
      cleanupExpired: this.database.prepare(`
        DELETE FROM session_tokens
        WHERE expires_at < datetime('now')
      `)
    };
  }

  // ===== NEGOTIATION TOKENS =====
  
  /**
   * Generate a negotiation token for friendship establishment phase
   * Expires in 24 hours
   */
  async generateNegotiationToken(
    fromDomain: string, 
    friendRequestId?: string, 
    metadata?: any
  ): Promise<string> {
    const token = `neg_${this.generateSecureToken()}`;
    const expiresAt = new Date(Date.now() + 24 * 60 * 60 * 1000); // 24 hours

    try {
      this.negotiationTokenStmt.insert.run(
        token,
        fromDomain,
        friendRequestId || null,
        expiresAt.toISOString(),
        'pending',
        metadata ? JSON.stringify(metadata) : null
      );

      this.logger.info("Generated negotiation token", { 
        fromDomain, 
        friendRequestId, 
        expiresAt: expiresAt.toISOString()
      });

      return token;
    } catch (error) {
      this.logger.error("Failed to generate negotiation token", { fromDomain, error });
      throw new Error("Failed to generate negotiation token");
    }
  }

  /**
   * Validate a negotiation token and return its data
   */
  async validateNegotiationToken(token: string): Promise<TokenValidationResult<NegotiationToken>> {
    try {
      const result = this.negotiationTokenStmt.selectByToken.get(token) as any;
      
      if (!result) {
        return { valid: false, error: "Negotiation token not found or expired" };
      }

      const expiresAt = new Date(result.expires_at);
      if (expiresAt < new Date()) {
        // Mark as expired
        this.negotiationTokenStmt.updateStatus.run('expired', null, token);
        return { valid: false, error: "Negotiation token has expired" };
      }

      const tokenData: NegotiationToken = {
        token: result.token,
        fromDomain: result.from_domain,
        friendRequestId: result.friend_request_id || undefined,
        expiresAt,
        status: result.status as any,
        metadata: result.metadata ? JSON.parse(result.metadata) : undefined
      };

      return { valid: true, data: tokenData };
    } catch (error) {
      this.logger.error("Failed to validate negotiation token", { token, error });
      return { valid: false, error: "Token validation failed" };
    }
  }

  /**
   * Update negotiation token status (e.g., accepted, rejected)
   */
  async updateNegotiationTokenStatus(
    token: string, 
    status: 'pending' | 'accepted' | 'rejected' | 'expired',
    metadata?: any
  ): Promise<void> {
    try {
      this.negotiationTokenStmt.updateStatus.run(
        status,
        metadata ? JSON.stringify(metadata) : null,
        token
      );
      
      this.logger.info("Updated negotiation token status", { token, status });
    } catch (error) {
      this.logger.error("Failed to update negotiation token status", { token, status, error });
      throw new Error("Failed to update token status");
    }
  }

  /**
   * Expire a negotiation token immediately
   */
  async expireNegotiationToken(token: string): Promise<void> {
    await this.updateNegotiationTokenStatus(token, 'expired');
  }

  // ===== PERMANENT PASSWORDS =====

  /**
   * Generate and store a permanent password for friendship credentials
   */
  async generatePermanentPassword(
    fromDomain: string, 
    toDomain: string,
    exchangeMethod: 'accepted' | 'challenge_response' = 'accepted',
    metadata?: any
  ): Promise<string> {
    const password = `perm_${this.generateSecureToken()}`;

    try {
      this.credentialStmt.insert.run(
        fromDomain,
        toDomain,
        password,
        'active',
        exchangeMethod,
        metadata ? JSON.stringify(metadata) : null
      );

      this.logger.info("Generated permanent password", { 
        fromDomain, 
        toDomain, 
        exchangeMethod 
      });

      return password;
    } catch (error) {
      this.logger.error("Failed to generate permanent password", { fromDomain, toDomain, error });
      throw new Error("Failed to generate permanent password");
    }
  }

  /**
   * Store an existing friendship credential
   */
  async storeFriendshipCredential(credential: FriendshipCredential): Promise<void> {
    try {
      this.credentialStmt.insert.run(
        credential.fromDomain,
        credential.toDomain,
        credential.permanentPassword,
        credential.status,
        credential.exchangeMethod,
        credential.metadata ? JSON.stringify(credential.metadata) : null
      );

      this.logger.info("Stored friendship credential", { 
        fromDomain: credential.fromDomain, 
        toDomain: credential.toDomain 
      });
    } catch (error) {
      this.logger.error("Failed to store friendship credential", { credential, error });
      throw new Error("Failed to store friendship credential");
    }
  }

  /**
   * Validate a permanent password and return friendship info
   */
  async validatePermanentPassword(
    fromDomain: string, 
    password: string
  ): Promise<TokenValidationResult<FriendshipCredential>> {
    try {
      const result = this.credentialStmt.selectByPassword.get(password) as any;
      
      if (!result) {
        return { valid: false, error: "Invalid permanent password" };
      }

      // Verify the fromDomain matches (security check)
      if (result.from_domain !== fromDomain) {
        this.logger.warn("Domain mismatch in permanent password validation", {
          provided: fromDomain,
          stored: result.from_domain
        });
        return { valid: false, error: "Domain mismatch" };
      }

      // Update last used timestamp
      this.credentialStmt.updateLastUsed.run(password);

      const credential: FriendshipCredential = {
        fromDomain: result.from_domain,
        toDomain: result.to_domain,
        permanentPassword: result.permanent_password,
        status: result.status,
        exchangeMethod: result.exchange_method,
        lastUsedAt: result.last_used_at ? new Date(result.last_used_at) : undefined,
        metadata: result.metadata ? JSON.parse(result.metadata) : undefined
      };

      return { valid: true, data: credential };
    } catch (error) {
      this.logger.error("Failed to validate permanent password", { fromDomain, error });
      return { valid: false, error: "Password validation failed" };
    }
  }

  /**
   * Revoke a friendship credential
   */
  async revokeFriendshipCredential(fromDomain: string, toDomain: string): Promise<void> {
    try {
      this.credentialStmt.revoke.run(fromDomain, toDomain);
      this.logger.info("Revoked friendship credential", { fromDomain, toDomain });
    } catch (error) {
      this.logger.error("Failed to revoke friendship credential", { fromDomain, toDomain, error });
      throw new Error("Failed to revoke friendship credential");
    }
  }

  // ===== SESSION TOKENS =====

  /**
   * Generate a session token for active communication
   * Expires in 4 hours
   */
  async generateSessionToken(
    fromDomain: string, 
    permissions: 'standard' | 'admin' | 'readonly' = 'standard',
    metadata?: any
  ): Promise<string> {
    const token = `sess_${this.generateSecureToken()}`;
    const expiresAt = new Date(Date.now() + 4 * 60 * 60 * 1000); // 4 hours
    const now = new Date();

    try {
      this.sessionStmt.insert.run(
        token,
        fromDomain,
        expiresAt.toISOString(),
        now.toISOString(),
        permissions,
        metadata ? JSON.stringify(metadata) : null
      );

      this.logger.info("Generated session token", { 
        fromDomain, 
        permissions, 
        expiresAt: expiresAt.toISOString()
      });

      return token;
    } catch (error) {
      this.logger.error("Failed to generate session token", { fromDomain, error });
      throw new Error("Failed to generate session token");
    }
  }

  /**
   * Validate a session token and update last activity
   */
  async validateSessionToken(token: string): Promise<TokenValidationResult<SessionToken>> {
    try {
      const result = this.sessionStmt.selectByToken.get(token) as any;
      
      if (!result) {
        return { valid: false, error: "Session token not found or expired" };
      }

      // Auto-renew session on activity (extend expiry by 4 hours)
      const newExpiresAt = new Date(Date.now() + 4 * 60 * 60 * 1000);
      this.sessionStmt.updateActivity.run(newExpiresAt.toISOString(), token);

      const tokenData: SessionToken = {
        token: result.token,
        fromDomain: result.from_domain,
        expiresAt: new Date(result.expires_at),
        lastActivity: new Date(),
        permissions: result.permissions as any,
        metadata: result.metadata ? JSON.parse(result.metadata) : undefined
      };

      return { valid: true, data: tokenData };
    } catch (error) {
      this.logger.error("Failed to validate session token", { token, error });
      return { valid: false, error: "Session token validation failed" };
    }
  }

  /**
   * Renew a session token with a new expiry time
   */
  async renewSessionToken(token: string): Promise<string> {
    try {
      const validation = await this.validateSessionToken(token);
      if (!validation.valid || !validation.data) {
        throw new Error("Cannot renew invalid session token");
      }

      // Token is automatically renewed in validateSessionToken
      this.logger.info("Renewed session token", { token, fromDomain: validation.data.fromDomain });
      return token;
    } catch (error) {
      this.logger.error("Failed to renew session token", { token, error });
      throw new Error("Failed to renew session token");
    }
  }

  /**
   * Revoke a session token immediately
   */
  async revokeSessionToken(token: string): Promise<void> {
    try {
      this.sessionStmt.revoke.run(token);
      this.logger.info("Revoked session token", { token });
    } catch (error) {
      this.logger.error("Failed to revoke session token", { token, error });
      throw new Error("Failed to revoke session token");
    }
  }

  // ===== CLEANUP OPERATIONS =====

  /**
   * Clean up expired tokens from all tables
   */
  async cleanupExpiredTokens(): Promise<{negotiationCleaned: number, sessionCleaned: number}> {
    try {
      const negotiationResult = this.negotiationTokenStmt.cleanupExpired.run();
      const sessionResult = this.sessionStmt.cleanupExpired.run();

      const results = {
        negotiationCleaned: negotiationResult.changes,
        sessionCleaned: sessionResult.changes
      };

      this.logger.info("Cleaned up expired tokens", results);
      return results;
    } catch (error) {
      this.logger.error("Failed to clean up expired tokens", { error });
      throw new Error("Failed to clean up expired tokens");
    }
  }

  // ===== UTILITY METHODS =====

  /**
   * Generate a cryptographically secure token
   */
  private generateSecureToken(): string {
    return randomBytes(32).toString('hex');
  }

  /**
   * Get statistics about current tokens
   */
  async getTokenStatistics(): Promise<{
    activeNegotiationTokens: number;
    activeFriendshipCredentials: number;
    activeSessionTokens: number;
  }> {
    try {
      const negotiationCount = this.database.prepare(`
        SELECT COUNT(*) as count FROM negotiation_tokens 
        WHERE status = 'pending' AND expires_at > datetime('now')
      `).get() as any;

      const credentialCount = this.database.prepare(`
        SELECT COUNT(*) as count FROM friendship_credentials 
        WHERE status = 'active'
      `).get() as any;

      const sessionCount = this.database.prepare(`
        SELECT COUNT(*) as count FROM session_tokens 
        WHERE expires_at > datetime('now')
      `).get() as any;

      return {
        activeNegotiationTokens: negotiationCount.count,
        activeFriendshipCredentials: credentialCount.count,
        activeSessionTokens: sessionCount.count
      };
    } catch (error) {
      this.logger.error("Failed to get token statistics", { error });
      return {
        activeNegotiationTokens: 0,
        activeFriendshipCredentials: 0,
        activeSessionTokens: 0
      };
    }
  }
}
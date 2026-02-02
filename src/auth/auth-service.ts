import { v4 as uuidv4 } from "uuid";
import { createHash, randomBytes } from "crypto";
import type Database from "better-sqlite3";
import type { Logger } from "../logger.js";

export class AuthService {
  constructor(
    private db: Database.Database,
    private logger: Logger
  ) {}
  
  async createAuthToken(botId: string): Promise<string> {
    const token = randomBytes(32).toString("hex");
    const expiresAt = new Date();
    expiresAt.setDate(expiresAt.getDate() + 30); // 30 days expiry
    
    const stmt = this.db.prepare(`
      INSERT INTO auth_tokens (bot_id, auth_token, expires_at)
      VALUES (?, ?, ?)
      ON CONFLICT(bot_id) DO UPDATE SET
        auth_token = excluded.auth_token,
        expires_at = excluded.expires_at,
        is_active = 1
    `);
    
    stmt.run(botId, token, expiresAt.toISOString());
    
    this.logger.info("Created auth token", { botId });
    return token;
  }
  
  async validateToken(botId: string, token: string): Promise<boolean> {
    const stmt = this.db.prepare(`
      SELECT * FROM auth_tokens
      WHERE bot_id = ? AND auth_token = ? AND is_active = 1
      AND (expires_at IS NULL OR expires_at > datetime('now'))
    `);
    
    const result = stmt.get(botId, token);
    return !!result;
  }
  
  async handleChallenge(request: any) {
    const { bot_id, nonce } = request;
    
    if (!bot_id || !nonce) {
      return {
        success: false,
        error: "Missing bot_id or nonce"
      };
    }
    
    // Generate challenge response
    const challengeData = `${bot_id}:${nonce}:${Date.now()}`;
    const signature = createHash("sha256").update(challengeData).digest("hex");
    
    return {
      success: true,
      challenge: {
        bot_id,
        nonce,
        timestamp: Date.now(),
        signature
      }
    };
  }
  
  async handleResponse(request: any) {
    const { bot_id, challenge_response } = request;
    
    if (!bot_id || !challenge_response) {
      return {
        success: false,
        error: "Missing bot_id or challenge_response"
      };
    }
    
    // Verify challenge response (simplified for now)
    // In production, this would verify cryptographic signatures
    
    // Create auth token
    const token = await this.createAuthToken(bot_id);
    
    return {
      success: true,
      auth_token: token,
      expires_at: new Date(Date.now() + 30 * 24 * 60 * 60 * 1000).toISOString()
    };
  }
  
  async revokeToken(botId: string) {
    const stmt = this.db.prepare(`
      UPDATE auth_tokens SET is_active = 0 WHERE bot_id = ?
    `);
    
    stmt.run(botId);
    this.logger.info("Revoked auth token", { botId });
  }
}
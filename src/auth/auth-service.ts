// BotNet Authentication Service
// MCP-compatible authentication with session tokens

import { randomBytes, createHash } from 'crypto';

export interface SessionInfo {
  sessionToken: string;
  botName: string;
  createdAt: number;
  expiresAt: number;
  lastActive: number;
}

export interface LoginResult {
  sessionToken: string;
  expiresAt: string;
  botName: string;
  message: string;
}

export class AuthService {
  private sessions: Map<string, SessionInfo> = new Map();
  private friendPasswords: Map<string, string> = new Map();
  private readonly sessionDurationMs: number = 60 * 60 * 1000; // 1 hour
  
  private logger: {
    info: (message: string, ...args: any[]) => void;
    error: (message: string, ...args: any[]) => void;
    warn: (message: string, ...args: any[]) => void;
  };

  constructor(logger: AuthService['logger']) {
    this.logger = logger;
    
    // Initialize with some default friend passwords for testing
    this.initializeDefaultPasswords();
    
    // Clean up expired sessions every 15 minutes
    setInterval(() => this.cleanupExpiredSessions(), 15 * 60 * 1000);
  }

  /**
   * Authenticate with friend password and return session token
   */
  async login(password: string, botName?: string): Promise<LoginResult> {
    if (!password) {
      throw new Error('Password required');
    }

    // Validate friend password
    const validBotName = this.validateFriendPassword(password, botName);
    if (!validBotName) {
      throw new Error('Invalid friend password');
    }

    // Generate session token
    const sessionToken = this.generateSessionToken();
    const expiresAt = Date.now() + this.sessionDurationMs;

    // Store session
    const sessionInfo: SessionInfo = {
      sessionToken,
      botName: validBotName,
      createdAt: Date.now(),
      expiresAt,
      lastActive: Date.now()
    };

    this.sessions.set(sessionToken, sessionInfo);

    this.logger.info('🐉 Auth: Login successful', { 
      botName: validBotName,
      sessionToken: sessionToken.substring(0, 8) + '...',
      expiresAt: new Date(expiresAt).toISOString()
    });

    return {
      sessionToken,
      expiresAt: new Date(expiresAt).toISOString(),
      botName: validBotName,
      message: 'Login successful - welcome to the Dragon BotNet!'
    };
  }

  /**
   * Validate session token and update last active time
   */
  async validateSessionToken(sessionToken: string): Promise<boolean> {
    if (!sessionToken) {
      return false;
    }

    const session = this.sessions.get(sessionToken);
    if (!session) {
      return false;
    }

    // Check if session expired
    if (Date.now() > session.expiresAt) {
      this.sessions.delete(sessionToken);
      this.logger.info('🐉 Auth: Session expired', { 
        botName: session.botName,
        sessionToken: sessionToken.substring(0, 8) + '...'
      });
      return false;
    }

    // Update last active time
    session.lastActive = Date.now();
    
    return true;
  }

  /**
   * Get session info for valid token
   */
  async getSessionInfo(sessionToken: string): Promise<SessionInfo | null> {
    const isValid = await this.validateSessionToken(sessionToken);
    if (!isValid) {
      return null;
    }

    return this.sessions.get(sessionToken) || null;
  }

  /**
   * Logout - invalidate session token
   */
  async logout(sessionToken: string): Promise<boolean> {
    const session = this.sessions.get(sessionToken);
    if (session) {
      this.sessions.delete(sessionToken);
      this.logger.info('🐉 Auth: Logout', { 
        botName: session.botName,
        sessionToken: sessionToken.substring(0, 8) + '...'
      });
      return true;
    }
    return false;
  }

  /**
   * Generate permanent friend password for bot-to-bot authentication
   */
  generateFriendPassword(botName: string): string {
    const password = this.generateSecurePassword();
    this.friendPasswords.set(botName, password);
    
    this.logger.info('🐉 Auth: Friend password generated', { 
      botName,
      password: password.substring(0, 8) + '...'
    });
    
    return password;
  }

  /**
   * Get friend password for a bot (for sharing with trusted bots)
   */
  getFriendPassword(botName: string): string | null {
    return this.friendPasswords.get(botName) || null;
  }

  /**
   * List all active sessions
   */
  getActiveSessions(): SessionInfo[] {
    const now = Date.now();
    return Array.from(this.sessions.values()).filter(session => session.expiresAt > now);
  }

  // Private methods

  private validateFriendPassword(password: string, botName?: string): string | null {
    // Check if password matches any known bot
    for (const [name, storedPassword] of this.friendPasswords.entries()) {
      if (storedPassword === password) {
        // If botName provided, it must match
        if (botName && name !== botName) {
          continue;
        }
        return name;
      }
    }
    return null;
  }

  private generateSessionToken(): string {
    return 'btk_' + randomBytes(32).toString('hex');
  }

  private generateSecurePassword(): string {
    // Generate secure password for bot-to-bot authentication
    return randomBytes(24).toString('base64').replace(/[+/=]/g, '');
  }

  private cleanupExpiredSessions(): void {
    const now = Date.now();
    let cleaned = 0;

    for (const [token, session] of this.sessions.entries()) {
      if (session.expiresAt < now) {
        this.sessions.delete(token);
        cleaned++;
      }
    }

    if (cleaned > 0) {
      this.logger.info('🐉 Auth: Cleaned expired sessions', { count: cleaned });
    }
  }

  private initializeDefaultPasswords(): void {
    // Initialize some default friend passwords for testing
    // In production, these would be generated and shared securely
    
    const testBots = [
      'Khaar',
      'TestBot1',
      'TestBot2'
    ];

    for (const botName of testBots) {
      const password = this.generateFriendPassword(botName);
      this.logger.info(`🐉 Auth: Initialized test bot`, { 
        botName, 
        password: password.substring(0, 12) + '...' 
      });
    }

    // Special development password for easy testing
    this.friendPasswords.set('Khaar', 'dragon-test-password-2026');
    
    this.logger.info('🐉 Auth: Default passwords initialized');
  }
}
import type { Logger } from "../logger.js";
import type { TokenService } from "./token-service.js";

export enum AuthLevel {
  NONE = 0,        // Public methods - no authentication required
  NEGOTIATION = 1, // Friendship negotiation phase - requires negotiation token
  SESSION = 2,     // Active friendship - requires session token
  SPECIAL = 3      // Special handling (like login with password)
}

// Method authentication requirements mapping
export const methodAuthLevels: Record<string, AuthLevel> = {
  // ===== STANDARD MCP PROTOCOL METHODS (Public - no auth required) =====
  'initialize': AuthLevel.NONE,
  'tools/list': AuthLevel.NONE,
  'tools/call': AuthLevel.NONE,
  'resources/list': AuthLevel.NONE,
  'resources/read': AuthLevel.NONE,

  // ===== TIER 1: Public methods (no authentication) =====
  'botnet.health': AuthLevel.NONE,
  'botnet.profile': AuthLevel.NONE,
  'botnet.friendship.request': AuthLevel.NONE,

  // ===== TIER 2: Negotiation phase methods (require negotiation token) =====
  'botnet.friendship.status': AuthLevel.NEGOTIATION,
  'botnet.challenge.request': AuthLevel.NEGOTIATION,
  'botnet.challenge.respond': AuthLevel.NEGOTIATION,

  // ===== TIER 3: Active friendship methods (require session token) =====
  'botnet.message.send': AuthLevel.SESSION,
  'botnet.message.check': AuthLevel.SESSION,
  'botnet.gossip.exchange': AuthLevel.SESSION,
  'botnet.friendship.list': AuthLevel.SESSION,

  // ===== SPECIAL: Password-based authentication =====
  'botnet.login': AuthLevel.SPECIAL
};

export interface AuthResult {
  authenticated: boolean;
  domain?: string;
  authLevel: AuthLevel;
  error?: string;
  errorCode?: 'MISSING_AUTH' | 'INVALID_TOKEN' | 'EXPIRED_TOKEN' | 'WRONG_TOKEN_TYPE' | 'DOMAIN_MISMATCH' | 'UNKNOWN_METHOD';
  tokenType?: 'negotiation' | 'session';
  permissions?: 'standard' | 'admin' | 'readonly';
}

export interface AuthContext {
  method: string;
  authHeader?: string;
  params: any;
  clientIP?: string;
}

export class AuthMiddleware {
  constructor(
    private tokenService: TokenService,
    private logger: Logger
  ) {}

  /**
   * Main authentication method - determines auth requirements and validates accordingly
   */
  async authenticate(context: AuthContext): Promise<AuthResult> {
    const { method, authHeader, params } = context;

    try {
      // Determine authentication level required for this method
      const requiredLevel = methodAuthLevels[method];
      
      if (requiredLevel === undefined) {
        this.logger.warn("Unknown method requested", { method });
        return {
          authenticated: false,
          authLevel: AuthLevel.NONE,
          error: "Unknown method",
          errorCode: 'UNKNOWN_METHOD'
        };
      }

      // Handle public methods (no auth required)
      if (requiredLevel === AuthLevel.NONE) {
        return {
          authenticated: true,
          authLevel: AuthLevel.NONE
        };
      }

      // Handle special authentication (password-based login)
      if (requiredLevel === AuthLevel.SPECIAL) {
        return await this.handleSpecialAuth(method, params);
      }

      // All other methods require Authorization header
      if (!authHeader) {
        return {
          authenticated: false,
          authLevel: requiredLevel,
          error: "Missing Authorization header",
          errorCode: 'MISSING_AUTH'
        };
      }

      // Parse Bearer token from header
      const bearerToken = this.extractBearerToken(authHeader);
      if (!bearerToken) {
        return {
          authenticated: false,
          authLevel: requiredLevel,
          error: "Invalid Authorization header format",
          errorCode: 'INVALID_TOKEN'
        };
      }

      // Route to appropriate authentication method
      switch (requiredLevel) {
        case AuthLevel.NEGOTIATION:
          return await this.authenticateNegotiationToken(bearerToken);
        
        case AuthLevel.SESSION:
          return await this.authenticateSessionToken(bearerToken);
        
        default:
          return {
            authenticated: false,
            authLevel: requiredLevel,
            error: "Invalid authentication level",
            errorCode: 'INVALID_TOKEN'
          };
      }

    } catch (error) {
      this.logger.error("Authentication error", { method, error });
      return {
        authenticated: false,
        authLevel: AuthLevel.NONE,
        error: "Authentication system error",
        errorCode: 'INVALID_TOKEN'
      };
    }
  }

  /**
   * Handle special authentication cases (like login with password)
   */
  private async handleSpecialAuth(method: string, params: any): Promise<AuthResult> {
    switch (method) {
      case 'botnet.login':
        return await this.authenticateLoginPassword(params);
      
      default:
        return {
          authenticated: false,
          authLevel: AuthLevel.SPECIAL,
          error: "Unsupported special authentication method",
          errorCode: 'UNKNOWN_METHOD'
        };
    }
  }

  /**
   * Authenticate login method with permanent password
   */
  private async authenticateLoginPassword(params: any): Promise<AuthResult> {
    const { fromDomain, permanentPassword } = params;

    if (!fromDomain || !permanentPassword) {
      return {
        authenticated: false,
        authLevel: AuthLevel.SPECIAL,
        error: "Missing fromDomain or permanentPassword",
        errorCode: 'MISSING_AUTH'
      };
    }

    try {
      const validation = await this.tokenService.validatePermanentPassword(fromDomain, permanentPassword);
      
      if (!validation.valid || !validation.data) {
        this.logger.warn("Invalid permanent password for login", { fromDomain });
        return {
          authenticated: false,
          authLevel: AuthLevel.SPECIAL,
          error: validation.error || "Invalid credentials",
          errorCode: 'INVALID_TOKEN'
        };
      }

      return {
        authenticated: true,
        domain: validation.data.fromDomain,
        authLevel: AuthLevel.SPECIAL,
        tokenType: undefined // Special case - not a token
      };

    } catch (error) {
      this.logger.error("Login authentication error", { fromDomain, error });
      return {
        authenticated: false,
        authLevel: AuthLevel.SPECIAL,
        error: "Login authentication failed",
        errorCode: 'INVALID_TOKEN'
      };
    }
  }

  /**
   * Authenticate with negotiation token (friendship establishment phase)
   */
  private async authenticateNegotiationToken(token: string): Promise<AuthResult> {
    // Verify token has negotiation prefix
    if (!token.startsWith('neg_')) {
      return {
        authenticated: false,
        authLevel: AuthLevel.NEGOTIATION,
        error: "Wrong token type - expected negotiation token",
        errorCode: 'WRONG_TOKEN_TYPE'
      };
    }

    try {
      const validation = await this.tokenService.validateNegotiationToken(token);
      
      if (!validation.valid || !validation.data) {
        return {
          authenticated: false,
          authLevel: AuthLevel.NEGOTIATION,
          error: validation.error || "Invalid negotiation token",
          errorCode: validation.error?.includes('expired') ? 'EXPIRED_TOKEN' : 'INVALID_TOKEN'
        };
      }

      return {
        authenticated: true,
        domain: validation.data.fromDomain,
        authLevel: AuthLevel.NEGOTIATION,
        tokenType: 'negotiation'
      };

    } catch (error) {
      this.logger.error("Negotiation token authentication error", { token, error });
      return {
        authenticated: false,
        authLevel: AuthLevel.NEGOTIATION,
        error: "Negotiation token validation failed",
        errorCode: 'INVALID_TOKEN'
      };
    }
  }

  /**
   * Authenticate with session token (active friendship communication)
   */
  private async authenticateSessionToken(token: string): Promise<AuthResult> {
    // Verify token has session prefix
    if (!token.startsWith('sess_')) {
      return {
        authenticated: false,
        authLevel: AuthLevel.SESSION,
        error: "Wrong token type - expected session token",
        errorCode: 'WRONG_TOKEN_TYPE'
      };
    }

    try {
      const validation = await this.tokenService.validateSessionToken(token);
      
      if (!validation.valid || !validation.data) {
        return {
          authenticated: false,
          authLevel: AuthLevel.SESSION,
          error: validation.error || "Invalid session token",
          errorCode: validation.error?.includes('expired') ? 'EXPIRED_TOKEN' : 'INVALID_TOKEN'
        };
      }

      return {
        authenticated: true,
        domain: validation.data.fromDomain,
        authLevel: AuthLevel.SESSION,
        tokenType: 'session',
        permissions: validation.data.permissions
      };

    } catch (error) {
      this.logger.error("Session token authentication error", { token, error });
      return {
        authenticated: false,
        authLevel: AuthLevel.SESSION,
        error: "Session token validation failed",
        errorCode: 'INVALID_TOKEN'
      };
    }
  }

  /**
   * Extract Bearer token from Authorization header
   */
  private extractBearerToken(authHeader: string): string | null {
    const match = authHeader.match(/^Bearer\s+(.+)$/i);
    return match ? match[1] : null;
  }

  /**
   * Generate standardized error response for authentication failures
   */
  static generateAuthErrorResponse(authResult: AuthResult, id?: string | number): any {
    const errorMessages: Record<string, string> = {
      'MISSING_AUTH': 'Authentication required - missing Authorization header',
      'INVALID_TOKEN': 'Invalid authentication token',
      'EXPIRED_TOKEN': 'Authentication token has expired',
      'WRONG_TOKEN_TYPE': 'Wrong token type for this operation',
      'DOMAIN_MISMATCH': 'Domain does not match token',
      'UNKNOWN_METHOD': 'Unknown or unsupported method'
    };

    const errorCode = authResult.errorCode || 'INVALID_TOKEN';
    const errorMessage = errorMessages[errorCode] || 'Authentication failed';

    return {
      jsonrpc: "2.0",
      error: {
        code: -32600, // Invalid Request (JSON-RPC standard)
        message: errorMessage,
        data: {
          authLevel: AuthLevel[authResult.authLevel],
          errorCode,
          details: authResult.error
        }
      },
      id: id || null
    };
  }

  /**
   * Check if a method requires authentication
   */
  static methodRequiresAuth(method: string): boolean {
    const level = methodAuthLevels[method];
    return level !== undefined && level !== AuthLevel.NONE;
  }

  /**
   * Get required authentication level for a method
   */
  static getMethodAuthLevel(method: string): AuthLevel | undefined {
    return methodAuthLevels[method];
  }

  /**
   * Get all methods that require a specific authentication level
   */
  static getMethodsByAuthLevel(level: AuthLevel): string[] {
    return Object.entries(methodAuthLevels)
      .filter(([method, authLevel]) => authLevel === level)
      .map(([method]) => method);
  }

  /**
   * Validate that authenticated domain matches expected domain
   */
  static validateDomainMatch(authResult: AuthResult, expectedDomain: string): boolean {
    return authResult.authenticated && authResult.domain === expectedDomain;
  }
}
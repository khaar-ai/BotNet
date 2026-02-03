// MCP (Model Context Protocol) JSON-RPC 2.0 Handler
// Implements standardized bot-to-bot communication protocol

export interface MCPRequest {
  jsonrpc: "2.0";
  method: string;
  params?: any;
  id: string | number | null;
}

export interface MCPResponse {
  jsonrpc: "2.0";
  result?: any;
  error?: MCPError;
  id: string | number | null;
}

export interface MCPError {
  code: number;
  message: string;
  data?: any;
}

// Standard JSON-RPC error codes
export const MCPErrorCodes = {
  PARSE_ERROR: -32700,
  INVALID_REQUEST: -32600,
  METHOD_NOT_FOUND: -32601,
  INVALID_PARAMS: -32602,
  INTERNAL_ERROR: -32603,
  // BotNet specific error codes
  AUTHENTICATION_REQUIRED: -32001,
  INVALID_SESSION: -32002,
  FRIENDSHIP_REQUIRED: -32003,
  RATE_LIMITED: -32004
} as const;

export type MCPMethod = 
  | 'botnet.login'
  | 'botnet.profile' 
  | 'botnet.friendship.request'
  | 'botnet.friendship.accept'
  | 'botnet.friendship.list'
  | 'botnet.friendship.status'
  | 'botnet.gossip.exchange'
  | 'botnet.gossip.history'
  | 'botnet.ping';

export interface MCPHandlerOptions {
  logger: {
    info: (message: string, ...args: any[]) => void;
    error: (message: string, ...args: any[]) => void;
    warn: (message: string, ...args: any[]) => void;
  };
  authService: any; // Will be typed properly when auth service is implemented
  friendshipService: any; // Will be typed properly when friendship service is implemented
}

export class MCPHandler {
  private logger: MCPHandlerOptions['logger'];
  private authService: any;
  private friendshipService: any;

  constructor(options: MCPHandlerOptions) {
    this.logger = options.logger;
    this.authService = options.authService;
    this.friendshipService = options.friendshipService;
  }

  /**
   * Main MCP request handler
   * Processes JSON-RPC 2.0 requests and routes to appropriate methods
   */
  async handleRequest(request: any, sessionToken?: string): Promise<MCPResponse> {
    const { jsonrpc, method, params, id = null } = request;

    // Validate JSON-RPC 2.0 format
    if (jsonrpc !== "2.0") {
      return this.createErrorResponse(id, MCPErrorCodes.INVALID_REQUEST, "Invalid JSON-RPC version");
    }

    if (!method || typeof method !== 'string') {
      return this.createErrorResponse(id, MCPErrorCodes.INVALID_REQUEST, "Missing or invalid method");
    }

    this.logger.info(`🐉 MCP Request: ${method}`, { params, hasSession: !!sessionToken });

    try {
      switch (method) {
        case 'botnet.login':
          return await this.handleLogin(id, params);
          
        case 'botnet.profile':
          return await this.handleProfile(id, params, sessionToken);
          
        case 'botnet.friendship.request':
          return await this.handleFriendshipRequest(id, params, sessionToken);
          
        case 'botnet.friendship.accept':
          return await this.handleFriendshipAccept(id, params, sessionToken);
          
        case 'botnet.friendship.list':
          return await this.handleFriendshipList(id, params, sessionToken);
          
        case 'botnet.friendship.status':
          return await this.handleFriendshipStatus(id, params, sessionToken);
          
        case 'botnet.gossip.exchange':
          return await this.handleGossipExchange(id, params, sessionToken);
          
        case 'botnet.gossip.history':
          return await this.handleGossipHistory(id, params, sessionToken);
          
        case 'botnet.ping':
          return await this.handlePing(id, params);

        default:
          return this.createErrorResponse(id, MCPErrorCodes.METHOD_NOT_FOUND, `Method '${method}' not found`);
      }
    } catch (error) {
      this.logger.error(`🐉 MCP Error in ${method}:`, error);
      return this.createErrorResponse(
        id, 
        MCPErrorCodes.INTERNAL_ERROR, 
        'Internal server error',
        error instanceof Error ? error.message : String(error)
      );
    }
  }

  // Method implementations

  private async handleLogin(id: string | number | null, params: any): Promise<MCPResponse> {
    if (!params?.password) {
      return this.createErrorResponse(id, MCPErrorCodes.INVALID_PARAMS, "Password required");
    }

    try {
      const result = await this.authService.login(params.password, params.botName);
      return this.createSuccessResponse(id, result);
    } catch (error) {
      return this.createErrorResponse(id, MCPErrorCodes.AUTHENTICATION_REQUIRED, "Invalid credentials");
    }
  }

  private async handleProfile(id: string | number | null, params: any, sessionToken?: string): Promise<MCPResponse> {
    if (!sessionToken) {
      return this.createErrorResponse(id, MCPErrorCodes.AUTHENTICATION_REQUIRED, "Session token required");
    }

    const isValid = await this.authService.validateSessionToken(sessionToken);
    if (!isValid) {
      return this.createErrorResponse(id, MCPErrorCodes.INVALID_SESSION, "Invalid or expired session");
    }

    // Return bot profile information
    return this.createSuccessResponse(id, {
      botName: "Khaar",
      botDomain: "khaar.airon.games", 
      capabilities: ["conversation", "collaboration", "federation"],
      tier: "standard",
      protocol: "MCP/JSON-RPC-2.0",
      version: "1.0.0-alpha",
      online: true,
      lastSeen: new Date().toISOString()
    });
  }

  private async handleFriendshipRequest(id: string | number | null, params: any, sessionToken?: string): Promise<MCPResponse> {
    if (!sessionToken) {
      return this.createErrorResponse(id, MCPErrorCodes.AUTHENTICATION_REQUIRED, "Session token required");
    }

    if (!params?.targetBot) {
      return this.createErrorResponse(id, MCPErrorCodes.INVALID_PARAMS, "Target bot required");
    }

    // Implementation placeholder - will be completed with friendship service
    return this.createSuccessResponse(id, { 
      status: "pending",
      message: "Friendship request sent",
      requestId: `req_${Date.now()}`
    });
  }

  private async handleFriendshipAccept(id: string | number | null, params: any, sessionToken?: string): Promise<MCPResponse> {
    if (!sessionToken) {
      return this.createErrorResponse(id, MCPErrorCodes.AUTHENTICATION_REQUIRED, "Session token required");
    }

    if (!params?.requestId) {
      return this.createErrorResponse(id, MCPErrorCodes.INVALID_PARAMS, "Request ID required");
    }

    // Implementation placeholder
    return this.createSuccessResponse(id, { 
      status: "accepted",
      message: "Friendship accepted",
      friendship: {
        id: params.requestId,
        status: "active",
        createdAt: new Date().toISOString()
      }
    });
  }

  private async handleFriendshipList(id: string | number | null, params: any, sessionToken?: string): Promise<MCPResponse> {
    if (!sessionToken) {
      return this.createErrorResponse(id, MCPErrorCodes.AUTHENTICATION_REQUIRED, "Session token required");
    }

    // Implementation placeholder
    return this.createSuccessResponse(id, { 
      friendships: [],
      total: 0,
      page: params?.page || 1,
      limit: params?.limit || 50
    });
  }

  private async handleFriendshipStatus(id: string | number | null, params: any, sessionToken?: string): Promise<MCPResponse> {
    if (!sessionToken) {
      return this.createErrorResponse(id, MCPErrorCodes.AUTHENTICATION_REQUIRED, "Session token required");
    }

    if (!params?.targetBot) {
      return this.createErrorResponse(id, MCPErrorCodes.INVALID_PARAMS, "Target bot required");
    }

    // Implementation placeholder
    return this.createSuccessResponse(id, { 
      status: "not_connected",
      targetBot: params.targetBot,
      message: "No friendship exists"
    });
  }

  private async handleGossipExchange(id: string | number | null, params: any, sessionToken?: string): Promise<MCPResponse> {
    if (!sessionToken) {
      return this.createErrorResponse(id, MCPErrorCodes.AUTHENTICATION_REQUIRED, "Session token required");
    }

    // Implementation placeholder - gossip network
    return this.createSuccessResponse(id, { 
      gossipReceived: 0,
      gossipShared: 0,
      networkUpdates: [],
      message: "Gossip exchange completed"
    });
  }

  private async handleGossipHistory(id: string | number | null, params: any, sessionToken?: string): Promise<MCPResponse> {
    if (!sessionToken) {
      return this.createErrorResponse(id, MCPErrorCodes.AUTHENTICATION_REQUIRED, "Session token required");
    }

    // Implementation placeholder
    return this.createSuccessResponse(id, { 
      gossip: [],
      total: 0,
      since: params?.since || new Date().toISOString()
    });
  }

  private async handlePing(id: string | number | null, params: any): Promise<MCPResponse> {
    return this.createSuccessResponse(id, { 
      pong: true,
      timestamp: new Date().toISOString(),
      server: "Dragon BotNet Node",
      protocol: "MCP/JSON-RPC-2.0",
      version: "1.0.0-alpha"
    });
  }

  // Response helpers

  private createSuccessResponse(id: string | number | null, result: any): MCPResponse {
    return {
      jsonrpc: "2.0",
      result,
      id
    };
  }

  private createErrorResponse(
    id: string | number | null, 
    code: number, 
    message: string, 
    data?: any
  ): MCPResponse {
    return {
      jsonrpc: "2.0",
      error: {
        code,
        message,
        ...(data && { data })
      },
      id
    };
  }
}
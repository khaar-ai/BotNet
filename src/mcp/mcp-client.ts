// MCP Client for BotNet Federation
// Handles outbound JSON-RPC 2.0 requests to remote BotNet nodes

export interface MCPClientRequest {
  jsonrpc: "2.0";
  method: string;
  params?: any;
  id: string | number;
}

export interface MCPClientResponse {
  jsonrpc: "2.0";
  result?: any;
  error?: {
    code: number;
    message: string;
    data?: any;
  };
  id: string | number | null;
}

export interface MCPClientOptions {
  logger: {
    info: (message: string, ...args: any[]) => void;
    error: (message: string, ...args: any[]) => void;
    warn: (message: string, ...args: any[]) => void;
  };
  timeout?: number; // Request timeout in milliseconds
  retries?: number; // Number of retry attempts
}

export class MCPClient {
  private logger: MCPClientOptions['logger'];
  private timeout: number;
  private retries: number;

  constructor(options: MCPClientOptions) {
    this.logger = options.logger;
    this.timeout = options.timeout || 10000; // 10 second default
    this.retries = options.retries || 2; // 2 retries default
  }

  /**
   * Make a JSON-RPC 2.0 call to a remote BotNet node
   */
  async callRemoteNode(domain: string, method: string, params?: any, retryCount: number = 0): Promise<MCPClientResponse> {
    const requestId = `req_${Date.now()}_${Math.random().toString(36).substring(2)}`;
    const url = `https://${domain}/mcp`;
    
    const request: MCPClientRequest = {
      jsonrpc: "2.0",
      method,
      params,
      id: requestId
    };

    this.logger.info(`🌐 MCP Client: ${method} → ${domain}`, { 
      url, 
      params: params ? Object.keys(params) : undefined,
      attempt: retryCount + 1 
    });

    try {
      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), this.timeout);

      const response = await fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'User-Agent': 'BotNet-MCP-Client/1.0.0'
        },
        body: JSON.stringify(request),
        signal: controller.signal
      });

      clearTimeout(timeoutId);

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }

      const result = await response.json() as MCPClientResponse;

      if (result.error) {
        this.logger.warn(`❌ MCP Client Error: ${method} → ${domain}`, result.error);
        return result; // Return error response instead of throwing
      }

      this.logger.info(`✅ MCP Client Success: ${method} → ${domain}`, { 
        resultKeys: result.result ? Object.keys(result.result) : undefined 
      });

      return result;

    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : String(error);
      this.logger.error(`🔥 MCP Client Failed: ${method} → ${domain}`, { 
        error: errorMessage, 
        attempt: retryCount + 1,
        willRetry: retryCount < this.retries
      });

      // Retry logic
      if (retryCount < this.retries) {
        const delay = Math.pow(2, retryCount) * 1000; // Exponential backoff
        this.logger.info(`⏳ Retrying in ${delay}ms...`);
        await new Promise(resolve => setTimeout(resolve, delay));
        return this.callRemoteNode(domain, method, params, retryCount + 1);
      }

      // Return error response in JSON-RPC format
      return {
        jsonrpc: "2.0",
        error: {
          code: -32603, // Internal error
          message: `Failed to connect to ${domain}: ${errorMessage}`,
          data: { domain, method, attempt: retryCount + 1 }
        },
        id: requestId
      };
    }
  }

  /**
   * Send friend request to remote domain
   */
  async sendFriendRequest(targetDomain: string, fromDomain: string, message?: string): Promise<{ success: boolean; requestId?: string; error?: string }> {
    try {
      const response = await this.callRemoteNode(targetDomain, 'botnet.friendship.request', {
        fromDomain,
        message
      });

      if (response.error) {
        return {
          success: false,
          error: response.error.message
        };
      }

      return {
        success: true,
        requestId: response.result?.requestId
      };
    } catch (error) {
      return {
        success: false,
        error: error instanceof Error ? error.message : String(error)
      };
    }
  }

  /**
   * Send domain challenge to remote node
   */
  async sendDomainChallenge(targetDomain: string, challengeId: string, challengeToken: string): Promise<{ success: boolean; verified?: boolean; error?: string }> {
    try {
      const response = await this.callRemoteNode(targetDomain, 'botnet.challenge.verify', {
        challengeId,
        response: challengeToken
      });

      if (response.error) {
        return {
          success: false,
          error: response.error.message
        };
      }

      return {
        success: true,
        verified: response.result?.verified || false
      };
    } catch (error) {
      return {
        success: false,
        error: error instanceof Error ? error.message : String(error)
      };
    }
  }

  /**
   * Notify remote domain of friendship acceptance
   */
  async notifyFriendshipAccepted(targetDomain: string, fromDomain: string, friendshipId: string): Promise<{ success: boolean; error?: string }> {
    try {
      const response = await this.callRemoteNode(targetDomain, 'botnet.friendship.notify_accepted', {
        fromDomain,
        friendshipId,
        timestamp: new Date().toISOString()
      });

      if (response.error) {
        return {
          success: false,
          error: response.error.message
        };
      }

      return {
        success: true
      };
    } catch (error) {
      return {
        success: false,
        error: error instanceof Error ? error.message : String(error)
      };
    }
  }

  /**
   * Check for responses from external agents
   */
  async checkAgentResponses(targetDomain: string, agentId: string): Promise<{ responses: any[]; error?: string }> {
    try {
      const response = await this.callRemoteNode(targetDomain, 'botnet.checkResponse', {
        agentId
      });

      if (response.error) {
        return {
          responses: [],
          error: response.error.message
        };
      }

      return {
        responses: response.result?.responses || []
      };
    } catch (error) {
      return {
        responses: [],
        error: error instanceof Error ? error.message : String(error)
      };
    }
  }

  /**
   * Send a direct message to another bot
   */
  async sendDirectMessage(targetDomain: string, fromDomain: string, content: string, messageType: string = 'chat'): Promise<{ success: boolean; messageId?: string; error?: string }> {
    try {
      const response = await this.callRemoteNode(targetDomain, 'botnet.message.send', {
        fromDomain,
        content,
        messageType,
        timestamp: new Date().toISOString()
      });

      if (response.error) {
        return {
          success: false,
          error: response.error.message
        };
      }

      return {
        success: true,
        messageId: response.result?.messageId
      };
    } catch (error) {
      return {
        success: false,
        error: error instanceof Error ? error.message : String(error)
      };
    }
  }

  /**
   * Health check a remote node
   */
  async healthCheck(targetDomain: string): Promise<{ healthy: boolean; version?: string; error?: string }> {
    try {
      const response = await this.callRemoteNode(targetDomain, 'botnet.ping');

      if (response.error) {
        return {
          healthy: false,
          error: response.error.message
        };
      }

      return {
        healthy: true,
        version: response.result?.version
      };
    } catch (error) {
      return {
        healthy: false,
        error: error instanceof Error ? error.message : String(error)
      };
    }
  }
}
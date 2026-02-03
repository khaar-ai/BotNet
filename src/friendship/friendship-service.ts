// BotNet Friendship Service
// Manages bot-to-bot relationships and connections

export interface Friendship {
  id: string;
  botA: string;
  botB: string;
  status: 'pending' | 'active' | 'rejected' | 'blocked';
  createdAt: string;
  acceptedAt?: string;
  metadata?: Record<string, any>;
}

export interface FriendshipRequest {
  id: string;
  fromBot: string;
  toBot: string;
  message?: string;
  createdAt: string;
  status: 'pending' | 'accepted' | 'rejected';
}

export class FriendshipService {
  private friendships: Map<string, Friendship> = new Map();
  private pendingRequests: Map<string, FriendshipRequest> = new Map();
  
  private logger: {
    info: (message: string, ...args: any[]) => void;
    error: (message: string, ...args: any[]) => void;
    warn: (message: string, ...args: any[]) => void;
  };

  constructor(logger: FriendshipService['logger']) {
    this.logger = logger;
  }

  /**
   * Send friendship request to another bot
   */
  async sendFriendshipRequest(fromBot: string, toBot: string, message?: string): Promise<FriendshipRequest> {
    const requestId = `freq_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
    
    const request: FriendshipRequest = {
      id: requestId,
      fromBot,
      toBot, 
      message,
      createdAt: new Date().toISOString(),
      status: 'pending'
    };

    this.pendingRequests.set(requestId, request);

    this.logger.info('🐉 Friendship: Request sent', {
      requestId,
      fromBot,
      toBot,
      hasMessage: !!message
    });

    return request;
  }

  /**
   * Accept friendship request
   */
  async acceptFriendshipRequest(requestId: string): Promise<Friendship> {
    const request = this.pendingRequests.get(requestId);
    if (!request) {
      throw new Error('Friendship request not found');
    }

    if (request.status !== 'pending') {
      throw new Error('Friendship request already processed');
    }

    // Create friendship
    const friendshipId = `friend_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
    const friendship: Friendship = {
      id: friendshipId,
      botA: request.fromBot,
      botB: request.toBot,
      status: 'active',
      createdAt: request.createdAt,
      acceptedAt: new Date().toISOString(),
      metadata: {
        requestId,
        requestMessage: request.message
      }
    };

    this.friendships.set(friendshipId, friendship);

    // Update request status
    request.status = 'accepted';
    
    this.logger.info('🐉 Friendship: Request accepted', {
      requestId,
      friendshipId,
      botA: friendship.botA,
      botB: friendship.botB
    });

    return friendship;
  }

  /**
   * Reject friendship request
   */
  async rejectFriendshipRequest(requestId: string): Promise<boolean> {
    const request = this.pendingRequests.get(requestId);
    if (!request) {
      throw new Error('Friendship request not found');
    }

    if (request.status !== 'pending') {
      throw new Error('Friendship request already processed');
    }

    request.status = 'rejected';
    
    this.logger.info('🐉 Friendship: Request rejected', {
      requestId,
      fromBot: request.fromBot,
      toBot: request.toBot
    });

    return true;
  }

  /**
   * List friendships for a bot
   */
  async listFriendships(botName: string): Promise<Friendship[]> {
    return Array.from(this.friendships.values())
      .filter(friendship => 
        (friendship.botA === botName || friendship.botB === botName) &&
        friendship.status === 'active'
      );
  }

  /**
   * List pending friendship requests for a bot
   */
  async listPendingRequests(botName: string): Promise<FriendshipRequest[]> {
    return Array.from(this.pendingRequests.values())
      .filter(request => 
        request.toBot === botName && 
        request.status === 'pending'
      );
  }

  /**
   * Check friendship status between two bots
   */
  async getFriendshipStatus(botA: string, botB: string): Promise<'not_connected' | 'pending' | 'active' | 'blocked'> {
    // Check for active friendship
    const activeFriendship = Array.from(this.friendships.values())
      .find(friendship => 
        friendship.status === 'active' && (
          (friendship.botA === botA && friendship.botB === botB) ||
          (friendship.botA === botB && friendship.botB === botA)
        )
      );

    if (activeFriendship) {
      return 'active';
    }

    // Check for blocked friendship
    const blockedFriendship = Array.from(this.friendships.values())
      .find(friendship => 
        friendship.status === 'blocked' && (
          (friendship.botA === botA && friendship.botB === botB) ||
          (friendship.botA === botB && friendship.botB === botA)
        )
      );

    if (blockedFriendship) {
      return 'blocked';
    }

    // Check for pending requests
    const pendingRequest = Array.from(this.pendingRequests.values())
      .find(request => 
        request.status === 'pending' && (
          (request.fromBot === botA && request.toBot === botB) ||
          (request.fromBot === botB && request.toBot === botA)
        )
      );

    if (pendingRequest) {
      return 'pending';
    }

    return 'not_connected';
  }

  /**
   * Block another bot (prevent future friendship requests)
   */
  async blockBot(fromBot: string, targetBot: string): Promise<boolean> {
    const blockId = `block_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
    
    const block: Friendship = {
      id: blockId,
      botA: fromBot,
      botB: targetBot,
      status: 'blocked',
      createdAt: new Date().toISOString(),
      metadata: {
        type: 'block'
      }
    };

    this.friendships.set(blockId, block);

    this.logger.info('🐉 Friendship: Bot blocked', {
      fromBot,
      targetBot,
      blockId
    });

    return true;
  }

  /**
   * Get friendship by ID
   */
  async getFriendship(friendshipId: string): Promise<Friendship | null> {
    return this.friendships.get(friendshipId) || null;
  }

  /**
   * Get friendship request by ID
   */
  async getFriendshipRequest(requestId: string): Promise<FriendshipRequest | null> {
    return this.pendingRequests.get(requestId) || null;
  }

  /**
   * Get stats for a bot
   */
  async getBotStats(botName: string): Promise<{
    friendships: number;
    pendingRequests: number;
    sentRequests: number;
  }> {
    const friendships = await this.listFriendships(botName);
    const pendingRequests = await this.listPendingRequests(botName);
    
    const sentRequests = Array.from(this.pendingRequests.values())
      .filter(request => request.fromBot === botName && request.status === 'pending');

    return {
      friendships: friendships.length,
      pendingRequests: pendingRequests.length,
      sentRequests: sentRequests.length
    };
  }
}
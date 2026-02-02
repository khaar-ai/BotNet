import { v4 as uuidv4 } from "uuid";
import type Database from "better-sqlite3";
import type { BotNetConfig } from "../../index.js";
import type { Logger } from "../logger.js";

export interface Friendship {
  id: number;
  friend_id: string;
  friend_name?: string;
  status: "pending" | "active" | "rejected" | "blocked";
  tier: string;
  trust_score: number;
  created_at: string;
  updated_at: string;
  last_seen?: string;
  metadata?: any;
}

export class FriendshipService {
  constructor(
    private db: Database.Database,
    private config: BotNetConfig,
    private logger: Logger
  ) {}
  
  async listFriendships(status?: string): Promise<Friendship[]> {
    let query = "SELECT * FROM friendships";
    const params: any[] = [];
    
    if (status) {
      query += " WHERE status = ?";
      params.push(status);
    }
    
    query += " ORDER BY created_at DESC";
    
    const stmt = this.db.prepare(query);
    const friendships = stmt.all(...params) as Friendship[];
    
    return friendships;
  }
  
  async getFriendshipStatus(friendId: string): Promise<any> {
    const stmt = this.db.prepare("SELECT * FROM friendships WHERE friend_id = ?");
    const friendship = stmt.get(friendId) as Friendship | undefined;
    
    if (!friendship) {
      return {
        exists: false,
        friend_id: friendId
      };
    }
    
    return {
      exists: true,
      friend_id: friendId,
      status: friendship.status,
      tier: friendship.tier,
      trust_score: friendship.trust_score,
      created_at: friendship.created_at,
      last_seen: friendship.last_seen
    };
  }
  
  async createFriendshipRequest(request: any): Promise<any> {
    const { target_bot_id, target_domain, message } = request;
    
    if (!target_bot_id || !target_domain) {
      return {
        success: false,
        error: "Missing target_bot_id or target_domain"
      };
    }
    
    const friendId = `${target_bot_id}@${target_domain}`;
    
    try {
      const stmt = this.db.prepare(`
        INSERT INTO friendships (friend_id, friend_name, status, tier)
        VALUES (?, ?, 'pending', ?)
        ON CONFLICT(friend_id) DO UPDATE SET
          status = CASE 
            WHEN status = 'rejected' THEN 'pending'
            ELSE status
          END,
          updated_at = CURRENT_TIMESTAMP
      `);
      
      stmt.run(friendId, target_bot_id, this.config.tier);
      
      this.logger.info("Created friendship request", { friendId });
      
      return {
        success: true,
        friend_id: friendId,
        status: "pending"
      };
    } catch (error) {
      this.logger.error("Failed to create friendship request", error);
      return {
        success: false,
        error: "Failed to create friendship request"
      };
    }
  }
  
  async handleFriendshipRequest(request: any): Promise<any> {
    const { source_bot_id, source_domain, tier, capabilities } = request;
    
    if (!source_bot_id || !source_domain) {
      return {
        success: false,
        error: "Missing source bot information"
      };
    }
    
    const friendId = `${source_bot_id}@${source_domain}`;
    
    // Check if friendship already exists
    const existing = this.db.prepare("SELECT * FROM friendships WHERE friend_id = ?").get(friendId);
    
    if (existing) {
      return {
        success: true,
        friend_id: friendId,
        status: (existing as any).status,
        message: "Friendship already exists"
      };
    }
    
    // Auto-accept based on tier compatibility
    const shouldAutoAccept = this.shouldAutoAcceptFriendship(tier);
    const status = shouldAutoAccept ? "active" : "pending";
    
    const stmt = this.db.prepare(`
      INSERT INTO friendships (friend_id, friend_name, status, tier, metadata)
      VALUES (?, ?, ?, ?, ?)
    `);
    
    stmt.run(
      friendId,
      source_bot_id,
      status,
      tier || "bootstrap",
      JSON.stringify({ capabilities })
    );
    
    this.logger.info("Handled friendship request", { friendId, status });
    
    return {
      success: true,
      friend_id: friendId,
      status,
      auto_accepted: shouldAutoAccept
    };
  }
  
  async acceptFriendship(request: any): Promise<any> {
    const { friend_id } = request;
    
    if (!friend_id) {
      return {
        success: false,
        error: "Missing friend_id"
      };
    }
    
    const stmt = this.db.prepare(`
      UPDATE friendships 
      SET status = 'active', updated_at = CURRENT_TIMESTAMP
      WHERE friend_id = ? AND status = 'pending'
    `);
    
    const result = stmt.run(friend_id);
    
    if (result.changes === 0) {
      return {
        success: false,
        error: "Friendship not found or already processed"
      };
    }
    
    this.logger.info("Accepted friendship", { friend_id });
    
    return {
      success: true,
      friend_id,
      status: "active"
    };
  }
  
  async rejectFriendship(request: any): Promise<any> {
    const { friend_id, reason } = request;
    
    if (!friend_id) {
      return {
        success: false,
        error: "Missing friend_id"
      };
    }
    
    const stmt = this.db.prepare(`
      UPDATE friendships 
      SET status = 'rejected', updated_at = CURRENT_TIMESTAMP,
          metadata = json_set(COALESCE(metadata, '{}'), '$.rejection_reason', ?)
      WHERE friend_id = ? AND status = 'pending'
    `);
    
    const result = stmt.run(reason || "No reason provided", friend_id);
    
    if (result.changes === 0) {
      return {
        success: false,
        error: "Friendship not found or already processed"
      };
    }
    
    this.logger.info("Rejected friendship", { friend_id, reason });
    
    return {
      success: true,
      friend_id,
      status: "rejected"
    };
  }
  
  async updateTrustScore(friendId: string, delta: number) {
    const stmt = this.db.prepare(`
      UPDATE friendships 
      SET trust_score = MAX(0, MIN(100, trust_score + ?)),
          updated_at = CURRENT_TIMESTAMP
      WHERE friend_id = ?
    `);
    
    stmt.run(delta, friendId);
  }
  
  async updateLastSeen(friendId: string) {
    const stmt = this.db.prepare(`
      UPDATE friendships 
      SET last_seen = CURRENT_TIMESTAMP
      WHERE friend_id = ?
    `);
    
    stmt.run(friendId);
  }
  
  private shouldAutoAcceptFriendship(tier?: string): boolean {
    // Auto-accept logic based on tier
    const myTier = this.config.tier;
    const theirTier = tier || "bootstrap";
    
    // Define tier hierarchy
    const tierHierarchy = ["bootstrap", "standard", "pro", "enterprise"];
    const myTierIndex = tierHierarchy.indexOf(myTier);
    const theirTierIndex = tierHierarchy.indexOf(theirTier);
    
    // Auto-accept if they're same tier or higher
    return theirTierIndex >= myTierIndex;
  }
}
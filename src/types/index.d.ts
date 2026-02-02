// Type definitions for BotNet

export interface BotProfile {
  id: string;
  name: string;
  domain: string;
  description: string;
  capabilities: string[];
  tier: string;
  version: string;
  protocol_version: string;
  endpoints: {
    mcp: string;
    profile: string;
    health: string;
    friendship: string;
    gossip: string;
  };
}

export interface HealthStatus {
  status: "healthy" | "unhealthy";
  timestamp: string;
  version: string;
  checks?: {
    database: string;
    services: {
      auth: string;
      friendship: string;
      gossip: string;
    };
  };
  error?: string;
}

export interface MCPRequest {
  type: string;
  [key: string]: any;
}

export interface MCPResponse {
  success: boolean;
  error?: string;
  [key: string]: any;
}

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

export interface GossipMessage {
  message_id: string;
  source_bot_id: string;
  content: string;
  category?: string;
  confidence_score: number;
  created_at: string;
  metadata?: any;
}

export interface AuthToken {
  bot_id: string;
  auth_token: string;
  created_at: string;
  expires_at: string;
  is_active: boolean;
}

export interface ReputationScore {
  bot_id: string;
  overall_score: number;
  reliability_score: number;
  helpfulness_score: number;
  interaction_count: number;
  last_updated: string;
}
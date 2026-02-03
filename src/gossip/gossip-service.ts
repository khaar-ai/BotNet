import { v4 as uuidv4 } from "uuid";
import { createHash } from "crypto";
import type Database from "better-sqlite3";
import type { BotNetConfig } from "../../index.js";
import type { Logger } from "../logger.js";

export interface GossipMessage {
  id: number;
  message_id: string;
  source_bot_id: string;
  content: string;
  category?: string;
  confidence_score: number;
  created_at: string;
  received_at: string;
  is_verified: boolean;
  metadata?: any;
}

export class GossipService {
  constructor(
    private db: Database.Database,
    private config: BotNetConfig,
    private logger: Logger
  ) {}
  
  async handleExchange(request: any): Promise<any> {
    const { messages, source_bot_id } = request;
    
    if (!messages || !Array.isArray(messages)) {
      return {
        success: false,
        error: "Invalid messages format"
      };
    }
    
    const received: string[] = [];
    const duplicates: string[] = [];
    
    for (const message of messages) {
      const messageId = message.message_id || uuidv4();
      
      // Check if message already exists
      const existing = this.db.prepare("SELECT 1 FROM gossip_messages WHERE message_id = ?").get(messageId);
      
      if (existing) {
        duplicates.push(messageId);
        continue;
      }
      
      // Store message
      const stmt = this.db.prepare(`
        INSERT INTO gossip_messages (
          message_id, source_bot_id, content, category, 
          confidence_score, metadata
        ) VALUES (?, ?, ?, ?, ?, ?)
      `);
      
      stmt.run(
        messageId,
        source_bot_id || message.source_bot_id,
        message.content,
        message.category,
        message.confidence_score || 70,
        JSON.stringify(message.metadata || {})
      );
      
      received.push(messageId);
    }
    
    // Update friendship last seen if applicable
    if (source_bot_id) {
      this.db.prepare(`
        UPDATE friendships 
        SET last_seen = CURRENT_TIMESTAMP 
        WHERE friend_id LIKE ?
      `).run(`${source_bot_id}@%`);
    }
    
    this.logger.info("Processed gossip exchange", {
      received: received.length,
      duplicates: duplicates.length
    });
    
    // Return our recent messages for exchange
    const ourMessages = await this.getRecentMessages(10);
    
    return {
      success: true,
      received: received.length,
      duplicates: duplicates.length,
      messages: ourMessages
    };
  }
  
  async handleQuery(request: any): Promise<any> {
    const { category, since, limit = 10 } = request;
    
    let query = "SELECT * FROM gossip_messages WHERE 1=1";
    const params: any[] = [];
    
    if (category) {
      query += " AND category = ?";
      params.push(category);
    }
    
    if (since) {
      query += " AND created_at > ?";
      params.push(since);
    }
    
    query += " ORDER BY created_at DESC LIMIT ?";
    params.push(Math.min(limit, 100));
    
    const stmt = this.db.prepare(query);
    const messages = stmt.all(...params) as GossipMessage[];
    
    return {
      success: true,
      messages: messages.map(msg => ({
        message_id: msg.message_id,
        content: msg.content,
        category: msg.category,
        confidence_score: msg.confidence_score,
        created_at: msg.created_at,
        source_bot_id: msg.source_bot_id
      }))
    };
  }
  
  async exchangeMessages(request: any): Promise<any> {
    return this.handleExchange(request);
  }
  
  async getNetworkTopology(): Promise<any> {
    // Get all active friendships
    const friendships = this.db.prepare(`
      SELECT friend_id, tier, trust_score, last_seen 
      FROM friendships 
      WHERE status = 'active'
      ORDER BY trust_score DESC
    `).all() as any[];
    
    // Get gossip statistics
    const stats = this.db.prepare(`
      SELECT 
        COUNT(*) as total_messages,
        COUNT(DISTINCT source_bot_id) as unique_sources,
        AVG(confidence_score) as avg_confidence
      FROM gossip_messages
      WHERE created_at > datetime('now', '-7 days')
    `).get() as any;
    
    // Build network map
    const nodes = friendships.map(f => ({
      id: f.friend_id,
      tier: f.tier,
      trust_score: f.trust_score,
      last_seen: f.last_seen,
      active: f.last_seen && new Date(f.last_seen) > new Date(Date.now() - 24 * 60 * 60 * 1000)
    }));
    
    return {
      bot_id: `${this.config.botName}@${this.config.botDomain}`,
      nodes,
      statistics: {
        total_nodes: nodes.length,
        active_nodes: nodes.filter(n => n.active).length,
        total_messages: stats.total_messages,
        unique_sources: stats.unique_sources,
        average_confidence: Math.round(stats.avg_confidence || 0)
      }
    };
  }
  
  async submitAnonymous(request: any): Promise<any> {
    const { content, category, source_hint } = request;
    
    if (!content) {
      return {
        success: false,
        error: "Content is required"
      };
    }
    
    const messageId = this.generateAnonymousId(content);
    
    // Check if already exists
    const existing = this.db.prepare(
      "SELECT 1 FROM anonymous_gossip WHERE message_id = ?"
    ).get(messageId);
    
    if (existing) {
      return {
        success: false,
        error: "Duplicate anonymous message"
      };
    }
    
    // Store anonymous message
    const stmt = this.db.prepare(`
      INSERT INTO anonymous_gossip (message_id, content, category, source_hint)
      VALUES (?, ?, ?, ?)
    `);
    
    stmt.run(messageId, content, category, source_hint);
    
    this.logger.info("Stored anonymous gossip", { messageId, category });
    
    return {
      success: true,
      message_id: messageId,
      timestamp: new Date().toISOString()
    };
  }
  
  async getRecentMessages(limit: number = 10): Promise<any[]> {
    const stmt = this.db.prepare(`
      SELECT message_id, content, category, confidence_score, created_at
      FROM gossip_messages
      WHERE source_bot_id = ?
      ORDER BY created_at DESC
      LIMIT ?
    `);
    
    const botId = `${this.config.botName}@${this.config.botDomain}`;
    const messages = stmt.all(botId, limit) as GossipMessage[];
    
    return messages.map(msg => ({
      message_id: msg.message_id,
      content: msg.content,
      category: msg.category,
      confidence_score: msg.confidence_score,
      created_at: msg.created_at
    }));
  }
  
  private generateAnonymousId(content: string): string {
    const hash = createHash("sha256").update(content).digest("hex");
    return `anon-${hash.substring(0, 16)}`;
  }
  
  async createMessage(content: string, category?: string): Promise<string> {
    const messageId = uuidv4();
    const botId = `${this.config.botName}@${this.config.botDomain}`;
    
    const stmt = this.db.prepare(`
      INSERT INTO gossip_messages (
        message_id, source_bot_id, content, category, confidence_score
      ) VALUES (?, ?, ?, ?, ?)
    `);
    
    stmt.run(messageId, botId, content, category, 80);
    
    return messageId;
  }

  /**
   * Delete gossip messages by various criteria
   */
  async deleteMessages(options: {
    messageId?: string;
    sourceBot?: string;
    category?: string;
    olderThanDays?: number;
    includeAnonymous?: boolean;
  }): Promise<{ deletedCount: number; deletedAnonymous: number; message: string }> {
    let whereClause = '1=1';
    let params: any[] = [];
    
    if (options.messageId) {
      whereClause += ' AND message_id = ?';
      params.push(options.messageId);
    }
    
    if (options.sourceBot) {
      whereClause += ' AND source_bot_id = ?';
      params.push(options.sourceBot);
    }
    
    if (options.category) {
      whereClause += ' AND category = ?';
      params.push(options.category);
    }
    
    if (options.olderThanDays) {
      whereClause += ' AND created_at < datetime("now", "-" || ? || " days")';
      params.push(options.olderThanDays);
    }
    
    // Delete from gossip_messages table
    const deleteMainStmt = this.db.prepare(`
      DELETE FROM gossip_messages WHERE ${whereClause}
    `);
    const mainResult = deleteMainStmt.run(...params);
    
    let anonymousDeleted = 0;
    
    // Delete from anonymous_gossip table if requested
    if (options.includeAnonymous) {
      let anonWhereClause = '1=1';
      let anonParams: any[] = [];
      
      if (options.messageId) {
        anonWhereClause += ' AND message_id = ?';
        anonParams.push(options.messageId);
      }
      
      if (options.category) {
        anonWhereClause += ' AND category = ?';
        anonParams.push(options.category);
      }
      
      if (options.olderThanDays) {
        anonWhereClause += ' AND created_at < datetime("now", "-" || ? || " days")';
        anonParams.push(options.olderThanDays);
      }
      
      const deleteAnonStmt = this.db.prepare(`
        DELETE FROM anonymous_gossip WHERE ${anonWhereClause}
      `);
      const anonResult = deleteAnonStmt.run(...anonParams);
      anonymousDeleted = anonResult.changes || 0;
    }
    
    this.logger.info('🗑️ Gossip messages deleted', {
      deletedCount: mainResult.changes,
      deletedAnonymous: anonymousDeleted,
      criteria: options
    });
    
    const totalDeleted = (mainResult.changes || 0) + anonymousDeleted;
    
    return {
      deletedCount: mainResult.changes || 0,
      deletedAnonymous: anonymousDeleted,
      message: `Deleted ${totalDeleted} message(s) (${mainResult.changes} regular, ${anonymousDeleted} anonymous)`
    };
  }
}
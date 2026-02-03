// BotNet Messaging Service
// Handles message sending/receiving with different behaviors for local vs federated nodes

import type Database from "better-sqlite3";
import type { BotNetConfig } from "../../index.js";
import { RateLimiter } from "../rate-limiter.js";
import { v4 as uuidv4 } from "uuid";

export interface BotNetMessage {
  id: string;
  from_domain: string;
  to_domain: string;
  content: string;
  message_type: string;
  status: 'pending' | 'delivered' | 'read' | 'responded';
  created_at: string;
  updated_at: string;
  metadata?: any;
}

export interface MessageResponse {
  id: string;
  message_id: string;
  from_domain: string;
  response_content: string;
  created_at: string;
  metadata?: any;
}

export class MessagingService {
  private rateLimiter: RateLimiter;

  // Database limits to prevent overfilling
  private readonly MAX_STORED_MESSAGES = 1000;
  private readonly MAX_MESSAGE_RESPONSES = 500;
  private readonly CLEANUP_MESSAGES_DAYS = 30;
  private readonly CLEANUP_RESPONSES_DAYS = 30;

  constructor(
    private database: Database.Database,
    private config: BotNetConfig,
    private logger: {
      info: (message: string, ...args: any[]) => void;
      error: (message: string, ...args: any[]) => void;
      warn: (message: string, ...args: any[]) => void;
    }
  ) {
    this.rateLimiter = new RateLimiter(logger, 60 * 1000, 10); // 10 messages per minute
  }

  /**
   * Check and enforce messaging limits
   */
  private checkMessagingLimits(): void {
    // Check total messages limit
    const messageCount = this.database.prepare(`
      SELECT COUNT(*) as count FROM messages
    `).get() as { count: number };
    
    if (messageCount.count >= this.MAX_STORED_MESSAGES) {
      // Auto-cleanup oldest messages if at limit
      const deleteOldest = this.database.prepare(`
        DELETE FROM messages 
        WHERE id IN (
          SELECT id FROM messages 
          ORDER BY created_at ASC 
          LIMIT 100
        )
      `);
      const deleted = deleteOldest.run();
      this.logger.info('🧹 Auto-cleaned oldest messages due to limit', {
        deleted: deleted.changes,
        limit: this.MAX_STORED_MESSAGES
      });
    }

    // Check message responses limit
    const responseCount = this.database.prepare(`
      SELECT COUNT(*) as count FROM message_responses
    `).get() as { count: number };
    
    if (responseCount.count >= this.MAX_MESSAGE_RESPONSES) {
      // Auto-cleanup oldest responses if at limit
      const deleteOldestResponses = this.database.prepare(`
        DELETE FROM message_responses 
        WHERE id IN (
          SELECT id FROM message_responses 
          ORDER BY created_at ASC 
          LIMIT 50
        )
      `);
      const deletedResponses = deleteOldestResponses.run();
      this.logger.info('🧹 Auto-cleaned oldest message responses due to limit', {
        deleted: deletedResponses.changes,
        limit: this.MAX_MESSAGE_RESPONSES
      });
    }
  }

  /**
   * Cleanup old messages and responses
   */
  private cleanupOldData(): void {
    // Delete old messages (older than cleanup days)
    const deleteOldMessages = this.database.prepare(`
      DELETE FROM messages 
      WHERE created_at < datetime('now', '-' || ? || ' days')
    `);
    const messagesDeleted = deleteOldMessages.run(this.CLEANUP_MESSAGES_DAYS);

    // Delete old message responses (older than cleanup days)
    const deleteOldResponses = this.database.prepare(`
      DELETE FROM message_responses 
      WHERE created_at < datetime('now', '-' || ? || ' days')
    `);
    const responsesDeleted = deleteOldResponses.run(this.CLEANUP_RESPONSES_DAYS);

    if (messagesDeleted.changes || responsesDeleted.changes) {
      this.logger.info('🧹 Messaging cleanup completed', {
        messagesDeleted: messagesDeleted.changes,
        responsesDeleted: responsesDeleted.changes,
        cleanupDays: this.CLEANUP_MESSAGES_DAYS
      });
    }
  }

  /**
   * Determine if a domain is local (no dots) or federated (botnet.*)
   * Domains with dots but without 'botnet.' prefix are invalid
   */
  private determineNodeType(domain: string): 'local' | 'federated' | 'invalid' {
    if (!domain.includes('.')) {
      return 'local'; // Names like "TestBot", "Alice"
    }
    if (domain.startsWith('botnet.')) {
      return 'federated'; // Domains like "botnet.example.com"
    }
    // Domains with dots but without botnet. prefix are invalid for federation
    return 'invalid';
  }

  /**
   * Send message to another domain/bot
   */
  async sendMessage(
    toDomain: string, 
    content: string, 
    messageType: string = 'chat',
    clientIP?: string
  ): Promise<{ messageId: string; status: string; requiresManualCheck?: boolean }> {
    
    // Rate limiting
    const rateLimitKey = clientIP || this.config.botDomain;
    if (!this.rateLimiter.checkRateLimit(rateLimitKey, 'sendMessage')) {
      throw new Error('Rate limit exceeded for message sending. Please try again later.');
    }

    // Cleanup old data and check limits before creating new messages
    this.cleanupOldData();
    this.checkMessagingLimits();

    // Validate target domain
    const toNodeType = this.determineNodeType(toDomain);
    if (toNodeType === 'invalid') {
      this.logger.warn('🚫 Cannot send message to invalid domain', {
        toDomain,
        reason: 'Target domain has dots but missing required botnet. prefix'
      });
      throw new Error(`Invalid target domain: ${toDomain}. BotNet messaging requires 'botnet.' prefix for domains.`);
    }

    const messageId = uuidv4();
    const fromDomain = this.config.botDomain;
    const fromNodeType = this.determineNodeType(fromDomain);

    // Store message locally
    const stmt = this.database.prepare(`
      INSERT INTO messages (
        message_id, from_domain, to_domain, content, message_type, status, metadata
      ) VALUES (?, ?, ?, ?, ?, 'pending', ?)
    `);

    const metadata = JSON.stringify({
      toNodeType,
      fromNodeType,
      requiresManualCheck: fromNodeType === 'local' && toNodeType === 'federated',
      sentAt: new Date().toISOString()
    });

    stmt.run(messageId, fromDomain, toDomain, content, messageType, metadata);

    this.logger.info('📤 Message sent', {
      messageId,
      fromDomain,
      toDomain,
      messageType,
      toNodeType,
      fromNodeType
    });

    // Different behavior based on node types
    if (fromNodeType === 'local' && toNodeType === 'federated') {
      // Local sender to federated receiver - must check manually for response
      return {
        messageId,
        status: 'sent_to_federated',
        requiresManualCheck: true
      };
    } else if (fromNodeType === 'federated' && toNodeType === 'local') {
      // Federated sender to local receiver - deliver immediately
      return {
        messageId,
        status: 'delivered_locally'
      };
    } else if (fromNodeType === 'local' && toNodeType === 'local') {
      // Local to local - immediate delivery
      return {
        messageId,
        status: 'delivered_locally'
      };
    } else {
      // Federated to federated - network routing required
      return {
        messageId,
        status: 'sent_via_network'
      };
    }
  }

  /**
   * Review messages (different behavior for local vs federated)
   */
  async reviewMessages(domain?: string, includeResponses: boolean = true, clientIP?: string): Promise<{
    messages: BotNetMessage[];
    responses?: MessageResponse[];
    requiresRemoteCheck?: boolean;
  }> {
    
    // Rate limiting
    const rateLimitKey = clientIP || this.config.botDomain;
    if (!this.rateLimiter.checkRateLimit(rateLimitKey, 'reviewMessages')) {
      throw new Error('Rate limit exceeded for message review. Please try again later.');
    }

    const currentDomain = domain || this.config.botDomain;
    const nodeType = this.determineNodeType(currentDomain);

    // Get incoming messages
    const messageStmt = this.database.prepare(`
      SELECT * FROM messages 
      WHERE to_domain = ?
      ORDER BY created_at DESC
      LIMIT 50
    `);
    const messages = messageStmt.all(currentDomain) as BotNetMessage[];

    let responses: MessageResponse[] = [];
    let requiresRemoteCheck = false;

    if (includeResponses) {
      // Get responses to our outgoing messages
      const responseStmt = this.database.prepare(`
        SELECT mr.*, m.to_domain, m.content as original_content
        FROM message_responses mr
        JOIN messages m ON mr.message_id = m.message_id
        WHERE m.from_domain = ?
        ORDER BY mr.created_at DESC
        LIMIT 50
      `);
      responses = responseStmt.all(currentDomain) as MessageResponse[];

      // Check if we need to fetch from remote nodes
      const pendingFederatedStmt = this.database.prepare(`
        SELECT COUNT(*) as count FROM messages 
        WHERE from_domain = ? AND JSON_EXTRACT(metadata, '$.toNodeType') = 'federated' 
        AND status IN ('pending', 'delivered')
      `);
      const pendingCount = (pendingFederatedStmt.get(currentDomain) as any).count;
      
      if (pendingCount > 0 && nodeType === 'local') {
        requiresRemoteCheck = true;
      }
    }

    this.logger.info('📨 Messages reviewed', {
      currentDomain,
      nodeType,
      messageCount: messages.length,
      responseCount: responses.length,
      requiresRemoteCheck
    });

    return {
      messages,
      responses,
      requiresRemoteCheck
    };
  }

  /**
   * Set response to a received message
   */
  async setResponse(
    messageId: string,
    responseContent: string,
    clientIP?: string
  ): Promise<{ responseId: string; status: string }> {
    
    // Rate limiting
    const rateLimitKey = clientIP || this.config.botDomain;
    if (!this.rateLimiter.checkRateLimit(rateLimitKey, 'setResponse')) {
      throw new Error('Rate limit exceeded for setting response. Please try again later.');
    }

    // Find the original message
    const messageStmt = this.database.prepare(`
      SELECT * FROM messages WHERE message_id = ?
    `);
    const message = messageStmt.get(messageId) as BotNetMessage | undefined;

    if (!message) {
      throw new Error('Message not found');
    }

    if (message.to_domain !== this.config.botDomain) {
      throw new Error('Cannot respond to message not addressed to this domain');
    }

    const responseId = uuidv4();
    const fromDomain = this.config.botDomain;

    // Store response
    const responseStmt = this.database.prepare(`
      INSERT INTO message_responses (
        response_id, message_id, from_domain, response_content, metadata
      ) VALUES (?, ?, ?, ?, ?)
    `);

    const metadata = JSON.stringify({
      originalFrom: message.from_domain,
      respondedAt: new Date().toISOString()
    });

    responseStmt.run(responseId, messageId, fromDomain, responseContent, metadata);

    // Update message status
    this.database.prepare(`
      UPDATE messages SET status = 'responded', updated_at = CURRENT_TIMESTAMP
      WHERE message_id = ?
    `).run(messageId);

    this.logger.info('📝 Response set', {
      responseId,
      messageId,
      fromDomain,
      originalFrom: message.from_domain
    });

    return {
      responseId,
      status: 'response_set'
    };
  }

  /**
   * Delete messages by criteria (with rate limiting)
   */
  async deleteMessages(options: {
    messageId?: string;
    fromDomain?: string;
    toDomain?: string;
    messageType?: string;
    olderThanDays?: number;
  }, clientIP?: string): Promise<{ deletedCount: number; message: string }> {
    
    // Rate limiting for deletion
    const rateLimitKey = clientIP || this.config.botDomain;
    if (!this.rateLimiter.checkRateLimit(rateLimitKey, 'deleteMessages')) {
      throw new Error('Rate limit exceeded for message deletion. Please try again later.');
    }

    let whereClause = '1=1';
    let params: any[] = [];
    
    if (options.messageId) {
      whereClause += ' AND message_id = ?';
      params.push(options.messageId);
    }
    
    if (options.fromDomain) {
      whereClause += ' AND from_domain = ?';
      params.push(options.fromDomain);
    }

    if (options.toDomain) {
      whereClause += ' AND to_domain = ?';
      params.push(options.toDomain);
    }
    
    if (options.messageType) {
      whereClause += ' AND message_type = ?';
      params.push(options.messageType);
    }
    
    if (options.olderThanDays) {
      whereClause += ' AND created_at < datetime("now", "-" || ? || " days")';
      params.push(options.olderThanDays);
    }
    
    const deleteStmt = this.database.prepare(`
      DELETE FROM messages WHERE ${whereClause}
    `);
    const result = deleteStmt.run(...params);
    
    this.logger.info('🗑️ Messages deleted', {
      deletedCount: result.changes,
      criteria: options
    });
    
    return {
      deletedCount: result.changes || 0,
      message: `Deleted ${result.changes} message(s)`
    };
  }

  /**
   * Receive incoming message from remote domain (MCP federation)
   */
  async receiveMessage(fromDomain: string, toDomain: string, content: string, messageType: string = 'chat', clientIP?: string): Promise<{ messageId: string; status: string }> {
    // Rate limiting
    const rateLimitKey = clientIP || fromDomain;
    if (!this.rateLimiter.checkRateLimit(rateLimitKey, 'receiveMessage')) {
      throw new Error('Rate limit exceeded for receiving messages. Please try again later.');
    }

    // Validate source domain
    const nodeType = this.determineNodeType(fromDomain);
    if (nodeType === 'invalid') {
      this.logger.warn('🚫 Rejected message from invalid domain', {
        fromDomain,
        reason: 'Source domain has dots but missing required botnet. prefix'
      });
      throw new Error(`Invalid source domain: ${fromDomain}. BotNet requires 'botnet.' prefix for federation domains.`);
    }

    const messageId = uuidv4();
    
    // Store incoming message
    const insertStmt = this.database.prepare(`
      INSERT INTO messages (
        message_id, from_domain, to_domain, content, message_type, 
        status, metadata
      ) VALUES (?, ?, ?, ?, ?, ?, ?)
    `);
    
    const metadata = JSON.stringify({
      receivedAt: new Date().toISOString(),
      nodeType,
      source: 'mcp_federation',
      clientIP: clientIP
    });
    
    insertStmt.run(
      messageId,
      fromDomain,
      toDomain,
      content,
      messageType,
      'delivered', // Mark as delivered since we received it
      metadata
    );
    
    this.logger.info('📨 Message received via MCP', {
      messageId,
      fromDomain,
      toDomain,
      messageType,
      nodeType,
      contentLength: content.length
    });
    
    return {
      messageId,
      status: 'received'
    };
  }
}
import { describe, it, expect, beforeEach, jest } from '@jest/globals';
import Database from 'better-sqlite3';
import { BotNetService } from './service.js';
import type { Logger } from './logger.js';
import type { BotNetConfig } from '../index.js';

// Mock logger
const mockLogger: Logger = {
  debug: jest.fn(),
  info: jest.fn(),
  warn: jest.fn(),
  error: jest.fn(),
  child: jest.fn(() => mockLogger),
} as any;

// Mock runtime
const mockRuntime = {
  config: {
    loadConfig: jest.fn(),
  },
  paths: {
    data: '/tmp',
    resolve: jest.fn((base, path) => `${base}/${path}`),
  },
};

describe('BotNetService', () => {
  let service: BotNetService;
  let db: Database.Database;
  
  const testConfig: BotNetConfig = {
    botName: 'TestBot',
    botDomain: 'test.example.com',
    botDescription: 'Test bot',
    capabilities: ['test'],
    tier: 'standard',
    databasePath: ':memory:',
    httpPort: 8080,
    logLevel: 'info',
  };
  
  beforeEach(() => {
    // Create in-memory database
    db = new Database(':memory:');
    
    // Initialize service
    service = new BotNetService({
      database: db,
      config: testConfig,
      logger: mockLogger,
      runtime: mockRuntime as any,
    });
  });
  
  afterEach(() => {
    db.close();
  });
  
  describe('getBotProfile', () => {
    it('should return bot profile with correct information', async () => {
      const profile = await service.getBotProfile();
      
      expect(profile).toEqual({
        id: 'TestBot@test.example.com',
        name: 'TestBot',
        domain: 'test.example.com',
        description: 'Test bot',
        capabilities: ['test'],
        tier: 'standard',
        version: '1.0.0',
        protocol_version: '1.0',
        endpoints: {
          mcp: '/api/botnet/mcp',
          profile: '/api/botnet/profile',
          health: '/api/botnet/health',
          friendship: '/api/botnet/friendship',
          gossip: '/api/botnet/gossip',
        },
      });
    });
  });
  
  describe('getHealthStatus', () => {
    it('should return healthy status when database is working', async () => {
      const health = await service.getHealthStatus();
      
      expect(health.status).toBe('healthy');
      expect(health.version).toBe('1.0.0');
      expect(health.checks.database).toBe('ok');
    });
  });
});
// Universal rate limiter for all BotNet operations

export class RateLimiter {
  private rateLimitMap: Map<string, { count: number; resetAt: number }> = new Map();
  private readonly RATE_LIMIT_WINDOW: number;
  private readonly RATE_LIMIT_MAX: number;

  constructor(
    private logger: {
      warn: (message: string, ...args: any[]) => void;
    },
    private windowMs: number = 60 * 1000,
    private maxRequests: number = 5
  ) {
    this.RATE_LIMIT_WINDOW = windowMs;
    this.RATE_LIMIT_MAX = maxRequests;
  }

  /**
   * Check rate limit for identifier (IP or domain)
   */
  checkRateLimit(identifier: string, operation: string = 'request'): boolean {
    const now = Date.now();
    const rateLimitData = this.rateLimitMap.get(identifier);

    if (!rateLimitData || now > rateLimitData.resetAt) {
      // Reset or first request
      this.rateLimitMap.set(identifier, { count: 1, resetAt: now + this.RATE_LIMIT_WINDOW });
      return true;
    }

    if (rateLimitData.count >= this.RATE_LIMIT_MAX) {
      this.logger.warn('🚫 Rate limit exceeded', { 
        identifier, 
        operation, 
        count: rateLimitData.count,
        window: this.RATE_LIMIT_WINDOW / 1000 + 's'
      });
      return false;
    }

    rateLimitData.count++;
    return true;
  }

  /**
   * Get current rate limit status for identifier
   */
  getRateLimitStatus(identifier: string): { remaining: number; resetAt: number } {
    const now = Date.now();
    const rateLimitData = this.rateLimitMap.get(identifier);

    if (!rateLimitData || now > rateLimitData.resetAt) {
      return { remaining: this.RATE_LIMIT_MAX, resetAt: now + this.RATE_LIMIT_WINDOW };
    }

    return { 
      remaining: Math.max(0, this.RATE_LIMIT_MAX - rateLimitData.count),
      resetAt: rateLimitData.resetAt
    };
  }

  /**
   * Clean up expired rate limit entries (garbage collection)
   */
  cleanup(): void {
    const now = Date.now();
    for (const [key, data] of this.rateLimitMap.entries()) {
      if (now > data.resetAt) {
        this.rateLimitMap.delete(key);
      }
    }
  }
}
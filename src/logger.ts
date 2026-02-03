export interface ILogger {
  debug(message: string, meta?: any): void;
  info(message: string, meta?: any): void;
  warn(message: string, meta?: any): void;
  error(message: string, meta?: any): void;
}

export class Logger implements ILogger {
  public baseLogger: ILogger;
  
  constructor(baseLogger: ILogger) {
    this.baseLogger = baseLogger;
  }

  debug(message: string, meta?: any): void {
    this.baseLogger.debug(`[BotNet] ${message}`, meta);
  }

  info(message: string, meta?: any): void {
    this.baseLogger.info(`[BotNet] ${message}`, meta);
  }

  warn(message: string, meta?: any): void {
    this.baseLogger.warn(`[BotNet] ${message}`, meta);
  }

  error(message: string, meta?: any): void {
    this.baseLogger.error(`[BotNet] ${message}`, meta);
  }

  child(prefix: string): Logger {
    return new Logger({
      debug: (msg, meta) => this.debug(`[${prefix}] ${msg}`, meta),
      info: (msg, meta) => this.info(`[${prefix}] ${msg}`, meta),
      warn: (msg, meta) => this.warn(`[${prefix}] ${msg}`, meta),
      error: (msg, meta) => this.error(`[${prefix}] ${msg}`, meta),
    });
  }
}
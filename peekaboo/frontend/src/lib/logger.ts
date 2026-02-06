/**
 * Logger - A simple logging utility with configurable log levels
 *
 * Log level is controlled by import.meta.env.VITE_LOG_LEVEL:
 * - 'debug': All logs (debug, info, warn, error)
 * - 'info': info, warn, error
 * - 'warn': warn, error
 * - 'error': error only
 * - 'silent': No logs
 *
 * Default: 'info' in production, 'debug' in development
 *
 * Log forwarding to backend (development only):
 * Set VITE_FORWARD_LOGS=true to also send logs to POST /api/log.
 * The backend must have FORWARD_FRONTEND_LOGS=true to accept them.
 */

export type LogLevel = 'debug' | 'info' | 'warn' | 'error' | 'silent';

const LOG_LEVELS: Record<LogLevel, number> = {
  debug: 0,
  info: 1,
  warn: 2,
  error: 3,
  silent: 4,
};

function getConfiguredLevel(): LogLevel {
  // Try to read from environment variable
  const envLevel = (import.meta.env?.VITE_LOG_LEVEL || '').toLowerCase();
  if (envLevel in LOG_LEVELS) {
    return envLevel as LogLevel;
  }

  // Default based on mode
  if (import.meta.env?.DEV) {
    return 'debug';
  }
  return 'info';
}

function isForwardingEnabled(): boolean {
  return (import.meta.env?.VITE_FORWARD_LOGS || '').toLowerCase() === 'true';
}

let currentLevel: LogLevel = getConfiguredLevel();
let forwardLogs: boolean = isForwardingEnabled();

/**
 * Forward a log message to the backend via POST /api/log.
 * Fire-and-forget - errors are silently ignored to avoid infinite loops.
 */
function forwardToBackend(level: string, message: string): void {
  if (!forwardLogs) return;
  try {
    fetch('/api/log', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ level, message }),
    }).catch(() => {
      // Silently ignore - we can't log a forwarding failure without recursion
    });
  } catch {
    // Silently ignore synchronous errors (e.g., fetch not available)
  }
}

/**
 * Set the current log level
 */
export function setLogLevel(level: LogLevel): void {
  currentLevel = level;
}

/**
 * Get the current log level
 */
export function getLogLevel(): LogLevel {
  return currentLevel;
}

/**
 * Enable or disable log forwarding to the backend
 */
export function setForwardLogs(enabled: boolean): void {
  forwardLogs = enabled;
}

function shouldLog(level: LogLevel): boolean {
  return LOG_LEVELS[level] >= LOG_LEVELS[currentLevel];
}

/**
 * Log a debug message (only shown when log level is 'debug')
 */
export function debug(message: string, ...args: unknown[]): void {
  if (shouldLog('debug')) {
    console.debug(`[DEBUG] ${message}`, ...args);
    forwardToBackend('debug', message);
  }
}

/**
 * Log an info message
 */
export function info(message: string, ...args: unknown[]): void {
  if (shouldLog('info')) {
    console.info(`[INFO] ${message}`, ...args);
    forwardToBackend('info', message);
  }
}

/**
 * Log a warning message
 */
export function warn(message: string, ...args: unknown[]): void {
  if (shouldLog('warn')) {
    console.warn(`[WARN] ${message}`, ...args);
    forwardToBackend('warn', message);
  }
}

/**
 * Log an error message
 */
export function error(message: string, ...args: unknown[]): void {
  if (shouldLog('error')) {
    console.error(`[ERROR] ${message}`, ...args);
    forwardToBackend('error', message);
  }
}

/**
 * Default logger export with all methods
 */
export const logger = {
  debug,
  info,
  warn,
  error,
  setLogLevel,
  getLogLevel,
  setForwardLogs,
};

export default logger;

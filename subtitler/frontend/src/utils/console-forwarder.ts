/**
 * Console log forwarder for development mode.
 *
 * This module intercepts console.log, console.warn, console.error, console.info,
 * and console.debug calls in the browser and forwards them to the backend
 * /api/log endpoint for easier debugging during development.
 *
 * This should only be enabled in development mode (not production).
 */

interface LogPayload {
  level: 'log' | 'warn' | 'error' | 'info' | 'debug';
  message: string;
  args?: unknown[];
  url: string;
  line?: number;
  column?: number;
}

// Store original console methods
const originalConsole = {
  log: console.log.bind(console),
  warn: console.warn.bind(console),
  error: console.error.bind(console),
  info: console.info.bind(console),
  debug: console.debug.bind(console),
};

// Track if we're currently sending a log to prevent infinite loops
let isSending = false;

// Queue for logs that occur while a send is in progress
const logQueue: LogPayload[] = [];
let isProcessingQueue = false;

/**
 * Formats console arguments into a string message
 */
function formatArgs(args: unknown[]): string {
  return args
    .map((arg) => {
      if (typeof arg === 'string') {
        return arg;
      }
      if (arg === undefined) {
        return 'undefined';
      }
      if (arg instanceof Error) {
        return `${arg.name}: ${arg.message}\n${arg.stack || ''}`;
      }
      try {
        return JSON.stringify(arg, null, 2);
      } catch {
        return String(arg);
      }
    })
    .join(' ');
}

/**
 * Sends a log entry to the backend
 */
async function sendLog(payload: LogPayload): Promise<void> {
  if (isSending) {
    // Queue the log to prevent stack overflow
    logQueue.push(payload);
    return;
  }

  isSending = true;
  try {
    await fetch('/api/log', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(payload),
    });
  } catch {
    // Silently fail - we don't want logging failures to break the app
  } finally {
    isSending = false;
    processQueue();
  }
}

/**
 * Process queued logs
 */
async function processQueue(): Promise<void> {
  if (isProcessingQueue || logQueue.length === 0) {
    return;
  }

  isProcessingQueue = true;
  while (logQueue.length > 0) {
    const payload = logQueue.shift();
    if (payload) {
      await sendLog(payload);
    }
  }
  isProcessingQueue = false;
}

/**
 * Creates an interceptor function for a specific console level
 */
function createInterceptor(level: LogPayload['level']): (...args: unknown[]) => void {
  return function (...args: unknown[]): void {
    // Always call the original console method first
    originalConsole[level](...args);

    // Format and send to backend
    const message = formatArgs(args);
    const payload: LogPayload = {
      level,
      message,
      url: typeof window !== 'undefined' ? window.location.href : '',
    };

    // Try to get line/column from stack trace for errors
    if (level === 'error') {
      const stack = new Error().stack;
      if (stack) {
        const lines = stack.split('\n');
        // Skip first two lines (Error and this function)
        if (lines.length > 2) {
          const match = lines[2].match(/:(\d+):(\d+)/);
          if (match) {
            payload.line = parseInt(match[1], 10);
            payload.column = parseInt(match[2], 10);
          }
        }
      }
    }

    sendLog(payload);
  };
}

/**
 * Installs console interceptors for development mode.
 * Call this once when the app initializes.
 *
 * @param isDev - Whether the app is running in development mode
 */
export function installConsoleForwarder(isDev: boolean = false): void {
  // Only install in development mode and in browser environment
  if (!isDev || typeof window === 'undefined') {
    return;
  }

  console.log = createInterceptor('log');
  console.warn = createInterceptor('warn');
  console.error = createInterceptor('error');
  console.info = createInterceptor('info');
  console.debug = createInterceptor('debug');

  // Also capture unhandled errors
  window.addEventListener('error', (event) => {
    const payload: LogPayload = {
      level: 'error',
      message: `Unhandled error: ${event.message}`,
      url: event.filename || window.location.href,
      line: event.lineno,
      column: event.colno,
    };
    sendLog(payload);
  });

  // Capture unhandled promise rejections
  window.addEventListener('unhandledrejection', (event) => {
    const message =
      event.reason instanceof Error
        ? `Unhandled promise rejection: ${event.reason.message}\n${event.reason.stack || ''}`
        : `Unhandled promise rejection: ${String(event.reason)}`;

    const payload: LogPayload = {
      level: 'error',
      message,
      url: window.location.href,
    };
    sendLog(payload);
  });

  // Log that the forwarder has been installed (using original to avoid recursion)
  originalConsole.log('[Console Forwarder] Installed - logs will be forwarded to backend');
}

/**
 * Restores original console methods.
 * Useful for testing or cleanup.
 */
export function uninstallConsoleForwarder(): void {
  console.log = originalConsole.log;
  console.warn = originalConsole.warn;
  console.error = originalConsole.error;
  console.info = originalConsole.info;
  console.debug = originalConsole.debug;
}

// Export for testing
export { formatArgs, originalConsole };

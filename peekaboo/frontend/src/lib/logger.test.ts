import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { logger, setLogLevel, getLogLevel, debug, info, warn, error, LogLevel, setForwardLogs } from './logger';

describe('logger', () => {
  // Save original console methods
  const originalConsole = {
    debug: console.debug,
    info: console.info,
    warn: console.warn,
    error: console.error,
  };

  let mockDebug: ReturnType<typeof vi.fn>;
  let mockInfo: ReturnType<typeof vi.fn>;
  let mockWarn: ReturnType<typeof vi.fn>;
  let mockError: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    // Mock console methods
    mockDebug = vi.fn();
    mockInfo = vi.fn();
    mockWarn = vi.fn();
    mockError = vi.fn();

    console.debug = mockDebug;
    console.info = mockInfo;
    console.warn = mockWarn;
    console.error = mockError;

    // Reset log level to debug for consistent testing
    setLogLevel('debug');
    // Disable forwarding by default in tests
    setForwardLogs(false);
  });

  afterEach(() => {
    // Restore original console methods
    console.debug = originalConsole.debug;
    console.info = originalConsole.info;
    console.warn = originalConsole.warn;
    console.error = originalConsole.error;
  });

  describe('setLogLevel and getLogLevel', () => {
    it('gets and sets log level', () => {
      setLogLevel('warn');
      expect(getLogLevel()).toBe('warn');

      setLogLevel('error');
      expect(getLogLevel()).toBe('error');

      setLogLevel('debug');
      expect(getLogLevel()).toBe('debug');
    });
  });

  describe('debug level', () => {
    beforeEach(() => {
      setLogLevel('debug');
    });

    it('logs all messages at debug level', () => {
      debug('debug message');
      info('info message');
      warn('warn message');
      error('error message');

      expect(mockDebug).toHaveBeenCalledWith('[DEBUG] debug message');
      expect(mockInfo).toHaveBeenCalledWith('[INFO] info message');
      expect(mockWarn).toHaveBeenCalledWith('[WARN] warn message');
      expect(mockError).toHaveBeenCalledWith('[ERROR] error message');
    });

    it('passes additional arguments', () => {
      debug('test', { foo: 'bar' }, 123);
      expect(mockDebug).toHaveBeenCalledWith('[DEBUG] test', { foo: 'bar' }, 123);
    });
  });

  describe('info level', () => {
    beforeEach(() => {
      setLogLevel('info');
    });

    it('suppresses debug messages', () => {
      debug('debug message');
      expect(mockDebug).not.toHaveBeenCalled();
    });

    it('logs info, warn, and error', () => {
      info('info message');
      warn('warn message');
      error('error message');

      expect(mockInfo).toHaveBeenCalled();
      expect(mockWarn).toHaveBeenCalled();
      expect(mockError).toHaveBeenCalled();
    });
  });

  describe('warn level', () => {
    beforeEach(() => {
      setLogLevel('warn');
    });

    it('suppresses debug and info messages', () => {
      debug('debug message');
      info('info message');

      expect(mockDebug).not.toHaveBeenCalled();
      expect(mockInfo).not.toHaveBeenCalled();
    });

    it('logs warn and error', () => {
      warn('warn message');
      error('error message');

      expect(mockWarn).toHaveBeenCalled();
      expect(mockError).toHaveBeenCalled();
    });
  });

  describe('error level', () => {
    beforeEach(() => {
      setLogLevel('error');
    });

    it('suppresses debug, info, and warn messages', () => {
      debug('debug message');
      info('info message');
      warn('warn message');

      expect(mockDebug).not.toHaveBeenCalled();
      expect(mockInfo).not.toHaveBeenCalled();
      expect(mockWarn).not.toHaveBeenCalled();
    });

    it('logs error', () => {
      error('error message');
      expect(mockError).toHaveBeenCalled();
    });
  });

  describe('silent level', () => {
    beforeEach(() => {
      setLogLevel('silent');
    });

    it('suppresses all messages', () => {
      debug('debug message');
      info('info message');
      warn('warn message');
      error('error message');

      expect(mockDebug).not.toHaveBeenCalled();
      expect(mockInfo).not.toHaveBeenCalled();
      expect(mockWarn).not.toHaveBeenCalled();
      expect(mockError).not.toHaveBeenCalled();
    });
  });

  describe('logger object', () => {
    it('exports all methods', () => {
      expect(typeof logger.debug).toBe('function');
      expect(typeof logger.info).toBe('function');
      expect(typeof logger.warn).toBe('function');
      expect(typeof logger.error).toBe('function');
      expect(typeof logger.setLogLevel).toBe('function');
      expect(typeof logger.getLogLevel).toBe('function');
      expect(typeof logger.setForwardLogs).toBe('function');
    });

    it('methods work through logger object', () => {
      logger.setLogLevel('debug');
      logger.debug('test');
      expect(mockDebug).toHaveBeenCalled();
    });
  });

  describe('log forwarding', () => {
    let mockFetch: ReturnType<typeof vi.fn>;

    beforeEach(() => {
      mockFetch = vi.fn().mockResolvedValue({ ok: true });
      vi.stubGlobal('fetch', mockFetch);
      setForwardLogs(true);
      setLogLevel('debug');
    });

    afterEach(() => {
      setForwardLogs(false);
      vi.unstubAllGlobals();
    });

    it('forwards error logs to backend when enabled', () => {
      error('test error');
      expect(mockFetch).toHaveBeenCalledWith('/api/log', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ level: 'error', message: 'test error' }),
      });
    });

    it('forwards warn logs to backend when enabled', () => {
      warn('test warning');
      expect(mockFetch).toHaveBeenCalledWith('/api/log', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ level: 'warn', message: 'test warning' }),
      });
    });

    it('forwards info logs to backend when enabled', () => {
      info('test info');
      expect(mockFetch).toHaveBeenCalledWith('/api/log', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ level: 'info', message: 'test info' }),
      });
    });

    it('forwards debug logs to backend when enabled', () => {
      debug('test debug');
      expect(mockFetch).toHaveBeenCalledWith('/api/log', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ level: 'debug', message: 'test debug' }),
      });
    });

    it('does not forward when forwarding is disabled', () => {
      setForwardLogs(false);
      error('test error');
      expect(mockFetch).not.toHaveBeenCalled();
    });

    it('does not forward when log level suppresses the message', () => {
      setLogLevel('error');
      debug('suppressed debug');
      expect(mockFetch).not.toHaveBeenCalled();
    });

    it('silently ignores fetch errors', () => {
      mockFetch.mockRejectedValue(new Error('network error'));
      // Should not throw
      expect(() => error('test error')).not.toThrow();
    });
  });
});

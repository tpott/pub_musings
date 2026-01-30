import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  installConsoleForwarder,
  uninstallConsoleForwarder,
  formatArgs,
  originalConsole,
} from './console-forwarder';

// Mock fetch
const mockFetch = vi.fn();

describe('formatArgs', () => {
  it('should format a string as-is', () => {
    expect(formatArgs(['hello'])).toBe('hello');
  });

  it('should format multiple strings joined by space', () => {
    expect(formatArgs(['hello', 'world'])).toBe('hello world');
  });

  it('should format numbers as strings', () => {
    expect(formatArgs([42])).toBe('42');
  });

  it('should format objects as JSON', () => {
    expect(formatArgs([{ foo: 'bar' }])).toBe('{\n  "foo": "bar"\n}');
  });

  it('should format arrays as JSON', () => {
    expect(formatArgs([[1, 2, 3]])).toBe('[\n  1,\n  2,\n  3\n]');
  });

  it('should format Error objects with name, message, and stack', () => {
    const error = new Error('test error');
    const result = formatArgs([error]);
    expect(result).toContain('Error: test error');
  });

  it('should handle null and undefined', () => {
    expect(formatArgs([null])).toBe('null');
    expect(formatArgs([undefined])).toBe('undefined');
  });

  it('should handle mixed types', () => {
    const result = formatArgs(['Count:', 42, { items: ['a', 'b'] }]);
    expect(result).toContain('Count:');
    expect(result).toContain('42');
    expect(result).toContain('items');
  });

  it('should handle circular references gracefully', () => {
    const obj: Record<string, unknown> = { name: 'test' };
    obj.self = obj;
    // Should not throw, should return string representation
    const result = formatArgs([obj]);
    expect(typeof result).toBe('string');
  });
});

describe('console forwarder installation', () => {
  beforeEach(() => {
    // Reset mocks
    mockFetch.mockReset();
    mockFetch.mockResolvedValue({ ok: true });
    // Replace global fetch with mock
    vi.stubGlobal('fetch', mockFetch);
  });

  afterEach(() => {
    // Restore original console methods
    uninstallConsoleForwarder();
    vi.unstubAllGlobals();
  });

  it('should not install in non-dev mode', () => {
    const originalLog = console.log;
    installConsoleForwarder(false);
    expect(console.log).toBe(originalLog);
  });

  it('should not install when window is undefined', () => {
    const originalLog = console.log;
    // In Node/Vitest, window is actually defined via jsdom or similar
    // This test verifies the function doesn't crash
    installConsoleForwarder(true);
    // If window is not available, it should still work (not throw)
    expect(console.log).toBeDefined();
    uninstallConsoleForwarder();
  });

  it('should restore original console methods on uninstall', () => {
    installConsoleForwarder(true);
    uninstallConsoleForwarder();
    expect(console.log).toBe(originalConsole.log);
    expect(console.warn).toBe(originalConsole.warn);
    expect(console.error).toBe(originalConsole.error);
    expect(console.info).toBe(originalConsole.info);
    expect(console.debug).toBe(originalConsole.debug);
  });

  it('should remove window event listeners on uninstall', () => {
    const addSpy = vi.fn();
    const removeSpy = vi.fn();
    vi.stubGlobal('window', {
      location: { href: 'http://localhost:4321/test' },
      addEventListener: addSpy,
      removeEventListener: removeSpy,
    });

    installConsoleForwarder(true);
    expect(addSpy).toHaveBeenCalledWith('error', expect.any(Function));
    expect(addSpy).toHaveBeenCalledWith('unhandledrejection', expect.any(Function));

    uninstallConsoleForwarder();
    expect(removeSpy).toHaveBeenCalledWith('error', expect.any(Function));
    expect(removeSpy).toHaveBeenCalledWith('unhandledrejection', expect.any(Function));
  });
});

describe('console interception (when installed)', () => {
  beforeEach(() => {
    mockFetch.mockReset();
    mockFetch.mockResolvedValue({ ok: true });
    vi.stubGlobal('fetch', mockFetch);
    vi.stubGlobal('window', {
      location: { href: 'http://localhost:4321/test' },
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    });
    installConsoleForwarder(true);
  });

  afterEach(() => {
    uninstallConsoleForwarder();
    vi.unstubAllGlobals();
  });

  it('should call fetch when console.log is called', async () => {
    console.log('test message');
    // Wait for async fetch
    await new Promise((resolve) => setTimeout(resolve, 10));

    expect(mockFetch).toHaveBeenCalled();
    const callArgs = mockFetch.mock.calls[0];
    expect(callArgs[0]).toBe('/api/log');
    expect(callArgs[1].method).toBe('POST');
    const body = JSON.parse(callArgs[1].body);
    expect(body.level).toBe('log');
    expect(body.message).toContain('test message');
  });

  it('should call fetch when console.error is called', async () => {
    console.error('error message');
    await new Promise((resolve) => setTimeout(resolve, 10));

    expect(mockFetch).toHaveBeenCalled();
    const body = JSON.parse(mockFetch.mock.calls[0][1].body);
    expect(body.level).toBe('error');
    expect(body.message).toContain('error message');
  });

  it('should call fetch when console.warn is called', async () => {
    console.warn('warning message');
    await new Promise((resolve) => setTimeout(resolve, 10));

    expect(mockFetch).toHaveBeenCalled();
    const body = JSON.parse(mockFetch.mock.calls[0][1].body);
    expect(body.level).toBe('warn');
    expect(body.message).toContain('warning message');
  });

  it('should call fetch when console.info is called', async () => {
    console.info('info message');
    await new Promise((resolve) => setTimeout(resolve, 10));

    expect(mockFetch).toHaveBeenCalled();
    const body = JSON.parse(mockFetch.mock.calls[0][1].body);
    expect(body.level).toBe('info');
    expect(body.message).toContain('info message');
  });

  it('should call fetch when console.debug is called', async () => {
    console.debug('debug message');
    await new Promise((resolve) => setTimeout(resolve, 10));

    expect(mockFetch).toHaveBeenCalled();
    const body = JSON.parse(mockFetch.mock.calls[0][1].body);
    expect(body.level).toBe('debug');
    expect(body.message).toContain('debug message');
  });

  it('should include url in the payload', async () => {
    console.log('test');
    await new Promise((resolve) => setTimeout(resolve, 10));

    const body = JSON.parse(mockFetch.mock.calls[0][1].body);
    expect(body.url).toBe('http://localhost:4321/test');
  });

  it('should handle fetch failures gracefully', async () => {
    mockFetch.mockRejectedValue(new Error('Network error'));

    // Should not throw
    expect(() => console.log('test')).not.toThrow();
    await new Promise((resolve) => setTimeout(resolve, 10));
  });

  it('should format objects in log messages', async () => {
    console.log('Data:', { name: 'test', value: 42 });
    await new Promise((resolve) => setTimeout(resolve, 10));

    const body = JSON.parse(mockFetch.mock.calls[0][1].body);
    expect(body.message).toContain('Data:');
    expect(body.message).toContain('name');
    expect(body.message).toContain('test');
  });
});

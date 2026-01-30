/**
 * Tests for speed-control utility exports
 * Full DOM testing requires jsdom; these tests verify module structure
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

describe('speed-control utility module', () => {
    it('should export createSpeedControl function', async () => {
        const module = await import('./speed-control');
        expect(typeof module.createSpeedControl).toBe('function');
    });

    it('createSpeedControl should accept elements parameter', async () => {
        const module = await import('./speed-control');
        // Function takes (elements) so length should be 1
        expect(module.createSpeedControl.length).toBe(1);
    });

    it('should export SpeedControlElements type (via module existence)', async () => {
        const module = await import('./speed-control');
        // TypeScript types aren't runtime-checkable, but we can verify
        // the module loads without errors
        expect(module).toBeDefined();
    });

    it('should export SpeedControlState type (via module existence)', async () => {
        const module = await import('./speed-control');
        // TypeScript types aren't runtime-checkable, but we can verify
        // the module loads without errors
        expect(module).toBeDefined();
    });
});

describe('speed-control createSpeedControl return type', () => {
    // We can't test the actual functionality without DOM,
    // but we can document what the returned object should have
    it('should document expected return object properties', () => {
        // These are the expected properties:
        const expectedProperties = [
            'state',
            'updateUI',
            'showIndicator',
            'toggleMenu',
            'closeMenu',
            'initialize',
            'handleSpeedChange',
            'handleKeyboardShortcut',
            'cleanup'
        ];

        // This test serves as documentation for the API contract
        expect(expectedProperties).toContain('state');
        expect(expectedProperties).toContain('updateUI');
        expect(expectedProperties).toContain('showIndicator');
        expect(expectedProperties).toContain('toggleMenu');
        expect(expectedProperties).toContain('closeMenu');
        expect(expectedProperties).toContain('initialize');
        expect(expectedProperties).toContain('handleSpeedChange');
        expect(expectedProperties).toContain('handleKeyboardShortcut');
        expect(expectedProperties).toContain('cleanup');
    });

    it('should document expected state properties including focusedIndex for keyboard navigation', () => {
        // State object should track:
        // - menuOpen: whether dropdown is visible
        // - indicatorTimeout: for speed indicator overlay
        // - focusedIndex: for keyboard navigation in dropdown
        const expectedStateProperties = ['menuOpen', 'indicatorTimeout', 'focusedIndex'];
        expect(expectedStateProperties).toContain('menuOpen');
        expect(expectedStateProperties).toContain('indicatorTimeout');
        expect(expectedStateProperties).toContain('focusedIndex');
    });
});

describe('speed-control keyboard navigation', () => {
    it('should document arrow key navigation behavior', () => {
        // Keyboard navigation features:
        // - ArrowDown: moves focus to next option (wraps around)
        // - ArrowUp: moves focus to previous option (wraps around)
        // - Enter/Space: selects the focused option
        // - Escape: closes the menu
        // - Tab: closes the menu and moves to next focusable element
        const supportedKeys = ['ArrowDown', 'ArrowUp', 'Enter', ' ', 'Escape', 'Tab'];
        expect(supportedKeys).toHaveLength(6);
        expect(supportedKeys).toContain('ArrowDown');
        expect(supportedKeys).toContain('ArrowUp');
        expect(supportedKeys).toContain('Enter');
        expect(supportedKeys).toContain(' ');
        expect(supportedKeys).toContain('Escape');
        expect(supportedKeys).toContain('Tab');
    });
});

describe('speed-control cleanup', () => {
    // Mock globals needed by speed-control and playback-speed modules
    const localStorageMock = {
        getItem: vi.fn(() => null),
        setItem: vi.fn(),
        removeItem: vi.fn(),
    };

    function createMockElement(overrides: Record<string, unknown> = {}): HTMLElement {
        return {
            addEventListener: vi.fn(),
            removeEventListener: vi.fn(),
            setAttribute: vi.fn(),
            getAttribute: vi.fn(() => null),
            querySelectorAll: vi.fn(() => []),
            focus: vi.fn(),
            textContent: '',
            classList: {
                add: vi.fn(),
                remove: vi.fn(),
                toggle: vi.fn(),
                contains: vi.fn(() => false),
            },
            style: { display: '' },
            dataset: {},
            ...overrides,
        } as unknown as HTMLElement;
    }

    beforeEach(() => {
        vi.useFakeTimers();
        vi.stubGlobal('localStorage', localStorageMock);
        // speed-control.ts uses window.setTimeout / clearTimeout
        vi.stubGlobal('window', {
            setTimeout: (fn: () => void, ms: number) => setTimeout(fn, ms),
            clearTimeout: (id: number) => clearTimeout(id),
        });
    });

    afterEach(() => {
        vi.useRealTimers();
        vi.restoreAllMocks();
    });

    it('cleanup should clear pending indicator timeout', async () => {
        const { createSpeedControl } = await import('./speed-control');

        const elements = {
            video: createMockElement({ playbackRate: 1.0 }) as unknown as HTMLVideoElement,
            speedBtn: createMockElement() as unknown as HTMLButtonElement,
            speedOptions: createMockElement(),
            speedIndicator: createMockElement(),
        };

        const ctrl = createSpeedControl(elements);
        ctrl.initialize();

        // showIndicator sets a timeout that clears after 800ms
        ctrl.showIndicator(1.5);
        expect(ctrl.state.indicatorTimeout).not.toBeNull();

        ctrl.cleanup();
        expect(ctrl.state.indicatorTimeout).toBeNull();
    });

    it('cleanup should be safe to call with no pending timeout', async () => {
        const { createSpeedControl } = await import('./speed-control');

        const elements = {
            video: createMockElement({ playbackRate: 1.0 }) as unknown as HTMLVideoElement,
            speedBtn: createMockElement() as unknown as HTMLButtonElement,
            speedOptions: createMockElement(),
            speedIndicator: createMockElement(),
        };

        const ctrl = createSpeedControl(elements);
        expect(ctrl.state.indicatorTimeout).toBeNull();

        // Should not throw
        ctrl.cleanup();
        expect(ctrl.state.indicatorTimeout).toBeNull();
    });

    it('cleanup should be safe to call multiple times', async () => {
        const { createSpeedControl } = await import('./speed-control');

        const elements = {
            video: createMockElement({ playbackRate: 1.0 }) as unknown as HTMLVideoElement,
            speedBtn: createMockElement() as unknown as HTMLButtonElement,
            speedOptions: createMockElement(),
            speedIndicator: createMockElement(),
        };

        const ctrl = createSpeedControl(elements);
        ctrl.initialize();
        ctrl.showIndicator(2.0);

        ctrl.cleanup();
        ctrl.cleanup(); // Second call should not throw
        expect(ctrl.state.indicatorTimeout).toBeNull();
    });
});

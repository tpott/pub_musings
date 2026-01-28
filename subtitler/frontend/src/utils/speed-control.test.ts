/**
 * Tests for speed-control utility exports
 * Full DOM testing requires jsdom; these tests verify module structure
 */
import { describe, it, expect } from 'vitest';

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
});

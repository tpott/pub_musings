/**
 * Tests for dialog utility exports
 * Full DOM testing requires jsdom; these tests verify module structure
 */
import { describe, it, expect } from 'vitest';

describe('dialog utility module', () => {
	it('should export showAlert function', async () => {
		const module = await import('./dialog');
		expect(typeof module.showAlert).toBe('function');
	});

	it('should export showConfirm function', async () => {
		const module = await import('./dialog');
		expect(typeof module.showConfirm).toBe('function');
	});

	it('should export destroyDialog function', async () => {
		const module = await import('./dialog');
		expect(typeof module.destroyDialog).toBe('function');
	});

	it('showAlert should accept message and type parameters', async () => {
		const module = await import('./dialog');
		// Check function signature by examining length (number of required params)
		// showAlert takes (message, type?) so length should be 1 (required param)
		expect(module.showAlert.length).toBeGreaterThanOrEqual(1);
	});

	it('showConfirm should accept message and options parameters', async () => {
		const module = await import('./dialog');
		// showConfirm takes (message, options?) so length should be 1 (required param)
		expect(module.showConfirm.length).toBeGreaterThanOrEqual(1);
	});

	it('destroyDialog should be callable without arguments', async () => {
		const module = await import('./dialog');
		// destroyDialog takes no arguments
		expect(module.destroyDialog.length).toBe(0);
	});
});

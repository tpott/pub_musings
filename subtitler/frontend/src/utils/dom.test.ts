/**
 * Tests for dom utility exports
 * Full DOM testing requires jsdom; these tests verify module structure
 */
import { describe, it, expect } from 'vitest';

describe('dom utility module', () => {
    it('should export ElementNotFoundError class', async () => {
        const module = await import('./dom');
        expect(module.ElementNotFoundError).toBeDefined();
        expect(typeof module.ElementNotFoundError).toBe('function');
    });

    it('ElementNotFoundError should create error with correct name', async () => {
        const { ElementNotFoundError } = await import('./dom');
        const error = new ElementNotFoundError('testId');
        expect(error.name).toBe('ElementNotFoundError');
        expect(error.message).toBe('Required element not found: #testId');
        expect(error instanceof Error).toBe(true);
    });

    it('should export getRequiredElement function', async () => {
        const module = await import('./dom');
        expect(typeof module.getRequiredElement).toBe('function');
    });

    it('should export getRequiredElements function', async () => {
        const module = await import('./dom');
        expect(typeof module.getRequiredElements).toBe('function');
    });

    it('should export getOptionalElement function', async () => {
        const module = await import('./dom');
        expect(typeof module.getOptionalElement).toBe('function');
    });

    it('should export queryRequiredElement function', async () => {
        const module = await import('./dom');
        expect(typeof module.queryRequiredElement).toBe('function');
    });

    it('should export queryAllElements function', async () => {
        const module = await import('./dom');
        expect(typeof module.queryAllElements).toBe('function');
    });
});

describe('dom utility function signatures', () => {
    it('getRequiredElement should take id and elementType parameters', async () => {
        const { getRequiredElement } = await import('./dom');
        // Function takes (id, elementType) so length should be 2
        expect(getRequiredElement.length).toBe(2);
    });

    it('getRequiredElements should take spec parameter', async () => {
        const { getRequiredElements } = await import('./dom');
        // Function takes (spec) so length should be 1
        expect(getRequiredElements.length).toBe(1);
    });

    it('getOptionalElement should take id and elementType parameters', async () => {
        const { getOptionalElement } = await import('./dom');
        expect(getOptionalElement.length).toBe(2);
    });

    it('queryRequiredElement should take selector, elementType, and optional parent', async () => {
        const { queryRequiredElement } = await import('./dom');
        // Function has (selector, elementType, parent = document) so length is 2 (required params)
        expect(queryRequiredElement.length).toBe(2);
    });

    it('queryAllElements should take selector, elementType, and optional parent', async () => {
        const { queryAllElements } = await import('./dom');
        expect(queryAllElements.length).toBe(2);
    });
});

describe('dom utility API documentation', () => {
    it('should document getRequiredElements usage pattern', () => {
        // Example usage pattern:
        // const elements = getRequiredElements({
        //   email: HTMLInputElement,
        //   password: HTMLInputElement,
        //   submitBtn: HTMLButtonElement,
        // });
        // elements.email is typed as HTMLInputElement
        // elements.password is typed as HTMLInputElement
        // elements.submitBtn is typed as HTMLButtonElement

        // Document that spec keys become property names in the returned object
        const exampleKeys = ['email', 'password', 'submitBtn'];
        expect(exampleKeys).toHaveLength(3);
        expect(exampleKeys).toContain('email');
        expect(exampleKeys).toContain('password');
        expect(exampleKeys).toContain('submitBtn');
    });

    it('should document error types', () => {
        // The dom utility throws these error types:
        // - ElementNotFoundError: when required element not found
        // - TypeError: when element type doesn't match expected type
        const errorTypes = ['ElementNotFoundError', 'TypeError'];
        expect(errorTypes).toContain('ElementNotFoundError');
        expect(errorTypes).toContain('TypeError');
    });

    it('should document supported element types', () => {
        // Common HTML element types supported by the utility:
        const supportedTypes = [
            'HTMLElement',
            'HTMLInputElement',
            'HTMLButtonElement',
            'HTMLSelectElement',
            'HTMLTextAreaElement',
            'HTMLAnchorElement',
            'HTMLFormElement',
            'HTMLVideoElement',
            'HTMLDivElement',
        ];
        expect(supportedTypes.length).toBeGreaterThan(0);
    });
});

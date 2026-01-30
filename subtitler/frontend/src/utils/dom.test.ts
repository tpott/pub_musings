/**
 * Tests for dom utility exports
 * Full DOM testing requires jsdom; these tests verify module structure
 */
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import {
    ElementNotFoundError,
    getRequiredElement,
    getRequiredElements,
    getOptionalElement,
    queryRequiredElement,
    queryAllElements,
} from './dom';

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

// Mock DOM environment for functional tests
// Define mock element store at module level for mocked document
let mockElements: Record<string, Element | null> = {};
let mockElementArrays: Record<string, Element[]> = {};

// Create mock element classes that properly pass instanceof checks
class MockHTMLElement {
    constructor() { /* mock */ }
}
class MockHTMLInputElement extends MockHTMLElement {
    constructor() { super(); }
}
class MockHTMLButtonElement extends MockHTMLElement {
    constructor() { super(); }
}
class MockHTMLDivElement extends MockHTMLElement {
    constructor() { super(); }
}
class MockHTMLAnchorElement extends MockHTMLElement {
    constructor() { super(); }
}
class MockHTMLFormElement extends MockHTMLElement {
    constructor() { super(); }
}
class MockHTMLSpanElement extends MockHTMLElement {
    constructor() { super(); }
}

// Mock document
const documentMock = {
    getElementById: vi.fn((id: string) => mockElements[id] || null),
    querySelector: vi.fn((selector: string) => mockElements[selector] || null),
    querySelectorAll: vi.fn((selector: string) => {
        const elements = mockElementArrays[selector] || [];
        // Create array-like object with forEach method
        const nodeList = {
            length: elements.length,
            forEach: (callback: (el: Element, index: number) => void) => {
                elements.forEach(callback);
            },
        };
        return nodeList as unknown as NodeListOf<Element>;
    }),
};

// Setup global document mock before any tests run
Object.defineProperty(global, 'document', {
    value: documentMock,
    writable: true,
});

// Also need to mock the HTML element classes
Object.defineProperty(global, 'HTMLElement', { value: MockHTMLElement, writable: true });
Object.defineProperty(global, 'HTMLInputElement', { value: MockHTMLInputElement, writable: true });
Object.defineProperty(global, 'HTMLButtonElement', { value: MockHTMLButtonElement, writable: true });
Object.defineProperty(global, 'HTMLDivElement', { value: MockHTMLDivElement, writable: true });
Object.defineProperty(global, 'HTMLAnchorElement', { value: MockHTMLAnchorElement, writable: true });
Object.defineProperty(global, 'HTMLFormElement', { value: MockHTMLFormElement, writable: true });
Object.defineProperty(global, 'HTMLSpanElement', { value: MockHTMLSpanElement, writable: true });

describe('dom utility functional tests (with mocks)', () => {
    beforeEach(() => {
        mockElements = {};
        mockElementArrays = {};
        vi.clearAllMocks();
    });

    describe('getRequiredElement', () => {
        it('should return element when found with correct type', () => {
            const mockInput = new MockHTMLInputElement();
            mockElements['myInput'] = mockInput as unknown as Element;

            const result = getRequiredElement('myInput', HTMLInputElement);
            expect(result).toBe(mockInput);
        });

        it('should throw ElementNotFoundError when element not found', () => {
            expect(() => getRequiredElement('nonexistent', HTMLInputElement))
                .toThrow(ElementNotFoundError);
        });

        it('should throw TypeError when element is wrong type', () => {
            const mockDiv = new MockHTMLDivElement();
            mockElements['myDiv'] = mockDiv as unknown as Element;

            expect(() => getRequiredElement('myDiv', HTMLInputElement))
                .toThrow(TypeError);
        });
    });

    describe('getOptionalElement', () => {
        it('should return element when found with correct type', () => {
            const mockButton = new MockHTMLButtonElement();
            mockElements['myButton'] = mockButton as unknown as Element;

            const result = getOptionalElement('myButton', HTMLButtonElement);
            expect(result).toBe(mockButton);
        });

        it('should return null when element not found', () => {
            const result = getOptionalElement('nonexistent', HTMLButtonElement);
            expect(result).toBeNull();
        });

        it('should throw TypeError when element is wrong type', () => {
            const mockSpan = new MockHTMLSpanElement();
            mockElements['mySpan'] = mockSpan as unknown as Element;

            expect(() => getOptionalElement('mySpan', HTMLButtonElement))
                .toThrow(TypeError);
        });
    });

    describe('getRequiredElements', () => {
        it('should return all elements when found with correct types', () => {
            const mockInput = new MockHTMLInputElement();
            const mockButton = new MockHTMLButtonElement();
            mockElements['email'] = mockInput as unknown as Element;
            mockElements['submit'] = mockButton as unknown as Element;

            const result = getRequiredElements({
                email: HTMLInputElement,
                submit: HTMLButtonElement,
            });

            expect(result.email).toBe(mockInput);
            expect(result.submit).toBe(mockButton);
        });

        it('should throw ElementNotFoundError listing all missing elements', () => {
            mockElements['found'] = new MockHTMLInputElement() as unknown as Element;

            expect(() => getRequiredElements({
                found: HTMLInputElement,
                missing1: HTMLButtonElement,
                missing2: HTMLDivElement,
            })).toThrow('missing1, missing2');
        });

        it('should throw TypeError when any element is wrong type', () => {
            mockElements['input'] = new MockHTMLInputElement() as unknown as Element;
            mockElements['wrongType'] = new MockHTMLDivElement() as unknown as Element;

            expect(() => getRequiredElements({
                input: HTMLInputElement,
                wrongType: HTMLButtonElement,
            })).toThrow(TypeError);
        });
    });

    describe('queryRequiredElement', () => {
        it('should return element when found with correct type', () => {
            const mockLink = new MockHTMLAnchorElement();
            mockElements['a.link'] = mockLink as unknown as Element;

            const result = queryRequiredElement('a.link', HTMLAnchorElement);
            expect(result).toBe(mockLink);
        });

        it('should throw Error when element not found', () => {
            expect(() => queryRequiredElement('.nonexistent', HTMLElement))
                .toThrow('Required element not found: .nonexistent');
        });

        it('should throw TypeError when element is wrong type', () => {
            const mockForm = new MockHTMLFormElement();
            mockElements['form'] = mockForm as unknown as Element;

            expect(() => queryRequiredElement('form', HTMLInputElement))
                .toThrow(TypeError);
        });
    });

    describe('queryAllElements', () => {
        it('should return empty array when no elements found', () => {
            mockElementArrays['.buttons'] = [];

            const result = queryAllElements('.buttons', HTMLButtonElement);
            expect(result).toHaveLength(0);
        });

        it('should return all elements with correct type', () => {
            const btn1 = new MockHTMLButtonElement();
            const btn2 = new MockHTMLButtonElement();
            mockElementArrays['.btn'] = [btn1 as unknown as Element, btn2 as unknown as Element];

            const result = queryAllElements('.btn', HTMLButtonElement);
            expect(result).toHaveLength(2);
            expect(result[0]).toBe(btn1);
            expect(result[1]).toBe(btn2);
        });

        it('should throw TypeError if any element is wrong type', () => {
            const btn = new MockHTMLButtonElement();
            const div = new MockHTMLDivElement();
            mockElementArrays['.mixed'] = [btn as unknown as Element, div as unknown as Element];

            expect(() => queryAllElements('.mixed', HTMLButtonElement))
                .toThrow(TypeError);
        });
    });
});

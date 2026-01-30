// DOM query helper utility
// Provides type-safe element access with validation

/**
 * Error thrown when a required DOM element is not found.
 */
export class ElementNotFoundError extends Error {
    constructor(id: string) {
        super(`Required element not found: #${id}`);
        this.name = 'ElementNotFoundError';
    }
}

/**
 * Type definition for element specifications.
 * Maps element IDs to their expected types.
 */
export type ElementSpec<T extends Record<string, HTMLElement>> = {
    [K in keyof T]: new () => T[K];
};

/**
 * Get a single required element by ID with type checking.
 * Throws if element not found or wrong type.
 *
 * @param id - The element ID (without #)
 * @param elementType - The expected element constructor (HTMLElement, HTMLInputElement, etc.)
 * @returns The element cast to the specified type
 * @throws ElementNotFoundError if element not found
 * @throws TypeError if element is wrong type
 *
 * @example
 * const input = getRequiredElement('email', HTMLInputElement);
 * // input is typed as HTMLInputElement
 */
export function getRequiredElement<T extends HTMLElement>(
    id: string,
    elementType: new () => T
): T {
    const element = document.getElementById(id);
    if (!element) {
        throw new ElementNotFoundError(id);
    }
    if (!(element instanceof elementType)) {
        throw new TypeError(
            `Element #${id} is ${element.constructor.name}, expected ${elementType.name}`
        );
    }
    return element;
}

/**
 * Get multiple required elements by ID with type checking.
 * All elements must exist, or an error is thrown.
 *
 * @param spec - Object mapping element IDs to their expected types
 * @returns Object with same keys as spec, values are the typed elements
 * @throws ElementNotFoundError if any element not found
 * @throws TypeError if any element is wrong type
 *
 * @example
 * const elements = getRequiredElements({
 *   email: HTMLInputElement,
 *   password: HTMLInputElement,
 *   submitBtn: HTMLButtonElement,
 *   form: HTMLFormElement,
 * });
 * // elements.email is HTMLInputElement
 * // elements.password is HTMLInputElement
 * // elements.submitBtn is HTMLButtonElement
 * // elements.form is HTMLFormElement
 */
export function getRequiredElements<T extends Record<string, HTMLElement>>(
    spec: ElementSpec<T>
): T {
    const result = {} as T;
    const missingIds: string[] = [];

    for (const [id, elementType] of Object.entries(spec)) {
        const element = document.getElementById(id);
        if (!element) {
            missingIds.push(id);
            continue;
        }
        if (!(element instanceof elementType)) {
            throw new TypeError(
                `Element #${id} is ${element.constructor.name}, expected ${elementType.name}`
            );
        }
        (result as Record<string, HTMLElement>)[id] = element;
    }

    if (missingIds.length > 0) {
        throw new ElementNotFoundError(missingIds.join(', '));
    }

    return result;
}

/**
 * Get a single optional element by ID with type checking.
 * Returns null if element not found.
 *
 * @param id - The element ID (without #)
 * @param elementType - The expected element constructor
 * @returns The element cast to the specified type, or null if not found
 * @throws TypeError if element exists but is wrong type
 *
 * @example
 * const sidebar = getOptionalElement('sidebar', HTMLElement);
 * if (sidebar) {
 *   sidebar.classList.add('visible');
 * }
 */
export function getOptionalElement<T extends HTMLElement>(
    id: string,
    elementType: new () => T
): T | null {
    const element = document.getElementById(id);
    if (!element) {
        return null;
    }
    if (!(element instanceof elementType)) {
        throw new TypeError(
            `Element #${id} is ${element.constructor.name}, expected ${elementType.name}`
        );
    }
    return element;
}

/**
 * Query a single required element by selector with type checking.
 *
 * @param selector - CSS selector
 * @param elementType - The expected element constructor
 * @param parent - Parent element to search within (default: document)
 * @returns The element cast to the specified type
 * @throws Error if element not found
 * @throws TypeError if element is wrong type
 *
 * @example
 * const firstInput = queryRequiredElement('form input', HTMLInputElement);
 */
export function queryRequiredElement<T extends Element>(
    selector: string,
    elementType: new () => T,
    parent: ParentNode = document
): T {
    const element = parent.querySelector(selector);
    if (!element) {
        throw new Error(`Required element not found: ${selector}`);
    }
    if (!(element instanceof elementType)) {
        throw new TypeError(
            `Element "${selector}" is ${element.constructor.name}, expected ${elementType.name}`
        );
    }
    return element;
}

/**
 * Query all elements matching a selector with type checking.
 *
 * @param selector - CSS selector
 * @param elementType - The expected element constructor
 * @param parent - Parent element to search within (default: document)
 * @returns Array of elements cast to the specified type
 * @throws TypeError if any element is wrong type
 *
 * @example
 * const buttons = queryAllElements('.btn', HTMLButtonElement);
 */
export function queryAllElements<T extends Element>(
    selector: string,
    elementType: new () => T,
    parent: ParentNode = document
): T[] {
    const elements = parent.querySelectorAll(selector);
    const result: T[] = [];

    elements.forEach((element, index) => {
        if (!(element instanceof elementType)) {
            throw new TypeError(
                `Element "${selector}"[${index}] is ${element.constructor.name}, expected ${elementType.name}`
            );
        }
        result.push(element);
    });

    return result;
}

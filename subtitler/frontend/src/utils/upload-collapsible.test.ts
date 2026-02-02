import { describe, it, expect, vi, beforeEach } from 'vitest';
import { setupCollapsible } from './upload-collapsible';

function createMockSection() {
	const classes = new Set<string>();
	return {
		classList: {
			add(c: string) { classes.add(c); },
			remove(c: string) { classes.delete(c); },
			contains(c: string) { return classes.has(c); },
		},
	} as unknown as HTMLElement;
}

function createMockHeader() {
	const attrs = new Map<string, string>();
	const listeners = new Map<string, Function[]>();
	return {
		setAttribute(name: string, val: string) { attrs.set(name, val); },
		getAttribute(name: string) { return attrs.get(name) ?? null; },
		addEventListener(event: string, handler: Function) {
			if (!listeners.has(event)) listeners.set(event, []);
			listeners.get(event)!.push(handler);
		},
		_fire(event: string, detail?: any) {
			for (const fn of listeners.get(event) || []) fn(detail);
		},
		_attrs: attrs,
	} as unknown as HTMLElement & { _fire: Function; _attrs: Map<string, string> };
}

describe('setupCollapsible', () => {
	let mockStorage: Map<string, string>;

	beforeEach(() => {
		mockStorage = new Map();
		vi.stubGlobal('localStorage', {
			getItem: (key: string) => mockStorage.get(key) ?? null,
			setItem: (key: string, val: string) => mockStorage.set(key, val),
		});
	});

	it('starts expanded when defaultCollapsed is false and no saved state', () => {
		const section = createMockSection();
		const header = createMockHeader();

		setupCollapsible(section, header, 'test-key', false);

		expect(section.classList.contains('collapsed')).toBe(false);
		expect(header._attrs.get('aria-expanded')).toBe('true');
	});

	it('starts collapsed when defaultCollapsed is true and no saved state', () => {
		const section = createMockSection();
		const header = createMockHeader();

		setupCollapsible(section, header, 'test-key', true);

		expect(section.classList.contains('collapsed')).toBe(true);
		expect(header._attrs.get('aria-expanded')).toBe('false');
	});

	it('respects saved collapsed state over default', () => {
		mockStorage.set('test-key', 'true');
		const section = createMockSection();
		const header = createMockHeader();

		setupCollapsible(section, header, 'test-key', false);

		expect(section.classList.contains('collapsed')).toBe(true);
		expect(header._attrs.get('aria-expanded')).toBe('false');
	});

	it('respects saved expanded state over default', () => {
		mockStorage.set('test-key', 'false');
		const section = createMockSection();
		const header = createMockHeader();

		setupCollapsible(section, header, 'test-key', true);

		expect(section.classList.contains('collapsed')).toBe(false);
		expect(header._attrs.get('aria-expanded')).toBe('true');
	});

	it('toggles on click', () => {
		const section = createMockSection();
		const header = createMockHeader();

		setupCollapsible(section, header, 'test-key', false);
		expect(section.classList.contains('collapsed')).toBe(false);

		header._fire('click');
		expect(section.classList.contains('collapsed')).toBe(true);
		expect(header._attrs.get('aria-expanded')).toBe('false');
		expect(mockStorage.get('test-key')).toBe('true');

		header._fire('click');
		expect(section.classList.contains('collapsed')).toBe(false);
		expect(header._attrs.get('aria-expanded')).toBe('true');
		expect(mockStorage.get('test-key')).toBe('false');
	});

	it('toggles on Enter key', () => {
		const section = createMockSection();
		const header = createMockHeader();

		setupCollapsible(section, header, 'test-key', false);

		header._fire('keydown', { key: 'Enter', preventDefault: vi.fn() });
		expect(section.classList.contains('collapsed')).toBe(true);
	});

	it('toggles on Space key', () => {
		const section = createMockSection();
		const header = createMockHeader();

		setupCollapsible(section, header, 'test-key', false);

		header._fire('keydown', { key: ' ', preventDefault: vi.fn() });
		expect(section.classList.contains('collapsed')).toBe(true);
	});

	it('does not toggle on other keys', () => {
		const section = createMockSection();
		const header = createMockHeader();

		setupCollapsible(section, header, 'test-key', false);

		header._fire('keydown', { key: 'a', preventDefault: vi.fn() });
		expect(section.classList.contains('collapsed')).toBe(false);
	});
});

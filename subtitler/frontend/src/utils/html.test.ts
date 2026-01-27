import { describe, it, expect, beforeEach, vi } from 'vitest';
import { escapeHtml } from './html';

// Mock document.createElement for testing
const createMockElement = () => {
	let textContentValue = '';
	return {
		get textContent() {
			return textContentValue;
		},
		set textContent(value: string) {
			textContentValue = value;
		},
		get innerHTML() {
			// Simulate browser's HTML escaping behavior
			return textContentValue
				.replace(/&/g, '&amp;')
				.replace(/</g, '&lt;')
				.replace(/>/g, '&gt;');
		},
	};
};

// Mock document
const documentMock = {
	createElement: vi.fn(() => createMockElement()),
};

Object.defineProperty(global, 'document', {
	value: documentMock,
	writable: true,
});

describe('escapeHtml', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('should escape HTML tags', () => {
		expect(escapeHtml('<script>alert("xss")</script>')).toBe(
			'&lt;script&gt;alert("xss")&lt;/script&gt;'
		);
	});

	it('should escape ampersands', () => {
		expect(escapeHtml('foo & bar')).toBe('foo &amp; bar');
	});

	it('should escape less than and greater than', () => {
		expect(escapeHtml('1 < 2 > 0')).toBe('1 &lt; 2 &gt; 0');
	});

	it('should handle empty string', () => {
		expect(escapeHtml('')).toBe('');
	});

	it('should handle normal text without special chars', () => {
		expect(escapeHtml('Hello World')).toBe('Hello World');
	});

	it('should handle unicode characters', () => {
		expect(escapeHtml('こんにちは 你好 مرحبا')).toBe('こんにちは 你好 مرحبا');
	});

	it('should handle newlines and whitespace', () => {
		expect(escapeHtml('line1\nline2\ttabbed')).toBe('line1\nline2\ttabbed');
	});

	it('should prevent XSS via onclick', () => {
		const malicious = '<img src="x" onerror="alert(1)">';
		const escaped = escapeHtml(malicious);
		expect(escaped).not.toContain('<img');
		expect(escaped).toContain('&lt;img');
	});

	it('should prevent XSS via event handlers', () => {
		const malicious = '<div onmouseover="alert(document.cookie)">hover me</div>';
		const escaped = escapeHtml(malicious);
		expect(escaped).not.toContain('<div');
		expect(escaped).toContain('&lt;div');
	});

	it('should handle script injection attempts', () => {
		const attempts = [
			'<script>alert(1)</script>',
			'<SCRIPT>alert(1)</SCRIPT>',
			'<ScRiPt>alert(1)</ScRiPt>',
			'<img src=x onerror=alert(1)>',
			'<svg onload=alert(1)>',
			'<body onload=alert(1)>',
			'<iframe src="javascript:alert(1)">',
		];

		for (const attempt of attempts) {
			const escaped = escapeHtml(attempt);
			expect(escaped).not.toMatch(/<[a-zA-Z]/);
		}
	});

	it('should call document.createElement with div', () => {
		escapeHtml('test');
		expect(documentMock.createElement).toHaveBeenCalledWith('div');
	});
});

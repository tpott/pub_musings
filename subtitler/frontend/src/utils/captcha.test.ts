import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { getCaptchaConfig, getCaptchaToken, resetCaptcha } from './captcha';

describe('CAPTCHA utility', () => {
	beforeEach(() => {
		vi.resetAllMocks();
		// Clear any cached config
		vi.resetModules();
	});

	afterEach(() => {
		vi.restoreAllMocks();
	});

	describe('getCaptchaConfig', () => {
		it('should return config from API when enabled', async () => {
			globalThis.fetch = vi.fn().mockResolvedValueOnce({
				ok: true,
				json: () => Promise.resolve({ enabled: true, site_key: 'test-site-key' }),
			} as Response);

			// Need to reimport to clear cache
			const { getCaptchaConfig: freshGetConfig } = await import('./captcha');
			const config = await freshGetConfig();

			expect(config).toEqual({ enabled: true, site_key: 'test-site-key' });
			expect(fetch).toHaveBeenCalledWith('/api/captcha/config');
		});

		it('should return disabled config when API returns error', async () => {
			globalThis.fetch = vi.fn().mockResolvedValueOnce({
				ok: false,
				status: 500,
			} as Response);

			const { getCaptchaConfig: freshGetConfig } = await import('./captcha');
			const config = await freshGetConfig();

			expect(config).toEqual({ enabled: false, site_key: '' });
		});

		it('should return disabled config when fetch fails', async () => {
			globalThis.fetch = vi.fn().mockRejectedValueOnce(new Error('Network error'));

			const { getCaptchaConfig: freshGetConfig } = await import('./captcha');
			const config = await freshGetConfig();

			expect(config).toEqual({ enabled: false, site_key: '' });
		});
	});

	// Note: getCaptchaToken and resetCaptcha tests require a browser environment
	// with the hcaptcha script loaded. These are better tested via E2E tests.
	// The core logic is simple: check if hcaptcha global exists and call its methods.
});

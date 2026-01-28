// CAPTCHA utility for hCaptcha integration

export interface CaptchaConfig {
	enabled: boolean;
	site_key: string;
}

// Timeout for loading hCaptcha script from CDN
const HCAPTCHA_LOAD_TIMEOUT_MS = 15000;

let captchaConfig: CaptchaConfig | null = null;
let hcaptchaLoaded = false;
let hcaptchaLoadPromise: Promise<void> | null = null;

// Fetch CAPTCHA configuration from the backend
export async function getCaptchaConfig(): Promise<CaptchaConfig> {
	if (captchaConfig !== null) {
		return captchaConfig;
	}

	try {
		const response = await fetch('/api/captcha/config');
		if (!response.ok) {
			captchaConfig = { enabled: false, site_key: '' };
			return captchaConfig;
		}
		captchaConfig = await response.json();
		return captchaConfig!;
	} catch (error) {
		console.error('Failed to fetch CAPTCHA config:', error);
		captchaConfig = { enabled: false, site_key: '' };
		return captchaConfig;
	}
}

// Custom error class for script load timeout
export class CaptchaLoadTimeoutError extends Error {
	constructor(message: string = 'hCaptcha script load timed out') {
		super(message);
		this.name = 'CaptchaLoadTimeoutError';
	}
}

// Load the hCaptcha script dynamically with timeout
function loadHCaptchaScript(): Promise<void> {
	if (hcaptchaLoadPromise) {
		return hcaptchaLoadPromise;
	}

	if (hcaptchaLoaded) {
		return Promise.resolve();
	}

	hcaptchaLoadPromise = new Promise((resolve, reject) => {
		const script = document.createElement('script');
		script.src = 'https://js.hcaptcha.com/1/api.js?render=explicit';
		script.async = true;
		script.defer = true;

		// Timeout to prevent indefinite waiting
		const timeoutId = setTimeout(() => {
			hcaptchaLoadPromise = null;
			script.remove();
			reject(new CaptchaLoadTimeoutError());
		}, HCAPTCHA_LOAD_TIMEOUT_MS);

		script.onload = () => {
			clearTimeout(timeoutId);
			hcaptchaLoaded = true;
			resolve();
		};

		script.onerror = () => {
			clearTimeout(timeoutId);
			hcaptchaLoadPromise = null;
			reject(new Error('Failed to load hCaptcha script'));
		};

		document.head.appendChild(script);
	});

	return hcaptchaLoadPromise;
}

// Render hCaptcha widget in a container element
export async function renderCaptcha(containerId: string, siteKey: string): Promise<string> {
	await loadHCaptchaScript();

	// Wait for hcaptcha to be ready
	const hcaptcha = (window as { hcaptcha?: HCaptcha }).hcaptcha;
	if (!hcaptcha) {
		throw new Error('hCaptcha not available');
	}

	return new Promise((resolve, reject) => {
		try {
			const widgetId = hcaptcha.render(containerId, {
				sitekey: siteKey,
				callback: (token: string) => {
					resolve(token);
				},
				'error-callback': () => {
					reject(new Error('CAPTCHA error'));
				},
				'expired-callback': () => {
					reject(new Error('CAPTCHA expired'));
				},
			});
			// Store widget ID on the container for later reset
			const container = document.getElementById(containerId);
			if (container) {
				container.dataset.widgetId = widgetId;
			}
		} catch (err) {
			reject(err);
		}
	});
}

// Get the response token from a rendered widget
export function getCaptchaToken(containerId: string): string | null {
	const hcaptcha = (window as { hcaptcha?: HCaptcha }).hcaptcha;
	if (!hcaptcha) {
		return null;
	}

	const container = document.getElementById(containerId);
	const widgetId = container?.dataset.widgetId;
	if (widgetId === undefined) {
		return null;
	}

	return hcaptcha.getResponse(widgetId);
}

// Reset the CAPTCHA widget
export function resetCaptcha(containerId: string): void {
	const hcaptcha = (window as { hcaptcha?: HCaptcha }).hcaptcha;
	if (!hcaptcha) {
		return;
	}

	const container = document.getElementById(containerId);
	const widgetId = container?.dataset.widgetId;
	if (widgetId === undefined) {
		return;
	}

	hcaptcha.reset(widgetId);
}

// Execute invisible CAPTCHA
export async function executeCaptcha(siteKey: string): Promise<string> {
	await loadHCaptchaScript();

	const hcaptcha = (window as { hcaptcha?: HCaptcha }).hcaptcha;
	if (!hcaptcha) {
		throw new Error('hCaptcha not available');
	}

	return hcaptcha.execute({ sitekey: siteKey, async: true }).response;
}

// Type definitions for hCaptcha
interface HCaptcha {
	render(container: string, options: HCaptchaRenderOptions): string;
	getResponse(widgetId?: string): string;
	reset(widgetId?: string): void;
	execute(options: { sitekey: string; async: true }): Promise<{ response: string }>;
}

interface HCaptchaRenderOptions {
	sitekey: string;
	callback?: (token: string) => void;
	'error-callback'?: () => void;
	'expired-callback'?: () => void;
	size?: 'normal' | 'compact' | 'invisible';
	theme?: 'light' | 'dark';
}

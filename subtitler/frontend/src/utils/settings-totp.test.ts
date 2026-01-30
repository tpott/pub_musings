import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
	setupTotp,
	type TotpElements,
	type TotpCallbacks,
	type TotpState,
} from './settings-totp';

// --- Mocks ---

vi.mock('./csrf', () => ({
	csrfFetch: vi.fn(),
}));

vi.mock('./validation', () => ({
	validatePassword: vi.fn((v: string) => v.length < 8 ? 'Password too short' : null),
	validateTotpCode: vi.fn((v: string) => v.length !== 6 ? 'Code must be 6 digits' : null),
}));

vi.mock('./form-errors', () => ({
	clearFieldError: vi.fn(),
	showFieldError: vi.fn(),
}));

vi.mock('./form-validation', () => ({
	setupBlurValidation: vi.fn(),
}));

vi.mock('./api-schemas', () => ({
	TotpSetupResponseSchema: {},
	TotpVerifyResponseSchema: {},
	RecoveryCodesResponseSchema: {},
	safeParse: vi.fn((schema: unknown, data: unknown) => data),
}));

function makeMockElements(): TotpElements {
	return {
		totpStatus: { textContent: '', className: '' } as unknown as HTMLElement,
		enableSection: { classList: { add: vi.fn(), remove: vi.fn() } } as unknown as HTMLElement,
		setupSection: { classList: { add: vi.fn(), remove: vi.fn() } } as unknown as HTMLElement,
		disableSection: { classList: { add: vi.fn(), remove: vi.fn() } } as unknown as HTMLElement,
		startSetupBtn: { disabled: false, textContent: '', addEventListener: vi.fn() } as unknown as HTMLButtonElement,
		cancelSetupBtn: { addEventListener: vi.fn() } as unknown as HTMLButtonElement,
		verifyForm: { addEventListener: vi.fn() } as unknown as HTMLFormElement,
		verifyBtn: { disabled: false, textContent: '', addEventListener: vi.fn() } as unknown as HTMLButtonElement,
		verifyCode: { value: '', focus: vi.fn(), addEventListener: vi.fn() } as unknown as HTMLInputElement,
		verifyCodeError: {} as unknown as HTMLElement,
		disableForm: { addEventListener: vi.fn() } as unknown as HTMLFormElement,
		disableBtn: { disabled: false, textContent: '', addEventListener: vi.fn() } as unknown as HTMLButtonElement,
		disablePassword: { value: '', focus: vi.fn(), addEventListener: vi.fn() } as unknown as HTMLInputElement,
		disablePasswordError: {} as unknown as HTMLElement,
		disableCode: { value: '', addEventListener: vi.fn() } as unknown as HTMLInputElement,
		disableCodeError: {} as unknown as HTMLElement,
		qrCode: { src: '' } as unknown as HTMLImageElement,
		secretDisplay: { textContent: '' } as unknown as HTMLElement,
		recoveryCodesSection: { classList: { add: vi.fn(), remove: vi.fn() } } as unknown as HTMLElement,
		recoveryCodesGrid: { innerHTML: '', appendChild: vi.fn() } as unknown as HTMLElement,
		copyCodesBtn: { textContent: '', addEventListener: vi.fn() } as unknown as HTMLButtonElement,
		downloadCodesBtn: { textContent: '', addEventListener: vi.fn() } as unknown as HTMLButtonElement,
		savedCodesCheckbox: { checked: false, addEventListener: vi.fn() } as unknown as HTMLInputElement,
		continueBtn: { disabled: true, addEventListener: vi.fn() } as unknown as HTMLButtonElement,
		managementOptions: { classList: { add: vi.fn(), remove: vi.fn() } } as unknown as HTMLElement,
		regenSection: { classList: { add: vi.fn(), remove: vi.fn() } } as unknown as HTMLElement,
		disableFormSection: { classList: { add: vi.fn(), remove: vi.fn() } } as unknown as HTMLElement,
		startRegenBtn: { addEventListener: vi.fn() } as unknown as HTMLButtonElement,
		startDisableBtn: { addEventListener: vi.fn() } as unknown as HTMLButtonElement,
		cancelRegenBtn: { addEventListener: vi.fn() } as unknown as HTMLButtonElement,
		cancelDisableBtn: { addEventListener: vi.fn() } as unknown as HTMLButtonElement,
		regenForm: { addEventListener: vi.fn() } as unknown as HTMLFormElement,
		regenBtn: { disabled: false, textContent: '', addEventListener: vi.fn() } as unknown as HTMLButtonElement,
		regenPassword: { value: '', focus: vi.fn(), addEventListener: vi.fn() } as unknown as HTMLInputElement,
		regenPasswordError: {} as unknown as HTMLElement,
		regenCode: { value: '', addEventListener: vi.fn() } as unknown as HTMLInputElement,
		regenCodeError: {} as unknown as HTMLElement,
	};
}

function makeCallbacks(): TotpCallbacks {
	return {
		showError: vi.fn(),
		showSuccess: vi.fn(),
		hideMessages: vi.fn(),
	};
}

function makeState(totpEnabled = false): TotpState {
	return {
		currentUser: { id: 'user-1', email: 'test@example.com', totp_enabled: totpEnabled },
	};
}

let mockDocument: Record<string, unknown>;

beforeEach(() => {
	vi.clearAllMocks();
	mockDocument = {
		createElement: vi.fn(() => ({
			className: '',
			textContent: '',
			value: '',
			href: '',
			download: '',
			select: vi.fn(),
			click: vi.fn(),
		})),
		body: {
			appendChild: vi.fn(),
			removeChild: vi.fn(),
		},
		execCommand: vi.fn(),
	};
	vi.stubGlobal('document', mockDocument);
});

afterEach(() => {
	vi.restoreAllMocks();
});

// --- Tests ---

describe('setupTotp', () => {
	it('should return object with updateUI function', () => {
		const els = makeMockElements();
		const callbacks = makeCallbacks();
		const state = makeState();

		const result = setupTotp(els, callbacks, state);

		expect(typeof result.updateUI).toBe('function');
	});

	it('should register all event listeners', () => {
		const els = makeMockElements();
		const callbacks = makeCallbacks();
		const state = makeState();

		setupTotp(els, callbacks, state);

		expect(els.startSetupBtn.addEventListener).toHaveBeenCalledWith('click', expect.any(Function));
		expect(els.cancelSetupBtn.addEventListener).toHaveBeenCalledWith('click', expect.any(Function));
		expect(els.verifyForm.addEventListener).toHaveBeenCalledWith('submit', expect.any(Function));
		expect(els.disableForm.addEventListener).toHaveBeenCalledWith('submit', expect.any(Function));
		expect(els.copyCodesBtn.addEventListener).toHaveBeenCalledWith('click', expect.any(Function));
		expect(els.downloadCodesBtn.addEventListener).toHaveBeenCalledWith('click', expect.any(Function));
		expect(els.savedCodesCheckbox.addEventListener).toHaveBeenCalledWith('change', expect.any(Function));
		expect(els.continueBtn.addEventListener).toHaveBeenCalledWith('click', expect.any(Function));
		expect(els.startRegenBtn.addEventListener).toHaveBeenCalledWith('click', expect.any(Function));
		expect(els.startDisableBtn.addEventListener).toHaveBeenCalledWith('click', expect.any(Function));
		expect(els.cancelRegenBtn.addEventListener).toHaveBeenCalledWith('click', expect.any(Function));
		expect(els.cancelDisableBtn.addEventListener).toHaveBeenCalledWith('click', expect.any(Function));
		expect(els.regenForm.addEventListener).toHaveBeenCalledWith('submit', expect.any(Function));
	});

	it('should set up blur validation for all input fields', async () => {
		const { setupBlurValidation } = await import('./form-validation');
		const els = makeMockElements();
		const callbacks = makeCallbacks();
		const state = makeState();

		setupTotp(els, callbacks, state);

		expect(setupBlurValidation).toHaveBeenCalledTimes(5);
	});
});

describe('updateUI', () => {
	it('should show disable section when TOTP is enabled', () => {
		const els = makeMockElements();
		const callbacks = makeCallbacks();
		const state = makeState(true);

		const { updateUI } = setupTotp(els, callbacks, state);
		updateUI();

		expect(els.totpStatus.textContent).toBe('Enabled');
		expect(els.totpStatus.className).toBe('status-badge status-enabled');
		expect(els.enableSection.classList.add).toHaveBeenCalledWith('hidden');
		expect(els.disableSection.classList.remove).toHaveBeenCalledWith('hidden');
		expect(els.managementOptions.classList.remove).toHaveBeenCalledWith('hidden');
	});

	it('should show enable section when TOTP is disabled', () => {
		const els = makeMockElements();
		const callbacks = makeCallbacks();
		const state = makeState(false);

		const { updateUI } = setupTotp(els, callbacks, state);
		updateUI();

		expect(els.totpStatus.textContent).toBe('Disabled');
		expect(els.totpStatus.className).toBe('status-badge status-disabled');
		expect(els.enableSection.classList.remove).toHaveBeenCalledWith('hidden');
		expect(els.disableSection.classList.add).toHaveBeenCalledWith('hidden');
	});

	it('should do nothing when currentUser is null', () => {
		const els = makeMockElements();
		const callbacks = makeCallbacks();
		const state: TotpState = { currentUser: null };

		const { updateUI } = setupTotp(els, callbacks, state);
		updateUI();

		expect(els.totpStatus.textContent).toBe('');
	});
});

describe('startSetup (via event listener)', () => {
	function getStartSetupHandler(els: TotpElements): Function {
		return (els.startSetupBtn.addEventListener as ReturnType<typeof vi.fn>).mock.calls.find(
			(call: unknown[]) => call[0] === 'click'
		)![1];
	}

	it('should call csrfFetch and show QR code on success', async () => {
		const { csrfFetch } = await import('./csrf');
		const mockCsrfFetch = csrfFetch as ReturnType<typeof vi.fn>;
		mockCsrfFetch.mockResolvedValueOnce({
			ok: true,
			json: async () => ({
				secret: 'JBSWY3DPEHPK3PXP',
				secret_display: 'JBSW Y3DP EHPK 3PXP',
				qr_code: 'data:image/png;base64,...',
			}),
		});

		const els = makeMockElements();
		const callbacks = makeCallbacks();
		const state = makeState();
		setupTotp(els, callbacks, state);

		const handler = getStartSetupHandler(els);
		await handler();

		expect(mockCsrfFetch).toHaveBeenCalledWith('/api/auth/totp/setup', { method: 'POST' });
		expect(els.secretDisplay.textContent).toBe('JBSW Y3DP EHPK 3PXP');
		expect(els.qrCode.src).toBe('data:image/png;base64,...');
		expect(els.setupSection.classList.remove).toHaveBeenCalledWith('hidden');
		expect(els.enableSection.classList.add).toHaveBeenCalledWith('hidden');
		expect(els.verifyCode.focus).toHaveBeenCalled();
	});

	it('should show error on API failure', async () => {
		const { csrfFetch } = await import('./csrf');
		const mockCsrfFetch = csrfFetch as ReturnType<typeof vi.fn>;
		mockCsrfFetch.mockResolvedValueOnce({
			ok: false,
			json: async () => ({ error: 'Already set up' }),
		});

		const els = makeMockElements();
		const callbacks = makeCallbacks();
		const state = makeState();
		setupTotp(els, callbacks, state);

		const handler = getStartSetupHandler(els);
		await handler();

		expect(callbacks.showError).toHaveBeenCalledWith('Already set up');
		expect(els.startSetupBtn.disabled).toBe(false);
		expect(els.startSetupBtn.textContent).toBe('Enable 2FA');
	});

	it('should show error on network failure', async () => {
		const { csrfFetch } = await import('./csrf');
		const mockCsrfFetch = csrfFetch as ReturnType<typeof vi.fn>;
		mockCsrfFetch.mockRejectedValueOnce(new Error('Network error'));

		const els = makeMockElements();
		const callbacks = makeCallbacks();
		const state = makeState();
		setupTotp(els, callbacks, state);

		const handler = getStartSetupHandler(els);
		await handler();

		expect(callbacks.showError).toHaveBeenCalledWith('Network error. Please try again.');
		expect(els.startSetupBtn.disabled).toBe(false);
	});
});

describe('cancelSetup (via event listener)', () => {
	it('should reset setup state', () => {
		const els = makeMockElements();
		const callbacks = makeCallbacks();
		const state = makeState();
		setupTotp(els, callbacks, state);

		const handler = (els.cancelSetupBtn.addEventListener as ReturnType<typeof vi.fn>).mock.calls.find(
			(call: unknown[]) => call[0] === 'click'
		)![1];
		handler();

		expect(callbacks.hideMessages).toHaveBeenCalled();
		expect(els.verifyCode.value).toBe('');
		expect(els.setupSection.classList.add).toHaveBeenCalledWith('hidden');
		expect(els.enableSection.classList.remove).toHaveBeenCalledWith('hidden');
		expect(els.startSetupBtn.disabled).toBe(false);
		expect(els.startSetupBtn.textContent).toBe('Enable 2FA');
	});
});

describe('verifyAndEnable (via form submit)', () => {
	function getVerifyHandler(els: TotpElements): Function {
		return (els.verifyForm.addEventListener as ReturnType<typeof vi.fn>).mock.calls.find(
			(call: unknown[]) => call[0] === 'submit'
		)![1];
	}

	it('should show validation error for invalid code', async () => {
		const { showFieldError } = await import('./form-errors');
		const els = makeMockElements();
		const callbacks = makeCallbacks();
		const state = makeState();
		setupTotp(els, callbacks, state);

		els.verifyCode.value = '123'; // too short

		const handler = getVerifyHandler(els);
		const event = { preventDefault: vi.fn() };
		await handler(event);

		expect(event.preventDefault).toHaveBeenCalled();
		expect(showFieldError).toHaveBeenCalled();
	});

	it('should call API and update state on successful verify', async () => {
		const { csrfFetch } = await import('./csrf');
		const mockCsrfFetch = csrfFetch as ReturnType<typeof vi.fn>;
		mockCsrfFetch.mockResolvedValueOnce({
			ok: true,
			json: async () => ({
				recovery_codes: ['code1', 'code2', 'code3'],
			}),
		});

		const els = makeMockElements();
		const callbacks = makeCallbacks();
		const state = makeState(false);
		setupTotp(els, callbacks, state);

		els.verifyCode.value = '123456';

		const handler = getVerifyHandler(els);
		await handler({ preventDefault: vi.fn() });

		expect(mockCsrfFetch).toHaveBeenCalledWith('/api/auth/totp/verify', expect.objectContaining({
			method: 'POST',
		}));
		expect(state.currentUser!.totp_enabled).toBe(true);
	});

	it('should display recovery codes when provided', async () => {
		const { csrfFetch } = await import('./csrf');
		const mockCsrfFetch = csrfFetch as ReturnType<typeof vi.fn>;
		mockCsrfFetch.mockResolvedValueOnce({
			ok: true,
			json: async () => ({
				recovery_codes: ['AAA-111', 'BBB-222'],
			}),
		});

		const els = makeMockElements();
		const callbacks = makeCallbacks();
		const state = makeState(false);
		setupTotp(els, callbacks, state);

		els.verifyCode.value = '123456';

		const handler = getVerifyHandler(els);
		await handler({ preventDefault: vi.fn() });

		expect(els.recoveryCodesSection.classList.remove).toHaveBeenCalledWith('hidden');
		expect(els.recoveryCodesGrid.appendChild).toHaveBeenCalledTimes(2);
	});

	it('should show error on verify failure', async () => {
		const { csrfFetch } = await import('./csrf');
		const mockCsrfFetch = csrfFetch as ReturnType<typeof vi.fn>;
		mockCsrfFetch.mockResolvedValueOnce({
			ok: false,
			json: async () => ({ error: 'Invalid code' }),
		});

		const els = makeMockElements();
		const callbacks = makeCallbacks();
		const state = makeState(false);
		setupTotp(els, callbacks, state);

		els.verifyCode.value = '123456';

		const handler = getVerifyHandler(els);
		await handler({ preventDefault: vi.fn() });

		expect(callbacks.showError).toHaveBeenCalledWith('Invalid code');
		expect(els.verifyBtn.disabled).toBe(false);
	});
});

describe('disable2FA (via form submit)', () => {
	function getDisableHandler(els: TotpElements): Function {
		return (els.disableForm.addEventListener as ReturnType<typeof vi.fn>).mock.calls.find(
			(call: unknown[]) => call[0] === 'submit'
		)![1];
	}

	it('should validate both password and code', async () => {
		const { showFieldError } = await import('./form-errors');
		const els = makeMockElements();
		const callbacks = makeCallbacks();
		const state = makeState(true);
		setupTotp(els, callbacks, state);

		els.disablePassword.value = 'short';
		els.disableCode.value = '123';

		const handler = getDisableHandler(els);
		await handler({ preventDefault: vi.fn() });

		expect(showFieldError).toHaveBeenCalledTimes(2);
	});

	it('should call API and disable TOTP on success', async () => {
		const { csrfFetch } = await import('./csrf');
		const mockCsrfFetch = csrfFetch as ReturnType<typeof vi.fn>;
		mockCsrfFetch.mockResolvedValueOnce({
			ok: true,
			json: async () => ({ status: 'ok' }),
		});

		const els = makeMockElements();
		const callbacks = makeCallbacks();
		const state = makeState(true);
		setupTotp(els, callbacks, state);

		els.disablePassword.value = 'password123';
		els.disableCode.value = '123456';

		const handler = getDisableHandler(els);
		await handler({ preventDefault: vi.fn() });

		expect(mockCsrfFetch).toHaveBeenCalledWith('/api/auth/totp/disable', expect.objectContaining({
			method: 'POST',
		}));
		expect(state.currentUser!.totp_enabled).toBe(false);
		expect(callbacks.showSuccess).toHaveBeenCalledWith('Two-factor authentication has been disabled.');
	});

	it('should show error on disable failure', async () => {
		const { csrfFetch } = await import('./csrf');
		const mockCsrfFetch = csrfFetch as ReturnType<typeof vi.fn>;
		mockCsrfFetch.mockResolvedValueOnce({
			ok: false,
			json: async () => ({ error: 'Wrong password' }),
		});

		const els = makeMockElements();
		const callbacks = makeCallbacks();
		const state = makeState(true);
		setupTotp(els, callbacks, state);

		els.disablePassword.value = 'password123';
		els.disableCode.value = '123456';

		const handler = getDisableHandler(els);
		await handler({ preventDefault: vi.fn() });

		expect(callbacks.showError).toHaveBeenCalledWith('Wrong password');
	});
});

describe('showRegenForm / cancelRegen (via event listeners)', () => {
	it('showRegenForm should show regen section and hide management', () => {
		const els = makeMockElements();
		const callbacks = makeCallbacks();
		const state = makeState(true);
		setupTotp(els, callbacks, state);

		const handler = (els.startRegenBtn.addEventListener as ReturnType<typeof vi.fn>).mock.calls.find(
			(call: unknown[]) => call[0] === 'click'
		)![1];
		handler();

		expect(callbacks.hideMessages).toHaveBeenCalled();
		expect(els.managementOptions.classList.add).toHaveBeenCalledWith('hidden');
		expect(els.regenSection.classList.remove).toHaveBeenCalledWith('hidden');
		expect(els.regenPassword.focus).toHaveBeenCalled();
	});

	it('cancelRegen should restore management section', () => {
		const els = makeMockElements();
		const callbacks = makeCallbacks();
		const state = makeState(true);
		setupTotp(els, callbacks, state);

		const handler = (els.cancelRegenBtn.addEventListener as ReturnType<typeof vi.fn>).mock.calls.find(
			(call: unknown[]) => call[0] === 'click'
		)![1];
		handler();

		expect(callbacks.hideMessages).toHaveBeenCalled();
		expect(els.regenPassword.value).toBe('');
		expect(els.regenCode.value).toBe('');
		expect(els.managementOptions.classList.remove).toHaveBeenCalledWith('hidden');
		expect(els.regenSection.classList.add).toHaveBeenCalledWith('hidden');
	});
});

describe('showDisableForm / cancelDisable (via event listeners)', () => {
	it('showDisableForm should show disable form section', () => {
		const els = makeMockElements();
		const callbacks = makeCallbacks();
		const state = makeState(true);
		setupTotp(els, callbacks, state);

		const handler = (els.startDisableBtn.addEventListener as ReturnType<typeof vi.fn>).mock.calls.find(
			(call: unknown[]) => call[0] === 'click'
		)![1];
		handler();

		expect(els.managementOptions.classList.add).toHaveBeenCalledWith('hidden');
		expect(els.disableFormSection.classList.remove).toHaveBeenCalledWith('hidden');
		expect(els.disablePassword.focus).toHaveBeenCalled();
	});

	it('cancelDisable should restore management section', () => {
		const els = makeMockElements();
		const callbacks = makeCallbacks();
		const state = makeState(true);
		setupTotp(els, callbacks, state);

		const handler = (els.cancelDisableBtn.addEventListener as ReturnType<typeof vi.fn>).mock.calls.find(
			(call: unknown[]) => call[0] === 'click'
		)![1];
		handler();

		expect(els.disablePassword.value).toBe('');
		expect(els.disableCode.value).toBe('');
		expect(els.managementOptions.classList.remove).toHaveBeenCalledWith('hidden');
		expect(els.disableFormSection.classList.add).toHaveBeenCalledWith('hidden');
	});
});

describe('regenerateCodes (via form submit)', () => {
	function getRegenHandler(els: TotpElements): Function {
		return (els.regenForm.addEventListener as ReturnType<typeof vi.fn>).mock.calls.find(
			(call: unknown[]) => call[0] === 'submit'
		)![1];
	}

	it('should validate inputs before calling API', async () => {
		const { showFieldError } = await import('./form-errors');
		const els = makeMockElements();
		const callbacks = makeCallbacks();
		const state = makeState(true);
		setupTotp(els, callbacks, state);

		els.regenPassword.value = 'short';
		els.regenCode.value = '123';

		const handler = getRegenHandler(els);
		await handler({ preventDefault: vi.fn() });

		expect(showFieldError).toHaveBeenCalledTimes(2);
	});

	it('should call API and display recovery codes on success', async () => {
		const { csrfFetch } = await import('./csrf');
		const mockCsrfFetch = csrfFetch as ReturnType<typeof vi.fn>;
		mockCsrfFetch.mockResolvedValueOnce({
			ok: true,
			json: async () => ({
				recovery_codes: ['NEW-001', 'NEW-002', 'NEW-003'],
			}),
		});

		const els = makeMockElements();
		const callbacks = makeCallbacks();
		const state = makeState(true);
		setupTotp(els, callbacks, state);

		els.regenPassword.value = 'password123';
		els.regenCode.value = '123456';

		const handler = getRegenHandler(els);
		await handler({ preventDefault: vi.fn() });

		expect(mockCsrfFetch).toHaveBeenCalledWith('/api/auth/totp/codes', expect.objectContaining({
			method: 'POST',
		}));
		expect(els.recoveryCodesSection.classList.remove).toHaveBeenCalledWith('hidden');
		expect(els.recoveryCodesGrid.appendChild).toHaveBeenCalledTimes(3);
	});

	it('should show error on regen failure', async () => {
		const { csrfFetch } = await import('./csrf');
		const mockCsrfFetch = csrfFetch as ReturnType<typeof vi.fn>;
		mockCsrfFetch.mockResolvedValueOnce({
			ok: false,
			json: async () => ({ error: 'Auth failed' }),
		});

		const els = makeMockElements();
		const callbacks = makeCallbacks();
		const state = makeState(true);
		setupTotp(els, callbacks, state);

		els.regenPassword.value = 'password123';
		els.regenCode.value = '123456';

		const handler = getRegenHandler(els);
		await handler({ preventDefault: vi.fn() });

		expect(callbacks.showError).toHaveBeenCalledWith('Auth failed');
		expect(els.regenBtn.disabled).toBe(false);
	});
});

describe('savedCodesCheckbox (via event listener)', () => {
	it('should enable continue button when checked', () => {
		const els = makeMockElements();
		const callbacks = makeCallbacks();
		const state = makeState();
		setupTotp(els, callbacks, state);

		const handler = (els.savedCodesCheckbox.addEventListener as ReturnType<typeof vi.fn>).mock.calls.find(
			(call: unknown[]) => call[0] === 'change'
		)![1];

		els.savedCodesCheckbox.checked = true;
		handler();

		expect(els.continueBtn.disabled).toBe(false);
	});

	it('should disable continue button when unchecked', () => {
		const els = makeMockElements();
		const callbacks = makeCallbacks();
		const state = makeState();
		setupTotp(els, callbacks, state);

		const handler = (els.savedCodesCheckbox.addEventListener as ReturnType<typeof vi.fn>).mock.calls.find(
			(call: unknown[]) => call[0] === 'change'
		)![1];

		els.savedCodesCheckbox.checked = false;
		handler();

		expect(els.continueBtn.disabled).toBe(true);
	});
});

describe('finishSetup (via continue button)', () => {
	it('should hide recovery codes and show success for initial setup', () => {
		const els = makeMockElements();
		const callbacks = makeCallbacks();
		const state = makeState(true);
		setupTotp(els, callbacks, state);

		const handler = (els.continueBtn.addEventListener as ReturnType<typeof vi.fn>).mock.calls.find(
			(call: unknown[]) => call[0] === 'click'
		)![1];
		handler();

		expect(els.recoveryCodesSection.classList.add).toHaveBeenCalledWith('hidden');
		expect(callbacks.showSuccess).toHaveBeenCalledWith(
			'Two-factor authentication has been enabled successfully!'
		);
	});
});

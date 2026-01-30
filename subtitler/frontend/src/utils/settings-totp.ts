import { validatePassword, validateTotpCode } from './validation';
import { csrfFetch } from './csrf';
import { clearFieldError, showFieldError } from './form-errors';
import { setupBlurValidation } from './form-validation';
import {
	TotpSetupResponseSchema,
	TotpVerifyResponseSchema,
	RecoveryCodesResponseSchema,
	safeParse,
} from './api-schemas';

export interface TotpElements {
	totpStatus: HTMLElement;
	enableSection: HTMLElement;
	setupSection: HTMLElement;
	disableSection: HTMLElement;
	startSetupBtn: HTMLButtonElement;
	cancelSetupBtn: HTMLButtonElement;
	verifyForm: HTMLFormElement;
	verifyBtn: HTMLButtonElement;
	verifyCode: HTMLInputElement;
	verifyCodeError: HTMLElement;
	disableForm: HTMLFormElement;
	disableBtn: HTMLButtonElement;
	disablePassword: HTMLInputElement;
	disablePasswordError: HTMLElement;
	disableCode: HTMLInputElement;
	disableCodeError: HTMLElement;
	qrCode: HTMLImageElement;
	secretDisplay: HTMLElement;
	recoveryCodesSection: HTMLElement;
	recoveryCodesGrid: HTMLElement;
	copyCodesBtn: HTMLButtonElement;
	downloadCodesBtn: HTMLButtonElement;
	savedCodesCheckbox: HTMLInputElement;
	continueBtn: HTMLButtonElement;
	managementOptions: HTMLElement;
	regenSection: HTMLElement;
	disableFormSection: HTMLElement;
	startRegenBtn: HTMLButtonElement;
	startDisableBtn: HTMLButtonElement;
	cancelRegenBtn: HTMLButtonElement;
	cancelDisableBtn: HTMLButtonElement;
	regenForm: HTMLFormElement;
	regenBtn: HTMLButtonElement;
	regenPassword: HTMLInputElement;
	regenPasswordError: HTMLElement;
	regenCode: HTMLInputElement;
	regenCodeError: HTMLElement;
}

export interface TotpCallbacks {
	showError: (message: string) => void;
	showSuccess: (message: string) => void;
	hideMessages: () => void;
}

export interface TotpState {
	currentUser: { id: string; email: string; totp_enabled: boolean } | null;
}

export function setupTotp(els: TotpElements, callbacks: TotpCallbacks, state: TotpState) {
	let setupSecret = '';
	let recoveryCodes: string[] = [];
	let isRegenerating = false;

	// Set up blur validation
	setupBlurValidation({ input: els.verifyCode, errorEl: els.verifyCodeError, validate: validateTotpCode });
	setupBlurValidation({ input: els.disablePassword, errorEl: els.disablePasswordError, validate: validatePassword });
	setupBlurValidation({ input: els.disableCode, errorEl: els.disableCodeError, validate: validateTotpCode });
	setupBlurValidation({ input: els.regenPassword, errorEl: els.regenPasswordError, validate: validatePassword });
	setupBlurValidation({ input: els.regenCode, errorEl: els.regenCodeError, validate: validateTotpCode });

	function updateUI() {
		if (!state.currentUser) return;

		if (state.currentUser.totp_enabled) {
			els.totpStatus.textContent = 'Enabled';
			els.totpStatus.className = 'status-badge status-enabled';
			els.enableSection.classList.add('hidden');
			els.setupSection.classList.add('hidden');
			els.disableSection.classList.remove('hidden');
			els.managementOptions.classList.remove('hidden');
			els.regenSection.classList.add('hidden');
			els.disableFormSection.classList.add('hidden');
		} else {
			els.totpStatus.textContent = 'Disabled';
			els.totpStatus.className = 'status-badge status-disabled';
			els.enableSection.classList.remove('hidden');
			els.setupSection.classList.add('hidden');
			els.disableSection.classList.add('hidden');
		}
	}

	function displayRecoveryCodes(codes: string[]) {
		recoveryCodes = codes;
		els.recoveryCodesGrid.innerHTML = '';
		codes.forEach(code => {
			const codeDiv = document.createElement('div');
			codeDiv.className = 'recovery-code';
			codeDiv.textContent = code;
			els.recoveryCodesGrid.appendChild(codeDiv);
		});
		els.setupSection.classList.add('hidden');
		els.recoveryCodesSection.classList.remove('hidden');
		els.savedCodesCheckbox.checked = false;
		els.continueBtn.disabled = true;
	}

	async function startSetup() {
		callbacks.hideMessages();
		els.startSetupBtn.disabled = true;
		els.startSetupBtn.textContent = 'Setting up...';

		try {
			const response = await csrfFetch('/api/auth/totp/setup', {
				method: 'POST',
			});

			const rawData = await response.json();

			if (!response.ok) {
				callbacks.showError(rawData.error || 'Failed to start setup');
				els.startSetupBtn.disabled = false;
				els.startSetupBtn.textContent = 'Enable 2FA';
				return;
			}

			const data = safeParse(TotpSetupResponseSchema, rawData);
			if (!data) {
				callbacks.showError('Invalid response from server');
				els.startSetupBtn.disabled = false;
				els.startSetupBtn.textContent = 'Enable 2FA';
				return;
			}

			setupSecret = data.secret;
			els.secretDisplay.textContent = data.secret_display || data.secret;
			els.qrCode.src = data.qr_code;
			els.enableSection.classList.add('hidden');
			els.setupSection.classList.remove('hidden');
			els.verifyCode.focus();

		} catch (err) {
			callbacks.showError('Network error. Please try again.');
			els.startSetupBtn.disabled = false;
			els.startSetupBtn.textContent = 'Enable 2FA';
		}
	}

	function cancelSetup() {
		callbacks.hideMessages();
		setupSecret = '';
		els.verifyCode.value = '';
		els.setupSection.classList.add('hidden');
		els.enableSection.classList.remove('hidden');
		els.startSetupBtn.disabled = false;
		els.startSetupBtn.textContent = 'Enable 2FA';
	}

	async function copyAllCodes() {
		const codesText = recoveryCodes.join('\n');
		try {
			await navigator.clipboard.writeText(codesText);
			els.copyCodesBtn.textContent = 'Copied!';
			setTimeout(() => {
				els.copyCodesBtn.textContent = 'Copy All Codes';
			}, 2000);
		} catch (err) {
			const textarea = document.createElement('textarea');
			textarea.value = codesText;
			document.body.appendChild(textarea);
			textarea.select();
			document.execCommand('copy');
			document.body.removeChild(textarea);
			els.copyCodesBtn.textContent = 'Copied!';
			setTimeout(() => {
				els.copyCodesBtn.textContent = 'Copy All Codes';
			}, 2000);
		}
	}

	function downloadCodes() {
		const codesText = `Subtitler 2FA Recovery Codes
============================
Generated: ${new Date().toISOString()}

Each code can only be used once.
Store these codes in a safe place.

${recoveryCodes.join('\n')}
`;
		const blob = new Blob([codesText], { type: 'text/plain' });
		const url = URL.createObjectURL(blob);
		const link = document.createElement('a');
		link.href = url;
		link.download = 'subtitler-recovery-codes.txt';
		document.body.appendChild(link);
		link.click();
		document.body.removeChild(link);
		URL.revokeObjectURL(url);

		els.downloadCodesBtn.textContent = 'Downloaded!';
		setTimeout(() => {
			els.downloadCodesBtn.textContent = 'Download as File';
		}, 2000);
	}

	function onSavedCodesChange() {
		els.continueBtn.disabled = !els.savedCodesCheckbox.checked;
	}

	function finishSetup() {
		els.recoveryCodesSection.classList.add('hidden');
		recoveryCodes = [];
		if (isRegenerating) {
			isRegenerating = false;
			callbacks.showSuccess('Recovery codes have been regenerated successfully!');
			els.regenBtn.disabled = false;
			els.regenBtn.textContent = 'Regenerate Codes';
		} else {
			callbacks.showSuccess('Two-factor authentication has been enabled successfully!');
		}
		updateUI();
	}

	async function verifyAndEnable(e: Event) {
		e.preventDefault();
		callbacks.hideMessages();

		const code = els.verifyCode.value.trim();
		const codeValidation = validateTotpCode(code);
		if (codeValidation) {
			showFieldError(els.verifyCode, els.verifyCodeError, codeValidation);
			return;
		}

		els.verifyBtn.disabled = true;
		els.verifyBtn.textContent = 'Verifying...';

		try {
			const response = await csrfFetch('/api/auth/totp/verify', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ code }),
			});

			const rawData = await response.json();

			if (!response.ok) {
				callbacks.showError(rawData.error || 'Verification failed');
				els.verifyBtn.disabled = false;
				els.verifyBtn.textContent = 'Verify & Enable';
				return;
			}

			const data = safeParse(TotpVerifyResponseSchema, rawData);
			if (!data) {
				callbacks.showError('Invalid response from server');
				els.verifyBtn.disabled = false;
				els.verifyBtn.textContent = 'Verify & Enable';
				return;
			}

			if (state.currentUser) {
				state.currentUser.totp_enabled = true;
			}
			setupSecret = '';
			els.verifyCode.value = '';

			if (data.recovery_codes && data.recovery_codes.length > 0) {
				displayRecoveryCodes(data.recovery_codes);
			} else {
				callbacks.showSuccess('Two-factor authentication has been enabled successfully!');
				updateUI();
			}

		} catch (err) {
			callbacks.showError('Network error. Please try again.');
			els.verifyBtn.disabled = false;
			els.verifyBtn.textContent = 'Verify & Enable';
		}
	}

	async function disable2FA(e: Event) {
		e.preventDefault();
		callbacks.hideMessages();

		const password = els.disablePassword.value;
		const code = els.disableCode.value.trim();

		let hasErrors = false;

		const passwordValidation = validatePassword(password);
		if (passwordValidation) {
			showFieldError(els.disablePassword, els.disablePasswordError, passwordValidation);
			hasErrors = true;
		}

		const codeValidation = validateTotpCode(code);
		if (codeValidation) {
			showFieldError(els.disableCode, els.disableCodeError, codeValidation);
			hasErrors = true;
		}

		if (hasErrors) {
			return;
		}

		els.disableBtn.disabled = true;
		els.disableBtn.textContent = 'Disabling...';

		try {
			const response = await csrfFetch('/api/auth/totp/disable', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ password, code }),
			});

			const data = await response.json();

			if (!response.ok) {
				callbacks.showError(data.error || 'Failed to disable 2FA');
				els.disableBtn.disabled = false;
				els.disableBtn.textContent = 'Disable 2FA';
				return;
			}

			if (state.currentUser) {
				state.currentUser.totp_enabled = false;
			}
			els.disablePassword.value = '';
			els.disableCode.value = '';
			callbacks.showSuccess('Two-factor authentication has been disabled.');
			updateUI();

		} catch (err) {
			callbacks.showError('Network error. Please try again.');
		} finally {
			els.disableBtn.disabled = false;
			els.disableBtn.textContent = 'Disable 2FA';
		}
	}

	function showRegenForm() {
		callbacks.hideMessages();
		els.managementOptions.classList.add('hidden');
		els.regenSection.classList.remove('hidden');
		els.disableFormSection.classList.add('hidden');
		els.regenPassword.focus();
	}

	function showDisableForm() {
		callbacks.hideMessages();
		els.managementOptions.classList.add('hidden');
		els.regenSection.classList.add('hidden');
		els.disableFormSection.classList.remove('hidden');
		els.disablePassword.focus();
	}

	function cancelRegen() {
		callbacks.hideMessages();
		els.regenPassword.value = '';
		els.regenCode.value = '';
		els.managementOptions.classList.remove('hidden');
		els.regenSection.classList.add('hidden');
	}

	function cancelDisable() {
		callbacks.hideMessages();
		els.disablePassword.value = '';
		els.disableCode.value = '';
		els.managementOptions.classList.remove('hidden');
		els.disableFormSection.classList.add('hidden');
	}

	async function regenerateCodes(e: Event) {
		e.preventDefault();
		callbacks.hideMessages();

		const password = els.regenPassword.value;
		const code = els.regenCode.value.trim();

		let hasErrors = false;

		const passwordValidation = validatePassword(password);
		if (passwordValidation) {
			showFieldError(els.regenPassword, els.regenPasswordError, passwordValidation);
			hasErrors = true;
		}

		const codeValidation = validateTotpCode(code);
		if (codeValidation) {
			showFieldError(els.regenCode, els.regenCodeError, codeValidation);
			hasErrors = true;
		}

		if (hasErrors) {
			return;
		}

		els.regenBtn.disabled = true;
		els.regenBtn.textContent = 'Regenerating...';

		try {
			const response = await csrfFetch('/api/auth/totp/codes', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ password, code }),
			});

			const rawData = await response.json();

			if (!response.ok) {
				callbacks.showError(rawData.error || 'Failed to regenerate recovery codes');
				els.regenBtn.disabled = false;
				els.regenBtn.textContent = 'Regenerate Codes';
				return;
			}

			const data = safeParse(RecoveryCodesResponseSchema, rawData);
			if (!data) {
				callbacks.showError('Invalid response from server');
				els.regenBtn.disabled = false;
				els.regenBtn.textContent = 'Regenerate Codes';
				return;
			}

			els.regenPassword.value = '';
			els.regenCode.value = '';
			els.regenSection.classList.add('hidden');
			isRegenerating = true;
			displayRecoveryCodes(data.recovery_codes);

		} catch (err) {
			callbacks.showError('Network error. Please try again.');
			els.regenBtn.disabled = false;
			els.regenBtn.textContent = 'Regenerate Codes';
		}
	}

	// Wire up event listeners
	els.startSetupBtn.addEventListener('click', startSetup);
	els.cancelSetupBtn.addEventListener('click', cancelSetup);
	els.verifyForm.addEventListener('submit', verifyAndEnable);
	els.disableForm.addEventListener('submit', disable2FA);
	els.copyCodesBtn.addEventListener('click', copyAllCodes);
	els.downloadCodesBtn.addEventListener('click', downloadCodes);
	els.savedCodesCheckbox.addEventListener('change', onSavedCodesChange);
	els.continueBtn.addEventListener('click', finishSetup);
	els.startRegenBtn.addEventListener('click', showRegenForm);
	els.startDisableBtn.addEventListener('click', showDisableForm);
	els.cancelRegenBtn.addEventListener('click', cancelRegen);
	els.cancelDisableBtn.addEventListener('click', cancelDisable);
	els.regenForm.addEventListener('submit', regenerateCodes);

	return { updateUI };
}

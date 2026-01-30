/**
 * Styled dialog utilities to replace native alert() and confirm()
 * Provides consistent styling, accessibility, and better UX
 */


// Dialog container singleton - created once and reused
// Event listeners are added once during container creation and persist for the page lifetime
// This is intentional: the singleton pattern means listeners don't accumulate
let dialogContainer: HTMLElement | null = null;
let activeDialog: { resolve: (value: boolean) => void } | null = null;

// Store references to event handlers for cleanup
let cancelClickHandler: (() => void) | null = null;
let confirmClickHandler: (() => void) | null = null;
let overlayClickHandler: ((e: MouseEvent) => void) | null = null;
let keydownHandler: ((e: KeyboardEvent) => void) | null = null;

/**
 * Dialog options for customization
 */
interface DialogOptions {
	title?: string;
	message: string;
	confirmText?: string;
	cancelText?: string;
	type?: 'info' | 'warning' | 'error' | 'confirm';
}

/**
 * Ensure the dialog container exists in the DOM
 */
function ensureContainer(): HTMLElement {
	if (!dialogContainer) {
		dialogContainer = document.createElement('div');
		dialogContainer.id = 'dialog-container';
		dialogContainer.className = 'dialog-overlay';
		dialogContainer.setAttribute('role', 'dialog');
		dialogContainer.setAttribute('aria-modal', 'true');
		dialogContainer.innerHTML = `
			<div class="dialog-content">
				<div class="dialog-header">
					<h3 class="dialog-title" id="dialog-title"></h3>
				</div>
				<div class="dialog-body">
					<p class="dialog-message" id="dialog-message"></p>
				</div>
				<div class="dialog-footer">
					<button class="dialog-btn dialog-btn-cancel" id="dialog-cancel" style="display: none;">Cancel</button>
					<button class="dialog-btn dialog-btn-confirm" id="dialog-confirm">OK</button>
				</div>
			</div>
		`;
		document.body.appendChild(dialogContainer);

		// Add styles if not already present
		if (!document.getElementById('dialog-styles')) {
			const style = document.createElement('style');
			style.id = 'dialog-styles';
			style.textContent = getDialogStyles();
			document.head.appendChild(style);
		}

		// Set up event listeners - store references for potential cleanup
		const cancelBtn = dialogContainer.querySelector('#dialog-cancel') as HTMLButtonElement;
		const confirmBtn = dialogContainer.querySelector('#dialog-confirm') as HTMLButtonElement;

		cancelClickHandler = () => closeDialog(false);
		confirmClickHandler = () => closeDialog(true);
		overlayClickHandler = (e: MouseEvent) => {
			if (e.target === dialogContainer) {
				closeDialog(false);
			}
		};
		keydownHandler = (e: KeyboardEvent) => {
			if (e.key === 'Escape') {
				closeDialog(false);
			}
		};

		cancelBtn.addEventListener('click', cancelClickHandler);
		confirmBtn.addEventListener('click', confirmClickHandler);
		dialogContainer.addEventListener('click', overlayClickHandler);
		dialogContainer.addEventListener('keydown', keydownHandler);
	}
	return dialogContainer;
}

/**
 * Close the dialog and resolve the promise
 */
function closeDialog(confirmed: boolean): void {
	if (dialogContainer) {
		dialogContainer.classList.remove('visible');
		document.body.style.overflow = '';
	}
	if (activeDialog) {
		activeDialog.resolve(confirmed);
		activeDialog = null;
	}
}

/**
 * Destroy the dialog container and clean up all event listeners
 * Useful for testing or when the dialog system is no longer needed
 */
export function destroyDialog(): void {
	if (dialogContainer) {
		// Remove event listeners
		const cancelBtn = dialogContainer.querySelector('#dialog-cancel') as HTMLButtonElement;
		const confirmBtn = dialogContainer.querySelector('#dialog-confirm') as HTMLButtonElement;

		if (cancelClickHandler) {
			cancelBtn?.removeEventListener('click', cancelClickHandler);
			cancelClickHandler = null;
		}
		if (confirmClickHandler) {
			confirmBtn?.removeEventListener('click', confirmClickHandler);
			confirmClickHandler = null;
		}
		if (overlayClickHandler) {
			dialogContainer.removeEventListener('click', overlayClickHandler);
			overlayClickHandler = null;
		}
		if (keydownHandler) {
			dialogContainer.removeEventListener('keydown', keydownHandler);
			keydownHandler = null;
		}

		// Remove from DOM
		dialogContainer.remove();
		dialogContainer = null;
	}

	// Remove styles
	const styles = document.getElementById('dialog-styles');
	if (styles) {
		styles.remove();
	}

	// Resolve any pending dialog
	if (activeDialog) {
		activeDialog.resolve(false);
		activeDialog = null;
	}
}

/**
 * Show a dialog with the given options
 */
function showDialog(options: DialogOptions): Promise<boolean> {
	const container = ensureContainer();
	const title = container.querySelector('#dialog-title') as HTMLElement;
	const message = container.querySelector('#dialog-message') as HTMLElement;
	const cancelBtn = container.querySelector('#dialog-cancel') as HTMLButtonElement;
	const confirmBtn = container.querySelector('#dialog-confirm') as HTMLButtonElement;

	// Set content (use textContent for both - it's safe and simpler than escapeHtml + innerHTML)
	title.textContent = options.title || getDefaultTitle(options.type);
	message.textContent = options.message;

	// Configure buttons
	confirmBtn.textContent = options.confirmText || 'OK';
	cancelBtn.textContent = options.cancelText || 'Cancel';
	cancelBtn.style.display = options.type === 'confirm' ? '' : 'none';

	// Set dialog type for styling
	const content = container.querySelector('.dialog-content') as HTMLElement;
	content.className = 'dialog-content';
	if (options.type) {
		content.classList.add(`dialog-${options.type}`);
	}

	// Show dialog
	container.classList.add('visible');
	document.body.style.overflow = 'hidden';

	// Focus the confirm button for keyboard users
	confirmBtn.focus();

	// Return promise that resolves when dialog closes
	return new Promise((resolve) => {
		activeDialog = { resolve };
	});
}

/**
 * Get default title based on dialog type
 */
function getDefaultTitle(type?: string): string {
	switch (type) {
		case 'error':
			return 'Error';
		case 'warning':
			return 'Warning';
		case 'confirm':
			return 'Confirm';
		default:
			return 'Notice';
	}
}

/**
 * Show an alert dialog (replacement for window.alert)
 */
export async function showAlert(message: string, type: 'info' | 'warning' | 'error' = 'info'): Promise<void> {
	await showDialog({
		message,
		type,
		title: getDefaultTitle(type),
	});
}

/**
 * Show a confirmation dialog (replacement for window.confirm)
 */
export async function showConfirm(message: string, options?: {
	title?: string;
	confirmText?: string;
	cancelText?: string;
}): Promise<boolean> {
	return showDialog({
		message,
		type: 'confirm',
		title: options?.title || 'Confirm',
		confirmText: options?.confirmText || 'Confirm',
		cancelText: options?.cancelText || 'Cancel',
	});
}

/**
 * Get the CSS styles for the dialog
 */
function getDialogStyles(): string {
	return `
		.dialog-overlay {
			position: fixed;
			inset: 0;
			background: rgba(0, 0, 0, 0.5);
			display: flex;
			align-items: center;
			justify-content: center;
			z-index: 10000;
			opacity: 0;
			visibility: hidden;
			transition: opacity 0.2s, visibility 0.2s;
		}

		.dialog-overlay.visible {
			opacity: 1;
			visibility: visible;
		}

		.dialog-content {
			background: var(--bg-primary, #ffffff);
			border-radius: 12px;
			box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 10px 10px -5px rgba(0, 0, 0, 0.04);
			max-width: 400px;
			width: 90%;
			animation: dialog-enter 0.2s ease-out;
		}

		@keyframes dialog-enter {
			from {
				opacity: 0;
				transform: scale(0.95) translateY(-10px);
			}
			to {
				opacity: 1;
				transform: scale(1) translateY(0);
			}
		}

		.dialog-header {
			padding: 1.25rem 1.5rem 0.75rem;
			border-bottom: 1px solid var(--border-color, #e5e7eb);
		}

		.dialog-title {
			margin: 0;
			font-size: 1.125rem;
			font-weight: 600;
			color: var(--text-primary, #111827);
		}

		.dialog-error .dialog-title {
			color: var(--error-color, #dc2626);
		}

		.dialog-warning .dialog-title {
			color: var(--warning-color, #d97706);
		}

		.dialog-body {
			padding: 1rem 1.5rem;
		}

		.dialog-message {
			margin: 0;
			font-size: 0.95rem;
			color: var(--text-secondary, #4b5563);
			line-height: 1.5;
			white-space: pre-wrap;
			word-break: break-word;
		}

		.dialog-footer {
			display: flex;
			justify-content: flex-end;
			gap: 0.75rem;
			padding: 1rem 1.5rem;
			border-top: 1px solid var(--border-color, #e5e7eb);
		}

		.dialog-btn {
			padding: 0.625rem 1.25rem;
			border-radius: 8px;
			font-size: 0.9rem;
			font-weight: 500;
			cursor: pointer;
			transition: all 0.15s;
			border: none;
		}

		.dialog-btn:focus {
			outline: 2px solid var(--accent-color, #3b82f6);
			outline-offset: 2px;
		}

		.dialog-btn-cancel {
			background: var(--bg-secondary, #f3f4f6);
			color: var(--text-primary, #374151);
		}

		.dialog-btn-cancel:hover {
			background: var(--bg-hover, #e5e7eb);
		}

		.dialog-btn-confirm {
			background: var(--accent-color, #3b82f6);
			color: white;
		}

		.dialog-btn-confirm:hover {
			background: var(--accent-hover, #2563eb);
		}

		.dialog-error .dialog-btn-confirm {
			background: var(--error-color, #dc2626);
		}

		.dialog-error .dialog-btn-confirm:hover {
			background: var(--error-hover, #b91c1c);
		}

		.dialog-confirm .dialog-btn-confirm {
			background: var(--accent-color, #3b82f6);
		}

		/* Dark mode support */
		@media (prefers-color-scheme: dark) {
			:root:not([data-theme="light"]) .dialog-content {
				background: var(--bg-primary, #1f2937);
				box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.3), 0 10px 10px -5px rgba(0, 0, 0, 0.2);
			}

			:root:not([data-theme="light"]) .dialog-title {
				color: var(--text-primary, #f9fafb);
			}

			:root:not([data-theme="light"]) .dialog-message {
				color: var(--text-secondary, #d1d5db);
			}

			:root:not([data-theme="light"]) .dialog-header,
			:root:not([data-theme="light"]) .dialog-footer {
				border-color: var(--border-color, #374151);
			}

			:root:not([data-theme="light"]) .dialog-btn-cancel {
				background: var(--bg-secondary, #374151);
				color: var(--text-primary, #f9fafb);
			}

			:root:not([data-theme="light"]) .dialog-btn-cancel:hover {
				background: var(--bg-hover, #4b5563);
			}
		}

		[data-theme="dark"] .dialog-content {
			background: var(--bg-primary, #1f2937);
			box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.3), 0 10px 10px -5px rgba(0, 0, 0, 0.2);
		}

		[data-theme="dark"] .dialog-title {
			color: var(--text-primary, #f9fafb);
		}

		[data-theme="dark"] .dialog-message {
			color: var(--text-secondary, #d1d5db);
		}

		[data-theme="dark"] .dialog-header,
		[data-theme="dark"] .dialog-footer {
			border-color: var(--border-color, #374151);
		}

		[data-theme="dark"] .dialog-btn-cancel {
			background: var(--bg-secondary, #374151);
			color: var(--text-primary, #f9fafb);
		}

		[data-theme="dark"] .dialog-btn-cancel:hover {
			background: var(--bg-hover, #4b5563);
		}
	`;
}

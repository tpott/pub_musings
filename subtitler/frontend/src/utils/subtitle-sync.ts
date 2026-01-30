/**
 * Subtitle synchronization, burn subtitles, and keyboard shortcuts
 * for the upload page.
 */

import { csrfFetch } from './csrf';
import { renderBionicText, isBionicEnabled } from './bionic';
import {
	PLAYBACK_SPEEDS,
	getSavedSpeed,
	setPlaybackSpeed,
	increaseSpeed,
	decreaseSpeed,
	formatSpeed
} from './playback-speed';
import type { PlaybackSpeed } from './playback-speed';
import { formatDuration } from './transcription-polling';
import {
	updateFeedbackButtons,
	scrollIntoContainerView,
	isTextInputFocused,
	getCurrentSegmentIndex,
	navigateToSegment,
	performUndo,
	performRedo
} from './segment-editor';
import type { SegmentEditorState, SegmentEditorElements } from './segment-editor';

// --- Subtitle sync ---

/** Set up video timeupdate handler for subtitle syncing */
export function setupSubtitleSync(
	previewVideo: HTMLVideoElement,
	updateFn: () => void,
	syncState: { active: boolean }
): void {
	if (syncState.active) {
		previewVideo.removeEventListener('timeupdate', updateFn);
	}
	previewVideo.addEventListener('timeupdate', updateFn);
	syncState.active = true;
}

/** Create the updateCurrentSubtitle function for a given page context */
export function createSubtitleUpdater(
	state: SegmentEditorState,
	previewVideo: HTMLVideoElement,
	currentSubtitle: HTMLElement,
	subtitleFeedback: HTMLElement,
	segmentsContainer: HTMLElement
): () => void {
	return function updateCurrentSubtitle() {
		const currentTime = previewVideo.currentTime;

		let currentSeg = null;
		let currentIndex = -1;

		for (let i = 0; i < state.transcriptionSegments.length; i++) {
			const seg = state.transcriptionSegments[i];
			if (currentTime >= seg.start && currentTime <= seg.end) {
				currentSeg = seg;
				currentIndex = i;
				break;
			}
		}

		if (currentSeg) {
			const useBionic = isBionicEnabled();
			if (useBionic) {
				currentSubtitle.innerHTML = renderBionicText(currentSeg.text.trim(), { enabled: true });
			} else {
				currentSubtitle.textContent = currentSeg.text.trim();
			}
		} else {
			currentSubtitle.textContent = '';
		}

		if (currentIndex !== state.lastActiveSegmentIndex) {
			if (state.lastActiveSegmentIndex >= 0 && state.lastActiveSegmentIndex < state.cachedSegmentElements.length) {
				state.cachedSegmentElements[state.lastActiveSegmentIndex].classList.remove('active');
			}
			if (currentIndex >= 0 && currentIndex < state.cachedSegmentElements.length) {
				state.cachedSegmentElements[currentIndex].classList.add('active');
				scrollIntoContainerView(segmentsContainer, state.cachedSegmentElements[currentIndex]);
			}
			state.lastActiveSegmentIndex = currentIndex;

			updateFeedbackButtons(state, subtitleFeedback, currentIndex);
		}
	};
}

// --- Playback speed UI ---

export interface SpeedUIState {
	speedMenuOpen: boolean;
	speedIndicatorTimeout: ReturnType<typeof setTimeout> | null;
	/** AbortController for document-level listeners; abort to clean up */
	abortController: AbortController | null;
}

export function createSpeedUIState(): SpeedUIState {
	return {
		speedMenuOpen: false,
		speedIndicatorTimeout: null,
		abortController: null
	};
}

export function updateSpeedUI(
	speedBtn: HTMLButtonElement,
	speedOptions: HTMLElement,
	speed: number
): void {
	speedBtn.textContent = formatSpeed(speed);
	speedOptions.querySelectorAll('.speed-option').forEach(option => {
		const optionSpeed = parseFloat((option as HTMLElement).dataset.speed || '1');
		if (optionSpeed === speed) {
			option.classList.add('active');
		} else {
			option.classList.remove('active');
		}
	});
}

export function showSpeedIndicator(
	speedIndicator: HTMLElement,
	speedState: SpeedUIState,
	speed: number
): void {
	speedIndicator.textContent = formatSpeed(speed);
	speedIndicator.classList.add('visible');

	if (speedState.speedIndicatorTimeout) {
		clearTimeout(speedState.speedIndicatorTimeout);
	}

	speedState.speedIndicatorTimeout = setTimeout(() => {
		speedIndicator.classList.remove('visible');
		speedState.speedIndicatorTimeout = null;
	}, 800);
}

function toggleSpeedMenu(
	speedState: SpeedUIState,
	speedBtn: HTMLButtonElement,
	speedOptions: HTMLElement
): void {
	speedState.speedMenuOpen = !speedState.speedMenuOpen;
	speedOptions.classList.toggle('visible', speedState.speedMenuOpen);
	speedBtn.setAttribute('aria-expanded', speedState.speedMenuOpen.toString());
}

function closeSpeedMenu(
	speedState: SpeedUIState,
	speedBtn: HTMLButtonElement,
	speedOptions: HTMLElement
): void {
	speedState.speedMenuOpen = false;
	speedOptions.classList.remove('visible');
	speedBtn.setAttribute('aria-expanded', 'false');
}

/** Set up all speed control event listeners */
export function setupSpeedControls(
	previewVideo: HTMLVideoElement,
	speedBtn: HTMLButtonElement,
	speedOptions: HTMLElement,
	speedDropdown: HTMLElement,
	speedState: SpeedUIState
): void {
	// Initialize speed from localStorage
	const savedSpeed = getSavedSpeed();
	previewVideo.playbackRate = savedSpeed;
	updateSpeedUI(speedBtn, speedOptions, savedSpeed);

	speedBtn.addEventListener('click', (e) => {
		e.stopPropagation();
		toggleSpeedMenu(speedState, speedBtn, speedOptions);
	});

	speedOptions.addEventListener('click', (e) => {
		const target = e.target as HTMLElement;
		if (target.classList.contains('speed-option')) {
			const speed = parseFloat(target.dataset.speed || '1');
			if (PLAYBACK_SPEEDS.includes(speed as PlaybackSpeed)) {
				setPlaybackSpeed(previewVideo, speed as PlaybackSpeed);
				updateSpeedUI(speedBtn, speedOptions, speed);
			}
			closeSpeedMenu(speedState, speedBtn, speedOptions);
		}
	});

	// Use AbortController for document-level listeners so they can be cleaned up
	if (speedState.abortController) {
		speedState.abortController.abort();
	}
	speedState.abortController = new AbortController();
	const { signal } = speedState.abortController;

	document.addEventListener('click', (e) => {
		if (speedState.speedMenuOpen && !speedDropdown.contains(e.target as Node)) {
			closeSpeedMenu(speedState, speedBtn, speedOptions);
		}
	}, { signal });

	document.addEventListener('keydown', (e) => {
		if (e.key === 'Escape' && speedState.speedMenuOpen) {
			closeSpeedMenu(speedState, speedBtn, speedOptions);
			speedBtn.focus();
		}
	}, { signal });
}

// --- Burn subtitles ---

export interface BurnState {
	isBurning: boolean;
	burnPollTimeout: ReturnType<typeof setTimeout> | null;
	fetchAbortController: AbortController | null;
}

export function createBurnState(): BurnState {
	return {
		isBurning: false,
		burnPollTimeout: null,
		fetchAbortController: null
	};
}

export interface BurnElements {
	burnBtn: HTMLButtonElement;
	burnStatus: HTMLElement;
	burnStatusText: HTMLElement;
	burnEtaText: HTMLElement;
	burnProgressFill: HTMLElement;
	downloadBurnedBtn: HTMLAnchorElement;
}

export async function startBurn(
	uploadId: string,
	burnState: BurnState,
	els: BurnElements
): Promise<void> {
	if (burnState.isBurning) return;

	burnState.isBurning = true;
	els.burnBtn.disabled = true;
	els.burnBtn.textContent = 'Burning...';
	els.burnStatus.classList.add('visible');
	els.burnStatus.classList.remove('complete', 'error');
	els.burnStatusText.textContent = 'Starting subtitle burn...';
	els.burnEtaText.textContent = 'Calculating time remaining...';
	els.burnEtaText.style.display = 'block';
	els.burnProgressFill.style.width = '0%';
	els.downloadBurnedBtn.style.display = 'none';

	try {
		const response = await csrfFetch(`/api/videos/${uploadId}/burn`, {
			method: 'POST',
		});
		const result = await response.json();

		if (!response.ok) {
			throw new Error(result.error || 'Failed to start burn');
		}

		pollBurnStatus(uploadId, burnState, els);
	} catch (error) {
		els.burnStatus.classList.add('error');
		els.burnStatusText.textContent = `Error: ${error instanceof Error ? error.message : error}`;
		els.burnBtn.disabled = false;
		els.burnBtn.textContent = 'Burn Subtitles into Video';
		burnState.isBurning = false;
	}
}

function pollBurnStatus(
	uploadId: string,
	burnState: BurnState,
	els: BurnElements
): void {
	if (burnState.burnPollTimeout !== null) {
		clearTimeout(burnState.burnPollTimeout);
		burnState.burnPollTimeout = null;
	}

	const poll = async () => {
		try {
			burnState.fetchAbortController = new AbortController();
			const response = await fetch(`/api/videos/${uploadId}/burn`, {
				signal: burnState.fetchAbortController.signal
			});
			const result = await response.json();

			els.burnStatusText.textContent = result.message || 'Processing...';
			els.burnProgressFill.style.width = `${result.progress || 0}%`;

			if (result.estimated_remaining_seconds !== undefined && result.estimated_remaining_seconds > 0) {
				els.burnEtaText.textContent = `Estimated time remaining: ${formatDuration(result.estimated_remaining_seconds)}`;
				els.burnEtaText.style.display = 'block';
			} else if (result.status === 'processing' && result.progress < 20) {
				els.burnEtaText.textContent = 'Calculating time remaining...';
				els.burnEtaText.style.display = 'block';
			} else {
				els.burnEtaText.style.display = 'none';
			}

			if (result.status === 'processing') {
				burnState.burnPollTimeout = setTimeout(poll, 2000);
			} else if (result.status === 'complete') {
				burnState.burnPollTimeout = null;
				els.burnStatus.classList.add('complete');
				els.burnStatusText.textContent = 'Subtitles burned successfully!';
				els.burnEtaText.style.display = 'none';
				els.burnProgressFill.style.width = '100%';
				els.downloadBurnedBtn.href = `/api/videos/${uploadId}/burned`;
				els.downloadBurnedBtn.style.display = 'inline-block';
				els.burnBtn.disabled = false;
				els.burnBtn.textContent = 'Burn Again';
				burnState.isBurning = false;
			} else if (result.status === 'error') {
				burnState.burnPollTimeout = null;
				els.burnStatus.classList.add('error');
				els.burnStatusText.textContent = `Error: ${result.message}`;
				els.burnEtaText.style.display = 'none';
				els.burnBtn.disabled = false;
				els.burnBtn.textContent = 'Retry Burn';
				burnState.isBurning = false;
			}
		} catch (error) {
			if (error instanceof Error && error.name === 'AbortError') {
				return;
			}
			burnState.burnPollTimeout = null;
			els.burnStatus.classList.add('error');
			els.burnStatusText.textContent = `Polling error: ${error}`;
			els.burnBtn.disabled = false;
			els.burnBtn.textContent = 'Retry Burn';
			burnState.isBurning = false;
		}
	};

	poll();
}

// --- Keyboard shortcuts ---

export interface KeyboardShortcutElements {
	keyboardHelpBtn: HTMLButtonElement;
	keyboardShortcutsModal: HTMLElement;
	modalClose: HTMLButtonElement;
	videoPreview: HTMLElement;
	previewVideo: HTMLVideoElement;
	speedBtn: HTMLButtonElement;
	speedOptions: HTMLElement;
	speedIndicator: HTMLElement;
}

export function setupKeyboardShortcuts(
	editorState: SegmentEditorState,
	editorEls: SegmentEditorElements,
	kbEls: KeyboardShortcutElements,
	speedState: SpeedUIState
): void {
	function openKeyboardModal() {
		kbEls.keyboardShortcutsModal.classList.add('visible');
		kbEls.modalClose.focus();
		document.addEventListener('keydown', trapFocus);
	}

	function closeKeyboardModal() {
		kbEls.keyboardShortcutsModal.classList.remove('visible');
		kbEls.keyboardHelpBtn.focus();
		document.removeEventListener('keydown', trapFocus);
	}

	function trapFocus(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			closeKeyboardModal();
			return;
		}
		if (e.key !== 'Tab') return;

		const focusableElements = kbEls.keyboardShortcutsModal.querySelectorAll(
			'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
		);
		const firstElement = focusableElements[0] as HTMLElement;
		const lastElement = focusableElements[focusableElements.length - 1] as HTMLElement;

		if (e.shiftKey && document.activeElement === firstElement) {
			e.preventDefault();
			lastElement.focus();
		} else if (!e.shiftKey && document.activeElement === lastElement) {
			e.preventDefault();
			firstElement.focus();
		}
	}

	kbEls.keyboardHelpBtn.addEventListener('click', openKeyboardModal);
	kbEls.modalClose.addEventListener('click', closeKeyboardModal);
	kbEls.keyboardShortcutsModal.addEventListener('click', (e) => {
		if (e.target === kbEls.keyboardShortcutsModal) {
			closeKeyboardModal();
		}
	});

	// Global keyboard shortcuts
	document.addEventListener('keydown', (e) => {
		if (kbEls.keyboardShortcutsModal.classList.contains('visible')) {
			return;
		}

		// Undo/redo shortcuts (edit mode only)
		if (editorState.isEditMode) {
			if ((e.ctrlKey || e.metaKey) && e.key === 'z' && !e.shiftKey) {
				e.preventDefault();
				performUndo(editorState, editorEls);
				return;
			}
			if ((e.ctrlKey || e.metaKey) && ((e.key === 'z' && e.shiftKey) || e.key === 'y')) {
				e.preventDefault();
				performRedo(editorState, editorEls);
				return;
			}
		}

		// Tab/Shift+Tab for segment navigation
		if (e.key === 'Tab' && editorState.transcriptionSegments.length > 0) {
			if (!kbEls.videoPreview.classList.contains('visible')) return;

			e.preventDefault();
			const currentIndex = getCurrentSegmentIndex(
				editorState.transcriptionSegments,
				kbEls.previewVideo.currentTime
			);
			if (e.shiftKey) {
				navigateToSegment(editorState, editorEls, kbEls.previewVideo, currentIndex - 1);
			} else {
				navigateToSegment(editorState, editorEls, kbEls.previewVideo, currentIndex + 1);
			}
			return;
		}

		// Skip other shortcuts if in a text input
		if (isTextInputFocused()) return;

		// Video playback shortcuts (only when video is visible)
		if (!kbEls.videoPreview.classList.contains('visible')) return;

		switch (e.key) {
			case ' ':
				e.preventDefault();
				if (kbEls.previewVideo.paused) {
					kbEls.previewVideo.play().catch(() => {});
				} else {
					kbEls.previewVideo.pause();
				}
				break;
			case 'ArrowLeft':
				e.preventDefault();
				kbEls.previewVideo.currentTime = Math.max(0, kbEls.previewVideo.currentTime - 5);
				break;
			case 'ArrowRight':
				e.preventDefault();
				kbEls.previewVideo.currentTime = Math.min(kbEls.previewVideo.duration, kbEls.previewVideo.currentTime + 5);
				break;
			case 'j':
			case 'J':
				e.preventDefault();
				kbEls.previewVideo.currentTime = Math.max(0, kbEls.previewVideo.currentTime - 10);
				break;
			case 'k':
			case 'K':
				e.preventDefault();
				kbEls.previewVideo.pause();
				break;
			case 'l':
			case 'L':
				e.preventDefault();
				kbEls.previewVideo.currentTime = Math.min(kbEls.previewVideo.duration, kbEls.previewVideo.currentTime + 10);
				break;
			case '[':
				e.preventDefault();
				{
					const newSpeed = decreaseSpeed(kbEls.previewVideo.playbackRate);
					setPlaybackSpeed(kbEls.previewVideo, newSpeed);
					updateSpeedUI(kbEls.speedBtn as HTMLButtonElement, kbEls.speedOptions, newSpeed);
					showSpeedIndicator(kbEls.speedIndicator, speedState, newSpeed);
				}
				break;
			case ']':
				e.preventDefault();
				{
					const newSpeed = increaseSpeed(kbEls.previewVideo.playbackRate);
					setPlaybackSpeed(kbEls.previewVideo, newSpeed);
					updateSpeedUI(kbEls.speedBtn as HTMLButtonElement, kbEls.speedOptions, newSpeed);
					showSpeedIndicator(kbEls.speedIndicator, speedState, newSpeed);
				}
				break;
			case '?':
				e.preventDefault();
				openKeyboardModal();
				break;
		}
	});
}

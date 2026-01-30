/**
 * Video modal playback, subtitle sync, and keyboard handling for the videos page.
 * Manages the modal lifecycle: open, close, subtitle display, and speed controls.
 */

import { renderBionicText, isBionicEnabled } from './bionic';
import { escapeHtml } from './html';
import {
	PLAYBACK_SPEEDS,
	type PlaybackSpeed,
	getSavedSpeed,
	setPlaybackSpeed,
	increaseSpeed,
	decreaseSpeed,
	formatSpeed
} from './playback-speed';
import { openSRT, openVTT, openJSON } from './subtitles';
import { showAlert } from './dialog';
import { fetchWithTimeout } from './fetch-timeout';
import { formatTime } from './transcription-polling';
import type { TranscriptionSegment } from '../types/transcription';
import { TranscriptionStatusResponseSchema, safeParse } from './api-schemas';

export interface ModalElements {
	videoModal: HTMLElement;
	modalClose: HTMLButtonElement;
	modalTitle: HTMLElement;
	modalLoading: HTMLElement;
	modalError: HTMLElement;
	modalVideoContainer: HTMLElement;
	modalVideo: HTMLVideoElement;
	modalCurrentSubtitle: HTMLElement;
	modalSegments: HTMLElement;
	modalDownloadSrt: HTMLAnchorElement;
	modalDownloadVtt: HTMLAnchorElement;
	modalDownloadJson: HTMLAnchorElement;
	modalEditLink: HTMLAnchorElement;
	modalSpeedBtn: HTMLButtonElement;
	modalSpeedOptions: HTMLElement;
	modalSpeedDropdown: HTMLElement;
	modalSpeedIndicator: HTMLElement;
	modalKidModeBtn: HTMLButtonElement;
}

export interface ModalState {
	currentVideoId: string | null;
	subtitleSegments: TranscriptionSegment[];
	subtitleSyncActive: boolean;
	triggerElement: HTMLElement | null;
	keydownController: AbortController | null;
	cachedSegmentElements: HTMLElement[];
	lastActiveSegmentIndex: number;
	speedMenuOpen: boolean;
	speedIndicatorTimeout: ReturnType<typeof setTimeout> | null;
	kidModeLocked: boolean;
	kidModeUnlockTimeout: ReturnType<typeof setTimeout> | null;
}

export function createModalState(): ModalState {
	return {
		currentVideoId: null,
		subtitleSegments: [],
		subtitleSyncActive: false,
		triggerElement: null,
		keydownController: null,
		cachedSegmentElements: [],
		lastActiveSegmentIndex: -1,
		speedMenuOpen: false,
		speedIndicatorTimeout: null,
		kidModeLocked: false,
		kidModeUnlockTimeout: null,
	};
}

function activateSegment(segmentEl: HTMLElement, modalVideo: HTMLVideoElement) {
	const start = parseFloat(segmentEl.getAttribute('data-start') || '0');
	modalVideo.currentTime = start;
	modalVideo.play().catch(() => {
		// Suppress play() rejections (e.g. NotAllowedError before user gesture)
	});
}

function updateModalSpeedUI(els: ModalElements, speed: number) {
	els.modalSpeedBtn.textContent = formatSpeed(speed);
	els.modalSpeedOptions.querySelectorAll('.speed-option').forEach(option => {
		const optionSpeed = parseFloat((option as HTMLElement).dataset.speed || '1');
		if (optionSpeed === speed) {
			option.classList.add('active');
		} else {
			option.classList.remove('active');
		}
	});
}

function showModalSpeedIndicator(els: ModalElements, state: ModalState, speed: number) {
	els.modalSpeedIndicator.textContent = formatSpeed(speed);
	els.modalSpeedIndicator.classList.add('visible');

	if (state.speedIndicatorTimeout) {
		clearTimeout(state.speedIndicatorTimeout);
	}

	state.speedIndicatorTimeout = setTimeout(() => {
		els.modalSpeedIndicator.classList.remove('visible');
		state.speedIndicatorTimeout = null;
	}, 800);
}

function toggleModalSpeedMenu(els: ModalElements, state: ModalState) {
	state.speedMenuOpen = !state.speedMenuOpen;
	els.modalSpeedOptions.classList.toggle('visible', state.speedMenuOpen);
	els.modalSpeedBtn.setAttribute('aria-expanded', state.speedMenuOpen.toString());
}

function closeModalSpeedMenu(els: ModalElements, state: ModalState) {
	state.speedMenuOpen = false;
	els.modalSpeedOptions.classList.remove('visible');
	els.modalSpeedBtn.setAttribute('aria-expanded', 'false');
}

function initializeModalSpeed(els: ModalElements) {
	const savedSpeed = getSavedSpeed();
	els.modalVideo.playbackRate = savedSpeed;
	updateModalSpeedUI(els, savedSpeed);

	// Re-apply speed after video loads, since browsers reset
	// playbackRate to 1.0 when a new source is loaded
	els.modalVideo.addEventListener('loadeddata', () => {
		const speed = getSavedSpeed();
		els.modalVideo.playbackRate = speed;
	}, { once: true });
}

function renderModalSegments(els: ModalElements, state: ModalState) {
	const useBionic = isBionicEnabled();
	els.modalSegments.innerHTML = state.subtitleSegments.map((seg, index) => {
		const displayText = useBionic ? renderBionicText(seg.text, { enabled: true }) : escapeHtml(seg.text);
		return `
		<div class="modal-segment" data-index="${index}" data-start="${seg.start}" data-end="${seg.end}"
			tabindex="0" role="button" aria-label="Seek to ${formatTime(seg.start)}: ${escapeHtml(seg.text.substring(0, 50))}${seg.text.length > 50 ? '...' : ''}">
			<div class="modal-segment-time">${formatTime(seg.start)} - ${formatTime(seg.end)}</div>
			<div class="modal-segment-text">${displayText}</div>
		</div>
	`}).join('');

	state.cachedSegmentElements = Array.from(els.modalSegments.querySelectorAll('.modal-segment')) as HTMLElement[];
	state.lastActiveSegmentIndex = -1;
}

function createUpdateModalSubtitle(els: ModalElements, state: ModalState): () => void {
	return function updateModalSubtitle() {
		const currentTime = els.modalVideo.currentTime;

		let currentSeg: TranscriptionSegment | null = null;
		let currentIndex = -1;

		for (let i = 0; i < state.subtitleSegments.length; i++) {
			const seg = state.subtitleSegments[i];
			if (currentTime >= seg.start && currentTime <= seg.end) {
				currentSeg = seg;
				currentIndex = i;
				break;
			}
		}

		if (currentSeg) {
			const useBionic = isBionicEnabled();
			if (useBionic) {
				els.modalCurrentSubtitle.innerHTML = renderBionicText(currentSeg.text.trim(), { enabled: true });
			} else {
				els.modalCurrentSubtitle.textContent = currentSeg.text.trim();
			}
		} else {
			els.modalCurrentSubtitle.textContent = '';
		}

		if (currentIndex !== state.lastActiveSegmentIndex) {
			if (state.lastActiveSegmentIndex >= 0 && state.lastActiveSegmentIndex < state.cachedSegmentElements.length) {
				state.cachedSegmentElements[state.lastActiveSegmentIndex].classList.remove('active');
			}
			if (currentIndex >= 0 && currentIndex < state.cachedSegmentElements.length) {
				const segmentEl = state.cachedSegmentElements[currentIndex];
				segmentEl.classList.add('active');

				const containerHeight = els.modalSegments.clientHeight;
				const segmentTop = segmentEl.offsetTop;
				const segmentHeight = segmentEl.offsetHeight;

				const targetScrollTop = segmentTop - (containerHeight / 2) + (segmentHeight / 2);
				els.modalSegments.scrollTop = Math.max(0, targetScrollTop);
			}
			state.lastActiveSegmentIndex = currentIndex;
		}
	};
}

function getModalContentEl(els: ModalElements): HTMLElement | null {
	return els.videoModal.querySelector('.modal-content');
}

export function enableKidMode(els: ModalElements, state: ModalState): void {
	state.kidModeLocked = true;
	const content = getModalContentEl(els);
	if (content) {
		content.classList.add('kid-mode-locked');
	}
	els.modalKidModeBtn.setAttribute('aria-pressed', 'true');
	els.modalKidModeBtn.setAttribute('aria-label', 'Unlock screen (hold for 1 second)');
}

export function disableKidMode(els: ModalElements, state: ModalState): void {
	state.kidModeLocked = false;
	if (state.kidModeUnlockTimeout) {
		clearTimeout(state.kidModeUnlockTimeout);
		state.kidModeUnlockTimeout = null;
	}
	const content = getModalContentEl(els);
	if (content) {
		content.classList.remove('kid-mode-locked');
	}
	els.modalKidModeBtn.classList.remove('kid-mode-unlocking');
	els.modalKidModeBtn.setAttribute('aria-pressed', 'false');
	els.modalKidModeBtn.setAttribute('aria-label', 'Lock screen for kid mode');
}

function startKidModeUnlock(els: ModalElements, state: ModalState): void {
	if (!state.kidModeLocked) return;
	els.modalKidModeBtn.classList.add('kid-mode-unlocking');
	state.kidModeUnlockTimeout = setTimeout(() => {
		disableKidMode(els, state);
	}, 1000);
}

function cancelKidModeUnlock(els: ModalElements, state: ModalState): void {
	if (state.kidModeUnlockTimeout) {
		clearTimeout(state.kidModeUnlockTimeout);
		state.kidModeUnlockTimeout = null;
	}
	els.modalKidModeBtn.classList.remove('kid-mode-unlocking');
}

function createHandleModalKeydown(
	els: ModalElements,
	state: ModalState,
	closeFn: () => void
): (e: KeyboardEvent) => void {
	return function handleModalKeydown(e: KeyboardEvent) {
		// Block all keyboard shortcuts (except Tab for a11y) when kid mode is locked
		if (state.kidModeLocked && e.key !== 'Tab') {
			return;
		}

		if (e.key === 'Escape') {
			if (state.speedMenuOpen) {
				closeModalSpeedMenu(els, state);
				return;
			}
			closeFn();
			return;
		}

		// Speed control shortcuts
		if (e.key === '[') {
			e.preventDefault();
			const newSpeed = decreaseSpeed(els.modalVideo.playbackRate);
			setPlaybackSpeed(els.modalVideo, newSpeed);
			updateModalSpeedUI(els, newSpeed);
			showModalSpeedIndicator(els, state, newSpeed);
			return;
		}
		if (e.key === ']') {
			e.preventDefault();
			const newSpeed = increaseSpeed(els.modalVideo.playbackRate);
			setPlaybackSpeed(els.modalVideo, newSpeed);
			updateModalSpeedUI(els, newSpeed);
			showModalSpeedIndicator(els, state, newSpeed);
			return;
		}

		// Trap focus inside modal when Tab is pressed
		if (e.key === 'Tab') {
			const focusableElements = els.videoModal.querySelectorAll<HTMLElement>(
				'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
			);
			const focusableArray = Array.from(focusableElements).filter(
				el => el.offsetParent !== null
			);

			if (focusableArray.length === 0) {
				e.preventDefault();
				return;
			}

			const firstFocusable = focusableArray[0];
			const lastFocusable = focusableArray[focusableArray.length - 1];
			const activeElement = document.activeElement as HTMLElement;

			if (e.shiftKey) {
				if (activeElement === firstFocusable || !focusableArray.includes(activeElement)) {
					e.preventDefault();
					lastFocusable.focus();
				}
			} else {
				if (activeElement === lastFocusable || !focusableArray.includes(activeElement)) {
					e.preventDefault();
					firstFocusable.focus();
				}
			}
		}
	};
}

/**
 * Set up all modal event listeners. Called once on page load.
 * Uses event delegation where possible to prevent memory leaks.
 */
export function setupModalListeners(els: ModalElements, state: ModalState): {
	openVideoModal: (videoId: string, filename: string) => Promise<void>;
	closeVideoModal: () => void;
	setTriggerElement: (el: HTMLElement) => void;
} {
	// Keep a reference to the update function for cleanup
	let updateSubtitleFn: (() => void) | null = null;

	function closeVideoModal() {
		els.videoModal.classList.remove('visible');
		document.body.style.overflow = '';

		els.modalVideo.pause();
		els.modalVideo.src = '';

		if (state.subtitleSyncActive && updateSubtitleFn) {
			els.modalVideo.removeEventListener('timeupdate', updateSubtitleFn);
			state.subtitleSyncActive = false;
		}

		if (state.keydownController) {
			state.keydownController.abort();
			state.keydownController = null;
		}

		// Reset kid mode on close
		disableKidMode(els, state);

		state.currentVideoId = null;
		(window as any).currentVideoId = null;
		state.subtitleSegments = [];
		state.cachedSegmentElements = [];

		if (state.triggerElement && document.contains(state.triggerElement)) {
			state.triggerElement.focus();
		}
		state.triggerElement = null;
	}

	async function openVideoModal(videoId: string, filename: string) {
		state.currentVideoId = videoId;
		(window as any).currentVideoId = videoId;
		state.subtitleSegments = [];

		// Reset modal state
		els.modalTitle.textContent = filename;
		els.modalLoading.style.display = 'block';
		els.modalError.style.display = 'none';
		els.modalVideoContainer.style.display = 'none';
		els.modalCurrentSubtitle.textContent = '';
		els.modalSegments.innerHTML = '';

		// Show modal
		els.videoModal.classList.add('visible');
		document.body.style.overflow = 'hidden';

		// Set up keyboard handlers
		if (state.keydownController) {
			state.keydownController.abort();
		}
		state.keydownController = new AbortController();
		const handleKeydown = createHandleModalKeydown(els, state, closeVideoModal);
		document.addEventListener('keydown', handleKeydown, { signal: state.keydownController.signal });

		// Move focus to close button
		els.modalClose.focus();

		els.modalEditLink.href = `/upload?id=${videoId}`;

		try {
			els.modalVideo.src = `/api/videos/${videoId}/video`;

			const response = await fetchWithTimeout(`/api/transcribe/${videoId}`);
			let rawData;
			try {
				rawData = await response.json();
			} catch (e) {
				if (e instanceof SyntaxError) {
					throw new Error('Server returned invalid data');
				}
				throw e;
			}

			if (!response.ok) {
				throw new Error(rawData.error || 'Failed to load transcription');
			}

			const data = safeParse(TranscriptionStatusResponseSchema, rawData);
			if (!data) {
				throw new Error('Invalid transcription response format');
			}

			if (data.status !== 'complete' || !data.result) {
				throw new Error('Transcription not available');
			}

			state.subtitleSegments = data.result.segments as TranscriptionSegment[];

			renderModalSegments(els, state);

			// Set up subtitle sync
			updateSubtitleFn = createUpdateModalSubtitle(els, state);
			if (state.subtitleSyncActive && updateSubtitleFn) {
				els.modalVideo.removeEventListener('timeupdate', updateSubtitleFn);
			}
			els.modalVideo.addEventListener('timeupdate', updateSubtitleFn);
			state.subtitleSyncActive = true;

			initializeModalSpeed(els);

			els.modalLoading.style.display = 'none';
			els.modalVideoContainer.style.display = 'block';

		} catch (err) {
			els.modalLoading.style.display = 'none';
			els.modalError.textContent = `Error: ${err instanceof Error ? err.message : 'Unknown error'}`;
			els.modalError.style.display = 'block';
		}
	}

	// Event delegation for modal segments
	els.modalSegments.addEventListener('click', (e) => {
		const target = e.target as HTMLElement;
		const segmentEl = target.closest('.modal-segment') as HTMLElement | null;
		if (segmentEl) {
			activateSegment(segmentEl, els.modalVideo);
		}
	});

	els.modalSegments.addEventListener('keydown', (e) => {
		if (e.key !== 'Enter' && e.key !== ' ') {
			return;
		}
		const target = e.target as HTMLElement;
		const segmentEl = target.closest('.modal-segment') as HTMLElement | null;
		if (segmentEl) {
			e.preventDefault();
			activateSegment(segmentEl, els.modalVideo);
		}
	});

	// Speed button controls
	els.modalSpeedBtn.addEventListener('click', (e) => {
		e.stopPropagation();
		toggleModalSpeedMenu(els, state);
	});

	els.modalSpeedOptions.addEventListener('click', (e) => {
		const target = e.target as HTMLElement;
		if (target.classList.contains('speed-option')) {
			const speed = parseFloat(target.dataset.speed || '1');
			if (PLAYBACK_SPEEDS.includes(speed as PlaybackSpeed)) {
				setPlaybackSpeed(els.modalVideo, speed as PlaybackSpeed);
				updateModalSpeedUI(els, speed);
			}
			closeModalSpeedMenu(els, state);
		}
	});

	els.videoModal.addEventListener('click', (e) => {
		if (state.speedMenuOpen && !els.modalSpeedDropdown.contains(e.target as Node)) {
			closeModalSpeedMenu(els, state);
		}
		// Close on clicking overlay background (blocked in kid mode)
		if (e.target === els.videoModal && !state.kidModeLocked) {
			closeVideoModal();
		}
	});

	// Kid mode lock button
	els.modalKidModeBtn.addEventListener('click', () => {
		if (!state.kidModeLocked) {
			enableKidMode(els, state);
		}
		// When locked, single click does nothing (must long-press to unlock)
	});

	// Long-press to unlock kid mode
	els.modalKidModeBtn.addEventListener('pointerdown', (e) => {
		if (state.kidModeLocked) {
			e.preventDefault();
			startKidModeUnlock(els, state);
		}
	});

	const cancelUnlock = () => cancelKidModeUnlock(els, state);
	els.modalKidModeBtn.addEventListener('pointerup', cancelUnlock);
	els.modalKidModeBtn.addEventListener('pointerleave', cancelUnlock);
	els.modalKidModeBtn.addEventListener('pointercancel', cancelUnlock);

	// Modal subtitle view buttons
	els.modalDownloadSrt.addEventListener('click', async (e) => {
		e.preventDefault();
		if (state.subtitleSegments.length === 0) {
			await showAlert('No subtitles available to view', 'info');
			return;
		}
		openSRT(state.subtitleSegments);
	});

	els.modalDownloadVtt.addEventListener('click', async (e) => {
		e.preventDefault();
		if (state.subtitleSegments.length === 0) {
			await showAlert('No subtitles available to view', 'info');
			return;
		}
		openVTT(state.subtitleSegments);
	});

	els.modalDownloadJson.addEventListener('click', async (e) => {
		e.preventDefault();
		if (state.subtitleSegments.length === 0) {
			await showAlert('No subtitles available to view', 'info');
			return;
		}
		openJSON(state.subtitleSegments);
	});

	// Modal close handler
	els.modalClose.addEventListener('click', closeVideoModal);

	return {
		openVideoModal,
		closeVideoModal,
		setTriggerElement: (el: HTMLElement) => { state.triggerElement = el; },
	};
}

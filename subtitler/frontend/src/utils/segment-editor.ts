/**
 * Segment editing logic for the upload page.
 * Handles undo/redo, edit mode toggling, segment rendering, and save.
 */

import { csrfFetch } from './csrf';
import { parseTimeString } from './validation';
import { escapeHtml } from './html';
import { renderBionicText, isBionicEnabled } from './bionic';
import { getOptionalElement } from './dom';
import { formatTime } from './transcription-polling';
import { MAX_HISTORY_SIZE } from './upload-constants';
import { detectGaps } from './gap-detection';
import { createGapSegment, insertGapSegment, transcribeGap } from './gap-fill';
import type { TranscriptionSegment } from '../types/transcription';

/** Feedback type for segment alignment quality */
export type FeedbackType = 'good' | 'early' | 'late' | 'missing' | null;

/** Mutable state for segment editing */
export interface SegmentEditorState {
	transcriptionSegments: TranscriptionSegment[];
	editedSegments: TranscriptionSegment[];
	isEditMode: boolean;
	hasUnsavedChanges: boolean;
	cachedSegmentElements: HTMLElement[];
	lastActiveSegmentIndex: number;
	undoStack: TranscriptionSegment[][];
	redoStack: TranscriptionSegment[][];
	feedbackActiveIndex: number;
	currentUploadId: string | null;
}

export function createSegmentEditorState(): SegmentEditorState {
	return {
		transcriptionSegments: [],
		editedSegments: [],
		isEditMode: false,
		hasUnsavedChanges: false,
		cachedSegmentElements: [],
		lastActiveSegmentIndex: -1,
		undoStack: [],
		redoStack: [],
		feedbackActiveIndex: -1,
		currentUploadId: null
	};
}

/** DOM elements needed by the segment editor */
export interface SegmentEditorElements {
	segments: HTMLElement;
	fullText: HTMLElement;
	editBtn: HTMLButtonElement;
	undoBtn: HTMLButtonElement;
	redoBtn: HTMLButtonElement;
	saveBtn: HTMLButtonElement;
	editActions: HTMLElement;
	addSegmentBtn: HTMLButtonElement;
	unsavedIndicator: HTMLElement;
	subtitleFeedback: HTMLElement;
	previewVideo: HTMLVideoElement;
}

/** Callbacks for segment editor to interact with the page */
export interface SegmentEditorCallbacks {
	showStatus: (message: string, type?: 'error' | 'success') => void;
}

// --- Feedback functions ---

function getFeedbackStorageKey(uploadId: string): string {
	return `subtitler:feedback:${uploadId}`;
}

export function getSegmentFeedback(uploadId: string | null, index: number): FeedbackType {
	if (!uploadId) return null;
	try {
		const stored = localStorage.getItem(getFeedbackStorageKey(uploadId));
		if (stored) {
			const feedbackMap = JSON.parse(stored) as Record<string, FeedbackType>;
			return feedbackMap[index.toString()] || null;
		}
	} catch (error) {
		console.error('Failed to parse segment feedback:', error);
	}
	return null;
}

export function setSegmentFeedback(uploadId: string | null, index: number, feedback: FeedbackType): void {
	if (!uploadId) return;
	try {
		const key = getFeedbackStorageKey(uploadId);
		const stored = localStorage.getItem(key);
		const feedbackMap: Record<string, FeedbackType> = stored ? JSON.parse(stored) : {};

		if (feedback === null) {
			delete feedbackMap[index.toString()];
		} else {
			feedbackMap[index.toString()] = feedback;
		}

		localStorage.setItem(key, JSON.stringify(feedbackMap));
	} catch (error) {
		console.error('Failed to save segment feedback:', error);
	}
}

function updateFeedbackDot(subtitleFeedback: HTMLElement, feedback: FeedbackType): void {
	const dot = subtitleFeedback.querySelector('.feedback-dot') as HTMLElement | null;
	if (!dot) return;
	dot.className = 'feedback-dot';
	if (feedback) dot.classList.add(feedback);
}

export function updateFeedbackButtons(
	state: SegmentEditorState,
	subtitleFeedback: HTMLElement,
	index: number
): void {
	if (index < 0) {
		subtitleFeedback.style.display = 'none';
		state.feedbackActiveIndex = -1;
		return;
	}
	subtitleFeedback.style.display = '';
	state.feedbackActiveIndex = index;
	const feedback = getSegmentFeedback(state.currentUploadId, index);
	subtitleFeedback.querySelectorAll('.feedback-btn').forEach(btn => {
		const btnType = btn.getAttribute('data-feedback');
		btn.classList.toggle('active', btnType === feedback);
	});
	updateFeedbackDot(subtitleFeedback, feedback);

	// Collapse options on segment change
	const options = subtitleFeedback.querySelector('.feedback-options') as HTMLElement | null;
	if (options) options.style.display = 'none';
}

/** Set up persistent click handler for subtitle feedback buttons */
export function setupFeedbackHandler(
	state: SegmentEditorState,
	subtitleFeedback: HTMLElement
): void {
	// Toggle button shows/hides feedback options
	const toggleBtn = subtitleFeedback.querySelector('.feedback-toggle-btn');
	const optionsEl = subtitleFeedback.querySelector('.feedback-options') as HTMLElement | null;

	if (toggleBtn && optionsEl) {
		toggleBtn.addEventListener('click', () => {
			optionsEl.style.display = optionsEl.style.display === 'none' ? 'flex' : 'none';
		});
	}

	// Feedback button clicks
	subtitleFeedback.addEventListener('click', (e) => {
		const target = (e.target as HTMLElement).closest('.feedback-btn') as HTMLButtonElement | null;
		if (!target || state.feedbackActiveIndex < 0) return;
		const feedbackType = target.getAttribute('data-feedback') as FeedbackType;
		const currentFeedback = getSegmentFeedback(state.currentUploadId, state.feedbackActiveIndex);

		if (currentFeedback === feedbackType) {
			setSegmentFeedback(state.currentUploadId, state.feedbackActiveIndex, null);
		} else {
			setSegmentFeedback(state.currentUploadId, state.feedbackActiveIndex, feedbackType);
		}

		const newFeedback = getSegmentFeedback(state.currentUploadId, state.feedbackActiveIndex);
		subtitleFeedback.querySelectorAll('.feedback-btn').forEach(btn => {
			const btnType = btn.getAttribute('data-feedback');
			btn.classList.toggle('active', btnType === newFeedback);
		});

		// Update dot and collapse options after selection
		updateFeedbackDot(subtitleFeedback, newFeedback);
		if (optionsEl) optionsEl.style.display = 'none';
	});
}

// --- Undo/Redo ---

export function updateUndoRedoButtons(state: SegmentEditorState, els: SegmentEditorElements): void {
	els.undoBtn.disabled = state.undoStack.length === 0;
	els.redoBtn.disabled = state.redoStack.length === 0;
}

export function pushToHistory(state: SegmentEditorState, els: SegmentEditorElements): void {
	const snapshot = structuredClone(state.editedSegments);
	state.undoStack.push(snapshot);

	if (state.undoStack.length > MAX_HISTORY_SIZE) {
		state.undoStack.shift();
	}

	state.redoStack = [];
	updateUndoRedoButtons(state, els);
}

function markUnsaved(state: SegmentEditorState, els: SegmentEditorElements): void {
	state.hasUnsavedChanges = true;
	els.unsavedIndicator.classList.add('visible');
}

function clearUnsaved(state: SegmentEditorState, els: SegmentEditorElements): void {
	state.hasUnsavedChanges = false;
	els.unsavedIndicator.classList.remove('visible');
}

export function performUndo(state: SegmentEditorState, els: SegmentEditorElements): void {
	if (state.undoStack.length === 0) return;

	state.redoStack.push(structuredClone(state.editedSegments));
	state.editedSegments = state.undoStack.pop()!;
	state.editedSegments.forEach((seg, i) => seg.id = i);

	updateUndoRedoButtons(state, els);
	renderSegments(state, els);

	if (JSON.stringify(state.editedSegments) !== JSON.stringify(state.transcriptionSegments)) {
		markUnsaved(state, els);
	} else {
		clearUnsaved(state, els);
	}
}

export function performRedo(state: SegmentEditorState, els: SegmentEditorElements): void {
	if (state.redoStack.length === 0) return;

	state.undoStack.push(structuredClone(state.editedSegments));
	state.editedSegments = state.redoStack.pop()!;
	state.editedSegments.forEach((seg, i) => seg.id = i);

	updateUndoRedoButtons(state, els);
	renderSegments(state, els);
	markUnsaved(state, els);
}

function clearHistory(state: SegmentEditorState, els: SegmentEditorElements): void {
	state.undoStack = [];
	state.redoStack = [];
	updateUndoRedoButtons(state, els);
}

// --- Timing adjustment ---

/** Minimum gap between start and end times in seconds */
const MIN_TIME_GAP = 0.1;

/** Round to 3 decimal places to avoid floating-point drift */
function roundTime(t: number): number {
	return Math.round(t * 1000) / 1000;
}

export function adjustSegmentTime(
	state: SegmentEditorState,
	els: SegmentEditorElements,
	index: number,
	type: 'start' | 'end',
	direction: 'earlier' | 'later',
	shiftKey: boolean
): void {
	const step = shiftKey ? 0.5 : 0.1;
	const delta = direction === 'earlier' ? -step : step;
	const segment = state.editedSegments[index];

	pushToHistory(state, els);

	if (type === 'start') {
		segment.start = roundTime(Math.max(0, segment.start + delta));
		if (segment.start >= segment.end - MIN_TIME_GAP) {
			segment.start = roundTime(segment.end - MIN_TIME_GAP);
		}
	} else {
		segment.end = roundTime(segment.end + delta);
		if (segment.end <= segment.start + MIN_TIME_GAP) {
			segment.end = roundTime(segment.start + MIN_TIME_GAP);
		}
	}

	markUnsaved(state, els);
	renderSegments(state, els);
}

// --- Segment rendering ---

function formatTimeForInput(seconds: number): string {
	const hrs = Math.floor(seconds / 3600);
	const mins = Math.floor((seconds % 3600) / 60);
	const secs = Math.floor(seconds % 60);
	const ms = Math.floor((seconds % 1) * 1000);
	return `${hrs.toString().padStart(2, '0')}:${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}.${ms.toString().padStart(3, '0')}`;
}

function renderSegmentHtml(seg: TranscriptionSegment, index: number, isEditMode: boolean, useBionic: boolean): string {
	const displayText = useBionic ? renderBionicText(seg.text, { enabled: true }) : escapeHtml(seg.text);
	return `
	<div class="segment${isEditMode ? ' editing' : ''}" data-index="${index}" data-start="${seg.start}" data-end="${seg.end}">
		<div class="segment-time">${formatTime(seg.start)} - ${formatTime(seg.end)}</div>
		<div class="segment-text">${displayText}</div>
		<div class="segment-edit">
			<div class="time-inputs">
				<div class="time-adjust-group">
					<button class="time-adjust-btn" data-direction="earlier" data-type="start" data-index="${index}" aria-label="Move start earlier">&#9664;</button>
					<button class="time-adjust-btn" data-direction="later" data-type="start" data-index="${index}" aria-label="Move start later">&#9654;</button>
					<label>Start:</label>
					<input type="text" class="time-input-start" value="${formatTimeForInput(seg.start)}" data-index="${index}" aria-describedby="time-error-start-${index}" />
				</div>
				<div class="time-adjust-group">
					<label>End:</label>
					<input type="text" class="time-input-end" value="${formatTimeForInput(seg.end)}" data-index="${index}" aria-describedby="time-error-end-${index}" />
					<button class="time-adjust-btn" data-direction="earlier" data-type="end" data-index="${index}" aria-label="Move end earlier">&#9664;</button>
					<button class="time-adjust-btn" data-direction="later" data-type="end" data-index="${index}" aria-label="Move end later">&#9654;</button>
				</div>
			</div>
			<div class="time-error" id="time-error-start-${index}" data-type="start" data-index="${index}" role="alert"></div>
			<div class="time-error" id="time-error-end-${index}" data-type="end" data-index="${index}" role="alert"></div>
			<textarea class="segment-textarea" data-index="${index}">${escapeHtml(seg.text)}</textarea>
			<div class="segment-actions">
				<button class="segment-delete-btn" data-index="${index}" aria-label="Delete segment ${index + 1}">Delete</button>
			</div>
		</div>
	</div>`;
}

function renderGapButton(start: number, end: number, afterIndex: number): string {
	const duration = (end - start).toFixed(1);
	return `<button class="gap-fill-btn" data-start="${start}" data-end="${end}" data-after="${afterIndex}" aria-label="Fill gap from ${formatTime(start)} to ${formatTime(end)}">+ Missing? (${duration}s gap)</button>`;
}

export function renderSegments(state: SegmentEditorState, els: SegmentEditorElements): void {
	const segsToRender = state.isEditMode ? state.editedSegments : state.transcriptionSegments;
	const useBionic = !state.isEditMode && isBionicEnabled();

	if (segsToRender.length > 0) {
		let html: string;
		if (state.isEditMode) {
			// Detect gaps and intersperse gap buttons
			const videoDuration = els.previewVideo.duration || 0;
			const gaps = detectGaps(segsToRender, videoDuration);
			const gapsByAfterIndex = new Map(gaps.map(g => [g.afterIndex, g]));
			const parts: string[] = [];

			// Gap before first segment
			const beforeGap = gapsByAfterIndex.get(-1);
			if (beforeGap) {
				parts.push(renderGapButton(beforeGap.start, beforeGap.end, beforeGap.afterIndex));
			}

			for (let i = 0; i < segsToRender.length; i++) {
				parts.push(renderSegmentHtml(segsToRender[i], i, true, false));
				const afterGap = gapsByAfterIndex.get(i);
				if (afterGap) {
					parts.push(renderGapButton(afterGap.start, afterGap.end, afterGap.afterIndex));
				}
			}
			html = parts.join('');
		} else {
			html = segsToRender.map((seg, index) =>
				renderSegmentHtml(seg, index, false, useBionic)
			).join('');
		}

		els.segments.innerHTML = html;

		// Cache segment elements for timeupdate performance
		state.cachedSegmentElements = Array.from(els.segments.querySelectorAll('.segment')) as HTMLElement[];
		state.lastActiveSegmentIndex = -1;

		if (state.isEditMode) {
			setupEditHandlers(state, els);
		} else {
			setupClickHandlers(els);
		}
	}
}

function setupClickHandlers(els: SegmentEditorElements): void {
	els.segments.querySelectorAll('.segment').forEach(el => {
		el.addEventListener('click', () => {
			const start = parseFloat(el.getAttribute('data-start') || '0');
			els.previewVideo.currentTime = start;
			els.previewVideo.play().catch(() => {});
		});
	});
}

function setupEditHandlers(state: SegmentEditorState, els: SegmentEditorElements): void {
	// Text editing
	els.segments.querySelectorAll('.segment-textarea').forEach((textarea: Element) => {
		textarea.addEventListener('focus', () => {
			pushToHistory(state, els);
		});
		textarea.addEventListener('input', (e) => {
			const target = e.target as HTMLTextAreaElement;
			const index = parseInt(target.getAttribute('data-index') || '0', 10);
			state.editedSegments[index].text = target.value;
			markUnsaved(state, els);
		});
	});

	// Time inputs - start
	els.segments.querySelectorAll('.time-input-start').forEach((input: Element) => {
		input.addEventListener('focus', () => {
			pushToHistory(state, els);
		});
		input.addEventListener('change', (e) => {
			const target = e.target as HTMLInputElement;
			const index = parseInt(target.getAttribute('data-index') || '0', 10);
			const errorEl = getOptionalElement(`time-error-start-${index}`, HTMLElement);
			const parsed = parseTimeString(target.value);

			if ('error' in parsed) {
				target.classList.add('error');
				if (errorEl) {
					errorEl.textContent = parsed.error;
					errorEl.classList.add('visible');
				}
			} else {
				target.classList.remove('error');
				if (errorEl) {
					errorEl.textContent = '';
					errorEl.classList.remove('visible');
				}
				state.editedSegments[index].start = parsed.seconds;
				markUnsaved(state, els);
			}
		});
	});

	// Time inputs - end
	els.segments.querySelectorAll('.time-input-end').forEach((input: Element) => {
		input.addEventListener('focus', () => {
			pushToHistory(state, els);
		});
		input.addEventListener('change', (e) => {
			const target = e.target as HTMLInputElement;
			const index = parseInt(target.getAttribute('data-index') || '0', 10);
			const errorEl = getOptionalElement(`time-error-end-${index}`, HTMLElement);
			const parsed = parseTimeString(target.value);

			if ('error' in parsed) {
				target.classList.add('error');
				if (errorEl) {
					errorEl.textContent = parsed.error;
					errorEl.classList.add('visible');
				}
			} else {
				target.classList.remove('error');
				if (errorEl) {
					errorEl.textContent = '';
					errorEl.classList.remove('visible');
				}
				state.editedSegments[index].end = parsed.seconds;
				markUnsaved(state, els);
			}
		});
	});

	// Timing adjustment buttons
	els.segments.querySelectorAll('.time-adjust-btn').forEach((btn: Element) => {
		btn.addEventListener('click', (e) => {
			e.stopPropagation();
			const target = btn as HTMLButtonElement;
			const index = parseInt(target.getAttribute('data-index') || '0', 10);
			const type = target.getAttribute('data-type') as 'start' | 'end';
			const direction = target.getAttribute('data-direction') as 'earlier' | 'later';
			const shiftKey = (e as MouseEvent).shiftKey;
			adjustSegmentTime(state, els, index, type, direction, shiftKey);
		});
	});

	// Delete buttons
	els.segments.querySelectorAll('.segment-delete-btn').forEach((btn: Element) => {
		btn.addEventListener('click', (e) => {
			e.stopPropagation();
			pushToHistory(state, els);
			const index = parseInt(btn.getAttribute('data-index') || '0', 10);
			state.editedSegments.splice(index, 1);
			state.editedSegments.forEach((seg, i) => seg.id = i);
			markUnsaved(state, els);
			renderSegments(state, els);
		});
	});

	// Gap fill buttons
	els.segments.querySelectorAll('.gap-fill-btn').forEach((btn: Element) => {
		btn.addEventListener('click', (e) => {
			e.stopPropagation();
			const target = btn as HTMLButtonElement;
			const start = parseFloat(target.getAttribute('data-start') || '0');
			const end = parseFloat(target.getAttribute('data-end') || '0');
			const afterIndex = parseInt(target.getAttribute('data-after') || '-1', 10);

			pushToHistory(state, els);
			const gap = createGapSegment(start, end);
			state.editedSegments = insertGapSegment(state.editedSegments, gap, afterIndex);
			markUnsaved(state, els);
			renderSegments(state, els);

			// Kick off async transcription for the gap
			if (state.currentUploadId) {
				const insertedIndex = afterIndex + 1;
				const segEl = els.segments.querySelector(`.segment[data-index="${insertedIndex}"]`);
				if (segEl) segEl.classList.add('transcribing');

				transcribeGap(state.currentUploadId, start, end).then(result => {
					// Update the segment text if still in edit mode
					if (state.isEditMode && state.editedSegments[insertedIndex]) {
						const text = result.text || result.segments.map(s => s.text).join(' ');
						if (text) {
							state.editedSegments[insertedIndex].text = text.trim();
							markUnsaved(state, els);
							renderSegments(state, els);
						}
					}
				}).catch(err => {
					console.error('Gap transcription failed:', err);
					// Remove transcribing indicator
					const el = els.segments.querySelector(`.segment[data-index="${insertedIndex}"]`);
					if (el) el.classList.remove('transcribing');
				});
			}
		});
	});
}

// --- Edit mode ---

export function enterEditMode(state: SegmentEditorState, els: SegmentEditorElements): void {
	state.isEditMode = true;
	els.editBtn.classList.add('active');
	els.editBtn.textContent = 'Editing...';
	els.editActions.classList.add('visible');
	els.addSegmentBtn.classList.add('visible');
	els.fullText.style.display = 'none';
	els.subtitleFeedback.style.display = 'none';
	state.editedSegments = structuredClone(state.transcriptionSegments);
	clearHistory(state, els);
	renderSegments(state, els);
}

export function exitEditMode(state: SegmentEditorState, els: SegmentEditorElements): void {
	state.isEditMode = false;
	els.editBtn.classList.remove('active');
	els.editBtn.textContent = 'Edit Subtitles';
	els.editActions.classList.remove('visible');
	els.addSegmentBtn.classList.remove('visible');
	els.fullText.style.display = 'block';
	clearUnsaved(state, els);
	renderSegments(state, els);
}

export async function saveSegments(
	state: SegmentEditorState,
	els: SegmentEditorElements,
	callbacks: SegmentEditorCallbacks
): Promise<void> {
	if (!state.currentUploadId) return;

	els.saveBtn.disabled = true;
	els.saveBtn.textContent = 'Saving...';

	try {
		const response = await csrfFetch(`/api/transcribe/${state.currentUploadId}/segments`, {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ segments: state.editedSegments })
		});

		const result = await response.json();

		if (!response.ok) {
			throw new Error(result.error || 'Failed to save');
		}

		state.transcriptionSegments = structuredClone(state.editedSegments);
		els.fullText.textContent = state.editedSegments.map((s) => s.text).join(' ');
		clearUnsaved(state, els);
		callbacks.showStatus('Subtitles saved successfully!', 'success');

		exitEditMode(state, els);
	} catch (error) {
		callbacks.showStatus(`Failed to save: ${error instanceof Error ? error.message : error}`, 'error');
	} finally {
		els.saveBtn.disabled = false;
		els.saveBtn.textContent = 'Save Changes';
	}
}

export function addNewSegment(state: SegmentEditorState, els: SegmentEditorElements): void {
	pushToHistory(state, els);
	const lastSeg = state.editedSegments[state.editedSegments.length - 1];
	const startTime = lastSeg ? lastSeg.end : 0;

	state.editedSegments.push({
		id: state.editedSegments.length,
		start: startTime,
		end: startTime + 3,
		text: ''
	});
	markUnsaved(state, els);
	renderSegments(state, els);

	const textareas = els.segments.querySelectorAll('.segment-textarea');
	const lastTextarea = textareas[textareas.length - 1] as HTMLTextAreaElement;
	if (lastTextarea) {
		lastTextarea.focus();
	}
}

// --- Navigation helpers ---

/** Check if focus is in a text input */
export function isTextInputFocused(): boolean {
	const active = document.activeElement;
	if (!active) return false;
	const tag = active.tagName.toLowerCase();
	return tag === 'input' || tag === 'textarea' || (active as HTMLElement).isContentEditable;
}

/** Get current segment index based on video time */
export function getCurrentSegmentIndex(
	segments: TranscriptionSegment[],
	currentTime: number
): number {
	for (let i = 0; i < segments.length; i++) {
		const seg = segments[i];
		if (currentTime >= seg.start && currentTime <= seg.end) {
			return i;
		}
	}
	for (let i = 0; i < segments.length; i++) {
		if (segments[i].start > currentTime) {
			return Math.max(0, i - 1);
		}
	}
	return segments.length - 1;
}

/**
 * Scroll element into view within container only (not the whole page).
 * IMPORTANT: Never use scrollIntoView() - it can scroll the entire page
 * and cause the video to go out of view. This was reported 3 times!
 */
export function scrollIntoContainerView(container: HTMLElement, element: HTMLElement): void {
	const containerRect = container.getBoundingClientRect();
	const elementRect = element.getBoundingClientRect();

	if (elementRect.top < containerRect.top) {
		container.scrollTop -= (containerRect.top - elementRect.top);
	} else if (elementRect.bottom > containerRect.bottom) {
		container.scrollTop += (elementRect.bottom - containerRect.bottom);
	}
}

/** Navigate to segment by index */
export function navigateToSegment(
	state: SegmentEditorState,
	els: SegmentEditorElements,
	previewVideo: HTMLVideoElement,
	index: number
): void {
	if (state.transcriptionSegments.length === 0) return;
	const clampedIndex = Math.max(0, Math.min(index, state.transcriptionSegments.length - 1));
	const seg = state.transcriptionSegments[clampedIndex];
	previewVideo.currentTime = seg.start;

	const segmentElements = els.segments.querySelectorAll('.segment');
	segmentElements.forEach((el, idx) => {
		if (idx === clampedIndex) {
			el.classList.add('active');
			scrollIntoContainerView(els.segments, el as HTMLElement);
		} else {
			el.classList.remove('active');
		}
	});
}

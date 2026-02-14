/**
 * Embedded subtitles handler for the upload page.
 * Displays notice when embedded subtitle tracks are detected and handles extraction.
 */

import { fetchWithTimeout } from './fetch-timeout';
import { parseSRT } from './transcription-polling';
import type { SubtitleTrack } from './transcription-polling';
import type { TranscriptionSegment } from '../types/transcription';
import type { SegmentEditorState } from './segment-editor';

export interface EmbeddedSubtitleElements {
	notice: HTMLElement;
	list: HTMLElement;
	trackSelect: HTMLSelectElement;
	useBtn: HTMLButtonElement;
	unsavedIndicator: HTMLElement;
	transcriptionStatus: HTMLElement;
}

export interface EmbeddedSubtitleCallbacks {
	renderSegments: () => void;
	setFullText: (text: string) => void;
}

/** Show or hide the embedded subtitles notice based on available tracks */
export function showEmbeddedSubtitlesNotice(
	tracks: SubtitleTrack[],
	els: EmbeddedSubtitleElements
): void {
	const textTracks = tracks.filter(t => t.text_based);
	if (textTracks.length === 0) {
		els.notice.style.display = 'none';
		return;
	}

	els.list.textContent = textTracks.map(t => {
		let label = t.language || 'Unknown';
		if (t.title) label += ` (${t.title})`;
		if (t.default) label += ' [default]';
		return label;
	}).join(', ');

	els.trackSelect.innerHTML = '';
	textTracks.forEach((track, i) => {
		const option = document.createElement('option');
		option.value = track.index.toString();
		let label = track.language || 'Unknown language';
		if (track.title) label = `${label} - ${track.title}`;
		label += ` (${track.codec})`;
		if (track.default) label += ' [default]';
		option.textContent = label;
		if (i === 0) option.selected = true;
		els.trackSelect.appendChild(option);
	});

	els.trackSelect.style.display = textTracks.length === 1 ? 'none' : 'block';
	els.notice.style.display = 'flex';
}

/** Set up the click handler for the "Use Embedded Subtitles" button */
export function setupEmbeddedSubtitlesHandler(
	editorState: SegmentEditorState,
	els: EmbeddedSubtitleElements,
	callbacks: EmbeddedSubtitleCallbacks
): void {
	els.useBtn.addEventListener('click', async () => {
		if (!editorState.currentUploadId) return;
		const trackIndex = parseInt(els.trackSelect.value, 10);
		els.useBtn.disabled = true;
		els.useBtn.textContent = 'Loading...';

		try {
			const response = await fetchWithTimeout(
				`/api/videos/${editorState.currentUploadId}/embedded-subtitles/${trackIndex}?format=srt`
			);
			if (!response.ok) {
				const err = await response.json();
				throw new Error(err.error || 'Failed to extract subtitles');
			}
			const srtContent = await response.text();
			const parsedSegments: TranscriptionSegment[] = parseSRT(srtContent);
			if (parsedSegments.length === 0) throw new Error('No subtitles found in embedded track');

			editorState.transcriptionSegments = parsedSegments;
			editorState.editedSegments = structuredClone(parsedSegments);
			editorState.hasUnsavedChanges = true;
			els.unsavedIndicator.classList.add('visible');

			callbacks.renderSegments();
			callbacks.setFullText(parsedSegments.map(s => s.text).join(' '));
			els.transcriptionStatus.textContent = 'Loaded embedded subtitles! (Unsaved)';
		} catch (error) {
			console.error('Failed to load embedded subtitles:', error);
			els.transcriptionStatus.textContent = `Error: ${error instanceof Error ? error.message : error}`;
		} finally {
			els.useBtn.disabled = false;
			els.useBtn.textContent = 'Use Embedded Subtitles';
		}
	});
}

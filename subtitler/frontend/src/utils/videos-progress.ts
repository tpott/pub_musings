/**
 * Real-time transcription progress polling for the My Videos page.
 * Polls transcription status for videos with "processing" or "pending" status
 * and updates a compact progress indicator in each video card.
 */

import { getOrCreateSessionId } from './session';
import { renderVideo, type Video } from './videos-list';

const POLL_INTERVAL_MS = 3000;

interface ProgressPoller {
	stop: () => void;
}

interface TranscriptionStatusResponse {
	status: string;
	message?: string;
	progress?: number;
}

/**
 * Start polling transcription progress for all processing/pending videos.
 * Updates progress indicators in video cards and refreshes card HTML when complete.
 */
export function startProgressPolling(
	videoList: HTMLElement,
	videos: Video[],
	isAuthenticated: boolean
): ProgressPoller {
	const activePolls = new Map<string, ReturnType<typeof setTimeout>>();
	let stopped = false;

	const processingVideos = videos.filter(
		v => v.transcription_status === 'processing' || v.transcription_status === 'pending'
	);

	for (const video of processingVideos) {
		pollVideo(video);
	}

	function pollVideo(video: Video): void {
		if (stopped) return;

		const poll = async () => {
			if (stopped) return;

			try {
				let url = `/api/transcribe/${video.id}`;
				if (!isAuthenticated) {
					const sessionId = getOrCreateSessionId();
					url += `?session_id=${encodeURIComponent(sessionId)}`;
				}

				const response = await fetch(url);
				if (!response.ok) {
					// Don't retry on auth/not-found errors
					if (response.status === 403 || response.status === 404) {
						activePolls.delete(video.id);
						return;
					}
					// Retry on transient errors
					if (!stopped) {
						activePolls.set(video.id, setTimeout(poll, POLL_INTERVAL_MS));
					}
					return;
				}

				const data: TranscriptionStatusResponse = await response.json();

				if (stopped) return;

				if (data.status === 'processing' || data.status === 'pending') {
					updateProgressIndicator(videoList, video.id, data);
					activePolls.set(video.id, setTimeout(poll, POLL_INTERVAL_MS));
				} else if (data.status === 'complete') {
					// Update the video object and re-render the card
					video.transcription_status = 'complete';
					replaceVideoCard(videoList, video);
					activePolls.delete(video.id);
				} else if (data.status === 'error') {
					video.transcription_status = 'error';
					replaceVideoCard(videoList, video);
					activePolls.delete(video.id);
				}
			} catch {
				// Network error — retry
				if (!stopped) {
					activePolls.set(video.id, setTimeout(poll, POLL_INTERVAL_MS));
				}
			}
		};

		poll();
	}

	return {
		stop() {
			stopped = true;
			for (const timeout of activePolls.values()) {
				clearTimeout(timeout);
			}
			activePolls.clear();
		}
	};
}

/**
 * Update the progress indicator inside a video card.
 */
function updateProgressIndicator(
	videoList: HTMLElement,
	videoId: string,
	data: TranscriptionStatusResponse
): void {
	const card = videoList.querySelector(`[data-video-id="${videoId}"].video-card`) as HTMLElement | null;
	if (!card) return;

	let indicator = card.querySelector('.video-progress') as HTMLElement | null;
	if (!indicator) {
		indicator = document.createElement('div');
		indicator.className = 'video-progress';
		// Insert before the video-actions div
		const actions = card.querySelector('.video-actions');
		if (actions) {
			card.insertBefore(indicator, actions);
		} else {
			card.appendChild(indicator);
		}
	}

	const progress = data.progress ?? 0;
	const message = data.status === 'pending' ? 'Waiting...' : (data.message || 'Processing...');
	const progressText = data.status === 'processing' && progress > 0 ? ` ${progress}%` : '';

	indicator.innerHTML = `
		<div class="video-progress-bar">
			<div class="video-progress-fill" style="width: ${progress}%"></div>
		</div>
		<span class="video-progress-text">${message}${progressText}</span>
	`;
}

/**
 * Replace a video card's HTML with a fresh render (e.g., after transcription completes).
 */
function replaceVideoCard(videoList: HTMLElement, video: Video): void {
	const card = videoList.querySelector(`[data-video-id="${video.id}"].video-card`) as HTMLElement | null;
	if (!card) return;

	const temp = document.createElement('div');
	temp.innerHTML = renderVideo(video);
	const newCard = temp.firstElementChild;
	if (newCard) {
		card.replaceWith(newCard);
	}
}

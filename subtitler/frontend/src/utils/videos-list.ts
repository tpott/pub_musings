/**
 * Video list rendering, formatting, and operations for the videos page.
 * Handles rendering video cards and video-level actions (delete, reprocess).
 */

import { csrfFetch } from './csrf';
import { getOrCreateSessionId } from './session';
import { escapeHtml } from './html';
import { showAlert, showConfirm } from './dialog';

export interface SubtitleTrack {
	index: number;
	language?: string;
	title?: string;
	codec: string;
	default?: boolean;
	forced?: boolean;
	text_based?: boolean;
}

export interface Video {
	id: string;
	filename: string;
	size: number;
	content_type?: string;
	created_at: string;
	transcription_status?: string;
	user_id?: string;
	session_id?: string;
	expires_at?: string | null;
	thumbnail_path?: string;
	embedded_subtitles?: SubtitleTrack[];
}

export function formatBytes(bytes: number): string {
	if (bytes < 1024) return bytes + ' B';
	if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
	return (bytes / 1024 / 1024).toFixed(1) + ' MB';
}

export function formatDate(dateStr: string): string {
	const date = new Date(dateStr);
	return date.toLocaleDateString('en-US', {
		year: 'numeric',
		month: 'short',
		day: 'numeric',
		hour: '2-digit',
		minute: '2-digit'
	});
}

export function formatRetention(video: Video): string {
	if (!video.expires_at) return '';

	const expiresAt = new Date(video.expires_at);
	const now = new Date();
	const msRemaining = expiresAt.getTime() - now.getTime();

	// If already expired
	if (msRemaining <= 0) {
		return '<span class="retention-badge retention-expired">Expired</span>';
	}

	const hoursRemaining = Math.ceil(msRemaining / (1000 * 60 * 60));
	const daysRemaining = Math.ceil(msRemaining / (1000 * 60 * 60 * 24));

	// Anonymous videos (48 hour retention) - show hours
	if (!video.user_id) {
		if (hoursRemaining <= 6) {
			return `<span class="retention-badge retention-warning">Expires in ${hoursRemaining}h</span>`;
		}
		return `<span class="retention-badge retention-info">Expires in ${hoursRemaining}h</span>`;
	}

	// Registered users (90 day retention) - show days
	if (daysRemaining <= 7) {
		return `<span class="retention-badge retention-warning">Expires in ${daysRemaining}d</span>`;
	}
	return `<span class="retention-badge retention-info">${daysRemaining}d remaining</span>`;
}

export function getStatusBadge(status: string | undefined): string {
	const statusLabels: Record<string, string> = {
		complete: 'Complete',
		processing: 'Processing',
		pending: 'Pending',
		error: 'Error',
		none: 'Not Started'
	};
	const s = status ?? 'none';
	const label = statusLabels[s] || s;
	return `<span class="status-badge status-${s}">${label}</span>`;
}

export function getEmbeddedSubtitlesBadge(video: Video): string {
	if (!video.embedded_subtitles || video.embedded_subtitles.length === 0) {
		return '';
	}

	// Extract unique languages
	const languages = video.embedded_subtitles
		.map(s => s.language || s.title || 'Unknown')
		.filter(lang => lang && lang.trim())
		.map(lang => lang.toUpperCase())
		.filter((v, i, a) => a.indexOf(v) === i); // unique

	const count = video.embedded_subtitles.length;
	const langStr = languages.length > 0 ? languages.join(', ') : `${count} track${count > 1 ? 's' : ''}`;
	const tooltip = `Video has embedded subtitles: ${langStr}`;

	return `<span class="embedded-subtitles-badge" title="${escapeHtml(tooltip)}">&#x1F4DD; ${escapeHtml(langStr)}</span>`;
}

export function renderVideo(video: Video): string {
	const showViewBtn = video.transcription_status === 'complete';
	const showDownloadBtn = video.transcription_status === 'complete';
	const showRetryBtn = video.transcription_status === 'error';
	const hasThumbnail = video.thumbnail_path && video.thumbnail_path !== '';

	return `
		<div class="video-card" data-video-id="${video.id}">
			<div class="video-top">
				${hasThumbnail
					? `<img class="video-thumbnail${showViewBtn ? ' clickable-thumbnail' : ''}" src="/api/videos/${video.id}/thumbnail" alt="Thumbnail for ${escapeHtml(video.filename)}" loading="lazy" ${showViewBtn ? `data-video-id="${video.id}" data-filename="${escapeHtml(video.filename)}" role="button" tabindex="0" aria-label="Play video: ${escapeHtml(video.filename)}" style="cursor: pointer;"` : ''} />`
					: `<div class="video-thumbnail-placeholder${showViewBtn ? ' clickable-thumbnail' : ''}" ${showViewBtn ? `data-video-id="${video.id}" data-filename="${escapeHtml(video.filename)}" role="button" tabindex="0" aria-label="Play video: ${escapeHtml(video.filename)}" style="cursor: pointer;"` : ''}>&#x1F3AC;</div>`
				}
				<div class="video-info">
					<div class="video-filename">${escapeHtml(video.filename)}</div>
					<div class="video-meta">
						<span>${formatBytes(video.size)}</span>
						<span>${formatDate(video.created_at)}</span>
						${getStatusBadge(video.transcription_status)}
						${getEmbeddedSubtitlesBadge(video)}
						${formatRetention(video)}
					</div>
				</div>
			</div>
			<div class="video-actions">
				${showViewBtn ? `<button class="btn btn-primary btn-view" data-video-id="${video.id}" data-filename="${escapeHtml(video.filename)}">View</button>` : ''}
				${showDownloadBtn ? `<button class="btn btn-link btn-download-srt" data-video-id="${video.id}" data-filename="${escapeHtml(video.filename)}" aria-label="Download subtitles in SRT format">SRT</button>` : ''}
				${showDownloadBtn ? `<button class="btn btn-link btn-download-vtt" data-video-id="${video.id}" data-filename="${escapeHtml(video.filename)}" aria-label="Download subtitles in VTT format">VTT</button>` : ''}
				${showDownloadBtn ? `<button class="btn btn-link btn-download-json" data-video-id="${video.id}" data-filename="${escapeHtml(video.filename)}" aria-label="Download subtitles in JSON format">JSON</button>` : ''}
				${video.transcription_status === 'pending' || video.transcription_status === 'none' ? `<a href="/upload?id=${video.id}" class="btn btn-primary">Transcribe</a>` : ''}
				${showRetryBtn ? `<button class="btn btn-retry" data-video-id="${video.id}">Retry</button>` : ''}
				<button class="btn btn-delete-sm" data-video-id="${video.id}" data-filename="${escapeHtml(video.filename)}" aria-label="Delete ${escapeHtml(video.filename)}">Delete</button>
			</div>
		</div>
	`;
}

export async function reprocessVideo(
	videoId: string,
	button: HTMLButtonElement,
	isAuthenticated: boolean
): Promise<void> {
	button.disabled = true;
	button.setAttribute('aria-busy', 'true');
	button.textContent = 'Retrying...';

	try {
		// For anonymous users, include session_id
		let url = `/api/videos/${videoId}/reprocess`;
		if (!isAuthenticated) {
			const sessionId = getOrCreateSessionId();
			url = `/api/videos/${videoId}/reprocess?session_id=${encodeURIComponent(sessionId)}`;
		}

		const response = await csrfFetch(url, { method: 'POST' });
		const data = await response.json();

		if (!response.ok) {
			await showAlert(`Failed to retry: ${data.error || 'Unknown error'}`, 'error');
			button.disabled = false;
			button.removeAttribute('aria-busy');
			button.textContent = 'Retry';
			return;
		}

		// Redirect to upload page to see progress
		window.location.href = `/upload?id=${videoId}`;
	} catch (err) {
		await showAlert(`Failed to retry: ${err instanceof Error ? err.message : 'Unknown error'}`, 'error');
		button.disabled = false;
		button.removeAttribute('aria-busy');
		button.textContent = 'Retry';
	}
}

export async function deleteVideo(
	videoId: string,
	filename: string,
	button: HTMLButtonElement,
	videoList: HTMLElement,
	isAuthenticated: boolean
): Promise<void> {
	// Confirm deletion with styled dialog
	const confirmed = await showConfirm(
		`Are you sure you want to delete "${filename}"? This cannot be undone.`,
		{ title: 'Delete Video', confirmText: 'Delete', cancelText: 'Cancel' }
	);
	if (!confirmed) {
		return;
	}

	button.disabled = true;
	button.setAttribute('aria-busy', 'true');
	button.textContent = 'Deleting...';

	try {
		// For anonymous users, include session_id
		let url = `/api/videos/${videoId}`;
		if (!isAuthenticated) {
			const sessionId = getOrCreateSessionId();
			url = `/api/videos/${videoId}?session_id=${encodeURIComponent(sessionId)}`;
		}

		const response = await csrfFetch(url, { method: 'DELETE' });
		const data = await response.json();

		if (!response.ok) {
			await showAlert(`Failed to delete: ${data.error || 'Unknown error'}`, 'error');
			button.disabled = false;
			button.removeAttribute('aria-busy');
			button.textContent = 'Delete';
			return;
		}

		// Remove the video card from the DOM
		const videoCard = button.closest('.video-card');
		if (videoCard) {
			videoCard.remove();
		}

		// Check if there are any videos left, show empty state if not
		const remainingVideos = videoList.querySelectorAll('.video-card');
		if (remainingVideos.length === 0) {
			videoList.innerHTML = `
				<div class="empty-state">
					<div class="empty-icon">&#x1F4F9;</div>
					<p class="empty-text">No videos found</p>
					<a href="/upload" class="upload-btn">Upload Your First Video</a>
				</div>
			`;
		}
	} catch (err) {
		await showAlert(`Failed to delete: ${err instanceof Error ? err.message : 'Unknown error'}`, 'error');
		button.disabled = false;
		button.textContent = 'Delete';
	}
}

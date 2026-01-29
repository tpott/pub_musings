/**
 * File upload logic for the upload page.
 * Handles single and chunked file uploads with progress tracking.
 */

import { csrfFetch, getCsrfToken, CSRF_HEADER } from './csrf';
import { getOrCreateSessionId, recordUploadSession, removeUploadSessionRecord } from './session';
import type { UploadResponse, UploadSession, LanguageHints } from '../types/transcription';
import {
	CHUNK_SIZE,
	MAX_FILE_SIZE,
	UPLOAD_TIMEOUT_MS,
	UPLOAD_SESSION_PREFIX
} from './upload-constants';

/** Callbacks the upload module uses to interact with the page */
export interface UploadCallbacks {
	showStatus: (message: string, type?: 'error' | 'success') => void;
	showProgress: (percent: number) => void;
	hideProgress: () => void;
	onUploadComplete: (uploadId: string, languageHints?: LanguageHints) => void;
	isAuthenticated: () => boolean;
}

/** Mutable state for tracking active upload operations */
export interface UploadState {
	isUploading: boolean;
	currentUploadXhr: XMLHttpRequest | null;
	uploadTimeoutId: ReturnType<typeof setTimeout> | null;
}

export function createUploadState(): UploadState {
	return {
		isUploading: false,
		currentUploadXhr: null,
		uploadTimeoutId: null
	};
}

/** Validate a file before upload. Returns error message or null if valid. */
export function validateFile(file: File): string | null {
	if (!file.type.startsWith('video/')) {
		return 'Please select a video file';
	}
	if (file.size > MAX_FILE_SIZE) {
		const actualSizeMB = (file.size / 1024 / 1024).toFixed(1);
		return `File too large: ${actualSizeMB} MB. Maximum allowed size is 500 MB.`;
	}
	return null;
}

/** Add session_id query param for anonymous users */
export function getSessionQueryUrl(baseUrl: string, isAuthenticated: boolean): string {
	if (!isAuthenticated) {
		const sessionId = getOrCreateSessionId();
		const separator = baseUrl.includes('?') ? '&' : '?';
		return `${baseUrl}${separator}session_id=${encodeURIComponent(sessionId)}`;
	}
	return baseUrl;
}

/** Upload a file (auto-selects single vs chunked based on size) */
export async function uploadFile(
	file: File,
	state: UploadState,
	callbacks: UploadCallbacks
): Promise<void> {
	state.isUploading = true;

	try {
		callbacks.showStatus('Uploading...');
		callbacks.showProgress(0);

		if (file.size > CHUNK_SIZE) {
			await uploadFileChunked(file, state, callbacks);
		} else {
			await uploadFileSingle(file, state, callbacks);
		}
	} catch (error) {
		callbacks.hideProgress();
		const errorMessage = error instanceof Error ? error.message : String(error);
		if (errorMessage === 'Upload cancelled') {
			callbacks.showStatus('Upload timed out. Please try again with a smaller file or check your connection.', 'error');
		} else {
			callbacks.showStatus(`Upload failed: ${errorMessage}`, 'error');
		}
	} finally {
		state.isUploading = false;
		state.currentUploadXhr = null;
		if (state.uploadTimeoutId) {
			clearTimeout(state.uploadTimeoutId);
			state.uploadTimeoutId = null;
		}
	}
}

/** Single-file upload for small files */
async function uploadFileSingle(
	file: File,
	state: UploadState,
	callbacks: UploadCallbacks
): Promise<void> {
	const formData = new FormData();
	formData.append('video', file);

	const xhr = new XMLHttpRequest();
	state.currentUploadXhr = xhr;

	xhr.upload.addEventListener('progress', (e) => {
		if (e.lengthComputable) {
			const percent = Math.round((e.loaded / e.total) * 100);
			callbacks.showProgress(percent);
			callbacks.showStatus(`Uploading... ${percent}%`);
		}
	});

	const uploadPromise = new Promise<UploadResponse>((resolve, reject) => {
		xhr.onload = () => {
			if (state.uploadTimeoutId) {
				clearTimeout(state.uploadTimeoutId);
				state.uploadTimeoutId = null;
			}
			state.currentUploadXhr = null;
			if (xhr.status >= 200 && xhr.status < 300) {
				try {
					resolve(JSON.parse(xhr.responseText));
				} catch (parseError) {
					console.error('Failed to parse upload response:', parseError, xhr.responseText);
					reject(new Error('Invalid JSON response'));
				}
			} else {
				try {
					const error = JSON.parse(xhr.responseText);
					reject(new Error(error.error || 'Upload failed'));
				} catch (parseError) {
					console.error('Failed to parse upload error response:', parseError, xhr.responseText);
					reject(new Error(`Upload failed: ${xhr.status}`));
				}
			}
		};
		xhr.onerror = () => {
			if (state.uploadTimeoutId) {
				clearTimeout(state.uploadTimeoutId);
				state.uploadTimeoutId = null;
			}
			state.currentUploadXhr = null;
			reject(new Error('Network error'));
		};
		xhr.onabort = () => {
			if (state.uploadTimeoutId) {
				clearTimeout(state.uploadTimeoutId);
				state.uploadTimeoutId = null;
			}
			state.currentUploadXhr = null;
			reject(new Error('Upload cancelled'));
		};
	});

	let uploadUrl = getSessionQueryUrl('/api/upload', callbacks.isAuthenticated());
	xhr.open('POST', uploadUrl);

	if (callbacks.isAuthenticated()) {
		const csrfToken = await getCsrfToken();
		if (csrfToken) {
			xhr.setRequestHeader(CSRF_HEADER, csrfToken);
		}
	}

	state.uploadTimeoutId = setTimeout(() => {
		if (state.currentUploadXhr) {
			state.currentUploadXhr.abort();
			state.currentUploadXhr = null;
		}
		state.uploadTimeoutId = null;
	}, UPLOAD_TIMEOUT_MS);

	xhr.send(formData);

	const result = await uploadPromise;
	callbacks.hideProgress();
	callbacks.showStatus('Upload complete! Starting transcription...', 'success');
	callbacks.onUploadComplete(result.upload_id, result.language_hints);
}

/** Chunked upload for large files */
async function uploadFileChunked(
	file: File,
	state: UploadState,
	callbacks: UploadCallbacks
): Promise<void> {
	const sessionKey = `${UPLOAD_SESSION_PREFIX}${file.name}:${file.size}`;
	let sessionId = localStorage.getItem(sessionKey);
	let uploadSession: UploadSession | null = null;
	let receivedChunks: number[] = [];

	// Check for resumable session
	if (sessionId) {
		try {
			const statusUrl = getSessionQueryUrl(`/api/upload/status/${sessionId}`, callbacks.isAuthenticated());
			const statusRes = await fetch(statusUrl);
			if (statusRes.ok) {
				uploadSession = await statusRes.json();
				if (uploadSession.status === 'in_progress') {
					receivedChunks = uploadSession.received_chunks || [];
					callbacks.showStatus(`Resuming upload... ${uploadSession.progress}% complete`);
					callbacks.showProgress(uploadSession.progress);
				} else {
					localStorage.removeItem(sessionKey);
					removeUploadSessionRecord(sessionKey);
					sessionId = null;
					uploadSession = null;
				}
			} else {
				localStorage.removeItem(sessionKey);
				removeUploadSessionRecord(sessionKey);
				sessionId = null;
			}
		} catch (error) {
			console.error('Failed to check resumable upload session:', error);
			localStorage.removeItem(sessionKey);
			removeUploadSessionRecord(sessionKey);
			sessionId = null;
		}
	}

	// Initialize new upload session if needed
	if (!sessionId) {
		const initUrl = getSessionQueryUrl('/api/upload/init', callbacks.isAuthenticated());
		const initRes = await csrfFetch(initUrl, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				filename: file.name,
				size: file.size,
				content_type: file.type || 'video/mp4',
				chunk_size: CHUNK_SIZE
			})
		});

		if (!initRes.ok) {
			const error = await initRes.json().catch(() => ({ error: 'Failed to initialize upload' }));
			throw new Error(error.error || 'Failed to initialize upload');
		}

		uploadSession = await initRes.json();
		sessionId = uploadSession.upload_session_id;
		localStorage.setItem(sessionKey, sessionId);
		recordUploadSession(sessionKey);
		receivedChunks = [];
	}

	const totalChunks = uploadSession!.total_chunks;

	// Upload chunks
	for (let i = 0; i < totalChunks; i++) {
		if (receivedChunks.includes(i)) {
			continue;
		}

		const start = i * CHUNK_SIZE;
		const end = Math.min(start + CHUNK_SIZE, file.size);
		const chunk = file.slice(start, end);

		const formData = new FormData();
		formData.append('upload_session_id', sessionId!);
		formData.append('chunk_index', i.toString());
		formData.append('chunk', chunk);

		const xhr = new XMLHttpRequest();

		xhr.upload.addEventListener('progress', (e) => {
			if (e.lengthComputable) {
				const chunkProgress = e.loaded / e.total;
				const overallProgress = ((i + chunkProgress) / totalChunks) * 100;
				callbacks.showProgress(Math.round(overallProgress));
				callbacks.showStatus(`Uploading chunk ${i + 1}/${totalChunks}... ${Math.round(chunkProgress * 100)}%`);
			}
		});

		const chunkUrl = getSessionQueryUrl('/api/upload/chunk', callbacks.isAuthenticated());
		const chunkCsrfToken = callbacks.isAuthenticated() ? await getCsrfToken() : null;

		state.currentUploadXhr = xhr;

		await new Promise<void>((resolve, reject) => {
			const chunkTimeout = setTimeout(() => {
				xhr.abort();
				reject(new Error(`Chunk ${i} upload timed out`));
			}, 60 * 1000);

			xhr.onload = () => {
				clearTimeout(chunkTimeout);
				state.currentUploadXhr = null;
				if (xhr.status >= 200 && xhr.status < 300) {
					resolve();
				} else {
					try {
						const error = JSON.parse(xhr.responseText);
						reject(new Error(error.error || `Chunk ${i} upload failed`));
					} catch (parseError) {
						console.error('Failed to parse chunk upload error response:', parseError, xhr.responseText);
						reject(new Error(`Chunk ${i} upload failed: ${xhr.status}`));
					}
				}
			};
			xhr.onerror = () => {
				clearTimeout(chunkTimeout);
				state.currentUploadXhr = null;
				reject(new Error(`Network error uploading chunk ${i}`));
			};
			xhr.onabort = () => {
				clearTimeout(chunkTimeout);
				state.currentUploadXhr = null;
				reject(new Error('Upload cancelled'));
			};

			xhr.open('POST', chunkUrl);
			if (chunkCsrfToken) {
				xhr.setRequestHeader(CSRF_HEADER, chunkCsrfToken);
			}
			xhr.send(formData);
		});

		const progress = Math.round(((i + 1) / totalChunks) * 100);
		callbacks.showProgress(progress);
		callbacks.showStatus(`Uploaded chunk ${i + 1}/${totalChunks} (${progress}%)`);
	}

	// Complete the upload
	callbacks.showStatus('Finalizing upload...');
	const completeUrl = getSessionQueryUrl('/api/upload/complete', callbacks.isAuthenticated());
	const completeRes = await csrfFetch(completeUrl, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ upload_session_id: sessionId })
	});

	if (!completeRes.ok) {
		const error = await completeRes.json().catch(() => ({ error: 'Failed to complete upload' }));
		throw new Error(error.error || 'Failed to complete upload');
	}

	const result = await completeRes.json();

	// Clean up localStorage
	localStorage.removeItem(sessionKey);
	removeUploadSessionRecord(sessionKey);

	callbacks.hideProgress();
	callbacks.showStatus('Upload complete! Starting transcription...', 'success');
	callbacks.onUploadComplete(result.upload_id, result.language_hints);
}

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
	createUploadState,
	validateFile,
	getSessionQueryUrl,
	uploadFile,
	type UploadCallbacks,
	type UploadState,
} from './upload-file';

// --- Mocks ---

vi.mock('./csrf', () => ({
	csrfFetch: vi.fn(),
	getCsrfToken: vi.fn(() => Promise.resolve('csrf-token-123')),
	CSRF_HEADER: 'X-CSRF-Token',
}));

vi.mock('./session', () => ({
	getOrCreateSessionId: vi.fn(() => 'session-abc-123'),
	recordUploadSession: vi.fn(),
	removeUploadSessionRecord: vi.fn(),
}));

// Mock upload-constants with smaller values for testing
vi.mock('./upload-constants', () => ({
	CHUNK_SIZE: 50 * 1024 * 1024, // 50MB
	MAX_FILE_SIZE: 500 * 1024 * 1024, // 500MB
	UPLOAD_TIMEOUT_MS: 5 * 60 * 1000, // 5 minutes
	UPLOAD_SESSION_PREFIX: 'subtitler:upload_session:',
}));

const localStorageMock = (() => {
	let store: Record<string, string> = {};
	return {
		getItem: vi.fn((key: string) => store[key] || null),
		setItem: vi.fn((key: string, value: string) => {
			store[key] = value;
		}),
		removeItem: vi.fn((key: string) => {
			delete store[key];
		}),
		clear: vi.fn(() => {
			store = {};
		}),
		_setStore: (newStore: Record<string, string>) => {
			store = { ...newStore };
		},
	};
})();

function makeCallbacks(overrides: Partial<UploadCallbacks> = {}): UploadCallbacks {
	return {
		showStatus: vi.fn(),
		showProgress: vi.fn(),
		hideProgress: vi.fn(),
		onUploadComplete: vi.fn(),
		isAuthenticated: vi.fn(() => true),
		...overrides,
	};
}

function makeFile(name: string, size: number, type = 'video/mp4'): File {
	// Create a mock File object
	const file = new File([new ArrayBuffer(Math.min(size, 100))], name, { type });
	Object.defineProperty(file, 'size', { value: size });
	return file;
}

beforeEach(() => {
	localStorageMock.clear();
	vi.stubGlobal('localStorage', localStorageMock);
	vi.clearAllMocks();
	vi.useFakeTimers();
});

afterEach(() => {
	vi.useRealTimers();
	vi.restoreAllMocks();
});

// --- Tests ---

describe('createUploadState', () => {
	it('should return default state', () => {
		const state = createUploadState();
		expect(state.isUploading).toBe(false);
		expect(state.currentUploadXhr).toBeNull();
		expect(state.uploadTimeoutId).toBeNull();
	});

	it('should return independent state objects', () => {
		const state1 = createUploadState();
		const state2 = createUploadState();
		state1.isUploading = true;
		expect(state2.isUploading).toBe(false);
	});
});

describe('validateFile', () => {
	it('should accept valid video file', () => {
		const file = makeFile('test.mp4', 10 * 1024 * 1024, 'video/mp4');
		expect(validateFile(file)).toBeNull();
	});

	it('should reject non-video files', () => {
		const file = makeFile('doc.pdf', 1024, 'application/pdf');
		expect(validateFile(file)).toBe('Please select a video file');
	});

	it('should reject files over 500MB', () => {
		const file = makeFile('huge.mp4', 501 * 1024 * 1024, 'video/mp4');
		const result = validateFile(file);
		expect(result).toContain('File too large');
		expect(result).toContain('501.0 MB');
		expect(result).toContain('500 MB');
	});

	it('should accept exactly 500MB file', () => {
		const file = makeFile('exact.mp4', 500 * 1024 * 1024, 'video/mp4');
		expect(validateFile(file)).toBeNull();
	});

	it('should accept various video types', () => {
		expect(validateFile(makeFile('a.mov', 1024, 'video/quicktime'))).toBeNull();
		expect(validateFile(makeFile('b.webm', 1024, 'video/webm'))).toBeNull();
		expect(validateFile(makeFile('c.avi', 1024, 'video/x-msvideo'))).toBeNull();
	});

	it('should reject image files', () => {
		const file = makeFile('pic.png', 1024, 'image/png');
		expect(validateFile(file)).toBe('Please select a video file');
	});
});

describe('getSessionQueryUrl', () => {
	it('should return base URL for authenticated users', () => {
		expect(getSessionQueryUrl('/api/upload', true)).toBe('/api/upload');
	});

	it('should append session_id for anonymous users', () => {
		const result = getSessionQueryUrl('/api/upload', false);
		expect(result).toContain('/api/upload?session_id=');
		expect(result).toContain('session-abc-123');
	});

	it('should use & separator when URL already has query params', () => {
		const result = getSessionQueryUrl('/api/upload?foo=bar', false);
		expect(result).toContain('&session_id=');
	});

	it('should URL-encode the session ID', async () => {
		const sessionMock = await import('./session');
		(sessionMock.getOrCreateSessionId as ReturnType<typeof vi.fn>).mockReturnValueOnce('session with spaces');

		const result = getSessionQueryUrl('/api/upload', false);
		expect(result).toContain('session%20with%20spaces');
	});
});

describe('uploadFile', () => {
	it('should set isUploading to true during upload', async () => {
		const state = createUploadState();
		const callbacks = makeCallbacks();

		// Mock XMLHttpRequest for single upload (small file)
		const mockXhr = {
			open: vi.fn(),
			send: vi.fn(),
			setRequestHeader: vi.fn(),
			upload: { addEventListener: vi.fn() },
			onload: null as (() => void) | null,
			onerror: null as (() => void) | null,
			onabort: null as (() => void) | null,
			status: 200,
			responseText: JSON.stringify({ upload_id: 'u1', filename: 'test.mp4', size: 1024, message: 'ok' }),
		};
		vi.stubGlobal('XMLHttpRequest', vi.fn(() => mockXhr));

		const file = makeFile('test.mp4', 1024, 'video/mp4');
		const uploadPromise = uploadFile(file, state, callbacks);

		expect(state.isUploading).toBe(true);

		// Trigger onload to complete upload
		if (mockXhr.onload) mockXhr.onload();
		await uploadPromise;

		expect(state.isUploading).toBe(false);
	});

	it('should call onUploadComplete on success', async () => {
		const state = createUploadState();
		const callbacks = makeCallbacks();

		const mockXhr = {
			open: vi.fn(),
			send: vi.fn(),
			setRequestHeader: vi.fn(),
			upload: { addEventListener: vi.fn() },
			onload: null as (() => void) | null,
			onerror: null as (() => void) | null,
			onabort: null as (() => void) | null,
			status: 200,
			responseText: JSON.stringify({
				upload_id: 'upload-xyz',
				filename: 'test.mp4',
				size: 1024,
				message: 'ok',
				language_hints: { hints: [], suggested_language: 'en', suggested_confidence: 'high' },
			}),
		};
		vi.stubGlobal('XMLHttpRequest', vi.fn(() => mockXhr));

		const file = makeFile('test.mp4', 1024, 'video/mp4');
		const uploadPromise = uploadFile(file, state, callbacks);
		if (mockXhr.onload) mockXhr.onload();
		await uploadPromise;

		expect(callbacks.onUploadComplete).toHaveBeenCalledWith(
			'upload-xyz',
			expect.objectContaining({ suggested_language: 'en' })
		);
		expect(callbacks.showStatus).toHaveBeenCalledWith(
			'Upload complete! Starting transcription...',
			'success'
		);
	});

	it('should handle network error', async () => {
		const state = createUploadState();
		const callbacks = makeCallbacks();

		const mockXhr = {
			open: vi.fn(),
			send: vi.fn(),
			setRequestHeader: vi.fn(),
			upload: { addEventListener: vi.fn() },
			onload: null as (() => void) | null,
			onerror: null as (() => void) | null,
			onabort: null as (() => void) | null,
		};
		vi.stubGlobal('XMLHttpRequest', vi.fn(() => mockXhr));

		const file = makeFile('test.mp4', 1024, 'video/mp4');
		const uploadPromise = uploadFile(file, state, callbacks);
		if (mockXhr.onerror) mockXhr.onerror();
		await uploadPromise;

		expect(callbacks.showStatus).toHaveBeenCalledWith(
			expect.stringContaining('Upload failed'),
			'error'
		);
		expect(state.isUploading).toBe(false);
	});

	it('should handle abort (timeout)', async () => {
		const state = createUploadState();
		const callbacks = makeCallbacks();

		const mockXhr = {
			open: vi.fn(),
			send: vi.fn(),
			setRequestHeader: vi.fn(),
			upload: { addEventListener: vi.fn() },
			onload: null as (() => void) | null,
			onerror: null as (() => void) | null,
			onabort: null as (() => void) | null,
		};
		vi.stubGlobal('XMLHttpRequest', vi.fn(() => mockXhr));

		const file = makeFile('test.mp4', 1024, 'video/mp4');
		const uploadPromise = uploadFile(file, state, callbacks);
		if (mockXhr.onabort) mockXhr.onabort();
		await uploadPromise;

		expect(callbacks.showStatus).toHaveBeenCalledWith(
			expect.stringContaining('timed out'),
			'error'
		);
	});

	it('should handle server error response', async () => {
		const state = createUploadState();
		const callbacks = makeCallbacks();

		const mockXhr = {
			open: vi.fn(),
			send: vi.fn(),
			setRequestHeader: vi.fn(),
			upload: { addEventListener: vi.fn() },
			onload: null as (() => void) | null,
			onerror: null as (() => void) | null,
			onabort: null as (() => void) | null,
			status: 413,
			responseText: JSON.stringify({ error: 'File too large' }),
		};
		vi.stubGlobal('XMLHttpRequest', vi.fn(() => mockXhr));

		const file = makeFile('test.mp4', 1024, 'video/mp4');
		const uploadPromise = uploadFile(file, state, callbacks);
		if (mockXhr.onload) mockXhr.onload();
		await uploadPromise;

		expect(callbacks.showStatus).toHaveBeenCalledWith(
			expect.stringContaining('File too large'),
			'error'
		);
	});

	it('should set CSRF header for authenticated users', async () => {
		const state = createUploadState();
		const callbacks = makeCallbacks({ isAuthenticated: vi.fn(() => true) });

		const mockXhr = {
			open: vi.fn(),
			send: vi.fn(),
			setRequestHeader: vi.fn(),
			upload: { addEventListener: vi.fn() },
			onload: null as (() => void) | null,
			onerror: null as (() => void) | null,
			onabort: null as (() => void) | null,
			status: 200,
			responseText: JSON.stringify({ upload_id: 'u1', filename: 'a.mp4', size: 1, message: 'ok' }),
		};
		vi.stubGlobal('XMLHttpRequest', vi.fn(() => mockXhr));

		const file = makeFile('test.mp4', 1024, 'video/mp4');
		const uploadPromise = uploadFile(file, state, callbacks);
		if (mockXhr.onload) mockXhr.onload();
		await uploadPromise;

		expect(mockXhr.setRequestHeader).toHaveBeenCalledWith('X-CSRF-Token', 'csrf-token-123');
	});

	it('should not set CSRF header for anonymous users', async () => {
		const state = createUploadState();
		const callbacks = makeCallbacks({ isAuthenticated: vi.fn(() => false) });

		const mockXhr = {
			open: vi.fn(),
			send: vi.fn(),
			setRequestHeader: vi.fn(),
			upload: { addEventListener: vi.fn() },
			onload: null as (() => void) | null,
			onerror: null as (() => void) | null,
			onabort: null as (() => void) | null,
			status: 200,
			responseText: JSON.stringify({ upload_id: 'u1', filename: 'a.mp4', size: 1, message: 'ok' }),
		};
		vi.stubGlobal('XMLHttpRequest', vi.fn(() => mockXhr));

		const file = makeFile('test.mp4', 1024, 'video/mp4');
		const uploadPromise = uploadFile(file, state, callbacks);
		if (mockXhr.onload) mockXhr.onload();
		await uploadPromise;

		expect(mockXhr.setRequestHeader).not.toHaveBeenCalledWith('X-CSRF-Token', expect.any(String));
	});

	it('should clean up timeout on completion', async () => {
		const state = createUploadState();
		const callbacks = makeCallbacks();

		const mockXhr = {
			open: vi.fn(),
			send: vi.fn(),
			setRequestHeader: vi.fn(),
			upload: { addEventListener: vi.fn() },
			onload: null as (() => void) | null,
			onerror: null as (() => void) | null,
			onabort: null as (() => void) | null,
			status: 200,
			responseText: JSON.stringify({ upload_id: 'u1', filename: 'a.mp4', size: 1, message: 'ok' }),
		};
		vi.stubGlobal('XMLHttpRequest', vi.fn(() => mockXhr));

		const file = makeFile('test.mp4', 1024, 'video/mp4');
		const uploadPromise = uploadFile(file, state, callbacks);
		if (mockXhr.onload) mockXhr.onload();
		await uploadPromise;

		expect(state.uploadTimeoutId).toBeNull();
		expect(state.currentUploadXhr).toBeNull();
	});
});

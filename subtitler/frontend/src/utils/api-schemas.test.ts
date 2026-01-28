/**
 * Tests for API response schema validation
 */
import { describe, it, expect, vi } from 'vitest';
import {
	VideoSchema,
	VideoListResponseSchema,
	UserSchema,
	AuthMeResponseSchema,
	SessionSchema,
	SessionListResponseSchema,
	TotpSetupResponseSchema,
	TotpVerifyResponseSchema,
	RecoveryCodesResponseSchema,
	TranscriptionSegmentSchema,
	TranscriptionResultSchema,
	TranscriptionStatusResponseSchema,
	UploadInitResponseSchema,
	UploadCompleteResponseSchema,
	CsrfTokenResponseSchema,
	LoginResponseSchema,
	RegisterResponseSchema,
	ErrorResponseSchema,
	safeParse,
	parse,
	isValid,
	validateResponse,
	validateResponseOrThrow,
} from './api-schemas';

describe('api-schemas', () => {
	describe('VideoSchema', () => {
		it('validates a complete video object', () => {
			const video = {
				id: 'abc123',
				filename: 'test.mp4',
				size: 1024,
				created_at: '2024-01-01T00:00:00Z',
			};
			expect(VideoSchema.safeParse(video).success).toBe(true);
		});

		it('validates a video with all optional fields', () => {
			const video = {
				id: 'abc123',
				filename: 'test.mp4',
				size: 1024,
				content_type: 'video/mp4',
				created_at: '2024-01-01T00:00:00Z',
				user_id: 'user123',
				session_id: 'sess123',
				transcription_status: 'complete',
				expires_at: '2024-02-01T00:00:00Z',
				thumbnail_path: '/path/to/thumb.jpg',
				embedded_subtitles: [
					{
						index: 0,
						language: 'en',
						title: 'English',
						codec: 'srt',
					},
				],
			};
			expect(VideoSchema.safeParse(video).success).toBe(true);
		});

		it('rejects video missing required fields', () => {
			const video = { id: 'abc123' };
			expect(VideoSchema.safeParse(video).success).toBe(false);
		});
	});

	describe('VideoListResponseSchema', () => {
		it('validates a video list response', () => {
			const response = {
				videos: [
					{
						id: 'abc123',
						filename: 'test.mp4',
						size: 1024,
						created_at: '2024-01-01T00:00:00Z',
					},
				],
				total_count: 1,
				has_more: false,
			};
			expect(VideoListResponseSchema.safeParse(response).success).toBe(true);
		});

		it('validates empty video list', () => {
			const response = { videos: [] };
			expect(VideoListResponseSchema.safeParse(response).success).toBe(true);
		});

		it('rejects missing videos array', () => {
			const response = { total_count: 0 };
			expect(VideoListResponseSchema.safeParse(response).success).toBe(false);
		});
	});

	describe('UserSchema', () => {
		it('validates a user object', () => {
			const user = { email: 'test@example.com' };
			expect(UserSchema.safeParse(user).success).toBe(true);
		});

		it('validates user with optional fields', () => {
			const user = {
				email: 'test@example.com',
				totp_enabled: true,
				role: 'admin',
			};
			expect(UserSchema.safeParse(user).success).toBe(true);
		});

		it('rejects invalid email', () => {
			const user = { email: 'invalid' };
			expect(UserSchema.safeParse(user).success).toBe(false);
		});
	});

	describe('AuthMeResponseSchema', () => {
		it('validates authenticated response', () => {
			const response = {
				user: { email: 'test@example.com' },
			};
			expect(AuthMeResponseSchema.safeParse(response).success).toBe(true);
		});

		it('validates unauthenticated response', () => {
			const response = {};
			expect(AuthMeResponseSchema.safeParse(response).success).toBe(true);
		});
	});

	describe('SessionListResponseSchema', () => {
		it('validates session list', () => {
			const response = {
				sessions: [
					{
						id: 'sess123',
						created_at: '2024-01-01T00:00:00Z',
					},
				],
			};
			expect(SessionListResponseSchema.safeParse(response).success).toBe(true);
		});
	});

	describe('TotpSetupResponseSchema', () => {
		it('validates TOTP setup response', () => {
			const response = {
				secret: 'ABCDEFGHIJKLMNOP',
				secret_display: 'ABCD EFGH IJKL MNOP',
				qr_code: 'data:image/png;base64,abc123',
			};
			expect(TotpSetupResponseSchema.safeParse(response).success).toBe(true);
		});
	});

	describe('TotpVerifyResponseSchema', () => {
		it('validates TOTP verify response', () => {
			const response = {
				success: true,
				recovery_codes: ['CODE1', 'CODE2'],
			};
			expect(TotpVerifyResponseSchema.safeParse(response).success).toBe(true);
		});

		it('validates response without recovery codes', () => {
			const response = { success: true };
			expect(TotpVerifyResponseSchema.safeParse(response).success).toBe(true);
		});
	});

	describe('TranscriptionStatusResponseSchema', () => {
		it('validates complete transcription', () => {
			const response = {
				status: 'complete',
				result: {
					segments: [{ id: 0, start: 0, end: 1, text: 'Hello' }],
				},
			};
			expect(TranscriptionStatusResponseSchema.safeParse(response).success).toBe(true);
		});

		it('validates processing status', () => {
			const response = {
				status: 'processing',
				progress: 50,
				message: 'Transcribing audio...',
			};
			expect(TranscriptionStatusResponseSchema.safeParse(response).success).toBe(true);
		});

		it('rejects invalid status', () => {
			const response = { status: 'invalid' };
			expect(TranscriptionStatusResponseSchema.safeParse(response).success).toBe(false);
		});
	});

	describe('CsrfTokenResponseSchema', () => {
		it('validates CSRF token response', () => {
			const response = { csrf_token: 'abc123' };
			expect(CsrfTokenResponseSchema.safeParse(response).success).toBe(true);
		});

		it('rejects missing token', () => {
			const response = {};
			expect(CsrfTokenResponseSchema.safeParse(response).success).toBe(false);
		});
	});

	describe('LoginResponseSchema', () => {
		it('validates successful login', () => {
			const response = { message: 'Login successful' };
			expect(LoginResponseSchema.safeParse(response).success).toBe(true);
		});

		it('validates TOTP required response', () => {
			const response = { totp_required: true };
			expect(LoginResponseSchema.safeParse(response).success).toBe(true);
		});

		it('validates email not verified response', () => {
			const response = { email_not_verified: true };
			expect(LoginResponseSchema.safeParse(response).success).toBe(true);
		});
	});

	describe('helper functions', () => {
		describe('safeParse', () => {
			it('returns data on valid input', () => {
				const data = { email: 'test@example.com' };
				const result = safeParse(UserSchema, data);
				expect(result).toEqual(data);
			});

			it('returns null on invalid input', () => {
				const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
				const data = { email: 'invalid' };
				const result = safeParse(UserSchema, data);
				expect(result).toBeNull();
				consoleSpy.mockRestore();
			});
		});

		describe('parse', () => {
			it('returns data on valid input', () => {
				const data = { email: 'test@example.com' };
				const result = parse(UserSchema, data);
				expect(result).toEqual(data);
			});

			it('throws on invalid input', () => {
				const data = { email: 'invalid' };
				expect(() => parse(UserSchema, data)).toThrow();
			});
		});

		describe('isValid', () => {
			it('returns true for valid data', () => {
				const data = { email: 'test@example.com' };
				expect(isValid(UserSchema, data)).toBe(true);
			});

			it('returns false for invalid data', () => {
				const data = { email: 'invalid' };
				expect(isValid(UserSchema, data)).toBe(false);
			});
		});

		describe('validateResponse', () => {
			it('validates response JSON', async () => {
				const mockResponse = {
					json: () => Promise.resolve({ email: 'test@example.com' }),
				} as Response;

				const result = await validateResponse(UserSchema, mockResponse);
				expect(result).toEqual({ email: 'test@example.com' });
			});

			it('returns null on invalid JSON', async () => {
				const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
				const mockResponse = {
					json: () => Promise.resolve({ email: 'invalid' }),
				} as Response;

				const result = await validateResponse(UserSchema, mockResponse);
				expect(result).toBeNull();
				consoleSpy.mockRestore();
			});

			it('returns null on JSON parse error', async () => {
				const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
				const mockResponse = {
					json: () => Promise.reject(new Error('Parse error')),
				} as Response;

				const result = await validateResponse(UserSchema, mockResponse);
				expect(result).toBeNull();
				consoleSpy.mockRestore();
			});
		});

		describe('validateResponseOrThrow', () => {
			it('validates response JSON', async () => {
				const mockResponse = {
					json: () => Promise.resolve({ email: 'test@example.com' }),
				} as Response;

				const result = await validateResponseOrThrow(UserSchema, mockResponse);
				expect(result).toEqual({ email: 'test@example.com' });
			});

			it('throws on invalid JSON', async () => {
				const mockResponse = {
					json: () => Promise.resolve({ email: 'invalid' }),
				} as Response;

				await expect(validateResponseOrThrow(UserSchema, mockResponse)).rejects.toThrow();
			});
		});
	});
});

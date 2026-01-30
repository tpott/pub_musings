/**
 * Zod schemas for API response validation
 *
 * Provides runtime validation for API responses to catch mismatches
 * between frontend expectations and backend responses.
 *
 * Usage:
 * ```typescript
 * import { VideoListResponseSchema, type VideoListResponse } from './api-schemas';
 *
 * const response = await fetch('/api/videos');
 * const data = await response.json();
 * const validated = VideoListResponseSchema.parse(data);
 * // validated is now type-safe with runtime validation
 * ```
 */
import { z } from 'zod';

// ============================================
// User & Auth Schemas
// ============================================

export const UserSchema = z.object({
	email: z.string().email(),
	totp_enabled: z.boolean().optional(),
	email_verified: z.boolean().optional(),
	role: z.string().optional(),
});

export type User = z.infer<typeof UserSchema>;

export const AuthMeResponseSchema = z.object({
	user: UserSchema.optional(),
});

export type AuthMeResponse = z.infer<typeof AuthMeResponseSchema>;

export const SessionSchema = z.object({
	id: z.string(),
	user_agent: z.string().optional(),
	ip_address: z.string().optional(),
	created_at: z.string(),
	expires_at: z.string().optional(),
	is_current: z.boolean().optional(),
});

export type Session = z.infer<typeof SessionSchema>;

export const SessionListResponseSchema = z.object({
	sessions: z.array(SessionSchema),
});

export type SessionListResponse = z.infer<typeof SessionListResponseSchema>;

export const RecoveryCodesResponseSchema = z.object({
	recovery_codes: z.array(z.string()),
});

export type RecoveryCodesResponse = z.infer<typeof RecoveryCodesResponseSchema>;

export const TotpSetupResponseSchema = z.object({
	secret: z.string(),
	secret_display: z.string().optional(),
	uri: z.string().optional(),
	issuer: z.string().optional(),
	qr_code: z.string(),
	recovery_codes: z.array(z.string()).optional(),
});

export type TotpSetupResponse = z.infer<typeof TotpSetupResponseSchema>;

export const TotpVerifyResponseSchema = z.object({
	message: z.string(),
	totp_enabled: z.boolean(),
	recovery_codes: z.array(z.string()).optional(),
});

export type TotpVerifyResponse = z.infer<typeof TotpVerifyResponseSchema>;

// ============================================
// Video Schemas
// ============================================

export const EmbeddedSubtitleTrackSchema = z.object({
	index: z.number(),
	language: z.string().optional(),
	title: z.string().optional(),
	codec: z.string(),
	default: z.boolean().optional(),
	forced: z.boolean().optional(),
	text_based: z.boolean().optional(),
});

export type EmbeddedSubtitleTrack = z.infer<typeof EmbeddedSubtitleTrackSchema>;

export const VideoSchema = z.object({
	id: z.string(),
	filename: z.string(),
	size: z.number(),
	content_type: z.string().optional(),
	created_at: z.string(),
	user_id: z.string().optional(),
	session_id: z.string().optional(),
	transcription_status: z.string().optional(),
	expires_at: z.string().optional().nullable(),
	thumbnail_path: z.string().optional().nullable(),
	embedded_subtitles: z.array(EmbeddedSubtitleTrackSchema).optional(),
});

export type Video = z.infer<typeof VideoSchema>;

export const VideoListResponseSchema = z.object({
	videos: z.array(VideoSchema),
	total_count: z.number().optional(),
	has_more: z.boolean().optional(),
});

export type VideoListResponse = z.infer<typeof VideoListResponseSchema>;

export const LanguageHintSchema = z.object({
	language: z.string(),
	source: z.string(),
	confidence: z.string(),
	language_name: z.string().optional(),
	raw_value: z.string().optional(),
});

export type LanguageHint = z.infer<typeof LanguageHintSchema>;

export const LanguageHintsResponseSchema = z.object({
	hints: z.array(LanguageHintSchema),
	suggested_language: z.string().optional(),
	suggested_confidence: z.string().optional(),
});

export type LanguageHintsResponse = z.infer<typeof LanguageHintsResponseSchema>;

// ============================================
// Transcription Schemas
// ============================================

export const TranscriptionSegmentSchema = z.object({
	id: z.number(),
	start: z.number(),
	end: z.number(),
	text: z.string(),
});

export type TranscriptionSegment = z.infer<typeof TranscriptionSegmentSchema>;

export const TranscriptionResultSchema = z.object({
	language: z.string().optional(),
	duration: z.number().optional(),
	text: z.string().optional(),
	segments: z.array(TranscriptionSegmentSchema),
});

export type TranscriptionResult = z.infer<typeof TranscriptionResultSchema>;

export const TranscriptionStatusResponseSchema = z.object({
	status: z.enum(['pending', 'processing', 'complete', 'error']),
	message: z.string().optional(),
	progress: z.number().optional(),
	result: TranscriptionResultSchema.optional(),
});

export type TranscriptionStatusResponse = z.infer<typeof TranscriptionStatusResponseSchema>;

// ============================================
// Upload Schemas
// ============================================

export const UploadSessionSchema = z.object({
	upload_session_id: z.string(),
	status: z.enum(['in_progress', 'complete']),
	progress: z.number(),
	received_chunks: z.array(z.number()),
	total_chunks: z.number(),
});

export type UploadSession = z.infer<typeof UploadSessionSchema>;

export const UploadInitResponseSchema = z.object({
	upload_session_id: z.string(),
	chunk_size: z.number(),
	total_chunks: z.number(),
});

export type UploadInitResponse = z.infer<typeof UploadInitResponseSchema>;

export const UploadCompleteResponseSchema = z.object({
	upload_id: z.string(),
	filename: z.string(),
	size: z.number(),
	message: z.string().optional(),
	language_hints: LanguageHintsResponseSchema.optional(),
});

export type UploadCompleteResponse = z.infer<typeof UploadCompleteResponseSchema>;

// ============================================
// CSRF Token Schema
// ============================================

export const CsrfTokenResponseSchema = z.object({
	csrf_token: z.string(),
});

export type CsrfTokenResponse = z.infer<typeof CsrfTokenResponseSchema>;

// ============================================
// Login/Auth Response Schemas
// ============================================

export const LoginResponseSchema = z.object({
	message: z.string().optional(),
	totp_required: z.boolean().optional(),
	email_not_verified: z.boolean().optional(),
	error: z.string().optional(),
});

export type LoginResponse = z.infer<typeof LoginResponseSchema>;

export const RegisterResponseSchema = z.object({
	message: z.string().optional(),
	email_verification: z.boolean().optional(),
	error: z.string().optional(),
});

export type RegisterResponse = z.infer<typeof RegisterResponseSchema>;

// ============================================
// Error Response Schema
// ============================================

export const ErrorResponseSchema = z.object({
	error: z.string(),
});

export type ErrorResponse = z.infer<typeof ErrorResponseSchema>;

// ============================================
// Helper Functions
// ============================================

/**
 * Safely parse API response with a Zod schema.
 * Returns the parsed data on success, or null on failure.
 *
 * @param schema - Zod schema to validate against
 * @param data - Data to validate
 * @returns Validated data or null
 */
export function safeParse<T extends z.ZodType>(schema: T, data: unknown): z.infer<T> | null {
	const result = schema.safeParse(data);
	if (result.success) {
		return result.data;
	}
	console.error('API response validation failed:', result.error.format());
	return null;
}

/**
 * Parse API response with a Zod schema.
 * Throws ZodError on validation failure.
 *
 * @param schema - Zod schema to validate against
 * @param data - Data to validate
 * @returns Validated data
 * @throws ZodError if validation fails
 */
export function parse<T extends z.ZodType>(schema: T, data: unknown): z.infer<T> {
	return schema.parse(data);
}

/**
 * Check if data matches a Zod schema.
 *
 * @param schema - Zod schema to check against
 * @param data - Data to check
 * @returns true if data matches schema
 */
export function isValid<T extends z.ZodType>(schema: T, data: unknown): data is z.infer<T> {
	return schema.safeParse(data).success;
}

/**
 * Validate and parse API response JSON with a schema.
 * This is the recommended way to consume API responses.
 *
 * On validation failure, logs the error and returns null.
 * Use this when you want graceful handling of unexpected responses.
 *
 * @param schema - Zod schema to validate against
 * @param response - Fetch Response object
 * @returns Validated data or null if validation fails
 *
 * @example
 * const response = await fetch('/api/videos');
 * const data = await validateResponse(VideoListResponseSchema, response);
 * if (data) {
 *   // data is now typed as VideoListResponse
 *   data.videos.forEach(v => console.log(v.filename));
 * }
 */
export async function validateResponse<T extends z.ZodType>(
	schema: T,
	response: Response
): Promise<z.infer<T> | null> {
	try {
		const json = await response.json();
		return safeParse(schema, json);
	} catch (err) {
		console.error('Failed to parse response JSON:', err);
		return null;
	}
}

/**
 * Validate and parse API response JSON with a schema, throwing on failure.
 * Use this when you want validation failures to propagate as errors.
 *
 * @param schema - Zod schema to validate against
 * @param response - Fetch Response object
 * @returns Validated data
 * @throws Error if JSON parsing fails
 * @throws ZodError if validation fails
 *
 * @example
 * try {
 *   const data = await validateResponseOrThrow(VideoListResponseSchema, response);
 *   // data is typed as VideoListResponse
 * } catch (err) {
 *   // Handle validation error
 * }
 */
export async function validateResponseOrThrow<T extends z.ZodType>(
	schema: T,
	response: Response
): Promise<z.infer<T>> {
	const json = await response.json();
	return parse(schema, json);
}

/**
 * Transcription segment with timing information
 */
export interface TranscriptionSegment {
  id: number;
  start: number; // start time in seconds
  end: number;   // end time in seconds
  text: string;
}

/**
 * Full transcription result from whisper-server
 */
export interface TranscriptionResult {
  language?: string;
  duration?: number;
  text?: string;
  segments: TranscriptionSegment[];
}

/**
 * Status response from transcription polling endpoint
 */
export interface TranscriptionStatusResponse {
  status: 'pending' | 'processing' | 'complete' | 'error';
  message?: string;
  progress?: number;
  result?: TranscriptionResult;
}

/**
 * Alignment API response
 */
export interface AlignmentResponse {
  segments: number;
  mode: 'standard' | 'lyrics';
  script_converted?: boolean;
  target_script?: string;
  conversion_failed_indices?: number[];
  stats?: {
    match_rate?: number;
  };
  error?: string;
}

/**
 * Burn status response from burn polling endpoint
 */
export interface BurnStatusResponse {
  status: 'pending' | 'processing' | 'complete' | 'error';
  message?: string;
  progress?: number;
  duration?: number;
  estimated_remaining_seconds?: number;
}

/**
 * Upload session for chunked uploads
 */
export interface UploadSession {
  upload_session_id: string;
  status: 'in_progress' | 'complete';
  progress: number;
  received_chunks: number[];
  total_chunks: number;
}

/**
 * Language hint from video metadata or filename
 */
export interface LanguageHint {
  source: string;        // "metadata" or "filename"
  language: string;      // ISO 639-1 code
  language_name: string; // Human-readable name
  confidence: string;    // "high", "medium", "low"
  raw_value: string;     // Original value before normalization
}

/**
 * Language hints with suggested language
 */
export interface LanguageHints {
  hints: LanguageHint[];
  suggested_language: string;
  suggested_confidence: string;
}

/**
 * Response from video upload endpoints (both simple and chunked)
 */
export interface UploadResponse {
  status: 'success';
  upload_id: string;
  filename: string;
  size: number;
  message: string;
  language_hints?: LanguageHints;
}

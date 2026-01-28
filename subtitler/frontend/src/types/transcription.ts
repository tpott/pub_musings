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

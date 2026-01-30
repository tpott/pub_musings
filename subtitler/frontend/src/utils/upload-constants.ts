/**
 * Constants and lookup maps for the upload page.
 */

/** Language display names for showing detected language */
export const LANGUAGE_DISPLAY_NAMES: Record<string, string> = {
	'auto': 'Auto-detect',
	'en': 'English',
	'es': 'Spanish',
	'fr': 'French',
	'de': 'German',
	'it': 'Italian',
	'pt': 'Portuguese',
	'ru': 'Russian',
	'zh': 'Chinese',
	'ja': 'Japanese',
	'ko': 'Korean',
	'ar': 'Arabic',
	'hi': 'Hindi',
	'nl': 'Dutch',
	'pl': 'Polish',
	'tr': 'Turkish',
	'vi': 'Vietnamese',
	'th': 'Thai',
	'id': 'Indonesian',
	'uk': 'Ukrainian',
	'sv': 'Swedish',
	'cs': 'Czech',
	'el': 'Greek',
	'he': 'Hebrew',
	'hu': 'Hungarian',
	'no': 'Norwegian',
	'da': 'Danish',
	'fi': 'Finnish',
	'ro': 'Romanian',
	'sk': 'Slovak',
	'bg': 'Bulgarian',
	'hr': 'Croatian',
	'lt': 'Lithuanian',
	'lv': 'Latvian',
	'sl': 'Slovenian',
	'et': 'Estonian',
	'ms': 'Malay',
	'tl': 'Filipino',
	'bn': 'Bengali',
	'ta': 'Tamil',
	'te': 'Telugu',
	'ur': 'Urdu',
	'fa': 'Persian',
	'ml': 'Malayalam',
	'sw': 'Swahili'
};

/** Language to native script mapping for script conversion */
export const LANGUAGE_SCRIPT_MAP: Record<string, string[]> = {
	'hi': ['Devanagari'],
	'ml': ['Malayalam'],
	'ta': ['Tamil'],
	'te': ['Telugu'],
	'kn': ['Kannada'],
	'bn': ['Bengali'],
	'gu': ['Gujarati'],
	'or': ['Oriya'],
	'pa': ['Gurmukhi']
};

/** Language display names for script conversion UI */
export const SCRIPT_LANGUAGE_NAMES: Record<string, string> = {
	'hi': 'Hindi',
	'ml': 'Malayalam',
	'ta': 'Tamil',
	'te': 'Telugu',
	'kn': 'Kannada',
	'bn': 'Bengali',
	'gu': 'Gujarati',
	'or': 'Oriya',
	'pa': 'Punjabi'
};

/** LocalStorage key for preferred transcription language */
export const PREFERRED_LANGUAGE_KEY = 'subtitler:transcription_language';

/** LocalStorage keys for collapsible section states */
export const PASTE_COLLAPSED_KEY = 'subtitler:paste_transcript_collapsed';
export const FULLTEXT_COLLAPSED_KEY = 'subtitler:full_text_collapsed';

/** Chunk size for chunked uploads (50 MB) */
export const CHUNK_SIZE = 50 * 1024 * 1024;

/** Max file size for uploads (500 MB) */
export const MAX_FILE_SIZE = 500 * 1024 * 1024;

/** Upload timeout in milliseconds (5 minutes) */
export const UPLOAD_TIMEOUT_MS = 5 * 60 * 1000;

/** LocalStorage prefix for upload session resumability */
export const UPLOAD_SESSION_PREFIX = 'subtitler:upload_session:';

/** Maximum text length for transcript alignment (100KB, matches backend) */
export const MAX_ALIGN_TEXT_LENGTH = 100 * 1024;

/** Maximum undo/redo history entries */
export const MAX_HISTORY_SIZE = 50;

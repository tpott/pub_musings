// Package language provides language detection for video files.
// It extracts language hints from video metadata and filename patterns.
package language

import (
	"encoding/json"
	"os/exec"
	"regexp"
	"strings"
)

// LanguageHint represents a detected language indicator from a source.
type LanguageHint struct {
	Source       string `json:"source"`        // "metadata", "filename"
	Language     string `json:"language"`      // ISO 639-1 code
	LanguageName string `json:"language_name"` // Human-readable name
	Confidence   string `json:"confidence"`    // "high", "medium", "low"
	RawValue     string `json:"raw_value"`     // Original value before normalization
}

// DetectionResult contains all hints and a suggestion.
type DetectionResult struct {
	Hints               []LanguageHint `json:"hints"`
	SuggestedLanguage   string         `json:"suggested_language"`
	SuggestedConfidence string         `json:"suggested_confidence"`
}

// ISO 639-2 (3-letter) to ISO 639-1 (2-letter) code mapping.
// Whisper uses 2-letter codes, but video metadata often uses 3-letter codes.
var iso639_2to1 = map[string]string{
	"eng": "en", "spa": "es", "fra": "fr", "fre": "fr", "deu": "de", "ger": "de",
	"ita": "it", "por": "pt", "rus": "ru", "zho": "zh", "chi": "zh",
	"jpn": "ja", "kor": "ko", "ara": "ar", "hin": "hi",
	"nld": "nl", "dut": "nl", "pol": "pl", "tur": "tr", "vie": "vi",
	"tha": "th", "ind": "id", "ukr": "uk", "swe": "sv",
	"ces": "cs", "cze": "cs", "dan": "da", "fin": "fi", "ell": "el", "gre": "el",
	"heb": "he", "hun": "hu", "nor": "no", "ron": "ro", "rum": "ro",
	"slk": "sk", "slo": "sk", "cat": "ca", "hrv": "hr", "bul": "bg",
	"lit": "lt", "lav": "lv", "est": "et", "msa": "ms", "may": "ms",
	"tgl": "tl", "fil": "tl", "ben": "bn", "tam": "ta", "tel": "te",
	"mar": "mr", "guj": "gu", "kan": "kn", "mal": "ml", "pan": "pa",
	"und": "", // Undetermined
}

// Language names for display.
var languageNames = map[string]string{
	"en": "English", "es": "Spanish", "fr": "French", "de": "German",
	"it": "Italian", "pt": "Portuguese", "ru": "Russian", "zh": "Chinese",
	"ja": "Japanese", "ko": "Korean", "ar": "Arabic", "hi": "Hindi",
	"nl": "Dutch", "pl": "Polish", "tr": "Turkish", "vi": "Vietnamese",
	"th": "Thai", "id": "Indonesian", "uk": "Ukrainian", "sv": "Swedish",
	"cs": "Czech", "da": "Danish", "fi": "Finnish", "el": "Greek",
	"he": "Hebrew", "hu": "Hungarian", "no": "Norwegian", "ro": "Romanian",
	"sk": "Slovak", "ca": "Catalan", "hr": "Croatian", "bg": "Bulgarian",
	"lt": "Lithuanian", "lv": "Latvian", "et": "Estonian", "ms": "Malay",
	"tl": "Filipino", "bn": "Bengali", "ta": "Tamil", "te": "Telugu",
	"mr": "Marathi", "gu": "Gujarati", "kn": "Kannada", "ml": "Malayalam",
	"pa": "Punjabi",
}

// Language name patterns (case-insensitive).
var languageNameToCode = map[string]string{
	"english": "en", "spanish": "es", "french": "fr", "german": "de",
	"italian": "it", "portuguese": "pt", "russian": "ru", "chinese": "zh",
	"japanese": "ja", "korean": "ko", "arabic": "ar", "hindi": "hi",
	"dutch": "nl", "polish": "pl", "turkish": "tr", "vietnamese": "vi",
	"thai": "th", "indonesian": "id", "ukrainian": "uk", "swedish": "sv",
	"czech": "cs", "danish": "da", "finnish": "fi", "greek": "el",
	"hebrew": "he", "hungarian": "hu", "norwegian": "no", "romanian": "ro",
	"slovak": "sk", "catalan": "ca", "croatian": "hr", "bulgarian": "bg",
	"lithuanian": "lt", "latvian": "lv", "estonian": "et", "malay": "ms",
	"filipino": "tl", "tagalog": "tl", "bengali": "bn", "tamil": "ta",
	"telugu": "te", "marathi": "mr", "gujarati": "gu", "kannada": "kn",
	"malayalam": "ml", "punjabi": "pa",
}

// Confidence levels in priority order.
const (
	ConfidenceHigh   = "high"
	ConfidenceMedium = "medium"
	ConfidenceLow    = "low"
)

// ffprobeAudioOutput represents the JSON output from ffprobe for audio streams.
type ffprobeAudioOutput struct {
	Streams []struct {
		Tags struct {
			Language string `json:"language"`
		} `json:"tags"`
	} `json:"streams"`
}

// ConvertISO639_2to1 converts ISO 639-2 (3-letter) codes to ISO 639-1 (2-letter) codes.
// Returns empty string if code is unknown or "und" (undetermined).
func ConvertISO639_2to1(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	if len(code) == 2 {
		// Already a 2-letter code, validate it
		if _, ok := languageNames[code]; ok {
			return code
		}
		return ""
	}
	if converted, ok := iso639_2to1[code]; ok {
		return converted
	}
	return ""
}

// GetLanguageName returns the human-readable name for a language code.
// Returns empty string if code is unknown.
func GetLanguageName(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	if name, ok := languageNames[code]; ok {
		return name
	}
	return ""
}

// DetectFromMetadata extracts language from video audio track metadata using ffprobe.
// Returns nil if no language metadata found or ffprobe is not available.
func DetectFromMetadata(filePath string) (*LanguageHint, error) {
	probePath, err := exec.LookPath("ffprobe")
	if err != nil {
		// ffprobe not available
		return nil, nil
	}

	// Use ffprobe to get audio stream language tag
	// -v error: only show errors
	// -select_streams a:0: select first audio stream
	// -show_entries stream_tags=language: only show language tag
	// -of json: output as JSON
	cmd := exec.Command(probePath,
		"-v", "error",
		"-select_streams", "a:0",
		"-show_entries", "stream_tags=language",
		"-of", "json",
		filePath,
	)
	output, err := cmd.Output()
	if err != nil {
		return nil, nil // Not a hard error, just no metadata
	}

	// Parse JSON output
	var probeOutput ffprobeAudioOutput
	if err := json.Unmarshal(output, &probeOutput); err != nil {
		return nil, nil
	}

	// Check if we have a language tag
	if len(probeOutput.Streams) == 0 {
		return nil, nil
	}
	rawLang := probeOutput.Streams[0].Tags.Language
	if rawLang == "" || rawLang == "und" {
		return nil, nil
	}

	// Convert to ISO 639-1
	langCode := ConvertISO639_2to1(rawLang)
	if langCode == "" {
		return nil, nil
	}

	return &LanguageHint{
		Source:       "metadata",
		Language:     langCode,
		LanguageName: GetLanguageName(langCode),
		Confidence:   ConfidenceHigh,
		RawValue:     rawLang,
	}, nil
}

// Filename pattern regexes (compiled for efficiency).
var (
	// ISO 639-1 codes before extension: video.en.mp4, video_en.mp4, video-en.mp4
	isoCodeBeforeExtPattern = regexp.MustCompile(`(?i)[._-](en|es|fr|de|it|pt|ru|zh|ja|ko|ar|hi|nl|pl|tr|vi|th|id|uk|sv|cs|da|fi|el|he|hu|no|ro|sk|ca|hr|bg|lt|lv|et|ms|tl|bn|ta|te|mr|gu|kn|ml|pa)\.[a-zA-Z0-9]+$`)

	// ISO 639-2 codes before extension: video.eng.mp4, video_eng.mp4
	iso3CodeBeforeExtPattern = regexp.MustCompile(`(?i)[._-](eng|spa|fra|fre|deu|ger|ita|por|rus|zho|chi|jpn|kor|ara|hin|nld|dut|pol|tur|vie|tha|ind|ukr|swe)\.[a-zA-Z0-9]+$`)

	// Bracketed codes: video[en].mp4, video(es).mp4
	bracketedCodePattern = regexp.MustCompile(`(?i)[\[\(](en|es|fr|de|it|pt|ru|zh|ja|ko|ar|hi|nl|pl|tr|vi|th|id|uk|sv)[\]\)]`)

	// Language names before extension: video_english.mp4, video-spanish.mp4
	langNameBeforeExtPattern = regexp.MustCompile(`(?i)[._-](english|spanish|french|german|italian|portuguese|russian|chinese|japanese|korean|arabic|hindi|dutch|polish|turkish|vietnamese|thai|indonesian|ukrainian|swedish)[._-]?\.`)

	// Note: langNameAnywherePattern was removed because it causes too many false positives
	// (e.g., "english_lesson.mp4" matching "English" when the video is about English grammar)
)

// DetectFromFilename extracts language from filename patterns.
// Returns nil if no language pattern found.
func DetectFromFilename(filename string) (*LanguageHint, error) {
	// Try patterns in order of confidence

	// High confidence: ISO code directly before extension
	if match := isoCodeBeforeExtPattern.FindStringSubmatch(filename); len(match) > 1 {
		code := strings.ToLower(match[1])
		if name := GetLanguageName(code); name != "" {
			return &LanguageHint{
				Source:       "filename",
				Language:     code,
				LanguageName: name,
				Confidence:   ConfidenceHigh,
				RawValue:     match[1],
			}, nil
		}
	}

	// High confidence: ISO 639-2 code before extension
	if match := iso3CodeBeforeExtPattern.FindStringSubmatch(filename); len(match) > 1 {
		raw := match[1]
		code := ConvertISO639_2to1(raw)
		if code != "" {
			return &LanguageHint{
				Source:       "filename",
				Language:     code,
				LanguageName: GetLanguageName(code),
				Confidence:   ConfidenceHigh,
				RawValue:     raw,
			}, nil
		}
	}

	// High confidence: Bracketed codes
	if match := bracketedCodePattern.FindStringSubmatch(filename); len(match) > 1 {
		code := strings.ToLower(match[1])
		if name := GetLanguageName(code); name != "" {
			return &LanguageHint{
				Source:       "filename",
				Language:     code,
				LanguageName: name,
				Confidence:   ConfidenceHigh,
				RawValue:     match[1],
			}, nil
		}
	}

	// Medium confidence: Language name before extension
	if match := langNameBeforeExtPattern.FindStringSubmatch(filename); len(match) > 1 {
		nameLower := strings.ToLower(match[1])
		if code, ok := languageNameToCode[nameLower]; ok {
			return &LanguageHint{
				Source:       "filename",
				Language:     code,
				LanguageName: GetLanguageName(code),
				Confidence:   ConfidenceMedium,
				RawValue:     match[1],
			}, nil
		}
	}

	// Note: We intentionally skip "language anywhere" pattern matching (low confidence)
	// because it causes too many false positives (e.g., "english_lesson.mp4" matching "English"
	// when the video is actually a Spanish lesson about English grammar).

	return nil, nil
}

// Detect runs all detection methods and returns combined results.
// filePath is the path to the video file (for metadata extraction).
// filename is the original filename (for pattern matching).
func Detect(filePath, filename string) *DetectionResult {
	result := &DetectionResult{
		Hints: make([]LanguageHint, 0),
	}

	// Try metadata detection
	if hint, err := DetectFromMetadata(filePath); err == nil && hint != nil {
		result.Hints = append(result.Hints, *hint)
	}

	// Try filename detection
	if hint, err := DetectFromFilename(filename); err == nil && hint != nil {
		// Check if we already have this language from metadata
		duplicate := false
		for _, existing := range result.Hints {
			if existing.Language == hint.Language {
				duplicate = true
				break
			}
		}
		if !duplicate {
			result.Hints = append(result.Hints, *hint)
		}
	}

	// Determine suggestion based on highest confidence
	if len(result.Hints) > 0 {
		// Priority: metadata > filename (metadata is high confidence by definition)
		bestHint := result.Hints[0]
		for _, hint := range result.Hints {
			if confidencePriority(hint.Confidence) > confidencePriority(bestHint.Confidence) {
				bestHint = hint
			}
		}
		result.SuggestedLanguage = bestHint.Language
		result.SuggestedConfidence = bestHint.Confidence
	}

	return result
}

// confidencePriority returns a numeric priority for a confidence level.
func confidencePriority(confidence string) int {
	switch confidence {
	case ConfidenceHigh:
		return 3
	case ConfidenceMedium:
		return 2
	case ConfidenceLow:
		return 1
	default:
		return 0
	}
}

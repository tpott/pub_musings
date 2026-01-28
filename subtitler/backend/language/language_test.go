package language

import (
	"testing"
)

func TestConvertISO639_2to1(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Common 3-letter codes
		{"English 3-letter", "eng", "en"},
		{"Spanish 3-letter", "spa", "es"},
		{"French bibliographic", "fra", "fr"},
		{"French terminology", "fre", "fr"},
		{"German bibliographic", "deu", "de"},
		{"German terminology", "ger", "de"},
		{"Chinese bibliographic", "zho", "zh"},
		{"Chinese terminology", "chi", "zh"},
		{"Japanese", "jpn", "ja"},
		{"Korean", "kor", "ko"},
		{"Arabic", "ara", "ar"},
		{"Hindi", "hin", "hi"},
		{"Dutch bibliographic", "nld", "nl"},
		{"Dutch terminology", "dut", "nl"},
		{"Portuguese", "por", "pt"},
		{"Russian", "rus", "ru"},
		{"Italian", "ita", "it"},

		// Already 2-letter codes (validation)
		{"English 2-letter", "en", "en"},
		{"Spanish 2-letter", "es", "es"},
		{"Japanese 2-letter", "ja", "ja"},

		// Case insensitivity
		{"Uppercase", "ENG", "en"},
		{"Mixed case", "EnG", "en"},

		// Undetermined
		{"Undetermined", "und", ""},

		// Unknown codes
		{"Unknown 3-letter", "xyz", ""},
		{"Unknown 2-letter", "xy", ""},
		{"Empty string", "", ""},
		{"Whitespace", "  eng  ", "en"},

		// Invalid 2-letter that's not in languageNames
		{"Invalid 2-letter", "xx", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertISO639_2to1(tt.input)
			if result != tt.expected {
				t.Errorf("ConvertISO639_2to1(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetLanguageName(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected string
	}{
		{"English", "en", "English"},
		{"Spanish", "es", "Spanish"},
		{"French", "fr", "French"},
		{"German", "de", "German"},
		{"Japanese", "ja", "Japanese"},
		{"Korean", "ko", "Korean"},
		{"Chinese", "zh", "Chinese"},
		{"Arabic", "ar", "Arabic"},
		{"Hindi", "hi", "Hindi"},
		{"Russian", "ru", "Russian"},
		{"Portuguese", "pt", "Portuguese"},
		{"Italian", "it", "Italian"},

		// Case insensitivity
		{"Uppercase", "EN", "English"},
		{"Mixed case", "En", "English"},

		// Unknown codes
		{"Unknown", "xx", ""},
		{"Empty", "", ""},

		// With whitespace
		{"Whitespace", "  en  ", "English"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetLanguageName(tt.code)
			if result != tt.expected {
				t.Errorf("GetLanguageName(%q) = %q, want %q", tt.code, result, tt.expected)
			}
		})
	}
}

func TestDetectFromFilename(t *testing.T) {
	tests := []struct {
		name         string
		filename     string
		expectedLang string
		expectedConf string
		expectedRaw  string
		expectedNil  bool
	}{
		// High confidence: ISO code before extension
		{"ISO code with dot", "video.en.mp4", "en", ConfidenceHigh, "en", false},
		{"ISO code with underscore", "video_es.mp4", "es", ConfidenceHigh, "es", false},
		{"ISO code with dash", "video-fr.mp4", "fr", ConfidenceHigh, "fr", false},
		{"Japanese ISO code", "movie.ja.mkv", "ja", ConfidenceHigh, "ja", false},
		{"Korean ISO code", "drama_ko.avi", "ko", ConfidenceHigh, "ko", false},
		{"Chinese ISO code", "film.zh.mp4", "zh", ConfidenceHigh, "zh", false},
		{"Arabic ISO code", "show.ar.webm", "ar", ConfidenceHigh, "ar", false},
		{"Hindi ISO code", "clip-hi.mp4", "hi", ConfidenceHigh, "hi", false},

		// High confidence: ISO 639-2 codes before extension
		{"3-letter English", "video.eng.mp4", "en", ConfidenceHigh, "eng", false},
		{"3-letter Spanish", "movie_spa.mkv", "es", ConfidenceHigh, "spa", false},
		{"3-letter German", "film.deu.mp4", "de", ConfidenceHigh, "deu", false},
		{"3-letter Japanese", "anime.jpn.mp4", "ja", ConfidenceHigh, "jpn", false},

		// High confidence: Bracketed codes
		{"Square brackets", "video[en].mp4", "en", ConfidenceHigh, "en", false},
		{"Parentheses", "movie(es).mkv", "es", ConfidenceHigh, "es", false},
		{"Square brackets French", "film[fr].avi", "fr", ConfidenceHigh, "fr", false},

		// Medium confidence: Language names before extension
		{"English name before ext", "video_english.mp4", "en", ConfidenceMedium, "english", false},
		{"Spanish name before ext", "movie-spanish.mkv", "es", ConfidenceMedium, "spanish", false},
		{"French name before ext", "film.french.avi", "fr", ConfidenceMedium, "french", false},
		{"Japanese name before ext", "anime_japanese.mp4", "ja", ConfidenceMedium, "japanese", false},
		{"German name before ext", "video_german.mp4", "de", ConfidenceMedium, "german", false},

		// No language hint found (previously "low confidence" but removed to reduce false positives)
		{"English anywhere", "english_lesson_01.mp4", "", "", "", true},
		{"Spanish anywhere", "spanish_tutorial.mkv", "", "", "", true},
		{"French anywhere", "learn_french_basics.avi", "", "", "", true},

		// No language hint found
		{"Plain filename", "my_video.mp4", "", "", "", true},
		{"Numbers only", "video_123.mp4", "", "", "", true},
		{"Random suffix", "video_abc.mp4", "", "", "", true},
		{"Date suffix", "video_2024.mp4", "", "", "", true},

		// Edge cases
		{"Case insensitive", "VIDEO.EN.MP4", "en", ConfidenceHigh, "EN", false},
		{"Mixed case", "Video_English.Mp4", "en", ConfidenceMedium, "English", false},
		{"Multiple dots", "video.s01e01.en.mp4", "en", ConfidenceHigh, "en", false},
		{"Complex filename", "Movie.2024.1080p.BluRay.eng.mp4", "en", ConfidenceHigh, "eng", false},

		// Potential false positives should NOT match (to avoid confusion)
		{"English lesson", "english_lesson.mp4", "", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hint, err := DetectFromFilename(tt.filename)
			if err != nil {
				t.Fatalf("DetectFromFilename(%q) error = %v", tt.filename, err)
			}
			if tt.expectedNil {
				if hint != nil {
					t.Errorf("DetectFromFilename(%q) = %+v, want nil", tt.filename, hint)
				}
				return
			}
			if hint == nil {
				t.Fatalf("DetectFromFilename(%q) = nil, want hint", tt.filename)
			}
			if hint.Language != tt.expectedLang {
				t.Errorf("DetectFromFilename(%q).Language = %q, want %q", tt.filename, hint.Language, tt.expectedLang)
			}
			if hint.Confidence != tt.expectedConf {
				t.Errorf("DetectFromFilename(%q).Confidence = %q, want %q", tt.filename, hint.Confidence, tt.expectedConf)
			}
			if hint.RawValue != tt.expectedRaw {
				t.Errorf("DetectFromFilename(%q).RawValue = %q, want %q", tt.filename, hint.RawValue, tt.expectedRaw)
			}
			if hint.Source != "filename" {
				t.Errorf("DetectFromFilename(%q).Source = %q, want \"filename\"", tt.filename, hint.Source)
			}
			if hint.LanguageName == "" && hint.Language != "" {
				t.Errorf("DetectFromFilename(%q).LanguageName is empty for language %q", tt.filename, hint.Language)
			}
		})
	}
}

func TestDetect(t *testing.T) {
	// Test without file path (filename only)
	t.Run("Filename only detection", func(t *testing.T) {
		result := Detect("", "video.en.mp4")
		if result == nil {
			t.Fatal("Detect returned nil")
		}
		if len(result.Hints) == 0 {
			t.Fatal("Detect returned no hints")
		}
		if result.SuggestedLanguage != "en" {
			t.Errorf("SuggestedLanguage = %q, want \"en\"", result.SuggestedLanguage)
		}
	})

	t.Run("No hints", func(t *testing.T) {
		result := Detect("", "my_video.mp4")
		if result == nil {
			t.Fatal("Detect returned nil")
		}
		if len(result.Hints) != 0 {
			t.Errorf("Expected no hints, got %d", len(result.Hints))
		}
		if result.SuggestedLanguage != "" {
			t.Errorf("SuggestedLanguage = %q, want empty", result.SuggestedLanguage)
		}
	})

	t.Run("Empty inputs", func(t *testing.T) {
		result := Detect("", "")
		if result == nil {
			t.Fatal("Detect returned nil")
		}
		if len(result.Hints) != 0 {
			t.Errorf("Expected no hints, got %d", len(result.Hints))
		}
	})
}

func TestConfidencePriority(t *testing.T) {
	tests := []struct {
		name       string
		confidence string
		expected   int
	}{
		{"High", ConfidenceHigh, 3},
		{"Medium", ConfidenceMedium, 2},
		{"Low", ConfidenceLow, 1},
		{"Unknown", "unknown", 0},
		{"Empty", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := confidencePriority(tt.confidence)
			if result != tt.expected {
				t.Errorf("confidencePriority(%q) = %d, want %d", tt.confidence, result, tt.expected)
			}
		})
	}
}

func TestLanguageNameMapping(t *testing.T) {
	// Verify all language codes in languageNameToCode have entries in languageNames
	for name, code := range languageNameToCode {
		if _, ok := languageNames[code]; !ok {
			t.Errorf("languageNameToCode[%q] = %q, but %q not in languageNames", name, code, code)
		}
	}

	// Verify ISO 639-2 to 639-1 mappings result in valid 2-letter codes
	for iso2, iso1 := range iso639_2to1 {
		if iso1 == "" {
			continue // "und" maps to empty
		}
		if _, ok := languageNames[iso1]; !ok {
			t.Errorf("iso639_2to1[%q] = %q, but %q not in languageNames", iso2, iso1, iso1)
		}
	}
}

func TestFilenamePatternEdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		filename     string
		shouldMatch  bool
		expectedLang string
	}{
		// Should NOT match - partial codes
		{"Partial code en", "enhance.mp4", false, ""},
		{"Partial code de", "defile.mp4", false, ""},
		{"Partial code fr", "friend.mp4", false, ""},

		// Should match - proper delimiters
		{"Dot delimiter", "video.en.mp4", true, "en"},
		{"Underscore delimiter", "video_en.mp4", true, "en"},
		{"Dash delimiter", "video-en.mp4", true, "en"},

		// Should NOT match - code in middle without proper delimiter
		{"Code in middle", "interview.mp4", false, ""},
		{"Code at start", "english.mp4", false, ""}, // No longer matches (removed low confidence)

		// Special characters
		{"Spaces in filename", "my video en.mp4", false, ""},
		// Unicode after code - should NOT match since code is not directly before extension
		{"Unicode filename suffix", "video_en_日本語.mp4", false, ""},
		// Unicode before code - SHOULD match since code is directly before extension
		{"Unicode filename prefix", "日本語_video.en.mp4", true, "en"},

		// Multiple potential matches (first/highest confidence wins)
		{"Multiple codes", "video.en.es.mp4", true, "es"}, // es is closer to extension
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hint, _ := DetectFromFilename(tt.filename)
			if tt.shouldMatch {
				if hint == nil {
					t.Errorf("DetectFromFilename(%q) = nil, expected match", tt.filename)
				} else if hint.Language != tt.expectedLang {
					t.Errorf("DetectFromFilename(%q).Language = %q, want %q", tt.filename, hint.Language, tt.expectedLang)
				}
			} else {
				if hint != nil {
					t.Errorf("DetectFromFilename(%q) = %+v, expected no match", tt.filename, hint)
				}
			}
		})
	}
}

func TestMoreISO639Codes(t *testing.T) {
	// Test additional language codes
	additionalCodes := map[string]string{
		"ces": "cs", // Czech bibliographic
		"cze": "cs", // Czech terminology
		"dan": "da", // Danish
		"fin": "fi", // Finnish
		"ell": "el", // Greek bibliographic
		"gre": "el", // Greek terminology
		"heb": "he", // Hebrew
		"hun": "hu", // Hungarian
		"nor": "no", // Norwegian
		"ron": "ro", // Romanian bibliographic
		"rum": "ro", // Romanian terminology
		"slk": "sk", // Slovak bibliographic
		"slo": "sk", // Slovak terminology
		"cat": "ca", // Catalan
		"hrv": "hr", // Croatian
		"bul": "bg", // Bulgarian
		"lit": "lt", // Lithuanian
		"lav": "lv", // Latvian
		"est": "et", // Estonian
		"msa": "ms", // Malay bibliographic
		"may": "ms", // Malay terminology
		"tgl": "tl", // Tagalog
		"fil": "tl", // Filipino
		"ben": "bn", // Bengali
		"tam": "ta", // Tamil
		"tel": "te", // Telugu
		"mar": "mr", // Marathi
		"guj": "gu", // Gujarati
		"kan": "kn", // Kannada
		"mal": "ml", // Malayalam
		"pan": "pa", // Punjabi
	}

	for iso2, expectedIso1 := range additionalCodes {
		t.Run(iso2, func(t *testing.T) {
			result := ConvertISO639_2to1(iso2)
			if result != expectedIso1 {
				t.Errorf("ConvertISO639_2to1(%q) = %q, want %q", iso2, result, expectedIso1)
			}
		})
	}
}

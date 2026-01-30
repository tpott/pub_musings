package script

import (
	"testing"
)

func TestDetectScript(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected Script
	}{
		{"Latin text", "Hello World", ScriptLatin},
		{"Devanagari text", "नमस्ते दुनिया", ScriptDevanagari},
		{"Bengali text", "নমস্কার", ScriptBengali},
		{"Tamil text", "வணக்கம்", ScriptTamil},
		{"Telugu text", "నమస్కారం", ScriptTelugu},
		{"Kannada text", "ನಮಸ್ಕಾರ", ScriptKannada},
		{"Malayalam text", "നമസ്കാരം", ScriptMalayalam},
		{"Gujarati text", "નમસ્તે", ScriptGujarati},
		{"Arabic text", "مرحبا", ScriptArabic},
		{"Cyrillic text", "Привет", ScriptCyrillic},
		{"CJK text", "你好", ScriptCJK},
		{"Mixed with punctuation", "Hello! 123", ScriptLatin},
		{"Empty string", "", ScriptUnknown},
		{"Only spaces", "   ", ScriptUnknown},
		{"Only punctuation", "...!!!", ScriptUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectScript(tt.text)
			if result != tt.expected {
				t.Errorf("DetectScript(%q) = %v, want %v", tt.text, result, tt.expected)
			}
		})
	}
}

func TestIsLanguageSupported(t *testing.T) {
	tests := []struct {
		lang     string
		expected bool
	}{
		{"hi", true},
		{"ml", true},
		{"ta", true},
		{"te", true},
		{"kn", true},
		{"bn", true},
		{"gu", true},
		{"or", true},
		{"pa", true},
		{"en", false},
		{"fr", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			result := IsLanguageSupported(tt.lang)
			if result != tt.expected {
				t.Errorf("IsLanguageSupported(%q) = %v, want %v", tt.lang, result, tt.expected)
			}
		})
	}
}

func TestIsScriptSupported(t *testing.T) {
	tests := []struct {
		script   Script
		expected bool
	}{
		{ScriptDevanagari, true},
		{ScriptBengali, true},
		{ScriptTamil, true},
		{ScriptTelugu, true},
		{ScriptKannada, true},
		{ScriptMalayalam, true},
		{ScriptGujarati, true},
		{ScriptGurmukhi, true},
		{ScriptOriya, true},
		{ScriptLatin, false},
		{ScriptArabic, false},
		{ScriptCyrillic, false},
		{ScriptUnknown, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.script), func(t *testing.T) {
			result := IsScriptSupported(tt.script)
			if result != tt.expected {
				t.Errorf("IsScriptSupported(%v) = %v, want %v", tt.script, result, tt.expected)
			}
		})
	}
}

func TestConverter_Convert_Hindi(t *testing.T) {
	c := NewConverter()

	// Note: This is a basic transliterator. It does not handle consonant clusters
	// with virama (halant) which would be needed for perfect Hindi output.
	// For production use, consider GoVarnam or Aksharamukha.
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Simple word", "namaste", "नमसते"}, // Basic output without halant
		{"Word with long vowel", "namaskar", "नमसकर"},
		{"Two words", "namaste duniya", "नमसते दुनिय"},
		{"Capital letters", "NAMASTE", "नमसते"},
		{"Mixed case", "Namaste", "नमसते"},
		{"Aspirated consonant kh", "khaana", "खान"},
		{"Aspirated consonant gh", "ghar", "घर"},
		{"Aspirated consonant ch", "chai", "चै"},
		{"Aspirated consonant bh", "bhai", "भै"},
		{"Aspirated consonant ph", "phool", "फूल"},
		{"Long ee vowel", "pee", "पी"},
		{"Long oo vowel", "pooja", "पूज"},
		{"Ai vowel", "main", "मैन"},
		{"Au vowel", "kaun", "कौन"},
		{"Word ending consonant", "kitab", "कितब"}, // Simple transliterator doesn't infer long 'aa'
		{"Common word tum", "tum", "तुम"},
		{"Common word pyar", "pyar", "पयर"}, // Without halant
		{"Song lyric tujhe", "tujhe", "तुझे"},
		{"Empty string", "", ""},
		{"Single letter", "a", "अ"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := c.Convert(tt.input, LangHindi, ScriptDevanagari)
			if err != nil {
				t.Errorf("Convert() error = %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("Convert(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestConverter_Convert_UnsupportedLanguage(t *testing.T) {
	c := NewConverter()

	// For unsupported languages, should return original text
	input := "Hello World"
	result, err := c.Convert(input, "en", ScriptLatin)
	if err != nil {
		t.Errorf("Convert() error = %v", err)
		return
	}
	if result != input {
		t.Errorf("Convert() for unsupported language = %q, want %q", result, input)
	}
}

func TestDetectLanguageFromRomanized(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected string
	}{
		{"Hindi text with common words", "main tumse pyar karta hoon", LangHindi},
		{"Hindi text with yeh", "yeh kya hai", LangHindi},
		{"Hindi text with kahan", "tum kahan ho", LangHindi},
		{"Malayalam text", "njan evide pokum enna paranjal", LangMalayalam},
		{"Tamil text", "naan eppadi avan kittu poren", LangTamil},
		{"Short text defaults to Hindi", "hello world", LangHindi},
		{"Empty text", "", LangHindi},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectLanguageFromRomanized(tt.text)
			if result != tt.expected {
				t.Errorf("DetectLanguageFromRomanized(%q) = %q, want %q", tt.text, result, tt.expected)
			}
		})
	}
}

func TestClassifyRune(t *testing.T) {
	tests := []struct {
		name     string
		r        rune
		expected Script
	}{
		{"Latin A", 'A', ScriptLatin},
		{"Latin z", 'z', ScriptLatin},
		{"Devanagari", 'अ', ScriptDevanagari},
		{"Bengali", 'অ', ScriptBengali},
		{"Tamil", 'அ', ScriptTamil},
		{"Telugu", 'అ', ScriptTelugu},
		{"Kannada", 'ಅ', ScriptKannada},
		{"Malayalam", 'അ', ScriptMalayalam},
		{"Gujarati", 'અ', ScriptGujarati},
		{"Gurmukhi", 'ਅ', ScriptGurmukhi},
		{"Oriya", 'ଅ', ScriptOriya},
		{"Arabic", 'ا', ScriptArabic},
		{"Cyrillic", 'А', ScriptCyrillic},
		{"CJK", '中', ScriptCJK},
		{"Japanese Hiragana", 'あ', ScriptCJK},
		{"Thai", 'ก', ScriptThai},
		{"Unknown", '☺', ScriptUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyRune(tt.r)
			if result != tt.expected {
				t.Errorf("classifyRune(%q) = %v, want %v", tt.r, result, tt.expected)
			}
		})
	}
}

func TestSupportedLanguages(t *testing.T) {
	langs := SupportedLanguages()
	if len(langs) != 9 {
		t.Errorf("SupportedLanguages() returned %d languages, want 9", len(langs))
	}

	// Check that Hindi is in the list
	found := false
	for _, l := range langs {
		if l == LangHindi {
			found = true
			break
		}
	}
	if !found {
		t.Error("Hindi not found in SupportedLanguages()")
	}
}

func TestSupportedScripts(t *testing.T) {
	scripts := SupportedScripts()
	if len(scripts) != 9 {
		t.Errorf("SupportedScripts() returned %d scripts, want 9", len(scripts))
	}

	// Check that Devanagari is in the list
	found := false
	for _, s := range scripts {
		if s == ScriptDevanagari {
			found = true
			break
		}
	}
	if !found {
		t.Error("Devanagari not found in SupportedScripts()")
	}
}

func TestConverter_isConsonant(t *testing.T) {
	c := NewConverter()

	consonants := []string{"k", "kh", "g", "gh", "ch", "j", "t", "d", "n", "p", "b", "m", "y", "r", "l", "v", "s", "h", "sh"}
	for _, cons := range consonants {
		if !c.isConsonant(cons) {
			t.Errorf("isConsonant(%q) = false, want true", cons)
		}
	}

	vowels := []string{"a", "aa", "i", "ee", "u", "oo", "e", "o", "ai", "au"}
	for _, vow := range vowels {
		if c.isConsonant(vow) {
			t.Errorf("isConsonant(%q) = true, want false", vow)
		}
	}
}

func TestConverter_getVowelMatra(t *testing.T) {
	c := NewConverter()

	tests := []struct {
		vowel    string
		expected string
		ok       bool
	}{
		{"aa", "ा", true},
		{"a", "", true},
		{"i", "ि", true},
		{"ee", "ी", true},
		{"u", "ु", true},
		{"oo", "ू", true},
		{"e", "े", true},
		{"o", "ो", true},
		{"ai", "ै", true},
		{"au", "ौ", true},
		{"x", "", false},
		{"k", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.vowel, func(t *testing.T) {
			matra, ok := c.getVowelMatra(tt.vowel)
			if ok != tt.ok {
				t.Errorf("getVowelMatra(%q) ok = %v, want %v", tt.vowel, ok, tt.ok)
			}
			if matra != tt.expected {
				t.Errorf("getVowelMatra(%q) = %q, want %q", tt.vowel, matra, tt.expected)
			}
		})
	}
}

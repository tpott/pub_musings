// Package script provides script detection and transliteration functionality.
package script

import (
	"strings"
	"unicode"
)

// Script represents a writing system
type Script string

const (
	ScriptLatin      Script = "Latin"
	ScriptDevanagari Script = "Devanagari"
	ScriptBengali    Script = "Bengali"
	ScriptTamil      Script = "Tamil"
	ScriptTelugu     Script = "Telugu"
	ScriptKannada    Script = "Kannada"
	ScriptMalayalam  Script = "Malayalam"
	ScriptGujarati   Script = "Gujarati"
	ScriptGurmukhi   Script = "Gurmukhi"
	ScriptOriya      Script = "Oriya"
	ScriptArabic     Script = "Arabic"
	ScriptCyrillic   Script = "Cyrillic"
	ScriptCJK        Script = "CJK"
	ScriptThai       Script = "Thai"
	ScriptUnknown    Script = "Unknown"
)

// Language codes for Indic languages
const (
	LangHindi     = "hi"
	LangMalayalam = "ml"
	LangTamil     = "ta"
	LangTelugu    = "te"
	LangKannada   = "kn"
	LangBengali   = "bn"
	LangGujarati  = "gu"
	LangOriya     = "or"
	LangPunjabi   = "pa"
)

// SupportedLanguages returns all supported language codes
func SupportedLanguages() []string {
	return []string{LangHindi, LangMalayalam, LangTamil, LangTelugu, LangKannada, LangBengali, LangGujarati, LangOriya, LangPunjabi}
}

// SupportedScripts returns all supported target scripts
func SupportedScripts() []Script {
	return []Script{ScriptDevanagari, ScriptBengali, ScriptTamil, ScriptTelugu, ScriptKannada, ScriptMalayalam, ScriptGujarati, ScriptGurmukhi, ScriptOriya}
}

// IsLanguageSupported checks if a language code is supported
func IsLanguageSupported(lang string) bool {
	for _, l := range SupportedLanguages() {
		if l == lang {
			return true
		}
	}
	return false
}

// IsScriptSupported checks if a target script is supported
func IsScriptSupported(script Script) bool {
	for _, s := range SupportedScripts() {
		if s == script {
			return true
		}
	}
	return false
}

// DetectScript analyzes text and returns the predominant script
func DetectScript(text string) Script {
	counts := make(map[Script]int)

	for _, r := range text {
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsDigit(r) {
			continue
		}

		script := classifyRune(r)
		counts[script]++
	}

	maxCount := 0
	result := ScriptUnknown

	for script, count := range counts {
		if count > maxCount {
			maxCount = count
			result = script
		}
	}

	return result
}

// classifyRune determines which script a Unicode rune belongs to
func classifyRune(r rune) Script {
	switch {
	case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z':
		return ScriptLatin
	case r >= 0x0900 && r <= 0x097F:
		return ScriptDevanagari
	case r >= 0x0980 && r <= 0x09FF:
		return ScriptBengali
	case r >= 0x0B80 && r <= 0x0BFF:
		return ScriptTamil
	case r >= 0x0C00 && r <= 0x0C7F:
		return ScriptTelugu
	case r >= 0x0C80 && r <= 0x0CFF:
		return ScriptKannada
	case r >= 0x0D00 && r <= 0x0D7F:
		return ScriptMalayalam
	case r >= 0x0A80 && r <= 0x0AFF:
		return ScriptGujarati
	case r >= 0x0A00 && r <= 0x0A7F:
		return ScriptGurmukhi
	case r >= 0x0B00 && r <= 0x0B7F:
		return ScriptOriya
	case r >= 0x0600 && r <= 0x06FF, r >= 0x0750 && r <= 0x077F:
		return ScriptArabic
	case r >= 0x0400 && r <= 0x04FF:
		return ScriptCyrillic
	case r >= 0x4E00 && r <= 0x9FFF, r >= 0x3040 && r <= 0x30FF:
		return ScriptCJK
	case r >= 0x0E00 && r <= 0x0E7F:
		return ScriptThai
	default:
		return ScriptUnknown
	}
}

// Converter handles transliteration between scripts
type Converter struct {
	// Mapping tables for each language
	hindiMap map[string]string
}

// NewConverter creates a new script converter
func NewConverter() *Converter {
	c := &Converter{}
	c.initHindiMap()
	return c
}

// initHindiMap initializes the romanized Hindi to Devanagari mapping
// Based on common ITRANS/Google transliteration conventions
func (c *Converter) initHindiMap() {
	c.hindiMap = map[string]string{
		// Vowels (independent)
		"aa": "आ",
		"AA": "आ",
		"a":  "अ",
		"A":  "अ",
		"ee": "ई",
		"EE": "ई",
		"ii": "ई",
		"II": "ई",
		"i":  "इ",
		"I":  "इ",
		"oo": "ऊ",
		"OO": "ऊ",
		"uu": "ऊ",
		"UU": "ऊ",
		"u":  "उ",
		"U":  "उ",
		"ai": "ऐ",
		"AI": "ऐ",
		"au": "औ",
		"AU": "औ",
		"e":  "ए",
		"E":  "ए",
		"o":  "ओ",
		"O":  "ओ",
		"ri": "ऋ",
		"RI": "ऋ",
		"R":  "ऋ",

		// Vowel signs (matras) - used after consonants
		"_aa": "ा",
		"_a":  "",
		"_ee": "ी",
		"_ii": "ी",
		"_i":  "ि",
		"_oo": "ू",
		"_uu": "ू",
		"_u":  "ु",
		"_ai": "ै",
		"_au": "ौ",
		"_e":  "े",
		"_o":  "ो",
		"_ri": "ृ",

		// Consonants
		"kh":  "ख",
		"KH":  "ख",
		"Kh":  "ख",
		"k":   "क",
		"K":   "क",
		"gh":  "घ",
		"GH":  "घ",
		"Gh":  "घ",
		"g":   "ग",
		"G":   "ग",
		"ng":  "ङ",
		"NG":  "ङ",
		"chh": "छ",
		"CHH": "छ",
		"ch":  "च",
		"CH":  "च",
		"Ch":  "च",
		"jh":  "झ",
		"JH":  "झ",
		"Jh":  "झ",
		"j":   "ज",
		"J":   "ज",
		"ny":  "ञ",
		"NY":  "ञ",
		"th":  "थ",
		"TH":  "ठ",
		"Th":  "थ",
		"t":   "त",
		"T":   "ट",
		"dh":  "ध",
		"DH":  "ढ",
		"Dh":  "ध",
		"d":   "द",
		"D":   "ड",
		"n":   "न",
		"N":   "ण",
		"ph":  "फ",
		"PH":  "फ",
		"Ph":  "फ",
		"p":   "प",
		"P":   "प",
		"bh":  "भ",
		"BH":  "भ",
		"Bh":  "भ",
		"b":   "ब",
		"B":   "ब",
		"m":   "म",
		"M":   "म",
		"y":   "य",
		"Y":   "य",
		"r":   "र",
		"l":   "ल",
		"L":   "ळ",
		"v":   "व",
		"V":   "व",
		"w":   "व",
		"W":   "व",
		"sh":  "श",
		"SH":  "ष",
		"Sh":  "श",
		"s":   "स",
		"S":   "स",
		"h":   "ह",
		"H":   "ह",
		"x":   "क्ष",
		"X":   "क्ष",
		"tr":  "त्र",
		"TR":  "त्र",
		"gn":  "ज्ञ",
		"GN":  "ज्ञ",
		"gy":  "ज्ञ",
		"GY":  "ज्ञ",

		// Nukta consonants (for borrowed sounds)
		"z": "ज़",
		"Z": "ज़",
		"f": "फ़",
		"F": "फ़",
		"q": "क़",
		"Q": "क़",

		// Special characters
		".":  "।",
		"..": "॥",
		".m": "ं", // anusvara (nasalization)
		".n": "ं", // alternative for anusvara
		".h": "ः", // visarga
		"~":  "ँ", // chandrabindu
		"'":  "ऽ", // avagraha

		// Virama (halant) - suppresses inherent vowel
		"\\": "्",
	}
}

// Convert transliterates text from Latin script to the target script
func (c *Converter) Convert(text string, lang string, targetScript Script) (string, error) {
	// Currently only Hindi/Devanagari is implemented
	if lang != LangHindi && targetScript != ScriptDevanagari {
		// For unsupported languages, return original text
		return text, nil
	}

	return c.latinToDevanagari(text), nil
}

// latinToDevanagari converts romanized Hindi to Devanagari script
func (c *Converter) latinToDevanagari(text string) string {
	var result strings.Builder
	text = strings.TrimSpace(text)
	words := strings.Fields(text)

	for i, word := range words {
		if i > 0 {
			result.WriteRune(' ')
		}
		result.WriteString(c.convertWord(word))
	}

	return result.String()
}

// convertWord converts a single romanized word to Devanagari
func (c *Converter) convertWord(word string) string {
	var result strings.Builder
	runes := []rune(strings.ToLower(word))
	n := len(runes)
	i := 0
	afterConsonant := false

	for i < n {
		matched := false

		// Try matching longest sequences first (up to 3 chars)
		for length := 3; length >= 1; length-- {
			if i+length > n {
				continue
			}

			substr := string(runes[i : i+length])

			// Check if this is a vowel after a consonant (use matra)
			if afterConsonant {
				if matra, ok := c.getVowelMatra(substr); ok {
					result.WriteString(matra)
					i += length
					matched = true
					afterConsonant = false
					break
				}
			}

			// Check for consonant or independent vowel
			if devanagari, ok := c.hindiMap[substr]; ok {
				// Check if it's a consonant
				if c.isConsonant(substr) {
					result.WriteString(devanagari)
					afterConsonant = true
				} else {
					// It's a vowel - add inherent 'a' halant if after consonant
					if afterConsonant {
						// Add matra instead of independent vowel
						if matra, ok := c.getVowelMatra(substr); ok {
							result.WriteString(matra)
						} else {
							result.WriteString(devanagari)
						}
						afterConsonant = false
					} else {
						result.WriteString(devanagari)
					}
				}
				i += length
				matched = true
				break
			}
		}

		if !matched {
			// No match found, keep original character
			// If we were after a consonant and hit a non-vowel, add implicit 'a' sound
			// (In Hindi, consonants have inherent 'a' unless followed by a matra)
			result.WriteRune(runes[i])
			afterConsonant = false
			i++
		}
	}

	return result.String()
}

// isConsonant checks if the romanized string represents a consonant
func (c *Converter) isConsonant(s string) bool {
	consonants := map[string]bool{
		"k": true, "kh": true, "g": true, "gh": true, "ng": true,
		"ch": true, "chh": true, "j": true, "jh": true, "ny": true,
		"t": true, "th": true, "d": true, "dh": true, "n": true,
		"p": true, "ph": true, "b": true, "bh": true, "m": true,
		"y": true, "r": true, "l": true, "v": true, "w": true,
		"sh": true, "s": true, "h": true,
		"x": true, "tr": true, "gn": true, "gy": true,
		"z": true, "f": true, "q": true,
	}
	return consonants[strings.ToLower(s)]
}

// getVowelMatra returns the matra (vowel sign) for a vowel
func (c *Converter) getVowelMatra(vowel string) (string, bool) {
	matras := map[string]string{
		"aa": "ा",
		"a":  "", // inherent, no visible matra
		"ee": "ी",
		"ii": "ी",
		"i":  "ि",
		"oo": "ू",
		"uu": "ू",
		"u":  "ु",
		"ai": "ै",
		"au": "ौ",
		"e":  "े",
		"o":  "ो",
		"ri": "ृ",
	}
	matra, ok := matras[strings.ToLower(vowel)]
	return matra, ok
}

// DetectLanguageFromRomanized attempts to guess the language from romanized text
// This is a simple heuristic based on common words
func DetectLanguageFromRomanized(text string) string {
	text = strings.ToLower(text)

	// Hindi indicators
	hindiWords := []string{"hai", "hain", "ka", "ki", "ke", "ko", "mein", "main", "yeh", "woh", "aur", "se", "par", "mujhe", "tujhe", "humein", "unhe", "kya", "kaise", "kyun", "kab", "kahan"}
	hindiCount := 0
	for _, word := range hindiWords {
		if strings.Contains(text, word) {
			hindiCount++
		}
	}

	// Malayalam indicators
	malayalamWords := []string{"enna", "njan", "ningal", "avan", "aval", "athu", "ithu", "enthu", "engane", "evide"}
	malayalamCount := 0
	for _, word := range malayalamWords {
		if strings.Contains(text, word) {
			malayalamCount++
		}
	}

	// Tamil indicators
	tamilWords := []string{"naan", "nee", "avan", "aval", "athu", "ithu", "enna", "eppadi", "enge", "yaaru"}
	tamilCount := 0
	for _, word := range tamilWords {
		if strings.Contains(text, word) {
			tamilCount++
		}
	}

	// Return best match
	if hindiCount > malayalamCount && hindiCount > tamilCount && hindiCount >= 2 {
		return LangHindi
	}
	if malayalamCount > hindiCount && malayalamCount > tamilCount && malayalamCount >= 2 {
		return LangMalayalam
	}
	if tamilCount > hindiCount && tamilCount > malayalamCount && tamilCount >= 2 {
		return LangTamil
	}

	// Default to Hindi as most common
	return LangHindi
}

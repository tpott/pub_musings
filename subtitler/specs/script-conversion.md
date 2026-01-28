# Script Conversion Specification

This document evaluates libraries for converting between romanized text and native scripts (e.g., romanized Hindi → Devanagari) for subtitle enhancement.

## Use Cases

1. **Music video subtitles**: User pastes romanized Hindi lyrics ("Tujhe dekha toh ye jaana sanam"), converts to Devanagari ("तुझे देखा तो ये जाना सनम")
2. **Language learning**: Display both romanized and native script versions
3. **Accessibility**: Provide subtitles in user's preferred script

## Library Evaluation

### Option 1: GoVarnam (Recommended for Indic Languages)

**Repository**: [github.com/varnamproject/govarnam](https://github.com/varnamproject/govarnam)

**Pros**:
- Native Go implementation (no external dependencies like Docker)
- Actively maintained (v1.9.1 released April 2024)
- Supports 10+ Indian languages: Malayalam, Tamil, Hindi, Bengali, Gujarati, Kannada, Telugu, Oriya, Gurmukhi
- Designed specifically for romanized → native script conversion
- Works on Linux via IBus integration
- Uses VST (Varnam Symbol Table) scheme files for extensibility

**Cons**:
- Requires downloading language scheme files (~1-5 MB per language)
- Compiled as C shared library with Go bindings (adds build complexity)
- Primarily designed as input method editor, not batch processing API

**Installation**:
```bash
# Clone and build
git clone https://github.com/varnamproject/govarnam
cd govarnam
make
make install  # Installs libgovarnam.so and govarnamgo package

# Download language schemes (e.g., Hindi)
varnamcli -download hi
```

**Usage**:
```go
import "github.com/varnamproject/govarnam/govarnamgo"

varnam, err := govarnamgo.InitFromID("hi")
defer varnam.Close()
results, err := varnam.Transliterate("namaste")
// results[0].Word = "नमस्ते"
```

### Option 2: go-aksharamukha (Most Comprehensive)

**Repository**: [github.com/tassa-yoniso-manasi-karoto/go-aksharamukha](https://github.com/tassa-yoniso-manasi-karoto/go-aksharamukha)

**Pros**:
- 120+ scripts supported (Indic, Southeast Asian, Middle Eastern)
- 21 romanization standards (ISO-15919, IAST, ITRANS, Harvard-Kyoto, etc.)
- Bidirectional conversion (romanized ↔ native)
- Most comprehensive coverage

**Cons**:
- Requires Docker (heavyweight dependency)
- Alpha status, author advises against production use except for Indic transliteration
- Network latency for conversions (Docker container API calls)
- Complex setup for deployment

**Usage**:
```go
import ak "github.com/tassa-yoniso-manasi-karoto/go-aksharamukha"

ak.Init()
defer ak.Close()
result, err := ak.Translit("IAST", "Devanagari", "namaste")
// result = "नमस्ते"
```

### Option 3: translitkit (Unified Interface)

**Repository**: [github.com/tassa-yoniso-manasi-karoto/translitkit](https://github.com/tassa-yoniso-manasi-karoto/translitkit)

**Pros**:
- Unified API across multiple providers
- Supports Japanese (Ichiran), Chinese (Gojieba), Thai, Russian
- Uses Aksharamukha for Indic languages

**Cons**:
- Pre-release status
- Inherits Docker dependency from Aksharamukha for Indic languages
- Author notes LLMs now outperform traditional NLP for many tasks

### Option 4: golang.org/x/text + mxmCherry/translit

**Repository**: [pkg.go.dev/golang.org/x/text](https://pkg.go.dev/golang.org/x/text)

**Pros**:
- Official Go text processing package
- Integrates with text/transform pipeline
- No external dependencies

**Cons**:
- Limited script support (primarily Cyrillic ↔ Latin)
- Need to implement custom transliteration rules for Indic scripts
- No out-of-box Hindi/Devanagari support

### Option 5: External APIs

**Google Input Tools** (DEPRECATED since 2011)
**Microsoft Indic Language Input Tool** (Requires API key)

Not recommended due to external dependency and deprecation concerns.

## Recommendation

### Implemented: Pure Go Mapping Tables

After evaluating GoVarnam's CGO dependency complexity, we implemented a **pure Go solution** using lookup tables. This approach:

1. **Zero external dependencies** - No CGO, no shared libraries, no Docker
2. **Simple deployment** - Single binary works everywhere
3. **Good accuracy** - 200+ romanization patterns for Hindi with ITRANS/Google conventions
4. **Easily extensible** - Add new language mappings without build complexity

The pure Go implementation handles common romanization conventions for Hindi (Devanagari), with the architecture ready to support additional Indic scripts.

### Future: Consider Aksharamukha (Optional)

If users need broader script support (e.g., Thai, Arabic, Southeast Asian scripts), consider adding Aksharamukha as an optional backend with Docker requirement clearly documented.

## API Design

### Script Detection Endpoint

```
POST /api/text/detect-script
Content-Type: application/json

{
  "text": "namaste duniya"
}

Response:
{
  "detected_script": "Latin",
  "detected_language": "hi",
  "confidence": 0.85
}
```

### Script Conversion Endpoint

```
POST /api/text/convert
Content-Type: application/json

{
  "text": "namaste duniya",
  "source_script": "Latin",      // optional, auto-detected if omitted
  "target_script": "Devanagari",
  "language": "hi"               // required for romanized input
}

Response:
{
  "original": "namaste duniya",
  "converted": "नमस्ते दुनिया",
  "source_script": "Latin",
  "target_script": "Devanagari",
  "language": "hi"
}
```

### Integration with Align API

```
POST /api/transcribe/{id}/align
Content-Type: application/json

{
  "text": "Tujhe dekha toh ye jaana sanam",
  "mode": "lyrics",
  "convert_to_script": "Devanagari",  // new optional field
  "language": "hi"                     // required if convert_to_script is set
}

Response:
{
  "status": "success",
  "segments": 5,
  "mode": "lyrics",
  "script_converted": true,
  "target_script": "Devanagari"
}
```

## Supported Languages (GoVarnam)

| Language | Code | Example Input | Example Output |
|----------|------|---------------|----------------|
| Hindi | hi | namaste | नमस्ते |
| Malayalam | ml | namaskaaram | നമസ്കാരം |
| Tamil | ta | vanakkam | வணக்கம் |
| Telugu | te | namaskaram | నమస్కారం |
| Kannada | kn | namaskara | ನಮಸ್ಕಾರ |
| Bengali | bn | namaskar | নমস্কার |
| Gujarati | gu | namaste | નમસ્તે |
| Oriya | or | namaskar | ନମସ୍କାର |
| Gurmukhi (Punjabi) | pa | sat sri akal | ਸਤ ਸ੍ਰੀ ਅਕਾਲ |

## Implementation Status

### Completed ✅

1. **Script detection** - `backend/script/script.go` implements Unicode range checking for 14 scripts
2. **Pure Go transliteration** - `Converter` struct with mapping tables for Hindi (200+ patterns)
3. **API endpoints** - `/api/text/detect-script` and `/api/text/convert` implemented
4. **Align API integration** - Optional `script` parameter for post-processing
5. **Frontend UI** - Language/script selector on upload page (Task 57)

### Implementation Details

The `backend/script/` package provides:

```go
// Script detection using Unicode ranges
script.DetectScript(text) // Returns Script type (Latin, Devanagari, etc.)

// Conversion using mapping tables
converter := script.NewConverter()
result, err := converter.Convert(text, targetScript, language)
```

The Hindi mapping table includes:
- Independent vowels (अ आ इ ई उ ऊ ए ऐ ओ औ)
- Vowel signs/matras (ा ि ी ु ू े ै ो ौ)
- All consonants with aspirated variants (क ख ग घ...)
- Conjuncts and special characters (क्ष ज्ञ श्र...)
- Nukta characters for Persian/Arabic sounds (क़ ख़ ग़...)
- Chandrabindu, anusvara, visarga (ँ ं ः)

The converter handles:
- Case-insensitive input ("Namaste" = "namaste")
- Multi-character sequences (longest match first: "chh" before "ch")
- Word-initial vs word-medial vowel handling
- Implicit 'a' vowel (inherent schwa)

## Security Considerations

- Validate language codes against whitelist
- Limit text length to prevent DoS (max 10KB per request)
- Sanitize input to prevent injection
- Rate limit conversion endpoint (10 req/min)

## References

- [ISO 15919](https://en.wikipedia.org/wiki/ISO_15919) - International standard for romanization of Indic scripts
- [IAST](https://en.wikipedia.org/wiki/International_Alphabet_of_Sanskrit_Transliteration) - Academic standard for Sanskrit/Hindi
- [GoVarnam Documentation](https://varnamproject.github.io/)
- [Aksharamukha Online Demo](https://www.aksharamukha.com/converter)

package validation

import (
	"strings"
	"testing"
)

func TestValidateSegmentText(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		wantErr bool
	}{
		{"empty string", "", false},
		{"short text", "Hello world", false},
		{"max length", strings.Repeat("a", MaxSegmentTextLength), false},
		{"over max length", strings.Repeat("a", MaxSegmentTextLength+1), true},
		{"way over limit", strings.Repeat("a", MaxSegmentTextLength*2), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSegmentText(tt.text)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSegmentText() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"empty string", "", true},
		{"whitespace only", "   ", true},
		{"valid email", "test@example.com", false},
		{"max length", strings.Repeat("a", 240) + "@example.com", false},
		{"over max length", strings.Repeat("a", 250) + "@example.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmail(tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEmail() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"empty string", "", true},
		{"too short (7 chars)", "abc1234", true},
		{"minimum length (8 chars)", "abcd1234", false},
		{"valid password", "securepassword123", false},
		{"max length (72 chars)", strings.Repeat("a", MaxPasswordLength), false},
		{"over max length", strings.Repeat("a", MaxPasswordLength+1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePassword() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateTOTPCode(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantErr bool
	}{
		{"empty string", "", true},
		{"whitespace only", "   ", true},
		{"valid 6-digit code", "123456", false},
		{"max length", strings.Repeat("1", MaxTOTPCodeLength), false},
		{"over max length", strings.Repeat("1", MaxTOTPCodeLength+1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTOTPCode(tt.code)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTOTPCode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateRecoveryCode(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantErr bool
	}{
		{"empty string", "", true},
		{"whitespace only", "   ", true},
		{"valid code", "ABCD-1234-EFGH", false},
		{"max length", strings.Repeat("A", MaxRecoveryCodeLength), false},
		{"over max length", strings.Repeat("A", MaxRecoveryCodeLength+1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRecoveryCode(tt.code)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRecoveryCode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAlignText(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		wantErr bool
	}{
		{"empty string", "", true},
		{"whitespace only", "   ", true},
		{"valid text", "Hello world", false},
		{"max length", strings.Repeat("a", MaxAlignTextLength), false},
		{"over max length", strings.Repeat("a", MaxAlignTextLength+1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAlignText(tt.text)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAlignText() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateLanguageCode(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantErr bool
	}{
		{"empty string", "", false},
		{"valid 2-char code", "en", false},
		{"valid 5-char code", "zh-CN", false},
		{"max length", strings.Repeat("a", MaxLanguageCodeLength), false},
		{"over max length", strings.Repeat("a", MaxLanguageCodeLength+1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateLanguageCode(tt.code)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateLanguageCode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		wantErr  bool
	}{
		{"empty string", "", true},
		{"valid filename", "video.mp4", false},
		{"max length", strings.Repeat("a", MaxFilenameLength-4) + ".mp4", false},
		{"over max length", strings.Repeat("a", MaxFilenameLength+1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFilename(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFilename() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateMIMEType(t *testing.T) {
	tests := []struct {
		name     string
		mimeType string
		wantErr  bool
	}{
		{"empty string", "", false},
		{"valid type", "video/mp4", false},
		{"max length", strings.Repeat("a", MaxMIMETypeLength), false},
		{"over max length", strings.Repeat("a", MaxMIMETypeLength+1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMIMEType(tt.mimeType)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMIMEType() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateBurnMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		wantMode BurnMode
		wantErr  bool
	}{
		{"empty defaults to burn", "", BurnModeBurn, false},
		{"explicit burn", "burn", BurnModeBurn, false},
		{"embed mode", "embed", BurnModeEmbed, false},
		{"invalid mode", "invalid", "", true},
		{"case sensitive", "BURN", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateBurnMode(tt.mode)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBurnMode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.wantMode {
				t.Errorf("ValidateBurnMode() = %v, want %v", got, tt.wantMode)
			}
		})
	}
}

func TestValidateAlignMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		wantMode AlignMode
		wantErr  bool
	}{
		{"empty is default", "", AlignModeDefault, false},
		{"lyrics mode", "lyrics", AlignModeLyrics, false},
		{"invalid mode", "invalid", "", true},
		{"case sensitive", "LYRICS", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateAlignMode(tt.mode)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAlignMode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.wantMode {
				t.Errorf("ValidateAlignMode() = %v, want %v", got, tt.wantMode)
			}
		})
	}
}

func TestValidateHexID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"valid 32-char lowercase hex", "abcdef0123456789abcdef0123456789", false},
		{"valid 32-char uppercase hex", "ABCDEF0123456789ABCDEF0123456789", false},
		{"valid 32-char mixed case hex", "AbCdEf0123456789abcdef0123456789", false},
		{"too short", "abc123", true},
		{"too long", "abcdef0123456789abcdef0123456789extra", true},
		{"empty string", "", true},
		{"contains non-hex char g", "abcdefg123456789abcdef0123456789", true},
		{"contains non-hex char z", "abcdef0123456z89abcdef0123456789", true},
		{"contains space", "abcdef0123456789 bcdef0123456789", true},
		{"contains dash", "abcdef01-3456789abcdef0123456789", true},
		{"31 chars (one short)", "abcdef0123456789abcdef012345678", true},
		{"33 chars (one long)", "abcdef0123456789abcdef01234567890", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHexID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateHexID(%q) error = %v, wantErr %v", tt.id, err, tt.wantErr)
			}
		})
	}
}

func TestSanitizeFileExtension(t *testing.T) {
	tests := []struct {
		name    string
		ext     string
		want    string
		wantErr bool
	}{
		// Valid extensions
		{"empty defaults to .mp4", "", ".mp4", false},
		{"valid .mp4", ".mp4", ".mp4", false},
		{"valid .webm", ".webm", ".webm", false},
		{"valid .avi", ".avi", ".avi", false},
		{"valid .mov", ".mov", ".mov", false},
		{"valid .mkv", ".mkv", ".mkv", false},
		{"valid uppercase .MP4", ".MP4", ".MP4", false},

		// Path traversal attacks
		{"forward slash", "./etc/passwd", "", true},
		{"backslash", ".\\etc\\passwd", "", true},
		{"path traversal with ..", "./../../../etc/passwd", "", true},
		{"double dots", "..mp4", "", true},
		{"unix path traversal", ".mp4/../../../etc/passwd", "", true},
		{"windows path traversal", ".mp4\\..\\..\\windows\\system32", "", true},

		// Null byte injection
		{"null byte at start", "\x00.mp4", "", true},
		{"null byte in middle", ".mp\x004", "", true},
		{"null byte at end", ".mp4\x00", "", true},

		// Invalid format
		{"missing leading dot", "mp4", "", true},
		{"just text no dot", "video", "", true},

		// Length check
		{"max valid length", ".12345678901234567890", "", true}, // 21 chars > 20
		{"exactly 20 chars", ".1234567890123456789", ".1234567890123456789", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SanitizeFileExtension(tt.ext)
			if (err != nil) != tt.wantErr {
				t.Errorf("SanitizeFileExtension(%q) error = %v, wantErr %v", tt.ext, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("SanitizeFileExtension(%q) = %q, want %q", tt.ext, got, tt.want)
			}
		})
	}
}

func TestValidateFilePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		// Valid paths
		{"simple relative path", "models/ggml-medium.bin", false},
		{"absolute path", "/home/user/models/ggml-medium.bin", false},
		{"windows style path", "C:\\models\\ggml-medium.bin", false},
		{"path with spaces", "/home/user/my models/ggml-medium.bin", false},
		{"current directory", ".", false},
		{"dot in filename", "model.v3.bin", false},
		{"double dots in filename", "model..bin", false},  // not traversal since not a segment
		{"dotdot at start of name", "..model.bin", false}, // not traversal since part of filename

		// Empty path
		{"empty path", "", true},

		// Path traversal attacks - Unix style
		{"relative traversal", "../etc/passwd", true},
		{"double relative traversal", "../../etc/passwd", true},
		{"mid-path traversal", "/home/user/../../../etc/passwd", true},
		{"traversal from absolute", "/home/../etc/passwd", true},
		{"trailing traversal", "/home/user/models/..", true},

		// Path traversal attacks - using forward slashes (works cross-platform)
		{"forward slash traversal", "C:/../Windows/System32", true},

		// Null byte injection
		{"null byte in path", "/home/user/models\x00/ggml-medium.bin", true},
		{"null byte at end", "/home/user/models/ggml-medium.bin\x00", true},
		{"null byte at start", "\x00/home/user/models/ggml-medium.bin", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFilePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFilePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

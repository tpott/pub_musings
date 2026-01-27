package audio

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestCheckFFmpegAvailable(t *testing.T) {
	// This test checks the actual system - will pass if ffmpeg is installed
	err := CheckFFmpegAvailable()
	if err != nil {
		t.Logf("ffmpeg not available (expected in some test environments): %v", err)
		// Don't fail - just log. This test documents the behavior.
	}
}

func TestCheckFFmpegAvailable_NotInPath(t *testing.T) {
	// Save original PATH
	origPath := os.Getenv("PATH")
	defer os.Setenv("PATH", origPath)

	// Set PATH to empty to simulate missing ffmpeg
	os.Setenv("PATH", "/nonexistent-path-for-testing")

	err := CheckFFmpegAvailable()
	if err == nil {
		t.Error("Expected error when ffmpeg is not in PATH")
	}
	if !strings.Contains(err.Error(), "ffmpeg not found") {
		t.Errorf("Expected 'ffmpeg not found' error, got: %v", err)
	}
}

func TestFFmpegExtractor_ExtractAudio_NotFound(t *testing.T) {
	// Save original PATH
	origPath := os.Getenv("PATH")
	defer os.Setenv("PATH", origPath)

	// Set PATH to empty to simulate missing ffmpeg
	os.Setenv("PATH", "/nonexistent-path-for-testing")

	extractor := NewFFmpegExtractor()
	err := extractor.ExtractAudio("/tmp/test.mp4", "/tmp/test.wav")

	if err == nil {
		t.Error("Expected error when ffmpeg is not in PATH")
	}
	// The error should indicate ffmpeg couldn't be found/executed
	if !strings.Contains(err.Error(), "ffmpeg error") {
		t.Errorf("Expected 'ffmpeg error', got: %v", err)
	}
}

func TestFFmpegExtractor_ExtractAudio_InvalidInput(t *testing.T) {
	// Skip if ffmpeg is not available
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available, skipping integration test")
	}

	extractor := NewFFmpegExtractor()
	err := extractor.ExtractAudio("/nonexistent/video.mp4", "/tmp/output.wav")

	if err == nil {
		t.Error("Expected error for nonexistent input file")
	}
	if !strings.Contains(err.Error(), "ffmpeg error") {
		t.Errorf("Expected 'ffmpeg error', got: %v", err)
	}
}

func TestMockExtractor(t *testing.T) {
	mock := NewMockExtractor()

	// Test successful extraction
	err := mock.ExtractAudio("/video.mp4", "/audio.wav")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if mock.CallCount != 1 {
		t.Errorf("Expected CallCount=1, got %d", mock.CallCount)
	}
	if mock.LastVideoPath != "/video.mp4" {
		t.Errorf("Expected LastVideoPath='/video.mp4', got '%s'", mock.LastVideoPath)
	}
	if mock.LastAudioPath != "/audio.wav" {
		t.Errorf("Expected LastAudioPath='/audio.wav', got '%s'", mock.LastAudioPath)
	}

	// Test failed extraction with default error
	mock.ShouldFail = true
	err = mock.ExtractAudio("/video2.mp4", "/audio2.wav")
	if err == nil {
		t.Error("Expected error when ShouldFail is true")
	}
	if !strings.Contains(err.Error(), "mock extraction failed") {
		t.Errorf("Expected 'mock extraction failed' error, got: %v", err)
	}
	if mock.CallCount != 2 {
		t.Errorf("Expected CallCount=2, got %d", mock.CallCount)
	}

	// Test failed extraction with custom error
	customErr := errors.New("custom ffmpeg error: codec not supported")
	mock.FailError = customErr
	err = mock.ExtractAudio("/video3.mp4", "/audio3.wav")
	if err != customErr {
		t.Errorf("Expected custom error, got: %v", err)
	}
}

func TestExtractorInterface(t *testing.T) {
	// Verify both implementations satisfy the Extractor interface
	var _ Extractor = (*FFmpegExtractor)(nil)
	var _ Extractor = (*MockExtractor)(nil)
}

func TestValidateVideoFile_NonexistentFile(t *testing.T) {
	// Skip if ffprobe is not available
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not available, skipping validation test")
	}

	err := ValidateVideoFile("/nonexistent/path/video.mp4")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "invalid video file") {
		t.Errorf("Expected 'invalid video file' error, got: %v", err)
	}
}

func TestValidateVideoFile_TextFile(t *testing.T) {
	// Skip if ffprobe is not available
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not available, skipping validation test")
	}

	// Create a temporary text file that pretends to be a video
	tmpFile, err := os.CreateTemp("", "fake-video-*.mp4")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write some text content (not a valid video)
	_, err = tmpFile.WriteString("This is not a video file, just some text content.")
	if err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Validate should fail because this is not a valid video
	err = ValidateVideoFile(tmpFile.Name())
	if err == nil {
		t.Error("Expected error for non-video file")
	}
	if !strings.Contains(err.Error(), "invalid video file") && !strings.Contains(err.Error(), "no video stream") {
		t.Errorf("Expected 'invalid video file' or 'no video stream' error, got: %v", err)
	}
}

func TestValidateVideoFile_FfprobeNotAvailable(t *testing.T) {
	// Save original PATH
	origPath := os.Getenv("PATH")
	defer os.Setenv("PATH", origPath)

	// Set PATH to empty to simulate missing ffprobe
	os.Setenv("PATH", "/nonexistent-path-for-testing")

	// When ffprobe is not available, validation should pass (fallback to allow)
	err := ValidateVideoFile("/any/path.mp4")
	if err != nil {
		t.Errorf("Expected no error when ffprobe not available (fallback to allow), got: %v", err)
	}
}

func TestGetVideoDuration_NonexistentFile(t *testing.T) {
	// Skip if ffprobe is not available
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not available, skipping duration test")
	}

	_, err := GetVideoDuration("/nonexistent/path/video.mp4")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestGetVideoDuration_FfprobeNotAvailable(t *testing.T) {
	// Save original PATH
	origPath := os.Getenv("PATH")
	defer os.Setenv("PATH", origPath)

	// Set PATH to empty to simulate missing ffprobe
	os.Setenv("PATH", "/nonexistent-path-for-testing")

	_, err := GetVideoDuration("/any/path.mp4")
	if err == nil {
		t.Error("Expected error when ffprobe is not available")
	}
	if !strings.Contains(err.Error(), "ffprobe not found") {
		t.Errorf("Expected 'ffprobe not found' error, got: %v", err)
	}
}

func TestGenerateThumbnail_NonexistentFile(t *testing.T) {
	// Skip if ffmpeg is not available
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available, skipping thumbnail test")
	}

	tmpDir := os.TempDir()
	outputPath := tmpDir + "/test_thumb.jpg"
	defer os.Remove(outputPath)

	err := GenerateThumbnail("/nonexistent/path/video.mp4", outputPath)
	if err == nil {
		t.Error("Expected error for nonexistent input file")
	}
	if !strings.Contains(err.Error(), "ffmpeg thumbnail generation failed") {
		t.Errorf("Expected 'ffmpeg thumbnail generation failed' error, got: %v", err)
	}
}

func TestGenerateThumbnail_FFmpegNotAvailable(t *testing.T) {
	// Save original PATH
	origPath := os.Getenv("PATH")
	defer os.Setenv("PATH", origPath)

	// Set PATH to empty to simulate missing ffmpeg
	os.Setenv("PATH", "/nonexistent-path-for-testing")

	err := GenerateThumbnail("/any/video.mp4", "/tmp/thumb.jpg")
	if err == nil {
		t.Error("Expected error when ffmpeg is not available")
	}
}

// Magic Bytes Validation Tests

func TestValidateMagicBytes_MP4(t *testing.T) {
	// MP4 files have "ftyp" at offset 4
	// Typical MP4 header: 00 00 00 20 66 74 79 70 (size + "ftyp")
	mp4Header := []byte{
		0x00, 0x00, 0x00, 0x20, // size (32 bytes)
		0x66, 0x74, 0x79, 0x70, // "ftyp"
		0x69, 0x73, 0x6F, 0x6D, // "isom" (brand)
	}
	reader := bytes.NewReader(mp4Header)
	err := ValidateMagicBytes(reader)
	if err != nil {
		t.Errorf("Expected valid MP4 to pass, got: %v", err)
	}
}

func TestValidateMagicBytes_MOV(t *testing.T) {
	// MOV files also use ftyp box
	movHeader := []byte{
		0x00, 0x00, 0x00, 0x14, // size
		0x66, 0x74, 0x79, 0x70, // "ftyp"
		0x71, 0x74, 0x20, 0x20, // "qt  " (QuickTime brand)
	}
	reader := bytes.NewReader(movHeader)
	err := ValidateMagicBytes(reader)
	if err != nil {
		t.Errorf("Expected valid MOV to pass, got: %v", err)
	}
}

func TestValidateMagicBytes_WebM(t *testing.T) {
	// WebM uses EBML header: 1A 45 DF A3
	webmHeader := []byte{
		0x1A, 0x45, 0xDF, 0xA3, // EBML header
		0x01, 0x00, 0x00, 0x00, // some additional data
		0x00, 0x00, 0x00, 0x00,
	}
	reader := bytes.NewReader(webmHeader)
	err := ValidateMagicBytes(reader)
	if err != nil {
		t.Errorf("Expected valid WebM to pass, got: %v", err)
	}
}

func TestValidateMagicBytes_MKV(t *testing.T) {
	// MKV also uses EBML header (same as WebM)
	mkvHeader := []byte{
		0x1A, 0x45, 0xDF, 0xA3, // EBML header
		0x93, 0x42, 0x82, 0x88, // some MKV specific data
		0x6D, 0x61, 0x74, 0x72,
	}
	reader := bytes.NewReader(mkvHeader)
	err := ValidateMagicBytes(reader)
	if err != nil {
		t.Errorf("Expected valid MKV to pass, got: %v", err)
	}
}

func TestValidateMagicBytes_AVI(t *testing.T) {
	// AVI: RIFF....AVI
	aviHeader := []byte{
		0x52, 0x49, 0x46, 0x46, // "RIFF"
		0x00, 0x00, 0x00, 0x00, // file size (placeholder)
		0x41, 0x56, 0x49, 0x20, // "AVI "
	}
	reader := bytes.NewReader(aviHeader)
	err := ValidateMagicBytes(reader)
	if err != nil {
		t.Errorf("Expected valid AVI to pass, got: %v", err)
	}
}

func TestValidateMagicBytes_OGV(t *testing.T) {
	// OGV uses OggS header
	ogvHeader := []byte{
		0x4F, 0x67, 0x67, 0x53, // "OggS"
		0x00, 0x02, 0x00, 0x00, // some additional Ogg data
		0x00, 0x00, 0x00, 0x00,
	}
	reader := bytes.NewReader(ogvHeader)
	err := ValidateMagicBytes(reader)
	if err != nil {
		t.Errorf("Expected valid OGV to pass, got: %v", err)
	}
}

func TestValidateMagicBytes_MPEG_ProgramStream(t *testing.T) {
	// MPEG Program Stream: 00 00 01 BA
	mpegHeader := []byte{
		0x00, 0x00, 0x01, 0xBA, // MPEG PS header
		0x21, 0x00, 0x01, 0x00, // some MPEG data
		0x00, 0x00, 0x00, 0x00,
	}
	reader := bytes.NewReader(mpegHeader)
	err := ValidateMagicBytes(reader)
	if err != nil {
		t.Errorf("Expected valid MPEG PS to pass, got: %v", err)
	}
}

func TestValidateMagicBytes_MPEG_SequenceHeader(t *testing.T) {
	// MPEG video sequence header: 00 00 01 B3
	mpegHeader := []byte{
		0x00, 0x00, 0x01, 0xB3, // MPEG sequence header
		0x00, 0x00, 0x00, 0x00, // additional data
		0x00, 0x00, 0x00, 0x00,
	}
	reader := bytes.NewReader(mpegHeader)
	err := ValidateMagicBytes(reader)
	if err != nil {
		t.Errorf("Expected valid MPEG to pass, got: %v", err)
	}
}

func TestValidateMagicBytes_InvalidFile(t *testing.T) {
	// Random bytes - not a valid video format
	invalidHeader := []byte{
		0x89, 0x50, 0x4E, 0x47, // PNG header (not video!)
		0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x00,
	}
	reader := bytes.NewReader(invalidHeader)
	err := ValidateMagicBytes(reader)
	if err == nil {
		t.Error("Expected error for invalid file format")
	}
	if !errors.Is(err, ErrInvalidMagicBytes) {
		t.Errorf("Expected ErrInvalidMagicBytes, got: %v", err)
	}
}

func TestValidateMagicBytes_TextFile(t *testing.T) {
	// Plain text content
	textContent := []byte("This is just a text file pretending to be video.mp4")
	reader := bytes.NewReader(textContent)
	err := ValidateMagicBytes(reader)
	if err == nil {
		t.Error("Expected error for text file")
	}
	if !errors.Is(err, ErrInvalidMagicBytes) {
		t.Errorf("Expected ErrInvalidMagicBytes, got: %v", err)
	}
}

func TestValidateMagicBytes_TooShort(t *testing.T) {
	// File too short to contain valid magic bytes
	shortData := []byte{0x00, 0x00, 0x00}
	reader := bytes.NewReader(shortData)
	err := ValidateMagicBytes(reader)
	if err == nil {
		t.Error("Expected error for file too short")
	}
}

func TestValidateMagicBytes_EmptyFile(t *testing.T) {
	reader := bytes.NewReader([]byte{})
	err := ValidateMagicBytes(reader)
	if err == nil {
		t.Error("Expected error for empty file")
	}
}

func TestValidateMagicBytes_AVI_Invalid_NotAVI(t *testing.T) {
	// RIFF header but not AVI (could be WAV audio)
	wavHeader := []byte{
		0x52, 0x49, 0x46, 0x46, // "RIFF"
		0x00, 0x00, 0x00, 0x00, // file size
		0x57, 0x41, 0x56, 0x45, // "WAVE" (not "AVI ")
	}
	reader := bytes.NewReader(wavHeader)
	err := ValidateMagicBytes(reader)
	if err == nil {
		t.Error("Expected error for RIFF/WAVE file (not AVI)")
	}
	if !errors.Is(err, ErrInvalidMagicBytes) {
		t.Errorf("Expected ErrInvalidMagicBytes, got: %v", err)
	}
}

func TestValidateMagicBytesFromFile_ValidMP4(t *testing.T) {
	// Create a temp file with MP4 magic bytes
	tmpFile, err := os.CreateTemp("", "test-video-*.mp4")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	mp4Header := []byte{
		0x00, 0x00, 0x00, 0x20,
		0x66, 0x74, 0x79, 0x70,
		0x69, 0x73, 0x6F, 0x6D,
	}
	if _, err := tmpFile.Write(mp4Header); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	err = ValidateMagicBytesFromFile(tmpFile.Name())
	if err != nil {
		t.Errorf("Expected valid MP4 file to pass, got: %v", err)
	}
}

func TestValidateMagicBytesFromFile_InvalidFile(t *testing.T) {
	// Create a temp file with non-video content
	tmpFile, err := os.CreateTemp("", "test-notavideo-*.mp4")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString("This is not a video file"); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	err = ValidateMagicBytesFromFile(tmpFile.Name())
	if err == nil {
		t.Error("Expected error for non-video file")
	}
	if !errors.Is(err, ErrInvalidMagicBytes) {
		t.Errorf("Expected ErrInvalidMagicBytes, got: %v", err)
	}
}

func TestValidateMagicBytesFromFile_NonexistentFile(t *testing.T) {
	err := ValidateMagicBytesFromFile("/nonexistent/path/to/video.mp4")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
	if errors.Is(err, ErrInvalidMagicBytes) {
		t.Error("Expected file open error, not ErrInvalidMagicBytes")
	}
}

// Subtitle Track Detection Tests

func TestGetSubtitleTracks_FfprobeNotAvailable(t *testing.T) {
	// Save original PATH
	origPath := os.Getenv("PATH")
	defer os.Setenv("PATH", origPath)

	// Set PATH to empty to simulate missing ffprobe
	os.Setenv("PATH", "/nonexistent-path-for-testing")

	// When ffprobe is not available, should return empty list without error
	tracks, err := GetSubtitleTracks("/any/path.mp4")
	if err != nil {
		t.Errorf("Expected no error when ffprobe not available, got: %v", err)
	}
	if len(tracks) != 0 {
		t.Errorf("Expected empty tracks slice, got %d tracks", len(tracks))
	}
}

func TestGetSubtitleTracks_NonexistentFile(t *testing.T) {
	// Skip if ffprobe is not available
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not available, skipping test")
	}

	_, err := GetSubtitleTracks("/nonexistent/path/video.mp4")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "ffprobe failed") {
		t.Errorf("Expected 'ffprobe failed' error, got: %v", err)
	}
}

func TestTextBasedCodecs(t *testing.T) {
	// Test that text-based codecs are correctly identified
	textBased := []string{"subrip", "ass", "ssa", "mov_text", "webvtt", "text"}
	for _, codec := range textBased {
		if !textBasedCodecs[codec] {
			t.Errorf("Codec %s should be text-based", codec)
		}
	}

	// Test that image-based codecs are correctly identified
	imageBased := []string{"hdmv_pgs_subtitle", "dvd_subtitle", "dvb_subtitle"}
	for _, codec := range imageBased {
		if textBasedCodecs[codec] {
			t.Errorf("Codec %s should NOT be text-based", codec)
		}
	}
}

func TestSubtitleTrackStruct(t *testing.T) {
	track := SubtitleTrack{
		Index:     2,
		Language:  "eng",
		Title:     "English",
		Codec:     "subrip",
		Default:   true,
		Forced:    false,
		TextBased: true,
	}

	if track.Index != 2 {
		t.Errorf("Expected Index=2, got %d", track.Index)
	}
	if track.Language != "eng" {
		t.Errorf("Expected Language='eng', got '%s'", track.Language)
	}
	if track.Codec != "subrip" {
		t.Errorf("Expected Codec='subrip', got '%s'", track.Codec)
	}
	if !track.Default {
		t.Error("Expected Default=true")
	}
	if track.Forced {
		t.Error("Expected Forced=false")
	}
	if !track.TextBased {
		t.Error("Expected TextBased=true")
	}
}

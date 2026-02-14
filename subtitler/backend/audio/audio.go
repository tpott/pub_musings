// Package audio provides audio extraction capabilities for video processing.
package audio

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Extractor defines the interface for extracting audio from video files.
type Extractor interface {
	// ExtractAudio extracts audio from a video file and saves it as WAV.
	// videoPath: path to the input video file
	// audioPath: path where the output WAV file will be saved
	ExtractAudio(videoPath, audioPath string) error

	// ExtractAudioSegment extracts a time range of audio from a video file as WAV.
	// startSec and endSec define the time range in seconds.
	ExtractAudioSegment(videoPath, audioPath string, startSec, endSec float64) error
}

// FFmpegExtractor implements Extractor using ffmpeg.
type FFmpegExtractor struct{}

// NewFFmpegExtractor creates a new FFmpegExtractor.
func NewFFmpegExtractor() *FFmpegExtractor {
	return &FFmpegExtractor{}
}

// ExtractAudio uses ffmpeg to extract audio from video as WAV.
// The output is 16kHz mono PCM, which is the format expected by whisper.
func (f *FFmpegExtractor) ExtractAudio(videoPath, audioPath string) error {
	cmd := exec.Command("ffmpeg",
		"-i", videoPath,
		"-vn",                  // no video
		"-acodec", "pcm_s16le", // WAV format
		"-ar", "16000", // 16kHz sample rate (whisper expects this)
		"-ac", "1", // mono
		"-y", // overwrite output
		audioPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg error: %v, output: %s", err, string(output))
	}
	return nil
}

// ExtractAudioSegment uses ffmpeg to extract a time range of audio from video as WAV.
// The output is 16kHz mono PCM, which is the format expected by whisper.
func (f *FFmpegExtractor) ExtractAudioSegment(videoPath, audioPath string, startSec, endSec float64) error {
	duration := endSec - startSec
	cmd := exec.Command("ffmpeg",
		"-ss", fmt.Sprintf("%.3f", startSec),
		"-t", fmt.Sprintf("%.3f", duration),
		"-i", videoPath,
		"-vn",
		"-acodec", "pcm_s16le",
		"-ar", "16000",
		"-ac", "1",
		"-y",
		audioPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg error: %v, output: %s", err, string(output))
	}
	return nil
}

// CheckFFmpegAvailable checks if ffmpeg is available in the system PATH.
// Returns nil if ffmpeg is found, or an error describing the problem.
func CheckFFmpegAvailable() error {
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg not found in PATH: %w. Install ffmpeg to enable video processing", err)
	}
	// Verify it's executable by getting version
	cmd := exec.Command(path, "-version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("ffmpeg found at %s but failed to execute: %w", path, err)
	}
	// Log version info (first line contains version)
	if len(output) > 0 {
		// Version check succeeded
		return nil
	}
	return nil
}

// ValidateVideoFile uses ffprobe to verify that a file is a valid video.
// This provides defense-in-depth beyond MIME type checking, as MIME types
// can be spoofed by malicious clients.
// Returns nil if the file is a valid video, or an error describing the problem.
func ValidateVideoFile(filePath string) error {
	// First check if ffprobe is available
	probePath, err := exec.LookPath("ffprobe")
	if err != nil {
		// ffprobe not available - fall back to allowing the file
		// (MIME check already passed at this point)
		return nil
	}

	// Use ffprobe to check if file contains a video stream
	// -v error: only show errors
	// -select_streams v:0: select first video stream
	// -show_entries stream=codec_type: only show codec type
	// -of csv=p=0: output as plain CSV without headers
	cmd := exec.Command(probePath,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=codec_type",
		"-of", "csv=p=0",
		filePath,
	)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("invalid video file: ffprobe failed to read media streams")
	}

	// Check if output contains "video"
	outputStr := string(output)
	if outputStr == "" || outputStr == "\n" {
		return fmt.Errorf("invalid video file: no video stream found")
	}

	return nil
}

// ErrInvalidMagicBytes is returned when a file's magic bytes don't match any known video format.
var ErrInvalidMagicBytes = errors.New("file does not match any known video format signature")

// VideoFormat represents a recognized video container format.
type VideoFormat struct {
	Name   string
	Offset int      // Byte offset where signature starts
	Magic  [][]byte // Multiple possible signatures (any match is valid)
}

// knownVideoFormats defines the magic byte signatures for supported video formats.
// Each format may have multiple valid signatures depending on the variant.
var knownVideoFormats = []VideoFormat{
	{
		Name:   "MP4/MOV/M4V",
		Offset: 4, // ftyp box starts at byte 4
		Magic: [][]byte{
			{0x66, 0x74, 0x79, 0x70}, // "ftyp"
		},
	},
	{
		Name:   "WebM/MKV",
		Offset: 0,
		Magic: [][]byte{
			{0x1A, 0x45, 0xDF, 0xA3}, // EBML header
		},
	},
	{
		Name:   "AVI",
		Offset: 0,
		Magic: [][]byte{
			{0x52, 0x49, 0x46, 0x46}, // "RIFF" (must also check for "AVI " at offset 8)
		},
	},
	{
		Name:   "OGV",
		Offset: 0,
		Magic: [][]byte{
			{0x4F, 0x67, 0x67, 0x53}, // "OggS"
		},
	},
	{
		Name:   "MPEG",
		Offset: 0,
		Magic: [][]byte{
			{0x00, 0x00, 0x01, 0xBA}, // MPEG Program Stream
			{0x00, 0x00, 0x01, 0xB3}, // MPEG video sequence header
		},
	},
}

// ValidateMagicBytes checks if a reader contains a file with valid video magic bytes.
// This provides defense-in-depth by validating the actual file content,
// not just the MIME type which can be spoofed.
// The reader must be at the beginning of the file.
// Returns nil if valid, ErrInvalidMagicBytes if not recognized.
func ValidateMagicBytes(r io.Reader) error {
	// Read the first 12 bytes to check magic signatures
	// We need 12 bytes to check MP4 ftyp (offset 4, 4 bytes) and AVI marker (offset 8, 4 bytes)
	header := make([]byte, 12)
	n, err := io.ReadFull(r, header)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return fmt.Errorf("failed to read file header: %w", err)
	}
	if n < 8 {
		return ErrInvalidMagicBytes
	}

	// Check each known format
	for _, format := range knownVideoFormats {
		for _, magic := range format.Magic {
			if format.Offset+len(magic) > n {
				continue
			}
			if bytes.Equal(header[format.Offset:format.Offset+len(magic)], magic) {
				// Special case for AVI: must also have "AVI " at offset 8
				if format.Name == "AVI" && n >= 12 {
					if !bytes.Equal(header[8:12], []byte{0x41, 0x56, 0x49, 0x20}) { // "AVI "
						continue
					}
				}
				return nil // Valid format found
			}
		}
	}

	return ErrInvalidMagicBytes
}

// ValidateMagicBytesFromFile checks if a file has valid video magic bytes.
// This is a convenience function that opens the file and calls ValidateMagicBytes.
func ValidateMagicBytesFromFile(filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()
	return ValidateMagicBytes(f)
}

// GetVideoDuration uses ffprobe to get the duration of a video in seconds.
// Returns 0 and an error if duration cannot be determined.
func GetVideoDuration(filePath string) (float64, error) {
	probePath, err := exec.LookPath("ffprobe")
	if err != nil {
		return 0, fmt.Errorf("ffprobe not found: %w", err)
	}

	// Use ffprobe to get duration
	// -v error: only show errors
	// -show_entries format=duration: only show duration
	// -of csv=p=0: output as plain CSV without headers
	cmd := exec.Command(probePath,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "csv=p=0",
		filePath,
	)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	// Parse duration as float
	var duration float64
	_, err = fmt.Sscanf(string(output), "%f", &duration)
	if err != nil {
		return 0, fmt.Errorf("failed to parse duration: %w", err)
	}

	return duration, nil
}

// GenerateThumbnail extracts a single frame from a video at 10% of its duration.
// The thumbnail is saved as a JPEG image at the specified output path.
// Returns an error if the operation fails.
func GenerateThumbnail(videoPath, thumbnailPath string) error {
	// Get video duration to calculate 10% position
	duration, err := GetVideoDuration(videoPath)
	if err != nil {
		// Fall back to 3 seconds if duration can't be determined
		duration = 30 // assume 30s video, so we get frame at 3s
	}

	// Calculate position at 10% of duration (minimum 1 second)
	seekTime := duration * 0.1
	if seekTime < 1 {
		seekTime = 1
	}

	// Generate thumbnail using ffmpeg
	// -ss before -i: fast seek to position
	// -vframes 1: extract only one frame
	// -q:v 2: high quality JPEG (1-31, lower is better)
	// -vf scale: resize to max 320x180 while maintaining aspect ratio
	cmd := exec.Command("ffmpeg",
		"-ss", fmt.Sprintf("%.2f", seekTime),
		"-i", videoPath,
		"-vframes", "1",
		"-q:v", "2",
		"-vf", "scale=320:180:force_original_aspect_ratio=decrease",
		"-y",
		thumbnailPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg thumbnail generation failed: %v, output: %s", err, string(output))
	}

	return nil
}

// SubtitleTrack represents an embedded subtitle track in a video file.
type SubtitleTrack struct {
	Index     int    `json:"index"`      // Stream index in the container
	Language  string `json:"language"`   // ISO 639-1/2 code (e.g., "eng", "en")
	Title     string `json:"title"`      // Optional track title
	Codec     string `json:"codec"`      // Codec name (e.g., "subrip", "ass", "mov_text")
	Default   bool   `json:"default"`    // Is this the default track?
	Forced    bool   `json:"forced"`     // Is this a forced subtitle track?
	TextBased bool   `json:"text_based"` // Can this be extracted as text (vs image-based)?
}

// textBasedCodecs lists subtitle codecs that can be extracted as text.
// Image-based codecs (PGS, VOBSUB, DVB) require OCR and are not text-based.
var textBasedCodecs = map[string]bool{
	"subrip":            true,  // SRT
	"ass":               true,  // Advanced SubStation Alpha
	"ssa":               true,  // SubStation Alpha
	"mov_text":          true,  // MP4/MOV text track
	"webvtt":            true,  // WebVTT
	"text":              true,  // Plain text
	"sami":              true,  // SAMI
	"microdvd":          true,  // MicroDVD
	"mpl2":              true,  // MPL2
	"pjs":               true,  // Phoenix Subtitle
	"realtext":          true,  // RealText
	"stl":               true,  // Spruce STL
	"subviewer":         true,  // SubViewer
	"subviewer1":        true,  // SubViewer v1
	"vplayer":           true,  // VPlayer
	"hdmv_pgs_subtitle": false, // Blu-ray PGS (image-based)
	"dvd_subtitle":      false, // DVD VOBSUB (image-based)
	"dvb_subtitle":      false, // DVB (image-based)
}

// ffprobeSubtitleOutput represents the JSON output from ffprobe for subtitle streams.
type ffprobeSubtitleOutput struct {
	Streams []struct {
		Index       int    `json:"index"`
		CodecName   string `json:"codec_name"`
		Disposition struct {
			Default int `json:"default"`
			Forced  int `json:"forced"`
		} `json:"disposition"`
		Tags struct {
			Language string `json:"language"`
			Title    string `json:"title"`
		} `json:"tags"`
	} `json:"streams"`
}

// GetSubtitleTracks uses ffprobe to detect embedded subtitle tracks in a video file.
// Returns an empty slice if no subtitle tracks are found or if ffprobe is not available.
func GetSubtitleTracks(filePath string) ([]SubtitleTrack, error) {
	probePath, err := exec.LookPath("ffprobe")
	if err != nil {
		// ffprobe not available - return empty list
		return []SubtitleTrack{}, nil
	}

	// Use ffprobe to get subtitle stream information in JSON format
	// -v error: only show errors
	// -select_streams s: select subtitle streams only
	// -show_entries: show stream index, codec, disposition, and tags
	// -of json: output as JSON
	cmd := exec.Command(probePath,
		"-v", "error",
		"-select_streams", "s",
		"-show_entries", "stream=index,codec_name:stream_disposition=default,forced:stream_tags=language,title",
		"-of", "json",
		filePath,
	)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed to read subtitle streams: %w", err)
	}

	// Parse JSON output
	var probeOutput ffprobeSubtitleOutput
	if err := json.Unmarshal(output, &probeOutput); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	// Convert to SubtitleTrack slice
	tracks := make([]SubtitleTrack, 0, len(probeOutput.Streams))
	for _, stream := range probeOutput.Streams {
		codec := strings.ToLower(stream.CodecName)
		textBased := true
		if known, ok := textBasedCodecs[codec]; ok {
			textBased = known
		}

		track := SubtitleTrack{
			Index:     stream.Index,
			Language:  stream.Tags.Language,
			Title:     stream.Tags.Title,
			Codec:     stream.CodecName,
			Default:   stream.Disposition.Default == 1,
			Forced:    stream.Disposition.Forced == 1,
			TextBased: textBased,
		}
		tracks = append(tracks, track)
	}

	return tracks, nil
}

// ExtractSubtitleTrack extracts an embedded subtitle track from a video file.
// The trackIndex is the ffprobe stream index (not the subtitle track number).
// Format can be "srt" or "vtt" - default is "srt".
// Returns the extracted subtitle content as a string.
func ExtractSubtitleTrack(videoPath string, trackIndex int, format string) (string, error) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return "", fmt.Errorf("ffmpeg not available: %w", err)
	}

	// Validate format
	if format == "" {
		format = "srt"
	}
	format = strings.ToLower(format)
	if format != "srt" && format != "vtt" {
		return "", fmt.Errorf("unsupported subtitle format: %s (use srt or vtt)", format)
	}

	// Validate track index
	if trackIndex < 0 {
		return "", fmt.Errorf("invalid track index: %d", trackIndex)
	}

	// Extract subtitle to stdout using ffmpeg
	// -i: input file
	// -map 0:{index}: select the specific stream by index
	// -c:s: specify subtitle codec (srt or webvtt)
	// -f: output format
	// pipe:1: write to stdout
	codec := "srt"
	outputFormat := "srt"
	if format == "vtt" {
		codec = "webvtt"
		outputFormat = "webvtt"
	}

	cmd := exec.Command(ffmpegPath,
		"-i", videoPath,
		"-map", fmt.Sprintf("0:%d", trackIndex),
		"-c:s", codec,
		"-f", outputFormat,
		"pipe:1",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Include stderr in error for debugging
		return "", fmt.Errorf("ffmpeg subtitle extraction failed: %w (stderr: %s)", err, stderr.String())
	}

	content := stdout.String()
	if content == "" {
		return "", fmt.Errorf("extracted subtitle content is empty")
	}

	return content, nil
}

// MockExtractor is a test implementation of Extractor.
type MockExtractor struct {
	// ShouldFail controls whether ExtractAudio returns an error.
	ShouldFail bool
	// FailError is the error to return when ShouldFail is true.
	FailError error
	// CallCount tracks how many times ExtractAudio was called.
	CallCount int
	// LastVideoPath stores the last videoPath argument.
	LastVideoPath string
	// LastAudioPath stores the last audioPath argument.
	LastAudioPath string
	// SegmentCallCount tracks how many times ExtractAudioSegment was called.
	SegmentCallCount int
	// LastStartSec stores the last startSec argument to ExtractAudioSegment.
	LastStartSec float64
	// LastEndSec stores the last endSec argument to ExtractAudioSegment.
	LastEndSec float64
}

// NewMockExtractor creates a new MockExtractor for testing.
func NewMockExtractor() *MockExtractor {
	return &MockExtractor{}
}

// ExtractAudio implements the Extractor interface for testing.
func (m *MockExtractor) ExtractAudio(videoPath, audioPath string) error {
	m.CallCount++
	m.LastVideoPath = videoPath
	m.LastAudioPath = audioPath
	if m.ShouldFail {
		if m.FailError != nil {
			return m.FailError
		}
		return fmt.Errorf("mock extraction failed")
	}
	return nil
}

// ExtractAudioSegment implements the Extractor interface for testing.
func (m *MockExtractor) ExtractAudioSegment(videoPath, audioPath string, startSec, endSec float64) error {
	m.SegmentCallCount++
	m.LastVideoPath = videoPath
	m.LastAudioPath = audioPath
	m.LastStartSec = startSec
	m.LastEndSec = endSec
	if m.ShouldFail {
		if m.FailError != nil {
			return m.FailError
		}
		return fmt.Errorf("mock segment extraction failed")
	}
	return nil
}

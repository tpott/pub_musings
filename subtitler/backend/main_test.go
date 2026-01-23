package main

import (
	"strings"
	"testing"
)

func TestFormatSRTTimestamp(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{0.0, "00:00:00,000"},
		{1.5, "00:00:01,500"},
		{61.123, "00:01:01,123"},
		{3661.999, "01:01:01,999"},
		{3723.456, "01:02:03,456"},
	}

	for _, tc := range tests {
		result := formatSRTTimestamp(tc.input)
		if result != tc.expected {
			t.Errorf("formatSRTTimestamp(%f) = %s, expected %s", tc.input, result, tc.expected)
		}
	}
}

func TestGenerateSRT(t *testing.T) {
	result := &WhisperResult{
		Language: "en",
		Duration: 10.0,
		Text:     "Hello world. This is a test.",
		Segments: []WhisperSegment{
			{ID: 0, Start: 0.0, End: 2.5, Text: " Hello world."},
			{ID: 1, Start: 3.0, End: 5.5, Text: " This is a test."},
		},
	}

	srt := generateSRT(result)

	// Check that we have proper SRT format
	if !strings.Contains(srt, "1\n") {
		t.Error("SRT should contain sequence number 1")
	}
	if !strings.Contains(srt, "2\n") {
		t.Error("SRT should contain sequence number 2")
	}
	if !strings.Contains(srt, "00:00:00,000 --> 00:00:02,500") {
		t.Error("SRT should contain first timestamp")
	}
	if !strings.Contains(srt, "00:00:03,000 --> 00:00:05,500") {
		t.Error("SRT should contain second timestamp")
	}
	if !strings.Contains(srt, "Hello world.") {
		t.Error("SRT should contain first subtitle text")
	}
	if !strings.Contains(srt, "This is a test.") {
		t.Error("SRT should contain second subtitle text")
	}

	// Check proper spacing between entries
	lines := strings.Split(srt, "\n")
	if len(lines) < 8 {
		t.Errorf("SRT should have at least 8 lines, got %d", len(lines))
	}
}

func TestGenerateSRTEmpty(t *testing.T) {
	result := &WhisperResult{
		Language: "en",
		Duration: 0.0,
		Text:     "",
		Segments: []WhisperSegment{},
	}

	srt := generateSRT(result)

	if srt != "" {
		t.Errorf("Empty segments should produce empty SRT, got: %s", srt)
	}
}

func TestFormatVTTTimestamp(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{0.0, "00:00:00.000"},
		{1.5, "00:00:01.500"},
		{61.123, "00:01:01.123"},
		{3661.999, "01:01:01.999"},
		{3723.456, "01:02:03.456"},
	}

	for _, tc := range tests {
		result := formatVTTTimestamp(tc.input)
		if result != tc.expected {
			t.Errorf("formatVTTTimestamp(%f) = %s, expected %s", tc.input, result, tc.expected)
		}
	}
}

func TestGenerateVTT(t *testing.T) {
	result := &WhisperResult{
		Language: "en",
		Duration: 10.0,
		Text:     "Hello world. This is a test.",
		Segments: []WhisperSegment{
			{ID: 0, Start: 0.0, End: 2.5, Text: " Hello world."},
			{ID: 1, Start: 3.0, End: 5.5, Text: " This is a test."},
		},
	}

	vtt := generateVTT(result)

	// Check WEBVTT header
	if !strings.HasPrefix(vtt, "WEBVTT\n\n") {
		t.Error("VTT should start with 'WEBVTT' header followed by blank line")
	}

	// Check that we have proper VTT format
	if !strings.Contains(vtt, "1\n") {
		t.Error("VTT should contain cue identifier 1")
	}
	if !strings.Contains(vtt, "2\n") {
		t.Error("VTT should contain cue identifier 2")
	}
	// VTT uses period instead of comma for milliseconds
	if !strings.Contains(vtt, "00:00:00.000 --> 00:00:02.500") {
		t.Error("VTT should contain first timestamp with period separator")
	}
	if !strings.Contains(vtt, "00:00:03.000 --> 00:00:05.500") {
		t.Error("VTT should contain second timestamp with period separator")
	}
	if !strings.Contains(vtt, "Hello world.") {
		t.Error("VTT should contain first subtitle text")
	}
	if !strings.Contains(vtt, "This is a test.") {
		t.Error("VTT should contain second subtitle text")
	}
}

func TestGenerateVTTEmpty(t *testing.T) {
	result := &WhisperResult{
		Language: "en",
		Duration: 0.0,
		Text:     "",
		Segments: []WhisperSegment{},
	}

	vtt := generateVTT(result)

	// Even with no segments, VTT should have header
	if vtt != "WEBVTT\n\n" {
		t.Errorf("Empty segments should produce just WEBVTT header, got: %s", vtt)
	}
}

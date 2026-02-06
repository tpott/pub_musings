package api

import (
	"os"
	"testing"
)

func TestWebMParserExtractsInitSegment(t *testing.T) {
	fixtureData, err := os.ReadFile("../../tests/fixtures/me-show-me-a-cat.webm")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	p := NewWebMParser()

	// Feed data in small chunks (like WebSocket)
	chunkSize := 256
	for offset := 0; offset < len(fixtureData); offset += chunkSize {
		end := offset + chunkSize
		if end > len(fixtureData) {
			end = len(fixtureData)
		}
		p.Append(fixtureData[offset:end])
	}

	if !p.Parsed() {
		t.Fatal("expected parser to find init segment")
	}

	initSeg := p.InitSegment()
	if len(initSeg) == 0 {
		t.Fatal("init segment is empty")
	}

	// Init segment must start with EBML magic
	if initSeg[0] != 0x1A || initSeg[1] != 0x45 || initSeg[2] != 0xDF || initSeg[3] != 0xA3 {
		t.Errorf("init segment does not start with EBML magic: got %02x %02x %02x %02x",
			initSeg[0], initSeg[1], initSeg[2], initSeg[3])
	}

	// Init segment must NOT contain cluster element ID
	for i := 0; i < len(initSeg)-3; i++ {
		if initSeg[i] == 0x1F && initSeg[i+1] == 0x43 && initSeg[i+2] == 0xB6 && initSeg[i+3] == 0x75 {
			t.Errorf("init segment contains cluster element ID at offset %d", i)
		}
	}

	t.Logf("init segment size: %d bytes", len(initSeg))
}

func TestWebMParserGrabAudioStartsWithEBMLMagic(t *testing.T) {
	fixtureData, err := os.ReadFile("../../tests/fixtures/me-show-me-a-cat.webm")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	p := NewWebMParser()

	// Feed first 5KB (enough to find init segment)
	p.Append(fixtureData[:5000])

	audio := p.GrabAudio()
	if audio == nil {
		t.Fatal("GrabAudio returned nil")
	}

	if audio[0] != 0x1A || audio[1] != 0x45 || audio[2] != 0xDF || audio[3] != 0xA3 {
		t.Errorf("first grab does not start with EBML magic: got %02x %02x %02x %02x",
			audio[0], audio[1], audio[2], audio[3])
	}

	// Feed more data (simulating continued recording)
	p.Append(fixtureData[5000:10000])

	audio2 := p.GrabAudio()
	if audio2 == nil {
		t.Fatal("second GrabAudio returned nil")
	}

	// Second grab must also start with EBML magic
	if audio2[0] != 0x1A || audio2[1] != 0x45 || audio2[2] != 0xDF || audio2[3] != 0xA3 {
		t.Errorf("second grab does not start with EBML magic: got %02x %02x %02x %02x",
			audio2[0], audio2[1], audio2[2], audio2[3])
	}

	// Second grab should be larger (more data accumulated)
	if len(audio2) <= len(audio) {
		t.Errorf("expected second grab (%d bytes) to be larger than first (%d bytes)",
			len(audio2), len(audio))
	}

	t.Logf("first grab: %d bytes, second grab: %d bytes", len(audio), len(audio2))
}

func TestWebMParserResetClearsState(t *testing.T) {
	fixtureData, err := os.ReadFile("../../tests/fixtures/me-show-me-a-cat.webm")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	p := NewWebMParser()
	p.Append(fixtureData[:5000])

	if !p.Parsed() {
		t.Fatal("expected parser to be parsed after appending data")
	}

	p.Reset()

	if p.Parsed() {
		t.Error("expected parser to be unparsed after Reset")
	}
	if p.InitSegment() != nil {
		t.Error("expected init segment to be nil after Reset")
	}
	if p.BufferLen() != 0 {
		t.Error("expected buffer to be empty after Reset")
	}
}

func TestWebMParserEmptyBuffer(t *testing.T) {
	p := NewWebMParser()

	if p.Parsed() {
		t.Error("new parser should not be parsed")
	}

	audio := p.GrabAudio()
	if audio != nil {
		t.Error("GrabAudio on empty buffer should return nil")
	}
}

func TestWebMParserTooSmallForInitSegment(t *testing.T) {
	p := NewWebMParser()

	// Feed data that's too small to contain a Cluster element ID
	p.Append([]byte{0x1A, 0x45, 0xDF, 0xA3, 0x00, 0x01, 0x02})

	if p.Parsed() {
		t.Error("parser should not be parsed with only 7 bytes of EBML header")
	}

	// GrabAudio should still return the raw bytes
	audio := p.GrabAudio()
	if audio == nil {
		t.Fatal("GrabAudio should return raw bytes when init segment not found")
	}
	if len(audio) != 7 {
		t.Errorf("expected 7 bytes, got %d", len(audio))
	}
}

func TestParseClusters(t *testing.T) {
	fixtureData, err := os.ReadFile("../../tests/fixtures/me-show-me-a-cat.webm")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	clusters := ParseClusters(fixtureData)
	if len(clusters) == 0 {
		t.Fatal("expected at least one cluster")
	}

	t.Logf("found %d cluster(s)", len(clusters))
	for i, c := range clusters {
		t.Logf("cluster %d: timecode=%d, offset=%d, length=%d", i, c.TimecodeMs, c.ByteOffset, c.ByteLength)
	}

	// The fixture has one cluster at offset 497
	if clusters[0].ByteOffset != 497 {
		t.Errorf("expected first cluster at offset 497, got %d", clusters[0].ByteOffset)
	}
}

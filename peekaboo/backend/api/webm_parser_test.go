package api

import (
	"bytes"
	"os"
	"testing"

	ebml "github.com/at-wat/ebml-go"
	"github.com/at-wat/ebml-go/webm"
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

func TestWebMParserClearThenAppendPreservesInit(t *testing.T) {
	data, err := os.ReadFile("../../tests/fixtures/me-show-me-a-cat.webm")
	if err != nil {
		t.Fatalf("failed to read test audio: %v", err)
	}

	p := NewWebMParser()
	p.Append(data)

	if !p.Parsed() {
		t.Fatal("expected parser to be parsed after appending WebM data")
	}

	initSeg := p.InitSegment()
	if initSeg == nil {
		t.Fatal("expected init segment to be non-nil")
	}

	// Grab audio before clear
	firstGrab := p.GrabAudio()
	if firstGrab == nil {
		t.Fatal("expected non-nil audio before clear")
	}

	// Clear the buffer
	p.Clear()
	if p.BufferLen() != 0 {
		t.Errorf("expected buffer to be empty after Clear, got %d", p.BufferLen())
	}

	// Append new cluster data (simulating new audio arriving)
	newCluster := data[len(initSeg):] // just the cluster portion
	p.Append(newCluster)

	// GrabAudio should return initSegment + newCluster
	secondGrab := p.GrabAudio()
	if secondGrab == nil {
		t.Fatal("expected non-nil audio after Clear + Append")
	}

	// Verify it starts with the EBML magic bytes
	if !bytes.HasPrefix(secondGrab, []byte{0x1A, 0x45, 0xDF, 0xA3}) {
		t.Error("GrabAudio after Clear should start with EBML header")
	}

	// Verify it contains the init segment
	if !bytes.HasPrefix(secondGrab, initSeg) {
		t.Error("GrabAudio after Clear should start with cached init segment")
	}

	// Verify the total length is initSegment + newCluster
	expectedLen := len(initSeg) + len(newCluster)
	if len(secondGrab) != expectedLen {
		t.Errorf("expected GrabAudio length %d (init=%d + cluster=%d), got %d",
			expectedLen, len(initSeg), len(newCluster), len(secondGrab))
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

// buildMultiClusterWebM creates a minimal valid WebM with multiple Cluster
// elements at known timecodes. Each cluster contains a small dummy audio frame.
func buildMultiClusterWebM(t *testing.T, timecodes []uint64) []byte {
	t.Helper()

	type webmFile struct {
		Header  webm.EBMLHeader `ebml:"EBML"`
		Segment struct {
			Info struct {
				TimecodeScale uint64 `ebml:"TimecodeScale"`
				MuxingApp     string `ebml:"MuxingApp"`
			} `ebml:"Info"`
			Tracks struct {
				TrackEntry []struct {
					TrackNumber uint64 `ebml:"TrackNumber"`
					TrackType   uint64 `ebml:"TrackType"`
					CodecID     string `ebml:"CodecID"`
				} `ebml:"TrackEntry"`
			} `ebml:"Tracks"`
			Cluster []webm.Cluster `ebml:"Cluster"`
		} `ebml:"Segment"`
	}

	f := webmFile{}
	f.Header.EBMLVersion = 1
	f.Header.EBMLReadVersion = 1
	f.Header.EBMLMaxIDLength = 4
	f.Header.EBMLMaxSizeLength = 8
	f.Header.DocType = "webm"
	f.Header.DocTypeVersion = 4
	f.Header.DocTypeReadVersion = 2
	f.Segment.Info.TimecodeScale = 1000000 // 1ms
	f.Segment.Info.MuxingApp = "test"
	f.Segment.Tracks.TrackEntry = []struct {
		TrackNumber uint64 `ebml:"TrackNumber"`
		TrackType   uint64 `ebml:"TrackType"`
		CodecID     string `ebml:"CodecID"`
	}{{TrackNumber: 1, TrackType: 2, CodecID: "A_OPUS"}}

	for _, tc := range timecodes {
		// SimpleBlock: trackNum=1 (0x81), timecode=0, keyframe flag=0x80
		block := []byte{0x81, 0x00, 0x00, 0x80}
		block = append(block, make([]byte, 100)...) // dummy audio data
		f.Segment.Cluster = append(f.Segment.Cluster, webm.Cluster{
			Timecode:    tc,
			SimpleBlock: []ebml.Block{{Data: [][]byte{block}}},
		})
	}

	var buf bytes.Buffer
	if err := ebml.Marshal(&f, &buf); err != nil {
		t.Fatalf("failed to marshal WebM: %v", err)
	}
	return buf.Bytes()
}

func TestWebMParserTrimBeforeMultiCluster(t *testing.T) {
	// Create a WebM with 4 clusters at 0ms, 500ms, 1000ms, 1500ms
	data := buildMultiClusterWebM(t, []uint64{0, 500, 1000, 1500})

	// Verify we can parse 4 clusters
	clusters := ParseClusters(data)
	if len(clusters) != 4 {
		t.Fatalf("expected 4 clusters, got %d", len(clusters))
	}
	t.Logf("clusters before trim: %d", len(clusters))
	for i, c := range clusters {
		t.Logf("  cluster %d: timecode=%d, offset=%d, len=%d", i, c.TimecodeMs, c.ByteOffset, c.ByteLength)
	}

	// Load into parser
	p := NewWebMParser()
	p.Append(data)

	if !p.Parsed() {
		t.Fatal("parser should be parsed")
	}

	bufBefore := p.BufferLen()

	// Trim clusters before 1000ms (should remove clusters at 0ms and 500ms)
	trimmed := p.TrimBefore(1000)
	if trimmed <= 0 {
		t.Fatalf("expected positive trim, got %d", trimmed)
	}

	bufAfter := p.BufferLen()
	if bufAfter >= bufBefore {
		t.Errorf("buffer should be smaller after trim: before=%d, after=%d", bufBefore, bufAfter)
	}

	// GrabAudio should still produce valid WebM (init + remaining clusters)
	audio := p.GrabAudio()
	if audio == nil {
		t.Fatal("GrabAudio returned nil after trim")
	}

	// Must start with EBML magic
	if audio[0] != 0x1A || audio[1] != 0x45 || audio[2] != 0xDF || audio[3] != 0xA3 {
		t.Errorf("trimmed audio does not start with EBML magic")
	}

	// Parse the trimmed output — should have only clusters at 1000ms and 1500ms
	remainingClusters := ParseClusters(audio)
	if len(remainingClusters) != 2 {
		t.Fatalf("expected 2 remaining clusters, got %d", len(remainingClusters))
	}
	if remainingClusters[0].TimecodeMs != 1000 {
		t.Errorf("expected first remaining cluster at 1000ms, got %d", remainingClusters[0].TimecodeMs)
	}
	if remainingClusters[1].TimecodeMs != 1500 {
		t.Errorf("expected second remaining cluster at 1500ms, got %d", remainingClusters[1].TimecodeMs)
	}
}

func TestWebMParserTrimBeforeNoOp(t *testing.T) {
	data := buildMultiClusterWebM(t, []uint64{0, 500, 1000})

	p := NewWebMParser()
	p.Append(data)

	// Trim at 0 should be a no-op (first cluster is at 0, nothing before it)
	trimmed := p.TrimBefore(0)
	if trimmed != 0 {
		t.Errorf("expected no trim at timecode 0, got %d bytes", trimmed)
	}

	// Trim beyond all clusters should be a no-op (no cluster at or after 9999ms)
	trimmed = p.TrimBefore(9999)
	if trimmed != 0 {
		t.Errorf("expected no trim beyond all clusters, got %d bytes", trimmed)
	}
}

func TestWebMParserTrimBeforeUnparsed(t *testing.T) {
	p := NewWebMParser()
	p.Append([]byte{0x01, 0x02, 0x03})

	// Should be no-op on unparsed buffer
	trimmed := p.TrimBefore(1000)
	if trimmed != 0 {
		t.Errorf("expected no trim on unparsed buffer, got %d", trimmed)
	}
}

func TestWebMParserAppendMaxBufferSize(t *testing.T) {
	p := &WebMParser{maxBufferSize: 100}

	// Append within limit should succeed
	ok := p.Append(make([]byte, 50))
	if !ok {
		t.Fatal("Append within limit should succeed")
	}

	// Append that would exceed limit should fail
	ok = p.Append(make([]byte, 60))
	if ok {
		t.Fatal("Append exceeding limit should fail")
	}

	// Buffer should remain at original size
	if p.BufferLen() != 50 {
		t.Errorf("Buffer should remain at 50 after rejected append, got %d", p.BufferLen())
	}

	// Append up to exact limit should succeed
	ok = p.Append(make([]byte, 50))
	if !ok {
		t.Fatal("Append to exact limit should succeed")
	}

	// Any further append should fail
	ok = p.Append(make([]byte, 1))
	if ok {
		t.Fatal("Append beyond limit should fail")
	}
}

func TestWebMParserClusterDataLen(t *testing.T) {
	data := buildMultiClusterWebM(t, []uint64{0, 500})

	p := NewWebMParser()
	if p.ClusterDataLen() != 0 {
		t.Error("expected 0 cluster data len before any data")
	}

	p.Append(data)
	cdl := p.ClusterDataLen()
	if cdl <= 0 {
		t.Errorf("expected positive cluster data len, got %d", cdl)
	}
	t.Logf("cluster data len: %d bytes", cdl)
}

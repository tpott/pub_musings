package api

import (
	"bytes"
	"log/slog"

	ebml "github.com/at-wat/ebml-go"
	"github.com/at-wat/ebml-go/webm"
)

// clusterElementID is the EBML element ID for the Cluster element (0x1F43B675).
var clusterElementID = []byte{0x1F, 0x43, 0xB6, 0x75}

// WebMParser extracts the init segment from a WebM byte stream and produces
// valid WebM output on every buffer grab.
//
// A WebM file starts with an init segment (EBML Header + Segment metadata +
// Info + Tracks) followed by one or more Cluster elements containing audio
// data. Every valid WebM must start with the init segment so decoders know
// the codec, sample rate, and channel layout.
//
// When the backend splits the audio buffer at a time threshold, naive
// splitting produces corrupt WebM: the first split has the init segment,
// but subsequent splits start mid-Cluster without it. This parser fixes
// that by caching the init segment and always including it in the output.
//
// Current strategy: each GrabAudio call returns initSegment + ALL cluster
// data accumulated so far. This re-transcribes earlier audio but produces
// correct WebM every time. Future phases will optimize with selective
// cluster extraction.
//
// defaultMaxBufferSize is the maximum audio buffer size (50 MB).
// Prevents memory exhaustion from stuck sessions that keep sending audio
// without the buffer being processed/cleared.
const defaultMaxBufferSize = 50 * 1024 * 1024

// Thread safety: WebMParser is NOT goroutine-safe. The caller must hold
// a lock (e.g. connectionState.mu) when calling any method.
type WebMParser struct {
	rawBuffer     []byte // all bytes received (never cleared while recording)
	initSegment   []byte // cached: bytes before first Cluster element
	clusterPos    int    // byte offset where cluster data begins
	parsed        bool   // true once init segment has been extracted
	maxBufferSize int    // max allowed rawBuffer size (0 = defaultMaxBufferSize)
}

// ClusterRef records the byte position and timecode of a Cluster element
// within a WebM byte stream. Exported for use by future phases.
type ClusterRef struct {
	TimecodeMs uint64 // absolute timecode from the Cluster element
	ByteOffset int    // offset where this Cluster element starts
	ByteLength int    // total length of the Cluster element (header + content)
}

// NewWebMParser creates a new parser with an empty buffer.
func NewWebMParser() *WebMParser {
	return &WebMParser{}
}

// maxBufSize returns the effective max buffer size.
func (p *WebMParser) maxBufSize() int {
	if p.maxBufferSize > 0 {
		return p.maxBufferSize
	}
	return defaultMaxBufferSize
}

// Append adds raw bytes to the internal buffer and attempts to extract
// the init segment if not yet found. Returns false if the append would
// exceed the maximum buffer size.
func (p *WebMParser) Append(data []byte) bool {
	if len(p.rawBuffer)+len(data) > p.maxBufSize() {
		return false
	}
	p.rawBuffer = append(p.rawBuffer, data...)

	if !p.parsed {
		p.tryExtractInitSegment()
	}
	return true
}

// tryExtractInitSegment searches for the first Cluster element ID in the
// buffer. Everything before it is the init segment.
func (p *WebMParser) tryExtractInitSegment() {
	idx := bytes.Index(p.rawBuffer, clusterElementID)
	if idx < 0 {
		return // not enough data yet
	}

	p.initSegment = make([]byte, idx)
	copy(p.initSegment, p.rawBuffer[:idx])
	p.clusterPos = idx
	p.parsed = true
}

// GrabAudio returns a valid WebM containing the init segment and all cluster
// data accumulated so far. The internal buffer is NOT cleared — data
// continues to accumulate for the next grab.
//
// Returns nil if the buffer is empty. Returns raw buffer bytes if the init
// segment has not yet been found (buffer too small).
func (p *WebMParser) GrabAudio() []byte {
	if len(p.rawBuffer) == 0 {
		return nil
	}

	if !p.parsed {
		// Init segment not yet found; return raw bytes as-is
		result := make([]byte, len(p.rawBuffer))
		copy(result, p.rawBuffer)
		return result
	}

	// Return init segment + all cluster data
	clusterData := p.rawBuffer[p.clusterPos:]
	result := make([]byte, len(p.initSegment)+len(clusterData))
	copy(result, p.initSegment)
	copy(result[len(p.initSegment):], clusterData)
	return result
}

// Clear resets the raw buffer but preserves the cached init segment.
func (p *WebMParser) Clear() {
	p.rawBuffer = nil
}

// BufferLen returns the current raw buffer length.
func (p *WebMParser) BufferLen() int {
	return len(p.rawBuffer)
}

// Parsed returns true if the init segment has been extracted.
func (p *WebMParser) Parsed() bool {
	return p.parsed
}

// InitSegment returns the cached init segment bytes, or nil if not yet found.
func (p *WebMParser) InitSegment() []byte {
	return p.initSegment
}

// Reset clears all state including the cached init segment.
func (p *WebMParser) Reset() {
	p.rawBuffer = nil
	p.initSegment = nil
	p.clusterPos = 0
	p.parsed = false
}

// TrimBefore discards all Cluster data in the raw buffer with timecodes
// strictly before the given timecode (in milliseconds). The init segment is
// preserved. Returns the number of bytes trimmed.
//
// This is used after LLM boundary detection: once the LLM confirms an
// instruction ends at a certain word, we map that word's end time to a
// Cluster boundary and trim everything before it. The remaining audio
// (after the instruction) stays in the buffer for the next transcription.
//
// If the buffer is not parsed or no clusters are found, this is a no-op.
func (p *WebMParser) TrimBefore(timecodeMs uint64) int {
	if !p.parsed || len(p.rawBuffer) <= p.clusterPos {
		return 0
	}

	// Build a valid WebM (init + clusters) so ParseClusters can parse it
	fullWebM := p.GrabAudio()
	clusters := ParseClusters(fullWebM)
	if len(clusters) == 0 {
		return 0
	}

	// Find the first cluster with TimecodeMs >= timecodeMs
	trimIdx := -1
	for i, c := range clusters {
		if c.TimecodeMs >= timecodeMs {
			trimIdx = i
			break
		}
	}

	if trimIdx <= 0 {
		// Nothing to trim (first cluster is already at or after the timecode,
		// or no cluster matches)
		return 0
	}

	// The cluster offsets in ParseClusters are relative to the full WebM
	// (init + clusters). Convert to rawBuffer offset.
	// In fullWebM: initSegment occupies [0, len(initSegment))
	//              clusters start at len(initSegment)
	// In rawBuffer: clusters start at p.clusterPos
	keepOffset := clusters[trimIdx].ByteOffset
	// Convert from fullWebM offset to rawBuffer offset
	rawKeepOffset := p.clusterPos + (keepOffset - len(p.initSegment))

	if rawKeepOffset <= p.clusterPos || rawKeepOffset >= len(p.rawBuffer) {
		return 0
	}

	trimmed := rawKeepOffset - p.clusterPos
	remaining := make([]byte, p.clusterPos+len(p.rawBuffer)-rawKeepOffset)
	copy(remaining, p.rawBuffer[:p.clusterPos])                 // keep init segment area
	copy(remaining[p.clusterPos:], p.rawBuffer[rawKeepOffset:]) // keep clusters from keepOffset onward
	p.rawBuffer = remaining

	return trimmed
}

// ClusterDataLen returns the length of cluster data (audio) in the buffer,
// excluding the init segment. Returns 0 if not yet parsed.
func (p *WebMParser) ClusterDataLen() int {
	if !p.parsed || len(p.rawBuffer) <= p.clusterPos {
		return 0
	}
	return len(p.rawBuffer) - p.clusterPos
}

// ParseClusters uses ebml-go to parse a complete WebM byte stream and return
// cluster references with timecodes and byte positions.
//
// Note: ebml-go read hooks fire before a Cluster's children are populated,
// so we use hooks only for byte positions and pair them with the fully
// parsed Segment.Cluster timecodes after unmarshal completes.
func ParseClusters(data []byte) []ClusterRef {
	var positions []uint64

	var ws struct {
		Header  webm.EBMLHeader `ebml:"EBML"`
		Segment webm.Segment    `ebml:"Segment"`
	}

	r := bytes.NewReader(data)
	if err := ebml.Unmarshal(r, &ws, ebml.WithElementReadHooks(func(elem *ebml.Element) {
		if elem.Name == "Cluster" {
			positions = append(positions, elem.Position)
		}
	})); err != nil {
		slog.Debug("webm unmarshal error", "error", err, "data_len", len(data))
	}

	// Pair hook positions with parsed cluster timecodes.
	// If counts don't match (shouldn't happen), use whichever is shorter.
	n := len(positions)
	if len(ws.Segment.Cluster) < n {
		n = len(ws.Segment.Cluster)
	}

	var refs []ClusterRef
	for i := 0; i < n; i++ {
		ref := ClusterRef{
			TimecodeMs: ws.Segment.Cluster[i].Timecode,
			ByteOffset: int(positions[i]),
		}
		if i+1 < n {
			ref.ByteLength = int(positions[i+1]) - ref.ByteOffset
		} else {
			ref.ByteLength = len(data) - ref.ByteOffset
		}
		refs = append(refs, ref)
	}

	return refs
}

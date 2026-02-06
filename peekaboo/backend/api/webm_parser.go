package api

import (
	"bytes"

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
// Thread safety: WebMParser is NOT goroutine-safe. The caller must hold
// a lock (e.g. connectionState.mu) when calling any method.
type WebMParser struct {
	rawBuffer   []byte // all bytes received (never cleared while recording)
	initSegment []byte // cached: bytes before first Cluster element
	clusterPos  int    // byte offset where cluster data begins
	parsed      bool   // true once init segment has been extracted
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

// Append adds raw bytes to the internal buffer and attempts to extract
// the init segment if not yet found.
func (p *WebMParser) Append(data []byte) {
	p.rawBuffer = append(p.rawBuffer, data...)

	if !p.parsed {
		p.tryExtractInitSegment()
	}
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

// ParseClusters uses ebml-go to parse a complete WebM byte stream and return
// cluster references with timecodes and byte positions. This is intended for
// audio trimming in later phases.
func ParseClusters(data []byte) []ClusterRef {
	type clusterHook struct {
		position uint64
		timecode uint64
	}
	var hooks []clusterHook

	var ws struct {
		Header  webm.EBMLHeader `ebml:"EBML"`
		Segment webm.Segment    `ebml:"Segment"`
	}

	r := bytes.NewReader(data)
	_ = ebml.Unmarshal(r, &ws, ebml.WithElementReadHooks(func(elem *ebml.Element) {
		if elem.Name == "Cluster" {
			if cluster, ok := elem.Value.(webm.Cluster); ok {
				hooks = append(hooks, clusterHook{
					position: elem.Position,
					timecode: cluster.Timecode,
				})
			}
		}
	}))

	var refs []ClusterRef
	for i, h := range hooks {
		ref := ClusterRef{
			TimecodeMs: h.timecode,
			ByteOffset: int(h.position),
		}
		if i+1 < len(hooks) {
			ref.ByteLength = int(hooks[i+1].position) - ref.ByteOffset
		} else {
			ref.ByteLength = len(data) - ref.ByteOffset
		}
		refs = append(refs, ref)
	}

	return refs
}

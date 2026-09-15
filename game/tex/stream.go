package tex

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/energye/gpui/game/core"
)

// Frozen stream budgets. Beyond budget is OutOfMemory, never a guess.
const (
	// MaxStreams caps ready plus in-flight ids in one Stream.
	MaxStreams = 1024
	// MaxStreamBytes caps retained decoded RGBA8 bytes in one Stream.
	MaxStreamBytes = 64 << 20
	// MaxStreamAssetBytes caps one KTX2 file payload.
	MaxStreamAssetBytes = 8 << 20
	// MaxStreamIDLen caps the id text length in bytes.
	MaxStreamIDLen = 128
)

// Frozen placeholder geometry: one 4x4 BC1 block of opaque magenta.
const (
	placeholderWidth  = 4
	placeholderHeight = 4
	placeholderUpload = 8
	placeholderPixels = 64
)

// placeholderImage is the shared missing-texture stand-in. Images are
// immutable after Ready, so sharing never aliases mutable state.
var placeholderImage = &Image{
	format: FormatBC1RGBAUnorm,
	width:  placeholderWidth,
	height: placeholderHeight,
	blocks: []byte{
		0x1F, 0xF8, 0x1F, 0xF8, 0, 0, 0, 0,
	},
	pixels: []byte{
		255, 0, 255, 255, 255, 0, 255, 255, 255, 0, 255, 255, 255, 0, 255, 255,
		255, 0, 255, 255, 255, 0, 255, 255, 255, 0, 255, 255, 255, 0, 255, 255,
		255, 0, 255, 255, 255, 0, 255, 255, 255, 0, 255, 255, 255, 0, 255, 255,
		255, 0, 255, 255, 255, 0, 255, 255, 255, 0, 255, 255, 255, 0, 255, 255,
	},
}

// PlaceholderImage returns the shared 4x4 magenta stand-in.
// Never nil; callers must not mutate it (accessors already copy).
func PlaceholderImage() *Image { return placeholderImage }

// State names the lifecycle of one streamed id.
type State int

const (
	// StateEmpty means no record for the id.
	StateEmpty State = iota
	// StateLoading means a background Request is in flight.
	StateLoading
	// StateReady means the decoded image is cached.
	StateReady
	// StateMissing is the placeholder for a NotFound id or file.
	StateMissing
	// StateFailed is the placeholder for BadData/Unsupported/OutOfMemory.
	StateFailed
)

var streamStateNames = []string{"empty", "loading", "ready", "missing", "failed"}

// String returns the stable log name of s.
func (s State) String() string {
	if s >= StateEmpty && int(s) < len(streamStateNames) {
		return streamStateNames[int(s)]
	}
	return "empty"
}

// Stats reports Stream counters. Count is Ready entries; TotalBytes sums
// their decoded RGBA8 bytes; TotalUpload sums their GPU block bytes.
type Stats struct {
	Count       int
	TotalBytes  int64
	TotalUpload int64
	Loads       int64
	Unloads     int64
	Requests    int64
}

type streamRecord struct {
	state   State
	image   *Image
	failure error
}

// Stream owns one background texture ledger: ids decode off the play
// path through tex.ParseKTX2, Poll never blocks, missing ids serve the
// shared magenta placeholder. A nil Stream never panics. Only core
// numbers are used; the old PNG/JPG/WebP sync path stays untouched.
type Stream struct {
	mu          sync.Mutex
	ledger      *core.Manager
	recs        map[core.AssetID]*streamRecord
	stacks      map[core.AssetID][]*core.Handle
	totalBytes  int64
	totalUpload int64
	loads       int64
	unloads     int64
	requests    int64
}

// NewStream builds an empty background ledger.
func NewStream() *Stream {
	return &Stream{
		ledger: core.NewManager(),
		recs:   map[core.AssetID]*streamRecord{},
		stacks: map[core.AssetID][]*core.Handle{},
	}
}

func checkStreamID(id core.AssetID) error {
	if id.Empty() {
		return core.InvalidArg("tex.Stream", "")
	}
	if len(string(id)) > MaxStreamIDLen {
		return core.InvalidArg("tex.Stream", string(id))
	}
	return nil
}

// storeReadyLocked caches im and acquires one claim. Caller holds s.mu.
func (s *Stream) storeReadyLocked(id core.AssetID, im *Image) *Image {
	r, ok := s.recs[id]
	if !ok {
		if len(s.recs) >= MaxStreams {
			return nil
		}
		r = &streamRecord{}
		s.recs[id] = r
	} else if r.state == StateReady && r.image != nil {
		s.totalBytes -= int64(r.image.PixelSize())
		s.totalUpload -= int64(r.image.UploadSize())
	}
	r.state = StateReady
	r.image = im
	r.failure = nil
	s.totalBytes += int64(im.PixelSize())
	s.totalUpload += int64(im.UploadSize())
	s.loads++
	h, _ := s.ledger.Acquire(id)
	s.stacks[id] = append(s.stacks[id], h)
	return im
}

// storeFailureLocked caches a failure without bytes. Caller holds s.mu.
func (s *Stream) storeFailureLocked(id core.AssetID, err error) {
	st := StateFailed
	if core.CodeOf(err) == core.CodeNotFound {
		st = StateMissing
	}
	r, ok := s.recs[id]
	if !ok {
		if len(s.recs) >= MaxStreams {
			return
		}
		r = &streamRecord{}
		s.recs[id] = r
	} else if r.state == StateReady && r.image != nil {
		s.totalBytes -= int64(r.image.PixelSize())
		s.totalUpload -= int64(r.image.UploadSize())
	}
	r.state = st
	r.image = nil
	r.failure = err
}

// Request starts a background decode of data under id. Data is copied
// before the worker starts, so the caller may reuse it. Poll reports
// Loading until the worker stores Ready, Missing, or Failed. Duplicate
// Requests while Loading are no-ops.
func (s *Stream) Request(id core.AssetID, data []byte) error {
	const op = "tex.Stream.Request"
	if s == nil {
		return core.InvalidArg(op, "stream")
	}
	if err := checkStreamID(id); err != nil {
		return err
	}
	if len(data) == 0 {
		return core.InvalidArg(op, string(id))
	}
	payload := cloneBytes(data)
	s.mu.Lock()
	if r, ok := s.recs[id]; ok && r.state == StateLoading {
		s.mu.Unlock()
		return nil
	}
	if _, ok := s.recs[id]; !ok && len(s.recs) >= MaxStreams {
		s.mu.Unlock()
		return core.OutOfMemory(op, string(id))
	}
	r, ok := s.recs[id]
	if !ok {
		r = &streamRecord{}
		s.recs[id] = r
	} else if r.state == StateReady && r.image != nil {
		s.totalBytes -= int64(r.image.PixelSize())
		s.totalUpload -= int64(r.image.UploadSize())
		r.image = nil
	}
	r.state = StateLoading
	r.failure = nil
	s.requests++
	s.mu.Unlock()
	go func() {
		if len(payload) > MaxStreamAssetBytes {
			s.mu.Lock()
			if cur, ok := s.recs[id]; ok && cur.state == StateLoading {
				s.storeFailureLocked(id, core.OutOfMemory(op, string(id)))
			}
			s.mu.Unlock()
			return
		}
		im, derr := ParseKTX2(payload)
		s.mu.Lock()
		defer s.mu.Unlock()
		cur, ok := s.recs[id]
		if !ok || cur.state != StateLoading {
			return
		}
		if derr != nil {
			s.storeFailureLocked(id, derr)
			return
		}
		if s.totalBytes+int64(im.PixelSize()) > MaxStreamBytes {
			s.storeFailureLocked(id, core.OutOfMemory(op, string(id)))
			return
		}
		s.storeReadyLocked(id, im)
	}()
	return nil
}

// RequestFile starts a background load of path under id. Missing files
// surface as Missing through Poll/Wait, never as a Request error.
func (s *Stream) RequestFile(id core.AssetID, path string) error {
	const op = "tex.Stream.RequestFile"
	if s == nil {
		return core.InvalidArg(op, "stream")
	}
	if err := checkStreamID(id); err != nil {
		return err
	}
	if path == "" {
		return core.InvalidArg(op, string(id))
	}
	clean := filepath.Clean(path)
	s.mu.Lock()
	if r, ok := s.recs[id]; ok && r.state == StateLoading {
		s.mu.Unlock()
		return nil
	}
	if _, ok := s.recs[id]; !ok && len(s.recs) >= MaxStreams {
		s.mu.Unlock()
		return core.OutOfMemory(op, string(id))
	}
	r, ok := s.recs[id]
	if !ok {
		r = &streamRecord{}
		s.recs[id] = r
	} else if r.state == StateReady && r.image != nil {
		s.totalBytes -= int64(r.image.PixelSize())
		s.totalUpload -= int64(r.image.UploadSize())
		r.image = nil
	}
	r.state = StateLoading
	r.failure = nil
	s.requests++
	s.mu.Unlock()
	go func() {
		raw, ferr := os.ReadFile(clean)
		if ferr != nil {
			s.mu.Lock()
			if cur, ok := s.recs[id]; ok && cur.state == StateLoading {
				s.storeFailureLocked(id, core.NotFound(op, string(id), ferr))
			}
			s.mu.Unlock()
			return
		}
		if len(raw) == 0 {
			s.mu.Lock()
			if cur, ok := s.recs[id]; ok && cur.state == StateLoading {
				s.storeFailureLocked(id, core.InvalidArg(op, string(id)))
			}
			s.mu.Unlock()
			return
		}
		if len(raw) > MaxStreamAssetBytes {
			s.mu.Lock()
			if cur, ok := s.recs[id]; ok && cur.state == StateLoading {
				s.storeFailureLocked(id, core.OutOfMemory(op, string(id)))
			}
			s.mu.Unlock()
			return
		}
		im, derr := ParseKTX2(raw)
		s.mu.Lock()
		defer s.mu.Unlock()
		cur, ok := s.recs[id]
		if !ok || cur.state != StateLoading {
			return
		}
		if derr != nil {
			s.storeFailureLocked(id, derr)
			return
		}
		if s.totalBytes+int64(im.PixelSize()) > MaxStreamBytes {
			s.storeFailureLocked(id, core.OutOfMemory(op, string(id)))
			return
		}
		s.storeReadyLocked(id, im)
	}()
	return nil
}

// Poll reports the lifecycle state of id without blocking. Unknown ids
// are StateEmpty plus NotFound; Loading and Ready report nil; Missing
// and Failed replay the stored cause.
func (s *Stream) Poll(id core.AssetID) (State, error) {
	const op = "tex.Stream.Poll"
	if s == nil {
		return StateEmpty, core.InvalidArg(op, "stream")
	}
	if err := checkStreamID(id); err != nil {
		return StateEmpty, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.recs[id]
	if !ok {
		return StateEmpty, core.NotFound(op, string(id))
	}
	switch r.state {
	case StateLoading, StateReady:
		return r.state, nil
	case StateMissing, StateFailed:
		if r.failure != nil {
			return r.state, r.failure
		}
		return r.state, core.NotFound(op, string(id))
	default:
		return StateEmpty, core.NotFound(op, string(id))
	}
}

// Wait blocks until id leaves Loading, then returns its image.
// Unknown ids return NotFound; Missing/Failed replay the cause.
// Images are immutable: the returned pointer stays valid.
// The wait caps at 5 seconds so a stuck worker cannot hang play.
func (s *Stream) Wait(id core.AssetID) (*Image, error) {
	const op = "tex.Stream.Wait"
	if s == nil {
		return PlaceholderImage(), core.InvalidArg(op, "stream")
	}
	if err := checkStreamID(id); err != nil {
		return PlaceholderImage(), err
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		s.mu.Lock()
		r, ok := s.recs[id]
		if !ok {
			s.mu.Unlock()
			return PlaceholderImage(), core.NotFound(op, string(id))
		}
		switch r.state {
		case StateReady:
			im := r.image
			s.mu.Unlock()
			if im == nil {
				return PlaceholderImage(), core.NotFound(op, string(id))
			}
			return im, nil
		case StateMissing, StateFailed:
			failure := r.failure
			s.mu.Unlock()
			if failure == nil {
				return PlaceholderImage(), core.NotFound(op, string(id))
			}
			return PlaceholderImage(), failure
		case StateLoading:
			s.mu.Unlock()
		default:
			s.mu.Unlock()
			return PlaceholderImage(), core.NotFound(op, string(id))
		}
		if time.Now().After(deadline) {
			return PlaceholderImage(), core.OutOfMemory(op, string(id))
		}
		time.Sleep(time.Millisecond)
	}
}

// Get returns the Ready image without blocking. Ready ids report
// (image, true); anything else reports (placeholder, false).
// Never nil, never panics.
func (s *Stream) Get(id core.AssetID) (*Image, bool) {
	if s == nil || id.Empty() {
		return PlaceholderImage(), false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, ok := s.recs[id]; ok && r.state == StateReady && r.image != nil {
		return r.image, true
	}
	return PlaceholderImage(), false
}

// Placeholder returns a usable stand-in for id so missing textures never
// branch on nil. Ready ids return their image; anything else returns the
// shared 4x4 magenta. Never nil.
func (s *Stream) Placeholder(id core.AssetID) *Image {
	im, ok := s.Get(id)
	if ok {
		return im
	}
	_ = id
	return PlaceholderImage()
}

// Ready reports whether id holds a decoded image.
func (s *Stream) Ready(id core.AssetID) bool {
	if s == nil || id.Empty() {
		return false
	}
	return s.StateOf(id) == StateReady
}

// StateOf returns the lifecycle state of id.
func (s *Stream) StateOf(id core.AssetID) State {
	if s == nil || id.Empty() {
		return StateEmpty
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, ok := s.recs[id]; ok {
		return r.state
	}
	return StateEmpty
}

// Ref adds one claim on a Ready id and returns its image.
// Loading ids report NotFound; Missing/Failed replay the cause.
func (s *Stream) Ref(id core.AssetID) (*Image, error) {
	const op = "tex.Stream.Ref"
	if s == nil {
		return PlaceholderImage(), core.InvalidArg(op, "stream")
	}
	if err := checkStreamID(id); err != nil {
		return PlaceholderImage(), err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.recs[id]
	if !ok {
		return PlaceholderImage(), core.NotFound(op, string(id))
	}
	switch r.state {
	case StateReady:
		if r.image == nil {
			return PlaceholderImage(), core.NotFound(op, string(id))
		}
		h, err := s.ledger.Acquire(id)
		if err != nil {
			return r.image, err
		}
		s.stacks[id] = append(s.stacks[id], h)
		return r.image, nil
	case StateLoading:
		return PlaceholderImage(), core.NotFound(op, string(id))
	case StateMissing, StateFailed:
		if r.failure != nil {
			return PlaceholderImage(), r.failure
		}
		return PlaceholderImage(), core.NotFound(op, string(id))
	default:
		return PlaceholderImage(), core.NotFound(op, string(id))
	}
}

// Unload drops one claim on id. Unknown ids are NotFound;
// ids with no live claims are InvalidArg and change nothing.
// Cached bytes stay for quick reload; use Evict to free them.
func (s *Stream) Unload(id core.AssetID) error {
	const op = "tex.Stream.Unload"
	if s == nil {
		return core.InvalidArg(op, "stream")
	}
	if err := checkStreamID(id); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.recs[id]; !ok {
		return core.NotFound(op, string(id))
	}
	stack := s.stacks[id]
	if len(stack) == 0 {
		return core.InvalidArg(op, string(id))
	}
	top := stack[len(stack)-1]
	s.stacks[id] = stack[:len(stack)-1]
	s.unloads++
	return top.Release()
}

// Evict frees the cached image of id when it has no live claims.
// Live ids report InvalidArg; unknown ids report NotFound.
func (s *Stream) Evict(id core.AssetID) error {
	const op = "tex.Stream.Evict"
	if s == nil {
		return core.InvalidArg(op, "stream")
	}
	if err := checkStreamID(id); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.recs[id]
	if !ok {
		return core.NotFound(op, string(id))
	}
	if len(s.stacks[id]) > 0 {
		return core.InvalidArg(op, string(id))
	}
	if r.state == StateReady && r.image != nil {
		s.totalBytes -= int64(r.image.PixelSize())
		s.totalUpload -= int64(r.image.UploadSize())
	}
	delete(s.recs, id)
	delete(s.stacks, id)
	return nil
}

// LiveCount returns the live reference count of id.
func (s *Stream) LiveCount(id core.AssetID) int {
	if s == nil || id.Empty() {
		return 0
	}
	return s.ledger.LiveCount(id)
}

// Stats returns a copy of the Stream counters.
func (s *Stream) Stats() Stats {
	if s == nil {
		return Stats{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, r := range s.recs {
		if r.state == StateReady {
			n++
		}
	}
	return Stats{
		Count:       n,
		TotalBytes:  s.totalBytes,
		TotalUpload: s.totalUpload,
		Loads:       s.loads,
		Unloads:     s.unloads,
		Requests:    s.requests,
	}
}

// Count returns the Ready entry count.
func (s *Stream) Count() int {
	if s == nil {
		return 0
	}
	return s.Stats().Count
}

// TotalBytes returns the retained decoded RGBA8 bytes.
func (s *Stream) TotalBytes() int64 {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.totalBytes
}

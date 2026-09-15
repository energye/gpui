package sprite

import (
	"github.com/energye/gpui/game/core"
)

// LoopMode selects what happens when a clip runs past its last frame.
type LoopMode int

const (
	// LoopOnce plays to the last frame, holds it, then stops.
	LoopOnce LoopMode = 0
	// LoopLoop wraps to the first frame and keeps playing.
	LoopLoop LoopMode = 1
	// LoopPingPong bounces at both ends: 0 1 2 3 2 1 0 1 ...
	LoopPingPong LoopMode = 2
)

// String returns the stable log name of m.
func (m LoopMode) String() string {
	switch m {
	case LoopOnce:
		return "once"
	case LoopLoop:
		return "loop"
	case LoopPingPong:
		return "pingpong"
	default:
		return "unknown"
	}
}

// Clip is one named action: idle, run, jump each own one. Frames holds the
// image ids in play order; FrameDur is how long each frame shows;
// Loop selects once, loop, or pingpong. Only core numbers are used;
// the caller draws the returned frame id with the existing render draws.
type Clip struct {
	Name     string
	Frames   []int
	FrameDur core.Duration
	Loop     LoopMode
}

// EventKind tells what an Update crossed: a new frame, a wrap or bounce,
// or the end of a once clip.
type EventKind int

const (
	// EventNone means the visible frame did not change.
	EventNone EventKind = 0
	// EventFrame means the frame advanced without wrapping or finishing.
	EventFrame EventKind = 1
	// EventLoop means a loop clip wrapped or a pingpong clip bounced.
	EventLoop EventKind = 2
	// EventFinished means a once clip reached its end and stopped.
	EventFinished EventKind = 3
)

// String returns the stable log name of k.
func (k EventKind) String() string {
	switch k {
	case EventFrame:
		return "frame"
	case EventLoop:
		return "loop"
	case EventFinished:
		return "finished"
	default:
		return "none"
	}
}

// Event is what happened in one Update: which clip, which image id,
// which position inside the clip, and what kind of crossing.
type Event struct {
	Clip  string
	Frame int
	Index int
	Kind  EventKind
}

// Flipbook plays frame numbers over time: Play picks the action,
// Update advances by real time, OnEvent reports crossings.
// It draws nothing; the caller draws the returned frame id.
type Flipbook struct {
	clips   map[string]Clip
	cur     string
	hasCur  bool
	pos     int
	elapsed core.Duration
	playing bool
	dir     int
	onEvent func(Event)
}

func validLoop(m LoopMode) bool {
	return m == LoopOnce || m == LoopLoop || m == LoopPingPong
}

// NewFlipbook builds an empty action library. Clips arrive via AddClip,
// the current action via Play.
func NewFlipbook() *Flipbook {
	return &Flipbook{clips: map[string]Clip{}, dir: 1}
}

// AddClip stores one action. Name must be non-empty and unique, Frames
// must hold at least one non-negative image id, FrameDur must be positive,
// Loop must be once, loop, or pingpong. Bad clips return a core
// InvalidArg error and store nothing; the frame list is copied so later
// writes by the caller cannot change the library.
func (f *Flipbook) AddClip(c Clip) error {
	if f == nil {
		return core.InvalidArg("sprite.AddClip", "flipbook")
	}
	if c.Name == "" {
		return core.InvalidArg("sprite.AddClip", "name")
	}
	if len(c.Frames) == 0 {
		return core.InvalidArg("sprite.AddClip", "frames")
	}
	for _, fr := range c.Frames {
		if fr < 0 {
			return core.InvalidArg("sprite.AddClip", "frames")
		}
	}
	if c.FrameDur <= 0 {
		return core.InvalidArg("sprite.AddClip", "frameDur")
	}
	if !validLoop(c.Loop) {
		return core.InvalidArg("sprite.AddClip", "loop")
	}
	if _, dup := f.clips[c.Name]; dup {
		return core.InvalidArg("sprite.AddClip", "name")
	}
	cp := make([]int, len(c.Frames))
	copy(cp, c.Frames)
	f.clips[c.Name] = Clip{Name: c.Name, Frames: cp, FrameDur: c.FrameDur, Loop: c.Loop}
	return nil
}

// Has reports whether the named action exists.
func (f *Flipbook) Has(name string) bool {
	if f == nil {
		return false
	}
	_, ok := f.clips[name]
	return ok
}

// Count returns how many actions the library holds.
func (f *Flipbook) Count() int {
	if f == nil {
		return 0
	}
	return len(f.clips)
}

// OnEvent sets the crossing callback; nil clears it. The handler runs
// synchronously inside Update and must not call back into the same
// Flipbook.
func (f *Flipbook) OnEvent(h func(Event)) {
	if f == nil {
		return
	}
	f.onEvent = h
}

// Play picks the named action and rewinds to its first frame.
// Unknown names return core NotFound, empty names core InvalidArg;
// either way the current action and position stay untouched.
func (f *Flipbook) Play(name string) error {
	if f == nil {
		return core.InvalidArg("sprite.Play", "flipbook")
	}
	if name == "" {
		return core.InvalidArg("sprite.Play", "name")
	}
	if _, ok := f.clips[name]; !ok {
		return core.NotFound("sprite.Play", name)
	}
	f.cur = name
	f.hasCur = true
	f.pos = 0
	f.elapsed = 0
	f.playing = true
	f.dir = 1
	return nil
}

// Current returns the current action, position, and image id.
// ok is false when Play never succeeded.
func (f *Flipbook) Current() (clip string, index int, frame int, ok bool) {
	if f == nil || !f.hasCur {
		return "", 0, 0, false
	}
	c := f.clips[f.cur]
	if f.pos < 0 || f.pos >= len(c.Frames) {
		return f.cur, 0, 0, false
	}
	return f.cur, f.pos, c.Frames[f.pos], true
}

// IsPlaying reports whether time advances the current action.
// A finished once clip reports false until Play rewinds it.
func (f *Flipbook) IsPlaying() bool {
	if f == nil {
		return false
	}
	return f.hasCur && f.playing
}

func (f *Flipbook) emit(ev Event) {
	if ev.Kind != EventNone && f.onEvent != nil {
		f.onEvent(ev)
	}
}

// Update moves the playhead by dt and returns the visible image id.
// dt at or below zero advances nothing and reports EventNone.
// With no current action it returns ok=false and emits nothing.
// Large dt coalesces to one event: finished beats loop beats frame.
func (f *Flipbook) Update(dt core.Duration) (frame int, ev Event, ok bool) {
	if f == nil || !f.hasCur {
		return 0, Event{}, false
	}
	c := f.clips[f.cur]
	curFrame := c.Frames[f.pos]
	none := Event{Clip: f.cur, Frame: curFrame, Index: f.pos, Kind: EventNone}
	if !f.playing {
		return curFrame, none, true
	}
	if dt <= 0 {
		return curFrame, none, true
	}
	n := len(c.Frames)
	f.elapsed += dt
	steps := int64(f.elapsed / c.FrameDur)
	rest := f.elapsed % c.FrameDur
	if steps == 0 {
		return curFrame, none, true
	}
	// A single-image loop or pingpong clip never moves: every crossing
	// is a loop event on the same frame. Once keeps its own path below
	// (it must finish instead of looping).
	if n == 1 && (c.Loop == LoopLoop || c.Loop == LoopPingPong) {
		f.elapsed = rest
		ev = Event{Clip: f.cur, Frame: c.Frames[f.pos], Index: f.pos, Kind: EventLoop}
		f.emit(ev)
		return ev.Frame, ev, true
	}
	switch c.Loop {
	case LoopOnce:
		if int64(f.pos)+steps < int64(n) {
			f.pos += int(steps)
			f.elapsed = rest
			ev = Event{Clip: f.cur, Frame: c.Frames[f.pos], Index: f.pos, Kind: EventFrame}
			f.emit(ev)
			return ev.Frame, ev, true
		}
		f.pos = n - 1
		f.elapsed = 0
		f.playing = false
		ev = Event{Clip: f.cur, Frame: c.Frames[f.pos], Index: f.pos, Kind: EventFinished}
		f.emit(ev)
		return ev.Frame, ev, true
	case LoopLoop:
		total := int64(f.pos) + steps
		loops := total / int64(n)
		f.pos = int(total % int64(n))
		f.elapsed = rest
		kind := EventFrame
		if loops > 0 {
			kind = EventLoop
		}
		ev = Event{Clip: f.cur, Frame: c.Frames[f.pos], Index: f.pos, Kind: kind}
		f.emit(ev)
		return ev.Frame, ev, true
	case LoopPingPong:
		period := int64(2*n - 2)
		phase := pingPhase(f.pos, f.dir, n)
		bounced := hitEnd(phase, steps, period, int64(n-1))
		newPhase := (phase + steps) % period
		pos, dir := pingPosDir(newPhase, n)
		f.pos = pos
		f.dir = dir
		f.elapsed = rest
		kind := EventFrame
		if bounced {
			kind = EventLoop
		}
		ev = Event{Clip: f.cur, Frame: c.Frames[f.pos], Index: f.pos, Kind: kind}
		f.emit(ev)
		return ev.Frame, ev, true
	default:
		f.elapsed -= dt
		return curFrame, none, true
	}
}

// pingPhase maps the (pos, dir) pair onto the linear bounce phase in
// [0, 2n-2): forward run 0..n-1, backward run n..2n-3.
func pingPhase(pos, dir, n int) int64 {
	if n <= 1 {
		return 0
	}
	period := int64(2*n - 2)
	if dir >= 0 {
		return int64(pos)
	}
	if pos == 0 {
		return 0
	}
	return period - int64(pos)
}

// pingPosDir maps a linear phase back to (pos, next dir).
func pingPosDir(phase int64, n int) (int, int) {
	if n <= 1 {
		return 0, 1
	}
	if phase < int64(n) {
		pos := int(phase)
		if pos >= n-1 {
			return pos, -1
		}
		return pos, 1
	}
	period := int64(2*n - 2)
	pos := int(period - phase)
	if pos <= 0 {
		return 0, 1
	}
	return pos, -1
}

// hitEnd reports whether the open run (phase, phase+steps] touches either
// bounce end (phase 0 or phase n-1, modulo the period). A run covering a
// whole period touches both ends, so it returns early; the floor division
// below only handles the shorter remainder and stays exact for it.
func hitEnd(phase, steps, period, last int64) bool {
	if steps <= 0 {
		return false
	}
	if steps >= period {
		return true
	}
	for _, r := range []int64{0, last} {
		if floorDiv(phase+steps-r, period)-floorDiv(phase-r, period) > 0 {
			return true
		}
	}
	return false
}

func floorDiv(a, b int64) int64 {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

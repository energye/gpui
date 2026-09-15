package input

import (
	"sort"

	"github.com/energye/gpui/game/core"
)

// Frozen buffer budgets. Push beyond MaxBuffered drops the oldest press
// and keeps the newest, so long runs never grow.
const (
	// MaxBuffered caps stored presses per Buffer.
	MaxBuffered = 64
	// MaxTouches caps simultaneous active touches per TouchTracker.
	MaxTouches = 10
	// DefaultBufferWindow is the combo cache window (ms).
	DefaultBufferWindow core.Duration = 150
)

// BufferedPress is one cached action press: action name plus push time.
type BufferedPress struct {
	Action string
	At     core.Duration
}

// Buffer caches recent action presses so a combo hit slightly early is
// not lost. Presses expire window after push; Consume takes the oldest
// live match. It stores strings only and draws nothing.
type Buffer struct {
	window  core.Duration
	presses []BufferedPress
}

// NewBuffer builds an empty buffer with the given cache window.
// Window must be > 0; bad windows are core InvalidArg with nil buffer.
func NewBuffer(window core.Duration) (*Buffer, error) {
	if window <= 0 {
		return nil, core.InvalidArg("input.NewBuffer", "window")
	}
	return &Buffer{window: window}, nil
}

// Window returns the cache window. Nil buffers return 0.
func (b *Buffer) Window() core.Duration {
	if b == nil {
		return 0
	}
	return b.window
}

// SetWindow retunes the cache window. Bad windows are core InvalidArg
// and leave the old window untouched.
func (b *Buffer) SetWindow(window core.Duration) error {
	if b == nil {
		return core.InvalidArg("input.SetWindow", "buffer")
	}
	if window <= 0 {
		return core.InvalidArg("input.SetWindow", "window")
	}
	b.window = window
	return nil
}

// Push caches one action press at time at. Empty actions and negative
// times are core InvalidArg and store nothing. A full buffer drops the
// oldest press to make room and still stores the new one.
func (b *Buffer) Push(action string, at core.Duration) error {
	if b == nil {
		return core.InvalidArg("input.Push", "buffer")
	}
	if action == "" {
		return core.InvalidArg("input.Push", "action")
	}
	if at < 0 {
		return core.InvalidArg("input.Push", "at")
	}
	if len(b.presses) >= MaxBuffered {
		copy(b.presses, b.presses[1:])
		b.presses = b.presses[:len(b.presses)-1]
	}
	b.presses = append(b.presses, BufferedPress{Action: action, At: at})
	return nil
}

// pruneLocked drops presses with now-at > window. Future presses stay.
func (b *Buffer) pruneLocked(now core.Duration) {
	kept := b.presses[:0]
	for _, p := range b.presses {
		if p.At > now {
			kept = append(kept, p)
			continue
		}
		if now-p.At <= b.window {
			kept = append(kept, p)
		}
	}
	b.presses = kept
}

// Prune drops expired presses at time now. Nil buffers and negative
// times do nothing.
func (b *Buffer) Prune(now core.Duration) {
	if b == nil || now < 0 {
		return
	}
	b.pruneLocked(now)
}

// liveLocked reports whether p is consumable at now.
func (b *Buffer) liveLocked(p BufferedPress, now core.Duration) bool {
	if p.At > now {
		return false
	}
	return now-p.At <= b.window
}

// Consume takes the oldest live press for action at time now. Hit returns
// true; miss returns false with no error. Empty actions and negative
// times are core InvalidArg with false and change nothing.
func (b *Buffer) Consume(action string, now core.Duration) (bool, error) {
	if b == nil {
		return false, core.InvalidArg("input.Consume", "buffer")
	}
	if action == "" {
		return false, core.InvalidArg("input.Consume", "action")
	}
	if now < 0 {
		return false, core.InvalidArg("input.Consume", "now")
	}
	b.pruneLocked(now)
	for i, p := range b.presses {
		if p.Action == action && b.liveLocked(p, now) {
			b.presses = append(b.presses[:i], b.presses[i+1:]...)
			return true, nil
		}
	}
	return false, nil
}

// Peek reports whether a live press for action exists at now without
// removing it. Validation matches Consume.
func (b *Buffer) Peek(action string, now core.Duration) (bool, error) {
	if b == nil {
		return false, core.InvalidArg("input.Peek", "buffer")
	}
	if action == "" {
		return false, core.InvalidArg("input.Peek", "action")
	}
	if now < 0 {
		return false, core.InvalidArg("input.Peek", "now")
	}
	for _, p := range b.presses {
		if p.Action == action && b.liveLocked(p, now) {
			return true, nil
		}
	}
	return false, nil
}

// Len returns how many presses are stored (including expired not yet
// pruned). Nil buffers return 0.
func (b *Buffer) Len() int {
	if b == nil {
		return 0
	}
	return len(b.presses)
}

// LiveCount counts consumable presses at now. Nil buffers and negative
// times return 0.
func (b *Buffer) LiveCount(now core.Duration) int {
	if b == nil || now < 0 {
		return 0
	}
	n := 0
	for _, p := range b.presses {
		if b.liveLocked(p, now) {
			n++
		}
	}
	return n
}

// Clear drops every stored press. Nil buffers do nothing.
func (b *Buffer) Clear() {
	if b == nil {
		return
	}
	b.presses = nil
}

// Presses returns a copy of the stored presses. Writing the result
// cannot change the buffer. Nil buffers return nil.
func (b *Buffer) Presses() []BufferedPress {
	if b == nil {
		return nil
	}
	out := make([]BufferedPress, len(b.presses))
	copy(out, b.presses)
	return out
}

// Touch is one tracked finger: id plus last position plus update time.
type Touch struct {
	ID  int
	Pos core.Vec2
	At  core.Duration
}

type touchEntry struct {
	pos core.Vec2
	at  core.Duration
}

// TouchTracker tracks live touch ids to positions. Ids are caller
// assigned (>= 0); positions are game units. It draws nothing.
type TouchTracker struct {
	live map[int]touchEntry
}

// NewTouchTracker builds an empty tracker.
func NewTouchTracker() *TouchTracker {
	return &TouchTracker{live: map[int]touchEntry{}}
}

func checkTouchPos(pos core.Vec2) error {
	if !finite(pos.X) || !finite(pos.Y) {
		return core.InvalidArg("input.Touch", "pos")
	}
	return nil
}

// Begin starts tracking id at pos. Duplicate ids are core InvalidArg;
// a full tracker (MaxTouches live) is core OutOfMemory; either way the
// live set stays untouched.
func (t *TouchTracker) Begin(id int, pos core.Vec2, at core.Duration) error {
	if t == nil {
		return core.InvalidArg("input.Begin", "tracker")
	}
	if id < 0 {
		return core.InvalidArg("input.Begin", "id")
	}
	if err := checkTouchPos(pos); err != nil {
		return err
	}
	if at < 0 {
		return core.InvalidArg("input.Begin", "at")
	}
	if _, dup := t.live[id]; dup {
		return core.InvalidArg("input.Begin", "id")
	}
	if len(t.live) >= MaxTouches {
		return core.OutOfMemory("input.Begin", "touches")
	}
	t.live[id] = touchEntry{pos: pos, at: at}
	return nil
}

// Move updates the position of a live id. Unknown ids are core NotFound
// and change nothing.
func (t *TouchTracker) Move(id int, pos core.Vec2, at core.Duration) error {
	if t == nil {
		return core.InvalidArg("input.Move", "tracker")
	}
	if id < 0 {
		return core.InvalidArg("input.Move", "id")
	}
	if err := checkTouchPos(pos); err != nil {
		return err
	}
	if at < 0 {
		return core.InvalidArg("input.Move", "at")
	}
	if _, ok := t.live[id]; !ok {
		return core.NotFound("input.Move", "touch")
	}
	t.live[id] = touchEntry{pos: pos, at: at}
	return nil
}

// End stops tracking id (lift or interrupt). Unknown ids are core
// NotFound and change nothing, never a panic.
func (t *TouchTracker) End(id int, at core.Duration) error {
	if t == nil {
		return core.InvalidArg("input.End", "tracker")
	}
	if id < 0 {
		return core.InvalidArg("input.End", "id")
	}
	if at < 0 {
		return core.InvalidArg("input.End", "at")
	}
	if _, ok := t.live[id]; !ok {
		return core.NotFound("input.End", "touch")
	}
	delete(t.live, id)
	return nil
}

// Position returns the last position of id. Unknown ids and nil trackers
// report ok=false.
func (t *TouchTracker) Position(id int) (core.Vec2, bool) {
	if t == nil || id < 0 {
		return core.Vec2{}, false
	}
	e, ok := t.live[id]
	if !ok {
		return core.Vec2{}, false
	}
	return e.pos, true
}

// ActiveCount returns how many touches are live. Nil trackers return 0.
func (t *TouchTracker) ActiveCount() int {
	if t == nil {
		return 0
	}
	return len(t.live)
}

// ActiveIDs returns the live ids in sorted order. The result is fresh;
// nil trackers return nil.
func (t *TouchTracker) ActiveIDs() []int {
	if t == nil {
		return nil
	}
	out := make([]int, 0, len(t.live))
	for id := range t.live {
		out = append(out, id)
	}
	sort.Ints(out)
	return out
}

// Touches returns the live touches sorted by id. The result is fresh;
// nil trackers return nil.
func (t *TouchTracker) Touches() []Touch {
	if t == nil {
		return nil
	}
	out := make([]Touch, 0, len(t.live))
	for id, e := range t.live {
		out = append(out, Touch{ID: id, Pos: e.pos, At: e.at})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Clear drops every live touch. Nil trackers do nothing.
func (t *TouchTracker) Clear() {
	if t == nil {
		return
	}
	t.live = map[int]touchEntry{}
}

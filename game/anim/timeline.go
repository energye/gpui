package anim

import (
	"sort"

	"github.com/energye/gpui/game/core"
)

// LoopMode selects what happens when the head runs past the duration.
type LoopMode int

const (
	// LoopOnce holds the last frame and stops at the end.
	LoopOnce LoopMode = 0
	// LoopLoop wraps to the start and keeps playing.
	LoopLoop LoopMode = 1
	// LoopPingPong bounces at both ends and keeps playing.
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

// Key is one keyframe: Value at At, eased toward the next key with Ease.
// Ease rides on the starting key of each segment; the last key holds.
type Key struct {
	At    core.Duration
	Value float64
	Ease  Kind
}

// Event is one fired cue: Name crossed at At. Names are caller-owned
// (slash, sfx, call, show); the engine never hardcodes user text.
type Event struct {
	At   core.Duration
	Name string
}

type eventRec struct {
	at   core.Duration
	name string
	seq  int
}

// Timeline plays float tracks over a fixed duration plus a cue list.
// It only computes numbers; the caller feeds samples into existing draws
// and maps event names to slash, sound, method, or visibility actions.
// Time is integer milliseconds, so long runs never drift.
type Timeline struct {
	dur     core.Duration
	loop    LoopMode
	tracks  map[string][]Key
	events  []eventRec
	seq     int
	pos     core.Duration
	dir     int
	playing bool
	onEvent func(Event)
}

func validWrap(m LoopMode) bool {
	return m == LoopOnce || m == LoopLoop || m == LoopPingPong
}

// NewTimeline builds an empty timeline of dur. dur must be positive;
// bad durations return a core InvalidArg error and nil. The head starts
// at 0, forward, playing.
func NewTimeline(dur core.Duration) (*Timeline, error) {
	if dur <= 0 {
		return nil, core.InvalidArg("anim.NewTimeline", "duration")
	}
	return &Timeline{
		dur:     dur,
		loop:    LoopOnce,
		tracks:  map[string][]Key{},
		dir:     1,
		playing: true,
	}, nil
}

// Duration returns the timeline length, or 0 on a nil timeline.
func (t *Timeline) Duration() core.Duration {
	if t == nil {
		return 0
	}
	return t.dur
}

// Loop returns the wrap mode, or LoopOnce on a nil timeline.
func (t *Timeline) Loop() LoopMode {
	if t == nil {
		return LoopOnce
	}
	return t.loop
}

// SetLoop picks the wrap mode. Bad modes return core InvalidArg and keep
// the old mode; a nil timeline also returns InvalidArg.
func (t *Timeline) SetLoop(m LoopMode) error {
	if t == nil {
		return core.InvalidArg("anim.SetLoop", "timeline")
	}
	if !validWrap(m) {
		return core.InvalidArg("anim.SetLoop", "loop")
	}
	t.loop = m
	return nil
}

// AddKey stores one keyframe on track. track must be non-empty, At must
// land in [0, duration], At must be unused on that track, Ease must name
// a frozen curve. Bad keys return core InvalidArg and store nothing.
func (t *Timeline) AddKey(track string, k Key) error {
	if t == nil {
		return core.InvalidArg("anim.AddKey", "timeline")
	}
	if track == "" {
		return core.InvalidArg("anim.AddKey", "track")
	}
	if k.At < 0 || k.At > t.dur {
		return core.InvalidArg("anim.AddKey", "at")
	}
	if !Valid(k.Ease) {
		return core.InvalidArg("anim.AddKey", "ease")
	}
	keys := t.tracks[track]
	at := sort.Search(len(keys), func(i int) bool { return keys[i].At >= k.At })
	if at < len(keys) && keys[at].At == k.At {
		return core.InvalidArg("anim.AddKey", "at")
	}
	keys = append(keys, Key{})
	copy(keys[at+1:], keys[at:])
	keys[at] = k
	t.tracks[track] = keys
	return nil
}

// AddEvent stores one cue. At must land in [0, duration], name must be
// non-empty; same-At cues keep insertion order. Bad cues return core
// InvalidArg and store nothing.
func (t *Timeline) AddEvent(at core.Duration, name string) error {
	if t == nil {
		return core.InvalidArg("anim.AddEvent", "timeline")
	}
	if name == "" {
		return core.InvalidArg("anim.AddEvent", "name")
	}
	if at < 0 || at > t.dur {
		return core.InvalidArg("anim.AddEvent", "at")
	}
	atIdx := sort.Search(len(t.events), func(i int) bool { return t.events[i].at > at })
	t.events = append(t.events, eventRec{})
	copy(t.events[atIdx+1:], t.events[atIdx:])
	t.events[atIdx] = eventRec{at: at, name: name, seq: t.seq}
	t.seq++
	return nil
}

// TrackNames returns every track with at least one key, sorted.
func (t *Timeline) TrackNames() []string {
	if t == nil {
		return nil
	}
	out := make([]string, 0, len(t.tracks))
	for name := range t.tracks {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// KeyCount returns how many keys track holds, or 0 when unknown.
func (t *Timeline) KeyCount(track string) int {
	if t == nil {
		return 0
	}
	return len(t.tracks[track])
}

// EventCount returns how many cues the timeline holds.
func (t *Timeline) EventCount() int {
	if t == nil {
		return 0
	}
	return len(t.events)
}

// OnEvent sets the cue callback; nil clears it. The handler runs
// synchronously inside Update and must not call back into the same
// Timeline.
func (t *Timeline) OnEvent(h func(Event)) {
	if t == nil {
		return
	}
	t.onEvent = h
}

// Pos returns the head position. A nil timeline reports 0.
func (t *Timeline) Pos() core.Duration {
	if t == nil {
		return 0
	}
	return t.pos
}

// IsPlaying reports whether Update advances the head.
// A finished once timeline reports false until Seek plus Resume.
func (t *Timeline) IsPlaying() bool {
	if t == nil {
		return false
	}
	return t.playing
}

// Pause freezes the head; Resume restarts it. Both are nil-safe.
// Resume on a finished once timeline replays from the end (no rewind);
// Seek first to play again.
func (t *Timeline) Pause() {
	if t == nil {
		return
	}
	t.playing = false
}

// Resume restarts a paused head without moving it.
func (t *Timeline) Resume() {
	if t == nil {
		return
	}
	t.playing = true
}

// Seek moves the head to at, clamped to [0, duration], facing forward.
// It keeps the playing state and fires nothing. A nil timeline returns
// core InvalidArg.
func (t *Timeline) Seek(at core.Duration) error {
	if t == nil {
		return core.InvalidArg("anim.Seek", "timeline")
	}
	t.pos = at.Clamp(0, t.dur)
	t.dir = 1
	return nil
}

// wrapRead maps an absolute time to a readable position: once clamps,
// loop wraps modulo the duration, pingpong triangle-bounces with period
// 2*duration. Negative times park at 0.
func (t *Timeline) wrapRead(at core.Duration) core.Duration {
	if at < 0 {
		return 0
	}
	switch t.loop {
	case LoopLoop:
		if at >= t.dur {
			return at % t.dur
		}
		return at
	case LoopPingPong:
		period := 2 * t.dur
		m := at % period
		if m > t.dur {
			return period - m
		}
		return m
	default:
		return at.Clamp(0, t.dur)
	}
}

// Sample reads one track at an absolute time without moving the head or
// firing cues. ok is false for a nil timeline or an unknown track.
func (t *Timeline) Sample(track string, at core.Duration) (float64, bool) {
	if t == nil {
		return 0, false
	}
	keys, ok := t.tracks[track]
	if !ok || len(keys) == 0 {
		return 0, false
	}
	return sampleKeys(keys, t.wrapRead(at)), true
}

func sampleKeys(keys []Key, w core.Duration) float64 {
	if w <= keys[0].At {
		return keys[0].Value
	}
	if w >= keys[len(keys)-1].At {
		return keys[len(keys)-1].Value
	}
	i := sort.Search(len(keys), func(i int) bool { return keys[i].At > w })
	k0, k1 := keys[i-1], keys[i]
	span := k1.At - k0.At
	tt := float64(w-k0.At) / float64(span)
	return k0.Value + (k1.Value-k0.Value)*Ease(k0.Ease, tt)
}

func (t *Timeline) emitAll(fired []Event) {
	if t.onEvent == nil {
		return
	}
	for _, ev := range fired {
		t.onEvent(ev)
	}
}

// Update moves the head by dt and returns the cues crossed in order.
// dt at or below zero, or a paused head, advances nothing and returns
// nil. A finished once head stays at the end and returns nil.
// An Update covering a full period or more reports each cue once in
// crossing order instead of repeating it unboundedly. A pingpong head
// crossing the same cue in both directions inside one Update reports it
// twice, once per crossing.
func (t *Timeline) Update(dt core.Duration) ([]Event, bool) {
	if t == nil {
		return nil, false
	}
	if dt <= 0 || !t.playing {
		return nil, true
	}
	var fired []Event
	switch t.loop {
	case LoopLoop:
		fired = t.stepLoop(dt)
	case LoopPingPong:
		fired = t.stepPingPong(dt)
	default:
		fired = t.stepOnce(dt)
	}
	t.emitAll(fired)
	if len(fired) == 0 {
		return nil, true
	}
	return fired, true
}

func (t *Timeline) stepOnce(dt core.Duration) []Event {
	newPos := t.pos + dt
	if newPos > t.dur {
		newPos = t.dur
	}
	var fired []Event
	for _, r := range t.events {
		if r.at > t.pos && r.at <= newPos {
			fired = append(fired, Event{At: r.at, Name: r.name})
		}
	}
	t.pos = newPos
	if t.pos == t.dur {
		t.playing = false
	}
	return fired
}

func (t *Timeline) stepLoop(dt core.Duration) []Event {
	total := t.pos + dt
	loops := total / t.dur
	t.pos = total % t.dur
	if loops == 0 {
		old := total - dt
		var fired []Event
		for _, r := range t.events {
			if r.at > old && r.at <= t.pos {
				fired = append(fired, Event{At: r.at, Name: r.name})
			}
		}
		return fired
	}
	// Full period or more: every cue once, ordered from after the old head.
	old := total - dt
	idx := sort.Search(len(t.events), func(i int) bool { return t.events[i].at > old })
	fired := make([]Event, 0, len(t.events))
	for _, r := range t.events[idx:] {
		fired = append(fired, Event{At: r.at, Name: r.name})
	}
	for _, r := range t.events[:idx] {
		fired = append(fired, Event{At: r.at, Name: r.name})
	}
	return fired
}

// cuePhases maps a cue time to its head-phase positions: interior cues
// exist twice per bounce period (forward and back), ends exist once.
func cuePhases(at, dur core.Duration) []core.Duration {
	if at == 0 || at == dur {
		return []core.Duration{at}
	}
	return []core.Duration{at, 2*dur - at}
}

func (t *Timeline) stepPingPong(dt core.Duration) []Event {
	period := 2 * t.dur
	var p core.Duration
	if t.dir >= 0 {
		p = t.pos
	} else {
		p = period - t.pos
	}
	steps := dt
	newPhase := (p + steps) % period
	type cross struct {
		abs int64
		seq int
		ev  Event
	}
	var cs []cross
	pi, si, peri := int64(p), int64(steps), int64(period)
	if si >= peri {
		for _, r := range t.events {
			best := peri
			for _, ph := range cuePhases(r.at, t.dur) {
				d := (int64(ph) - pi) % peri
				if d < 0 {
					d += peri
				}
				if d == 0 {
					d = peri
				}
				if d < best {
					best = d
				}
			}
			cs = append(cs, cross{abs: best, seq: r.seq, ev: Event{At: r.at, Name: r.name}})
		}
	} else {
		for _, r := range t.events {
			for _, ph := range cuePhases(r.at, t.dur) {
				phi := int64(ph)
				k := int64(0)
				if phi <= pi {
					k = (pi-phi)/peri + 1
				}
				if abs := phi + k*peri; abs <= pi+si {
					cs = append(cs, cross{abs: abs, seq: r.seq, ev: Event{At: r.at, Name: r.name}})
				}
			}
		}
	}
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].abs != cs[j].abs {
			return cs[i].abs < cs[j].abs
		}
		return cs[i].seq < cs[j].seq
	})
	var fired []Event
	for _, c := range cs {
		fired = append(fired, c.ev)
	}
	if newPhase <= t.dur {
		t.pos, t.dir = newPhase, 1
	} else {
		t.pos, t.dir = period-newPhase, -1
	}
	return fired
}

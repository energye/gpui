//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package audio

import (
	"math"
	"sort"

	"github.com/energye/gpui/engine/core"
)

// Large-voice reuse budget (S89, N6 subset).
//
// 128 requests feed 64 channels, the loudest 32 stay audible. The keep
// count matches DefaultMaxVoices so the mixer ceiling never moves.
const (
	// MaxVoiceRequests bounds one frame of voice asks.
	MaxVoiceRequests = 128
	// MaxVoiceChannels bounds simultaneous channels.
	MaxVoiceChannels = 64
	// AudibleVoices keeps the loudest channels.
	AudibleVoices = DefaultMaxVoices
)

// Beat clock limits: musical range only, caller owns tempo meaning.
const (
	// MinBPM floors usable tempos.
	MinBPM = 30
	// MaxBPM ceilings usable tempos.
	MaxBPM = 300
	// MinBeatsPerBar floors time signatures.
	MinBeatsPerBar = 1
	// MaxBeatsPerBar ceilings time signatures.
	MaxBeatsPerBar = 12
)

// VoiceRequest is one voice ask: where it plays plus how loud it is and
// whether it is an explosion that should duck music. Level is the base
// voice level in [0,1] before distance hush. Names stay out: the caller
// maps indices back to its own assets.
type VoiceRequest struct {
	Sound     PosSound
	Level     float64
	Explosive bool
}

// NewVoiceRequest builds one ask. Sound must come from NewPosSound,
// Level must be finite in [0,1]. Bad inputs are InvalidArg.
func NewVoiceRequest(sound PosSound, level float64, explosive bool) (VoiceRequest, error) {
	const op = "audio.NewVoiceRequest"
	if !sound.valid() {
		return VoiceRequest{}, core.InvalidArg(op, "sound")
	}
	if !finite(level) || level < 0 || level > 1 {
		return VoiceRequest{}, core.InvalidArg(op, "level")
	}
	return VoiceRequest{Sound: sound, Level: level, Explosive: explosive}, nil
}

// valid reports whether r could have come from NewVoiceRequest.
func (r VoiceRequest) valid() bool {
	return r.Sound.valid() && finite(r.Level) && r.Level >= 0 && r.Level <= 1
}

// VoicePick is the per-request verdict: Loud ranks, Gain/Pan feed the
// backend, Audible tells whether this frame emits samples.
type VoicePick struct {
	Index   int
	Loud    float64
	Gain    float64
	Pan     float64
	Audible bool
}

// VoiceStats sums one selection: counts plus the capped total.
type VoiceStats struct {
	Requested    int
	Channels     int
	Audible      int
	Total        float64
	Clipped      bool
	HasExplosion bool
}

// SelectVoices ranks reqs heard at listener and keeps the loudest.
// Loudness is mix gain times request level; out-of-range voices rank 0
// and never win a channel. The loudest MaxVoiceChannels take channels,
// the loudest AudibleVoices of those stay audible. The input is never
// mutated; the result is fresh every call. A bad listener returns
// ok=false with silent picks. Nil or empty input returns empty picks
// with ok=true. Requests beyond MaxVoiceRequests still select: only the
// first MaxVoiceRequests rank, the rest park silent so long runs never
// grow.
func SelectVoices(listener core.Vec2, reqs []VoiceRequest) ([]VoicePick, VoiceStats, bool) {
	if !finiteVec(listener) {
		out := make([]VoicePick, len(reqs))
		for i := range out {
			out[i] = VoicePick{Index: i}
		}
		return out, VoiceStats{Requested: len(reqs)}, false
	}
	out := make([]VoicePick, len(reqs))
	for i := range out {
		out[i] = VoicePick{Index: i}
	}
	ranked := len(reqs)
	if ranked > MaxVoiceRequests {
		ranked = MaxVoiceRequests
	}
	type scored struct {
		idx  int
		loud float64
		gain float64
		pan  float64
	}
	scores := make([]scored, 0, ranked)
	for i := 0; i < ranked; i++ {
		r := reqs[i]
		if !r.valid() {
			continue
		}
		m, ok := r.Sound.Mix(listener)
		if !ok || !m.Audible {
			continue
		}
		loud := m.Gain * r.Level
		if !finite(loud) || loud <= 0 {
			continue
		}
		scores = append(scores, scored{idx: i, loud: loud, gain: loud, pan: m.Pan})
	}
	// Loudest first, index breaks ties so replays agree.
	sort.Slice(scores, func(a, b int) bool {
		if scores[a].loud != scores[b].loud {
			return scores[a].loud > scores[b].loud
		}
		return scores[a].idx < scores[b].idx
	})
	channels := len(scores)
	if channels > MaxVoiceChannels {
		channels = MaxVoiceChannels
	}
	audible := channels
	if audible > AudibleVoices {
		audible = AudibleVoices
	}
	kept := map[int]scored{}
	for k := 0; k < audible; k++ {
		kept[scores[k].idx] = scores[k]
	}
	sum := 0.0
	hasBoom := false
	for i := 0; i < ranked; i++ {
		if s, ok := kept[i]; ok {
			out[i] = VoicePick{Index: i, Loud: s.loud, Gain: s.gain, Pan: s.pan, Audible: true}
			sum += s.gain
			if reqs[i].Explosive {
				hasBoom = true
			}
		}
	}
	total := sum
	clipped := false
	if total > 1 {
		total = 1
		clipped = true
	}
	if !finite(total) {
		total = 0
		clipped = false
	}
	st := VoiceStats{
		Requested:    len(reqs),
		Channels:     channels,
		Audible:      audible,
		Total:        total,
		Clipped:      clipped,
		HasExplosion: hasBoom,
	}
	return out, st, true
}

// SelectAtListener ranks reqs heard at the current listener position.
// Nobody current returns ok=false with silent picks.
func SelectAtListener(l Listener2D, reqs []VoiceRequest) ([]VoicePick, VoiceStats, bool) {
	return SelectVoices(l.pos, reqs)
}

// FollowAndSelect glues l to cameraPos then ranks reqs there. It is the
// hear-point rider: the caller passes the camera effective center each
// frame, no camera type is imported. A nil listener or bad cameraPos
// returns ok=false with silent picks and moves nothing.
func FollowAndSelect(l *Listener2D, cameraPos core.Vec2, reqs []VoiceRequest) ([]VoicePick, VoiceStats, bool) {
	if l == nil {
		out := make([]VoicePick, len(reqs))
		for i := range out {
			out[i] = VoicePick{Index: i}
		}
		return out, VoiceStats{Requested: len(reqs)}, false
	}
	if err := l.FollowCamera(cameraPos); err != nil {
		out := make([]VoicePick, len(reqs))
		for i := range out {
			out[i] = VoicePick{Index: i}
		}
		return out, VoiceStats{Requested: len(reqs)}, false
	}
	return SelectVoices(l.pos, reqs)
}

// ApplyExplosionDuck holds music down while an audible explosion rings.
// It triggers d only when stats report an audible explosion; otherwise it
// is a no-op returning false. Nil duckers report InvalidArg and change
// nothing. Strength must be finite in [0,1].
func ApplyExplosionDuck(d *Ducker, stats VoiceStats, strength float64) (bool, error) {
	const op = "audio.ApplyExplosionDuck"
	if d == nil {
		return false, core.InvalidArg(op, "ducker")
	}
	if !goodUnit(strength) {
		return false, core.InvalidArg(op, "strength")
	}
	if !stats.HasExplosion {
		return false, nil
	}
	if err := d.Trigger(strength); err != nil {
		return false, err
	}
	return true, nil
}

// BeatClock is the S89 beat ruler for rhythm plays and timeline cues.
// It only counts beats; the caller maps beat numbers to its own event
// names, sounds, or timeline seeks. Time stays integer milliseconds so
// long songs never drift.
type BeatClock struct {
	bpm         float64
	beatsPerBar int
	pos         core.Duration
	beat        int64
}

// NewBeatClock builds a ruler. BPM must be finite in [MinBPM,MaxBPM],
// beatsPerBar in [MinBeatsPerBar,MaxBeatsPerBar]. Bad inputs are
// InvalidArg.
func NewBeatClock(bpm float64, beatsPerBar int) (*BeatClock, error) {
	const op = "audio.NewBeatClock"
	if !finite(bpm) || bpm < MinBPM || bpm > MaxBPM {
		return nil, core.InvalidArg(op, "bpm")
	}
	if beatsPerBar < MinBeatsPerBar || beatsPerBar > MaxBeatsPerBar {
		return nil, core.InvalidArg(op, "beatsPerBar")
	}
	return &BeatClock{bpm: bpm, beatsPerBar: beatsPerBar}, nil
}

// BPM returns the tempo, or 0 on nil.
func (c *BeatClock) BPM() float64 {
	if c == nil {
		return 0
	}
	return c.bpm
}

// BeatsPerBar returns the meter, or 0 on nil.
func (c *BeatClock) BeatsPerBar() int {
	if c == nil {
		return 0
	}
	return c.beatsPerBar
}

// Pos returns the song position, or 0 on nil.
func (c *BeatClock) Pos() core.Duration {
	if c == nil {
		return 0
	}
	return c.pos
}

// Beat returns crossed beats, or 0 on nil.
func (c *BeatClock) Beat() int64 {
	if c == nil {
		return 0
	}
	return c.beat
}

// Bar returns the current bar (beat / meter), or 0 on nil.
func (c *BeatClock) Bar() int64 {
	if c == nil || c.beatsPerBar <= 0 {
		return 0
	}
	return c.beat / int64(c.beatsPerBar)
}

// BeatInBar returns the beat inside the bar, or 0 on nil.
func (c *BeatClock) BeatInBar() int64 {
	if c == nil || c.beatsPerBar <= 0 {
		return 0
	}
	return c.beat % int64(c.beatsPerBar)
}

// Interval returns one beat in milliseconds, or 0 on nil.
func (c *BeatClock) Interval() core.Duration {
	if c == nil || c.bpm <= 0 {
		return 0
	}
	return core.Duration(60000 / c.bpm)
}

// Phase returns the position inside the beat in [0,1), or 0 on nil.
func (c *BeatClock) Phase() float64 {
	if c == nil || c.bpm <= 0 {
		return 0
	}
	iv := 60000 / c.bpm
	if iv <= 0 {
		return 0
	}
	beatFloat := float64(c.pos) / iv
	_, frac := splitFloat(beatFloat)
	if frac < 0 {
		frac = 0
	}
	if frac >= 1 {
		frac = 0
	}
	return frac
}

func splitFloat(x float64) (int64, float64) {
	i := int64(x)
	return i, x - float64(i)
}

// SetBPM retunes the tempo. Bad values are InvalidArg and keep the old.
// The beat count restats from the current pos so the next Update never
// jumps through NaN.
func (c *BeatClock) SetBPM(bpm float64) error {
	const op = "audio.BeatClock.SetBPM"
	if c == nil {
		return core.InvalidArg(op, "clock")
	}
	if !finite(bpm) || bpm < MinBPM || bpm > MaxBPM {
		return core.InvalidArg(op, "bpm")
	}
	c.bpm = bpm
	c.beat = beatAt(c.pos, bpm)
	return nil
}

// SetBeatsPerBar retunes the meter. Bad values are InvalidArg.
func (c *BeatClock) SetBeatsPerBar(n int) error {
	const op = "audio.BeatClock.SetBeatsPerBar"
	if c == nil {
		return core.InvalidArg(op, "clock")
	}
	if n < MinBeatsPerBar || n > MaxBeatsPerBar {
		return core.InvalidArg(op, "beatsPerBar")
	}
	c.beatsPerBar = n
	return nil
}

// Reset parks the ruler at zero. Nil is a no-op.
func (c *BeatClock) Reset() {
	if c == nil {
		return
	}
	c.pos = 0
	c.beat = 0
}

// Seek moves the song position. Negative targets are InvalidArg and move
// nothing. The beat count follows the new pos.
func (c *BeatClock) Seek(pos core.Duration) error {
	const op = "audio.BeatClock.Seek"
	if c == nil {
		return core.InvalidArg(op, "clock")
	}
	if pos < 0 {
		return core.InvalidArg(op, "pos")
	}
	c.pos = pos
	c.beat = beatAt(pos, c.bpm)
	return nil
}

func beatAt(pos core.Duration, bpm float64) int64 {
	if bpm <= 0 || pos <= 0 {
		return 0
	}
	iv := 60000 / bpm
	if iv <= 0 {
		return 0
	}
	return int64(float64(pos) / iv)
}

// Update advances the song clock and returns newly crossed beat numbers.
// Non-positive dt, settled nil, and nil receivers return nil and move
// nothing. The returned slice is fresh; the caller fires its own cues.
func (c *BeatClock) Update(dt core.Duration) []int64 {
	if c == nil || dt <= 0 {
		return nil
	}
	c.pos += dt
	want := beatAt(c.pos, c.bpm)
	if want <= c.beat {
		return nil
	}
	out := make([]int64, 0, want-c.beat)
	for b := c.beat + 1; b <= want; b++ {
		out = append(out, b)
	}
	c.beat = want
	return out
}

// TimeOfBeat returns the song position of beat n. Negative n is
// InvalidArg with 0. Nil returns 0.
func (c *BeatClock) TimeOfBeat(n int64) (core.Duration, error) {
	const op = "audio.BeatClock.TimeOfBeat"
	if c == nil {
		return 0, core.InvalidArg(op, "clock")
	}
	if n < 0 {
		return 0, core.InvalidArg(op, "beat")
	}
	iv := 60000 / c.bpm
	return core.Duration(math.Round(float64(n) * iv)), nil
}

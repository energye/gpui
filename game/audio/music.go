package audio

import (
	"sort"

	"github.com/energye/gpui/game/core"
)

// Frozen budgets and defaults for the music chain.
const (
	// DefaultBusVolume is the flat 0dB bus level.
	DefaultBusVolume = 1.0
	// DefaultMaxVoices caps simultaneous mixed voices.
	DefaultMaxVoices = 32
	// MaxMixerVoices bounds NewMixer/SetMaxVoices.
	MaxMixerVoices = 256
	// DefaultDuckDepth is how far a full-strength hit ducks music.
	DefaultDuckDepth = 0.5
	// DefaultFade is the standard music switch time.
	DefaultFade = core.Second
	// MaxMusicStack bounds pushed music layers so long runs never grow.
	MaxMusicStack = 8
)

func goodUnit(x float64) bool { return finite(x) && x >= 0 && x <= 1 }

// settleFading completes any in-flight fade so the new switch starts
// from a full target. armFade stages total/elapsed/fading after the
// caller has staged from/stack; zero fades settle instantly via snap.
// Four switches share them so Push/Pop/CrossfadeTo/Stop never drift.
func (p *MusicPlayer) settleFading() {
	if p.fading {
		p.snap()
	}
}

func (p *MusicPlayer) armFade(fade core.Duration) {
	if fade == 0 {
		p.snap()
		return
	}
	p.total = fade
	p.elapsed = 0
	p.fading = true
}

// MusicPlayer is a LIFO stack of music tracks with linear crossfade.
// The stack top is the target; Blend reports the audible mix of the
// fading-out source and the fading-in target. Only AssetID and Duration
// from core are used; nothing is played here.
type MusicPlayer struct {
	stack   []core.AssetID
	from    core.AssetID
	total   core.Duration
	elapsed core.Duration
	fading  bool
}

// NewMusicPlayer builds an empty (silent) player.
func NewMusicPlayer() *MusicPlayer { return &MusicPlayer{} }

// snap completes an in-progress fade instantly. The stack already holds
// the previous target, so only the blend state resets.
func (p *MusicPlayer) snap() {
	p.from = ""
	p.fading = false
	p.total = 0
	p.elapsed = 0
}

// Depth returns the stack depth. Nil players report 0.
func (p *MusicPlayer) Depth() int {
	if p == nil {
		return 0
	}
	return len(p.stack)
}

// Top returns the target track, or "" when silent. Nil players return "".
func (p *MusicPlayer) Top() core.AssetID {
	if p == nil || len(p.stack) == 0 {
		return ""
	}
	return p.stack[len(p.stack)-1]
}

// From returns the fading-out source, or "" when settled. Nil returns "".
func (p *MusicPlayer) From() core.AssetID {
	if p == nil {
		return ""
	}
	return p.from
}

// IsFading reports whether a switch is in flight. Nil reports false.
func (p *MusicPlayer) IsFading() bool {
	if p == nil {
		return false
	}
	return p.fading
}

// Progress returns elapsed/total in [0,1]; settled players report 1.
func (p *MusicPlayer) Progress() float64 {
	if p == nil || !p.fading || p.total <= 0 {
		return 1
	}
	t := float64(p.elapsed) / float64(p.total)
	return clampGain(t)
}

// Blend returns the audible pair: fading-out source with its gain and
// fading-in target with its gain. Gains are linear 1-t/t in [0,1].
// Empty ids are silent; their gain is ignored by the backend.
// Settled players report ("",0,top,1); silent ones ("",0,"",0).
func (p *MusicPlayer) Blend() (from core.AssetID, fromGain float64, to core.AssetID, toGain float64) {
	if p == nil {
		return "", 0, "", 0
	}
	to = p.Top()
	if !p.fading {
		if to == "" {
			return "", 0, "", 0
		}
		return "", 0, to, 1
	}
	t := p.Progress()
	return p.from, clampGain(1 - t), to, clampGain(t)
}

// Play resets the stack to id with no fade. Empty ids are InvalidArg.
func (p *MusicPlayer) Play(id core.AssetID) error {
	if p == nil {
		return core.InvalidArg("audio.Play", "player")
	}
	if id.Empty() {
		return core.InvalidArg("audio.Play", "id")
	}
	p.stack = []core.AssetID{id}
	p.snap()
	return nil
}

// Push crossfades to a new top layer. Stack beyond MaxMusicStack is
// OutOfMemory. Empty ids and negative fades are InvalidArg.
func (p *MusicPlayer) Push(id core.AssetID, fade core.Duration) error {
	if p == nil {
		return core.InvalidArg("audio.Push", "player")
	}
	if id.Empty() {
		return core.InvalidArg("audio.Push", "id")
	}
	if fade < 0 {
		return core.InvalidArg("audio.Push", "fade")
	}
	if len(p.stack) >= MaxMusicStack {
		return core.OutOfMemory("audio.Push", "stack")
	}
	p.settleFading()
	if len(p.stack) == 0 {
		p.stack = []core.AssetID{id}
		p.armFade(fade)
		return nil
	}
	p.from = p.stack[len(p.stack)-1]
	p.stack = append(p.stack, id)
	p.armFade(fade)
	return nil
}

// Pop crossfades back to the layer below. Empty stacks are NotFound.
// Negative fades are InvalidArg; zero fades switch instantly.
func (p *MusicPlayer) Pop(fade core.Duration) error {
	if p == nil {
		return core.InvalidArg("audio.Pop", "player")
	}
	if fade < 0 {
		return core.InvalidArg("audio.Pop", "fade")
	}
	if len(p.stack) == 0 {
		return core.NotFound("audio.Pop", "stack")
	}
	p.settleFading()
	top := p.stack[len(p.stack)-1]
	p.stack = p.stack[:len(p.stack)-1]
	p.from = top
	p.armFade(fade)
	return nil
}

// CrossfadeTo replaces the top without changing depth. Empty ids are
// InvalidArg (use Stop for silence). Same-id retargets are a no-op.
func (p *MusicPlayer) CrossfadeTo(id core.AssetID, fade core.Duration) error {
	if p == nil {
		return core.InvalidArg("audio.CrossfadeTo", "player")
	}
	if id.Empty() {
		return core.InvalidArg("audio.CrossfadeTo", "id")
	}
	if fade < 0 {
		return core.InvalidArg("audio.CrossfadeTo", "fade")
	}
	p.settleFading()
	if len(p.stack) == 0 {
		p.stack = []core.AssetID{id}
		p.armFade(fade)
		return nil
	}
	if p.stack[len(p.stack)-1] == id {
		return nil
	}
	p.from = p.stack[len(p.stack)-1]
	p.stack[len(p.stack)-1] = id
	p.armFade(fade)
	return nil
}

// Stop fades the stack to silence and clears it. Negative fades are
// InvalidArg; silent players are a no-op.
func (p *MusicPlayer) Stop(fade core.Duration) error {
	if p == nil {
		return core.InvalidArg("audio.Stop", "player")
	}
	if fade < 0 {
		return core.InvalidArg("audio.Stop", "fade")
	}
	if len(p.stack) == 0 && !p.fading {
		return nil
	}
	p.settleFading()
	if len(p.stack) == 0 {
		return nil
	}
	top := p.stack[len(p.stack)-1]
	p.stack = nil
	p.from = top
	p.armFade(fade)
	return nil
}

// Update advances the fade clock. Non-positive dt, settled, and nil
// players do nothing.
func (p *MusicPlayer) Update(dt core.Duration) {
	if p == nil || dt <= 0 || !p.fading {
		return
	}
	p.elapsed += dt
	if p.elapsed >= p.total {
		p.snap()
	}
}

// Bus is one mix bus: a volume plus a mute switch.
type Bus struct {
	name   string
	volume float64
	mute   bool
}

// NewBus builds a bus. Empty names and volumes outside finite [0,1]
// are InvalidArg.
func NewBus(name string, volume float64) (*Bus, error) {
	if name == "" {
		return nil, core.InvalidArg("audio.NewBus", "name")
	}
	if !goodUnit(volume) {
		return nil, core.InvalidArg("audio.NewBus", "volume")
	}
	return &Bus{name: name, volume: volume}, nil
}

// Name returns the bus name. Nil returns "".
func (b *Bus) Name() string {
	if b == nil {
		return ""
	}
	return b.name
}

// Volume returns the bus volume. Nil returns 0.
func (b *Bus) Volume() float64 {
	if b == nil {
		return 0
	}
	return b.volume
}

// SetVolume retunes the bus. Bad values are InvalidArg and keep the old.
func (b *Bus) SetVolume(v float64) error {
	if b == nil {
		return core.InvalidArg("audio.SetVolume", "bus")
	}
	if !goodUnit(v) {
		return core.InvalidArg("audio.SetVolume", "volume")
	}
	b.volume = v
	return nil
}

// SetMute flips the mute switch. Nil buses report InvalidArg.
func (b *Bus) SetMute(m bool) error {
	if b == nil {
		return core.InvalidArg("audio.SetMute", "bus")
	}
	b.mute = m
	return nil
}

// Muted reports the mute switch. Nil reports false.
func (b *Bus) Muted() bool {
	if b == nil {
		return false
	}
	return b.mute
}

// Gain returns 0 when muted, else the volume. Nil returns 0.
func (b *Bus) Gain() float64 {
	if b == nil || b.mute {
		return 0
	}
	return clampGain(b.volume)
}

// MixResult is the capped voice sum for one frame.
type MixResult struct {
	Total   float64
	Clipped bool
	Audible int
	Kept    int
}

// Mixer caps simultaneous voices to the loudest MaxVoices.
type Mixer struct {
	maxVoices int
}

// NewMixer builds a voice cap in [1,MaxMixerVoices]. Others are InvalidArg.
func NewMixer(maxVoices int) (*Mixer, error) {
	if maxVoices < 1 || maxVoices > MaxMixerVoices {
		return nil, core.InvalidArg("audio.NewMixer", "maxVoices")
	}
	return &Mixer{maxVoices: maxVoices}, nil
}

// MaxVoices returns the cap. Nil returns 0.
func (m *Mixer) MaxVoices() int {
	if m == nil {
		return 0
	}
	return m.maxVoices
}

// SetMaxVoices retunes the cap. Out-of-range values are InvalidArg.
func (m *Mixer) SetMaxVoices(n int) error {
	if m == nil {
		return core.InvalidArg("audio.SetMaxVoices", "mixer")
	}
	if n < 1 || n > MaxMixerVoices {
		return core.InvalidArg("audio.SetMaxVoices", "maxVoices")
	}
	m.maxVoices = n
	return nil
}

// Mix sums the loudest kept voices and ceilings at 1. Non-finite and
// non-positive entries are silent. The input is never mutated; the result
// is always finite in [0,1]. Nil mixers return a zero result.
func (m *Mixer) Mix(gains []float64) MixResult {
	if m == nil {
		return MixResult{}
	}
	loud := make([]float64, 0, len(gains))
	for _, g := range gains {
		if finite(g) && g > 0 {
			loud = append(loud, g)
		}
	}
	audible := len(loud)
	if audible == 0 {
		return MixResult{}
	}
	kept := audible
	if kept > m.maxVoices {
		kept = m.maxVoices
		// Order only matters when capping: full-set sums skip the sort.
		sort.Slice(loud, func(i, j int) bool { return loud[i] > loud[j] })
		loud = loud[:kept]
	}
	sum := 0.0
	for _, g := range loud {
		sum += g
	}
	if !finite(sum) {
		return MixResult{Audible: audible, Kept: kept}
	}
	if sum > 1 {
		return MixResult{Total: 1, Clipped: true, Audible: audible, Kept: kept}
	}
	return MixResult{Total: sum, Clipped: false, Audible: audible, Kept: kept}
}

// Ducker ducks music while loud hits ring: full duck during hold, then a
// linear climb back during release. Only core numbers are used.
type Ducker struct {
	depth     float64
	hold      core.Duration
	release   core.Duration
	remaining core.Duration
	strength  float64
}

// NewDucker builds a ducker. Depth must be finite [0,1]; negative
// hold/release are InvalidArg.
func NewDucker(depth float64, hold, release core.Duration) (*Ducker, error) {
	if !goodUnit(depth) {
		return nil, core.InvalidArg("audio.NewDucker", "depth")
	}
	if hold < 0 || release < 0 {
		return nil, core.InvalidArg("audio.NewDucker", "time")
	}
	return &Ducker{depth: depth, hold: hold, release: release}, nil
}

// Depth returns the duck depth. Nil returns 0.
func (d *Ducker) Depth() float64 {
	if d == nil {
		return 0
	}
	return d.depth
}

// Active reports whether the duck timer runs. Nil reports false.
func (d *Ducker) Active() bool {
	if d == nil {
		return false
	}
	return d.remaining > 0
}

// Trigger starts (or extends) a duck. Strength must be finite [0,1];
// bad values are InvalidArg. Retriggers keep the louder strength.
func (d *Ducker) Trigger(strength float64) error {
	if d == nil {
		return core.InvalidArg("audio.Trigger", "ducker")
	}
	if !goodUnit(strength) {
		return core.InvalidArg("audio.Trigger", "strength")
	}
	total := d.hold + d.release
	if total <= 0 {
		d.remaining = 0
		d.strength = strength
		return nil
	}
	if d.remaining > 0 && strength < d.strength {
		strength = d.strength
	}
	d.strength = strength
	d.remaining = total
	return nil
}

// Update ticks the duck clock down. Non-positive dt and nil duckers
// do nothing.
func (d *Ducker) Update(dt core.Duration) {
	if d == nil || dt <= 0 || d.remaining <= 0 {
		return
	}
	d.remaining -= dt
	if d.remaining < 0 {
		d.remaining = 0
	}
}

// Gain returns the music multiplier in [0,1]: 1 at rest, 1-depth*strength
// during hold, linear climb during release. Nil and idle return 1.
func (d *Ducker) Gain() float64 {
	if d == nil || d.remaining <= 0 {
		return 1
	}
	dip := d.depth * d.strength
	if !finite(dip) {
		return 1
	}
	if d.remaining > d.release {
		return clampGain(1 - dip)
	}
	t := float64(d.remaining) / float64(d.release)
	return clampGain(1 - dip*t)
}

// CombineGains multiplies one music voice by its bus and duck gains and
// ceilings at 1. Non-finite inputs fail closed at 0 so a bad clock never
// blasts the backend. The caller applies it per audible track: the from
// and to gains from Blend each pass through here with the same bus/duck.
func CombineGains(music, bus, duck float64) float64 {
	if !finite(music) || !finite(bus) || !finite(duck) {
		return 0
	}
	g := music * bus * duck
	if !finite(g) {
		return 0
	}
	return clampGain(g)
}

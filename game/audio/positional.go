package audio

import (
	"math"

	"github.com/energye/gpui/game/core"
)

// DefaultAttenuation is the linear falloff: gain falls straight with distance.
const DefaultAttenuation = 1.0

// DefaultPanStrength is normal stereo spread: the range edge pans fully.
const DefaultPanStrength = 1.0

func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

func finiteVec(v core.Vec2) bool { return finite(v.X) && finite(v.Y) }

// goodNonNeg reports whether x is a usable range/attenuation/strength:
// finite and >= 0. One helper covers the three scalar fields so the
// constructor and valid share the same gate.
func goodNonNeg(x float64) bool { return finite(x) && x >= 0 }

func clampPan(p float64) float64 {
	if p < -1 {
		return -1
	}
	if p > 1 {
		return 1
	}
	return p
}

// panOf maps the horizontal gap to [-1,1]. Range 0 never attenuates but
// still pans (Godot parity); otherwise the gap is relative to the range
// so the range edge pans fully at strength 1.
func panOf(dx, rng, strength float64) float64 {
	if rng == 0 {
		return clampPan(dx * strength)
	}
	return clampPan(dx / rng * strength)
}

// clampGain pins a computed gain to [0,1]. Non-finite or negative parks
// at silent 0 so a rounding slip never turns loud or NaN.
func clampGain(g float64) float64 {
	if !finite(g) || g < 0 {
		return 0
	}
	if g > 1 {
		return 1
	}
	return g
}

// PosSound is one 2D sound source: where it plays, how far it carries,
// how fast it hushes, how wide it spreads. It plays nothing by itself;
// Mix computes the numbers the backend needs.
type PosSound struct {
	Pos         core.Vec2
	Range       float64
	Attenuation float64
	PanStrength float64
}

// NewPosSound builds a source. Pos must be finite, Range finite and >= 0
// (0 never attenuates, Godot parity), Attenuation finite and >= 0
// (1 linear, 2 squared, 0 no falloff inside the range), PanStrength
// finite and >= 0 (0 mono, 1 normal). Bad arguments return a core
// InvalidArg error and store nothing.
func NewPosSound(pos core.Vec2, rng, att, panStrength float64) (PosSound, error) {
	if !finiteVec(pos) {
		return PosSound{}, core.InvalidArg("audio.NewPosSound", "pos")
	}
	if !goodNonNeg(rng) {
		return PosSound{}, core.InvalidArg("audio.NewPosSound", "range")
	}
	if !goodNonNeg(att) {
		return PosSound{}, core.InvalidArg("audio.NewPosSound", "attenuation")
	}
	if !goodNonNeg(panStrength) {
		return PosSound{}, core.InvalidArg("audio.NewPosSound", "panStrength")
	}
	return PosSound{Pos: pos, Range: rng, Attenuation: att, PanStrength: panStrength}, nil
}

// valid reports whether s could have come from NewPosSound.
func (s PosSound) valid() bool {
	return finiteVec(s.Pos) && goodNonNeg(s.Range) &&
		goodNonNeg(s.Attenuation) && goodNonNeg(s.PanStrength)
}

// Mix is the audible result: Gain multiplier in [0,1], Pan in [-1,1]
// (-1 full left, +1 full right), Distance Euclidean game units,
// Audible true only when the backend should emit samples.
type Mix struct {
	Gain     float64
	Pan      float64
	Distance float64
	Audible  bool
}

// silent returns the fail-closed mix: no gain, centered, not audible.
func silent(distance float64) Mix {
	if !finite(distance) || distance < 0 {
		distance = 0
	}
	return Mix{Gain: 0, Pan: 0, Distance: distance, Audible: false}
}

// Mix computes the gain and pan of s heard at listener. Valid inputs
// always return ok=true, even when silent beyond the range (gain 0,
// Audible false, pan still shows the direction). Bad inputs (non-finite
// listener, invalid source) return ok=false with a silent mix, never NaN
// and never a panic. The input source is never mutated.
func (s PosSound) Mix(listener core.Vec2) (Mix, bool) {
	if !finiteVec(listener) || !s.valid() {
		d := 0.0
		if finiteVec(listener) && finiteVec(s.Pos) {
			if dist, ok := distOf(s.Pos, listener); ok {
				d = dist
			}
		}
		return silent(d), false
	}
	dx := s.Pos.X - listener.X
	d, _ := distOf(s.Pos, listener)
	pan := panOf(dx, s.Range, s.PanStrength)
	if s.Range == 0 {
		return Mix{Gain: 1, Pan: pan, Distance: d, Audible: true}, true
	}
	if d >= s.Range {
		return Mix{Gain: 0, Pan: pan, Distance: d, Audible: false}, true
	}
	if s.Attenuation == 0 {
		return Mix{Gain: 1, Pan: pan, Distance: d, Audible: true}, true
	}
	t := d / s.Range
	var gain float64
	switch s.Attenuation {
	case 1:
		gain = 1 - t
	case 2:
		gain = (1 - t) * (1 - t)
	default:
		gain = math.Pow(1-t, s.Attenuation)
	}
	gain = clampGain(gain)
	if gain == 0 {
		return Mix{Gain: 0, Pan: pan, Distance: d, Audible: false}, true
	}
	return Mix{Gain: gain, Pan: pan, Distance: d, Audible: true}, true
}

// distOf returns the Euclidean distance, ok=false for bad inputs.
// Hypot stays overflow-safe for huge-but-finite worlds (1e308 still
// measures far instead of wrapping to loud).
func distOf(a, b core.Vec2) (float64, bool) {
	if !finiteVec(a) || !finiteVec(b) {
		return 0, false
	}
	dx := a.X - b.X
	dy := a.Y - b.Y
	d := math.Hypot(dx, dy)
	if !finite(d) {
		return 0, false
	}
	return d, true
}

// MixAll computes every source heard at listener in order. The input
// slice is never mutated; the result is a fresh slice the caller mixes.
// A bad listener returns nil,false. Bad sources do not fail the whole
// call: their slots hold a silent mix while the rest still compute, so
// one torn sound never mutes the scene. A nil or empty input returns an
// empty non-nil slice with ok=true.
func MixAll(listener core.Vec2, sources []PosSound) ([]Mix, bool) {
	if !finiteVec(listener) {
		return nil, false
	}
	out := make([]Mix, len(sources))
	for i, s := range sources {
		m, _ := s.Mix(listener)
		out[i] = m
	}
	return out, true
}

// Stereo spreads one mono sample into left and right with linear balance:
// l = mono*gain*min(1,1-pan), r = mono*gain*min(1,1+pan). Center stays
// full on both sides, full right mutes left (and mirrored). A mix the
// constructor could never produce, silence, or non-finite input returns
// 0,0 (fail closed, no NaN into the backend), never a panic.
func Stereo(mono float64, m Mix) (l, r float64) {
	if !m.Audible {
		return 0, 0
	}
	if !finite(mono) || !validStereo(m) {
		return 0, 0
	}
	l = mono * m.Gain * math.Min(1, 1-m.Pan)
	r = mono * m.Gain * math.Min(1, 1+m.Pan)
	if !finite(l) || !finite(r) {
		return 0, 0
	}
	return l, r
}

// validStereo reports whether m plus mono could flow to the backend:
// mono finite, audible gain in (0,1], pan in [-1,1], distance finite >= 0.
func validStereo(m Mix) bool {
	return finite(m.Gain) && m.Gain > 0 && m.Gain <= 1 &&
		finite(m.Pan) && m.Pan >= -1 && m.Pan <= 1 &&
		finite(m.Distance) && m.Distance >= 0
}

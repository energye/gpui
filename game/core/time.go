package core

import "math"

// Duration is game time stored as integer milliseconds. Integer storage
// means a million accumulated steps still land on the exact millisecond,
// while Seconds() gives the float seconds the render side wants.
type Duration int64

const (
	Millisecond Duration = 1
	Second      Duration = 1000 * Millisecond
	Minute      Duration = 60 * Second
	Hour        Duration = 60 * Minute
)

// Milliseconds builds a Duration from a millisecond count.
func Milliseconds(ms int64) Duration { return Duration(ms) }

// SecondsFloat builds a Duration from float seconds (rounded to the ms).
func SecondsFloat(s float64) Duration { return Duration(math.Round(s * 1000)) }

// Milliseconds returns the raw millisecond count.
func (d Duration) Milliseconds() int64 { return int64(d) }

// Seconds returns d in float seconds for the render/physics boundary.
func (d Duration) Seconds() float64 { return float64(d) / 1000 }

// Add returns d+o.
func (d Duration) Add(o Duration) Duration { return d + o }

// Sub returns d-o (may be negative; callers clamp).
func (d Duration) Sub(o Duration) Duration { return d - o }

// Scale multiplies d by f (time-scale, slow-mo). Rounded to the ms.
func (d Duration) Scale(f float64) Duration { return Duration(math.Round(float64(d) * f)) }

// Clamp pins d to [min, max]. max < min behaves as max == min.
func (d Duration) Clamp(min, max Duration) Duration {
	if d < min {
		return min
	}
	if d > max {
		return max
	}
	return d
}

// Step is one fixed logic tick: sequence number plus its width. The fixed
// loop itself lives in game/step (capabilities 8.1/11.1); core only keeps
// the drift-free counting so every consumer agrees on "which tick".
type Step struct {
	Index uint64
	Dt    Duration
}

// FirstStep starts a run with the given tick width.
func FirstStep(dt Duration) Step { return Step{Index: 0, Dt: dt} }

// Next advances the sequence number. Dt never changes mid-run, so a long
// run cannot drift the way repeated float adds would.
func (s Step) Next() Step { return Step{Index: s.Index + 1, Dt: s.Dt} }

// Total returns Index*Dt: elapsed logic time before this step.
func (s Step) Total() Duration { return Duration(s.Index) * s.Dt }

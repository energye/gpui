//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package anim

import (
	"math"

	"github.com/energye/gpui/engine/core"
)

// LodLevel is the per-skeleton update rate: off skips, far runs at half
// rate, near runs every frame. Only the scheduler owns the rate; Pose
// math in skeleton.go stays untouched.
type LodLevel int

const (
	// LodOff culls: bounds sit outside the viewport, no update.
	LodOff LodLevel = 0
	// LodFar throttles: visible but far, every other frame.
	LodFar LodLevel = 1
	// LodNear tracks: visible and near, every frame.
	LodNear LodLevel = 2
)

// String returns the stable log name of l.
func (l LodLevel) String() string {
	switch l {
	case LodOff:
		return "off"
	case LodFar:
		return "far"
	case LodNear:
		return "near"
	default:
		return "unknown"
	}
}

// DefaultLodNearDist is the visible-near radius in game units.
const DefaultLodNearDist = 400.0

func finiteRect(r core.Rect) bool {
	return finite(r.X) && finite(r.Y) && finite(r.W) && finite(r.H)
}

// LodPolicy owns the near radius. Viewports arrive per frame so one
// policy serves every window size.
type LodPolicy struct {
	nearDist float64
}

// NewLodPolicy builds a policy. nearDist must be finite and >= 0.
// Bad inputs are InvalidArg.
func NewLodPolicy(nearDist float64) (LodPolicy, error) {
	const op = "anim.NewLodPolicy"
	if !finite(nearDist) || nearDist < 0 {
		return LodPolicy{}, core.InvalidArg(op, "nearDist")
	}
	return LodPolicy{nearDist: nearDist}, nil
}

// NearDist returns the near radius.
func (p LodPolicy) NearDist() float64 { return p.nearDist }

// Classify picks the level of bounds seen through viewport. Outside the
// viewport is off and never updates; inside within nearDist of the
// viewport center is near, the rest of the visible set is far. Empty or
// bad inputs park at off with ok=false, never a panic.
func (p LodPolicy) Classify(bounds, viewport core.Rect) (LodLevel, bool) {
	if !finite(p.nearDist) || p.nearDist < 0 {
		return LodOff, false
	}
	if !finiteRect(bounds) || !finiteRect(viewport) {
		return LodOff, false
	}
	if bounds.IsEmpty() || viewport.IsEmpty() {
		return LodOff, false
	}
	if !bounds.Intersects(viewport) {
		return LodOff, true
	}
	bc := bounds.Center()
	vc := viewport.Center()
	d := math.Hypot(bc.X-vc.X, bc.Y-vc.Y)
	if !finite(d) {
		return LodOff, false
	}
	if d <= p.nearDist {
		return LodNear, true
	}
	return LodFar, true
}

// Batch classifies every bound through one viewport in order. The input
// is never mutated; the result is fresh every call. Bad slots park at
// off while the rest still classify.
func (p LodPolicy) Batch(bounds []core.Rect, viewport core.Rect) []LodLevel {
	out := make([]LodLevel, len(bounds))
	for i, b := range bounds {
		lv, _ := p.Classify(b, viewport)
		out[i] = lv
	}
	return out
}

// ShouldUpdate reports whether slot index needs a pose update on tick.
// Near updates every frame, far updates on alternating frames staggered
// by index parity so half the far set runs each tick, off never updates.
func ShouldUpdate(level LodLevel, tick uint64, index int) bool {
	switch level {
	case LodNear:
		return true
	case LodFar:
		if index < 0 {
			index = 0
		}
		return (tick+uint64(index&1))%2 == 0
	default:
		return false
	}
}

// LodFrame is one scheduling verdict: per-slot levels plus per-slot
// update flags plus counts.
type LodFrame struct {
	Levels  []LodLevel
	Updates []bool
	Off     int
	Far     int
	Near    int
	Wanted  int
}

// LodScheduler counts frames so far slots alternate without caller state.
type LodScheduler struct {
	policy LodPolicy
	tick   uint64
}

// NewLodScheduler builds a scheduler around policy. A torn policy parks
// at the default radius instead of failing the window.
func NewLodScheduler(policy LodPolicy) *LodScheduler {
	if !finite(policy.nearDist) || policy.nearDist < 0 {
		policy = LodPolicy{nearDist: DefaultLodNearDist}
	}
	return &LodScheduler{policy: policy}
}

// Policy returns the scheduling policy.
func (s *LodScheduler) Policy() LodPolicy {
	if s == nil {
		p, _ := NewLodPolicy(DefaultLodNearDist)
		return p
	}
	return s.policy
}

// Tick returns the frame counter, or 0 on nil.
func (s *LodScheduler) Tick() uint64 {
	if s == nil {
		return 0
	}
	return s.tick
}

// Reset parks the frame counter at zero. Nil is a no-op.
func (s *LodScheduler) Reset() {
	if s == nil {
		return
	}
	s.tick = 0
}

// NextFrame classifies bounds through viewport for the current tick and
// advances the counter. The input is never mutated; levels and updates
// are fresh slices. A nil scheduler or bad viewport parks everything at
// off. Tick wraps naturally at 2^64; parity scheduling never breaks.
func (s *LodScheduler) NextFrame(bounds []core.Rect, viewport core.Rect) LodFrame {
	if s == nil {
		off := make([]LodLevel, len(bounds))
		return LodFrame{Levels: off, Updates: make([]bool, len(bounds)), Off: len(bounds)}
	}
	levels := s.policy.Batch(bounds, viewport)
	updates := make([]bool, len(levels))
	fr := LodFrame{Levels: levels, Updates: updates}
	for i, lv := range levels {
		switch lv {
		case LodNear:
			fr.Near++
		case LodFar:
			fr.Far++
		default:
			fr.Off++
		}
		if ShouldUpdate(lv, s.tick, i) {
			updates[i] = true
			fr.Wanted++
		}
	}
	s.tick++
	return fr
}

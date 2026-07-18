package render

import (
	"errors"
	"fmt"
	"image"

	gpucontext "github.com/energye/gpui/gpu/context"
)

// PresentMode describes how PresentFrameAuto / PlanFramePresent should hit the surface.
//
// Steady UI frames should prefer Idle or Damage* modes. PresentModeFull is reserved
// for bootstrap, display-mode switches, and deliberate full redraw (H01-class).
type PresentMode int

const (
	// PresentModeIdle: no dirty region — skip GPU flush and the present callback.
	PresentModeIdle PresentMode = iota
	// PresentModeDamageMulti: multi-rect damage present (ADR-028 independent scissors).
	PresentModeDamageMulti
	// PresentModeDamageUnion: single bounding-box damage present.
	PresentModeDamageUnion
	// PresentModeFull: full-surface PresentFrame path (explicit full redraw).
	PresentModeFull
)

// String returns a stable mode name for logs and tests.
func (m PresentMode) String() string {
	switch m {
	case PresentModeIdle:
		return "idle"
	case PresentModeDamageMulti:
		return "damage_multi"
	case PresentModeDamageUnion:
		return "damage_union"
	case PresentModeFull:
		return "full"
	default:
		return fmt.Sprintf("PresentMode(%d)", int(m))
	}
}

// Damage planning knobs (S6.1). Values are part of the frame-enforce contract;
// change only with docs/S6_1_FRAME_ENFORCE.md + TestS61_* updates.
const (
	// MaxTrackedDamageRects is the public name for the trackDamage collapse threshold.
	MaxTrackedDamageRects = maxDamageRects

	// DamageFullCoverageThreshold promotes nearly-full dirty regions to PresentModeFull.
	// LoadOpLoad of ≥85% of the surface is rarely cheaper than an explicit full redraw.
	DamageFullCoverageThreshold = 0.85

	// DamageMultiWasteRatio prefers multi-rect when union area exceeds sum(rect areas)
	// by this factor (large empty gaps between dirty regions).
	DamageMultiWasteRatio = 1.35
)

// FramePresentPlan is the pure decision result for a set of physical damage rects.
type FramePresentPlan struct {
	Mode  PresentMode
	Rects []image.Rectangle // rects to feed PresentFrameDamageRects (multi) or single union
	Union image.Rectangle   // bounding box (empty when idle)
}

// PresentOutcome reports what PresentFrameAuto actually executed.
type PresentOutcome struct {
	Mode  PresentMode
	Idle  bool
	Rects int
}

// BeginFrame starts a retained steady UI frame by clearing the per-frame damage list.
// Call once before drawing dirty content. Prefer this over ad-hoc ResetFrameDamage
// in application code so the frame model has a single entry point.
func (c *Context) BeginFrame() {
	if c == nil {
		return
	}
	c.ResetFrameDamage()
	// P1-3: per-frame flush metric (F.03).
	c.pathStats.FrameFlushes = 0
	// Frame boundary: free previous GPU command buffers + reset LoadOp state.
	// Apps (e.g. particle_kitchen_sink) call BeginFrame each tick; without this
	// session.prevCmdBufs never drain on the window present path.
	if rc := c.gpuCtxOps(); rc != nil {
		rc.BeginFrame()
	}
}

// Invalidate marks a logical rectangle dirty (HiDPI-scaled to physical pixels).
// It is the app-facing name for TrackDamageRect.
func (c *Context) Invalidate(bounds image.Rectangle) {
	if c == nil {
		return
	}
	c.TrackDamageRect(bounds)
}

// MarkFullRedraw records the entire logical surface as dirty.
// PresentFrameAuto will typically promote this to PresentModeFull when coverage is high;
// callers that must force the full clear path should call PresentFrameFull directly.
func (c *Context) MarkFullRedraw() {
	if c == nil {
		return
	}
	c.TrackDamageRect(image.Rect(0, 0, c.Width(), c.Height()))
}

// PlanPresent chooses a present mode from the current FrameDamage and physical
// present extent (swapchain / offscreen size in physical pixels).
func (c *Context) PlanPresent(surfaceW, surfaceH int) FramePresentPlan {
	if c == nil {
		return FramePresentPlan{Mode: PresentModeIdle}
	}
	return PlanFramePresent(c.FrameDamage(), surfaceW, surfaceH)
}

// PlanFramePresent is the pure multi-vs-union-vs-full policy used by PresentFrameAuto.
// rects must already be in physical pixels (as produced by FrameDamage / TrackDamageRect).
func PlanFramePresent(rects []image.Rectangle, surfaceW, surfaceH int) FramePresentPlan {
	rects = filterNonEmptyRects(rects)
	if len(rects) == 0 || surfaceW <= 0 || surfaceH <= 0 {
		return FramePresentPlan{Mode: PresentModeIdle}
	}

	rects = CoalesceDamageRects(rects, MaxTrackedDamageRects)

	surface := image.Rect(0, 0, surfaceW, surfaceH)
	// Clip to surface for coverage math.
	clipped := make([]image.Rectangle, 0, len(rects))
	for _, r := range rects {
		r = r.Intersect(surface)
		if !r.Empty() {
			clipped = append(clipped, r)
		}
	}
	if len(clipped) == 0 {
		return FramePresentPlan{Mode: PresentModeIdle}
	}
	union := unionDamageRects(clipped)
	if union.Empty() {
		return FramePresentPlan{Mode: PresentModeIdle}
	}

	surfaceArea := rectArea(surface)
	unionArea := rectArea(union)
	var sum int64
	for _, r := range clipped {
		sum += rectArea(r)
	}

	// Full promote uses *actual dirty pixel estimate* (sum of rect areas), never the
	// sparse union bbox alone — two corner widgets must stay multi, not full.
	if surfaceArea > 0 && float64(sum) >= DamageFullCoverageThreshold*float64(surfaceArea) {
		full := surface
		return FramePresentPlan{
			Mode:  PresentModeFull,
			Rects: []image.Rectangle{full},
			Union: full,
		}
	}

	if len(clipped) == 1 {
		// Single large dirty region covering most of the surface → full clear path.
		if surfaceArea > 0 && float64(unionArea) >= DamageFullCoverageThreshold*float64(surfaceArea) {
			full := surface
			return FramePresentPlan{
				Mode:  PresentModeFull,
				Rects: []image.Rectangle{full},
				Union: full,
			}
		}
		return FramePresentPlan{
			Mode:  PresentModeDamageUnion,
			Rects: clipped,
			Union: union,
		}
	}

	// Large empty gap inside the bounding box → keep independent regions (multi).
	if sum > 0 && float64(unionArea) >= float64(sum)*DamageMultiWasteRatio {
		return FramePresentPlan{
			Mode:  PresentModeDamageMulti,
			Rects: clipped,
			Union: union,
		}
	}

	// Clustered / overlapping dirty regions → one union scissor.
	// If that union itself covers almost the whole surface, promote to full.
	if surfaceArea > 0 && float64(unionArea) >= DamageFullCoverageThreshold*float64(surfaceArea) {
		full := surface
		return FramePresentPlan{
			Mode:  PresentModeFull,
			Rects: []image.Rectangle{full},
			Union: full,
		}
	}
	return FramePresentPlan{
		Mode:  PresentModeDamageUnion,
		Rects: []image.Rectangle{union},
		Union: union,
	}
}

// CoalesceDamageRects merges intersecting or edge-adjacent rects, then collapses
// to a single union if the count still exceeds maxRects.
//
// maxRects ≤ 0 means "no cap" (only pairwise coalesce).
func CoalesceDamageRects(rects []image.Rectangle, maxRects int) []image.Rectangle {
	rects = filterNonEmptyRects(rects)
	if len(rects) <= 1 {
		return rects
	}

	// Greedy merge until stable: union any pair that intersects or touches.
	out := append([]image.Rectangle(nil), rects...)
	changed := true
	for changed {
		changed = false
		next := out[:0:len(out)]
		for _, r := range out {
			merged := false
			for i := range next {
				if rectsTouchOrOverlap(next[i], r) {
					next[i] = next[i].Union(r)
					merged = true
					changed = true
					break
				}
			}
			if !merged {
				next = append(next, r)
			}
		}
		out = next
	}

	if maxRects > 0 && len(out) > maxRects {
		u := unionDamageRects(out)
		return []image.Rectangle{u}
	}
	return out
}

// PresentFrameFull is the explicit full-surface redraw API.
// Use for bootstrap, resize/mode switch, and deliberate full redraw scenes.
// Steady UI frames should prefer PresentFrameAuto.
func (c *Context) PresentFrameFull(view gpucontext.TextureView, width, height uint32, present func() error) error {
	return c.PresentFrame(view, width, height, present)
}

// PresentFrameAuto presents according to PlanPresent(width, height).
//
//	idle          → no FlushGPU, present callback not invoked
//	damage_multi  → PresentFrameDamageRects
//	damage_union  → PresentFrameDamage
//	full          → PresentFrameFull
//
// Application code for retained UI should default to this helper instead of
// always calling PresentFrame (which implies a full clear path).
func (c *Context) PresentFrameAuto(view gpucontext.TextureView, width, height uint32, present func() error) (PresentOutcome, error) {
	if c == nil {
		return PresentOutcome{}, errors.New("render: nil context")
	}
	plan := c.PlanPresent(int(width), int(height))
	out := PresentOutcome{Mode: plan.Mode, Rects: len(plan.Rects)}

	switch plan.Mode {
	case PresentModeIdle:
		out.Idle = true
		return out, nil
	case PresentModeDamageMulti:
		if err := c.PresentFrameDamageRects(view, width, height, plan.Rects, present); err != nil {
			return out, err
		}
		return out, nil
	case PresentModeDamageUnion:
		if err := c.PresentFrameDamage(view, width, height, plan.Union, present); err != nil {
			return out, err
		}
		return out, nil
	case PresentModeFull:
		if err := c.PresentFrameFull(view, width, height, present); err != nil {
			return out, err
		}
		return out, nil
	default:
		return out, fmt.Errorf("render: unknown present mode %v", plan.Mode)
	}
}

func filterNonEmptyRects(rects []image.Rectangle) []image.Rectangle {
	if len(rects) == 0 {
		return nil
	}
	out := make([]image.Rectangle, 0, len(rects))
	for _, r := range rects {
		if !r.Empty() {
			out = append(out, r)
		}
	}
	return out
}

func unionDamageRects(rects []image.Rectangle) image.Rectangle {
	var u image.Rectangle
	for _, r := range rects {
		if r.Empty() {
			continue
		}
		if u.Empty() {
			u = r
			continue
		}
		u = u.Union(r)
	}
	return u
}

func rectArea(r image.Rectangle) int64 {
	if r.Empty() {
		return 0
	}
	return int64(r.Dx()) * int64(r.Dy())
}

// rectsTouchOrOverlap is true when a and b intersect or share an edge/corner
// (expanded by 1px so adjacent dirty widgets coalesce).
func rectsTouchOrOverlap(a, b image.Rectangle) bool {
	if a.Empty() || b.Empty() {
		return false
	}
	ae := image.Rect(a.Min.X-1, a.Min.Y-1, a.Max.X+1, a.Max.Y+1)
	return !ae.Intersect(b).Empty()
}

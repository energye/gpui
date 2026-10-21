//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build linux && !(js && wasm)

package gles

import (
	"image"
)

// maxPresentDamageHistory is how many past presented damage unions are kept.
// EGL buffer age can be as large as the swapchain depth; 4 covers the usual
// double/triple buffering and older ages fall back to a full present.
const maxPresentDamageHistory = 4

// presentDamageFullCoverage promotes a damage union covering most of the
// surface to a full present: a compositor hint that large is not cheaper
// than a plain swap, and the full path is always safe.
const presentDamageFullCoverage = 0.85

// resetPresentDamageHistory forces the next present onto the full path.
// Called when the EGL surface or its buffers are recreated.
func (s *Surface) resetPresentDamageHistory() {
	if s == nil {
		return
	}
	s.presentDamageHist = [maxPresentDamageHistory]image.Rectangle{}
	s.presentDamageHistLen = 0
}

// damageUnionForAge returns the union of current plus the damage presented
// for the previous age-1 frames, so the back buffer being written is current
// everywhere it can be displayed after the swap.
//
// ok is false when history is too short (first frames after configure, age
// larger than the kept window, or no history at all); the caller must then
// present the full surface.
func (s *Surface) damageUnionForAge(age int, current image.Rectangle) (image.Rectangle, bool) {
	if s == nil || current.Empty() {
		return image.Rectangle{}, false
	}
	if age < 1 || age > maxPresentDamageHistory+1 {
		return image.Rectangle{}, false
	}
	// No presented history (first frame after configure/recreate) means the
	// back buffer contents are undefined: present full.
	if s.presentDamageHistLen == 0 {
		return image.Rectangle{}, false
	}
	need := age - 1
	if need > s.presentDamageHistLen {
		return image.Rectangle{}, false
	}
	u := current
	for i := 0; i < need; i++ {
		u = u.Union(s.presentDamageHist[i])
	}
	if u.Empty() {
		return image.Rectangle{}, false
	}
	return u, true
}

// recordPresentDamage pushes the damage actually painted into the persistent
// FBO for this frame into the history. The first frame (or an explicit
// no-damage full present) records the full surface: that buffer is valid
// everywhere and later ages must union it until it rotates out.
func (s *Surface) recordPresentDamage(r image.Rectangle) {
	if s == nil {
		return
	}
	if s.presentDamageHistLen < maxPresentDamageHistory {
		s.presentDamageHistLen++
	}
	copy(s.presentDamageHist[1:s.presentDamageHistLen], s.presentDamageHist[:s.presentDamageHistLen-1])
	s.presentDamageHist[0] = r
}

// canScissorPresent reports whether the damage-scissored blit is 1:1 with the
// window. A borrowed old FBO (resize storm) is stretched to the new size, so
// it must present full.
func (s *Surface) canScissorPresent() bool {
	if s == nil || s.config == nil {
		return false
	}
	if s.fboWidth == 0 || s.fboHeight == 0 {
		return false
	}
	return s.fboWidth == s.config.Width && s.fboHeight == s.config.Height
}

// presentDamageUnion merges the backend-neutral damage list render passes in
// into one rect (EGL damage is a hint; one union keeps history simple and is
// still far below the full surface for the steady single-widget case).
func presentDamageUnion(rects []image.Rectangle) image.Rectangle {
	var u image.Rectangle
	for _, r := range rects {
		if r.Empty() {
			continue
		}
		u = u.Union(r)
	}
	return u
}

// presentDamageCoversFull reports whether u covers at least presentDamageFullCoverage
// of a w×h surface. Empty/invalid sizes report true (use the full path).
func presentDamageCoversFull(u image.Rectangle, w, h int32) bool {
	if w <= 0 || h <= 0 {
		return true
	}
	if u.Empty() {
		return true
	}
	area := int64(u.Dx()) * int64(u.Dy())
	surface := int64(w) * int64(h)
	return float64(area) >= presentDamageFullCoverage*float64(surface)
}

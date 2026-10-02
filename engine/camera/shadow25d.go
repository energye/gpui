//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// ShadowMath25D lands a true-2.5D shadow on the ground, S79 scope (S79).
//
// The demo fires a 3D ray (ShapeCast3D) and parks the shadow on the hit
// point, hiding it when nothing is hit. Our physics has no 3D bodies, so
// this file does the landing math only: the shadow falls straight down
// to the ground height, presses flat through the same basis, and hides
// when the target is below ground or the numbers are bad. The signature
// takes the target point (later: the ray hit point) so a future 3D cast
// feeds in without changing this function.
//
// Layers (bottom-up): LandShadow projects the ground point; ShadowVisible
// reports whether to draw it; the window puts the shadow one z slot above
// its owner's YSort25D slot, matching the demo's odd-slot convention.
package camera

import (
	"github.com/energye/gpui/engine/core"
)

// LandShadow drops target straight down to groundY and presses the
// ground point flat through basis. Below-ground targets and NaN/Inf
// input report ok=false (hide the shadow), never NaN.
func LandShadow(b Basis25D, target core.Vec3, groundY float64) (flat core.Vec2, ok bool) {
	if !finite(target.X) || !finite(target.Y) || !finite(target.Z) || !finite(groundY) {
		return core.Vec2{}, false
	}
	if target.Y < groundY {
		return core.Vec2{}, false
	}
	return b.Project(core.V3(target.X, groundY, target.Z))
}

// ShadowVisible reports whether a landed shadow should draw:
// landed ok plus the target above the ground plane.
func ShadowVisible(target core.Vec3, groundY float64, landedOK bool) bool {
	if !landedOK {
		return false
	}
	if !finite(target.Y) || !finite(groundY) {
		return false
	}
	return target.Y >= groundY
}

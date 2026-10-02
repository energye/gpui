//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// Basis25D presses a 3D point flat into 2D, the Godot misc/2.5d way (S79).
//
// The demo's Node25D keeps three 2D axes and draws every point as
// x*basisX + y*basisY + z*basisZ. Six view modes only swap the three
// axes; SCALE stays 32. This file copies the six axis sets number for
// number from addons/node25d/node_25d.gd (only copying the method, not
// the code), so the matrices match the demo bit for bit.
//
// This is parallel squash, not the 1.1 pinhole Projector next door:
// no focal length, no depth scale, nothing to reconcile. Depth only
// matters for draw order (see ysort25d.go) and shadow landing
// (see shadow25d.go).
package camera

import (
	"github.com/energye/gpui/engine/core"
)

// BasisScale is the demo's SCALE: 32 two-dimensional units per 3D unit.
const BasisScale = 32

// ViewMode names one of the demo's six 2.5D views, in demo order:
// 45-degree, isometric, top-down, front-side, oblique-Y, oblique-Z.
type ViewMode int

const (
	View45 ViewMode = iota
	ViewIsometric
	ViewTopDown
	ViewFrontSide
	ViewObliqueY
	ViewObliqueZ
)

// Basis25D holds the three 2D axes for one view mode.
// A 3D point (x,y,z) lands on x*X + y*Y + z*Z, exactly like the demo's
// flat_pos = x*_basisX + y*_basisY + z*_basisZ.
type Basis25D struct {
	X, Y, Z core.Vec2
	mode    ViewMode
}

// NewBasis25D builds the axis set for one view mode.
// The numbers are the demo's set_view_mode values times BasisScale,
// written out so the six matrices stay readable next to the demo.
func NewBasis25D(mode ViewMode) (Basis25D, error) {
	const op = "camera.NewBasis25D"
	s := float64(BasisScale)
	switch mode {
	case View45:
		return Basis25D{
			X:    core.V2(s*1, s*0),
			Y:    core.V2(s*0, s*-0.70710678118),
			Z:    core.V2(s*0, s*0.70710678118),
			mode: mode,
		}, nil
	case ViewIsometric:
		return Basis25D{
			X:    core.V2(s*0.86602540378, s*0.5),
			Y:    core.V2(s*0, s*-1),
			Z:    core.V2(s*-0.86602540378, s*0.5),
			mode: mode,
		}, nil
	case ViewTopDown:
		return Basis25D{
			X:    core.V2(s*1, s*0),
			Y:    core.V2(s*0, s*0),
			Z:    core.V2(s*0, s*1),
			mode: mode,
		}, nil
	case ViewFrontSide:
		return Basis25D{
			X:    core.V2(s*1, s*0),
			Y:    core.V2(s*0, s*-1),
			Z:    core.V2(s*0, s*0),
			mode: mode,
		}, nil
	case ViewObliqueY:
		return Basis25D{
			X:    core.V2(s*1, s*0),
			Y:    core.V2(s*-0.70710678118, s*-0.70710678118),
			Z:    core.V2(s*0, s*1),
			mode: mode,
		}, nil
	case ViewObliqueZ:
		return Basis25D{
			X:    core.V2(s*1, s*0),
			Y:    core.V2(s*0, s*-1),
			Z:    core.V2(s*-0.70710678118, s*0.70710678118),
			mode: mode,
		}, nil
	}
	return Basis25D{}, core.InvalidArg(op, "mode")
}

// Mode returns the view mode this basis was built for.
func (b Basis25D) Mode() ViewMode { return b.mode }

// Project presses one 3D point flat through the three axes.
// NaN/Inf input returns ok=false with a zero point, never NaN.
func (b Basis25D) Project(p core.Vec3) (flat core.Vec2, ok bool) {
	if !finite(p.X) || !finite(p.Y) || !finite(p.Z) {
		return core.Vec2{}, false
	}
	flat = core.V2(
		p.X*b.X.X+p.Y*b.Y.X+p.Z*b.Z.X,
		p.X*b.X.Y+p.Y*b.Y.Y+p.Z*b.Z.Y,
	)
	if !finiteVec(flat) {
		return core.Vec2{}, false
	}
	return flat, true
}

// ProjectPoints presses many 3D points flat in order.
// A bad point fails the whole set (ok=false) so the caller never
// draws half a shape, same contract as Projector.ProjectQuad.
func (b Basis25D) ProjectPoints(ps []core.Vec3) ([]core.Vec2, bool) {
	out := make([]core.Vec2, len(ps))
	for i, p := range ps {
		flat, ok := b.Project(p)
		if !ok {
			return nil, false
		}
		out[i] = flat
	}
	return out, true
}

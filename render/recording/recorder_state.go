//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package recording

// --------------------------------------------------------------------------
// State Management
// --------------------------------------------------------------------------

// Save saves the current graphics state to the stack.
// The state includes transform, fill style, stroke style, and line properties.
func (r *Recorder) Save() {
	// Clone dash pattern
	var dashCopy []float64
	if r.dashPattern != nil {
		dashCopy = make([]float64, len(r.dashPattern))
		copy(dashCopy, r.dashPattern)
	}

	r.stateStack = append(r.stateStack, recorderState{
		fillBrush:   r.fillBrush,
		strokeBrush: r.strokeBrush,
		lineWidth:   r.lineWidth,
		lineCap:     r.lineCap,
		lineJoin:    r.lineJoin,
		miterLimit:  r.miterLimit,
		dashPattern: dashCopy,
		dashOffset:  r.dashOffset,
		fillRule:    r.fillRule,
		antiAlias:   r.antiAlias,
		transform:   r.transform,
		fontFace:    r.fontFace,
		fontFamily:  r.fontFamily,
		fontSize:    r.fontSize,
	})

	r.commands = append(r.commands, SaveCommand{})
}

// Restore restores the previously saved graphics state.
// If the state stack is empty, this is a no-op.
func (r *Recorder) Restore() {
	if len(r.stateStack) == 0 {
		return
	}

	state := r.stateStack[len(r.stateStack)-1]
	r.stateStack = r.stateStack[:len(r.stateStack)-1]

	r.fillBrush = state.fillBrush
	r.strokeBrush = state.strokeBrush
	r.lineWidth = state.lineWidth
	r.lineCap = state.lineCap
	r.lineJoin = state.lineJoin
	r.miterLimit = state.miterLimit
	r.dashPattern = state.dashPattern
	r.dashOffset = state.dashOffset
	r.fillRule = state.fillRule
	r.antiAlias = state.antiAlias
	r.transform = state.transform
	r.fontFace = state.fontFace
	r.fontFamily = state.fontFamily
	r.fontSize = state.fontSize
	r.syncPathBasis()

	r.commands = append(r.commands, RestoreCommand{})
}

// Push is an alias for Save, matching render.Context API.
func (r *Recorder) Push() {
	r.Save()
}

// Pop is an alias for Restore, matching render.Context API.
func (r *Recorder) Pop() {
	r.Restore()
}

// --------------------------------------------------------------------------
// Transform
// --------------------------------------------------------------------------

// syncPathBasis keeps the open path expressible under the current CTM.
//
// Recording stores geometry in user space and lets playback re-apply the CTM,
// but render.Context bakes every verb with the matrix in force at that moment.
// When a transform changes while a path is open, the verbs already added are
// remapped into the new space so that one matrix (pathBasis, always equal to
// the current transform) describes the whole path. Remapping is a no-op in the
// common case where nothing was added yet.
func (r *Recorder) syncPathBasis() {
	if r.currentPath == nil || r.currentPath.NumVerbs() == 0 {
		r.pathBasis = r.transform
		return
	}
	if r.pathBasis == r.transform {
		return
	}
	delta := r.transform.Invert().Multiply(r.pathBasis)
	r.currentPath = r.currentPath.Transform(delta)
	r.pathBasis = r.transform
}

// concatOp applies op to the recorder's CTM and records that same op.
//
// Recorded transform ops are relative on purpose: at playback they compose onto
// whatever CTM the target context already carries (a layer offset and the device
// scale at raster time) instead of replacing it, so one recording replays
// correctly at any scale and position. Emitting the op verbatim — rather than
// the recomputed delta prev⁻¹·(prev·op) — keeps the replayed product the same
// sequence of multiplications a live Context performs, so replay is bit-exact
// with live painting, and skips an inversion per transform per frame.
func (r *Recorder) concatOp(op Matrix) {
	if op == Identity() {
		return
	}
	prev := r.transform
	r.transform = prev.Multiply(op)
	if r.transform == prev {
		return
	}
	r.syncPathBasis()
	r.commands = append(r.commands, ConcatCommand{Matrix: op})
}

// emitAbsoluteConcat records the delta that carries prev to the recorder's
// current transform, for operations that set an absolute CTM (Identity).
func (r *Recorder) emitAbsoluteConcat(prev Matrix) {
	if prev == r.transform {
		return
	}
	r.commands = append(r.commands, ConcatCommand{Matrix: prev.Invert().Multiply(r.transform)})
}

// Identity resets the transformation matrix to identity.
func (r *Recorder) Identity() {
	prev := r.transform
	r.transform = Identity()
	r.syncPathBasis()
	r.emitAbsoluteConcat(prev)
}

// Translate applies a translation to the transformation matrix.
func (r *Recorder) Translate(x, y float64) {
	r.concatOp(Translate(x, y))
}

// Scale applies a scaling transformation.
func (r *Recorder) Scale(sx, sy float64) {
	r.concatOp(Scale(sx, sy))
}

// Rotate applies a rotation (angle in radians).
func (r *Recorder) Rotate(angle float64) {
	r.concatOp(Rotate(angle))
}

// RotateAbout rotates around a specific point.
func (r *Recorder) RotateAbout(angle, x, y float64) {
	r.Translate(x, y)
	r.Rotate(angle)
	r.Translate(-x, -y)
}

// Shear applies a shear transformation.
func (r *Recorder) Shear(x, y float64) {
	r.concatOp(Shear(x, y))
}

// Transform multiplies the current transformation matrix by the given matrix.
func (r *Recorder) Transform(m Matrix) {
	r.concatOp(m)
}

// SetTransform replaces the current transformation matrix. Because the target
// is absolute rather than an operation, the recorded concat is the delta that
// carries the previous matrix into it.
func (r *Recorder) SetTransform(m Matrix) {
	prev := r.transform
	r.transform = m
	r.syncPathBasis()
	r.emitAbsoluteConcat(prev)
}

// GetTransform returns a copy of the current transformation matrix.
func (r *Recorder) GetTransform() Matrix {
	return r.transform
}

// TransformPoint transforms a point by the current matrix.
func (r *Recorder) TransformPoint(x, y float64) (float64, float64) {
	return TransformPoint(r.transform, x, y)
}

// InvertY inverts the Y axis (useful for coordinate system changes).
func (r *Recorder) InvertY() {
	r.Translate(0, float64(r.height))
	r.Scale(1, -1)
}

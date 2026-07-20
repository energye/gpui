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

	r.commands = append(r.commands, RestoreCommand{})
}

// Push is an alias for Save, matching gg.Context API.
func (r *Recorder) Push() {
	r.Save()
}

// Pop is an alias for Restore, matching gg.Context API.
func (r *Recorder) Pop() {
	r.Restore()
}

// --------------------------------------------------------------------------
// Transform
// --------------------------------------------------------------------------

// Identity resets the transformation matrix to identity.
func (r *Recorder) Identity() {
	r.transform = Identity()
	r.commands = append(r.commands, SetTransformCommand{Matrix: r.transform})
}

// Translate applies a translation to the transformation matrix.
func (r *Recorder) Translate(x, y float64) {
	r.transform = r.transform.Multiply(Translate(x, y))
	r.commands = append(r.commands, SetTransformCommand{Matrix: r.transform})
}

// Scale applies a scaling transformation.
func (r *Recorder) Scale(sx, sy float64) {
	r.transform = r.transform.Multiply(Scale(sx, sy))
	r.commands = append(r.commands, SetTransformCommand{Matrix: r.transform})
}

// Rotate applies a rotation (angle in radians).
func (r *Recorder) Rotate(angle float64) {
	r.transform = r.transform.Multiply(Rotate(angle))
	r.commands = append(r.commands, SetTransformCommand{Matrix: r.transform})
}

// RotateAbout rotates around a specific point.
func (r *Recorder) RotateAbout(angle, x, y float64) {
	r.Translate(x, y)
	r.Rotate(angle)
	r.Translate(-x, -y)
}

// Shear applies a shear transformation.
func (r *Recorder) Shear(x, y float64) {
	r.transform = r.transform.Multiply(Shear(x, y))
	r.commands = append(r.commands, SetTransformCommand{Matrix: r.transform})
}

// Transform multiplies the current transformation matrix by the given matrix.
func (r *Recorder) Transform(m Matrix) {
	r.transform = r.transform.Multiply(m)
	r.commands = append(r.commands, SetTransformCommand{Matrix: r.transform})
}

// SetTransform replaces the current transformation matrix.
func (r *Recorder) SetTransform(m Matrix) {
	r.transform = m
	r.commands = append(r.commands, SetTransformCommand{Matrix: r.transform})
}

// GetTransform returns a copy of the current transformation matrix.
func (r *Recorder) GetTransform() Matrix {
	return r.transform
}

// TransformPoint transforms a point by the current matrix.
func (r *Recorder) TransformPoint(x, y float64) (float64, float64) {
	return r.transform.TransformPoint(x, y)
}

// InvertY inverts the Y axis (useful for coordinate system changes).
func (r *Recorder) InvertY() {
	r.Translate(0, float64(r.height))
	r.Scale(1, -1)
}

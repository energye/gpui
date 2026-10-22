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

import (
	"math"
	"strings"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// --------------------------------------------------------------------------
// Path Building
// --------------------------------------------------------------------------

// Path verbs are recorded in user space: the CTM is carried by the
// ConcatCommand stream and applied by each backend at playback, exactly
// like render.Context bakes them with its own matrix. syncPathBasis keeps the
// invariant even when a transform changes mid-path.

// MoveTo starts a new subpath at the given point.
func (r *Recorder) MoveTo(x, y float64) {
	r.currentPath.MoveTo(x, y)
}

// LineTo adds a line to the current path.
func (r *Recorder) LineTo(x, y float64) {
	r.currentPath.LineTo(x, y)
}

// QuadraticTo adds a quadratic Bezier curve to the current path.
func (r *Recorder) QuadraticTo(cx, cy, x, y float64) {
	r.currentPath.QuadraticTo(cx, cy, x, y)
}

// CubicTo adds a cubic Bezier curve to the current path.
func (r *Recorder) CubicTo(c1x, c1y, c2x, c2y, x, y float64) {
	r.currentPath.CubicTo(c1x, c1y, c2x, c2y, x, y)
}

// ClosePath closes the current subpath.
func (r *Recorder) ClosePath() {
	r.currentPath.Close()
}

// ClearPath clears the current path.
func (r *Recorder) ClearPath() {
	r.currentPath.Clear()
}

// NewSubPath starts a new subpath without closing the previous one.
// This is a no-op as MoveTo already creates a new subpath.
func (r *Recorder) NewSubPath() {
	// No-op, provided for API compatibility
}

// --------------------------------------------------------------------------
// Drawing
// --------------------------------------------------------------------------

// Fill fills the current path and clears it. Recording cannot fail before
// rasterization, so the error is always nil; it exists to match
// render.Context.Fill, which lets the same paint code target either surface.
func (r *Recorder) Fill() error {
	if r.currentPath.NumVerbs() == 0 {
		return nil
	}

	pathRef := r.resources.AddPath(r.currentPath)
	brushRef := r.resources.AddBrush(r.fillBrush)

	r.commands = append(r.commands, FillPathCommand{
		Path:  pathRef,
		Brush: brushRef,
		Rule:  r.fillRule,
	})

	r.currentPath = render.NewPath()
	r.pathBasis = r.transform
	return nil
}

// FillPreserve fills the current path without clearing it.
func (r *Recorder) FillPreserve() error {
	return r.recordFill(false)
}

// recordFill emits a fill of the current path, optionally clearing it.
func (r *Recorder) recordFill(clear bool) error {
	if r.currentPath.NumVerbs() == 0 {
		return nil
	}

	pathRef := r.resources.AddPath(r.currentPath)
	brushRef := r.resources.AddBrush(r.fillBrush)

	r.commands = append(r.commands, FillPathCommand{
		Path:  pathRef,
		Brush: brushRef,
		Rule:  r.fillRule,
	})
	if clear {
		r.currentPath = render.NewPath()
		r.pathBasis = r.transform
	}
	return nil
}

// Stroke strokes the current path and clears it.
func (r *Recorder) Stroke() error {
	return r.recordStroke(true)
}

// StrokePreserve strokes the current path without clearing it.
func (r *Recorder) StrokePreserve() error {
	return r.recordStroke(false)
}

// recordStroke emits a stroke of the current path, optionally clearing it.
func (r *Recorder) recordStroke(clear bool) error {
	if r.currentPath.NumVerbs() == 0 {
		return nil
	}

	pathRef := r.resources.AddPath(r.currentPath)
	brushRef := r.resources.AddBrush(r.strokeBrush)

	stroke := Stroke{
		Width:       r.lineWidth,
		Cap:         r.lineCap,
		Join:        r.lineJoin,
		MiterLimit:  r.miterLimit,
		DashPattern: r.dashPattern,
		DashOffset:  r.dashOffset,
	}

	r.commands = append(r.commands, StrokePathCommand{
		Path:   pathRef,
		Brush:  brushRef,
		Stroke: stroke,
	})
	if clear {
		r.currentPath = render.NewPath()
		r.pathBasis = r.transform
	}
	return nil
}

// FillStroke fills and then strokes the current path, then clears it.
func (r *Recorder) FillStroke() error {
	if err := r.FillPreserve(); err != nil {
		return err
	}
	return r.Stroke()
}

// --------------------------------------------------------------------------
// Shapes
// --------------------------------------------------------------------------

// DrawPoint draws a single point at the given coordinates.
func (r *Recorder) DrawPoint(x, y, radius float64) {
	r.DrawCircle(x, y, radius)
}

// DrawLine draws a line between two points.
func (r *Recorder) DrawLine(x1, y1, x2, y2 float64) {
	r.MoveTo(x1, y1)
	r.LineTo(x2, y2)
}

// DrawRectangle draws a rectangle.
func (r *Recorder) DrawRectangle(x, y, w, h float64) {
	r.MoveTo(x, y)
	r.LineTo(x+w, y)
	r.LineTo(x+w, y+h)
	r.LineTo(x, y+h)
	r.ClosePath()
}

// DrawRoundedRectangle draws a rectangle with rounded corners.
func (r *Recorder) DrawRoundedRectangle(x, y, w, h, radius float64) {
	// Clamp radius to half of the smaller dimension
	maxR := math.Min(w, h) / 2
	if radius > maxR {
		radius = maxR
	}

	r.MoveTo(x+radius, y)
	r.LineTo(x+w-radius, y)
	r.drawArcPath(x+w-radius, y+radius, radius, -math.Pi/2, 0)
	r.LineTo(x+w, y+h-radius)
	r.drawArcPath(x+w-radius, y+h-radius, radius, 0, math.Pi/2)
	r.LineTo(x+radius, y+h)
	r.drawArcPath(x+radius, y+h-radius, radius, math.Pi/2, math.Pi)
	r.LineTo(x, y+radius)
	r.drawArcPath(x+radius, y+radius, radius, math.Pi, 3*math.Pi/2)
	r.ClosePath()
}

// DrawCircle draws a circle.
func (r *Recorder) DrawCircle(x, y, radius float64) {
	const k = 0.5522847498307936 // 4/3 * (sqrt(2) - 1)
	offset := radius * k

	r.MoveTo(x+radius, y)
	r.CubicTo(x+radius, y+offset, x+offset, y+radius, x, y+radius)
	r.CubicTo(x-offset, y+radius, x-radius, y+offset, x-radius, y)
	r.CubicTo(x-radius, y-offset, x-offset, y-radius, x, y-radius)
	r.CubicTo(x+offset, y-radius, x+radius, y-offset, x+radius, y)
	r.ClosePath()
}

// DrawEllipse draws an ellipse.
func (r *Recorder) DrawEllipse(x, y, rx, ry float64) {
	const k = 0.5522847498307936
	ox := rx * k
	oy := ry * k

	r.MoveTo(x+rx, y)
	r.CubicTo(x+rx, y+oy, x+ox, y+ry, x, y+ry)
	r.CubicTo(x-ox, y+ry, x-rx, y+oy, x-rx, y)
	r.CubicTo(x-rx, y-oy, x-ox, y-ry, x, y-ry)
	r.CubicTo(x+ox, y-ry, x+rx, y-oy, x+rx, y)
	r.ClosePath()
}

// DrawArc draws a circular arc.
func (r *Recorder) DrawArc(x, y, radius, angle1, angle2 float64) {
	r.drawArcPath(x, y, radius, angle1, angle2)
}

// drawArcPath adds arc segments to the current path.
func (r *Recorder) drawArcPath(cx, cy, radius, angle1, angle2 float64) {
	const twoPi = 2 * math.Pi
	for angle2 < angle1 {
		angle2 += twoPi
	}

	const maxAngle = math.Pi / 2
	numSegments := int(math.Ceil((angle2 - angle1) / maxAngle))
	angleStep := (angle2 - angle1) / float64(numSegments)

	for i := 0; i < numSegments; i++ {
		a1 := angle1 + float64(i)*angleStep
		a2 := a1 + angleStep
		r.arcSegment(cx, cy, radius, a1, a2)
	}
}

// arcSegment adds a single arc segment using cubic Bezier curves.
// Endpoints and control handles route through MoveTo/CubicTo, so the recorded
// arc follows the same user-space path as Context's own arc construction.
func (r *Recorder) arcSegment(cx, cy, radius, a1, a2 float64) {
	alpha := math.Sin(a2-a1) * (math.Sqrt(4+3*math.Tan((a2-a1)/2)*math.Tan((a2-a1)/2)) - 1) / 3

	cos1, sin1 := math.Cos(a1), math.Sin(a1)
	cos2, sin2 := math.Cos(a2), math.Sin(a2)

	x1 := cx + radius*cos1
	y1 := cy + radius*sin1
	x2 := cx + radius*cos2
	y2 := cy + radius*sin2

	c1x := x1 - alpha*radius*sin1
	c1y := y1 + alpha*radius*cos1
	c2x := x2 + alpha*radius*sin2
	c2y := y2 - alpha*radius*cos2

	if r.currentPath.NumVerbs() == 0 {
		r.MoveTo(x1, y1)
	}
	r.CubicTo(c1x, c1y, c2x, c2y, x2, y2)
}

// DrawEllipticalArc draws an elliptical arc.
func (r *Recorder) DrawEllipticalArc(x, y, rx, ry, angle1, angle2 float64) {
	r.Save()
	r.Translate(x, y)
	r.Scale(rx, ry)
	r.DrawArc(0, 0, 1, angle1, angle2)
	// Restore state but keep path changes
	if len(r.stateStack) > 0 {
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
		r.transform = state.transform
		r.syncPathBasis()
	}
	r.commands = append(r.commands, RestoreCommand{})
}

// --------------------------------------------------------------------------
// Rectangles (Optimized)
// --------------------------------------------------------------------------

// FillRectangle fills a rectangle without adding it to the path.
// This is an optimized operation for the common case of axis-aligned rectangles.
func (r *Recorder) FillRectangle(x, y, w, h float64) {
	rect := NewRect(x, y, w, h)
	brushRef := r.resources.AddBrush(r.fillBrush)

	r.commands = append(r.commands, FillRectCommand{
		Rect:  rect,
		Brush: brushRef,
	})
}

// StrokeRectangle strokes a rectangle without adding it to the path.
func (r *Recorder) StrokeRectangle(x, y, w, h float64) {
	rect := NewRect(x, y, w, h)
	brushRef := r.resources.AddBrush(r.strokeBrush)

	stroke := Stroke{
		Width:       r.lineWidth,
		Cap:         r.lineCap,
		Join:        r.lineJoin,
		MiterLimit:  r.miterLimit,
		DashPattern: r.dashPattern,
		DashOffset:  r.dashOffset,
	}

	r.commands = append(r.commands, StrokeRectCommand{
		Rect:   rect,
		Brush:  brushRef,
		Stroke: stroke,
	})
}

// --------------------------------------------------------------------------
// Clipping
// --------------------------------------------------------------------------

// Clip sets the current path as the clipping region and clears the path.
func (r *Recorder) Clip() {
	if r.currentPath.NumVerbs() == 0 {
		return
	}

	pathRef := r.resources.AddPath(r.currentPath)
	r.commands = append(r.commands, SetClipCommand{
		Path: pathRef,
		Rule: r.fillRule,
	})

	r.currentPath = render.NewPath()
}

// ClipPreserve sets the current path as the clipping region but keeps the path.
func (r *Recorder) ClipPreserve() {
	if r.currentPath.NumVerbs() == 0 {
		return
	}

	pathRef := r.resources.AddPath(r.currentPath)
	r.commands = append(r.commands, SetClipCommand{
		Path: pathRef,
		Rule: r.fillRule,
	})
}

// ResetClip removes all clipping regions.
func (r *Recorder) ResetClip() {
	r.commands = append(r.commands, ClearClipCommand{})
}

// ClipRoundRect sets a rounded rectangle as the clipping region.
// The rectangle is defined in user-space coordinates with uniform corner radius.
func (r *Recorder) ClipRoundRect(x, y, w, h, radius float64) {
	r.commands = append(r.commands, ClipRoundRectCommand{
		X: x, Y: y, W: w, H: h, Radius: radius,
	})
}

// --------------------------------------------------------------------------
// Text
// --------------------------------------------------------------------------

// SetFont sets the current font face for text drawing.
func (r *Recorder) SetFont(face text.Face) {
	r.fontFace = face
}

// currentFontRef stores the current face in the resource pool. Text commands
// carry the reference so playback backends can use the actual face; the old
// encoder dropped it and playback always passed nil to DrawText.
func (r *Recorder) currentFontRef() FontRef {
	if r.fontFace == nil {
		return FontRef(InvalidRef)
	}
	return r.resources.AddFont(r.fontFace)
}

// SetFontSize sets the current font size in points.
func (r *Recorder) SetFontSize(size float64) {
	r.fontSize = size
}

// SetFontFamily sets the current font family name.
func (r *Recorder) SetFontFamily(family string) {
	r.fontFamily = family
}

// DrawString draws text at position (x, y) where y is the baseline.
func (r *Recorder) DrawString(s string, x, y float64) {
	brushRef := r.resources.AddBrush(r.fillBrush)

	r.commands = append(r.commands, DrawTextCommand{
		Text:       s,
		X:          x,
		Y:          y,
		FontSize:   r.fontSize,
		FontFamily: r.fontFamily,
		Font:       r.currentFontRef(),
		Brush:      brushRef,
	})
}

// DrawStringAnchored draws text with an anchor point.
// The anchor point is specified by ax and ay, which are in the range [0, 1].
func (r *Recorder) DrawStringAnchored(s string, x, y, ax, ay float64) {
	// For recording, we store the base position and let the backend handle anchoring
	// This is a simplification; a full implementation would measure text
	r.DrawString(s, x, y)
}

// StrokeString strokes text outlines at position (x, y) where y is the baseline.
// The stroke width, cap, join, and dash are captured from the current recorder state.
func (r *Recorder) StrokeString(s string, x, y float64) {
	brushRef := r.resources.AddBrush(r.strokeBrush)

	stroke := Stroke{
		Width:       r.lineWidth,
		Cap:         r.lineCap,
		Join:        r.lineJoin,
		MiterLimit:  r.miterLimit,
		DashPattern: r.dashPattern,
		DashOffset:  r.dashOffset,
	}

	r.commands = append(r.commands, StrokeTextCommand{
		Text:       s,
		X:          x,
		Y:          y,
		FontSize:   r.fontSize,
		FontFamily: r.fontFamily,
		Font:       r.currentFontRef(),
		Brush:      brushRef,
		Stroke:     stroke,
	})
}

// StrokeStringAnchored strokes text outlines with an anchor point.
// The anchor point is specified by ax and ay, which are in the range [0, 1].
func (r *Recorder) StrokeStringAnchored(s string, x, y, ax, ay float64) {
	// For recording, we store the base position and let the backend handle anchoring
	// This is a simplification; a full implementation would measure text
	r.StrokeString(s, x, y)
}

// MeasureString returns approximate dimensions of text.
// Note: Actual measurement depends on the backend and font.
// Returns (0, 0) if no font is set.
func (r *Recorder) MeasureString(s string) (w, h float64) {
	if r.fontFace == nil {
		// Approximate measurement based on font size
		// Average character width is roughly 0.6 * font size
		w = float64(len(s)) * r.fontSize * 0.6
		h = r.fontSize * 1.2
		return
	}
	return text.Measure(s, r.fontFace)
}

// WordWrap wraps text to fit within the given width using word boundaries.
// Uses the current font face for text measurement.
// If no font face is set, returns the input string as a single-element slice.
func (r *Recorder) WordWrap(s string, w float64) []string {
	if r.fontFace == nil {
		return []string{s}
	}
	results := text.WrapText(s, r.fontFace, w, text.WrapWord)
	lines := make([]string, len(results))
	for i, res := range results {
		// Match Context.WordWrap (and gg): separators at the break point are
		// not part of the returned line.
		lines[i] = strings.TrimSpace(res.Text)
	}
	return lines
}

// MeasureMultilineString measures text that may contain newlines.
// Returns (width, height) where width is the maximum line width.
// If no font face is set, returns (0, 0).
func (r *Recorder) MeasureMultilineString(s string, lineSpacing float64) (width, height float64) {
	if r.fontFace == nil {
		return 0, 0
	}
	lines := recorderSplitLines(s)
	metrics := r.fontFace.Metrics()
	fh := metrics.LineHeight()
	for _, line := range lines {
		lw, _ := text.Measure(line, r.fontFace)
		if lw > width {
			width = lw
		}
	}
	// Keep the same visual block height as Context.MeasureMultilineString:
	// first ascent + inter-line gaps + last descent.
	n := float64(len(lines))
	height = (n-1)*fh*lineSpacing + metrics.Ascent + metrics.Descent
	return
}

// DrawStringWrapped wraps text to the given width and draws it with alignment.
// Each wrapped line is recorded as a separate DrawText command.
func (r *Recorder) DrawStringWrapped(s string, x, y, ax, ay, width, lineSpacing float64, align text.Alignment) {
	lines := r.WordWrap(s, width)
	if len(lines) == 0 {
		return
	}

	var fh, ascent, descent float64
	if r.fontFace != nil {
		m := r.fontFace.Metrics()
		fh, ascent, descent = m.LineHeight(), m.Ascent, m.Descent
	} else {
		fh = r.fontSize * 1.2
		ascent, descent = fh, 0
	}

	// Total visual height (same formula as MeasureMultilineString).
	n := float64(len(lines))
	h := (n-1)*fh*lineSpacing + ascent + descent

	// Adjust starting position by anchor. DrawString records a baseline, so
	// the first baseline sits one ascent below the block top.
	x -= ax * width
	y = y + ascent - ay*h

	// Adjust x base for alignment
	switch align {
	case text.AlignCenter:
		x += width / 2
	case text.AlignRight:
		x += width
	}

	for _, line := range lines {
		drawX := x
		switch align {
		case text.AlignCenter:
			lw, _ := r.MeasureString(line)
			drawX = x - lw/2
		case text.AlignRight:
			lw, _ := r.MeasureString(line)
			drawX = x - lw
		}
		r.DrawString(line, drawX, y)
		y += fh * lineSpacing
	}
}

// recorderSplitLines splits text by line breaks, normalizing \r\n and \r to \n.
func recorderSplitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.Split(s, "\n")
}

// --------------------------------------------------------------------------
// Utility Methods
// --------------------------------------------------------------------------

// Clear resets the entire canvas to transparent (zero alpha),
// matching [render.Context.Clear] semantics. To fill with a specific
// background color, use [Recorder.ClearWithColor].
func (r *Recorder) Clear() {
	r.ClearWithColor(render.Transparent)
}

// ClearWithColor fills the entire canvas with the specified color.
// This is the recommended way to set a background color before drawing.
func (r *Recorder) ClearWithColor(c render.RGBA) {
	oldBrush := r.fillBrush
	r.fillBrush = NewSolidBrush(c)
	r.FillRectangle(0, 0, float64(r.width), float64(r.height))
	r.fillBrush = oldBrush
}

// GetCurrentPoint returns the current point of the path.
// Returns (0, 0, false) if there is no current point.
func (r *Recorder) GetCurrentPoint() (x, y float64, ok bool) {
	if r.currentPath == nil || !r.currentPath.HasCurrentPoint() {
		return 0, 0, false
	}
	pt := r.currentPath.CurrentPoint()
	return pt.X, pt.Y, true
}

// Package scene holds retained scene types for L1 (Layer tree, FramePacket, Picture).
package scene

import "github.com/energye/gpui/render"

// PictureOpKind identifies a recorded draw command in a Picture display list.
type PictureOpKind int

const (
	// OpFillRect fills an axis-aligned rectangle.
	OpFillRect PictureOpKind = iota + 1
	// OpStrokeRect strokes an axis-aligned rectangle.
	OpStrokeRect
)

// PictureOp is one retained draw command (Flutter Picture display-list subset).
// Coordinates are absolute on the target Context at replay time (Y-down).
type PictureOp struct {
	Kind       PictureOpKind
	X, Y, W, H float64
	R, G, B, A float64
	// LineWidth is used by OpStrokeRect (≤0 → 1 at replay).
	LineWidth float64
}

// Picture is a retained draw-ops handle (display list).
// Zero value means "no retained picture" (empty ops, invalid).
//
// Valid is false when the picture must be re-recorded.
// Ops holds the recorded display list (may be non-empty while Valid is false
// until the next successful EndRecording replaces them).
type Picture struct {
	// ID is stable identity for cache keys (optional).
	ID uint64
	// Valid is false when the picture must be re-recorded.
	Valid bool
	// Ops is the retained display list (nil/empty = no recorded content).
	Ops []PictureOp
}

// OpCount returns the number of recorded draw ops.
func (p *Picture) OpCount() int {
	if p == nil {
		return 0
	}
	return len(p.Ops)
}

// IsEmpty reports whether there are no recorded ops.
func (p *Picture) IsEmpty() bool {
	return p == nil || len(p.Ops) == 0
}

// Invalidate marks the picture for re-record (does not clear Ops until re-record).
func (p *Picture) Invalidate() {
	if p == nil {
		return
	}
	p.Valid = false
}

// Clear drops all ops and marks invalid.
func (p *Picture) Clear() {
	if p == nil {
		return
	}
	p.Ops = nil
	p.Valid = false
}

// Replay applies recorded ops onto dc (CPU/GPU Context). No-op if p is nil/empty
// or dc is nil. Does not change Valid/NeedsRaster flags.
func (p *Picture) Replay(dc *render.Context) {
	if p == nil || dc == nil || len(p.Ops) == 0 {
		return
	}
	for i := range p.Ops {
		applyPictureOp(dc, &p.Ops[i])
	}
}

func applyPictureOp(dc *render.Context, op *PictureOp) {
	if dc == nil || op == nil || op.W <= 0 || op.H <= 0 {
		return
	}
	a := op.A
	if a == 0 && (op.R != 0 || op.G != 0 || op.B != 0) {
		a = 1
	}
	dc.SetRGBA(op.R, op.G, op.B, a)
	switch op.Kind {
	case OpFillRect:
		dc.DrawRectangle(op.X, op.Y, op.W, op.H)
		_ = dc.Fill()
	case OpStrokeRect:
		lw := op.LineWidth
		if lw <= 0 {
			lw = 1
		}
		dc.SetLineWidth(lw)
		dc.DrawRectangle(op.X, op.Y, op.W, op.H)
		_ = dc.Stroke()
	}
}

// PictureRecorder builds a Picture display list (Flutter PictureRecorder subset).
// Call drawing methods then EndRecording / Finish.
type PictureRecorder struct {
	ops []PictureOp
}

// NewPictureRecorder starts an empty recording session.
func NewPictureRecorder() *PictureRecorder {
	return &PictureRecorder{}
}

// FillRect records a filled rectangle (R,G,B,A in 0..1).
func (r *PictureRecorder) FillRect(x, y, w, h, red, gre, blu, a float64) {
	if r == nil || w <= 0 || h <= 0 {
		return
	}
	r.ops = append(r.ops, PictureOp{
		Kind: OpFillRect,
		X:    x, Y: y, W: w, H: h,
		R: red, G: gre, B: blu, A: a,
	})
}

// StrokeRect records a stroked rectangle.
func (r *PictureRecorder) StrokeRect(x, y, w, h, lineWidth, red, gre, blu, a float64) {
	if r == nil || w <= 0 || h <= 0 {
		return
	}
	r.ops = append(r.ops, PictureOp{
		Kind: OpStrokeRect,
		X:    x, Y: y, W: w, H: h,
		R: red, G: gre, B: blu, A: a,
		LineWidth: lineWidth,
	})
}

// OpCount returns ops recorded so far (before EndRecording).
func (r *PictureRecorder) OpCount() int {
	if r == nil {
		return 0
	}
	return len(r.ops)
}

// EndRecording finishes the session and returns a Valid Picture with a copy of ops.
// The recorder is reset (empty) after EndRecording.
func (r *PictureRecorder) EndRecording() Picture {
	if r == nil {
		return Picture{}
	}
	ops := append([]PictureOp(nil), r.ops...)
	r.ops = nil
	return Picture{
		Valid: len(ops) > 0,
		Ops:   ops,
	}
}

// Finish writes the recording into dst (replaces Ops), sets Valid from non-empty,
// and resets the recorder. dst may be nil (no-op besides reset).
func (r *PictureRecorder) Finish(dst *Picture) {
	pic := r.EndRecording()
	if dst == nil {
		return
	}
	dst.Ops = pic.Ops
	dst.Valid = pic.Valid
}

// RecordInto records via fn into an existing Picture (clears previous ops).
func RecordInto(dst *Picture, fn func(*PictureRecorder)) {
	if dst == nil || fn == nil {
		return
	}
	rec := NewPictureRecorder()
	fn(rec)
	rec.Finish(dst)
}

// RecordPicture is a convenience that returns a new recorded Picture.
func RecordPicture(fn func(*PictureRecorder)) Picture {
	rec := NewPictureRecorder()
	if fn != nil {
		fn(rec)
	}
	return rec.EndRecording()
}

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
	"github.com/energye/gpui/render"
)

// Brush represents a fill/stroke style for recording commands.
// This is a sealed interface - only types in this package implement it.
//
// Unlike render.Brush which is designed for immediate-mode rendering with ColorAt,
// recording.Brush stores the brush definition for later playback to different
// backends (raster, PDF, SVG).
type Brush interface {
	// brushMarker is an unexported method that seals this interface.
	brushMarker()
}

// SolidBrush is a solid color brush.
type SolidBrush struct {
	Color render.RGBA
}

func (SolidBrush) brushMarker() {}

// NewSolidBrush creates a solid color brush.
func NewSolidBrush(color render.RGBA) SolidBrush {
	return SolidBrush{Color: color}
}

// LinearGradientBrush is a linear gradient brush.
// Colors transition linearly from Start to End.
type LinearGradientBrush struct {
	Start  render.Point   // Start point of the gradient
	End    render.Point   // End point of the gradient
	Stops  []GradientStop // Color stops defining the gradient
	Extend ExtendMode     // How gradient extends beyond bounds
}

func (LinearGradientBrush) brushMarker() {}

// NewLinearGradientBrush creates a new linear gradient brush.
func NewLinearGradientBrush(x0, y0, x1, y1 float64) *LinearGradientBrush {
	return &LinearGradientBrush{
		Start:  render.Point{X: x0, Y: y0},
		End:    render.Point{X: x1, Y: y1},
		Stops:  nil,
		Extend: ExtendPad,
	}
}

// AddColorStop adds a color stop at the specified offset.
// Offset should be in the range [0, 1].
// Returns the gradient for method chaining.
func (g *LinearGradientBrush) AddColorStop(offset float64, color render.RGBA) *LinearGradientBrush {
	g.Stops = append(g.Stops, GradientStop{Offset: offset, Color: color})
	return g
}

// SetExtend sets the extend mode for the gradient.
// Returns the gradient for method chaining.
func (g *LinearGradientBrush) SetExtend(mode ExtendMode) *LinearGradientBrush {
	g.Extend = mode
	return g
}

// RadialGradientBrush is a radial gradient brush.
// Colors radiate from Focus within a circle defined by Center and EndRadius.
type RadialGradientBrush struct {
	Center      render.Point   // Center of the gradient circle
	Focus       render.Point   // Focal point (can differ from center)
	StartRadius float64        // Inner radius where gradient begins (t=0)
	EndRadius   float64        // Outer radius where gradient ends (t=1)
	Stops       []GradientStop // Color stops defining the gradient
	Extend      ExtendMode     // How gradient extends beyond bounds
}

func (RadialGradientBrush) brushMarker() {}

// NewRadialGradientBrush creates a new radial gradient brush.
// The gradient transitions from startRadius to endRadius around (cx, cy).
// Focus defaults to center.
func NewRadialGradientBrush(cx, cy, startRadius, endRadius float64) *RadialGradientBrush {
	center := render.Point{X: cx, Y: cy}
	return &RadialGradientBrush{
		Center:      center,
		Focus:       center, // Default focus at center
		StartRadius: startRadius,
		EndRadius:   endRadius,
		Stops:       nil,
		Extend:      ExtendPad,
	}
}

// SetFocus sets the focal point of the gradient.
// Returns the gradient for method chaining.
func (g *RadialGradientBrush) SetFocus(fx, fy float64) *RadialGradientBrush {
	g.Focus = render.Point{X: fx, Y: fy}
	return g
}

// AddColorStop adds a color stop at the specified offset.
// Offset should be in the range [0, 1].
// Returns the gradient for method chaining.
func (g *RadialGradientBrush) AddColorStop(offset float64, color render.RGBA) *RadialGradientBrush {
	g.Stops = append(g.Stops, GradientStop{Offset: offset, Color: color})
	return g
}

// SetExtend sets the extend mode for the gradient.
// Returns the gradient for method chaining.
func (g *RadialGradientBrush) SetExtend(mode ExtendMode) *RadialGradientBrush {
	g.Extend = mode
	return g
}

// SweepGradientBrush is an angular (conic) gradient brush.
// Colors sweep from StartAngle to EndAngle around Center.
type SweepGradientBrush struct {
	Center     render.Point   // Center of the sweep
	StartAngle float64        // Start angle in radians
	EndAngle   float64        // End angle in radians
	Stops      []GradientStop // Color stops defining the gradient
	Extend     ExtendMode     // How gradient extends beyond bounds
}

func (SweepGradientBrush) brushMarker() {}

// NewSweepGradientBrush creates a new sweep (conic) gradient brush.
// The gradient sweeps a full 360 degrees by default.
func NewSweepGradientBrush(cx, cy, startAngle float64) *SweepGradientBrush {
	const twoPi = 6.283185307179586 // 2 * math.Pi
	return &SweepGradientBrush{
		Center:     render.Point{X: cx, Y: cy},
		StartAngle: startAngle,
		EndAngle:   startAngle + twoPi,
		Stops:      nil,
		Extend:     ExtendPad,
	}
}

// SetEndAngle sets the end angle of the sweep.
// Returns the gradient for method chaining.
func (g *SweepGradientBrush) SetEndAngle(endAngle float64) *SweepGradientBrush {
	g.EndAngle = endAngle
	return g
}

// AddColorStop adds a color stop at the specified offset.
// Offset should be in the range [0, 1].
// Returns the gradient for method chaining.
func (g *SweepGradientBrush) AddColorStop(offset float64, color render.RGBA) *SweepGradientBrush {
	g.Stops = append(g.Stops, GradientStop{Offset: offset, Color: color})
	return g
}

// SetExtend sets the extend mode for the gradient.
// Returns the gradient for method chaining.
func (g *SweepGradientBrush) SetExtend(mode ExtendMode) *SweepGradientBrush {
	g.Extend = mode
	return g
}

// PatternBrush is an image pattern brush: one sub-rect of a pooled image,
// tiled across the surface through a fixed sampling inverse. It is the recorded
// form of render.NewImagePattern — the whole description, so replay rebuilds a
// pattern that samples exactly the same texels as the live one did.
//
// SrcW/SrcH of 0 mean "whole image dimension" and Opacity of 0 means
// "unspecified" (both are render.ImagePattern's own conventions).
type PatternBrush struct {
	Image   *render.ImageBuf // source image, held by pointer as every pooled resource is
	SrcX    int              // source region offset within the image
	SrcY    int
	SrcW    int // source region size (0 = full image dimension)
	SrcH    int
	Inverse render.Matrix // device-space -> image-space, as the pattern stores it
	Opacity float64       // opacity multiplier (0 = default 1)
	Clamp   bool          // out-of-range coordinates return transparent instead of tiling
}

func (PatternBrush) brushMarker() {}

// NewPatternBrush creates a new pattern brush tiling the whole of the image
// from the origin.
func NewPatternBrush(img *render.ImageBuf) *PatternBrush {
	return &PatternBrush{
		Image:   img,
		Inverse: render.Identity(),
	}
}

// SetSourceRegion restricts the pattern to a sub-rect of the image. w/h of 0
// mean the full image dimension. Returns the brush for method chaining.
func (b *PatternBrush) SetSourceRegion(x, y, w, h int) *PatternBrush {
	b.SrcX, b.SrcY, b.SrcW, b.SrcH = x, y, w, h
	return b
}

// SetInverse sets the device-space -> image-space sampling matrix, which is
// what an image pattern actually stores (a forward transform would have to be
// inverted at replay, and inverting twice is not bit-exact).
// Returns the brush for method chaining.
func (b *PatternBrush) SetInverse(m render.Matrix) *PatternBrush {
	b.Inverse = m
	return b
}

// SetOpacity sets the opacity multiplier; 0 means unspecified (1).
// Returns the brush for method chaining.
func (b *PatternBrush) SetOpacity(opacity float64) *PatternBrush {
	b.Opacity = opacity
	return b
}

// SetClamp makes coordinates outside the source region transparent instead of
// tiling. Returns the brush for method chaining.
func (b *PatternBrush) SetClamp(clamp bool) *PatternBrush {
	b.Clamp = clamp
	return b
}

// Source renders the recorded brush as render's own pattern description, the
// form replay hands to the context.
func (b *PatternBrush) Source() render.ImagePatternSource {
	return render.ImagePatternSource{
		Image:         b.Image,
		SrcX:          b.SrcX,
		SrcY:          b.SrcY,
		SrcW:          b.SrcW,
		SrcH:          b.SrcH,
		Inverse:       b.Inverse,
		Opacity:       b.Opacity,
		ClampToSource: b.Clamp,
	}
}

// CustomPatternBrush carries a Pattern implementation that is not data —
// a caller-supplied ColorAt, in other words a closure. It has no exportable
// description (PDF/SVG backends cannot write one either), so replay hands the
// very same pattern to the target context.
//
// The contract is the one every retained packet already lives under: the
// pattern becomes read-only when it is recorded, exactly like the text.Face a
// recording holds. A caller that keeps mutating a live pattern object while the
// recording may replay it is racing, not painting.
type CustomPatternBrush struct {
	Pattern render.Pattern
}

func (CustomPatternBrush) brushMarker() {}

// NewCustomPatternBrush records an arbitrary render.Pattern by reference.
func NewCustomPatternBrush(p render.Pattern) CustomPatternBrush {
	return CustomPatternBrush{Pattern: p}
}

// GradientStop defines a color stop in a gradient.
type GradientStop struct {
	Offset float64     // Position in gradient, 0.0 to 1.0
	Color  render.RGBA // Color at this position
}

// ExtendMode defines how gradients extend beyond their defined bounds.
type ExtendMode int

const (
	// ExtendPad extends edge colors beyond bounds (default behavior).
	ExtendPad ExtendMode = iota
	// ExtendRepeat repeats the gradient pattern.
	ExtendRepeat
	// ExtendReflect mirrors the gradient pattern.
	ExtendReflect
)

// ToRenderPattern converts a recorded brush back to a render.Pattern for
// playback onto a live context. It returns false for brushes that are not
// patterns (solid, gradient), which the caller replays as brushes. A true with a
// nil Pattern means "this was a pattern, but there is nothing to sample" — the
// recorded image is gone — and the caller must paint nothing rather than invent
// a colour.
func ToRenderPattern(b Brush) (render.Pattern, bool) {
	switch br := b.(type) {
	case *PatternBrush:
		if br.Image == nil || br.Image.Disposed() {
			// The buffer is gone between recording and replay. Handing back a
			// zero-sized pattern would divide by zero in the tiling sampler, so
			// this reports "no pattern" and the caller paints nothing.
			return nil, true
		}
		return render.NewImagePatternFromSource(br.Source()), true
	case CustomPatternBrush:
		return br.Pattern, true
	default:
		return nil, false
	}
}

// FromRenderPattern converts a render.Pattern into a recorded brush. render's
// own paint treats a solid pattern as an inline colour, an image pattern as a
// data description, and anything else as a live sampler, so the recording splits
// the same three ways. The pool is where the pattern's image gets registered, so
// a retained packet keeps sealing it (see Recording.ImageBufs).
func FromRenderPattern(p render.Pattern, pool *ResourcePool) Brush {
	switch pat := p.(type) {
	case nil:
		return NewSolidBrush(render.Black)
	case *render.SolidPattern:
		return SolidBrush{Color: pat.Color}
	case *render.ImagePattern:
		src := pat.Source()
		if src.Image != nil && pool != nil {
			pool.AddImageBuf(src.Image)
		}
		return (&PatternBrush{Image: src.Image}).
			SetSourceRegion(src.SrcX, src.SrcY, src.SrcW, src.SrcH).
			SetInverse(src.Inverse).
			SetOpacity(src.Opacity).
			SetClamp(src.ClampToSource)
	default:
		return NewCustomPatternBrush(pat)
	}
}

// FromRenderBrush converts a render.Brush to a recording.Brush.
// This extracts the brush definition for storage in a recording.
func FromRenderBrush(b render.Brush) Brush {
	switch brush := b.(type) {
	case render.SolidBrush:
		return SolidBrush{Color: brush.Color}
	case *render.LinearGradientBrush:
		return linearGradientFromRender(brush)
	case *render.RadialGradientBrush:
		return radialGradientFromRender(brush)
	case *render.SweepGradientBrush:
		return sweepGradientFromRender(brush)
	default:
		// For unknown brush types, sample at origin and use as solid color
		// This is a fallback that preserves color but loses patterns
		color := b.ColorAt(0, 0)
		return SolidBrush{Color: color}
	}
}

// ToRenderBrush converts a recording.Brush back to a render.Brush for playback
// onto a render.Context. Pattern brushes have no render.Brush form at all —
// render.Brush is sealed and an image pattern is a Pattern, not a Brush — so
// they return nil here and replay through ToRenderPattern instead. ApplyBrush
// does that routing; a caller that goes straight to SetFillBrush would paint
// black, which is exactly the degradation this function refuses to hide.
func ToRenderBrush(b Brush) render.Brush {
	switch br := b.(type) {
	case SolidBrush:
		return render.Solid(br.Color)
	case *LinearGradientBrush:
		grad := render.NewLinearGradientBrush(br.Start.X, br.Start.Y, br.End.X, br.End.Y)
		for _, stop := range br.Stops {
			grad.AddColorStop(stop.Offset, stop.Color)
		}
		grad.SetExtend(render.ExtendMode(br.Extend))
		return grad
	case *RadialGradientBrush:
		grad := render.NewRadialGradientBrush(br.Center.X, br.Center.Y, br.StartRadius, br.EndRadius)
		grad.SetFocus(br.Focus.X, br.Focus.Y)
		for _, stop := range br.Stops {
			grad.AddColorStop(stop.Offset, stop.Color)
		}
		grad.SetExtend(render.ExtendMode(br.Extend))
		return grad
	case *SweepGradientBrush:
		grad := render.NewSweepGradientBrush(br.Center.X, br.Center.Y, br.StartAngle)
		grad.SetEndAngle(br.EndAngle)
		for _, stop := range br.Stops {
			grad.AddColorStop(stop.Offset, stop.Color)
		}
		grad.SetExtend(render.ExtendMode(br.Extend))
		return grad
	case *PatternBrush, CustomPatternBrush:
		// A pattern is a render.Pattern, not a render.Brush: replay routes it
		// through SetFillPattern/SetStrokePattern. Handing back a colour here
		// would silently repaint a textured surface solid black.
		return nil
	case nil:
		return render.Solid(render.Black)
	default:
		return render.Solid(render.Black)
	}
}

// linearGradientFromRender converts a render.LinearGradientBrush to recording.LinearGradientBrush.
func linearGradientFromRender(g *render.LinearGradientBrush) *LinearGradientBrush {
	stops := make([]GradientStop, len(g.Stops))
	for i, stop := range g.Stops {
		stops[i] = GradientStop{Offset: stop.Offset, Color: stop.Color}
	}
	return &LinearGradientBrush{
		Start:  g.Start,
		End:    g.End,
		Stops:  stops,
		Extend: ExtendMode(g.Extend),
	}
}

// radialGradientFromRender converts a render.RadialGradientBrush to recording.RadialGradientBrush.
func radialGradientFromRender(g *render.RadialGradientBrush) *RadialGradientBrush {
	stops := make([]GradientStop, len(g.Stops))
	for i, stop := range g.Stops {
		stops[i] = GradientStop{Offset: stop.Offset, Color: stop.Color}
	}
	return &RadialGradientBrush{
		Center:      g.Center,
		Focus:       g.Focus,
		StartRadius: g.StartRadius,
		EndRadius:   g.EndRadius,
		Stops:       stops,
		Extend:      ExtendMode(g.Extend),
	}
}

// sweepGradientFromRender converts a render.SweepGradientBrush to recording.SweepGradientBrush.
func sweepGradientFromRender(g *render.SweepGradientBrush) *SweepGradientBrush {
	stops := make([]GradientStop, len(g.Stops))
	for i, stop := range g.Stops {
		stops[i] = GradientStop{Offset: stop.Offset, Color: stop.Color}
	}
	return &SweepGradientBrush{
		Center:     g.Center,
		StartAngle: g.StartAngle,
		EndAngle:   g.EndAngle,
		Stops:      stops,
		Extend:     ExtendMode(g.Extend),
	}
}

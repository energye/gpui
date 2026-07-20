package recording

import (
	"github.com/energye/gpui/render"
)

// --------------------------------------------------------------------------
// Color/Style
// --------------------------------------------------------------------------

// SetColor sets both fill and stroke color from a color.Color.
func (r *Recorder) SetColor(c render.RGBA) {
	brush := NewSolidBrush(c)
	r.fillBrush = brush
	r.strokeBrush = brush
	brushRef := r.resources.AddBrush(brush)
	r.commands = append(r.commands,
		SetFillStyleCommand{Brush: brushRef},
		SetStrokeStyleCommand{Brush: brushRef})
}

// SetRGB sets both fill and stroke color using RGB values (0-1).
func (r *Recorder) SetRGB(red, green, blue float64) {
	r.SetColor(render.RGB(red, green, blue))
}

// SetRGBA sets both fill and stroke color using RGBA values (0-1).
func (r *Recorder) SetRGBA(red, green, blue, alpha float64) {
	r.SetColor(render.RGBA2(red, green, blue, alpha))
}

// SetHexColor sets both fill and stroke color using a hex string.
func (r *Recorder) SetHexColor(hex string) {
	r.SetColor(render.Hex(hex))
}

// SetFillStyle sets the fill brush.
func (r *Recorder) SetFillStyle(brush Brush) {
	r.fillBrush = brush
	brushRef := r.resources.AddBrush(brush)
	r.commands = append(r.commands, SetFillStyleCommand{Brush: brushRef})
}

// SetStrokeStyle sets the stroke brush.
func (r *Recorder) SetStrokeStyle(brush Brush) {
	r.strokeBrush = brush
	brushRef := r.resources.AddBrush(brush)
	r.commands = append(r.commands, SetStrokeStyleCommand{Brush: brushRef})
}

// SetFillBrush sets the fill brush from a gg.Brush.
func (r *Recorder) SetFillBrush(brush render.Brush) {
	r.SetFillStyle(BrushFromGG(brush))
}

// SetStrokeBrush sets the stroke brush from a gg.Brush.
func (r *Recorder) SetStrokeBrush(brush render.Brush) {
	r.SetStrokeStyle(BrushFromGG(brush))
}

// SetFillRGB sets the fill color using RGB values (0-1).
func (r *Recorder) SetFillRGB(red, green, blue float64) {
	r.SetFillStyle(NewSolidBrush(render.RGB(red, green, blue)))
}

// SetFillRGBA sets the fill color using RGBA values (0-1).
func (r *Recorder) SetFillRGBA(red, green, blue, alpha float64) {
	r.SetFillStyle(NewSolidBrush(render.RGBA2(red, green, blue, alpha)))
}

// SetStrokeRGB sets the stroke color using RGB values (0-1).
func (r *Recorder) SetStrokeRGB(red, green, blue float64) {
	r.SetStrokeStyle(NewSolidBrush(render.RGB(red, green, blue)))
}

// SetStrokeRGBA sets the stroke color using RGBA values (0-1).
func (r *Recorder) SetStrokeRGBA(red, green, blue, alpha float64) {
	r.SetStrokeStyle(NewSolidBrush(render.RGBA2(red, green, blue, alpha)))
}

// --------------------------------------------------------------------------
// Line Properties
// --------------------------------------------------------------------------

// SetLineWidth sets the line width for stroking.
func (r *Recorder) SetLineWidth(width float64) {
	r.lineWidth = width
	r.commands = append(r.commands, SetLineWidthCommand{Width: width})
}

// SetLineCap sets the line cap style.
func (r *Recorder) SetLineCap(lc LineCap) {
	r.lineCap = lc
	r.commands = append(r.commands, SetLineCapCommand{Cap: lc})
}

// SetLineCapGG sets the line cap style from gg.LineCap.
func (r *Recorder) SetLineCapGG(lc render.LineCap) {
	// #nosec G115 -- LineCap enum values are within uint8 range
	r.SetLineCap(LineCap(lc))
}

// SetLineJoin sets the line join style.
func (r *Recorder) SetLineJoin(join LineJoin) {
	r.lineJoin = join
	r.commands = append(r.commands, SetLineJoinCommand{Join: join})
}

// SetLineJoinGG sets the line join style from gg.LineJoin.
func (r *Recorder) SetLineJoinGG(join render.LineJoin) {
	// #nosec G115 -- LineJoin enum values are within uint8 range
	r.SetLineJoin(LineJoin(join))
}

// SetMiterLimit sets the miter limit for line joins.
func (r *Recorder) SetMiterLimit(limit float64) {
	r.miterLimit = limit
	r.commands = append(r.commands, SetMiterLimitCommand{Limit: limit})
}

// SetDash sets the dash pattern for stroking.
// Pass alternating dash and gap lengths.
// Passing no arguments clears the dash pattern (returns to solid lines).
func (r *Recorder) SetDash(lengths ...float64) {
	if len(lengths) == 0 {
		r.ClearDash()
		return
	}

	r.dashPattern = make([]float64, len(lengths))
	copy(r.dashPattern, lengths)
	r.commands = append(r.commands, SetDashCommand{Pattern: r.dashPattern, Offset: r.dashOffset})
}

// SetDashOffset sets the starting offset into the dash pattern.
func (r *Recorder) SetDashOffset(offset float64) {
	r.dashOffset = offset
	if r.dashPattern != nil {
		r.commands = append(r.commands, SetDashCommand{Pattern: r.dashPattern, Offset: r.dashOffset})
	}
}

// ClearDash removes the dash pattern, returning to solid lines.
func (r *Recorder) ClearDash() {
	r.dashPattern = nil
	r.dashOffset = 0
	r.commands = append(r.commands, SetDashCommand{Pattern: nil, Offset: 0})
}

// SetFillRule sets the fill rule.
func (r *Recorder) SetFillRule(rule FillRule) {
	r.fillRule = rule
	r.commands = append(r.commands, SetFillRuleCommand{Rule: rule})
}

// SetFillRuleGG sets the fill rule from gg.FillRule.
func (r *Recorder) SetFillRuleGG(rule render.FillRule) {
	// #nosec G115 -- FillRule enum values are within uint8 range
	r.SetFillRule(FillRule(rule))
}

// SetAntiAlias enables or disables anti-aliasing for geometry rendering.
// When disabled, shapes are rendered with binary coverage (no gray edge pixels).
func (r *Recorder) SetAntiAlias(enabled bool) {
	r.antiAlias = enabled
	r.commands = append(r.commands, SetAntiAliasCommand{Enabled: enabled})
}

// AntiAlias returns whether anti-aliasing is enabled.
func (r *Recorder) AntiAlias() bool {
	return r.antiAlias
}

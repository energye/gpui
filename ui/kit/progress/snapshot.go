package progress

import (
	"github.com/energye/gpui/render"
)

// ProgressSnap is the frozen paint input for one progress frame (R2-6, button
// snapshot paradigm). The UI thread refreshes it in markPaint (funnel for
// every paint-affecting setter) and in New; the raster paint path reads only
// this value, never the live Progress. percent/successPercent/phase stay
// atomic live reads (animation/data state); everything theme/config derived
// and all resolved colors are frozen here.
//
// Frozen colors are resolved at refresh time (stroke override else status
// token), so status-Auto colors follow SetPercent through the same funnel.
type ProgressSnap struct {
	Ptype  ProgressType
	Status ProgressStatus // normalized EffectiveStatus at refresh
	Linecap StrokeLinecap // effective cap (round default)

	SolidFill    render.RGBA // stroke override else status token
	SuccessColor render.RGBA // success split color, resolved
	Rail         render.RGBA // rail override else fill-secondary token
	SweepLite    render.RGBA // ColorBgContainer-derived glass highlight

	HasGradient bool
	GradFrom    render.RGBA
	GradTo      render.RGBA

	Steps      int
	StepColors []render.RGBA // copied at refresh
	StepGap    float64       // effective gap (line px / circle degrees)

	GapDegree    float64              // effective dashboard gap (deg)
	GapPlacement ProgressGapPlacement // effective gap side

	ReduceMotion bool
	StrokePct    float64 // effective circle stroke % (strokeWidth or 6)
	LineHeight   float64 // resolved track height (sizePx/size/token)
	CircleSize   float64 // resolved edge (sizePx/size/token)
}

// refreshSnapshot freezes the current paint inputs (UI thread only). Pure
// value build: no raster-side field is touched, so a concurrent paint of
// the previous snapshot is unaffected.
func (p *Progress) refreshSnapshot() {
	if p == nil {
		return
	}
	tok := p.tokens()
	s := ProgressSnap{
		Ptype:        p.ptype,
		Status:       p.EffectiveStatus(),
		Linecap:      p.EffectiveStrokeLinecap(),
		SolidFill:    p.solidFillColor(),
		SuccessColor: p.EffectiveSuccessColor(),
		Rail:         p.EffectiveRailColor(),
		SweepLite:    themeToRGBA(tok.ColorBgContainer),
		HasGradient:  p.hasGradient,
		GradFrom:     p.gradFrom,
		GradTo:       p.gradTo,
		Steps:        p.steps,
		StepGap:      p.EffectiveStepGap(),
		GapDegree:    p.EffectiveGapDegree(),
		GapPlacement: p.EffectiveGapPlacement(),
		ReduceMotion: p.reduceMotion,
		StrokePct:    p.EffectiveStrokePct(),
		LineHeight:   p.LineHeight(),
		CircleSize:   p.CircleSize(),
	}
	if len(p.stepColors) > 0 {
		s.StepColors = make([]render.RGBA, len(p.stepColors))
		copy(s.StepColors, p.stepColors)
	}
	p.snap.Store(s)
}

// loadSnapshot returns the last frozen snapshot; zero value paints nothing
// harmful (zero lines/blocks, resolved token colors come from the caller).
func (p *Progress) loadSnapshot() ProgressSnap {
	if p == nil {
		return ProgressSnap{}
	}
	if v, ok := p.snap.Load().(ProgressSnap); ok {
		return v
	}
	return ProgressSnap{}
}
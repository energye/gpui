package spin

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
)

// SpinSnap is the frozen paint input for one spin frame (R2-6, button
// snapshot paradigm). The UI thread refreshes it on every paint-affecting
// setter and on Tick state flips (delay display, auto percent); the raster
// paint path reads only this value, never the live Spin. phase and indSize
// stay atomic live reads (animation / layout-cache state); everything
// theme/config/visibility derived is frozen here.
type SpinSnap struct {
	Visible  bool // IsDisplaySpinning resolved at refresh (spinning+delay+display)
	WantMask bool // host paint defers to the overlay when a mask is shown
	HasContent bool
	Fullscreen bool

	DotSize  float64
	Gap      float64
	FontSize float64

	IndicatorColor   render.RGBA
	DescriptionColor render.RGBA
	TrackColor       render.RGBA

	Description string
	HasPercent  bool
	EffPercent  float64 // frozen at refresh; Tick refreshes to keep auto smoothing
	Indicator   rendering.RenderObject
	Face        text.Face
}

// refreshSnapshot freezes the current paint inputs (UI thread only). Pure
// value build: no raster-side field is touched, so a concurrent paint of
// the previous snapshot is unaffected.
func (s *Spin) refreshSnapshot() {
	if s == nil {
		return
	}
	v := SpinSnap{
		Visible:          s.IsDisplaySpinning(),
		HasContent:       s.content != nil,
		Fullscreen:       s.fullscreen,
		DotSize:          s.DotSize(),
		Gap:              s.EffectiveGap(),
		FontSize:         s.EffectiveFontSize(),
		IndicatorColor:   s.EffectiveIndicatorColor(),
		DescriptionColor: s.EffectiveDescriptionColor(),
		TrackColor:       s.EffectiveTrackColor(),
		Description:      s.EffectiveDescription(),
		HasPercent:       s.hasPercent,
		EffPercent:       s.EffectivePercent(),
		Indicator:        s.EffectiveIndicator(),
		Face:             s.textFace,
	}
	v.WantMask = v.Visible && (v.HasContent || v.Fullscreen)
	s.snap.Store(v)
}

// loadSnapshot returns the last frozen snapshot; zero value paints nothing
// (Visible=false).
func (s *Spin) loadSnapshot() SpinSnap {
	if s == nil {
		return SpinSnap{}
	}
	if v, ok := s.snap.Load().(SpinSnap); ok {
		return v
	}
	return SpinSnap{}
}
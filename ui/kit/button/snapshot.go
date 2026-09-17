package button

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/kit/icon"
	"github.com/energye/gpui/ui/rendering"
)

// ButtonSnap is the frozen paint input for one button frame (T2 D1).
// The UI thread refreshes it on every dirty/layout (refreshSnapshot); the
// raster-thread paint path reads only this value, never the live Button.
// UI Tick/Pointer/Focus may keep mutating the Button while raster paints —
// the pixels always match the last UI refresh, with no cross-thread reads
// or writes of widget state.
type ButtonSnap struct {
	// Size is the laid-out size at refresh time (reference only; paint uses
	// its size param for canvas bounds, snapshot for content inputs).
	Size rendering.Size
	// Resolved chrome at refresh time (style/semantic/theme already applied).
	Fill, Border, Text    render.RGBA
	Dashed, HasBorder     bool
	Hovered               bool
	IsLink                bool
	LinkUnderline         bool
	Loading               bool
	LoadingOpacity        float64
	UseGradient           bool
	GradFrom, GradTo      render.RGBA
	GradHoverFade         float64 // 0 = full gradient, 1 = hover-faded to base fill
	UseShadow             bool
	Circle                bool
	RadiusBase            float64
	Label                 string
	FontSize              float64
	Face                  text.Face
	HasLead, HasText      bool
	IconFirst             bool
	LeadW, GapW, PadW     float64
	IconName              string
	IconInk               render.RGBA
	HasSpinner            bool
	SpinnerName           string
	SpinnerAngle          float64
	SpinnerInk            render.RGBA
	WaveOK                bool
	WaveSpread, WaveAlpha float64
	WaveColor             render.RGBA
	// Custom wave effects (wave.tsx Inset/Shake): recorded at click time,
	// painted from snapshot inputs.
	WaveShake              float64 // Shake: horizontal silhouette offset
	WaveInset              bool    // Inset: expanding white dot
	WaveInsetX, WaveInsetY float64
	WaveInsetSize          float64
	FocusRing              bool
	Ring                   render.RGBA
	RingWidth              float64
}

// refreshSnapshot freezes the current paint inputs (UI thread only: dirty,
// layout, construction — never on raster).
func (b *Button) refreshSnapshot() {
	if b == nil {
		return
	}
	// Spinner hot-region geometry stays on the UI thread (paint must never
	// write node offsets on raster). Idempotent: size change relayouts the
	// child, otherwise only a paint-neutral offset write.
	b.syncSpin()
	var s ButtonSnap
	s.Size = b.lastSize
	fill, border, text, dashed, hasBorder := b.chrome()
	s.Fill, s.Border, s.Text = fill, border, text
	s.Dashed, s.HasBorder = dashed, hasBorder
	s.Hovered = b.hovered && !b.pressed
	s.IsLink = b.EffectiveVariant() == VariantLink
	// antd link hover shows an underline (optional in §6.5, always on here:
	// one deterministic rule beats a second config).
	s.LinkUnderline = s.IsLink && s.Hovered
	s.Loading = b.loading
	s.LoadingOpacity = b.LoadingOpacity()
	s.UseGradient = b.HasGradient() && !b.ghost && !b.style.UseBg
	if s.UseGradient {
		from, to, _ := b.GradientColors()
		s.GradFrom, s.GradTo = from, to
		// linear-gradient demo: ::before overlay fades on hover, revealing
		// the primary fill beneath (antd transition 0.3s; immediate here).
		if b.hovered {
			s.GradHoverFade = 1
		}
	}
	if b.style.UseShadow {
		s.UseShadow = true
	} else if st, ok := b.SemanticStyle(SemanticRoot); ok && st.UseShadow {
		s.UseShadow = true
	}
	s.Circle = b.shape == ButtonShapeCircle
	s.RadiusBase = b.Radius()
	s.Label = b.DisplayLabel()
	s.FontSize = b.FontSize()
	s.Face = b.textFace
	_, _, _, hasLead, hasText := b.slots(s.Size.Width, s.Size.Height)
	s.HasLead, s.HasText = hasLead, hasText
	s.IconFirst = b.IconFirst()
	s.LeadW = b.leadingWidth()
	s.GapW = b.IconGap()
	s.PadW = b.PaddingInline()
	s.HasSpinner = b.HasSpinner()
	if s.HasSpinner {
		s.SpinnerName = "loading"
		if b.loadingIcon != "" {
			s.SpinnerName = normalizeIconName(b.loadingIcon)
		}
		s.SpinnerAngle = b.spinPhase * 360
		s.SpinnerInk = b.IconColor()
		s.SpinnerInk.A *= b.LoadingOpacity()
	} else if hasLead {
		s.IconName = normalizeIconName(b.iconName)
		s.IconInk = b.IconColor()
	}
	if spread, alpha, ok := b.WaveProgress(); ok {
		s.WaveOK = true
		s.WaveSpread, s.WaveAlpha = spread, alpha
		s.WaveColor = b.waveColor
	}
	if b.waveEffect == WaveEffectInset && b.hasWave {
		s.WaveInset = true
		s.WaveInsetX, s.WaveInsetY = b.waveAtX, b.waveAtY
		if d, _, ok := b.WaveProgress(); ok {
			// Inset dot grows toward 200px over the fade life (§6.9.0).
			s.WaveInsetSize = d / waveSpreadMax * 200
			if s.WaveInsetSize < 2 {
				s.WaveInsetSize = 2
			}
		}
		s.WaveAlpha = 0.65
	}
	if b.waveEffect == WaveEffectShake && b.hasWave {
		if off, _, ok := b.WaveProgress(); ok {
			// Shake path: ±15° sequence approximated as a ±4px horizontal
			// silhouette swing over the spread life.
			s.WaveShake = 4 * (off / waveSpreadMax)
			if int(off*10)%2 == 0 {
				s.WaveShake = -s.WaveShake
			}
		}
	}
	if b.FocusRingVisible() {
		s.FocusRing = true
		tok := b.themeTokens()
		ring := themeToRGBA(tok.ColorPrimaryBorder)
		if ring.A == 0 {
			ring = parseHexRender("#91caff")
		}
		s.Ring = ring
		s.RingWidth = tok.LineWidthFocus
		if s.RingWidth <= 0 {
			s.RingWidth = 3
		}
	}
	b.paintSnap.Store(s)
}

// loadSnapshot returns the last UI refresh (zero value before the first one).
func (b *Button) loadSnapshot() ButtonSnap {
	if b == nil {
		return ButtonSnap{}
	}
	if s, ok := b.paintSnap.Load().(ButtonSnap); ok {
		return s
	}
	return ButtonSnap{}
}

// textWidthOf estimates the DisplayLabel advance (same formula as textWidth:
// ascii 0.6em, wide 1em) without touching the live Button.
func textWidthOf(label string, fontSize float64) float64 {
	if label == "" {
		return 0
	}
	ascii, wide := 0, 0
	for _, r := range label {
		if r < 128 {
			ascii++
		} else {
			wide++
		}
	}
	return (float64(ascii)*0.6 + float64(wide)*1.0) * fontSize
}

// slotsOf splits the content row (same math as slots, from snapshot inputs).
// gap must already be 0 unless both slots are present (slots only separates
// icon and text when both show).
func slotsOf(w, pad, textW, leadW, gap float64, circle, iconFirst, hasLead, hasText bool) (leadCX, textCX, textWOut float64) {
	inner := w - 2*pad
	if circle {
		pad, inner = 0, w
	}
	total := textW + leadW + gap
	x := pad + (inner-total)/2
	if iconFirst {
		if hasLead {
			leadCX = x + leadW/2
			x += leadW + gap
		}
		if hasText {
			textCX, textWOut = x+textW/2, textW
		}
		return leadCX, textCX, textWOut
	}
	if hasText {
		textCX, textWOut = x+textW/2, textW
		x += textW + gap
	}
	if hasLead {
		leadCX = x + leadW/2
	}
	return leadCX, textCX, textWOut
}

// PaintButton paints chrome from a frozen snapshot (raster thread only:
// reads s, never the live Button). Geometry uses the size param; content
// inputs come from s, so output matches paint at refresh time for the same
// size (layout size in every window/test path).
func PaintButton(pc *rendering.PaintContext, size rendering.Size, s ButtonSnap) {
	if pc == nil || size.Width <= 0 || size.Height <= 0 {
		return
	}
	w, h := size.Width, size.Height
	radius := s.RadiusBase
	if s.Circle {
		radius = h / 2
	}
	if s.UseShadow {
		// style-class Object demo: 0 1px 2px rgba(0,0,0,0.05) offset
		// silhouette behind the chrome (button-local, no filter).
		sr := radius + 2
		if sr > h/2+2 {
			sr = h/2 + 2
		}
		rendering.FillRoundRect(pc, 1, 2, w, h, sr,
			0, 0, 0, 0.05)
	}
	layered := s.Loading && pc.DC != nil
	if layered {
		layered = pc.SaveLayer(w, h, s.LoadingOpacity)
	}
	if s.UseGradient {
		// Base fill first (what hover reveals), gradient over it.
		if s.Fill.A > 0 {
			rendering.FillRoundRect(pc, 0, 0, w, h, radius, s.Fill.R, s.Fill.G, s.Fill.B, s.Fill.A)
		}
		pc.PushClipRRect(0, 0, w, h, radius)
		fr, fg, fb, fa := s.GradFrom.R, s.GradFrom.G, s.GradFrom.B, s.GradFrom.A
		tr, tg, tb, ta := s.GradTo.R, s.GradTo.G, s.GradTo.B, s.GradTo.A
		if s.GradHoverFade > 0 {
			// Fade the overlay toward transparent on hover (§6.9.0).
			f := 1 - s.GradHoverFade
			fa *= f
			ta *= f
		}
		rendering.FillLinearGradient(pc, 0, 0, w, h, 0, 0, w, h, fr, fg, fb, fa, tr, tg, tb, ta)
		pc.PopClip()
	} else if s.Fill.A > 0 {
		rendering.FillRoundRect(pc, 0, 0, w, h, radius, s.Fill.R, s.Fill.G, s.Fill.B, s.Fill.A)
	}
	if s.HasBorder && w > 2 && h > 2 {
		if pc.DC != nil && s.Dashed {
			pc.DC.SetDash(dashOn, dashOff)
		}
		r := radius - 0.5
		if r < 0 {
			r = 0
		}
		rendering.StrokeRoundRect(pc, 0.5, 0.5, w-1, h-1, r, 1, s.Border.R, s.Border.G, s.Border.B, s.Border.A)
		if pc.DC != nil && s.Dashed {
			pc.DC.SetDash()
		}
	}
	textW := textWidthOf(s.Label, s.FontSize)
	gap := s.GapW
	if !(s.HasLead && s.HasText) {
		gap = 0
	}
	leadCX, textCX, textWOut := slotsOf(w, s.PadW, textW, s.LeadW, gap, s.Circle, s.IconFirst, s.HasLead, s.HasText)
	cy := h / 2
	if s.HasLead && !s.HasSpinner && s.LeadW > 0 {
		icon.PaintGlyph(pc, s.IconName, leadCX-s.LeadW/2, cy-s.LeadW/2, s.LeadW, s.IconInk, 0)
	}
	if s.HasText && pc.DC != nil && s.Label != "" {
		if s.Face != nil {
			pc.DC.SetFont(s.Face)
		}
		pc.DC.SetRGBA(s.Text.R, s.Text.G, s.Text.B, s.Text.A)
		top := (h - s.FontSize) / 2
		if top < 0 {
			top = 0
		}
		ax, ay := pc.Abs(textCX, top)
		pc.DC.DrawStringAnchored(s.Label, ax, ay, 0.5, 0)
		if s.LinkUnderline {
			// Link hover underline: one text-ink line under the label.
			lw := textWOut
			if lw <= 0 {
				lw = textWidthOf(s.Label, s.FontSize)
			}
			uy := top + s.FontSize + 1
			if uy > h-1 {
				uy = h - 1
			}
			rendering.StrokeLine(pc, textCX-lw/2, uy, textCX+lw/2, uy, 1,
				s.Text.R, s.Text.G, s.Text.B, s.Text.A)
		}
	}
	if layered {
		pc.RestoreLayer()
	}
	// Custom wave effects (wave.tsx Inset/Shake): snapshot inputs only.
	// Shake offsets a same-fill silhouette behind the chrome; Inset paints
	// the expanding white dot at the recorded press point.
	if s.WaveShake != 0 {
		rendering.FillRoundRect(pc, s.WaveShake, 0, w, h, radius,
			s.Fill.R, s.Fill.G, s.Fill.B, s.Fill.A)
	}
	if s.WaveInset {
		d := s.WaveInsetSize
		if d > 0 {
			rendering.FillCircle(pc, s.WaveInsetX, s.WaveInsetY, d/2, 1, 1, 1, s.WaveAlpha)
		}
	}
	if s.WaveOK {
		rendering.StrokeRoundRect(pc, -s.WaveSpread, -s.WaveSpread, w+2*s.WaveSpread, h+2*s.WaveSpread, radius+s.WaveSpread, 2, s.WaveColor.R, s.WaveColor.G, s.WaveColor.B, s.WaveAlpha)
	}
	if s.FocusRing {
		rx, ry, rw, rh := focus.FocusRingRect(0, 0, w, h, focusRingOutset)
		rendering.StrokeRoundRect(pc, rx, ry, rw, rh, radius+focusRingOutset, s.RingWidth, s.Ring.R, s.Ring.G, s.Ring.B, s.Ring.A)
	}
}

// PaintButtonSpin paints the spinner from a frozen snapshot.
func PaintButtonSpin(pc *rendering.PaintContext, size rendering.Size, s ButtonSnap) {
	if pc == nil || size.Width <= 0 || size.Height <= 0 {
		return
	}
	if !s.HasSpinner {
		return
	}
	icon.PaintGlyph(pc, s.SpinnerName, 0, 0, size.Width, s.SpinnerInk, s.SpinnerAngle)
}

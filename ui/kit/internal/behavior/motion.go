package behavior

import (
	"strconv"
	"strings"

	"github.com/energye/gpui/ui/animation"
	"github.com/energye/gpui/ui/kit/internal/scope"
	"github.com/energye/gpui/ui/scheduler"
	"github.com/energye/gpui/ui/theme"
)

// Motion durations resolve from theme tokens (Fast 0.1s, Mid 0.2s,
// Slow 0.3s). Spinner and Wave wrap ui/animation only; they never
// draw and never hardcode timing.

// ParseDuration parses CSS-ish "0.1s"/"200ms" into seconds.
// Unknown or empty input falls back to 0.2s (Mid).
func ParseDuration(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0.2
	}
	if strings.HasSuffix(s, "ms") {
		v, err := strconv.ParseFloat(strings.TrimSuffix(s, "ms"), 64)
		if err != nil || v < 0 {
			return 0.2
		}
		return v / 1000
	}
	if strings.HasSuffix(s, "s") {
		v, err := strconv.ParseFloat(strings.TrimSuffix(s, "s"), 64)
		if err != nil || v < 0 {
			return 0.2
		}
		return v
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 {
		return 0.2
	}
	return v
}

// MotionDurations carries resolved fast/mid/slow seconds.
type MotionDurations struct {
	Fast float64
	Mid  float64
	Slow float64
}

// ResolveDurations reads the three duration tokens.
func ResolveDurations(tok theme.Tokens) MotionDurations {
	return MotionDurations{
		Fast: ParseDuration(tok.MotionDurationFast),
		Mid:  ParseDuration(tok.MotionDurationMid),
		Slow: ParseDuration(tok.MotionDurationSlow),
	}
}

// ResolveCurve maps a token-style name to an engine curve.
// Unknown names fall back to EaseInOut (pleasant default).
func ResolveCurve(name string) animation.Curve {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "linear":
		return animation.CurveLinear
	case "ease-in":
		return animation.CurveEaseIn
	case "ease-out":
		return animation.CurveEaseOut
	case "ease", "ease-in-out", "":
		return animation.CurveEaseInOut
	default:
		return animation.CurveEaseInOut
	}
}

// Spinner drives a true rotation angle via a repeating Controller.
// Angle = 360 * curved progress. Reduced-motion or a disabled master
// switch freezes the spinner (Stop + no restart).
type Spinner struct {
	ctrl    *animation.Controller
	enabled bool
}

// NewSpinner builds a looping spinner over durSec seconds.
func NewSpinner(durSec float64, curve animation.Curve) *Spinner {
	if durSec <= 0 {
		durSec = 1
	}
	if curve == nil {
		curve = animation.CurveLinear
	}
	c := animation.NewController(durSec)
	c.SetCurve(curve)
	c.SetRepeat(true)
	return &Spinner{ctrl: c, enabled: true}
}

// Controller exposes the engine ticker for window wiring.
func (s *Spinner) Controller() *animation.Controller {
	if s == nil {
		return nil
	}
	return s.ctrl
}

// Start begins ticking when motion is allowed.
func (s *Spinner) Start(reg *scheduler.TickerRegistry, motion scope.MotionConfig) {
	if s == nil || s.ctrl == nil {
		return
	}
	if !motion.MotionEnabled() {
		s.enabled = false
		return
	}
	s.enabled = true
	s.ctrl.Start(reg)
}

// Stop freezes the spinner.
func (s *Spinner) Stop() {
	if s == nil || s.ctrl == nil {
		return
	}
	s.ctrl.Stop()
}

// IsSpinning reports active ticking.
func (s *Spinner) IsSpinning() bool {
	return s != nil && s.ctrl != nil && s.ctrl.IsRunning()
}

// Angle returns the current rotation degrees in [0,360).
func (s *Spinner) Angle() float64 {
	if s == nil || s.ctrl == nil {
		return 0
	}
	return s.ctrl.Value() * 360
}

// WaveConfig describes one ripple: enabled switch, border presence,
// text/link suppression, and disabled/loading suppression.
type WaveConfig struct {
	Enabled    bool
	HasBorder  bool
	IsTextLink bool
	Disabled   bool
	Loading    bool
}

// CanWave reports whether a ripple may start. Press-and-hold must not
// ripple (only post-tap starts call this); disabled, loading, text/link
// and reduced-motion never ripple.
func CanWave(motion scope.MotionConfig, cfg WaveConfig) bool {
	if !motion.MotionEnabled() {
		return false
	}
	if !cfg.Enabled || cfg.Disabled || cfg.Loading || cfg.IsTextLink {
		return false
	}
	return true
}

// WaveColor picks the ripple color: border color when bordered,
// otherwise the background color (follows WIDGET_MODEL §4).
func WaveColor(border, bg theme.Color, hasBorder bool) theme.Color {
	if hasBorder {
		return border
	}
	return bg
}

// Wave is a one-shot ripple: 0→6 radius spread over 0.4s, full fade
// over 2s, initial depth 20%. Driven by wall-clock ticks.
type Wave struct {
	elapsed float64
	running bool
}

// NewWave builds an idle ripple.
func NewWave() *Wave { return &Wave{} }

// Start begins the ripple when allowed; returns false when suppressed.
func (w *Wave) Start(motion scope.MotionConfig, cfg WaveConfig) bool {
	if w == nil {
		return false
	}
	if !CanWave(motion, cfg) {
		return false
	}
	w.elapsed = 0
	w.running = true
	return true
}

// Tick advances the ripple; returns false when finished.
func (w *Wave) Tick(dt float64) bool {
	if w == nil || !w.running {
		return false
	}
	if dt < 0 {
		dt = 0
	}
	w.elapsed += dt
	if w.elapsed >= 2 {
		w.running = false
		return false
	}
	return true
}

// IsRunning reports an active ripple.
func (w *Wave) IsRunning() bool { return w != nil && w.running }

// Radius returns the spread radius in [0,6].
func (w *Wave) Radius() float64 {
	if w == nil {
		return 0
	}
	t := w.elapsed / 0.4
	if t > 1 {
		t = 1
	}
	// EaseOut quadratic matches engine EaseOut.
	return 6 * (1 - (1-t)*(1-t))
}

// Alpha returns the ripple alpha in [0,0.2].
func (w *Wave) Alpha() float64 {
	if w == nil || !w.running {
		return 0
	}
	a := 0.2 * (1 - w.elapsed/2)
	if a < 0 {
		return 0
	}
	return a
}

// Elapsed reports seconds since start (tests).
func (w *Wave) Elapsed() float64 {
	if w == nil {
		return 0
	}
	return w.elapsed
}

// FadeAt returns implicit fade opacity for linear t in [0,1].
func FadeAt(t float64) float64 {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return t * t * (3 - 2*t)
}

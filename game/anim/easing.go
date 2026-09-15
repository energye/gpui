package anim

import (
	"math"

	"github.com/energye/gpui/game/core"
)

// Kind names one frozen easing curve. Values are frozen; new curves only
// append after InOutElastic and never renumber the old ones.
type Kind int

const (
	Linear Kind = iota
	InSine
	OutSine
	InOutSine
	InQuad
	OutQuad
	InOutQuad
	InCubic
	OutCubic
	InOutCubic
	InQuart
	OutQuart
	InOutQuart
	InQuint
	OutQuint
	InOutQuint
	InExpo
	OutExpo
	InOutExpo
	InCirc
	OutCirc
	InOutCirc
	InBack
	OutBack
	InOutBack
	InBounce
	OutBounce
	InOutBounce
	InElastic
	OutElastic
	InOutElastic
)

// kindNames freezes the asset-visible name of each Kind in declaration
// order. Parse and Name share this table so the file and the code agree.
var kindNames = []string{
	"linear",
	"in_sine", "out_sine", "in_out_sine",
	"in_quad", "out_quad", "in_out_quad",
	"in_cubic", "out_cubic", "in_out_cubic",
	"in_quart", "out_quart", "in_out_quart",
	"in_quint", "out_quint", "in_out_quint",
	"in_expo", "out_expo", "in_out_expo",
	"in_circ", "out_circ", "in_out_circ",
	"in_back", "out_back", "in_out_back",
	"in_bounce", "out_bounce", "in_out_bounce",
	"in_elastic", "out_elastic", "in_out_elastic",
}

// Valid reports whether k names a frozen curve.
func Valid(k Kind) bool { return k >= Linear && int(k) < len(kindNames) }

// Name returns the frozen asset name of k, or "unknown" for bad kinds.
func Name(k Kind) string {
	if !Valid(k) {
		return "unknown"
	}
	return kindNames[int(k)]
}

// Parse looks k up by frozen name. Empty and unknown names return a core
// InvalidArg error and never guess a curve.
func Parse(name string) (Kind, error) {
	for i, n := range kindNames {
		if n == name {
			return Kind(i), nil
		}
	}
	return Linear, core.InvalidArg("anim.Parse", name)
}

// AllKinds returns every frozen kind in declaration order.
func AllKinds() []Kind {
	out := make([]Kind, len(kindNames))
	for i := range kindNames {
		out[i] = Kind(i)
	}
	return out
}

// Back and elastic constants (Godot/Penner analytic form, not Flutter's
// cubic-bezier subset): back overshoot uses 1.70158, the in-out variant
// widens it by 1.525; elastic out uses period 2pi/3 while in-out uses
// 2pi/4.5, so their phases differ by design.
const (
	backC1 = 1.70158
	backC3 = backC1 + 1
	backC2 = backC1 * 1.525

	elasticC4      = 2 * math.Pi / 3
	elasticInOutC4 = 2 * math.Pi / 4.5
)

// clampT pins t to [0,1]. NaN maps to 0 so a bad clock parks at the start
// instead of poisoning the pose with NaN; infinities clamp to the ends.
// Intentional difference from camera/project: those reject bad input with
// ok=false, while easing stays a total function with no hot-path branch
// so timeline sampling (4.2) never checks ok per key.
func clampT(t float64) float64 {
	if math.IsNaN(t) {
		return 0
	}
	if t < 0 {
		return 0
	}
	if t > 1 {
		return 1
	}
	return t
}

// Ease maps linear progress t to shaped progress. The input is clamped
// first, clamped ends return exactly 0/1, unknown kinds fall back to
// linear, and the result is always finite (never NaN/Inf, never a panic).
// Back and elastic may leave [0,1] mid-flight by design; the rest stay in
// [0,1].
func Ease(k Kind, t float64) float64 {
	t = clampT(t)
	if t == 0 || t == 1 {
		return t
	}
	switch k {
	case Linear:
		return t
	case InSine:
		return 1 - math.Cos(t*math.Pi/2)
	case OutSine:
		return math.Sin(t * math.Pi / 2)
	case InOutSine:
		return -(math.Cos(math.Pi*t) - 1) / 2
	case InQuad:
		return t * t
	case OutQuad:
		return 1 - (1-t)*(1-t)
	case InOutQuad:
		if t < 0.5 {
			return 2 * t * t
		}
		u := -2*t + 2
		return 1 - u*u/2
	case InCubic:
		return t * t * t
	case OutCubic:
		u := 1 - t
		return 1 - u*u*u
	case InOutCubic:
		if t < 0.5 {
			return 4 * t * t * t
		}
		u := -2*t + 2
		return 1 - u*u*u/2
	case InQuart:
		return t * t * t * t
	case OutQuart:
		u := 1 - t
		uu := u * u
		return 1 - uu*uu
	case InOutQuart:
		if t < 0.5 {
			return 8 * t * t * t * t
		}
		u := -2*t + 2
		uu := u * u
		return 1 - uu*uu/2
	case InQuint:
		return t * t * t * t * t
	case OutQuint:
		u := 1 - t
		uu := u * u
		return 1 - uu*uu*u
	case InOutQuint:
		if t < 0.5 {
			return 16 * t * t * t * t * t
		}
		u := -2*t + 2
		uu := u * u
		return 1 - uu*uu*u/2
	case InExpo:
		return math.Pow(2, 10*t-10)
	case OutExpo:
		return 1 - math.Pow(2, -10*t)
	case InOutExpo:
		if t < 0.5 {
			return math.Pow(2, 20*t-10) / 2
		}
		return (2 - math.Pow(2, -20*t+10)) / 2
	case InCirc:
		return 1 - sqrtNonNeg(1-t*t)
	case OutCirc:
		u := t - 1
		return sqrtNonNeg(1 - u*u)
	case InOutCirc:
		if t < 0.5 {
			return (1 - sqrtNonNeg(1-4*t*t)) / 2
		}
		u := -2*t + 2
		return (sqrtNonNeg(1-u*u) + 1) / 2
	case InBack:
		return backC3*t*t*t - backC1*t*t
	case OutBack:
		u := t - 1
		return 1 + backC3*u*u*u + backC1*u*u
	case InOutBack:
		if t < 0.5 {
			u := 2 * t
			return (u * u * ((backC2+1)*u - backC2)) / 2
		}
		u := 2*t - 2
		return (u*u*((backC2+1)*u+backC2) + 2) / 2
	case InBounce:
		return 1 - bounceOut(1-t)
	case OutBounce:
		return bounceOut(t)
	case InOutBounce:
		if t < 0.5 {
			return (1 - bounceOut(1-2*t)) / 2
		}
		return (1 + bounceOut(2*t-1)) / 2
	case InElastic:
		return -math.Pow(2, 10*t-10) * math.Sin((t*10-10.75)*elasticC4)
	case OutElastic:
		return math.Pow(2, -10*t)*math.Sin((t*10-0.75)*elasticC4) + 1
	case InOutElastic:
		if t < 0.5 {
			return -(math.Pow(2, 20*t-10) * math.Sin((20*t-11.125)*elasticInOutC4)) / 2
		}
		return (math.Pow(2, -20*t+10)*math.Sin((20*t-11.125)*elasticInOutC4))/2 + 1
	default:
		return t
	}
}

// Lerp blends a to b with the eased progress: a+(b-a)*Ease(kind, t).
// The t handling matches Ease (clamped, NaN parks at a); a and b pass
// through untouched so callers see their own numbers back at the ends.
// It never panics; non-finite a/b follow IEEE-754 without a guard.
func Lerp(a, b, t float64, kind Kind) float64 {
	return a + (b-a)*Ease(kind, t)
}

// sqrtNonNeg roots max(v,0): the circ families feed 1-u*u where float
// rounding can leave a tiny negative (e.g. 1-1 = -1e-17) that would
// otherwise poison the pose with NaN. One gate covers all four call sites.
func sqrtNonNeg(v float64) float64 {
	if v < 0 {
		return 0
	}
	return math.Sqrt(v)
}

// bounceOut is the shared out-bounce polynomial; in and in-out derive
// from it so the three always agree at the joints.
func bounceOut(t float64) float64 {
	const n1 = 7.5625
	const d1 = 2.75
	switch {
	case t < 1/d1:
		return n1 * t * t
	case t < 2/d1:
		t -= 1.5 / d1
		return n1*t*t + 0.75
	case t < 2.5/d1:
		t -= 2.25 / d1
		return n1*t*t + 0.9375
	default:
		t -= 2.625 / d1
		return n1*t*t + 0.984375
	}
}

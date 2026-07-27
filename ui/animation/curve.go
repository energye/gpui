package animation

import "math"

// Curve maps linear progress t∈[0,1] to a shaped progress y∈[0,1].
// Implementations should clamp or be well-defined outside [0,1].
type Curve interface {
	Transform(t float64) float64
}

// CurveFunc adapts a function to Curve.
type CurveFunc func(t float64) float64

// Transform implements Curve.
func (f CurveFunc) Transform(t float64) float64 {
	if f == nil {
		return clamp01(t)
	}
	return f(t)
}

// Linear is y = t.
func Linear(t float64) float64 { return clamp01(t) }

// EaseInOut is a smoothstep cubic (3t²−2t³), flat at 0 and 1.
func EaseInOut(t float64) float64 {
	t = clamp01(t)
	return t * t * (3 - 2*t)
}

// EaseIn is quadratic ease-in.
func EaseIn(t float64) float64 {
	t = clamp01(t)
	return t * t
}

// EaseOut is quadratic ease-out.
func EaseOut(t float64) float64 {
	t = clamp01(t)
	return 1 - (1-t)*(1-t)
}

// Curves for SetCurve without allocating when using package funcs as CurveFunc.
var (
	CurveLinear    Curve = CurveFunc(Linear)
	CurveEaseInOut Curve = CurveFunc(EaseInOut)
	CurveEaseIn    Curve = CurveFunc(EaseIn)
	CurveEaseOut   Curve = CurveFunc(EaseOut)
)

func clamp01(t float64) float64 {
	if t < 0 {
		return 0
	}
	if t > 1 {
		return 1
	}
	if math.IsNaN(t) {
		return 0
	}
	return t
}

// applyCurve returns curved value; nil curve → linear.
func applyCurve(c Curve, t float64) float64 {
	t = clamp01(t)
	if c == nil {
		return t
	}
	return clamp01(c.Transform(t))
}

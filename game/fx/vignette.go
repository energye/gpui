package fx

import (
	"math"

	"github.com/energye/gpui/game/core"
)

// MaxVignetteRadius caps Inner/Outer in UV units (2 covers any corner from
// any center inside the frame).
const MaxVignetteRadius = 2

// Vignette darkens frame corners: pixels inside Inner keep full brightness,
// pixels outside Outer are scaled by (1-Strength), and the ring between is
// a smooth Hermite ramp. Center is in normalized UV (0..1, 0.5,0.5 is the
// frame middle); distances are plain UV Euclidean, so wide frames fall off
// faster horizontally by design (a future aspect-correct mode adds, never
// changes, this math).
type Vignette struct {
	Center   core.Vec2
	Inner    float64
	Outer    float64
	Strength float64
}

// NewVignette builds a Vignette. Center must be finite, Inner in
// [0,MaxVignetteRadius], Outer in (Inner,MaxVignetteRadius], Strength in
// [0,1]. Anything else is InvalidArg and builds nothing.
func NewVignette(center core.Vec2, inner, outer, strength float64) (Vignette, error) {
	const op = "fx.NewVignette"
	v := Vignette{Center: center, Inner: inner, Outer: outer, Strength: strength}
	if err := v.Validate(); err != nil {
		return Vignette{}, err
	}
	_ = op
	return v, nil
}

// Validate reports InvalidArg for non-finite or out-of-range fields.
func (v Vignette) Validate() error {
	const op = "fx.Vignette.Validate"
	if !finiteFloat(v.Center.X) || !finiteFloat(v.Center.Y) {
		return core.InvalidArg(op, "center")
	}
	if !finiteFloat(v.Inner) || v.Inner < 0 || v.Inner > MaxVignetteRadius {
		return core.InvalidArg(op, "inner")
	}
	if !finiteFloat(v.Outer) || v.Outer <= v.Inner || v.Outer > MaxVignetteRadius {
		return core.InvalidArg(op, "outer")
	}
	if !finiteFloat(v.Strength) || v.Strength < 0 || v.Strength > 1 {
		return core.InvalidArg(op, "strength")
	}
	return nil
}

// FactorAt returns the brightness multiplier at normalized uv (for probes
// and tests): 1 inside Inner, (1-Strength) outside Outer, smooth between.
func (v Vignette) FactorAt(uv core.Vec2) float64 {
	dx := uv.X - v.Center.X
	dy := uv.Y - v.Center.Y
	d := math.Sqrt(dx*dx + dy*dy)
	t := smoothstep(v.Inner, v.Outer, d)
	return 1 - v.Strength*t
}

func smoothstep(edge0, edge1, x float64) float64 {
	if x <= edge0 {
		return 0
	}
	if x >= edge1 {
		return 1
	}
	t := (x - edge0) / (edge1 - edge0)
	return t * t * (3 - 2*t)
}

// Apply returns src darkened by the vignette factor, as a fresh image.
// RGB is scaled, alpha preserved exactly, src never mutated. A nil src is
// InvalidArg, a corrupt src is BadData, an invalid Vignette is InvalidArg.
func (v Vignette) Apply(src *Image) (*Image, error) {
	const op = "fx.Vignette.Apply"
	if err := src.check(op); err != nil {
		return nil, err
	}
	if err := v.Validate(); err != nil {
		return nil, err
	}
	out := make([]core.Color, len(src.Pix))
	for y := 0; y < src.H; y++ {
		uvy := (float64(y) + 0.5) / float64(src.H)
		for x := 0; x < src.W; x++ {
			uvx := (float64(x) + 0.5) / float64(src.W)
			f := v.FactorAt(core.V2(uvx, uvy))
			c := src.Pix[y*src.W+x]
			out[y*src.W+x] = core.RGBA(
				clamp01(c.R*f), clamp01(c.G*f), clamp01(c.B*f), c.A)
		}
	}
	return &Image{W: src.W, H: src.H, Pix: out}, nil
}

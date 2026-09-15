package fx

import (
	"math"

	"github.com/energye/gpui/game/core"
)

// Frozen warp budgets. A picture beyond budget is well-formed but too
// big: OutOfMemory, never a guess. Same cap as the tex intake so a
// warped frame always fits where its source fit.
const (
	// MaxWarpDimension caps one side in pixels.
	MaxWarpDimension = 8192
	// MaxWarpPixels caps the area in pixels.
	MaxWarpPixels = 8192 * 8192
)

// Noise names the offset field one Warp samples: which lattice (Seed),
// how tight the wobble is (Freq, cycles per pixel), and how fast the
// field drifts with time (Drift, lattice units per second).
type Noise struct {
	Seed  uint64
	Freq  float64
	Drift core.Vec2
}

// DefaultNoise is the frozen water-wobble field: seed 20260915, one
// wobble every 32 pixels, slow drift down-right. Tests pin it.
func DefaultNoise() Noise {
	return Noise{Seed: 20260915, Freq: 1.0 / 32.0, Drift: core.V2(0.5, 0.25)}
}

// Warp is a heat-haze / water-wobble sampler: picture pixels are read
// at p + OffsetAt(p, t) instead of p, with the offset scaled by
// Strength (pixels at noise output 1). The zero value is valid and is
// the exact identity (strength 0 reads every pixel in place).
type Warp struct {
	Strength float64
	Noise    Noise
}

func finiteFloat(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

func validNoise(n Noise) bool {
	return n.Freq > 0 && finiteFloat(n.Freq) &&
		finiteFloat(n.Drift.X) && finiteFloat(n.Drift.Y)
}

// NewWarp builds a Warp. Strength must be finite and non-negative,
// Freq finite and positive, Drift finite. A bad argument is InvalidArg
// and builds nothing.
func NewWarp(strength float64, n Noise) (Warp, error) {
	const op = "fx.NewWarp"
	if !finiteFloat(strength) || strength < 0 {
		return Warp{}, core.InvalidArg(op, "strength")
	}
	if !validNoise(n) {
		return Warp{}, core.InvalidArg(op, "noise")
	}
	return Warp{Strength: strength, Noise: n}, nil
}

// SetStrength swaps the wobble amplitude. Bad input is InvalidArg and
// leaves w unchanged.
func (w *Warp) SetStrength(strength float64) error {
	const op = "fx.Warp.SetStrength"
	if w == nil {
		return core.InvalidArg(op, "warp")
	}
	if !finiteFloat(strength) || strength < 0 {
		return core.InvalidArg(op, "strength")
	}
	w.Strength = strength
	return nil
}

// SetNoise swaps the sampled field. Bad input is InvalidArg and leaves
// w unchanged.
func (w *Warp) SetNoise(n Noise) error {
	const op = "fx.Warp.SetNoise"
	if w == nil {
		return core.InvalidArg(op, "warp")
	}
	if !validNoise(n) {
		return core.InvalidArg(op, "noise")
	}
	w.Noise = n
	return nil
}

// hash01 maps one lattice corner to [0, 1): splitmix64 over the mixed
// corner plus seed plus channel salt. Same inputs give the same output
// on every machine, so replays are bitwise identical.
func hash01(ix, iy int64, seed uint64, channel uint64) float64 {
	h := uint64(ix)*0x9E3779B97F4A7C15 ^ uint64(iy)*0xBF58476D1CE4E5B9 ^
		seed ^ channel*0x94D049BB133111EB
	h += 0x9E3779B97F4A7C15
	z := h
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	z ^= z >> 31
	return float64(z>>11) / (1 << 53)
}

// channel salts: X and Y read different lattices so the two axes never
// mirror each other.
const (
	chanX = 0x1234ABCD
	chanY = 0xABCD1234
)

// noise1 samples one channel of the value-noise field at lattice point
// (x, y): corner hashes blended with a smoothstep, mapped to [-1, 1].
// Pure function of its inputs; both backends share this formula.
func noise1(x, y float64, seed uint64, channel uint64) float64 {
	ix := math.Floor(x)
	iy := math.Floor(y)
	fx := x - ix
	fy := y - iy
	ux := fx * fx * (3 - 2*fx)
	uy := fy * fy * (3 - 2*fy)
	a := hash01(int64(ix), int64(iy), seed, channel)
	b := hash01(int64(ix)+1, int64(iy), seed, channel)
	c := hash01(int64(ix), int64(iy)+1, seed, channel)
	d := hash01(int64(ix)+1, int64(iy)+1, seed, channel)
	return (a+(b-a)*ux+(c-a)*uy+(a-b-c+d)*ux*uy)*2 - 1
}

// OffsetAt returns the sample shift at picture point p and time t
// (seconds): strength * noise(p*freq + drift*t), one channel per axis.
// Total: strength 0 is the exact identity, and any non-finite p, t, or
// warp field yields the zero offset, never a panic and never a NaN
// leaking downstream.
func (w Warp) OffsetAt(p core.Vec2, t float64) core.Vec2 {
	if w.Strength == 0 {
		return core.Vec2{}
	}
	if !finiteFloat(w.Strength) || !validNoise(w.Noise) {
		return core.Vec2{}
	}
	if !finiteFloat(p.X) || !finiteFloat(p.Y) || !finiteFloat(t) {
		return core.Vec2{}
	}
	sx := p.X*w.Noise.Freq + w.Noise.Drift.X*t
	sy := p.Y*w.Noise.Freq + w.Noise.Drift.Y*t
	nx := noise1(sx, sy, w.Noise.Seed, chanX)
	ny := noise1(sy+17.5, sx+9.25, w.Noise.Seed, chanY)
	return core.Vec2{X: w.Strength * nx, Y: w.Strength * ny}
}

// WarpPoint returns where picture point p is sampled from at time t:
// p + OffsetAt(p, t). Same totality as OffsetAt.
func (w Warp) WarpPoint(p core.Vec2, t float64) core.Vec2 {
	o := w.OffsetAt(p, t)
	return core.Vec2{X: p.X + o.X, Y: p.Y + o.Y}
}

// checkImage validates a w-by-h RGBA8 slice: positive sides in budget,
// length exactly w*h*4.
func checkImage(op string, px []uint8, w, h int) error {
	if w <= 0 || h <= 0 {
		return core.InvalidArg(op, "size")
	}
	if w > MaxWarpDimension || h > MaxWarpDimension ||
		uint64(w)*uint64(h) > MaxWarpPixels {
		return core.OutOfMemory(op, "size")
	}
	if len(px) != w*h*4 {
		return core.BadData(op, "pixels")
	}
	return nil
}

// SampleClamped reads one RGBA8 pixel with clamped edges: out-of-range
// coordinates pin to the nearest edge texel, exactly like the GPU
// clamp-to-edge sampler, so both sides agree at the border. Bad
// arguments are InvalidArg and read nothing.
func SampleClamped(px []uint8, w, h, x, y int) ([4]uint8, error) {
	const op = "fx.SampleClamped"
	if err := checkImage(op, px, w, h); err != nil {
		return [4]uint8{}, err
	}
	if x < 0 {
		x = 0
	}
	if x >= w {
		x = w - 1
	}
	if y < 0 {
		y = 0
	}
	if y >= h {
		y = h - 1
	}
	i := (y*w + x) * 4
	return [4]uint8{px[i], px[i+1], px[i+2], px[i+3]}, nil
}

// WarpRGBA applies w to src (w-by-h RGBA8) at time t and returns a fresh
// intermediate copy: dst[i] samples src at WarpPoint rounded to the
// nearest texel with clamped edges. src is never written; the caller
// keeps the live frame and swaps in the returned copy. Strength 0
// returns an exact copy. A length mismatch is BadData, an over-budget
// picture OutOfMemory, a zero or negative side InvalidArg.
func WarpRGBA(src []uint8, w, h int, t float64, wv Warp) ([]uint8, error) {
	const op = "fx.WarpRGBA"
	if err := checkImage(op, src, w, h); err != nil {
		return nil, err
	}
	dst := make([]uint8, len(src))
	if wv.Strength == 0 || !finiteFloat(t) {
		copy(dst, src)
		return dst, nil
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			o := wv.OffsetAt(core.V2(float64(x), float64(y)), t)
			sx := int(math.Round(float64(x) + o.X))
			sy := int(math.Round(float64(y) + o.Y))
			if sx < 0 {
				sx = 0
			}
			if sx >= w {
				sx = w - 1
			}
			if sy < 0 {
				sy = 0
			}
			if sy >= h {
				sy = h - 1
			}
			si := (sy*w + sx) * 4
			di := (y*w + x) * 4
			dst[di] = src[si]
			dst[di+1] = src[si+1]
			dst[di+2] = src[si+2]
			dst[di+3] = src[si+3]
		}
	}
	return dst, nil
}

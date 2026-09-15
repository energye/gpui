package fx

import (
	"github.com/energye/gpui/game/core"
)

// MaxImagePixels caps one grading intermediate (16M pixels, 64MB at RGBA8).
// Bigger pictures report core OutOfMemory instead of allocating.
const MaxImagePixels = 16 << 20

// MaxBloomRadius caps the box-blur radius (16px each pass is plenty for a
// glow; bigger radii belong to a GPU blur, not this CPU grade path).
const MaxBloomRadius = 16

// MaxBloomIntensity caps the additive glow strength.
const MaxBloomIntensity = 4

// Image is one isolated grading intermediate: W*H straight-alpha
// core.Colors in row-major order. It never aliases another Image:
// constructors copy, Apply returns fresh images, Clone copies again.
type Image struct {
	W, H int
	Pix  []core.Color
}

// NewImage builds a W-by-H transparent-black image. Non-positive sizes are
// InvalidArg; W*H beyond MaxImagePixels is OutOfMemory.
func NewImage(w, h int) (*Image, error) {
	const op = "fx.NewImage"
	if w <= 0 || h <= 0 {
		return nil, core.InvalidArg(op, "size")
	}
	if int64(w)*int64(h) > int64(MaxImagePixels) {
		return nil, core.OutOfMemory(op, "pixels")
	}
	return &Image{W: w, H: h, Pix: make([]core.Color, w*h)}, nil
}

// NewImageFromColors builds an image copying pix (row-major, len W*H).
// Bad sizes are InvalidArg, a length mismatch is BadData, over-budget is
// OutOfMemory. A NaN/Inf component is BadData, never a silent pixel.
func NewImageFromColors(w, h int, pix []core.Color) (*Image, error) {
	const op = "fx.NewImageFromColors"
	if w <= 0 || h <= 0 {
		return nil, core.InvalidArg(op, "size")
	}
	if int64(w)*int64(h) > int64(MaxImagePixels) {
		return nil, core.OutOfMemory(op, "pixels")
	}
	if len(pix) != w*h {
		return nil, core.BadData(op, "length")
	}
	for i, c := range pix {
		if !finiteColor(c) {
			return nil, core.BadData(op, "pixel")
		}
		_ = i
	}
	cp := make([]core.Color, len(pix))
	copy(cp, pix)
	return &Image{W: w, H: h, Pix: cp}, nil
}

// valid reports whether img is drawable (non-nil, positive, exact pixels).
func (img *Image) valid() bool {
	return img != nil && img.W > 0 && img.H > 0 && len(img.Pix) == img.W*img.H
}

// check validates img for Apply: nil is InvalidArg, corrupt is BadData.
func (img *Image) check(op string) error {
	if img == nil {
		return core.InvalidArg(op, "image")
	}
	if !img.valid() {
		return core.BadData(op, "image")
	}
	return nil
}

// At returns the pixel at (x, y), or false when out of range.
func (img *Image) At(x, y int) (core.Color, bool) {
	if !img.valid() || x < 0 || y < 0 || x >= img.W || y >= img.H {
		return core.Color{}, false
	}
	return img.Pix[y*img.W+x], true
}

// Set stores c at (x, y). False on bad coordinates or a corrupt image;
// a non-finite c is rejected (false) and stores nothing.
func (img *Image) Set(x, y int, c core.Color) bool {
	if !img.valid() || x < 0 || y < 0 || x >= img.W || y >= img.H {
		return false
	}
	if !finiteColor(c) {
		return false
	}
	img.Pix[y*img.W+x] = c
	return true
}

// Clone returns a fresh copy, or nil for a nil/corrupt image (never panics).
func (img *Image) Clone() *Image {
	if !img.valid() {
		return nil
	}
	cp := make([]core.Color, len(img.Pix))
	copy(cp, img.Pix)
	return &Image{W: img.W, H: img.H, Pix: cp}
}

// ApproxEqual reports whether o has the same size and every component
// differs by less than eps. Non-positive eps means exact equality.
func (img *Image) ApproxEqual(o *Image, eps float64) bool {
	if !img.valid() || !o.valid() || img.W != o.W || img.H != o.H {
		return false
	}
	if eps <= 0 {
		return equalExact(img.Pix, o.Pix)
	}
	for i := range img.Pix {
		if !img.Pix[i].ApproxEqual(o.Pix[i], eps) {
			return false
		}
	}
	return true
}

func equalExact(a, b []core.Color) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// finiteFloat lives in warp.go (S39, same package): shared NaN/Inf guard.
func finiteColor(c core.Color) bool {
	return finiteFloat(c.R) && finiteFloat(c.G) && finiteFloat(c.B) && finiteFloat(c.A)
}

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

// luminance is the Rec.709 luma of the straight-alpha RGB (alpha ignored,
// alpha is always preserved by every stage).
func luminance(c core.Color) float64 {
	return 0.2126*c.R + 0.7152*c.G + 0.0722*c.B
}

// Bloom glows bright areas: pixels above Threshold are masked, box-blurred
// by Radius, and added back scaled by Intensity. Radius 0 (or Intensity 0)
// is a valid near-identity: the mask still applies but (almost) nothing is
// added, so callers can freeze one Grade and fade the glow with Intensity.
type Bloom struct {
	Threshold float64
	Radius    int
	Intensity float64
}

// NewBloom builds a Bloom. Threshold must be in [0,1], Radius in
// [0,MaxBloomRadius], Intensity in [0,MaxBloomIntensity]; all floats must
// be finite. Anything else is InvalidArg and builds nothing.
func NewBloom(threshold float64, radius int, intensity float64) (Bloom, error) {
	const op = "fx.NewBloom"
	b := Bloom{Threshold: threshold, Radius: radius, Intensity: intensity}
	if err := b.Validate(); err != nil {
		return Bloom{}, err
	}
	_ = op
	return b, nil
}

// Validate reports InvalidArg for non-finite or out-of-range fields.
func (b Bloom) Validate() error {
	const op = "fx.Bloom.Validate"
	if !finiteFloat(b.Threshold) || b.Threshold < 0 || b.Threshold > 1 {
		return core.InvalidArg(op, "threshold")
	}
	if b.Radius < 0 || b.Radius > MaxBloomRadius {
		return core.InvalidArg(op, "radius")
	}
	if !finiteFloat(b.Intensity) || b.Intensity < 0 || b.Intensity > MaxBloomIntensity {
		return core.InvalidArg(op, "intensity")
	}
	return nil
}

// IsIdentity reports whether Apply adds (almost) nothing: zero intensity
// always, or zero radius with zero threshold-mask output handled by Apply.
func (b Bloom) IsIdentity() bool { return b.Intensity == 0 }

// Apply returns src plus the blurred bright-pass, as a fresh image.
// src is never mutated. A nil src is InvalidArg, a corrupt src is BadData,
// an invalid Bloom is InvalidArg; all leave nothing allocated to the caller.
func (b Bloom) Apply(src *Image) (*Image, error) {
	const op = "fx.Bloom.Apply"
	if err := src.check(op); err != nil {
		return nil, err
	}
	if err := b.Validate(); err != nil {
		return nil, err
	}
	n := len(src.Pix)
	masked := make([]core.Color, n)
	for i, c := range src.Pix {
		r, g, bl := clamp01(c.R), clamp01(c.G), clamp01(c.B)
		m := brightMask(luminance(core.RGB(r, g, bl)), b.Threshold)
		masked[i] = core.RGBA(r*m, g*m, bl*m, 0)
	}
	blurred := boxBlur(masked, src.W, src.H, b.Radius)
	out := make([]core.Color, n)
	for i, c := range src.Pix {
		r := clamp01(c.R + blurred[i].R*b.Intensity)
		g := clamp01(c.G + blurred[i].G*b.Intensity)
		bl := clamp01(c.B + blurred[i].B*b.Intensity)
		out[i] = core.RGBA(r, g, bl, c.A)
	}
	return &Image{W: src.W, H: src.H, Pix: out}, nil
}

// brightMask maps luminance to [0,1]: 0 at or below threshold, ramping to
// 1 at full white. Threshold 1 yields 0 everywhere (nothing ever glows).
func brightMask(lum, threshold float64) float64 {
	if lum <= threshold {
		return 0
	}
	denom := 1 - threshold
	if denom <= 0 {
		return 0
	}
	return clamp01((lum - threshold) / denom)
}

// boxBlur blurs RGB (alpha ignored) with a separable clamped-edge box of
// half-width r. r<=0 returns a copy. Pure function: input never mutated.
func boxBlur(src []core.Color, w, h, r int) []core.Color {
	n := len(src)
	tmp := make([]core.Color, n)
	out := make([]core.Color, n)
	if r <= 0 {
		copy(out, src)
		return out
	}
	width := 2*r + 1
	inv := 1 / float64(width)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var sr, sg, sb float64
			for k := -r; k <= r; k++ {
				xx := x + k
				if xx < 0 {
					xx = 0
				}
				if xx >= w {
					xx = w - 1
				}
				c := src[y*w+xx]
				sr += c.R
				sg += c.G
				sb += c.B
			}
			tmp[y*w+x] = core.RGBA(sr*inv, sg*inv, sb*inv, 0)
		}
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var sr, sg, sb float64
			for k := -r; k <= r; k++ {
				yy := y + k
				if yy < 0 {
					yy = 0
				}
				if yy >= h {
					yy = h - 1
				}
				c := tmp[yy*w+x]
				sr += c.R
				sg += c.G
				sb += c.B
			}
			out[y*w+x] = core.RGBA(sr*inv, sg*inv, sb*inv, 0)
		}
	}
	return out
}

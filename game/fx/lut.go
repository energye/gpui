package fx

import (
	"github.com/energye/gpui/game/core"
)

// LUT sizes: 1D per-channel tables this short cannot bend color (identity
// would be the only curve); this long belongs to a 3D cube, not this file.
const (
	MinLUTSize = 2
	MaxLUTSize = 256
)

// MaxExposure caps tonemap exposure (16x is already daylight-to-night).
const MaxExposure = 16

// LUT remaps each channel through its own 1D table with linear interp.
// R/G/B each hold Size samples of input 0..1 (index 0 is black, index
// Size-1 is white). Tables are always copied on the way in and never
// shared with the caller.
type LUT struct {
	Size int
	R    []float64
	G    []float64
	B    []float64
}

// NewLUT builds a LUT copying r/g/b. All three must share one length in
// [MinLUTSize,MaxLUTSize] with every entry finite in [0,1]. Anything else
// is InvalidArg and builds nothing.
func NewLUT(r, g, b []float64) (LUT, error) {
	const op = "fx.NewLUT"
	if len(r) < MinLUTSize || len(r) > MaxLUTSize || len(g) != len(r) || len(b) != len(r) {
		return LUT{}, core.InvalidArg(op, "size")
	}
	for _, tbl := range [][]float64{r, g, b} {
		for _, v := range tbl {
			if !finiteFloat(v) || v < 0 || v > 1 {
				return LUT{}, core.InvalidArg(op, "entry")
			}
		}
	}
	rc := make([]float64, len(r))
	gc := make([]float64, len(g))
	bc := make([]float64, len(b))
	copy(rc, r)
	copy(gc, g)
	copy(bc, b)
	return LUT{Size: len(r), R: rc, G: gc, B: bc}, nil
}

// NewIdentityLUT builds the do-nothing table of size entries:
// table[i] = i/(size-1). size outside [MinLUTSize,MaxLUTSize] is InvalidArg.
func NewIdentityLUT(size int) (LUT, error) {
	const op = "fx.NewIdentityLUT"
	if size < MinLUTSize || size > MaxLUTSize {
		return LUT{}, core.InvalidArg(op, "size")
	}
	t := make([]float64, size)
	for i := range t {
		t[i] = float64(i) / float64(size-1)
	}
	r := make([]float64, size)
	g := make([]float64, size)
	b := make([]float64, size)
	copy(r, t)
	copy(g, t)
	copy(b, t)
	return LUT{Size: size, R: r, G: g, B: b}, nil
}

// Validate reports InvalidArg for a zero value or a table the constructors
// would reject (wrong lengths, non-finite or out-of-range entries).
func (l LUT) Validate() error {
	const op = "fx.LUT.Validate"
	if l.Size < MinLUTSize || l.Size > MaxLUTSize {
		return core.InvalidArg(op, "size")
	}
	if len(l.R) != l.Size || len(l.G) != l.Size || len(l.B) != l.Size {
		return core.InvalidArg(op, "size")
	}
	for _, tbl := range [][]float64{l.R, l.G, l.B} {
		for _, v := range tbl {
			if !finiteFloat(v) || v < 0 || v > 1 {
				return core.InvalidArg(op, "entry")
			}
		}
	}
	return nil
}

// sample maps x in [0,1] through tbl with linear interpolation.
func sample(tbl []float64, x float64) float64 {
	x = clamp01(x)
	pos := x * float64(len(tbl)-1)
	i0 := int(pos)
	if i0 >= len(tbl)-1 {
		return tbl[len(tbl)-1]
	}
	f := pos - float64(i0)
	return tbl[i0]*(1-f) + tbl[i0+1]*f
}

// Sample maps one channel value through the matching table (for probes).
// ch is 0/1/2 for R/G/B; anything else clamps x through R by design? No:
// unknown channels are InvalidArg-safe only via Apply; Sample returns 0
// for a bad channel and never panics.
func (l LUT) Sample(ch int, x float64) float64 {
	switch ch {
	case 0:
		return sample(l.R, x)
	case 1:
		return sample(l.G, x)
	case 2:
		return sample(l.B, x)
	default:
		return 0
	}
}

// Apply returns src remapped through the tables, as a fresh image. Alpha
// preserved exactly, src never mutated. A nil src is InvalidArg, a corrupt
// src is BadData, an invalid LUT is InvalidArg.
func (l LUT) Apply(src *Image) (*Image, error) {
	const op = "fx.LUT.Apply"
	if err := src.check(op); err != nil {
		return nil, err
	}
	if err := l.Validate(); err != nil {
		return nil, err
	}
	out := make([]core.Color, len(src.Pix))
	for i, c := range src.Pix {
		out[i] = core.RGBA(
			sample(l.R, c.R), sample(l.G, c.G), sample(l.B, c.B), c.A)
	}
	return &Image{W: src.W, H: src.H, Pix: out}, nil
}

// Tonemap names the highlight rolloff applied after the LUT.
type Tonemap int

const (
	// TonemapNone disables tonemapping (Grade skips the stage entirely).
	TonemapNone Tonemap = iota
	// TonemapReinhard is x/(1+x) per channel (Godot Reinhard mode idea).
	TonemapReinhard
	// TonemapACES is the Narkowicz filmic approximation (Godot ACES idea,
	// Skia has no built-in curve so games carry this formula).
	TonemapACES
)

// String returns the stable log name of t.
func (t Tonemap) String() string {
	switch t {
	case TonemapReinhard:
		return "reinhard"
	case TonemapACES:
		return "aces"
	default:
		return "none"
	}
}

// ParseTonemap maps "none"/"reinhard"/"aces" to a Tonemap. Anything else is
// InvalidArg and returns TonemapNone.
func ParseTonemap(s string) (Tonemap, error) {
	switch s {
	case "none":
		return TonemapNone, nil
	case "reinhard":
		return TonemapReinhard, nil
	case "aces":
		return TonemapACES, nil
	default:
		return TonemapNone, core.InvalidArg("fx.ParseTonemap", s)
	}
}

// validExposure reports whether e is a usable exposure.
func validExposure(e float64) bool { return finiteFloat(e) && e >= 0 && e <= MaxExposure }

// Map maps one channel value through t at exposure (for probes).
func (t Tonemap) Map(x, exposure float64) float64 {
	switch t {
	case TonemapReinhard:
		xe := x * exposure
		return clamp01(xe / (1 + xe))
	case TonemapACES:
		xe := x * exposure
		return clamp01(xe * (2.51*xe + 0.03) / (xe*(2.43*xe+0.59) + 0.14))
	default:
		return clamp01(x)
	}
}

// ApplyTonemap returns src tonemapped, as a fresh image. TonemapNone
// returns a copy (still isolated, never an alias). A nil src is InvalidArg,
// a corrupt src is BadData, a bad tonemap or exposure is InvalidArg.
func ApplyTonemap(src *Image, t Tonemap, exposure float64) (*Image, error) {
	const op = "fx.ApplyTonemap"
	if err := src.check(op); err != nil {
		return nil, err
	}
	if t != TonemapNone && t != TonemapReinhard && t != TonemapACES {
		return nil, core.InvalidArg(op, "tonemap")
	}
	if t != TonemapNone && !validExposure(exposure) {
		return nil, core.InvalidArg(op, "exposure")
	}
	out := make([]core.Color, len(src.Pix))
	if t == TonemapNone {
		for i, c := range src.Pix {
			out[i] = core.RGBA(clamp01(c.R), clamp01(c.G), clamp01(c.B), c.A)
		}
		return &Image{W: src.W, H: src.H, Pix: out}, nil
	}
	for i, c := range src.Pix {
		out[i] = core.RGBA(
			t.Map(c.R, exposure), t.Map(c.G, exposure), t.Map(c.B, exposure), c.A)
	}
	return &Image{W: src.W, H: src.H, Pix: out}, nil
}

// Grade chains the whole post look in filter-chain order: bloom, vignette,
// LUT, tonemap. A nil LUT skips the LUT stage; TonemapNone skips the
// tonemap stage. Every stage allocates its own fresh image, so a 4-stage
// grade holds at most two intermediates at once and never touches the
// caller's pixels.
type Grade struct {
	Bloom    Bloom
	Vignette Vignette
	LUT      *LUT
	Tonemap  Tonemap
	Exposure float64
}

// NewGrade builds a Grade validating every stage (a nil lut skips LUT).
// A bad tonemap is InvalidArg; exposure is checked only when tonemap is
// not None (None ignores it).
func NewGrade(bloom Bloom, vig Vignette, lut *LUT, tm Tonemap, exposure float64) (Grade, error) {
	const op = "fx.NewGrade"
	if err := bloom.Validate(); err != nil {
		return Grade{}, err
	}
	if err := vig.Validate(); err != nil {
		return Grade{}, err
	}
	if lut != nil {
		if err := lut.Validate(); err != nil {
			return Grade{}, err
		}
	}
	if tm != TonemapNone && tm != TonemapReinhard && tm != TonemapACES {
		return Grade{}, core.InvalidArg(op, "tonemap")
	}
	if tm != TonemapNone && !validExposure(exposure) {
		return Grade{}, core.InvalidArg(op, "exposure")
	}
	var lutCp *LUT
	if lut != nil {
		cp := *lut
		cp.R = append([]float64(nil), lut.R...)
		cp.G = append([]float64(nil), lut.G...)
		cp.B = append([]float64(nil), lut.B...)
		lutCp = &cp
	}
	return Grade{Bloom: bloom, Vignette: vig, LUT: lutCp, Tonemap: tm, Exposure: exposure}, nil
}

// Stages returns the frozen filter-chain order this grade runs.
func (g Grade) Stages() []string {
	return []string{"bloom", "vignette", "lut", "tonemap"}
}

// Apply runs bloom, vignette, LUT (unless nil), tonemap (unless None) and
// returns the final fresh image. src is never mutated. A nil src is
// InvalidArg, a corrupt src is BadData.
func (g Grade) Apply(src *Image) (*Image, error) {
	const op = "fx.Grade.Apply"
	if err := src.check(op); err != nil {
		return nil, err
	}
	cur, err := g.Bloom.Apply(src)
	if err != nil {
		return nil, err
	}
	nxt, err := g.Vignette.Apply(cur)
	if err != nil {
		return nil, err
	}
	cur = nxt
	if g.LUT != nil {
		nxt, err = g.LUT.Apply(cur)
		if err != nil {
			return nil, err
		}
		cur = nxt
	}
	nxt, err = ApplyTonemap(cur, g.Tonemap, g.Exposure)
	if err != nil {
		return nil, err
	}
	return nxt, nil
}

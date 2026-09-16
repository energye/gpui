package light

import (
	"math"

	"github.com/energye/gpui/game/core"
)

// Normal is one surface direction in light space, X right, Y down (image
// pixels), Z out of the screen. Unit length is the norm; Normalize fixes
// the rest. Bad values never survive: Normalize maps them to flat.
type Normal struct {
	X, Y, Z float64
}

// N builds a Normal from components.
func N(x, y, z float64) Normal { return Normal{X: x, Y: y, Z: z} }

// FlatNormal is the default surface: straight at the viewer. Missing or
// degenerate normal data falls back here, never to black.
func FlatNormal() Normal { return Normal{X: 0, Y: 0, Z: 1} }

func finiteNormal(n Normal) bool { return finite(n.X) && finite(n.Y) && finite(n.Z) }

// Normalize returns the unit vector. Zero, non-finite, or overflowing
// input returns FlatNormal (missing tilt reads as flat, never NaN).
func (n Normal) Normalize() Normal {
	if !finiteNormal(n) {
		return FlatNormal()
	}
	l := math.Sqrt(n.X*n.X + n.Y*n.Y + n.Z*n.Z)
	if !finite(l) || l == 0 {
		return FlatNormal()
	}
	return Normal{X: n.X / l, Y: n.Y / l, Z: n.Z / l}
}

// NdotL is the diffuse cosine between surface n and incoming light l:
// both normalized inside, negative clamped to 0, result in [0,1].
// Degenerate input reads through FlatNormal, never NaN.
func NdotL(n, l Normal) float64 {
	nn := n.Normalize()
	ll := l.Normalize()
	return clamp01(nn.X*ll.X + nn.Y*ll.Y + nn.Z*ll.Z)
}

// LightDir returns the unit vector from surface point p toward light l,
// with height world units above the plane. Point lights aim at the bulb,
// directional lights aim at infinity along Dir (Dir points toward the
// light, Z comes from height; 6.3 shadows extend along -Dir). False when
// the geometry is bad (bad light, bad p, height outside (0,MaxLightRange]).
func LightDir(l Light, p core.Vec2, height float64) (Normal, bool) {
	if !finite(height) || height <= 0 || height > MaxLightRange {
		return Normal{}, false
	}
	if !finiteVec(p) {
		return Normal{}, false
	}
	switch l.Kind {
	case KindDirectional:
		if !finiteVec(l.Dir) || l.Dir.IsZero() {
			return Normal{}, false
		}
		return N(l.Dir.X, l.Dir.Y, height).Normalize(), true
	default:
		if l.Kind != KindPoint {
			return Normal{}, false
		}
		if !finiteVec(l.Pos) || !finite(l.Range) || l.Range <= 0 || l.Range > MaxLightRange {
			return Normal{}, false
		}
		return N(l.Pos.X-p.X, l.Pos.Y-p.Y, height).Normalize(), true
	}
}

// NormalMap is one isolated bump sheet: W*H normals, row-major, aligned
// 1:1 with the lit Image pixels. Constructors copy; At only reads.
type NormalMap struct {
	W, H int
	N    []Normal
}

// NewNormalMap builds a W-by-H map copying n. Non-positive sizes are
// InvalidArg, W*H beyond MaxImagePixels is OutOfMemory, a length mismatch
// or a non-finite normal is BadData.
func NewNormalMap(w, h int, n []Normal) (NormalMap, error) {
	const op = "light.NewNormalMap"
	if w <= 0 || h <= 0 {
		return NormalMap{}, core.InvalidArg(op, "size")
	}
	if int64(w)*int64(h) > int64(MaxImagePixels) {
		return NormalMap{}, core.OutOfMemory(op, "pixels")
	}
	if len(n) != w*h {
		return NormalMap{}, core.BadData(op, "length")
	}
	for _, v := range n {
		if !finiteNormal(v) {
			return NormalMap{}, core.BadData(op, "normal")
		}
	}
	cp := make([]Normal, len(n))
	copy(cp, n)
	return NormalMap{W: w, H: h, N: cp}, nil
}

// NewNormalMapFromColors decodes a mid-gray-up RGB sheet: flat is
// (0.5,0.5,1), X=2R-1, Y=2G-1, Z=2B-1, alpha ignored. Same size and
// budget rules as NewNormalMap; a non-finite pixel is BadData.
func NewNormalMapFromColors(w, h int, pix []core.Color) (NormalMap, error) {
	const op = "light.NewNormalMapFromColors"
	if w <= 0 || h <= 0 {
		return NormalMap{}, core.InvalidArg(op, "size")
	}
	if int64(w)*int64(h) > int64(MaxImagePixels) {
		return NormalMap{}, core.OutOfMemory(op, "pixels")
	}
	if len(pix) != w*h {
		return NormalMap{}, core.BadData(op, "length")
	}
	for _, c := range pix {
		if !finiteColor(c) {
			return NormalMap{}, core.BadData(op, "pixel")
		}
	}
	n := make([]Normal, len(pix))
	for i, c := range pix {
		n[i] = N(2*c.R-1, 2*c.G-1, 2*c.B-1)
	}
	return NormalMap{W: w, H: h, N: n}, nil
}

// Validate reports InvalidArg for a zero value, BadData for a length
// mismatch or a non-finite normal, OutOfMemory for over-budget.
func (m NormalMap) Validate() error {
	const op = "light.NormalMap.Validate"
	if m.W <= 0 || m.H <= 0 {
		return core.InvalidArg(op, "size")
	}
	if int64(m.W)*int64(m.H) > int64(MaxImagePixels) {
		return core.OutOfMemory(op, "pixels")
	}
	if len(m.N) != m.W*m.H {
		return core.BadData(op, "length")
	}
	for _, v := range m.N {
		if !finiteNormal(v) {
			return core.BadData(op, "normal")
		}
	}
	return nil
}

// At returns the normal at (x, y), or false out of range or corrupt.
// A degenerate (zero) normal is valid storage: shading reads it as flat.
func (m NormalMap) At(x, y int) (Normal, bool) {
	if m.Validate() != nil {
		return Normal{}, false
	}
	if x < 0 || y < 0 || x >= m.W || y >= m.H {
		return Normal{}, false
	}
	return m.N[y*m.W+x], true
}

// normalFactorShared holds the frozen normal-mapped geometry both
// backends run: the 6.1 factor times the diffuse cosine. Miss or bad
// light reads 0; bad normal reads as flat (picture stays, never NaN).
func normalFactorShared(l Light, p core.Vec2, layer Layer, n Normal, height float64) float64 {
	f := l.factorShared(p, layer)
	if f <= 0 {
		return 0
	}
	dir, ok := LightDir(l, p, height)
	if !ok {
		return 0
	}
	return clamp01(f * NdotL(n, dir))
}

// NormalFactor returns the normal-mapped factor at p for layer (CPU
// path). Never NaN, never panics.
func NormalFactor(l Light, p core.Vec2, layer Layer, n Normal, height float64) float64 {
	return normalFactorShared(l, p, layer, n, height)
}

// NormalFactorGPU recomputes the same frozen geometry (GPU mirror):
// same inputs give the identical factor.
func NormalFactorGPU(l Light, p core.Vec2, layer Layer, n Normal, height float64) float64 {
	return normalFactorShared(l, p, layer, n, height)
}

// litNormalShared holds the frozen normal-mapped pixel sum both backends
// run: base*night plus base times each light's factor*cosine*intensity,
// clamped to [0,1], alpha untouched.
func litNormalShared(base core.Color, p core.Vec2, layer Layer, lights []Light, night core.Color, n Normal, height float64) core.Color {
	br := sanitize01(base.R)
	bg := sanitize01(base.G)
	bb := sanitize01(base.B)
	ba := sanitize01(base.A)
	nr, ng, nb := 1.0, 1.0, 1.0
	if validColor(night) {
		nr, ng, nb = night.R, night.G, night.B
	}
	var sr, sg, sb float64
	for i := range lights {
		l := lights[i]
		if !validColor(l.Color) || !finite(l.Intensity) || l.Intensity < 0 || l.Intensity > MaxIntensity {
			continue
		}
		f := normalFactorShared(l, p, layer, n, height)
		if f <= 0 || l.Intensity == 0 {
			continue
		}
		sr += f * l.Intensity * l.Color.R
		sg += f * l.Intensity * l.Color.G
		sb += f * l.Intensity * l.Color.B
	}
	return core.RGBA(
		clamp01(br*nr+br*sr),
		clamp01(bg*ng+bg*sb),
		clamp01(bb*nb+bb*sb),
		ba,
	)
}

// LitNormalCPU lights one normal-mapped pixel (CPU path). Bad night reads
// white; bad lights contribute 0; degenerate normal reads as flat.
func LitNormalCPU(base core.Color, p core.Vec2, layer Layer, lights []Light, night core.Color, n Normal, height float64) core.Color {
	return litNormalShared(base, p, layer, lights, night, n, height)
}

// LitNormalGPU recomputes the same frozen pixel (GPU mirror): bitwise
// identical to LitNormalCPU for the same inputs.
func LitNormalGPU(base core.Color, p core.Vec2, layer Layer, lights []Light, night core.Color, n Normal, height float64) core.Color {
	return litNormalShared(base, p, layer, lights, night, n, height)
}

// NormalScene is one frozen normal-mapped night: global night color plus
// lights plus the lamp height above the plane. NewNormalScene copies the
// slice; Apply lights src after the fx grade using nmap 1:1.
type NormalScene struct {
	Night  core.Color
	Lights []Light
	Height float64
}

// NewNormalScene builds a NormalScene copying lights. Bad night, bad
// light, or height outside (0,MaxLightRange] is InvalidArg; more than
// MaxLights is InvalidArg and builds nothing.
func NewNormalScene(night core.Color, lights []Light, height float64) (NormalScene, error) {
	const op = "light.NewNormalScene"
	if !validColor(night) {
		return NormalScene{}, core.InvalidArg(op, "night")
	}
	if !finite(height) || height <= 0 || height > MaxLightRange {
		return NormalScene{}, core.InvalidArg(op, "height")
	}
	if len(lights) > MaxLights {
		return NormalScene{}, core.InvalidArg(op, "lights")
	}
	for i := range lights {
		if err := lights[i].Validate(); err != nil {
			return NormalScene{}, err
		}
	}
	cp := append([]Light(nil), lights...)
	if cp == nil {
		cp = []Light{}
	}
	for i := range cp {
		if cp[i].Cookie != nil {
			m := append([]float64(nil), cp[i].Cookie.Mask...)
			ck := *cp[i].Cookie
			ck.Mask = m
			cp[i].Cookie = &ck
		}
	}
	return NormalScene{Night: night, Lights: cp, Height: height}, nil
}

// Validate reports InvalidArg for a bad night, a bad light, a bad height,
// or too many lights.
func (s NormalScene) Validate() error {
	const op = "light.NormalScene.Validate"
	if !validColor(s.Night) {
		return core.InvalidArg(op, "night")
	}
	if !finite(s.Height) || s.Height <= 0 || s.Height > MaxLightRange {
		return core.InvalidArg(op, "height")
	}
	if len(s.Lights) > MaxLights {
		return core.InvalidArg(op, "lights")
	}
	for i := range s.Lights {
		if err := s.Lights[i].Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Lit lights one normal-mapped pixel through the scene (CPU path).
func (s NormalScene) Lit(base core.Color, p core.Vec2, layer Layer, n Normal) core.Color {
	return litNormalShared(base, p, layer, s.Lights, s.Night, n, s.Height)
}

// LitGPU recomputes the same pixel (GPU mirror): bitwise identical.
func (s NormalScene) LitGPU(base core.Color, p core.Vec2, layer Layer, n Normal) core.Color {
	return litNormalShared(base, p, layer, s.Lights, s.Night, n, s.Height)
}

// applyNormalShared lights every pixel through lights+night+height into a
// fresh image, reading nmap 1:1. Nil map is InvalidArg (missing normal
// sheet), corrupt map or size mismatch is BadData.
func applyNormalShared(src *Image, nmap *NormalMap, lights []Light, night core.Color, height float64) (*Image, error) {
	const op = "light.NormalScene.Apply"
	if err := src.check(op); err != nil {
		return nil, err
	}
	if nmap == nil {
		return nil, core.InvalidArg(op, "normalmap")
	}
	if err := nmap.Validate(); err != nil {
		return nil, err
	}
	if nmap.W != src.W || nmap.H != src.H {
		return nil, core.BadData(op, "size")
	}
	out := make([]core.Color, len(src.Pix))
	lp := make([]Layer, len(src.Layers))
	copy(lp, src.Layers)
	for y := 0; y < src.H; y++ {
		for x := 0; x < src.W; x++ {
			i := y*src.W + x
			p := core.V2(src.Origin.X+float64(x)*src.Step, src.Origin.Y+float64(y)*src.Step)
			out[i] = litNormalShared(src.Pix[i], p, src.Layers[i], lights, night, nmap.N[i], height)
		}
	}
	return &Image{W: src.W, H: src.H, Origin: src.Origin, Step: src.Step, Pix: out, Layers: lp}, nil
}

// Apply lights src with nmap (CPU path). Neither input is mutated. Nil
// src is InvalidArg, corrupt src BadData, invalid scene InvalidArg, nil
// map InvalidArg, corrupt or mis-sized map BadData.
func (s NormalScene) Apply(src *Image, nmap *NormalMap) (*Image, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return applyNormalShared(src, nmap, s.Lights, s.Night, s.Height)
}

// ApplyCPU is the explicit CPU path (same as Apply).
func (s NormalScene) ApplyCPU(src *Image, nmap *NormalMap) (*Image, error) {
	return s.Apply(src, nmap)
}

// ApplyGPU is the GPU mirror path: same frozen math, bitwise identical.
func (s NormalScene) ApplyGPU(src *Image, nmap *NormalMap) (*Image, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return applyNormalShared(src, nmap, s.Lights, s.Night, s.Height)
}

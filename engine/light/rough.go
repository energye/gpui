//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package light

import (
	"math"

	"github.com/energye/gpui/engine/core"
)

// RoughWeightTol is the allowed drift of Diffuse+Specular away from 1.
// A larger drift is BadData, never silently renormalized.
const RoughWeightTol = 0.001

// FlatRough is the fallback roughness when the sheet is missing:
// middle matte, never black, never a crash.
func FlatRough() float64 { return 0.5 }

func finiteRough(r float64) bool { return finite(r) && r >= 0 && r <= 1 }

func sanitizeRough(r float64) float64 {
	if !finite(r) {
		return FlatRough()
	}
	return clamp01(r)
}

// RoughMap is one isolated roughness sheet: W*H samples in [0,1],
// row-major, aligned 1:1 with the lit Image pixels. 0 reads mirror,
// 1 reads matte. Constructors copy; At only reads.
type RoughMap struct {
	W, H int
	R    []float64
}

// NewRoughMap builds a W-by-H map copying r. Non-positive sizes, a length
// mismatch, or a non-finite/out-of-range sample is BadData; W*H beyond
// MaxImagePixels is OutOfMemory.
func NewRoughMap(w, h int, r []float64) (RoughMap, error) {
	const op = "light.NewRoughMap"
	if w <= 0 || h <= 0 {
		return RoughMap{}, core.BadData(op, "size")
	}
	if int64(w)*int64(h) > int64(MaxImagePixels) {
		return RoughMap{}, core.OutOfMemory(op, "pixels")
	}
	if len(r) != w*h {
		return RoughMap{}, core.BadData(op, "length")
	}
	for _, v := range r {
		if !finiteRough(v) {
			return RoughMap{}, core.BadData(op, "sample")
		}
	}
	cp := make([]float64, len(r))
	copy(cp, r)
	return RoughMap{W: w, H: h, R: cp}, nil
}

// Validate reports BadData for a zero value, a length mismatch, or a
// non-finite/out-of-range sample; OutOfMemory for over-budget.
func (m RoughMap) Validate() error {
	const op = "light.RoughMap.Validate"
	if m.W <= 0 || m.H <= 0 {
		return core.BadData(op, "size")
	}
	if int64(m.W)*int64(m.H) > int64(MaxImagePixels) {
		return core.OutOfMemory(op, "pixels")
	}
	if len(m.R) != m.W*m.H {
		return core.BadData(op, "length")
	}
	for _, v := range m.R {
		if !finiteRough(v) {
			return core.BadData(op, "sample")
		}
	}
	return nil
}

// At returns the roughness at (x, y). Out of range or corrupt reads
// FlatRough with false, never NaN and never a panic.
func (m RoughMap) At(x, y int) (float64, bool) {
	if m.Validate() != nil {
		return FlatRough(), false
	}
	if x < 0 || y < 0 || x >= m.W || y >= m.H {
		return FlatRough(), false
	}
	return m.R[y*m.W+x], true
}

// RoughWeights splits one light share into diffuse and specular.
// The sum must stay 1 within RoughWeightTol; anything else is BadData
// and never silently renormalized.
type RoughWeights struct {
	Diffuse  float64
	Specular float64
}

// NewRoughWeights builds weights. Non-finite, negative, or a sum drifting
// past RoughWeightTol is BadData.
func NewRoughWeights(diffuse, specular float64) (RoughWeights, error) {
	const op = "light.NewRoughWeights"
	w := RoughWeights{Diffuse: diffuse, Specular: specular}
	if err := w.Validate(); err != nil {
		_ = op
		return RoughWeights{}, err
	}
	return w, nil
}

// Validate reports BadData for non-finite, negative, or off-sum weights.
func (w RoughWeights) Validate() error {
	const op = "light.RoughWeights.Validate"
	if !finite(w.Diffuse) || !finite(w.Specular) {
		return core.BadData(op, "weight")
	}
	if w.Diffuse < 0 || w.Specular < 0 {
		return core.BadData(op, "weight")
	}
	if math.Abs(w.Diffuse+w.Specular-1) > RoughWeightTol {
		return core.BadData(op, "weight")
	}
	return nil
}

// DefaultRoughWeights is the frozen split used by the golden pictures:
// mostly diffuse, a visible specular lift.
func DefaultRoughWeights() RoughWeights { return RoughWeights{Diffuse: 0.7, Specular: 0.3} }

func validRoughWeights(w RoughWeights) bool { return w.Validate() == nil }

// RoughSpec is the Blinn-Phong answer for one lamp: half-vector dot,
// shininess 64 at mirror sliding to 8 at matte, scaled by (1-rough) so
// matte kills the highlight. Result in [0,1]; degenerate or non-finite
// input reads 0, never NaN.
func RoughSpec(n, l Normal, rough float64) float64 {
	r := sanitizeRough(rough)
	nn := n.Normalize()
	ll := l.Normalize()
	hx := ll.X
	hy := ll.Y
	hz := ll.Z + 1
	hl := math.Sqrt(hx*hx + hy*hy + hz*hz)
	if !finite(hl) || hl == 0 {
		return 0
	}
	hx /= hl
	hy /= hl
	hz /= hl
	d := nn.X*hx + nn.Y*hy + nn.Z*hz
	if !finite(d) || d <= 0 {
		return 0
	}
	if d > 1 {
		d = 1
	}
	shininess := 8 + (1-r)*56
	s := math.Pow(d, shininess)
	if !finite(s) {
		return 0
	}
	return clamp01(s * (1 - r))
}

// roughFactorShared holds the frozen rough geometry both backends run:
// the 6.1 factor times the weighted diffuse+specular answer. Miss, bad
// light, or bad weights read 0; bad roughness reads as flat.
func roughFactorShared(l Light, p core.Vec2, layer Layer, n Normal, rough float64, height float64, w RoughWeights) float64 {
	if !validRoughWeights(w) {
		return 0
	}
	f := l.factorShared(p, layer)
	if f <= 0 {
		return 0
	}
	dir, ok := LightDir(l, p, height)
	if !ok {
		return 0
	}
	diff := NdotL(n, dir)
	spec := RoughSpec(n, dir, rough)
	return clamp01(f * (w.Diffuse*diff + w.Specular*spec))
}

// RoughFactor returns the rough factor at p (CPU path). Never NaN.
func RoughFactor(l Light, p core.Vec2, layer Layer, n Normal, rough float64, height float64, w RoughWeights) float64 {
	return roughFactorShared(l, p, layer, n, rough, height, w)
}

// RoughFactorGPU recomputes the same frozen geometry (GPU mirror):
// same inputs give the identical factor.
func RoughFactorGPU(l Light, p core.Vec2, layer Layer, n Normal, rough float64, height float64, w RoughWeights) float64 {
	return roughFactorShared(l, p, layer, n, rough, height, w)
}

// litRoughShared holds the frozen rough pixel sum both backends run:
// base*night plus base times each light's rough factor*intensity,
// clamped to [0,1], alpha untouched.
func litRoughShared(base core.Color, p core.Vec2, layer Layer, lights []Light, night core.Color, n Normal, rough float64, height float64, w RoughWeights) core.Color {
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
		f := roughFactorShared(l, p, layer, n, rough, height, w)
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

// LitRoughCPU lights one rough pixel (CPU path). Bad night reads white;
// bad lights contribute 0; degenerate normal reads as flat, bad
// roughness as FlatRough, bad weights as 0 share.
func LitRoughCPU(base core.Color, p core.Vec2, layer Layer, lights []Light, night core.Color, n Normal, rough float64, height float64, w RoughWeights) core.Color {
	return litRoughShared(base, p, layer, lights, night, n, rough, height, w)
}

// LitRoughGPU recomputes the same frozen pixel (GPU mirror): bitwise
// identical to LitRoughCPU for the same inputs.
func LitRoughGPU(base core.Color, p core.Vec2, layer Layer, lights []Light, night core.Color, n Normal, rough float64, height float64, w RoughWeights) core.Color {
	return litRoughShared(base, p, layer, lights, night, n, rough, height, w)
}

// RoughScene is one frozen rough night: global night color plus lights
// plus the lamp height plus the diffuse/specular split. NewRoughScene
// copies the slice; Apply lights src after the fx grade using nmap and
// rmap 1:1. A nil map falls back (flat normal, FlatRough), never a
// crash; a corrupt or mis-sized map is BadData.
type RoughScene struct {
	Night   core.Color
	Lights  []Light
	Height  float64
	Weights RoughWeights
}

// NewRoughScene builds a RoughScene copying lights. Bad night, bad light,
// or height outside (0,MaxLightRange] is InvalidArg; more than MaxLights
// is InvalidArg; bad weights is BadData. Builds nothing on error.
func NewRoughScene(night core.Color, lights []Light, height float64, w RoughWeights) (RoughScene, error) {
	const op = "light.NewRoughScene"
	if !validColor(night) {
		return RoughScene{}, core.InvalidArg(op, "night")
	}
	if !finite(height) || height <= 0 || height > MaxLightRange {
		return RoughScene{}, core.InvalidArg(op, "height")
	}
	if len(lights) > MaxLights {
		return RoughScene{}, core.InvalidArg(op, "lights")
	}
	for i := range lights {
		if err := lights[i].Validate(); err != nil {
			return RoughScene{}, err
		}
	}
	if err := w.Validate(); err != nil {
		return RoughScene{}, err
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
	return RoughScene{Night: night, Lights: cp, Height: height, Weights: w}, nil
}

// Validate reports InvalidArg for a bad night, light, height, or count;
// BadData for off-sum weights.
func (s RoughScene) Validate() error {
	const op = "light.RoughScene.Validate"
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
	if err := s.Weights.Validate(); err != nil {
		return err
	}
	return nil
}

// Lit lights one rough pixel through the scene (CPU path).
func (s RoughScene) Lit(base core.Color, p core.Vec2, layer Layer, n Normal, rough float64) core.Color {
	return litRoughShared(base, p, layer, s.Lights, s.Night, n, rough, s.Height, s.Weights)
}

// LitGPU recomputes the same pixel (GPU mirror): bitwise identical.
func (s RoughScene) LitGPU(base core.Color, p core.Vec2, layer Layer, n Normal, rough float64) core.Color {
	return litRoughShared(base, p, layer, s.Lights, s.Night, n, rough, s.Height, s.Weights)
}

// applyRoughShared lights every pixel through lights+night+height+weights
// into a fresh image, reading nmap and rmap 1:1. Nil maps fall back per
// pixel (flat normal, FlatRough); corrupt or mis-sized maps are BadData.
func applyRoughShared(src *Image, nmap *NormalMap, rmap *RoughMap, lights []Light, night core.Color, height float64, w RoughWeights) (*Image, error) {
	const op = "light.RoughScene.Apply"
	if err := src.check(op); err != nil {
		return nil, err
	}
	if nmap != nil {
		if err := nmap.Validate(); err != nil {
			return nil, err
		}
		if nmap.W != src.W || nmap.H != src.H {
			return nil, core.BadData(op, "size")
		}
	}
	if rmap != nil {
		if err := rmap.Validate(); err != nil {
			return nil, err
		}
		if rmap.W != src.W || rmap.H != src.H {
			return nil, core.BadData(op, "size")
		}
	}
	out := make([]core.Color, len(src.Pix))
	lp := make([]Layer, len(src.Layers))
	copy(lp, src.Layers)
	for y := 0; y < src.H; y++ {
		for x := 0; x < src.W; x++ {
			i := y*src.W + x
			p := core.V2(src.Origin.X+float64(x)*src.Step, src.Origin.Y+float64(y)*src.Step)
			n := FlatNormal()
			if nmap != nil {
				n = nmap.N[i]
			}
			r := FlatRough()
			if rmap != nil {
				r = rmap.R[i]
			}
			out[i] = litRoughShared(src.Pix[i], p, src.Layers[i], lights, night, n, r, height, w)
		}
	}
	return &Image{W: src.W, H: src.H, Origin: src.Origin, Step: src.Step, Pix: out, Layers: lp}, nil
}

// Apply lights src with nmap and rmap (CPU path). Neither input is
// mutated. Nil src is InvalidArg, corrupt src BadData, invalid scene
// InvalidArg, nil maps fall back, corrupt or mis-sized maps BadData.
func (s RoughScene) Apply(src *Image, nmap *NormalMap, rmap *RoughMap) (*Image, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return applyRoughShared(src, nmap, rmap, s.Lights, s.Night, s.Height, s.Weights)
}

// ApplyCPU is the explicit CPU path (same as Apply).
func (s RoughScene) ApplyCPU(src *Image, nmap *NormalMap, rmap *RoughMap) (*Image, error) {
	return s.Apply(src, nmap, rmap)
}

// ApplyGPU is the GPU mirror path: same frozen math, bitwise identical.
func (s RoughScene) ApplyGPU(src *Image, nmap *NormalMap, rmap *RoughMap) (*Image, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return applyRoughShared(src, nmap, rmap, s.Lights, s.Night, s.Height, s.Weights)
}

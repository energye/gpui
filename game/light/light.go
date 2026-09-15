package light

import (
	"math"

	"github.com/energye/gpui/game/core"
)

// Budgets. Over-budget pictures report OutOfMemory; over-count scenes and
// out-of-range scalars report InvalidArg. Never allocate then fail.
const (
	MaxLightRange      = 8192.0
	MaxIntensity       = 8.0
	MaxLights          = 256
	MaxCookieDimension = 512
	MaxImagePixels     = 16 << 20
)

// Layer is the draw band a light touches. Values mirror game/sprite bands
// so callers pass the same numbers; the rule stays local here.
type Layer int

const (
	LayerWorld Layer = 0
	LayerFX    Layer = 1
	LayerUI    Layer = 2
)

// LayersFor packs bands into a mask. Empty means lights nothing.
func LayersFor(layers ...Layer) uint32 {
	var m uint32
	for _, l := range layers {
		if l >= 0 && l < 32 {
			m |= 1 << uint(l)
		}
	}
	return m
}

// HitsLayer reports whether mask covers layer. Out-of-range layers miss.
func HitsLayer(mask uint32, layer Layer) bool {
	if layer < 0 || layer >= 32 {
		return false
	}
	return mask&(1<<uint(layer)) != 0
}

// Kind names the light shape.
type Kind int

const (
	KindPoint Kind = iota
	KindDirectional
)

// String returns the stable log name of k.
func (k Kind) String() string {
	switch k {
	case KindDirectional:
		return "directional"
	default:
		return "point"
	}
}

// ParseKind maps "point"/"directional" to a Kind. Anything else is
// InvalidArg and returns KindPoint.
func ParseKind(s string) (Kind, error) {
	switch s {
	case "point":
		return KindPoint, nil
	case "directional":
		return KindDirectional, nil
	default:
		return KindPoint, core.InvalidArg("light.ParseKind", s)
	}
}

func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

func finiteVec(v core.Vec2) bool { return finite(v.X) && finite(v.Y) }

func finiteColor(c core.Color) bool {
	return finite(c.R) && finite(c.G) && finite(c.B) && finite(c.A)
}

func in01(x float64) bool { return finite(x) && x >= 0 && x <= 1 }

func validColor(c core.Color) bool {
	return in01(c.R) && in01(c.G) && in01(c.B) && in01(c.A)
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

// sanitize01 maps non-finite to 0 then clamps (hot path never emits NaN).
func sanitize01(x float64) float64 {
	if !finite(x) {
		return 0
	}
	return clamp01(x)
}

// Cookie is a flashlight slide: W*H samples in [0,1], row-major.
// Nil cookie means bare bulb. Always copied on the way in.
type Cookie struct {
	W, H int
	Mask []float64
}

// NewCookie builds a Cookie copying mask. Bad sizes are InvalidArg, a
// length mismatch is BadData, a non-finite or out-of-range sample is
// BadData, over-dimension is OutOfMemory.
func NewCookie(w, h int, mask []float64) (Cookie, error) {
	const op = "light.NewCookie"
	if w <= 0 || h <= 0 {
		return Cookie{}, core.InvalidArg(op, "size")
	}
	if w > MaxCookieDimension || h > MaxCookieDimension {
		return Cookie{}, core.OutOfMemory(op, "size")
	}
	if len(mask) != w*h {
		return Cookie{}, core.BadData(op, "length")
	}
	for _, v := range mask {
		if !in01(v) {
			return Cookie{}, core.BadData(op, "sample")
		}
	}
	cp := make([]float64, len(mask))
	copy(cp, mask)
	return Cookie{W: w, H: h, Mask: cp}, nil
}

// Validate reports InvalidArg for a zero value, BadData for corrupt
// samples, OutOfMemory for over-dimension.
func (c Cookie) Validate() error {
	const op = "light.Cookie.Validate"
	if c.W <= 0 || c.H <= 0 {
		return core.InvalidArg(op, "size")
	}
	if c.W > MaxCookieDimension || c.H > MaxCookieDimension {
		return core.OutOfMemory(op, "size")
	}
	if len(c.Mask) != c.W*c.H {
		return core.BadData(op, "length")
	}
	for _, v := range c.Mask {
		if !in01(v) {
			return core.BadData(op, "sample")
		}
	}
	return nil
}

// At returns the texel at (x, y), or false when out of range or corrupt.
func (c Cookie) At(x, y int) (float64, bool) {
	if c.Validate() != nil {
		return 0, false
	}
	if x < 0 || y < 0 || x >= c.W || y >= c.H {
		return 0, false
	}
	return c.Mask[y*c.W+x], true
}

// Sample reads uv in [0,1] with bilinear filtering and clamped edges.
// Invalid cookie or non-finite uv reads 0 (FactorAt substitutes 1 for a
// missing slide; direct Sample stays fail-closed).
func (c Cookie) Sample(uv core.Vec2) float64 {
	if c.Validate() != nil {
		return 0
	}
	if !finiteVec(uv) {
		return 0
	}
	u := clamp01(uv.X)
	v := clamp01(uv.Y)
	x := u * float64(c.W-1)
	y := v * float64(c.H-1)
	x0 := int(math.Floor(x))
	y0 := int(math.Floor(y))
	x1 := x0 + 1
	if x1 > c.W-1 {
		x1 = c.W - 1
	}
	y1 := y0 + 1
	if y1 > c.H-1 {
		y1 = c.H - 1
	}
	fx := x - float64(x0)
	fy := y - float64(y0)
	a := c.Mask[y0*c.W+x0]
	b := c.Mask[y0*c.W+x1]
	d := c.Mask[y1*c.W+x0]
	e := c.Mask[y1*c.W+x1]
	return (a*(1-fx)+b*fx)*(1-fy) + (d*(1-fx)+e*fx)*fy
}

// Light is one frozen 2D light. Point uses Pos/Range/Cookie; directional
// uses Dir and ignores Pos/Range/Cookie. Layers is a bit mask over Layer
// (bit l covers band l). Cookie nil means bare bulb.
type Light struct {
	Kind      Kind
	Pos       core.Vec2
	Dir       core.Vec2
	Range     float64
	Color     core.Color
	Intensity float64
	Layers    uint32
	Cookie    *Cookie
}

// NewPointLight builds a point light. Pos must be finite, Range in
// (0,MaxLightRange], Color components in [0,1], Intensity in
// [0,MaxIntensity], cookie nil or valid. Anything else is InvalidArg.
func NewPointLight(pos core.Vec2, rng float64, color core.Color, intensity float64, layers uint32, cookie *Cookie) (Light, error) {
	const op = "light.NewPointLight"
	l := Light{Kind: KindPoint, Pos: pos, Range: rng, Color: color, Intensity: intensity, Layers: layers}
	if cookie != nil {
		cp := *cookie
		cp.Mask = append([]float64(nil), cookie.Mask...)
		l.Cookie = &cp
	}
	if err := l.Validate(); err != nil {
		_ = op
		return Light{}, err
	}
	return l, nil
}

// NewDirectionalLight builds a directional light. Dir must be finite and
// non-zero, Color in [0,1], Intensity in [0,MaxIntensity]. Anything else
// is InvalidArg. Direction is carried for 6.3 shadows; the 6.1 factor is
// uniform over hit layers.
func NewDirectionalLight(dir core.Vec2, color core.Color, intensity float64, layers uint32) (Light, error) {
	const op = "light.NewDirectionalLight"
	l := Light{Kind: KindDirectional, Dir: dir, Color: color, Intensity: intensity, Layers: layers}
	if err := l.Validate(); err != nil {
		_ = op
		return Light{}, err
	}
	return l, nil
}

// Validate reports InvalidArg for a bad light. Point checks Pos/Range;
// directional checks Dir; both check Color/Intensity; cookie, when set,
// must validate.
func (l Light) Validate() error {
	const op = "light.Light.Validate"
	if l.Kind != KindPoint && l.Kind != KindDirectional {
		return core.InvalidArg(op, "kind")
	}
	if !validColor(l.Color) {
		return core.InvalidArg(op, "color")
	}
	if !finite(l.Intensity) || l.Intensity < 0 || l.Intensity > MaxIntensity {
		return core.InvalidArg(op, "intensity")
	}
	switch l.Kind {
	case KindPoint:
		if !finiteVec(l.Pos) {
			return core.InvalidArg(op, "pos")
		}
		if !finite(l.Range) || l.Range <= 0 || l.Range > MaxLightRange {
			return core.InvalidArg(op, "range")
		}
	case KindDirectional:
		if !finiteVec(l.Dir) || l.Dir.IsZero() {
			return core.InvalidArg(op, "dir")
		}
	}
	if l.Cookie != nil {
		if err := l.Cookie.Validate(); err != nil {
			return core.InvalidArg(op, "cookie")
		}
	}
	return nil
}

// HitsLayer reports whether l touches layer.
func (l Light) HitsLayer(layer Layer) bool { return HitsLayer(l.Layers, layer) }

// atten maps t = 1-dist/range in [0,1] through a Hermite ramp.
func atten(t float64) float64 {
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	return t * t * (3 - 2*t)
}

// cookieFactor samples the slide at world p. Nil or corrupt cookie reads
// 1 (missing slide falls back to bare bulb, never dark).
func (l Light) cookieFactor(p core.Vec2) float64 {
	if l.Cookie == nil || l.Cookie.Validate() != nil {
		return 1
	}
	u := (p.X - (l.Pos.X - l.Range)) / (2 * l.Range)
	v := (p.Y - (l.Pos.Y - l.Range)) / (2 * l.Range)
	if u < 0 || u > 1 || v < 0 || v > 1 {
		return 0
	}
	return clamp01(l.Cookie.Sample(core.V2(u, v)))
}

// factorShared holds the frozen geometry both backends run.
func (l Light) factorShared(p core.Vec2, layer Layer) float64 {
	if !finiteVec(p) || !l.HitsLayer(layer) {
		return 0
	}
	switch l.Kind {
	case KindDirectional:
		if !finiteVec(l.Dir) || l.Dir.IsZero() {
			return 0
		}
		return 1
	default:
		if l.Kind != KindPoint {
			return 0
		}
		if !finiteVec(l.Pos) || !finite(l.Range) || l.Range <= 0 || l.Range > MaxLightRange {
			return 0
		}
		dx := p.X - l.Pos.X
		dy := p.Y - l.Pos.Y
		dist := math.Sqrt(dx*dx + dy*dy)
		if !finite(dist) || dist >= l.Range {
			return 0
		}
		a := atten(1 - dist/l.Range)
		return clamp01(a * l.cookieFactor(p))
	}
}

// FactorAt returns the geometric factor at p for layer (CPU path):
// attenuation times cookie, 1 for a hit directional, 0 on a miss or any
// bad input. Never NaN, never panics.
func (l Light) FactorAt(p core.Vec2, layer Layer) float64 {
	return l.factorShared(p, layer)
}

// FactorGPU recomputes the same frozen geometry (GPU mirror): same inputs
// give the identical factor, so CPU and GPU受光系数 agree by construction.
func (l Light) FactorGPU(p core.Vec2, layer Layer) float64 {
	return l.factorShared(p, layer)
}

// litShared holds the frozen pixel sum both backends run.
func litShared(base core.Color, p core.Vec2, layer Layer, lights []Light, night core.Color) core.Color {
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
		f := l.factorShared(p, layer)
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

// LitCPU lights one pixel (CPU path). Bad night reads white (never black
// screen); bad lights contribute 0; alpha rides through.
func LitCPU(base core.Color, p core.Vec2, layer Layer, lights []Light, night core.Color) core.Color {
	return litShared(base, p, layer, lights, night)
}

// LitGPU recomputes the same frozen pixel (GPU mirror): bitwise identical
// to LitCPU for the same inputs.
func LitGPU(base core.Color, p core.Vec2, layer Layer, lights []Light, night core.Color) core.Color {
	return litShared(base, p, layer, lights, night)
}

// Image is one isolated lighting intermediate: W*H base colors plus a
// per-pixel layer band, row-major. Origin is the world position of pixel
// (0,0)'s center; Step is world units per pixel. Constructors copy;
// Apply returns fresh images; Clone copies again.
type Image struct {
	W, H   int
	Origin core.Vec2
	Step   float64
	Pix    []core.Color
	Layers []Layer
}

// NewImage builds a W-by-H transparent image on LayerWorld. Non-positive
// sizes or non-positive/non-finite step or non-finite origin are
// InvalidArg; W*H beyond MaxImagePixels is OutOfMemory.
func NewImage(w, h int, origin core.Vec2, step float64) (*Image, error) {
	const op = "light.NewImage"
	if w <= 0 || h <= 0 {
		return nil, core.InvalidArg(op, "size")
	}
	if !finite(step) || step <= 0 {
		return nil, core.InvalidArg(op, "step")
	}
	if !finiteVec(origin) {
		return nil, core.InvalidArg(op, "origin")
	}
	if int64(w)*int64(h) > int64(MaxImagePixels) {
		return nil, core.OutOfMemory(op, "pixels")
	}
	return &Image{
		W: w, H: h, Origin: origin, Step: step,
		Pix: make([]core.Color, w*h), Layers: make([]Layer, w*h),
	}, nil
}

// NewImageFromColors builds an image copying pix and layers. Bad sizes
// are InvalidArg, length mismatches are BadData, over-budget is
// OutOfMemory, a non-finite pixel is BadData.
func NewImageFromColors(w, h int, origin core.Vec2, step float64, pix []core.Color, layers []Layer) (*Image, error) {
	const op = "light.NewImageFromColors"
	if w <= 0 || h <= 0 {
		return nil, core.InvalidArg(op, "size")
	}
	if !finite(step) || step <= 0 {
		return nil, core.InvalidArg(op, "step")
	}
	if !finiteVec(origin) {
		return nil, core.InvalidArg(op, "origin")
	}
	if int64(w)*int64(h) > int64(MaxImagePixels) {
		return nil, core.OutOfMemory(op, "pixels")
	}
	if len(pix) != w*h || len(layers) != w*h {
		return nil, core.BadData(op, "length")
	}
	for _, c := range pix {
		if !finiteColor(c) {
			return nil, core.BadData(op, "pixel")
		}
	}
	cp := make([]core.Color, len(pix))
	copy(cp, pix)
	lp := make([]Layer, len(layers))
	copy(lp, layers)
	return &Image{W: w, H: h, Origin: origin, Step: step, Pix: cp, Layers: lp}, nil
}

func (img *Image) valid() bool {
	return img != nil && img.W > 0 && img.H > 0 &&
		finite(img.Step) && img.Step > 0 && finiteVec(img.Origin) &&
		len(img.Pix) == img.W*img.H && len(img.Layers) == img.W*img.H
}

func (img *Image) check(op string) error {
	if img == nil {
		return core.InvalidArg(op, "image")
	}
	if !img.valid() {
		return core.BadData(op, "image")
	}
	return nil
}

// PosAt returns the world position of pixel (x, y)'s center.
func (img *Image) PosAt(x, y int) (core.Vec2, bool) {
	if !img.valid() || x < 0 || y < 0 || x >= img.W || y >= img.H {
		return core.Vec2{}, false
	}
	return core.V2(
		img.Origin.X+float64(x)*img.Step,
		img.Origin.Y+float64(y)*img.Step,
	), true
}

// At returns the pixel and its layer at (x, y), or false out of range.
func (img *Image) At(x, y int) (core.Color, Layer, bool) {
	if !img.valid() || x < 0 || y < 0 || x >= img.W || y >= img.H {
		return core.Color{}, LayerWorld, false
	}
	return img.Pix[y*img.W+x], img.Layers[y*img.W+x], true
}

// Set stores c and layer at (x, y). False on bad coordinates, corrupt
// image, or non-finite c; stores nothing then.
func (img *Image) Set(x, y int, c core.Color, layer Layer) bool {
	if !img.valid() || x < 0 || y < 0 || x >= img.W || y >= img.H {
		return false
	}
	if !finiteColor(c) {
		return false
	}
	img.Pix[y*img.W+x] = c
	img.Layers[y*img.W+x] = layer
	return true
}

// Clone returns a fresh copy, or nil for a nil/corrupt image.
func (img *Image) Clone() *Image {
	if !img.valid() {
		return nil
	}
	cp := make([]core.Color, len(img.Pix))
	copy(cp, img.Pix)
	lp := make([]Layer, len(img.Layers))
	copy(lp, img.Layers)
	return &Image{W: img.W, H: img.H, Origin: img.Origin, Step: img.Step, Pix: cp, Layers: lp}
}

// ApproxEqual reports whether o matches in geometry and every pixel/layer.
// Non-positive eps means exact equality.
func (img *Image) ApproxEqual(o *Image, eps float64) bool {
	if !img.valid() || !o.valid() ||
		img.W != o.W || img.H != o.H ||
		img.Origin != o.Origin || img.Step != o.Step {
		return false
	}
	for i := range img.Layers {
		if img.Layers[i] != o.Layers[i] {
			return false
		}
	}
	if eps <= 0 {
		for i := range img.Pix {
			if img.Pix[i] != o.Pix[i] {
				return false
			}
		}
		return true
	}
	for i := range img.Pix {
		if !img.Pix[i].ApproxEqual(o.Pix[i], eps) {
			return false
		}
	}
	return true
}

// Scene is one frozen night: global night color plus lights. NewScene
// copies the slice; Apply lights src after the fx grade.
type Scene struct {
	Night  core.Color
	Lights []Light
}

// NewScene builds a Scene copying lights. Bad night or bad light is
// InvalidArg; more than MaxLights is InvalidArg and builds nothing.
func NewScene(night core.Color, lights []Light) (Scene, error) {
	const op = "light.NewScene"
	if !validColor(night) {
		return Scene{}, core.InvalidArg(op, "night")
	}
	if len(lights) > MaxLights {
		return Scene{}, core.InvalidArg(op, "lights")
	}
	for i := range lights {
		if err := lights[i].Validate(); err != nil {
			return Scene{}, err
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
	return Scene{Night: night, Lights: cp}, nil
}

// Validate reports InvalidArg for a bad night, a bad light, or too many
// lights.
func (s Scene) Validate() error {
	const op = "light.Scene.Validate"
	if !validColor(s.Night) {
		return core.InvalidArg(op, "night")
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

// Lit lights one pixel through the scene (CPU path).
func (s Scene) Lit(base core.Color, p core.Vec2, layer Layer) core.Color {
	return litShared(base, p, layer, s.Lights, s.Night)
}

// LitGPU recomputes the same pixel (GPU mirror): bitwise identical.
func (s Scene) LitGPU(base core.Color, p core.Vec2, layer Layer) core.Color {
	return litShared(base, p, layer, s.Lights, s.Night)
}

// applyShared lights every pixel through lights+night into a fresh image.
func applyShared(src *Image, lights []Light, night core.Color) (*Image, error) {
	const op = "light.Scene.Apply"
	if err := src.check(op); err != nil {
		return nil, err
	}
	out := make([]core.Color, len(src.Pix))
	lp := make([]Layer, len(src.Layers))
	copy(lp, src.Layers)
	for y := 0; y < src.H; y++ {
		for x := 0; x < src.W; x++ {
			i := y*src.W + x
			p := core.V2(src.Origin.X+float64(x)*src.Step, src.Origin.Y+float64(y)*src.Step)
			out[i] = litShared(src.Pix[i], p, src.Layers[i], lights, night)
		}
	}
	return &Image{W: src.W, H: src.H, Origin: src.Origin, Step: src.Step, Pix: out, Layers: lp}, nil
}

// Apply lights src (CPU path). src never mutated. Nil src is InvalidArg,
// corrupt src BadData, invalid scene InvalidArg.
func (s Scene) Apply(src *Image) (*Image, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return applyShared(src, s.Lights, s.Night)
}

// ApplyCPU is the explicit CPU path (same as Apply).
func (s Scene) ApplyCPU(src *Image) (*Image, error) {
	return s.Apply(src)
}

// ApplyGPU is the GPU mirror path: same frozen math, bitwise identical.
func (s Scene) ApplyGPU(src *Image) (*Image, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return applyShared(src, s.Lights, s.Night)
}

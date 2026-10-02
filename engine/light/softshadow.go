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
	"github.com/energye/gpui/engine/core"
)

// S63 soft shadows: PCF taps over the frozen shadow core plus billing.
//
// What it does: SoftShadow wraps a frozen Shadow without touching its
// geometry. Mode picks the edge softness (None is one tap, PCF5 averages
// 5 taps, PCF13 averages 13 taps, PCF5 is the default). Radius is world
// units per tap step. Billing counts how many pixels each light covers
// per frame so big lights pay by footprint. Over-budget lights warn,
// OOM always steps one grade down with a log line, never silently.
const (
	// MaxShadowLights caps shadowed lights for 60 fps.
	MaxShadowLights = 10
	// MaxPCF13Lights caps PCF13 lights: only a few may use 13 taps.
	MaxPCF13Lights = 2
	// LargeLightPixelsWarn fires when one light covers this many pixels.
	LargeLightPixelsWarn = 2048
	// MinNightChannel keeps night auto-readable: never black screen.
	MinNightChannel = 0.03
	// MinLitPixels counts lit pixels for the gain gate.
	MinLitPixels = 1000
	// MinLitGain is the mean gain lit pixels must beat.
	MinLitGain = 0.6
	// DefaultSoftRadius is the tap step when callers pass 0.
	DefaultSoftRadius = 0.5
)

// SoftMode names the shadow edge grade.
type SoftMode int

const (
	// SoftNone is one hard tap.
	SoftNone SoftMode = iota
	// SoftPCF5 averages center plus 4 axial taps.
	SoftPCF5
	// SoftPCF13 averages center plus 12 ring taps.
	SoftPCF13
)

// DefaultSoftMode is the middle grade.
const DefaultSoftMode = SoftPCF5

// String returns the stable log name of m.
func (m SoftMode) String() string {
	switch m {
	case SoftNone:
		return "None"
	case SoftPCF5:
		return "PCF5"
	case SoftPCF13:
		return "PCF13"
	default:
		return "Unknown"
	}
}

// ParseSoftMode maps "None"/"PCF5"/"PCF13" to a SoftMode.
func ParseSoftMode(s string) (SoftMode, error) {
	switch s {
	case "None":
		return SoftNone, nil
	case "PCF5":
		return SoftPCF5, nil
	case "PCF13":
		return SoftPCF13, nil
	default:
		return SoftNone, core.InvalidArg("light.ParseSoftMode", s)
	}
}

// Validate reports InvalidArg for an unknown grade.
func (m SoftMode) Validate() error {
	switch m {
	case SoftNone, SoftPCF5, SoftPCF13:
		return nil
	default:
		return core.InvalidArg("light.SoftMode.Validate", m.String())
	}
}

// Taps returns the tap count: 1, 5, or 13. Unknown reads 1, never 0.
func (m SoftMode) Taps() int {
	switch m {
	case SoftPCF5:
		return 5
	case SoftPCF13:
		return 13
	default:
		return 1
	}
}

var softPCF5Offsets = []core.Vec2{
	{X: 0, Y: 0},
	{X: 1, Y: 0},
	{X: -1, Y: 0},
	{X: 0, Y: 1},
	{X: 0, Y: -1},
}

var softPCF13Offsets = []core.Vec2{
	{X: 0, Y: 0},
	{X: 1, Y: 0},
	{X: -1, Y: 0},
	{X: 0, Y: 1},
	{X: 0, Y: -1},
	{X: 1, Y: 1},
	{X: 1, Y: -1},
	{X: -1, Y: 1},
	{X: -1, Y: -1},
	{X: 2, Y: 0},
	{X: -2, Y: 0},
	{X: 0, Y: 2},
	{X: 0, Y: -2},
}

// SoftShadow is a frozen shadow plus an edge grade. Shadow keeps the
// frozen occluders, Mode picks the tap count, Radius is world units per
// tap step (0 means DefaultSoftRadius).
type SoftShadow struct {
	Shadow Shadow
	Mode   SoftMode
	Radius float64
}

// NewSoftShadow builds a SoftShadow. Bad mode or bad radius is
// InvalidArg, bad shadow fails with its own code. Radius 0 reads as
// DefaultSoftRadius.
func NewSoftShadow(sh Shadow, mode SoftMode, radius float64) (SoftShadow, error) {
	const op = "light.NewSoftShadow"
	if err := mode.Validate(); err != nil {
		return SoftShadow{}, err
	}
	if radius == 0 {
		radius = DefaultSoftRadius
	}
	if !finite(radius) || radius < 0 || radius > MaxLightRange {
		return SoftShadow{}, core.InvalidArg(op, "radius")
	}
	if err := sh.Validate(); err != nil {
		return SoftShadow{}, err
	}
	return SoftShadow{Shadow: sh, Mode: mode, Radius: radius}, nil
}

// Validate reports InvalidArg for a bad mode, radius, or shadow.
func (s SoftShadow) Validate() error {
	const op = "light.SoftShadow.Validate"
	if err := s.Mode.Validate(); err != nil {
		return err
	}
	r := s.Radius
	if r == 0 {
		r = DefaultSoftRadius
	}
	if !finite(r) || r < 0 || r > MaxLightRange {
		return core.InvalidArg(op, "radius")
	}
	return s.Shadow.Validate()
}

// Taps returns the tap count of the current grade.
func (s SoftShadow) Taps() int { return s.Mode.Taps() }

// radiusOrDefault maps 0 to DefaultSoftRadius.
func (s SoftShadow) radiusOrDefault() float64 {
	if s.Radius == 0 {
		return DefaultSoftRadius
	}
	return s.Radius
}

// softFactorShared averages frozen taps around p. One tap for None,
// 5 or 13 taps for PCF. Bad geometry reads through the hard core.
func (s SoftShadow) softFactorShared(l Light, p core.Vec2) float64 {
	if !finiteVec(p) {
		return 1
	}
	if s.Mode == SoftNone {
		return s.Shadow.FactorFor(l, p)
	}
	r := s.radiusOrDefault()
	if !finite(r) || r <= 0 {
		return s.Shadow.FactorFor(l, p)
	}
	var offs []core.Vec2
	if s.Mode == SoftPCF13 {
		offs = softPCF13Offsets
	} else {
		offs = softPCF5Offsets
	}
	sum := 0.0
	for _, o := range offs {
		q := core.Vec2{X: p.X + o.X*r, Y: p.Y + o.Y*r}
		if !finiteVec(q) {
			sum += 1
			continue
		}
		sum += s.Shadow.FactorFor(l, q)
	}
	return clamp01(sum / float64(len(offs)))
}

// SoftFactor returns the soft multiplier at p (CPU path).
func (s SoftShadow) SoftFactor(l Light, p core.Vec2) float64 {
	return s.softFactorShared(l, p)
}

// SoftFactorGPU recomputes the same taps (GPU mirror): identical.
func (s SoftShadow) SoftFactorGPU(l Light, p core.Vec2) float64 {
	return s.softFactorShared(l, p)
}

// ClampNightFloor lifts dark nights to the readable floor. Invalid night
// reads white (never black screen), else RGB pinned at MinNightChannel.
func ClampNightFloor(night core.Color) core.Color {
	if !validColor(night) {
		return core.White
	}
	r, g, b := night.R, night.G, night.B
	if r < MinNightChannel {
		r = MinNightChannel
	}
	if g < MinNightChannel {
		g = MinNightChannel
	}
	if b < MinNightChannel {
		b = MinNightChannel
	}
	return core.RGBA(r, g, b, night.A)
}

// CheckNightFloor reports InvalidArg for an invalid night or a channel
// below MinNightChannel.
func CheckNightFloor(night core.Color) error {
	const op = "light.CheckNightFloor"
	if !validColor(night) {
		return core.InvalidArg(op, "night")
	}
	if night.R < MinNightChannel || night.G < MinNightChannel || night.B < MinNightChannel {
		return core.InvalidArg(op, "night")
	}
	return nil
}

// NightOnlyPixel is the torch-miss reference: base times floored night.
func NightOnlyPixel(base, night core.Color) core.Color {
	n := ClampNightFloor(night)
	return core.RGBA(
		clamp01(sanitize01(base.R)*sanitize01(n.R)),
		clamp01(sanitize01(base.G)*sanitize01(n.G)),
		clamp01(sanitize01(base.B)*sanitize01(n.B)),
		sanitize01(base.A),
	)
}

// softLitShared lights one pixel with soft taps plus the floored night.
func (s SoftShadow) softLitShared(base core.Color, p core.Vec2, layer Layer, lights []Light, night core.Color) core.Color {
	br := sanitize01(base.R)
	bg := sanitize01(base.G)
	bb := sanitize01(base.B)
	ba := sanitize01(base.A)
	fn := ClampNightFloor(night)
	nr, ng, nb := fn.R, fn.G, fn.B
	var sr, sg, sb float64
	for i := range lights {
		l := lights[i]
		if !validColor(l.Color) || !finite(l.Intensity) || l.Intensity < 0 || l.Intensity > MaxIntensity {
			continue
		}
		f := l.FactorAt(p, layer)
		if f <= 0 || l.Intensity == 0 {
			continue
		}
		f *= s.softFactorShared(l, p)
		if f <= 0 {
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

// SoftLit lights one pixel through soft taps (CPU path).
func (s SoftShadow) SoftLit(base core.Color, p core.Vec2, layer Layer, lights []Light, night core.Color) core.Color {
	return s.softLitShared(base, p, layer, lights, night)
}

// SoftLitGPU recomputes the same pixel (GPU mirror): identical.
func (s SoftShadow) SoftLitGPU(base core.Color, p core.Vec2, layer Layer, lights []Light, night core.Color) core.Color {
	return s.softLitShared(base, p, layer, lights, night)
}

// FrameBill is one frame of light billing: covered pixels per light,
// total footprint, taps, and cost (total times taps).
type FrameBill struct {
	PerLight []int
	Total    int
	Taps     int
	Cost     int
}

// CoverPixels counts pixels where l contributes (FactorAt above 0).
// Nil or corrupt src reads 0, never panics.
func CoverPixels(l Light, src *Image) int {
	if src == nil || !src.valid() {
		return 0
	}
	n := 0
	for y := 0; y < src.H; y++ {
		for x := 0; x < src.W; x++ {
			i := y*src.W + x
			p := core.V2(src.Origin.X+float64(x)*src.Step, src.Origin.Y+float64(y)*src.Step)
			if l.FactorAt(p, src.Layers[i]) > 0 {
				n++
			}
		}
	}
	return n
}

// Bill charges the frame: per-light footprints plus total cost.
func (s SoftShadow) Bill(src *Image, lights []Light) FrameBill {
	taps := s.Taps()
	per := make([]int, len(lights))
	total := 0
	for i := range lights {
		per[i] = CoverPixels(lights[i], src)
		total += per[i]
	}
	if per == nil {
		per = []int{}
	}
	return FrameBill{PerLight: per, Total: total, Taps: taps, Cost: total * taps}
}

// WarnLarge reports the first light covering beyond LargeLightPixelsWarn.
func (b FrameBill) WarnLarge() (bool, int) {
	for i, n := range b.PerLight {
		if n > LargeLightPixelsWarn {
			return true, i
		}
	}
	return false, -1
}

// OverShadowLightCount reports whether n lights exceed the 60 fps count.
func OverShadowLightCount(n int) bool { return n > MaxShadowLights }

// OverPCF13Count reports whether n PCF13 lights exceed the small count.
func (s SoftShadow) OverPCF13Count(n int) bool { return s.Mode == SoftPCF13 && n > MaxPCF13Lights }

// DowngradeOOM steps one grade down and logs the move. Nil soft or nil
// log refuses (silent downgrade is a failure). Already-None logs the
// request and reports no change.
func (s *SoftShadow) DowngradeOOM(log *[]string, reason string) bool {
	if s == nil || log == nil {
		return false
	}
	if reason == "" {
		reason = "oom"
	}
	switch s.Mode {
	case SoftPCF13:
		s.Mode = SoftPCF5
		*log = append(*log, "softshadow downgrade PCF13->PCF5: "+reason)
		return true
	case SoftPCF5:
		s.Mode = SoftNone
		*log = append(*log, "softshadow downgrade PCF5->None: "+reason)
		return true
	default:
		*log = append(*log, "softshadow downgrade requested at None: "+reason+" (no change)")
		return false
	}
}

// AutoDowngradeForLights downgrades when light counts exceed budget:
// PCF13 beyond MaxPCF13Lights, or any grade beyond MaxShadowLights.
// Returns true when a step happened (always with a log line).
func (s *SoftShadow) AutoDowngradeForLights(log *[]string, nLights int) bool {
	if s == nil || log == nil {
		return false
	}
	if s.Mode == SoftPCF13 && nLights > MaxPCF13Lights {
		return s.DowngradeOOM(log, "pcf13 over budget")
	}
	if nLights > MaxShadowLights && s.Mode != SoftNone {
		return s.DowngradeOOM(log, "too many shadow lights")
	}
	return false
}

// DowngradeOnAllocFail downgrades when err is OutOfMemory. Other codes
// and nil errors never downgrade.
func (s *SoftShadow) DowngradeOnAllocFail(log *[]string, err error) bool {
	if err == nil || core.CodeOf(err) != core.CodeOutOfMemory {
		return false
	}
	return s.DowngradeOOM(log, err.Error())
}

// applySoftShared lights every pixel with soft taps into a fresh image.
func (s SoftShadow) applySoftShared(src *Image, lights []Light, night core.Color) (*Image, error) {
	const op = "light.SoftShadow.Apply"
	if err := src.check(op); err != nil {
		return nil, err
	}
	if len(lights) > MaxLights {
		return nil, core.InvalidArg(op, "lights")
	}
	out := make([]core.Color, len(src.Pix))
	lp := make([]Layer, len(src.Layers))
	copy(lp, src.Layers)
	for y := 0; y < src.H; y++ {
		for x := 0; x < src.W; x++ {
			i := y*src.W + x
			p := core.V2(src.Origin.X+float64(x)*src.Step, src.Origin.Y+float64(y)*src.Step)
			out[i] = s.softLitShared(src.Pix[i], p, src.Layers[i], lights, night)
		}
	}
	return &Image{W: src.W, H: src.H, Origin: src.Origin, Step: src.Step, Pix: out, Layers: lp}, nil
}

// Apply lights src with soft taps and returns the frame bill. src never
// mutated. Invalid soft or light count is InvalidArg, nil src
// InvalidArg, corrupt src BadData.
func (s SoftShadow) Apply(src *Image, lights []Light, night core.Color) (*Image, FrameBill, error) {
	if err := s.Validate(); err != nil {
		return nil, FrameBill{}, err
	}
	img, err := s.applySoftShared(src, lights, night)
	if err != nil {
		return nil, FrameBill{}, err
	}
	return img, s.Bill(src, lights), nil
}

// ApplyCPU is the explicit CPU path (same as Apply).
func (s SoftShadow) ApplyCPU(src *Image, lights []Light, night core.Color) (*Image, FrameBill, error) {
	return s.Apply(src, lights, night)
}

// ApplyGPU is the GPU mirror path: same taps, identical pixels and bill.
func (s SoftShadow) ApplyGPU(src *Image, lights []Light, night core.Color) (*Image, FrameBill, error) {
	return s.Apply(src, lights, night)
}

// LitStats counts lit pixels and their mean gain. Gain is lit luminance
// minus the night-only baseline; lit means gain at or above MinLitGain.
// Bad inputs read 0, 0.
func LitStats(src, lit *Image, night core.Color) (int, float64) {
	if src == nil || lit == nil || !src.valid() || !lit.valid() {
		return 0, 0
	}
	if src.W != lit.W || src.H != lit.H {
		return 0, 0
	}
	if !validColor(night) {
		return 0, 0
	}
	fn := ClampNightFloor(night)
	count := 0
	sum := 0.0
	for i := range src.Pix {
		b := src.Pix[i]
		g := lit.Pix[i]
		if !finiteColor(b) || !finiteColor(g) {
			continue
		}
		base := (sanitize01(b.R)*sanitize01(fn.R) + sanitize01(b.G)*sanitize01(fn.G) + sanitize01(b.B)*sanitize01(fn.B)) / 3
		got := (sanitize01(g.R) + sanitize01(g.G) + sanitize01(g.B)) / 3
		gain := got - base
		if gain >= MinLitGain {
			count++
			sum += gain
		}
	}
	if count == 0 {
		return 0, 0
	}
	return count, sum / float64(count)
}

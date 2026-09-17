package particle

import (
	"math"

	"github.com/energye/gpui/game/core"
	"github.com/energye/gpui/game/sprite"
)

const (
	// MaxParticlesCap bounds one emitter budget (D thousand plus headroom).
	MaxParticlesCap = 8192
	// MaxSubPerDeath bounds children per death (one level, no recursion).
	MaxSubPerDeath = 8
	// MaxRate bounds spawns per second.
	MaxRate = 20000
	// MaxSpeed bounds sampled speed in world units per second.
	MaxSpeed = 5000
)

// ShapeKind names the spawn-position rule.
type ShapeKind int

const (
	// ShapePoint spawns at Origin.
	ShapePoint ShapeKind = 0
	// ShapeCone spawns at Origin with spread around Dir.
	ShapeCone ShapeKind = 1
	// ShapeBox samples uniformly in [-Extents, +Extents].
	ShapeBox ShapeKind = 2
	// ShapeRing samples uniformly in the annulus [Inner, Outer].
	ShapeRing ShapeKind = 3
)

// String returns the stable log name of k.
func (k ShapeKind) String() string {
	switch k {
	case ShapePoint:
		return "point"
	case ShapeCone:
		return "cone"
	case ShapeBox:
		return "box"
	case ShapeRing:
		return "ring"
	default:
		return "unknown"
	}
}

// ParseShape maps "point"/"cone"/"box"/"ring" to a kind, else ShapePoint
// with ok=false (never guessed).
func ParseShape(s string) (ShapeKind, bool) {
	switch s {
	case "point":
		return ShapePoint, true
	case "cone":
		return ShapeCone, true
	case "box":
		return ShapeBox, true
	case "ring":
		return ShapeRing, true
	default:
		return ShapePoint, false
	}
}

func finiteFloat(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

func finiteVec(v core.Vec2) bool { return finiteFloat(v.X) && finiteFloat(v.Y) }

func finiteColor(c core.Color) bool {
	return finiteFloat(c.R) && finiteFloat(c.G) && finiteFloat(c.B) && finiteFloat(c.A)
}

// Shape is one spawn rule: Kind picks the position sample, Dir picks the
// velocity direction (cone adds spread, ring uses radial instead).
// Angle is the cone half-angle in radians, Extents the box half size,
// Inner/Outer the ring radii. Only core numbers are used.
type Shape struct {
	Kind    ShapeKind
	Dir     core.Vec2
	Angle   float64
	Extents core.Vec2
	Inner   float64
	Outer   float64
}

// PointShape builds a point emitter in direction dir. Dir must be finite;
// zero is allowed and yields zero velocity.
func PointShape(dir core.Vec2) (Shape, error) {
	if !finiteVec(dir) {
		return Shape{}, core.InvalidArg("particle.PointShape", "dir")
	}
	return Shape{Kind: ShapePoint, Dir: dir}, nil
}

// ConeShape builds a cone emitter: Dir is the center axis (finite,
// non-zero), Angle the half-spread in [0, pi]. Bad input is InvalidArg.
func ConeShape(dir core.Vec2, angle float64) (Shape, error) {
	if !finiteVec(dir) || dir.IsZero() {
		return Shape{}, core.InvalidArg("particle.ConeShape", "dir")
	}
	if !finiteFloat(angle) || angle < 0 || angle > math.Pi {
		return Shape{}, core.InvalidArg("particle.ConeShape", "angle")
	}
	return Shape{Kind: ShapeCone, Dir: dir, Angle: angle}, nil
}

// BoxShape builds a box emitter: Dir is the velocity direction (finite),
// Extents the half size per axis (finite, >= 0). Bad input is InvalidArg.
func BoxShape(dir, extents core.Vec2) (Shape, error) {
	if !finiteVec(dir) {
		return Shape{}, core.InvalidArg("particle.BoxShape", "dir")
	}
	if !finiteVec(extents) || extents.X < 0 || extents.Y < 0 {
		return Shape{}, core.InvalidArg("particle.BoxShape", "extents")
	}
	return Shape{Kind: ShapeBox, Dir: dir, Extents: extents}, nil
}

// RingShape builds a ring emitter: radii must be finite, >= 0, Inner <=
// Outer. Direction is radial outward. Bad input is InvalidArg.
func RingShape(inner, outer float64) (Shape, error) {
	if !finiteFloat(inner) || !finiteFloat(outer) || inner < 0 || outer < 0 || inner > outer {
		return Shape{}, core.InvalidArg("particle.RingShape", "radii")
	}
	return Shape{Kind: ShapeRing, Inner: inner, Outer: outer}, nil
}

// Validate reports whether s is a constructor-built shape.
func (s Shape) Validate() error {
	const op = "particle.Shape.Validate"
	switch s.Kind {
	case ShapePoint:
		if !finiteVec(s.Dir) {
			return core.InvalidArg(op, "dir")
		}
		return nil
	case ShapeCone:
		if !finiteVec(s.Dir) || s.Dir.IsZero() {
			return core.InvalidArg(op, "dir")
		}
		if !finiteFloat(s.Angle) || s.Angle < 0 || s.Angle > math.Pi {
			return core.InvalidArg(op, "angle")
		}
		return nil
	case ShapeBox:
		if !finiteVec(s.Dir) {
			return core.InvalidArg(op, "dir")
		}
		if !finiteVec(s.Extents) || s.Extents.X < 0 || s.Extents.Y < 0 {
			return core.InvalidArg(op, "extents")
		}
		return nil
	case ShapeRing:
		if !finiteFloat(s.Inner) || !finiteFloat(s.Outer) || s.Inner < 0 || s.Outer < 0 || s.Inner > s.Outer {
			return core.InvalidArg(op, "radii")
		}
		return nil
	default:
		return core.InvalidArg(op, "kind")
	}
}

// sample returns the position offset and unit velocity direction for one
// birth. The generator is consumed in a fixed order per kind so the same
// seed replays the same sequence.
func (s Shape) sample(r *core.Rand) (offset, dir core.Vec2) {
	if r == nil {
		return core.Vec2{}, core.V2(0, -1)
	}
	switch s.Kind {
	case ShapeCone:
		base := s.Dir.Atan2()
		jitter := (r.Float64()*2 - 1) * s.Angle
		a := base + jitter
		return core.Vec2{}, core.V2(math.Cos(a), math.Sin(a))
	case ShapeBox:
		offset = core.V2((r.Float64()*2-1)*s.Extents.X, (r.Float64()*2-1)*s.Extents.Y)
		if s.Dir.IsZero() {
			return offset, core.Vec2{}
		}
		return offset, s.Dir.Normalize()
	case ShapeRing:
		theta := r.Float64() * 2 * math.Pi
		radius := s.Inner
		if s.Outer > s.Inner {
			radius = s.Inner + r.Float64()*(s.Outer-s.Inner)
		}
		offset = core.V2(math.Cos(theta)*radius, math.Sin(theta)*radius)
		if radius == 0 {
			return offset, core.V2(math.Cos(theta), math.Sin(theta))
		}
		return offset, offset.Normalize()
	default:
		if s.Dir.IsZero() {
			return core.Vec2{}, core.Vec2{}
		}
		return core.Vec2{}, s.Dir.Normalize()
	}
}

// Turbulence is a deterministic drift: Strength in world units per second
// squared, Scale in radians per second. Zero strength disables it.
type Turbulence struct {
	Strength float64
	Scale    float64
}

// NoTurbulence returns the disabled drift.
func NoTurbulence() Turbulence { return Turbulence{} }

// NewTurbulence builds a drift: Strength must be finite and >= 0, Scale
// finite. Bad input is InvalidArg.
func NewTurbulence(strength, scale float64) (Turbulence, error) {
	if !finiteFloat(strength) || strength < 0 {
		return Turbulence{}, core.InvalidArg("particle.NewTurbulence", "strength")
	}
	if !finiteFloat(scale) {
		return Turbulence{}, core.InvalidArg("particle.NewTurbulence", "scale")
	}
	return Turbulence{Strength: strength, Scale: scale}, nil
}

// Vec returns the acceleration at (seed, ageSec): Strength-scaled sin/cos
// so the same seed replays the same drift without per-tick randomness.
func (t Turbulence) Vec(seed, ageSec float64) core.Vec2 {
	if t.Strength == 0 || !finiteFloat(seed) || !finiteFloat(ageSec) {
		return core.Vec2{}
	}
	phase := seed * 2 * math.Pi
	return core.V2(
		t.Strength*math.Sin(t.Scale*ageSec+phase),
		t.Strength*math.Cos(t.Scale*ageSec*0.9+phase*1.3),
	)
}

// Sub is the one-level child rule: each non-child death spawns Count
// children with speeds in [SpeedMin, SpeedMax] and lives in
// [LifeMin, LifeMax]. Children inherit the death spot plus a quarter of
// the parent velocity and never spawn further.
type Sub struct {
	Count    int
	SpeedMin float64
	SpeedMax float64
	LifeMin  core.Duration
	LifeMax  core.Duration
}

// NoSub returns the disabled child rule.
func NoSub() Sub { return Sub{} }

// NewSub builds a child rule: Count in [0, MaxSubPerDeath]; Count 0
// ignores the remaining fields. Positive counts need finite speeds with
// Min <= Max within MaxSpeed and positive lives with Min <= Max.
func NewSub(count int, speedMin, speedMax float64, lifeMin, lifeMax core.Duration) (Sub, error) {
	const op = "particle.NewSub"
	if count < 0 || count > MaxSubPerDeath {
		return Sub{}, core.InvalidArg(op, "count")
	}
	if count == 0 {
		return Sub{}, nil
	}
	if !finiteFloat(speedMin) || !finiteFloat(speedMax) || speedMin < 0 || speedMax < 0 || speedMin > speedMax || speedMax > MaxSpeed {
		return Sub{}, core.InvalidArg(op, "speed")
	}
	if lifeMin <= 0 || lifeMax <= 0 || lifeMin > lifeMax {
		return Sub{}, core.InvalidArg(op, "life")
	}
	return Sub{Count: count, SpeedMin: speedMin, SpeedMax: speedMax, LifeMin: lifeMin, LifeMax: lifeMax}, nil
}

// EmitterConfig is the frozen spawn recipe: Origin is the spawn center,
// Rate births per second, Max the live budget, Speed/Life the per-birth
// ranges, Gravity the constant acceleration, Start/End the color ramp,
// Shape the position rule, Turbulence the drift, Sub the child rule.
type EmitterConfig struct {
	Origin     core.Vec2
	Rate       float64
	Max        int
	SpeedMin   float64
	SpeedMax   float64
	LifeMin    core.Duration
	LifeMax    core.Duration
	Gravity    core.Vec2
	Start      core.Color
	End        core.Color
	Shape      Shape
	Turbulence Turbulence
	Sub        Sub
}

// Validate checks the full recipe: Max beyond MaxParticlesCap is
// OutOfMemory, everything else bad is InvalidArg.
func (c EmitterConfig) Validate() error {
	const op = "particle.EmitterConfig.Validate"
	if !finiteVec(c.Origin) {
		return core.InvalidArg(op, "origin")
	}
	if !finiteFloat(c.Rate) || c.Rate < 0 || c.Rate > MaxRate {
		return core.InvalidArg(op, "rate")
	}
	if c.Max <= 0 {
		return core.InvalidArg(op, "max")
	}
	if c.Max > MaxParticlesCap {
		return core.OutOfMemory(op, "max")
	}
	if !finiteFloat(c.SpeedMin) || !finiteFloat(c.SpeedMax) || c.SpeedMin < 0 || c.SpeedMax < 0 || c.SpeedMin > c.SpeedMax || c.SpeedMax > MaxSpeed {
		return core.InvalidArg(op, "speed")
	}
	if c.LifeMin <= 0 || c.LifeMax <= 0 || c.LifeMin > c.LifeMax {
		return core.InvalidArg(op, "life")
	}
	if !finiteVec(c.Gravity) {
		return core.InvalidArg(op, "gravity")
	}
	if !finiteColor(c.Start) || !finiteColor(c.End) {
		return core.InvalidArg(op, "color")
	}
	if err := c.Shape.Validate(); err != nil {
		return err
	}
	if !finiteFloat(c.Turbulence.Strength) || c.Turbulence.Strength < 0 {
		return core.InvalidArg(op, "turbulence")
	}
	if !finiteFloat(c.Turbulence.Scale) {
		return core.InvalidArg(op, "turbulence")
	}
	if c.Sub.Count < 0 || c.Sub.Count > MaxSubPerDeath {
		return core.InvalidArg(op, "sub")
	}
	if c.Sub.Count > 0 {
		if _, err := NewSub(c.Sub.Count, c.Sub.SpeedMin, c.Sub.SpeedMax, c.Sub.LifeMin, c.Sub.LifeMax); err != nil {
			return err
		}
	}
	return nil
}

// Emitter owns the live set plus the seeded generator. The same config
// with the same seed replays the same births. Not safe for concurrent use.
type Emitter struct {
	cfg      EmitterConfig
	rand     *core.Rand
	live     []Particle
	acc      float64
	spawned  int
	died     int
	children int
}

// NewEmitter builds an emitter from cfg with the replay seed. Bad recipes
// return InvalidArg (Max over cap is OutOfMemory) and nil.
func NewEmitter(cfg EmitterConfig, seed uint64) (*Emitter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Emitter{cfg: cfg, rand: core.NewRand(seed)}, nil
}

// Config returns a copy of the spawn recipe.
func (e *Emitter) Config() EmitterConfig {
	if e == nil {
		return EmitterConfig{}
	}
	return e.cfg
}

// Alive returns the live particle count.
func (e *Emitter) Alive() int {
	if e == nil {
		return 0
	}
	return len(e.live)
}

// Spawned returns total births (rate plus children).
func (e *Emitter) Spawned() int {
	if e == nil {
		return 0
	}
	return e.spawned
}

// Died returns total deaths.
func (e *Emitter) Died() int {
	if e == nil {
		return 0
	}
	return e.died
}

// ChildSpawned returns total child births.
func (e *Emitter) ChildSpawned() int {
	if e == nil {
		return 0
	}
	return e.children
}

// Particles returns a fresh copy of the live set; writing it cannot alias
// the emitter.
func (e *Emitter) Particles() []Particle {
	if e == nil || len(e.live) == 0 {
		return nil
	}
	out := make([]Particle, len(e.live))
	copy(out, e.live)
	return out
}

// Clear drops every live particle and resets the accumulator plus the
// counters. Config and generator sequence are kept.
func (e *Emitter) Clear() {
	if e == nil {
		return
	}
	e.live = nil
	e.acc = 0
	e.spawned = 0
	e.died = 0
	e.children = 0
}

func (e *Emitter) ensureRand() {
	if e.rand == nil {
		e.rand = core.NewRand(0)
	}
}

func sampleLife(r *core.Rand, min, max core.Duration) core.Duration {
	if min >= max {
		return min
	}
	span := int64(max - min)
	off := int64(r.Float64() * float64(span))
	return min + core.Milliseconds(off)
}

// spawnOne samples one parent birth at the emitter origin.
func (e *Emitter) spawnOne() Particle {
	offset, dir := e.cfg.Shape.sample(e.rand)
	speed := e.rand.RangeFloat(e.cfg.SpeedMin, e.cfg.SpeedMax)
	life := sampleLife(e.rand, e.cfg.LifeMin, e.cfg.LifeMax)
	seed := e.rand.Float64()
	return Particle{
		Pos:   e.cfg.Origin.Add(offset),
		Vel:   dir.Mul(speed),
		Age:   0,
		Life:  life,
		Seed:  seed,
		Child: false,
	}
}

// spawnChild samples one child at the death spot with a quarter of the
// parent velocity plus a fresh random push.
func (e *Emitter) spawnChild(at core.Vec2, parentVel core.Vec2) Particle {
	theta := e.rand.Float64() * 2 * math.Pi
	push := core.V2(math.Cos(theta), math.Sin(theta)).Mul(e.rand.RangeFloat(e.cfg.Sub.SpeedMin, e.cfg.Sub.SpeedMax))
	life := sampleLife(e.rand, e.cfg.Sub.LifeMin, e.cfg.Sub.LifeMax)
	return Particle{
		Pos:   at,
		Vel:   parentVel.Mul(0.25).Add(push),
		Age:   0,
		Life:  life,
		Seed:  e.rand.Float64(),
		Child: true,
	}
}

// Spawn births n parents now, capped by the remaining budget. Excess
// drops quietly. Negative n is InvalidArg; zero is a no-op.
func (e *Emitter) Spawn(n int) (int, error) {
	const op = "particle.Emitter.Spawn"
	if e == nil {
		return 0, core.InvalidArg(op, "emitter")
	}
	if n < 0 {
		return 0, core.InvalidArg(op, "n")
	}
	if n == 0 {
		return 0, nil
	}
	e.ensureRand()
	room := e.cfg.Max - len(e.live)
	if room <= 0 {
		return 0, nil
	}
	if n > room {
		n = room
	}
	for i := 0; i < n; i++ {
		e.live = append(e.live, e.spawnOne())
	}
	e.spawned += n
	return n, nil
}

// Update advances by dt: integrates existing particles, drops the dead,
// births children for non-child deaths, then births the rate share.
// Newborns start at Age 0 in the same tick. dt at or below zero is a
// no-op returning 0, 0. The live set never exceeds Max.
func (e *Emitter) Update(dt core.Duration) (spawned, died int) {
	if e == nil || dt <= 0 {
		return 0, 0
	}
	e.ensureRand()
	dtSec := dt.Seconds()
	if dtSec <= 0 || !finiteFloat(dtSec) {
		return 0, 0
	}
	kept := e.live[:0]
	var deaths []Particle
	// Childless emitters (the window hot loop) never read deaths, so skip
	// the per-frame deaths slice and just compact plus count the dead.
	if e.cfg.Sub.Count <= 0 {
		for i := range e.live {
			p := &e.live[i]
			p.step(dtSec, e.cfg.Gravity, e.cfg.Turbulence)
			if p.Alive() {
				kept = append(kept, *p)
			} else {
				died++
			}
		}
		e.live = kept
		e.died += died
	} else {
		for i := range e.live {
			p := &e.live[i]
			p.step(dtSec, e.cfg.Gravity, e.cfg.Turbulence)
			if p.Alive() {
				kept = append(kept, *p)
			} else {
				deaths = append(deaths, *p)
			}
		}
		e.live = kept
		e.died += len(deaths)
		died = len(deaths)
	}
	// Children first so a death-heavy tick still shows its puff.
	if e.cfg.Sub.Count > 0 {
		for _, d := range deaths {
			if d.Child {
				continue
			}
			for k := 0; k < e.cfg.Sub.Count; k++ {
				if len(e.live) >= e.cfg.Max {
					break
				}
				e.live = append(e.live, e.spawnChild(d.Pos, d.Vel))
				e.spawned++
				e.children++
				spawned++
			}
		}
	}
	if e.cfg.Rate > 0 {
		e.acc += e.cfg.Rate * dtSec
		n := int(math.Floor(e.acc))
		e.acc -= float64(n)
		if n > 0 {
			room := e.cfg.Max - len(e.live)
			if room > 0 {
				if n > room {
					n = room
				}
				for i := 0; i < n; i++ {
					e.live = append(e.live, e.spawnOne())
				}
				e.spawned += n
				spawned += n
			}
		}
		// Clamp tiny residue from float error.
		if e.acc < 0 {
			e.acc = 0
		}
		if e.acc >= 1 {
			e.acc = e.acc - math.Floor(e.acc)
		}
	}
	return spawned, died
}

// ColorOf lerps the emitter ramp by the particle age fraction.
func (e *Emitter) ColorOf(p Particle) core.Color {
	if e == nil {
		return core.Color{}
	}
	return p.ColorAt(e.cfg.Start, e.cfg.End)
}

// AppendToBatch appends one sprite per live particle centered at Pos with
// size by size: Src is the caller atlas block, Opacity the current alpha.
// Alpha at or below zero is skipped (fully transparent, saves a draw);
// non-finite positions are skipped so a burst never poisons the batch.
// Bad arguments return InvalidArg and append nothing.
func (e *Emitter) AppendToBatch(b *sprite.Batch, image core.AssetID, src core.Rect, size float64) (int, error) {
	const op = "particle.Emitter.AppendToBatch"
	if e == nil {
		return 0, core.InvalidArg(op, "emitter")
	}
	if b == nil {
		return 0, core.InvalidArg(op, "batch")
	}
	if image.Empty() {
		return 0, core.InvalidArg(op, "image")
	}
	if !finiteFloat(src.X) || !finiteFloat(src.Y) || !finiteFloat(src.W) || !finiteFloat(src.H) {
		return 0, core.InvalidArg(op, "src")
	}
	if !finiteFloat(size) || size <= 0 {
		return 0, core.InvalidArg(op, "size")
	}
	n := 0
	for _, p := range e.live {
		if !p.Alive() || !finiteVec(p.Pos) || !finiteVec(p.Vel) {
			continue
		}
		c := e.ColorOf(p)
		if c.A <= 0 {
			continue
		}
		dst := core.NewRect(p.Pos.X-size/2, p.Pos.Y-size/2, size, size)
		s, err := sprite.NewSprite(image, src, dst, c.A)
		if err != nil {
			continue
		}
		if ok, err := b.Add(s); err != nil || !ok {
			continue
		}
		n++
	}
	return n, nil
}

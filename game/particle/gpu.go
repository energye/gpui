package particle

import (
	"github.com/energye/gpui/game/core"
	"github.com/energye/gpui/game/sprite"
)

// MaxPoolTrailsCap bounds one pool: one trail per particle at most, same
// budget as the 5.1 emitter so the pool never promises more than the CPU
// math can birth.
const MaxPoolTrailsCap = MaxParticlesCap

// GPUPool is the 5.2 bulk particle pool (main reference Godot
// GPUParticles2D): positions come from the frozen 5.1 Emitter math
// (Spawn/Update semantics unchanged, same object), while every live
// particle owns one Trail ring recording its trajectory. Trails are keyed
// by the particle birth Seed: the same config with the same seed replays
// the same tracks. Two births sharing one Seed (53-bit space) share one
// trail by design, never crash. Drawing reuses the existing sprite
// Batch/DrawAtlas; only core numbers and core error codes are used.
// Not safe for concurrent use.
type GPUPool struct {
	emit     *Emitter
	trailCfg TrailConfig
	trails   map[float64]*Trail
	points   int
}

// NewGPUPool builds a pool from the 5.1 emitter recipe plus the trail
// recipe. Bad emitter recipes report their Emitter error, bad trail
// recipes report InvalidArg; both return nil.
func NewGPUPool(cfg EmitterConfig, seed uint64, trail TrailConfig) (*GPUPool, error) {
	e, err := NewEmitter(cfg, seed)
	if err != nil {
		return nil, err
	}
	if err := trail.Validate(); err != nil {
		return nil, err
	}
	return &GPUPool{emit: e, trailCfg: trail, trails: make(map[float64]*Trail)}, nil
}

// Config returns a copy of the emitter recipe.
func (p *GPUPool) Config() EmitterConfig {
	if p == nil || p.emit == nil {
		return EmitterConfig{}
	}
	return p.emit.Config()
}

// TrailConfig returns a copy of the trail recipe.
func (p *GPUPool) TrailConfig() TrailConfig {
	if p == nil {
		return TrailConfig{}
	}
	return p.trailCfg
}

// Alive returns the live particle count.
func (p *GPUPool) Alive() int {
	if p == nil || p.emit == nil {
		return 0
	}
	return p.emit.Alive()
}

// Spawned returns total births (rate plus children).
func (p *GPUPool) Spawned() int {
	if p == nil || p.emit == nil {
		return 0
	}
	return p.emit.Spawned()
}

// Died returns total deaths.
func (p *GPUPool) Died() int {
	if p == nil || p.emit == nil {
		return 0
	}
	return p.emit.Died()
}

// ChildSpawned returns total child births.
func (p *GPUPool) ChildSpawned() int {
	if p == nil || p.emit == nil {
		return 0
	}
	return p.emit.ChildSpawned()
}

// Trails returns the live trail count (one per live particle, shared on
// Seed collision).
func (p *GPUPool) Trails() int {
	if p == nil {
		return 0
	}
	return len(p.trails)
}

// TotalPoints returns the stored trajectory point count across all trails.
func (p *GPUPool) TotalPoints() int {
	if p == nil {
		return 0
	}
	return p.points
}

// Particles returns a fresh copy of the live set; writing it cannot alias
// the pool.
func (p *GPUPool) Particles() []Particle {
	if p == nil || p.emit == nil {
		return nil
	}
	return p.emit.Particles()
}

// ColorOf lerps the emitter ramp by the particle age fraction.
func (p *GPUPool) ColorOf(pt Particle) core.Color {
	if p == nil || p.emit == nil {
		return core.Color{}
	}
	return p.emit.ColorOf(pt)
}

// TrailOf returns a fresh copy of the trail for seed, or nil when the
// seed has no live trail. Writing it cannot alias the pool.
func (p *GPUPool) TrailOf(seed float64) *Trail {
	if p == nil {
		return nil
	}
	t, ok := p.trails[seed]
	if !ok || t == nil {
		return nil
	}
	cp := &Trail{cfg: t.cfg, pts: append([]core.Vec2(nil), t.pts...)}
	if len(t.pts) == 0 {
		cp.pts = nil
	}
	return cp
}

// Clear drops every live particle and every trail, resetting the counters.
// Recipes are kept.
func (p *GPUPool) Clear() {
	if p == nil {
		return
	}
	if p.emit != nil {
		p.emit.Clear()
	}
	p.trails = make(map[float64]*Trail)
	p.points = 0
}

// Spawn births n parents now, capped by the remaining budget. Excess drops
// quietly. Negative n is InvalidArg; zero is a no-op. Nil is InvalidArg.
func (p *GPUPool) Spawn(n int) (int, error) {
	const op = "particle.GPUPool.Spawn"
	if p == nil || p.emit == nil {
		return 0, core.InvalidArg(op, "pool")
	}
	got, err := p.emit.Spawn(n)
	if err != nil {
		return got, err
	}
	p.syncTrails()
	return got, nil
}

// Update advances by dt with the frozen 5.1 semantics, then records one
// trajectory point per live particle and drops dead tracks. dt at or below
// zero is a quiet no-op returning 0, 0. Nil is 0, 0.
func (p *GPUPool) Update(dt core.Duration) (spawned, died int) {
	if p == nil || p.emit == nil {
		return 0, 0
	}
	spawned, died = p.emit.Update(dt)
	p.syncTrails()
	return spawned, died
}

// syncTrails rebuilds the seed map from the live set: survivors keep their
// ring and gain one point, newborns start a ring, the dead leave the map.
// Non-finite positions keep the old ring (never poison the track).
func (p *GPUPool) syncTrails() {
	if p == nil || p.emit == nil {
		return
	}
	live := p.emit.Particles()
	kept := make(map[float64]*Trail, len(live))
	points := 0
	for _, pt := range live {
		t, ok := p.trails[pt.Seed]
		if !ok || t == nil {
			t = &Trail{cfg: p.trailCfg}
		}
		if finiteVec(pt.Pos) {
			_ = t.Push(pt.Pos)
		}
		kept[pt.Seed] = t
		points += t.Len()
	}
	p.trails = kept
	p.points = points
}

// AppendToBatch appends one square sprite per stored trail point with the
// WidthAt size and the ColorAt alpha, mirroring the Emitter skip rules.
// Bad arguments return InvalidArg and append nothing.
func (p *GPUPool) AppendToBatch(b *sprite.Batch, image core.AssetID, src core.Rect) (int, error) {
	const op = "particle.GPUPool.AppendToBatch"
	if p == nil || p.emit == nil {
		return 0, core.InvalidArg(op, "pool")
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
	n := 0
	seen := make(map[float64]bool)
	for _, pt := range p.emit.Particles() {
		if seen[pt.Seed] {
			continue
		}
		seen[pt.Seed] = true
		t := p.trails[pt.Seed]
		if t == nil {
			continue
		}
		m, err := t.AppendToBatch(b, image, src)
		if err != nil {
			continue
		}
		n += m
	}
	return n, nil
}

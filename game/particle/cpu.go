package particle

import (
	"math"

	"github.com/energye/gpui/game/core"
)

// Particle is one live CPU particle: position and velocity in world units,
// age and life in integer milliseconds, a replay seed in [0, 1), and the
// child flag marking sub-emitter births. Only core numbers are used.
type Particle struct {
	Pos   core.Vec2
	Vel   core.Vec2
	Age   core.Duration
	Life  core.Duration
	Seed  float64
	Child bool
}

// AgeFrac returns Age/Life clamped to [0, 1]. Non-positive Life parks at 1.
func (p Particle) AgeFrac() float64 {
	if p.Life <= 0 {
		return 1
	}
	f := float64(p.Age) / float64(p.Life)
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

// Alive reports whether Age is strictly below Life with a positive Life.
func (p Particle) Alive() bool {
	return p.Life > 0 && p.Age < p.Life
}

// ColorAt lerps start to end by AgeFrac. NaN frac never leaks: AgeFrac is
// already clamped, so the result stays finite for finite endpoints.
func (p Particle) ColorAt(start, end core.Color) core.Color {
	return start.Lerp(end, p.AgeFrac())
}

// step advances one particle by dtSec with gravity plus deterministic
// turbulence. Age always advances; callers drop the dead afterwards.
func (p *Particle) step(dtSec float64, gravity core.Vec2, turb Turbulence) {
	if p == nil || dtSec <= 0 || math.IsNaN(dtSec) || math.IsInf(dtSec, 0) {
		return
	}
	ageSec := float64(p.Age) / 1000
	tv := turb.Vec(p.Seed, ageSec)
	ax := gravity.X + tv.X
	ay := gravity.Y + tv.Y
	if math.IsNaN(ax) || math.IsInf(ax, 0) || math.IsNaN(ay) || math.IsInf(ay, 0) {
		return
	}
	p.Vel = core.V2(p.Vel.X+ax*dtSec, p.Vel.Y+ay*dtSec)
	p.Pos = core.V2(p.Pos.X+p.Vel.X*dtSec, p.Pos.Y+p.Vel.Y*dtSec)
	p.Age += core.SecondsFloat(dtSec)
}

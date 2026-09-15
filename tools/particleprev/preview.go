package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"

	"github.com/energye/gpui/game/core"
	"github.com/energye/gpui/game/particle"
)

// Effect is one opened particle effect: the filed recipe echo plus one
// fixed probe replay through the real engine. Missing files open as a
// clean placeholder (HasFile false, nil error); torn files set FileErr
// and return the core error with a zeroed effect. Nothing here panics.
type Effect struct {
	Dir     string
	Name    string
	HasFile bool

	Rate      float64
	Max       int
	SpeedMin  float64
	SpeedMax  float64
	LifeMinMs int64
	LifeMaxMs int64
	ShapeKind string
	Seed      uint64

	ProbeSpawn    int
	ProbeUpdateMs int64
	Spawned       int
	Alive         int
	Child         int

	Notes   []string
	FileErr string

	cfg particle.EmitterConfig
}

// Config returns the engine recipe this effect replays.
func (e *Effect) Config() particle.EmitterConfig {
	if e == nil {
		return particle.EmitterConfig{}
	}
	return e.cfg
}

// BuildEmitter builds a fresh engine emitter from the filed recipe with
// the filed seed. The window and the tests replay through this so the
// numbers always come from game/particle itself.
func (e *Effect) BuildEmitter() (*particle.Emitter, error) {
	if e == nil || !e.HasFile {
		return nil, core.InvalidArg("particleprev.BuildEmitter", "effect")
	}
	return particle.NewEmitter(e.cfg, e.Seed)
}

// ReplayEffect runs the filed probe (Spawn then Update) on a fresh
// emitter and returns spawned/alive/child straight from the engine.
func ReplayEffect(e *Effect) (spawned, alive, child int, err error) {
	em, err := e.BuildEmitter()
	if err != nil {
		return 0, 0, 0, err
	}
	if _, err := em.Spawn(e.ProbeSpawn); err != nil {
		return 0, 0, 0, err
	}
	em.Update(core.Milliseconds(e.ProbeUpdateMs))
	return em.Spawned(), em.Alive(), em.ChildSpawned(), nil
}

// VerifyReplay reports whether eff carries exactly the engine replay
// numbers. Zero difference passes; anything else is BadData.
func VerifyReplay(eff Effect) error {
	const op = "particleprev.VerifyReplay"
	if !eff.HasFile {
		return core.InvalidArg(op, "effect")
	}
	s, a, c, err := ReplayEffect(&eff)
	if err != nil {
		return err
	}
	if s != eff.Spawned || a != eff.Alive || c != eff.Child {
		return core.BadData(op, "replay-counts")
	}
	return nil
}

type shapeFile struct {
	Kind     string    `json:"kind"`
	Dir      []float64 `json:"dir"`
	AngleDeg float64   `json:"angle_deg"`
	Extents  []float64 `json:"extents"`
	Inner    float64   `json:"ring_inner"`
	Outer    float64   `json:"ring_outer"`
}

type effectFile struct {
	Name      string    `json:"name"`
	Origin    []float64 `json:"origin"`
	Shape     shapeFile `json:"shape"`
	Rate      float64   `json:"rate"`
	Max       int       `json:"max"`
	Speed     []float64 `json:"speed"`
	LifeMs    []int64   `json:"life_ms"`
	Gravity   []float64 `json:"gravity"`
	Start     []float64 `json:"start"`
	End       []float64 `json:"end"`
	Strength  float64   `json:"strength"`
	Scale     float64   `json:"scale"`
	SubCount  int       `json:"sub_count"`
	SubSpeed  []float64 `json:"sub_speed"`
	SubLifeMs []int64   `json:"sub_life_ms"`
	Seed      uint64    `json:"seed"`
	Spawn     int       `json:"spawn"`
	UpdateMs  int64     `json:"update_ms"`
}

func vec2Of(v []float64) (core.Vec2, error) {
	if len(v) != 2 {
		return core.Vec2{}, core.BadData("particleprev.effect", "vec2")
	}
	return core.V2(v[0], v[1]), nil
}

func colorOf(v []float64) (core.Color, error) {
	if len(v) != 4 {
		return core.Color{}, core.BadData("particleprev.effect", "color")
	}
	return core.RGBA(v[0], v[1], v[2], v[3]), nil
}

// ParseEffect validates effect file bytes into an Effect with its probe
// already replayed. Empty input is InvalidArg, torn JSON or a bad recipe
// is BadData, an over-cap Max is OutOfMemory; all straight from the
// engine constructors, never guessed.
func ParseEffect(data []byte) (Effect, error) {
	const op = "particleprev.ParseEffect"
	if len(data) == 0 {
		return Effect{}, core.InvalidArg(op, "data")
	}
	var f effectFile
	if err := json.Unmarshal(data, &f); err != nil {
		return Effect{}, core.BadData(op, "json", err)
	}
	origin, err := vec2Of(f.Origin)
	if err != nil {
		return Effect{}, err
	}
	gravity, err := vec2Of(f.Gravity)
	if err != nil {
		return Effect{}, err
	}
	start, err := colorOf(f.Start)
	if err != nil {
		return Effect{}, err
	}
	end, err := colorOf(f.End)
	if err != nil {
		return Effect{}, err
	}
	if len(f.Speed) != 2 || len(f.LifeMs) != 2 {
		return Effect{}, core.BadData(op, "speed-life")
	}
	dir, err := vec2Of(f.Shape.Dir)
	if err != nil {
		return Effect{}, err
	}
	var shape particle.Shape
	switch f.Shape.Kind {
	case "point":
		shape, err = particle.PointShape(dir)
	case "cone":
		shape, err = particle.ConeShape(dir, f.Shape.AngleDeg*math.Pi/180)
	case "box":
		ext, verr := vec2Of(f.Shape.Extents)
		if verr != nil {
			return Effect{}, verr
		}
		shape, err = particle.BoxShape(dir, ext)
	case "ring":
		shape, err = particle.RingShape(f.Shape.Inner, f.Shape.Outer)
	default:
		return Effect{}, core.BadData(op, "shape")
	}
	if err != nil {
		return Effect{}, err
	}
	turb, err := particle.NewTurbulence(f.Strength, f.Scale)
	if err != nil {
		return Effect{}, err
	}
	var sub particle.Sub
	if f.SubCount == 0 {
		sub = particle.NoSub()
	} else {
		if len(f.SubSpeed) != 2 || len(f.SubLifeMs) != 2 {
			return Effect{}, core.BadData(op, "sub")
		}
		sub, err = particle.NewSub(f.SubCount, f.SubSpeed[0], f.SubSpeed[1],
			core.Milliseconds(f.SubLifeMs[0]), core.Milliseconds(f.SubLifeMs[1]))
		if err != nil {
			return Effect{}, err
		}
	}
	cfg := particle.EmitterConfig{
		Origin:     origin,
		Rate:       f.Rate,
		Max:        f.Max,
		SpeedMin:   f.Speed[0],
		SpeedMax:   f.Speed[1],
		LifeMin:    core.Milliseconds(f.LifeMs[0]),
		LifeMax:    core.Milliseconds(f.LifeMs[1]),
		Gravity:    gravity,
		Start:      start,
		End:        end,
		Shape:      shape,
		Turbulence: turb,
		Sub:        sub,
	}
	em, err := particle.NewEmitter(cfg, f.Seed)
	if err != nil {
		return Effect{}, err
	}
	if _, err := em.Spawn(f.Spawn); err != nil {
		return Effect{}, err
	}
	em.Update(core.Milliseconds(f.UpdateMs))
	return Effect{
		Name: f.Name, HasFile: true,
		Rate: f.Rate, Max: f.Max,
		SpeedMin: f.Speed[0], SpeedMax: f.Speed[1],
		LifeMinMs: f.LifeMs[0], LifeMaxMs: f.LifeMs[1],
		ShapeKind: f.Shape.Kind, Seed: f.Seed,
		ProbeSpawn: f.Spawn, ProbeUpdateMs: f.UpdateMs,
		Spawned: em.Spawned(), Alive: em.Alive(), Child: em.ChildSpawned(),
		cfg: cfg,
	}, nil
}

// OpenEffect opens dir/effect_<name>.json as a particle effect. A
// missing file is a clean placeholder (HasFile false, nil error) so an
// empty project still opens; a torn file sets FileErr and returns the
// core error with a zeroed effect. Empty dir paths are InvalidArg,
// missing directories are NotFound.
func OpenEffect(dir, name string) (Effect, error) {
	const op = "particleprev.OpenEffect"
	if dir == "" || name == "" {
		return Effect{}, core.InvalidArg(op, "dir-name")
	}
	fi, err := os.Stat(dir)
	if err != nil || !fi.IsDir() {
		return Effect{}, core.NotFound(op, dir)
	}
	path := filepath.Join(dir, "effect_"+name+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return Effect{
			Dir: dir, Name: name,
			Notes: []string{"effect_" + name + ".json 缺:空白特效占位"},
		}, nil
	}
	eff, perr := ParseEffect(raw)
	if perr != nil {
		return Effect{
			Dir: dir, Name: name, FileErr: perr.Error(),
			Notes: []string{"effect_" + name + ".json 坏:特效占位,看 FileErr"},
		}, perr
	}
	eff.Dir = dir
	return eff, nil
}

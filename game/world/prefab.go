package world

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/energye/gpui/game/core"
)

// Prefab and scene files (10.2, S33/W4): the level the planner laid out,
// stored as JSON, stamped into a World or opened as a live Scene in one
// go. Prefab is one reusable entity template; SceneFile (see scene.go) is
// one level on disk. Instantiate stamps a prefab under any parent; Open
// builds the whole level with every birth active.
//
// File shape frozen: version "1.0", entity names, local placements,
// comps with referenced asset ids, parent links by name with parents
// filed before their children. Only core numbers are used; no render
// types appear here. Window intent: game_world--case=open; pure file
// math, offscreen golden only (round-trip byte-identical).

// PrefabVersion is the prefab file version every Encode writes.
// Bump Minor for additive fields, Major for a breaking shape.
var PrefabVersion = core.Version{Major: 1, Minor: 0}

// Frozen budgets. A well-formed file beyond budget is OutOfMemory,
// never a guessed load. A bad value is InvalidArg via constructors
// and BadData via Parse.
const (
	// MaxPrefabNameLen caps prefab names in bytes.
	MaxPrefabNameLen = 64
	// MaxPrefabComps caps comps on one prefab.
	MaxPrefabComps = 16
	// MaxPrefabBytes caps one prefab file.
	MaxPrefabBytes = 64 << 10
)

func validPrefabName(s string) bool { return s != "" && len(s) <= MaxPrefabNameLen }

// Prefab is one reusable entity template: a name, a local placement,
// and comps in stamp order. Fields stay private so every write passes
// validation; readers use the accessors below, which copy.
type Prefab struct {
	name    string
	version core.Version
	local   Transform
	comps   []Comp
}

// NewPrefab builds a template with PrefabVersion, the given local, and
// no comps. Empty or overlong names and non-finite numbers are a core
// InvalidArg error and store nothing.
func NewPrefab(name string, local Transform) (Prefab, error) {
	if !validPrefabName(name) {
		return Prefab{}, core.InvalidArg("world.NewPrefab", "name")
	}
	if err := checkTransform("world.NewPrefab", local); err != nil {
		return Prefab{}, err
	}
	return Prefab{name: name, version: PrefabVersion, local: local}, nil
}

// Name returns the template name, or "" on a nil prefab.
func (p *Prefab) Name() string {
	if p == nil {
		return ""
	}
	return p.name
}

// Version returns the stored file version.
func (p *Prefab) Version() core.Version {
	if p == nil {
		return core.Version{}
	}
	return p.version
}

// Local returns the template placement.
func (p *Prefab) Local() Transform {
	if p == nil {
		return Transform{}
	}
	return p.local
}

// SetLocal replaces the template placement. Non-finite numbers are a
// core InvalidArg error and change nothing. Nil prefabs report
// InvalidArg.
func (p *Prefab) SetLocal(t Transform) error {
	if p == nil {
		return core.InvalidArg("world.Prefab.SetLocal", "prefab")
	}
	if err := checkTransform("world.Prefab.SetLocal", t); err != nil {
		return err
	}
	p.local = t
	return nil
}

// AddComp appends c in stamp order. An empty Kind is InvalidArg; past
// MaxPrefabComps is OutOfMemory. Failures change nothing. Nil prefabs
// report InvalidArg.
func (p *Prefab) AddComp(c Comp) error {
	if p == nil {
		return core.InvalidArg("world.Prefab.AddComp", "prefab")
	}
	if c.Kind == "" {
		return core.InvalidArg("world.Prefab.AddComp", "kind")
	}
	if len(p.comps) >= MaxPrefabComps {
		return core.OutOfMemory("world.Prefab.AddComp", "comps")
	}
	p.comps = append(p.comps, c)
	return nil
}

// Comps returns the comps in stamp order. The result is a fresh slice;
// writing it cannot change the prefab. Nil prefabs report nil.
func (p *Prefab) Comps() []Comp {
	if p == nil {
		return nil
	}
	return append([]Comp(nil), p.comps...)
}

// Equal reports whether o holds the same name, version, local, and
// comps in order. A nil prefab only equals nothing, never a value.
func (p *Prefab) Equal(o Prefab) bool {
	if p == nil {
		return false
	}
	if p.name != o.name || p.version != o.version || p.local != o.local {
		return false
	}
	if len(p.comps) != len(o.comps) {
		return false
	}
	for i := range p.comps {
		if p.comps[i] != o.comps[i] {
			return false
		}
	}
	return true
}

// Instantiate stamps the template into w under parent (NoEntity for a
// root): one Spawn, the stored local, comps in order. A missing parent
// is NotFound; a nil world is InvalidArg. A failed stamp births
// nothing: the half-born entity is removed before the error returns.
func (p *Prefab) Instantiate(w *World, parent ID) (ID, error) {
	const op = "world.Prefab.Instantiate"
	if p == nil {
		return NoEntity, core.InvalidArg(op, "prefab")
	}
	if w == nil {
		return NoEntity, core.InvalidArg(op, "world")
	}
	if err := p.checkShape(op); err != nil {
		return NoEntity, err
	}
	id, err := w.Spawn(parent)
	if err != nil {
		return NoEntity, err
	}
	if err := w.SetTransform(id, p.local); err != nil {
		_ = w.Despawn(id)
		return NoEntity, err
	}
	for _, c := range p.comps {
		if err := w.AddComp(id, c); err != nil {
			_ = w.Despawn(id)
			return NoEntity, err
		}
	}
	return id, nil
}

// checkShape validates an in-memory prefab for Encode/Instantiate:
// bad values are InvalidArg, budget overruns are OutOfMemory.
// Version is not checked here.
func (p *Prefab) checkShape(op string) error {
	if p == nil {
		return core.InvalidArg(op, "prefab")
	}
	if !validPrefabName(p.name) {
		return core.InvalidArg(op, "name")
	}
	if len(p.comps) > MaxPrefabComps {
		return core.OutOfMemory(op, "comps")
	}
	for _, c := range p.comps {
		if c.Kind == "" {
			return core.InvalidArg(op, "kind")
		}
	}
	return nil
}

// jsonTransform is the frozen local placement on disk.
type jsonTransform struct {
	Pos   [2]float64 `json:"pos"`
	Rot   float64    `json:"rot"`
	Scale [2]float64 `json:"scale"`
}

// jsonComp is the frozen comp on disk: Kind names it, Ref carries the
// asset id and may stay empty for pure-logic comps.
type jsonComp struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref"`
}

// jsonPrefab is the frozen prefab file.
type jsonPrefab struct {
	Version string         `json:"version"`
	Name    string         `json:"name"`
	Local   *jsonTransform `json:"local"`
	Comps   []jsonComp     `json:"comps"`
}

func transformToJSON(t Transform) jsonTransform {
	return jsonTransform{
		Pos:   [2]float64{t.Pos.X, t.Pos.Y},
		Rot:   t.Rot,
		Scale: [2]float64{t.Scale.X, t.Scale.Y},
	}
}

func transformFromJSON(op string, j *jsonTransform) (Transform, error) {
	if j == nil {
		return Transform{}, core.BadData(op, "local")
	}
	t := Transform{
		Pos:   core.V2(j.Pos[0], j.Pos[1]),
		Rot:   j.Rot,
		Scale: core.V2(j.Scale[0], j.Scale[1]),
	}
	if !finite(t.Pos.X) || !finite(t.Pos.Y) || !finite(t.Rot) ||
		!finite(t.Scale.X) || !finite(t.Scale.Y) {
		return Transform{}, core.BadData(op, "transform")
	}
	return t, nil
}

func compToJSON(c Comp) jsonComp { return jsonComp{Kind: c.Kind, Ref: string(c.Ref)} }

func compFromJSON(op string, j jsonComp) (Comp, error) {
	if j.Kind == "" {
		return Comp{}, core.BadData(op, "kind")
	}
	return Comp{Kind: j.Kind, Ref: core.AssetID(j.Ref)}, nil
}

// Encode renders the canonical file bytes. The prefab must already
// carry PrefabVersion; otherwise the result is a core VersionMismatch
// error. Bad shapes are InvalidArg, budget overruns are OutOfMemory.
// The input is never mutated. Empty comp lists encode as [] so a file
// round-trips byte-identical.
func (p *Prefab) Encode() ([]byte, error) {
	const op = "world.Prefab.Encode"
	if err := p.checkShape(op); err != nil {
		return nil, err
	}
	if p.version != PrefabVersion {
		return nil, core.VersionMismatch(op, p.version.String())
	}
	comps := make([]jsonComp, len(p.comps))
	for i, c := range p.comps {
		comps[i] = compToJSON(c)
	}
	local := transformToJSON(p.local)
	out, err := json.Marshal(jsonPrefab{
		Version: p.version.String(),
		Name:    p.name,
		Local:   &local,
		Comps:   comps,
	})
	if err != nil {
		return nil, core.BadData(op, "encode")
	}
	if len(out) > MaxPrefabBytes {
		return nil, core.OutOfMemory(op, "bytes")
	}
	return out, nil
}

// ParsePrefab validates file bytes into a Prefab. Empty input is
// InvalidArg, size overruns are OutOfMemory, torn JSON and bad shapes
// are BadData, foreign or newer versions are VersionMismatch. Missing
// comps default to empty.
func ParsePrefab(data []byte) (Prefab, error) {
	const op = "world.ParsePrefab"
	if len(data) == 0 {
		return Prefab{}, core.InvalidArg(op, "data")
	}
	if len(data) > MaxPrefabBytes {
		return Prefab{}, core.OutOfMemory(op, "bytes")
	}
	var raw jsonPrefab
	if err := json.Unmarshal(data, &raw); err != nil {
		return Prefab{}, core.BadData(op, "json", err)
	}
	if raw.Version == "" {
		return Prefab{}, core.BadData(op, "version")
	}
	ver, err := core.ParseVersion(raw.Version)
	if err != nil {
		return Prefab{}, core.BadData(op, "version")
	}
	if !ver.CompatibleWith(PrefabVersion) {
		return Prefab{}, core.VersionMismatch(op, ver.String())
	}
	if !validPrefabName(raw.Name) {
		return Prefab{}, core.BadData(op, "name")
	}
	local, err := transformFromJSON(op, raw.Local)
	if err != nil {
		return Prefab{}, err
	}
	if len(raw.Comps) > MaxPrefabComps {
		return Prefab{}, core.OutOfMemory(op, "comps")
	}
	comps := make([]Comp, 0, len(raw.Comps))
	for _, jc := range raw.Comps {
		c, err := compFromJSON(op, jc)
		if err != nil {
			return Prefab{}, err
		}
		comps = append(comps, c)
	}
	return Prefab{name: raw.Name, version: ver, local: local, comps: comps}, nil
}

// LoadPrefab reads path as a prefab file. Empty paths are InvalidArg,
// missing files are NotFound; the rest matches ParsePrefab.
func LoadPrefab(path string) (Prefab, error) {
	const op = "world.LoadPrefab"
	if path == "" {
		return Prefab{}, core.InvalidArg(op, "path")
	}
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return Prefab{}, core.NotFound(op, path, err)
	}
	return ParsePrefab(raw)
}

// SavePrefab writes the prefab to path, creating parent directories.
// Empty paths are InvalidArg; OS failures are NotFound with the cause;
// prefab-shape errors match Encode. Nil prefabs report InvalidArg.
func (p *Prefab) SavePrefab(path string) error {
	const op = "world.Prefab.SavePrefab"
	if path == "" {
		return core.InvalidArg(op, "path")
	}
	raw, err := p.Encode()
	if err != nil {
		return err
	}
	clean := filepath.Clean(path)
	if dir := filepath.Dir(clean); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return core.NotFound(op, path, err)
		}
	}
	if err := os.WriteFile(clean, raw, 0o644); err != nil {
		return core.NotFound(op, path, err)
	}
	return nil
}

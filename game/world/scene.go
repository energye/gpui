package world

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/energye/gpui/game/core"
)

// Scene lifecycle (10.3, S34/W4): birth, activation, sleep, destroy.
//
// Scene owns one World for entity storage; lifecycle state lives beside
// it. Spawn births active; Sleep parks alive but inactive; Wake
// reactivates; Dispose destroys and clears resources (comps drop with the
// entity, the lifecycle entry drops too). IDs never repeat, Clear never
// rewinds issuance. Sleeping keeps transforms and comps; only Dispose
// clears them. Disposing a parent orphans children as roots keeping their
// own states; sleeping never cascades.
//
// Use World for placement and comp reads/writes; births, deaths, and
// level switches go through Scene. Never call Spawn, Despawn, or Clear on
// the returned World. File load/save lives in the 10.2 section (S33).
//
// Errors: missing ids are NotFound; nil scenes take InvalidArg on writes
// and park on reads.

// LifeState is the activation flag of one live entity.
type LifeState int

const (
	// LifeActive runs; Spawn births here.
	LifeActive LifeState = iota
	// LifeSleeping parks alive but inactive until Wake.
	LifeSleeping
)

// String names the state for logs.
func (s LifeState) String() string {
	if s == LifeSleeping {
		return "sleeping"
	}
	return "active"
}

// Scene is one level: entity storage plus activation flags.
type Scene struct {
	w     World
	state map[ID]LifeState
}

// NewScene builds an empty level issuing IDs from 1.
func NewScene() Scene {
	return Scene{w: NewWorld(), state: map[ID]LifeState{}}
}

func (s *Scene) mark(id ID, st LifeState) {
	if s.state == nil {
		s.state = map[ID]LifeState{}
	}
	s.state[id] = st
}

// Spawn births one entity under parent (NoEntity for a root), active.
// A missing parent is NotFound.
func (s *Scene) Spawn(parent ID) (ID, error) {
	if s == nil {
		return NoEntity, core.InvalidArg("scene.Spawn", "scene")
	}
	id, err := s.w.Spawn(parent)
	if err != nil {
		return NoEntity, err
	}
	s.mark(id, LifeActive)
	return id, nil
}

// Sleep parks id alive but inactive. Missing ids are NotFound;
// re-sleeping is a no-op.
func (s *Scene) Sleep(id ID) error {
	if s == nil {
		return core.InvalidArg("scene.Sleep", "scene")
	}
	if !s.w.Alive(id) {
		return core.NotFound("scene.Sleep", "entity")
	}
	s.mark(id, LifeSleeping)
	return nil
}

// Wake reactivates id. Missing ids are NotFound; re-waking is a no-op.
func (s *Scene) Wake(id ID) error {
	if s == nil {
		return core.InvalidArg("scene.Wake", "scene")
	}
	if !s.w.Alive(id) {
		return core.NotFound("scene.Wake", "entity")
	}
	s.mark(id, LifeActive)
	return nil
}

// State returns the activation flag of id.
func (s *Scene) State(id ID) (LifeState, error) {
	if s == nil || s.state == nil {
		return LifeActive, core.NotFound("scene.State", "entity")
	}
	st, ok := s.state[id]
	if !ok || !s.w.Alive(id) {
		return LifeActive, core.NotFound("scene.State", "entity")
	}
	return st, nil
}

// IsActive reports whether id is alive and running.
func (s *Scene) IsActive(id ID) bool {
	st, err := s.State(id)
	return err == nil && st == LifeActive
}

// IsSleeping reports whether id is alive and parked.
func (s *Scene) IsSleeping(id ID) bool {
	st, err := s.State(id)
	return err == nil && st == LifeSleeping
}

// Alive reports whether id names a live entity (active or sleeping).
func (s *Scene) Alive(id ID) bool {
	if s == nil {
		return false
	}
	return s.w.Alive(id)
}

// Dispose destroys id and clears its resources: comps drop with the
// entity and the lifecycle entry drops too. Children survive as roots
// keeping their own states. Missing ids are NotFound, never a crash.
func (s *Scene) Dispose(id ID) error {
	if s == nil {
		return core.InvalidArg("scene.Dispose", "scene")
	}
	if err := s.w.Despawn(id); err != nil {
		return err
	}
	delete(s.state, id)
	return nil
}

// Count returns the live entities (active plus sleeping).
func (s *Scene) Count() int {
	if s == nil {
		return 0
	}
	return s.w.Count()
}

// ActiveCount counts live entities flagged active.
func (s *Scene) ActiveCount() int {
	if s == nil || s.state == nil {
		return 0
	}
	n := 0
	for id, st := range s.state {
		if st == LifeActive && s.w.Alive(id) {
			n++
		}
	}
	return n
}

// SleepingCount counts live entities flagged sleeping.
func (s *Scene) SleepingCount() int {
	if s == nil || s.state == nil {
		return 0
	}
	n := 0
	for id, st := range s.state {
		if st == LifeSleeping && s.w.Alive(id) {
			n++
		}
	}
	return n
}

// Spawned counts every birth here, including the disposed. It only
// grows, so replays can prove ids never repeat and levels never leak.
func (s *Scene) Spawned() uint64 {
	if s == nil {
		return 0
	}
	return s.w.Spawned()
}

// Clear drops every entity, comp, and flag for a level switch. Issuance
// never rewinds: the next Spawn still mints a fresh id.
func (s *Scene) Clear() {
	if s == nil {
		return
	}
	s.w.Clear()
	s.state = map[ID]LifeState{}
}

// World exposes placement and comp storage for this level. Births,
// deaths, and level switches go through Scene; never call Spawn,
// Despawn, or Clear on the result.
func (s *Scene) World() *World {
	if s == nil {
		return nil
	}
	return &s.w
}

// Scene files (10.2, S33/W4): the level on disk. S34 above owns the
// live Scene; SceneFile below is only the filed shape (name, version,
// entities with parent links, comps, referenced asset ids). LoadScene
// reads it, Open builds a live Scene with every birth active plus the
// filed-name to live-id index. Parents are always filed before their
// children so Open stamps in one pass, never guessing order.

// SceneFileVersion is the scene file version every Encode writes.
// Bump Minor for additive fields, Major for a breaking shape.
var SceneFileVersion = core.Version{Major: 1, Minor: 0}

// Frozen budgets. A well-formed file beyond budget is OutOfMemory,
// never a guessed load. A bad value is InvalidArg via constructors
// and BadData via Parse.
const (
	// MaxSceneNameLen caps scene and entity names in bytes.
	MaxSceneNameLen = 64
	// MaxSceneEntities caps entities in one scene file.
	MaxSceneEntities = 4096
	// MaxSceneComps caps comps on one filed entity.
	MaxSceneComps = 16
	// MaxSceneBytes caps one scene file.
	MaxSceneBytes = 4 << 20
)

func validSceneName(s string) bool { return s != "" && len(s) <= MaxSceneNameLen }

// SceneNode is one filed entity: its name, its parent's name ("" for a
// root), its local placement, and its comps in stamp order.
type SceneNode struct {
	Name   string
	Parent string
	Local  Transform
	Comps  []Comp
}

// SceneFile is one level on disk: a name plus entities in file order
// (parents before children). Fields stay private so every write passes
// validation; Entities returns a deep copy.
type SceneFile struct {
	name    string
	version core.Version
	nodes   []SceneNode
}

// NewSceneFile builds an empty level with SceneFileVersion. Empty or
// overlong names are a core InvalidArg error.
func NewSceneFile(name string) (SceneFile, error) {
	if !validSceneName(name) {
		return SceneFile{}, core.InvalidArg("world.NewSceneFile", "name")
	}
	return SceneFile{name: name, version: SceneFileVersion}, nil
}

// Name returns the level name, or "" on a nil file.
func (s *SceneFile) Name() string {
	if s == nil {
		return ""
	}
	return s.name
}

// Version returns the stored file version.
func (s *SceneFile) Version() core.Version {
	if s == nil {
		return core.Version{}
	}
	return s.version
}

// Count returns the filed entities, or 0 on a nil file.
func (s *SceneFile) Count() int {
	if s == nil {
		return 0
	}
	return len(s.nodes)
}

// Entities returns the filed entities in file order with fresh comp
// slices; writing the result cannot change the file. Nil files report
// nil.
func (s *SceneFile) Entities() []SceneNode {
	if s == nil {
		return nil
	}
	out := make([]SceneNode, len(s.nodes))
	for i, n := range s.nodes {
		out[i] = SceneNode{
			Name:   n.Name,
			Parent: n.Parent,
			Local:  n.Local,
			Comps:  append([]Comp(nil), n.Comps...),
		}
	}
	return out
}

// AddEntity files one entity after its parent ("" for a root).
// Duplicate names, bad placements, and empty comp kinds are InvalidArg;
// an unknown parent is NotFound (file the parent first); past
// MaxSceneEntities or MaxSceneComps is OutOfMemory. Failures file
// nothing. Nil files report InvalidArg.
func (s *SceneFile) AddEntity(name, parent string, local Transform, comps ...Comp) error {
	const op = "world.SceneFile.AddEntity"
	if s == nil {
		return core.InvalidArg(op, "scene")
	}
	if !validSceneName(name) {
		return core.InvalidArg(op, "entity")
	}
	for _, n := range s.nodes {
		if n.Name == name {
			return core.InvalidArg(op, "entity")
		}
	}
	if parent != "" {
		found := false
		for _, n := range s.nodes {
			if n.Name == parent {
				found = true
				break
			}
		}
		if !found {
			return core.NotFound(op, parent)
		}
	}
	if err := checkTransform(op, local); err != nil {
		return err
	}
	if len(comps) > MaxSceneComps {
		return core.OutOfMemory(op, "comps")
	}
	for _, c := range comps {
		if c.Kind == "" {
			return core.InvalidArg(op, "kind")
		}
	}
	if len(s.nodes) >= MaxSceneEntities {
		return core.OutOfMemory(op, "entities")
	}
	s.nodes = append(s.nodes, SceneNode{
		Name:   name,
		Parent: parent,
		Local:  local,
		Comps:  append([]Comp(nil), comps...),
	})
	return nil
}

// Equal reports whether o holds the same name, version, and entities
// in order with the same comps. A nil file only equals nothing, never
// a value.
func (s *SceneFile) Equal(o SceneFile) bool {
	if s == nil {
		return false
	}
	if s.name != o.name || s.version != o.version || len(s.nodes) != len(o.nodes) {
		return false
	}
	for i := range s.nodes {
		a, b := s.nodes[i], o.nodes[i]
		if a.Name != b.Name || a.Parent != b.Parent || a.Local != b.Local {
			return false
		}
		if len(a.Comps) != len(b.Comps) {
			return false
		}
		for j := range a.Comps {
			if a.Comps[j] != b.Comps[j] {
				return false
			}
		}
	}
	return true
}

// checkShape validates an in-memory file for Encode/Open: bad values
// are InvalidArg, budget overruns are OutOfMemory. Version is not
// checked here.
func (s *SceneFile) checkShape(op string) error {
	if s == nil {
		return core.InvalidArg(op, "scene")
	}
	if !validSceneName(s.name) {
		return core.InvalidArg(op, "name")
	}
	if len(s.nodes) > MaxSceneEntities {
		return core.OutOfMemory(op, "entities")
	}
	seen := make(map[string]bool, len(s.nodes))
	for _, n := range s.nodes {
		if !validSceneName(n.Name) {
			return core.InvalidArg(op, "entity")
		}
		if seen[n.Name] {
			return core.InvalidArg(op, "entity")
		}
		if n.Parent != "" && !seen[n.Parent] {
			return core.InvalidArg(op, "parent")
		}
		seen[n.Name] = true
		if err := checkTransform(op, n.Local); err != nil {
			return err
		}
		if len(n.Comps) > MaxSceneComps {
			return core.OutOfMemory(op, "comps")
		}
		for _, c := range n.Comps {
			if c.Kind == "" {
				return core.InvalidArg(op, "kind")
			}
		}
	}
	return nil
}

// jsonSceneEntity is the frozen filed entity on disk.
type jsonSceneEntity struct {
	Name   string         `json:"name"`
	Parent string         `json:"parent"`
	Local  *jsonTransform `json:"local"`
	Comps  []jsonComp     `json:"comps"`
}

// jsonScene is the frozen scene file.
type jsonScene struct {
	Version  string            `json:"version"`
	Name     string            `json:"name"`
	Entities []jsonSceneEntity `json:"entities"`
}

// Encode renders the canonical file bytes. The file must already carry
// SceneFileVersion; otherwise the result is a core VersionMismatch
// error. Bad shapes are InvalidArg, budget overruns are OutOfMemory.
// The input is never mutated. Empty entity and comp lists encode as []
// so a file round-trips byte-identical.
func (s *SceneFile) Encode() ([]byte, error) {
	const op = "world.SceneFile.Encode"
	if err := s.checkShape(op); err != nil {
		return nil, err
	}
	if s.version != SceneFileVersion {
		return nil, core.VersionMismatch(op, s.version.String())
	}
	ents := make([]jsonSceneEntity, len(s.nodes))
	for i, n := range s.nodes {
		comps := make([]jsonComp, len(n.Comps))
		for j, c := range n.Comps {
			comps[j] = compToJSON(c)
		}
		local := transformToJSON(n.Local)
		ents[i] = jsonSceneEntity{Name: n.Name, Parent: n.Parent, Local: &local, Comps: comps}
	}
	out, err := json.Marshal(jsonScene{Version: s.version.String(), Name: s.name, Entities: ents})
	if err != nil {
		return nil, core.BadData(op, "encode")
	}
	if len(out) > MaxSceneBytes {
		return nil, core.OutOfMemory(op, "bytes")
	}
	return out, nil
}

// ParseScene validates file bytes into a SceneFile. Empty input is
// InvalidArg, size overruns are OutOfMemory, torn JSON and bad shapes
// (duplicates, unknown or forward parents, bad links) are BadData,
// foreign or newer versions are VersionMismatch. Missing entity and
// comp lists default to empty.
func ParseScene(data []byte) (SceneFile, error) {
	const op = "world.ParseScene"
	if len(data) == 0 {
		return SceneFile{}, core.InvalidArg(op, "data")
	}
	if len(data) > MaxSceneBytes {
		return SceneFile{}, core.OutOfMemory(op, "bytes")
	}
	var raw jsonScene
	if err := json.Unmarshal(data, &raw); err != nil {
		return SceneFile{}, core.BadData(op, "json", err)
	}
	if raw.Version == "" {
		return SceneFile{}, core.BadData(op, "version")
	}
	ver, err := core.ParseVersion(raw.Version)
	if err != nil {
		return SceneFile{}, core.BadData(op, "version")
	}
	if !ver.CompatibleWith(SceneFileVersion) {
		return SceneFile{}, core.VersionMismatch(op, ver.String())
	}
	if !validSceneName(raw.Name) {
		return SceneFile{}, core.BadData(op, "name")
	}
	if len(raw.Entities) > MaxSceneEntities {
		return SceneFile{}, core.OutOfMemory(op, "entities")
	}
	sf := SceneFile{name: raw.Name, version: ver}
	seen := make(map[string]bool, len(raw.Entities))
	for _, je := range raw.Entities {
		if !validSceneName(je.Name) {
			return SceneFile{}, core.BadData(op, "entity")
		}
		if seen[je.Name] {
			return SceneFile{}, core.BadData(op, "entity")
		}
		if je.Parent != "" && !seen[je.Parent] {
			return SceneFile{}, core.BadData(op, "parent")
		}
		seen[je.Name] = true
		local, err := transformFromJSON(op, je.Local)
		if err != nil {
			return SceneFile{}, err
		}
		if len(je.Comps) > MaxSceneComps {
			return SceneFile{}, core.OutOfMemory(op, "comps")
		}
		comps := make([]Comp, 0, len(je.Comps))
		for _, jc := range je.Comps {
			c, err := compFromJSON(op, jc)
			if err != nil {
				return SceneFile{}, err
			}
			comps = append(comps, c)
		}
		sf.nodes = append(sf.nodes, SceneNode{
			Name:   je.Name,
			Parent: je.Parent,
			Local:  local,
			Comps:  comps,
		})
	}
	return sf, nil
}

// LoadScene reads path as a scene file. Empty paths are InvalidArg,
// missing files are NotFound; the rest matches ParseScene.
func LoadScene(path string) (SceneFile, error) {
	const op = "world.LoadScene"
	if path == "" {
		return SceneFile{}, core.InvalidArg(op, "path")
	}
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return SceneFile{}, core.NotFound(op, path, err)
	}
	return ParseScene(raw)
}

// SaveScene writes the file to path, creating parent directories.
// Empty paths are InvalidArg; OS failures are NotFound with the cause;
// file-shape errors match Encode. Nil files report InvalidArg.
func (s *SceneFile) SaveScene(path string) error {
	const op = "world.SceneFile.SaveScene"
	if path == "" {
		return core.InvalidArg(op, "path")
	}
	raw, err := s.Encode()
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

// Open builds the live level: every entity spawned in file order (so
// parents exist before their children), locals and comps stamped, every
// birth active. It also returns the filed-name to live-id index. A bad
// in-memory file is InvalidArg or OutOfMemory and builds nothing. The
// filed shape is never mutated.
func (s *SceneFile) Open() (Scene, map[string]ID, error) {
	const op = "world.SceneFile.Open"
	if err := s.checkShape(op); err != nil {
		return Scene{}, nil, err
	}
	sc := NewScene()
	w := sc.World()
	ids := make(map[string]ID, len(s.nodes))
	for _, n := range s.nodes {
		parent := NoEntity
		if n.Parent != "" {
			parent = ids[n.Parent]
		}
		id, err := sc.Spawn(parent)
		if err != nil {
			return Scene{}, nil, err
		}
		if err := w.SetTransform(id, n.Local); err != nil {
			return Scene{}, nil, err
		}
		for _, c := range n.Comps {
			if err := w.AddComp(id, c); err != nil {
				return Scene{}, nil, err
			}
		}
		ids[n.Name] = id
	}
	return sc, ids, nil
}

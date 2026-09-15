package world

import (
	"math"

	"github.com/energye/gpui/game/core"
)

// ID names one entity. NoEntity (0) never names a live entity and means
// "no parent" in Spawn and SetParent.
type ID uint64

// NoEntity is the null id: no parent, no match.
const NoEntity ID = 0

// Transform is a local placement: position plus rotation (radians,
// counter-clockwise) plus scale. Scale may be zero (collapsed) or
// negative (mirrored); only non-finite numbers are rejected.
type Transform struct {
	Pos   core.Vec2
	Rot   float64
	Scale core.Vec2
}

// IdentityTransform returns the neutral placement: origin, no rotation,
// unit scale.
func IdentityTransform() Transform { return Transform{Scale: core.V2(1, 1)} }

// LocalMatrix renders t as Translate(Pos)*Rotate(Rot)*Scale(Scale).
// Same layout as core.Mat2D; convert once at the render boundary.
func LocalMatrix(t Transform) core.Mat2D {
	return core.Translate2D(t.Pos.X, t.Pos.Y).
		Mul(core.Rotate2D(t.Rot)).
		Mul(core.Scale2D(t.Scale.X, t.Scale.Y))
}

// Comp is one capability hung on an entity: Kind names it ("sprite",
// "collider", "script", ...) and must be non-empty; Ref carries the
// asset id and may stay empty for pure-logic comps.
type Comp struct {
	Kind string
	Ref  core.AssetID
}

type node struct {
	parent   ID
	children []ID
	local    Transform
	comps    []Comp
}

// World owns every entity: ids, links, locals, comps. The zero value is
// usable but NewWorld states the intent. IDs never repeat inside one
// World, not even after Despawn or Clear.
type World struct {
	nodes   map[ID]*node
	next    ID
	spawned uint64
}

// NewWorld builds an empty world issuing IDs from 1.
func NewWorld() World {
	return World{nodes: map[ID]*node{}, next: 1}
}

func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

func checkTransform(op string, t Transform) error {
	if !finite(t.Pos.X) || !finite(t.Pos.Y) || !finite(t.Rot) ||
		!finite(t.Scale.X) || !finite(t.Scale.Y) {
		return core.InvalidArg(op, "transform")
	}
	return nil
}

func (w *World) get(id ID) (*node, error) {
	if w == nil || w.nodes == nil {
		return nil, core.InvalidArg("world", "world")
	}
	n, ok := w.nodes[id]
	if !ok || id == NoEntity {
		return nil, core.NotFound("world", "entity")
	}
	return n, nil
}

func detach(children []ID, id ID) []ID {
	for i, c := range children {
		if c == id {
			return append(children[:i], children[i+1:]...)
		}
	}
	return children
}

// Spawn births one entity under parent (NoEntity for a root) with the
// identity transform and no comps. A missing parent is NotFound.
func (w *World) Spawn(parent ID) (ID, error) {
	if w == nil {
		return NoEntity, core.InvalidArg("world.Spawn", "world")
	}
	if parent != NoEntity {
		if _, err := w.get(parent); err != nil {
			return NoEntity, err
		}
	}
	if w.nodes == nil {
		w.nodes = map[ID]*node{}
	}
	if w.next == NoEntity {
		w.next = 1
	}
	id := w.next
	w.next++
	n := &node{parent: parent, local: IdentityTransform()}
	w.nodes[id] = n
	w.spawned++
	if parent != NoEntity {
		p := w.nodes[parent]
		p.children = append(p.children, id)
	}
	return id, nil
}

// Despawn deletes id and drops its comps. Children survive: they fall
// back to roots keeping their local transforms. A missing id is
// NotFound, never a crash.
func (w *World) Despawn(id ID) error {
	n, err := w.get(id)
	if err != nil {
		return err
	}
	for _, c := range n.children {
		if k, ok := w.nodes[c]; ok {
			k.parent = NoEntity
		}
	}
	if n.parent != NoEntity {
		if p, ok := w.nodes[n.parent]; ok {
			p.children = detach(p.children, id)
		}
	}
	delete(w.nodes, id)
	return nil
}

// Alive reports whether id names a live entity. Nil worlds and NoEntity
// report false.
func (w *World) Alive(id ID) bool {
	if _, err := w.get(id); err != nil {
		return false
	}
	return true
}

// Count returns the number of live entities, or 0 on a nil world.
func (w *World) Count() int {
	if w == nil || w.nodes == nil {
		return 0
	}
	return len(w.nodes)
}

// Spawned returns how many entities were ever born here, including the
// despawned. It only grows, so replays can prove ids never repeat.
func (w *World) Spawned() uint64 {
	if w == nil {
		return 0
	}
	return w.spawned
}

// isBelow reports whether id sits on ancestor's chain (id == ancestor or
// below it). Callers hold valid nodes, so the walk always ends.
func (w *World) isBelow(id, ancestor ID) bool {
	for c := id; c != NoEntity; {
		if c == ancestor {
			return true
		}
		n, ok := w.nodes[c]
		if !ok {
			return false
		}
		c = n.parent
	}
	return false
}

// SetParent re-links child under parent (NoEntity detaches to a root).
// Self-parenting and cycles (parent on child's own chain) are InvalidArg;
// missing ids are NotFound. Re-linking to the same parent is a no-op.
func (w *World) SetParent(child, parent ID) error {
	c, err := w.get(child)
	if err != nil {
		return err
	}
	if parent != NoEntity {
		if _, err := w.get(parent); err != nil {
			return err
		}
	}
	if parent == child {
		return core.InvalidArg("world.SetParent", "parent")
	}
	if parent != NoEntity && w.isBelow(parent, child) {
		return core.InvalidArg("world.SetParent", "cycle")
	}
	if c.parent == parent {
		return nil
	}
	if c.parent != NoEntity {
		if p, ok := w.nodes[c.parent]; ok {
			p.children = detach(p.children, child)
		}
	}
	c.parent = parent
	if parent != NoEntity {
		p := w.nodes[parent]
		p.children = append(p.children, child)
	}
	return nil
}

// Parent returns the parent of id (NoEntity for a root).
func (w *World) Parent(id ID) (ID, error) {
	n, err := w.get(id)
	if err != nil {
		return NoEntity, err
	}
	return n.parent, nil
}

// Children returns the direct children of id in link order. The result is
// a fresh slice; the input world is never exposed.
func (w *World) Children(id ID) ([]ID, error) {
	n, err := w.get(id)
	if err != nil {
		return nil, err
	}
	return append([]ID(nil), n.children...), nil
}

// SetTransform replaces the local transform of id. Non-finite numbers
// are InvalidArg and store nothing.
func (w *World) SetTransform(id ID, t Transform) error {
	n, err := w.get(id)
	if err != nil {
		return err
	}
	if err := checkTransform("world.SetTransform", t); err != nil {
		return err
	}
	n.local = t
	return nil
}

// Local returns the local transform of id.
func (w *World) Local(id ID) (Transform, error) {
	n, err := w.get(id)
	if err != nil {
		return Transform{}, err
	}
	return n.local, nil
}

// composeTransform folds a local placement under an already-folded
// parent: scales multiply component-wise, rotations add, the local offset
// is scaled then rotated by the parent and added to the parent position.
func composeTransform(p, l Transform) Transform {
	sx, sy := l.Pos.X*p.Scale.X, l.Pos.Y*p.Scale.Y
	c, s := math.Cos(p.Rot), math.Sin(p.Rot)
	return Transform{
		Pos:   core.V2(p.Pos.X+sx*c-sy*s, p.Pos.Y+sx*s+sy*c),
		Rot:   p.Rot + l.Rot,
		Scale: core.V2(p.Scale.X*l.Scale.X, p.Scale.Y*l.Scale.Y),
	}
}

// chain returns the ancestor locals from id up to the root, id first.
func (w *World) chain(id ID) []Transform {
	var out []Transform
	for c := id; c != NoEntity; {
		n, ok := w.nodes[c]
		if !ok {
			break
		}
		out = append(out, n.local)
		c = n.parent
	}
	return out
}

// WorldOf returns the world transform of id: every ancestor folded from
// the root down (see composeTransform).
func (w *World) WorldOf(id ID) (Transform, error) {
	if _, err := w.get(id); err != nil {
		return Transform{}, err
	}
	got := w.chain(id)
	wfold := IdentityTransform()
	for i := len(got) - 1; i >= 0; i-- {
		wfold = composeTransform(wfold, got[i])
	}
	return wfold, nil
}

// WorldMatrix returns the world matrix of id: each ancestor local folded
// as parent * local from the root down. On rotation-free or uniform-scale
// chains it matches LocalMatrix(WorldOf(id)) inside 1e-9; the translation
// always matches.
func (w *World) WorldMatrix(id ID) (core.Mat2D, error) {
	if _, err := w.get(id); err != nil {
		return core.Mat2D{}, err
	}
	got := w.chain(id)
	m := core.Identity2D()
	for i := len(got) - 1; i >= 0; i-- {
		m = m.Mul(LocalMatrix(got[i]))
	}
	return m, nil
}

// AddComp hangs c on id. An empty Kind is InvalidArg; duplicates are
// allowed (two scripts on one entity queue in order).
func (w *World) AddComp(id ID, c Comp) error {
	n, err := w.get(id)
	if err != nil {
		return err
	}
	if c.Kind == "" {
		return core.InvalidArg("world.AddComp", "kind")
	}
	n.comps = append(n.comps, c)
	return nil
}

// RemoveComp drops the first comp of that kind on id. An empty kind is
// InvalidArg; a kind the entity does not carry is NotFound.
func (w *World) RemoveComp(id ID, kind string) error {
	n, err := w.get(id)
	if err != nil {
		return err
	}
	if kind == "" {
		return core.InvalidArg("world.RemoveComp", "kind")
	}
	for i, c := range n.comps {
		if c.Kind == kind {
			n.comps = append(n.comps[:i], n.comps[i+1:]...)
			return nil
		}
	}
	return core.NotFound("world.RemoveComp", kind)
}

// HasComp reports whether id carries at least one comp of that kind.
// Missing entities and empty kinds report false.
func (w *World) HasComp(id ID, kind string) bool {
	n, err := w.get(id)
	if err != nil || kind == "" {
		return false
	}
	for _, c := range n.comps {
		if c.Kind == kind {
			return true
		}
	}
	return false
}

// Comps returns the comps of id in add order. The result is a fresh
// slice; the stored order is never exposed.
func (w *World) Comps(id ID) ([]Comp, error) {
	n, err := w.get(id)
	if err != nil {
		return nil, err
	}
	return append([]Comp(nil), n.comps...), nil
}

// Clear drops every entity and comp. Issuance never rewinds: the next
// Spawn still mints a fresh id above every id ever born here.
func (w *World) Clear() {
	if w == nil {
		return
	}
	w.nodes = map[ID]*node{}
}

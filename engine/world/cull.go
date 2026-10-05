//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package world

import (
	"math"
	"sort"
	"time"

	"github.com/energye/gpui/engine/core"
)

// Large-entity cull and sort (S85, V3): the active set plus sleep plus budget.
//
// A large level holds 20000 entities but the screen only shows about 1500.
// Refreshing every matrix every frame wastes the frame, so Cull keeps the
// update set: entities inside the view plus pinned awake ones. Sleepers skip
// matrices, only the active set is sorted, attached children follow their
// parents through the same fold WorldOf uses, so a weapon never detaches.
//
// Godot mapping (only the approach is copied, no code is moved):
//   - CanvasItem identity card (G01): every tracked entity carries a layer
//     band plus a feet Y. Layer numbers match engine/sprite one to one
//     (0 world, 1 fx, 2 ui, custom bands stay plain integers).
//   - RendererCanvasCull visible order (G17): the active set sorts by layer
//     ascending, then world feet Y ascending, ties broken by track order so
//     equal heights never flicker. D15 pins the same rule.
//   - RemoteTransform2D挂点 (N4): parent links fold exactly like WorldOf
//     (composeTransform), moving the parent moves the child, no second index.
//   - Marker2D出生点 (N4): a spawn marker is a plain entity with zero extent
//     and a "marker" comp; culling treats it like any other point.
//   - VisibleOnScreenNotifier/Enabler2D进屏启停 (N4): every Refresh reports
//     the entered/exited visibility sets; with AutoSleep on, entered
//     sleepers wake and exited non-pinned entities sleep, mirroring the
//     enabler toggling its target.
//
// Broad phase folds positions for every tracked entity (cheap math, no
// heap); the caller computes WorldOf/WorldMatrix only for Active, so
// sleeping entities never pay for matrices. entity.go/scene.go semantics
// are untouched: this file only reads them and drives Sleep/Wake.
//
// Errors: bad numbers are InvalidArg, unknown ids are NotFound, past
// MaxCullEntities is OutOfMemory. Bad writes change nothing. Nil receivers
// never panic: getters park at zero, writers report InvalidArg.

// MaxCullEntities caps one Cull index. Past it Track reports OutOfMemory,
// never a half entry. 20000 large-scene entities fit with room to spare.
const MaxCullEntities = 32768

// cullEntry is one tracked entity: the draw band, the feet offset above
// the world origin, the half extents of the visibility box, and the awake
// pin. seq is the track order and breaks sort ties deterministically.
type cullEntry struct {
	id     ID
	seq    uint64
	layer  int
	footDY float64
	hw     float64
	hh     float64
	awake  bool
}

// cullKey is the sort scratch for one final active entity: y carries the
// feet-adjusted sort height, wy keeps the raw world spot for Snapshot;
// pinned and parent ride along so Snapshot needs no second lookup round.
type cullKey struct {
	id     ID
	layer  int
	x      float64
	y      float64
	wy     float64
	pinned bool
	parent ID
	seq    uint64
}

// ActiveSnap is one update-set member with its world spot: the production
// read path. Snapshot fills them during Refresh, so per-frame callers copy
// structs instead of paying WorldOf plus three map lookups per active; the
// positions equal WorldOf exactly (the unit test pins that agreement).
type ActiveSnap struct {
	ID     ID
	X, Y   float64
	Layer  int
	Pinned bool
	Parent ID
}

// Cull is one visibility index over a World. The World stays the only
// placement truth; Cull only caches the per-entity draw band, feet offset,
// visibility box, and awake pin the caller registered. Refresh rebuilds
// the active set from the live world numbers, so moving an entity in the
// World is enough: no second placement table drifts.
type Cull struct {
	view      core.Rect
	autoSleep bool
	maxActive int
	items     []cullEntry
	index     map[ID]int
	seq       uint64
	active    []ID
	entered   []ID
	exited    []ID
	candByID  []ID
	prevCand  []ID
	keys      []cullKey
	snaps     []ActiveSnap
	refreshes int64
	dropped   int
	trimmed   int
	lastUs    int64
}

func finiteCull(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

func finiteRectCull(r core.Rect) bool {
	return finiteCull(r.X) && finiteCull(r.Y) && finiteCull(r.W) && finiteCull(r.H)
}

// NewCull builds a visibility index over view. The view must hold finite
// numbers; anything else is InvalidArg. An empty view is allowed and means
// awake pins only. AutoSleep starts off, MaxActive starts unlimited.
func NewCull(view core.Rect) (Cull, error) {
	const op = "world.NewCull"
	if !finiteRectCull(view) {
		return Cull{}, core.InvalidArg(op, "view")
	}
	return Cull{view: view}, nil
}

// SetView replaces the view box. Non-finite numbers are InvalidArg and
// keep the old view. Empty views are allowed (awake pins only).
func (c *Cull) SetView(view core.Rect) error {
	const op = "world.Cull.SetView"
	if c == nil {
		return core.InvalidArg(op, "cull")
	}
	if !finiteRectCull(view) {
		return core.InvalidArg(op, "view")
	}
	c.view = view
	return nil
}

// View returns the current view box, or zero on a nil index.
func (c *Cull) View() core.Rect {
	if c == nil {
		return core.Rect{}
	}
	return c.view
}

// SetAutoSleep flips the enabler: with it on, Refresh wakes every visible
// sleeper (entered or already inside) and sleeps exited non-pinned entities
// through the Scene. With it off, Refresh never writes the Scene and only
// reports sets.
func (c *Cull) SetAutoSleep(on bool) {
	if c == nil {
		return
	}
	c.autoSleep = on
}

// AutoSleep reports the enabler switch, or false on a nil index.
func (c *Cull) AutoSleep() bool {
	if c == nil {
		return false
	}
	return c.autoSleep
}

// SetMaxActive caps the update set: past n actives Refresh keeps the
// first n in draw order and counts the rest in Trimmed. Zero means
// unlimited. Negative numbers are InvalidArg and keep the old cap.
func (c *Cull) SetMaxActive(n int) error {
	const op = "world.Cull.SetMaxActive"
	if c == nil {
		return core.InvalidArg(op, "cull")
	}
	if n < 0 {
		return core.InvalidArg(op, "max")
	}
	c.maxActive = n
	return nil
}

// MaxActive returns the update cap (0 is unlimited), or 0 on nil.
func (c *Cull) MaxActive() int {
	if c == nil {
		return 0
	}
	return c.maxActive
}

func checkCullBox(op string, footDY, hw, hh float64) error {
	if !finiteCull(footDY) || !finiteCull(hw) || !finiteCull(hh) {
		return core.InvalidArg(op, "box")
	}
	if hw < 0 || hh < 0 {
		return core.InvalidArg(op, "extent")
	}
	return nil
}

// Track registers id with its draw band, feet offset, and visibility half
// extents (0/0 is a point, like a spawn marker). Awake pins are set with
// Awake, not here. NoEntity is InvalidArg, duplicates are InvalidArg,
// past MaxCullEntities is OutOfMemory. The entity need not be alive yet;
// dead ids drop at the next Refresh and count in Dropped.
func (c *Cull) Track(id ID, layer int, footDY, hw, hh float64) error {
	const op = "world.Cull.Track"
	if c == nil {
		return core.InvalidArg(op, "cull")
	}
	if id == NoEntity {
		return core.InvalidArg(op, "entity")
	}
	if err := checkCullBox(op, footDY, hw, hh); err != nil {
		return err
	}
	if c.index != nil {
		if _, dup := c.index[id]; dup {
			return core.InvalidArg(op, "duplicate")
		}
	}
	if len(c.items) >= MaxCullEntities {
		return core.OutOfMemory(op, "entities")
	}
	if c.index == nil {
		c.index = map[ID]int{}
	}
	c.seq++
	c.index[id] = len(c.items)
	c.items = append(c.items, cullEntry{id: id, seq: c.seq, layer: layer, footDY: footDY, hw: hw, hh: hh})
	return nil
}

// Update rewrites the band, feet offset, and extents of a tracked id.
// Unknown ids are NotFound; bad numbers change nothing.
func (c *Cull) Update(id ID, layer int, footDY, hw, hh float64) error {
	const op = "world.Cull.Update"
	if c == nil {
		return core.InvalidArg(op, "cull")
	}
	if err := checkCullBox(op, footDY, hw, hh); err != nil {
		return err
	}
	i, ok := c.index[id]
	if !ok {
		return core.NotFound(op, "entity")
	}
	c.items[i].layer = layer
	c.items[i].footDY = footDY
	c.items[i].hw = hw
	c.items[i].hh = hh
	return nil
}

// Untrack forgets id. Unknown ids are NotFound. The World entity itself
// is untouched; only the index entry drops.
func (c *Cull) Untrack(id ID) error {
	const op = "world.Cull.Untrack"
	if c == nil {
		return core.InvalidArg(op, "cull")
	}
	i, ok := c.index[id]
	if !ok {
		return core.NotFound(op, "entity")
	}
	c.removeAt(i)
	return nil
}

func (c *Cull) removeAt(i int) {
	last := len(c.items) - 1
	if i != last {
		c.items[i] = c.items[last]
		c.index[c.items[i].id] = i
	}
	delete(c.index, c.items[last].id)
	c.items = c.items[:last]
}

// Awake pins id: it stays in the update set even offscreen (the camera
// target, the player). Unknown ids are NotFound.
func (c *Cull) Awake(id ID) error {
	const op = "world.Cull.Awake"
	if c == nil {
		return core.InvalidArg(op, "cull")
	}
	i, ok := c.index[id]
	if !ok {
		return core.NotFound(op, "entity")
	}
	c.items[i].awake = true
	return nil
}

// Release unpins id. Unknown ids are NotFound.
func (c *Cull) Release(id ID) error {
	const op = "world.Cull.Release"
	if c == nil {
		return core.InvalidArg(op, "cull")
	}
	i, ok := c.index[id]
	if !ok {
		return core.NotFound(op, "entity")
	}
	c.items[i].awake = false
	return nil
}

// IsAwake reports the pin flag, or false for unknown and nil ids.
func (c *Cull) IsAwake(id ID) bool {
	if c == nil {
		return false
	}
	i, ok := c.index[id]
	if !ok {
		return false
	}
	return c.items[i].awake
}

// foldWorld folds the world transform of id through the same parent chain
// WorldOf uses (composeTransform from the root down). The fast path keeps
// the hot tick at two map lookups for the 2.5% attached entities and one
// for the rest: roots read their local directly, children fold exactly one
// parent compose. Deeper chains (grandchildren and below) fall back to the
// full recursion so挂点 semantics stay exact. Missing ids report false.
func (w *World) foldWorld(id ID) (Transform, bool) {
	if w == nil || w.nodes == nil || id == NoEntity {
		return Transform{}, false
	}
	n, ok := w.nodes[id]
	if !ok {
		return Transform{}, false
	}
	if n.parent == NoEntity {
		return n.local, true
	}
	p, ok := w.nodes[n.parent]
	if !ok {
		return Transform{}, false
	}
	if p.parent == NoEntity {
		return composeTransform(p.local, n.local), true
	}
	pp, ok := w.foldWorld(n.parent)
	if !ok {
		return Transform{}, false
	}
	return composeTransform(pp, n.local), true
}

// Refresh rebuilds the update set from the live world numbers: every
// tracked entity folds its world position, in-view plus awake pins become
// visibility candidates, entered/exited diff against the last refresh,
// the enabler optionally drives Scene Sleep/Wake, and the final actives
// sort by layer then feet Y. Dead ids drop silently and count in Dropped.
// A nil World is InvalidArg and changes nothing; a nil Scene means pure
// query (no sleep reads, no enabler writes).
func (c *Cull) Refresh(w *World, sc *Scene) error {
	const op = "world.Cull.Refresh"
	if c == nil {
		return core.InvalidArg(op, "cull")
	}
	if w == nil {
		return core.InvalidArg(op, "world")
	}
	t0 := time.Now()
	c.active = c.active[:0]
	c.entered = c.entered[:0]
	c.exited = c.exited[:0]
	c.candByID = c.candByID[:0]
	c.keys = c.keys[:0]
	c.snaps = c.snaps[:0]
	c.dropped = 0

	dead := 0
	vx, vy, vw, vh := c.view.X, c.view.Y, c.view.W, c.view.H
	emptyView := c.view.IsEmpty()
	for i := 0; i < len(c.items); i++ {
		e := c.items[i]
		n, ok := w.nodes[e.id]
		if !ok || e.id == NoEntity {
			dead++
			continue
		}
		var wx, wy float64
		if n.parent == NoEntity {
			wx, wy = n.local.Pos.X, n.local.Pos.Y
		} else if wt, ok := w.foldWorld(e.id); ok {
			wx, wy = wt.Pos.X, wt.Pos.Y
		} else {
			dead++
			continue
		}
		hit := e.awake
		if !hit && !emptyView {
			if e.hw == 0 && e.hh == 0 {
				hit = wx >= vx && wx < vx+vw && wy >= vy && wy < vy+vh
			} else {
				x1 := wx - e.hw
				if x1 < vx {
					x1 = vx
				}
				y1 := wy - e.hh
				if y1 < vy {
					y1 = vy
				}
				x2 := wx + e.hw
				if x2 > vx+vw {
					x2 = vx + vw
				}
				y2 := wy + e.hh
				if y2 > vy+vh {
					y2 = vy + vh
				}
				hit = x2 > x1 && y2 > y1
			}
		}
		if hit {
			c.candByID = append(c.candByID, e.id)
			c.keys = append(c.keys, cullKey{id: e.id, layer: e.layer, x: wx, y: wy + e.footDY, wy: wy, pinned: e.awake, parent: n.parent, seq: e.seq})
		}
	}
	if dead > 0 {
		kept := c.items[:0]
		for _, e := range c.items {
			if _, ok := w.foldWorld(e.id); ok {
				kept = append(kept, e)
			}
		}
		for i := range kept {
			c.index[kept[i].id] = i
		}
		for _, e := range c.items[len(kept):] {
			delete(c.index, e.id)
		}
		c.items = kept
		c.dropped = dead
	}
	sort.Sort(idOrder(c.candByID))
	c.entered, c.exited = diffSortedCull(c.candByID, c.prevCand, c.entered, c.exited)
	c.prevCand = append(c.prevCand[:0], c.candByID...)

	if sc != nil && c.autoSleep {
		for _, id := range c.candByID {
			if sc.IsSleeping(id) {
				_ = sc.Wake(id)
			}
		}
		for _, id := range c.exited {
			if !c.IsAwake(id) && sc.IsActive(id) {
				_ = sc.Sleep(id)
			}
		}
	}

	kept := c.keys[:0]
	for _, k := range c.keys {
		if sc == nil || sc.IsActive(k.id) || c.IsAwake(k.id) {
			kept = append(kept, k)
		}
	}
	c.keys = kept
	sort.Sort(keyOrder(c.keys))
	c.trimmed = 0
	if c.maxActive > 0 && len(c.keys) > c.maxActive {
		c.trimmed = len(c.keys) - c.maxActive
		c.keys = c.keys[:c.maxActive]
	}
	for _, k := range c.keys {
		c.active = append(c.active, k.id)
		c.snaps = append(c.snaps, ActiveSnap{ID: k.id, X: k.x, Y: k.wy, Layer: k.layer, Pinned: k.pinned, Parent: k.parent})
	}
	c.refreshes++
	c.lastUs = time.Since(t0).Microseconds()
	return nil
}

// Snapshot returns the update set with world spots in draw order, backed by
// the index scratch: valid until the next Refresh or Reset, never nil-safe
// to hold. It allocates nothing, so per-frame paint reads it instead of
// paying WorldOf per active.
func (c *Cull) Snapshot() []ActiveSnap {
	if c == nil || len(c.snaps) == 0 {
		return nil
	}
	return c.snaps
}

// idOrder sorts ids ascending without reflection: the hot path never pays
// for sort.Slice's interface dance.
type idOrder []ID

func (s idOrder) Len() int           { return len(s) }
func (s idOrder) Less(i, j int) bool { return s[i] < s[j] }
func (s idOrder) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// keyOrder sorts the draw order without reflection. seq is unique per
// tracked entity, so (layer, y, seq) is a total order: plain sort lands
// the same sequence on every machine, no tie drift.
type keyOrder []cullKey

func (s keyOrder) Len() int { return len(s) }
func (s keyOrder) Less(i, j int) bool {
	if s[i].layer != s[j].layer {
		return s[i].layer < s[j].layer
	}
	if s[i].y != s[j].y {
		return s[i].y < s[j].y
	}
	return s[i].seq < s[j].seq
}
func (s keyOrder) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// diffSortedCull diffs two id-sorted lists: entered holds ids only in cur,
// exited holds ids only in prev. out buffers are reused (cut to zero first
// by the caller) so the hot path stays off the heap.
func diffSortedCull(cur, prev, entered, exited []ID) ([]ID, []ID) {
	i, j := 0, 0
	for i < len(cur) && j < len(prev) {
		switch {
		case cur[i] < prev[j]:
			entered = append(entered, cur[i])
			i++
		case cur[i] > prev[j]:
			exited = append(exited, prev[j])
			j++
		default:
			i++
			j++
		}
	}
	for ; i < len(cur); i++ {
		entered = append(entered, cur[i])
	}
	for ; j < len(prev); j++ {
		exited = append(exited, prev[j])
	}
	return entered, exited
}

// Active returns the update set in draw order: layer ascending, then feet
// Y ascending, stable for ties. A fresh slice every call; nil on a nil
// index or an empty set.
func (c *Cull) Active() []ID {
	if c == nil || len(c.active) == 0 {
		return nil
	}
	return append([]ID(nil), c.active...)
}

// ActiveCount counts the update set, or 0 on a nil index.
func (c *Cull) ActiveCount() int {
	if c == nil {
		return 0
	}
	return len(c.active)
}

// Entered returns the ids that joined the visibility set since the last
// Refresh, in id order. A fresh slice every call.
func (c *Cull) Entered() []ID {
	if c == nil || len(c.entered) == 0 {
		return nil
	}
	return append([]ID(nil), c.entered...)
}

// Exited returns the ids that left the visibility set since the last
// Refresh, in id order. A fresh slice every call.
func (c *Cull) Exited() []ID {
	if c == nil || len(c.exited) == 0 {
		return nil
	}
	return append([]ID(nil), c.exited...)
}

// IsActive reports whether id sits in the current update set.
// Linear scan: call it for spot checks, not inside per-frame loops.
func (c *Cull) IsActive(id ID) bool {
	if c == nil {
		return false
	}
	for _, a := range c.active {
		if a == id {
			return true
		}
	}
	return false
}

// InView reports whether id sits in the current visibility set
// (in-view or pinned), even when it sleeps out of the update set.
func (c *Cull) InView(id ID) bool {
	if c == nil {
		return false
	}
	i, j := 0, len(c.candByID)-1
	for i <= j {
		m := (i + j) / 2
		switch {
		case c.candByID[m] < id:
			i = m + 1
		case c.candByID[m] > id:
			j = m - 1
		default:
			return true
		}
	}
	return false
}

// Count returns the tracked entities, or 0 on a nil index.
func (c *Cull) Count() int {
	if c == nil {
		return 0
	}
	return len(c.items)
}

// Dropped returns how many dead ids the last Refresh dropped, or 0.
func (c *Cull) Dropped() int {
	if c == nil {
		return 0
	}
	return c.dropped
}

// Trimmed returns how many actives the last Refresh cut past MaxActive.
func (c *Cull) Trimmed() int {
	if c == nil {
		return 0
	}
	return c.trimmed
}

// Refreshes counts every Refresh call here, including failed ones before
// the world check. Zero on a nil index.
func (c *Cull) Refreshes() int64 {
	if c == nil {
		return 0
	}
	return c.refreshes
}

// LastRefreshUs reports the last Refresh cost in microseconds: the full
// 20000 fold plus the active-only sort. Informational only; the gate
// timing is measured by the test and the window, never by this number.
func (c *Cull) LastRefreshUs() int64 {
	if c == nil {
		return 0
	}
	return c.lastUs
}

// Reset forgets every tracked entity, set, and counter. The view, the
// AutoSleep switch, and the MaxActive cap stay: a level switch re-tracks
// into the same frame.
func (c *Cull) Reset() {
	if c == nil {
		return
	}
	c.items = c.items[:0]
	c.index = map[ID]int{}
	c.active = c.active[:0]
	c.entered = c.entered[:0]
	c.exited = c.exited[:0]
	c.candByID = c.candByID[:0]
	c.prevCand = c.prevCand[:0]
	c.keys = c.keys[:0]
	c.snaps = c.snaps[:0]
	c.refreshes = 0
	c.dropped = 0
	c.trimmed = 0
	c.lastUs = 0
}

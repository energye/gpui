//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package physics

import (
	"math"
	"sort"
	"time"

	"github.com/energye/gpui/engine/core"
)

// Large-yard broad phase (S86, V3): uniform grid plus dynamic sleep plus a
// per-frame candidate cap.
//
// A thousand boxes checked pairwise costs 600000 narrow tests a frame.
// Broadphase buckets bodies into 128px cells and only tests pairs inside
// the same or neighboring cells, so the narrow phase sees hundreds, not
// hundreds of thousands. Sleeping dynamics stay collidable (a fast box
// still wakes them) but the caller skips integrating them; every contact
// with a sleeper is reported in Woke, and AutoWake wakes them at once.
//
// Godot mapping (only the approach is copied, no code is moved):
//   - CollisionObject layer/mask (G23): CanCollide bit math is reused
//     untouched; masked-out pairs never meet even in one cell.
//   - CharacterBody move_and_slide/snap/45-degree (G24): the caller still
//     steps with Step and resolves with Contact; this file only shortlists.
//   - RayCast segment cull (G26): Ray prefilters by the segment box, then
//     runs the same slab/quadratic narrow tests as CastRay.
//   - Body can_sleep/active: explicit Sleep/Wake plus displacement
//     auto-sleep; contact wakes (Woke list, optional AutoWake).
//   - Springboard has no Godot 2D source (scene/2d carries no spring
//     joint): Spring is filed as the engine-level single point, impulse
//     plus decay in one honest formula.
//
// Semantics borrowed from body.go unchanged: edge touch overlaps, origin
// inside reports Dist 0, ties keep the smaller track index, pairs run in
// track order. body.go/platform.go are untouched; this file only reads
// Body/Slope and drives nothing by itself.
//
// Errors: bad numbers or handles are InvalidArg, unknown handles are
// NotFound, past MaxBroadphaseBodies is OutOfMemory. Bad writes change
// nothing. Nil receivers never panic: getters park at zero, writers
// report InvalidArg.

const (
	// DefaultBroadphaseCell is the grid pitch in world units. 128px keeps
	// a 64px box inside 1-4 cells; the yard bodies are smaller.
	DefaultBroadphaseCell = 128.0
	// MaxBroadphaseBodies caps one index. Past it Track reports
	// OutOfMemory, never a half entry. The large yard holds 1200.
	MaxBroadphaseBodies = 8192
	// DefaultMaxCandidates caps pair evaluations per Query. Zero means
	// unlimited. 40 covers a 128px pile-up with margin.
	DefaultMaxCandidates = 40
	// DefaultSleepEps is the per-Update displacement under which a
	// dynamic counts as still, in world units.
	DefaultSleepEps = 0.5
	// DefaultSleepFrames stills in a row before a dynamic sleeps.
	DefaultSleepFrames = 60
)

// Spring is one jump pad: Impulse is the extra upward kick in world
// units per second, Damp is the decay (fraction of incoming falling
// speed kept, 0 dead pad, 1 perfectly bouncy). +Y runs down like the
// slopes, so falling means vy > 0 and the launch is negative.
type Spring struct {
	Impulse float64
	Damp    float64
}

// NewSpring builds a jump pad. Impulse must be finite and >= 0, Damp in
// [0,1], else InvalidArg.
func NewSpring(impulse, damp float64) (Spring, error) {
	const op = "physics.NewSpring"
	if !finite(impulse) || impulse < 0 {
		return Spring{}, core.InvalidArg(op, "impulse")
	}
	if !finite(damp) || damp < 0 || damp > 1 {
		return Spring{}, core.InvalidArg(op, "damp")
	}
	return Spring{Impulse: impulse, Damp: damp}, nil
}

// Valid reports whether s carries a usable pad.
func (s Spring) Valid() bool {
	return finite(s.Impulse) && s.Impulse >= 0 && finite(s.Damp) && s.Damp >= 0 && s.Damp <= 1
}

// Bounce fires the pad: a falling body (vy > 0) leaves with
// -(vy*Damp + Impulse); a rising or still body passes through untouched
// (top-only, like the one-way spirit). An invalid pad never fires and
// returns vy unchanged.
func (s Spring) Bounce(vy float64) float64 {
	if !s.Valid() || !finite(vy) || vy <= 0 {
		return vy
	}
	return -(vy*s.Damp + s.Impulse)
}

// bpEntry is one tracked body: the volume, the static flag, the sleep
// state, and the motion history driving auto-sleep.
type bpEntry struct {
	body   Body
	static bool
	asleep bool
	lastX  float64
	lastY  float64
	still  int
	alive  bool
}

// cellRef pairs one body slot with one grid cell key for the sort pass.
type cellRef struct {
	key  int64
	slot int
}

// pairRef is one candidate body pair in slot order (a < b always).
type pairRef struct {
	a int
	b int
}

// Broadphase is one yard index over tracked bodies. The caller owns
// placement truth through Track/Move; Refresh-free reads never drift
// because every Query rebuilds from the stored volumes when dirty.
type Broadphase struct {
	cell       float64
	entries    []bpEntry
	free       []int
	maxCand    int
	autoWake   bool
	autoSleep  bool
	sleepEps   float64
	sleepFrame int
	dirty      bool
	pairsStale bool
	refs       []cellRef
	runKeys    []int64
	runStarts  []int
	pairs      []pairRef
	cands      []int
	active     []Contact
	woke       []int
	queries    int64
	evals      int
	trimmed    int
	lastUs     int64
}

func bpCellKey(cx, cy int) int64 {
	return int64(cx)<<32 ^ int64(int32(cy))
}

func bpCellOf(v, cell float64) int {
	return int(math.Floor(v / cell))
}

// NewBroadphase builds a yard index over 128px cells. Auto-sleep starts
// on with the default stillness (0.5 units, 60 frames); AutoWake starts
// on so contacted sleepers rejoin at once. The cap starts at 40.
func NewBroadphase() Broadphase {
	return Broadphase{
		cell:       DefaultBroadphaseCell,
		maxCand:    DefaultMaxCandidates,
		autoWake:   true,
		autoSleep:  true,
		sleepEps:   DefaultSleepEps,
		sleepFrame: DefaultSleepFrames,
		dirty:      true,
		pairsStale: true,
	}
}

// Cell returns the grid pitch, or 0 on a nil index.
func (b *Broadphase) Cell() float64 {
	if b == nil {
		return 0
	}
	return b.cell
}

// SetAutoWake flips contact wake: with it on, Query wakes contacted
// sleepers before returning; with it off, they stay asleep and only
// appear in Woke for the caller to wake.
func (b *Broadphase) SetAutoWake(on bool) {
	if b == nil {
		return
	}
	b.autoWake = on
}

// AutoWake reports the contact-wake switch, or false on nil.
func (b *Broadphase) AutoWake() bool {
	if b == nil {
		return false
	}
	return b.autoWake
}

// SetAutoSleep flips stillness sleep. With it off, only explicit
// Sleep parks dynamics.
func (b *Broadphase) SetAutoSleep(on bool) {
	if b == nil {
		return
	}
	b.autoSleep = on
}

// SetSleep tunes stillness sleep: displacement under eps for frames in
// a row parks a dynamic. eps must be finite and >= 0, frames >= 1, else
// InvalidArg keeps the old tuning.
func (b *Broadphase) SetSleep(eps float64, frames int) error {
	const op = "physics.Broadphase.SetSleep"
	if b == nil {
		return core.InvalidArg(op, "broadphase")
	}
	if !finite(eps) || eps < 0 || frames < 1 {
		return core.InvalidArg(op, "tuning")
	}
	b.sleepEps, b.sleepFrame = eps, frames
	return nil
}

// SetMaxCandidates caps pair evaluations per Query: past n pairs Query
// keeps the first n in track order and counts the rest in Trimmed. Zero
// means unlimited. Negative is InvalidArg and keeps the old cap.
func (b *Broadphase) SetMaxCandidates(n int) error {
	const op = "physics.Broadphase.SetMaxCandidates"
	if b == nil {
		return core.InvalidArg(op, "broadphase")
	}
	if n < 0 {
		return core.InvalidArg(op, "max")
	}
	b.maxCand = n
	return nil
}

// MaxCandidates returns the evaluation cap (0 is unlimited), or 0 on nil.
func (b *Broadphase) MaxCandidates() int {
	if b == nil {
		return 0
	}
	return b.maxCand
}

// Track registers a body and returns its stable handle. The body must be
// valid, else InvalidArg; past MaxBroadphaseBodies is OutOfMemory.
// static bodies never auto-sleep. Handles stay stable across Remove
// (slots recycle through a free list, never by swap).
func (b *Broadphase) Track(body Body, static bool) (int, error) {
	const op = "physics.Broadphase.Track"
	if b == nil {
		return -1, core.InvalidArg(op, "broadphase")
	}
	if !body.Valid() {
		return -1, core.InvalidArg(op, "body")
	}
	var h int
	if len(b.free) > 0 {
		h = b.free[len(b.free)-1]
		b.free = b.free[:len(b.free)-1]
		b.entries[h] = bpEntry{body: body, static: static, lastX: body.Pos.X, lastY: body.Pos.Y, alive: true}
	} else {
		live := 0
		for i := range b.entries {
			if b.entries[i].alive {
				live++
			}
		}
		if live >= MaxBroadphaseBodies {
			return -1, core.OutOfMemory(op, "bodies")
		}
		h = len(b.entries)
		b.entries = append(b.entries, bpEntry{body: body, static: static, lastX: body.Pos.X, lastY: body.Pos.Y, alive: true})
	}
	b.dirty = true
	b.pairsStale = true
	return h, nil
}

func (b *Broadphase) at(h int) (*bpEntry, bool) {
	if b == nil || h < 0 || h >= len(b.entries) || !b.entries[h].alive {
		return nil, false
	}
	return &b.entries[h], true
}

// Move rewrites the volume at h and wakes it: a repositioned body always
// rejoins. Unknown handles are NotFound; invalid bodies change nothing.
func (b *Broadphase) Move(h int, body Body) error {
	const op = "physics.Broadphase.Move"
	e, ok := b.at(h)
	if !ok {
		if b == nil {
			return core.InvalidArg(op, "broadphase")
		}
		return core.NotFound(op, "body")
	}
	if !body.Valid() {
		return core.InvalidArg(op, "body")
	}
	e.body = body
	e.asleep = false
	e.still = 0
	e.lastX, e.lastY = body.Pos.X, body.Pos.Y
	b.dirty = true
	b.pairsStale = true
	return nil
}

// Remove forgets h. Unknown handles are NotFound; the slot recycles.
func (b *Broadphase) Remove(h int) error {
	const op = "physics.Broadphase.Remove"
	e, ok := b.at(h)
	if !ok {
		if b == nil {
			return core.InvalidArg(op, "broadphase")
		}
		return core.NotFound(op, "body")
	}
	*e = bpEntry{}
	b.free = append(b.free, h)
	b.dirty = true
	b.pairsStale = true
	return nil
}

// SetStatic flips the static flag. Unknown handles are NotFound.
func (b *Broadphase) SetStatic(h int, static bool) error {
	const op = "physics.Broadphase.SetStatic"
	e, ok := b.at(h)
	if !ok {
		if b == nil {
			return core.InvalidArg(op, "broadphase")
		}
		return core.NotFound(op, "body")
	}
	e.static = static
	if static {
		e.asleep = false
		e.still = 0
	}
	b.dirty = true
	b.pairsStale = true
	return nil
}

// Sleep parks a dynamic; statics refuse with InvalidArg (they never
// sleep, they simply never move). Unknown handles are NotFound.
func (b *Broadphase) Sleep(h int) error {
	const op = "physics.Broadphase.Sleep"
	e, ok := b.at(h)
	if !ok {
		if b == nil {
			return core.InvalidArg(op, "broadphase")
		}
		return core.NotFound(op, "body")
	}
	if e.static {
		return core.InvalidArg(op, "static")
	}
	e.asleep = true
	return nil
}

// Wake rejoins h. Unknown handles are NotFound. Waking also resyncs the
// stillness baseline so a woken body never parks on its first Update.
func (b *Broadphase) Wake(h int) error {
	const op = "physics.Broadphase.Wake"
	e, ok := b.at(h)
	if !ok {
		if b == nil {
			return core.InvalidArg(op, "broadphase")
		}
		return core.NotFound(op, "body")
	}
	e.asleep = false
	e.still = 0
	e.lastX, e.lastY = e.body.Pos.X, e.body.Pos.Y
	return nil
}

// IsActive reports whether h is tracked and awake. Unknown and nil ids
// report false.
func (b *Broadphase) IsActive(h int) bool {
	e, ok := b.at(h)
	if !ok {
		return false
	}
	return !e.asleep
}

// Count returns live tracked bodies, or 0 on nil.
func (b *Broadphase) Count() int {
	if b == nil {
		return 0
	}
	n := 0
	for i := range b.entries {
		if b.entries[i].alive {
			n++
		}
	}
	return n
}

// SleepingCount counts parked dynamics, or 0 on nil.
func (b *Broadphase) SleepingCount() int {
	if b == nil {
		return 0
	}
	n := 0
	for i := range b.entries {
		if b.entries[i].alive && b.entries[i].asleep {
			n++
		}
	}
	return n
}

// Update steps stillness sleep without querying: dynamics displaced under
// eps since the last Update add a still frame and park at the tuned
// count; anything that moved restarts. Statics never park.
func (b *Broadphase) Update() {
	if b == nil {
		return
	}
	if !b.autoSleep {
		return
	}
	for i := range b.entries {
		e := &b.entries[i]
		if !e.alive || e.static || e.asleep {
			continue
		}
		if math.Abs(e.body.Pos.X-e.lastX) < b.sleepEps && math.Abs(e.body.Pos.Y-e.lastY) < b.sleepEps {
			e.still++
			if e.still >= b.sleepFrame {
				e.asleep = true
			}
		} else {
			e.still = 0
			e.lastX, e.lastY = e.body.Pos.X, e.body.Pos.Y
		}
	}
}

// rebuild buckets every live body into its overlapped cells. No maps on
// the hot path: one scratch array plus a concrete sort.
func (b *Broadphase) rebuild() {
	b.refs = b.refs[:0]
	for h := range b.entries {
		e := &b.entries[h]
		if !e.alive {
			continue
		}
		r, ok := Bounds(e.body)
		if !ok {
			continue
		}
		x0 := bpCellOf(r.X, b.cell)
		x1 := bpCellOf(r.X+r.W, b.cell)
		y0 := bpCellOf(r.Y, b.cell)
		y1 := bpCellOf(r.Y+r.H, b.cell)
		for cx := x0; cx <= x1; cx++ {
			for cy := y0; cy <= y1; cy++ {
				b.refs = append(b.refs, cellRef{key: bpCellKey(cx, cy), slot: h})
			}
		}
	}
	sort.Sort(cellOrder(b.refs))
	b.runKeys = b.runKeys[:0]
	b.runStarts = b.runStarts[:0]
	for i := 0; i < len(b.refs); {
		j := i
		for j < len(b.refs) && b.refs[j].key == b.refs[i].key {
			j++
		}
		b.runKeys = append(b.runKeys, b.refs[i].key)
		b.runStarts = append(b.runStarts, i)
		i = j
	}
}

type cellOrder []cellRef

func (s cellOrder) Len() int { return len(s) }
func (s cellOrder) Less(i, j int) bool {
	if s[i].key != s[j].key {
		return s[i].key < s[j].key
	}
	return s[i].slot < s[j].slot
}
func (s cellOrder) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

type pairOrder []pairRef

func (s pairOrder) Len() int { return len(s) }
func (s pairOrder) Less(i, j int) bool {
	if s[i].a != s[j].a {
		return s[i].a < s[j].a
	}
	return s[i].b < s[j].b
}
func (s pairOrder) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// collect emits every body pair sharing a cell, then sorts and uniques:
// the grid only shortlists, track order decides evaluation. Neighbor
// cells join through box span: a body registers in every cell its bounds
// touch, so a border-straddling pair always shares at least one cell and
// no offset table is needed.
func (b *Broadphase) collect() {
	b.pairs = b.pairs[:0]
	n := len(b.refs)
	i := 0
	for i < n {
		j := i
		for j < n && b.refs[j].key == b.refs[i].key {
			j++
		}
		for a := i; a < j; a++ {
			for c := a + 1; c < j; c++ {
				sa, sb := b.refs[a].slot, b.refs[c].slot
				if sa > sb {
					sa, sb = sb, sa
				}
				if sa != sb {
					b.pairs = append(b.pairs, pairRef{a: sa, b: sb})
				}
			}
		}
		i = j
	}
	sort.Sort(pairOrder(b.pairs))
	kept := b.pairs[:0]
	for k, p := range b.pairs {
		if k == 0 || p != b.pairs[k-1] {
			kept = append(kept, p)
		}
	}
	b.pairs = kept
}

// fastCollide is the hot-path narrow test: entries are already
// validated at Track/Move, so box-box pairs skip the per-pair Valid
// walk and run the edge-included extents check inline. Mask math runs
// first so masked-out pairs never touch floats. Non-box pairs fall
// back to Overlaps, which keeps the exact edge rule.
func fastCollide(ea, eb *bpEntry) bool {
	a := ea.body
	c := eb.body
	if a.Layer&c.Mask == 0 || c.Layer&a.Mask == 0 {
		return false
	}
	if a.Shape == ShapeBox && c.Shape == ShapeBox {
		dx := a.Pos.X - c.Pos.X
		if dx < 0 {
			dx = -dx
		}
		dy := a.Pos.Y - c.Pos.Y
		if dy < 0 {
			dy = -dy
		}
		return dx <= a.Half.X+c.Half.X && dy <= a.Half.Y+c.Half.Y
	}
	return Overlaps(a, c)
}

// Query returns every touching pair with matching masks in track order,
// shortlisted by the grid: identical pairs and order to Query over the
// same bodies when nothing sleeps and no cap applies. The pair list is
// cached while clean (Godot update() semantics): Track/Move/Remove/
// SetStatic mark dirty and rebuild once, steady frames reuse pairs and
// pay narrow tests only. Evaluations count
// every grid pair tested; the cap trims contacts only, never evaluations,
// so Trimmed plus contacts always reconcile with the pair count.
// stay collidable and land in Woke (auto-woken with AutoWake on). Past
// MaxCandidates Query keeps the first contacts and counts Trimmed; the
// wake scan only sees kept contacts, documented and deterministic.
func (b *Broadphase) Query() ([]Contact, error) {
	const op = "physics.Broadphase.Query"
	if b == nil {
		return nil, core.InvalidArg(op, "broadphase")
	}
	t0 := time.Now()
	if b.dirty {
		b.rebuild()
		b.dirty = false
		b.pairsStale = true
	}
	if b.pairsStale {
		b.collect()
		b.pairsStale = false
	}
	b.active = b.active[:0]
	b.woke = b.woke[:0]
	b.evals = 0
	b.trimmed = 0
	for _, p := range b.pairs {
		ea := &b.entries[p.a]
		eb := &b.entries[p.b]
		b.evals++
		if !fastCollide(ea, eb) {
			continue
		}
		if b.maxCand > 0 && len(b.active) >= b.maxCand {
			b.trimmed++
			continue
		}
		b.active = append(b.active, Contact{AI: p.a, BI: p.b, A: ea.body.Name, B: eb.body.Name, Trigger: ea.body.Trigger || eb.body.Trigger})
		for _, h := range []int{p.a, p.b} {
			e := &b.entries[h]
			if e.alive && e.asleep && !e.static {
				dup := false
				for _, w := range b.woke {
					if w == h {
						dup = true
						break
					}
				}
				if !dup {
					b.woke = append(b.woke, h)
				}
				if b.autoWake {
					e.asleep = false
					e.still = 0
				}
			}
		}
	}
	b.queries++
	b.lastUs = time.Since(t0).Microseconds()
	if len(b.active) == 0 {
		return nil, nil
	}
	return append([]Contact(nil), b.active...), nil
}

// Woke returns the handles parked-asleep but contacted by the last
// Query, in first-contact order. A fresh slice every call.
func (b *Broadphase) Woke() []int {
	if b == nil || len(b.woke) == 0 {
		return nil
	}
	return append([]int(nil), b.woke...)
}

// Evaluations returns the pair tests the last Query ran, or 0 on nil.
func (b *Broadphase) Evaluations() int {
	if b == nil {
		return 0
	}
	return b.evals
}

// Trimmed returns pairs the last Query dropped past MaxCandidates.
func (b *Broadphase) Trimmed() int {
	if b == nil {
		return 0
	}
	return b.trimmed
}

// Queries counts every Query call, or 0 on nil.
func (b *Broadphase) Queries() int64 {
	if b == nil {
		return 0
	}
	return b.queries
}

// LastQueryUs reports the last Query cost in microseconds, informational
// only; gates are measured by the test and the window, never here.
func (b *Broadphase) LastQueryUs() int64 {
	if b == nil {
		return 0
	}
	return b.lastUs
}

// QueryRegion returns the handles whose bounds touch rect, in ascending
// handle order: the Godot cull_aabb single query. Only the touched cells
// are walked, so one viewport lookup costs microseconds, not a yard scan.
// Empty or non-finite rects are InvalidArg; a clean miss returns nil.
func (b *Broadphase) QueryRegion(rect core.Rect) ([]int, error) {
	const op = "physics.Broadphase.QueryRegion"
	if b == nil {
		return nil, core.InvalidArg(op, "broadphase")
	}
	if rect.IsEmpty() || !finite(rect.X) || !finite(rect.Y) || !finite(rect.W) || !finite(rect.H) {
		return nil, core.InvalidArg(op, "rect")
	}
	if b.dirty {
		b.rebuild()
		b.dirty = false
	}
	x0 := bpCellOf(rect.X, b.cell)
	x1 := bpCellOf(rect.X+rect.W, b.cell)
	y0 := bpCellOf(rect.Y, b.cell)
	y1 := bpCellOf(rect.Y+rect.H, b.cell)
	b.cands = b.cands[:0]
	for cx := x0; cx <= x1; cx++ {
		for cy := y0; cy <= y1; cy++ {
			k := bpCellKey(cx, cy)
			lo, hi := 0, len(b.runKeys)
			for lo < hi {
				m := (lo + hi) / 2
				if b.runKeys[m] < k {
					lo = m + 1
				} else {
					hi = m
				}
			}
			if lo >= len(b.runKeys) || b.runKeys[lo] != k {
				continue
			}
			end := len(b.refs)
			if lo+1 < len(b.runStarts) {
				end = b.runStarts[lo+1]
			}
			for r := b.runStarts[lo]; r < end; r++ {
				b.cands = append(b.cands, b.refs[r].slot)
			}
		}
	}
	sort.Ints(b.cands)
	uniq := b.cands[:0]
	for i, s := range b.cands {
		if i == 0 || s != b.cands[i-1] {
			uniq = append(uniq, s)
		}
	}
	b.cands = uniq
	var out []int
	rx1 := rect.X + rect.W
	ry1 := rect.Y + rect.H
	for _, h := range b.cands {
		e := &b.entries[h]
		if !e.alive {
			continue
		}
		// Fast bounds: entries are validated at Track/Move, boxes inline
		// the extents without the per-body Valid walk or Rect alloc.
		// Same edge-included rule as Bounds.
		bd := e.body
		var bx0, by0, bx1, by1 float64
		if bd.Shape == ShapeBox {
			bx0, by0 = bd.Pos.X-bd.Half.X, bd.Pos.Y-bd.Half.Y
			bx1, by1 = bd.Pos.X+bd.Half.X, bd.Pos.Y+bd.Half.Y
		} else {
			r, ok := Bounds(bd)
			if !ok {
				continue
			}
			bx0, by0, bx1, by1 = r.X, r.Y, r.X+r.W, r.Y+r.H
		}
		if bx0 > rx1 || bx1 < rect.X || by0 > ry1 || by1 < rect.Y {
			continue
		}
		out = append(out, h)
	}
	return out, nil
}

// Ray casts through the grid: bodies whose bounds miss the segment box
// are skipped, the rest run the same slab/quadratic narrow tests as
// CastRay with the same nearest-plus-smallest-index rule. Sleeping
// bodies still block rays. Invalid rays are InvalidArg, matching CastRay.
func (b *Broadphase) Ray(ray Ray) (Hit, bool, error) {
	const op = "physics.Broadphase.Ray"
	if b == nil {
		return Hit{}, false, core.InvalidArg(op, "broadphase")
	}
	if !ray.Valid() {
		return Hit{}, false, core.InvalidArg(op, "ray")
	}
	dir := rayDirNorm(ray.Dir)
	if dir.IsZero() {
		return Hit{}, false, core.InvalidArg(op, "ray")
	}
	if ray.Mask == 0 {
		return Hit{}, false, nil
	}
	ex := ray.Origin.X + dir.X*ray.MaxDist
	ey := ray.Origin.Y + dir.Y*ray.MaxDist
	lox, hix := ray.Origin.X, ex
	if lox > hix {
		lox, hix = hix, lox
	}
	loy, hiy := ray.Origin.Y, ey
	if loy > hiy {
		loy, hiy = hiy, loy
	}
	if b.dirty {
		b.rebuild()
		b.dirty = false
	}
	// Segment-box cell walk: only bodies in touched cells run narrow.
	// Runs are key-sorted, so binary search finds each cell with no maps.
	b.cands = b.cands[:0]
	x0 := bpCellOf(lox, b.cell)
	x1 := bpCellOf(hix, b.cell)
	y0 := bpCellOf(loy, b.cell)
	y1 := bpCellOf(hiy, b.cell)
	for cx := x0; cx <= x1; cx++ {
		for cy := y0; cy <= y1; cy++ {
			k := bpCellKey(cx, cy)
			lo, hi := 0, len(b.runKeys)
			for lo < hi {
				m := (lo + hi) / 2
				if b.runKeys[m] < k {
					lo = m + 1
				} else {
					hi = m
				}
			}
			if lo >= len(b.runKeys) || b.runKeys[lo] != k {
				continue
			}
			end := len(b.refs)
			if lo+1 < len(b.runStarts) {
				end = b.runStarts[lo+1]
			}
			for r := b.runStarts[lo]; r < end; r++ {
				b.cands = append(b.cands, b.refs[r].slot)
			}
		}
	}
	sort.Ints(b.cands)
	uniq := b.cands[:0]
	for i, s := range b.cands {
		if i == 0 || s != b.cands[i-1] {
			uniq = append(uniq, s)
		}
	}
	b.cands = uniq
	best := math.Inf(1)
	bestH := -1
	var bestN core.Vec2
	for _, h := range b.cands {
		e := &b.entries[h]
		if !e.alive {
			continue
		}
		if e.body.Layer&ray.Mask == 0 {
			continue
		}
		var t float64
		var nrm core.Vec2
		var hit bool
		if e.body.Shape == ShapeCircle {
			t, nrm, hit = rayHitCircle(ray.Origin, dir, ray.MaxDist, e.body)
		} else {
			t, nrm, hit = rayHitBox(ray.Origin, dir, ray.MaxDist, e.body)
		}
		if !hit || !finite(t) || t < 0 || t > ray.MaxDist {
			continue
		}
		if t < best {
			best, bestH, bestN = t, h, nrm
		}
	}
	if bestH < 0 {
		return Hit{}, false, nil
	}
	e := &b.entries[bestH]
	return Hit{Index: bestH, Name: e.body.Name, Dist: best, Pos: core.V2(ray.Origin.X+dir.X*best, ray.Origin.Y+dir.Y*best), Normal: bestN, Trigger: e.body.Trigger}, true, nil
}

// Reset forgets every tracked body, set, and counter. Cell, cap, sleep
// tuning, and wake switches stay: a yard switch re-tracks into the same
// frame.
func (b *Broadphase) Reset() {
	if b == nil {
		return
	}
	b.entries = b.entries[:0]
	b.free = b.free[:0]
	b.refs = b.refs[:0]
	b.runKeys = b.runKeys[:0]
	b.runStarts = b.runStarts[:0]
	b.pairs = b.pairs[:0]
	b.cands = b.cands[:0]
	b.active = b.active[:0]
	b.woke = b.woke[:0]
	b.queries = 0
	b.evals = 0
	b.trimmed = 0
	b.lastUs = 0
	b.dirty = true
	b.pairsStale = true
}

//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package nav

import (
	"container/heap"
	"math"

	"github.com/energye/gpui/engine/core"
)

// Agent motion numbers (S87 truth): speed 200 world units/s matches the
// navigation demo feel, arrival tolerance 2 units is desired_distance.
const (
	DefaultSpeed = 200.0
	ArriveTol    = 2.0
)

// Step costs: orthogonal 1, diagonal sqrt(2) (AStarGrid2D weight feel).
var (
	costStraight = 1.0
	costDiagonal = math.Sqrt2
)

// Cell is one grid address.
type Cell struct {
	X, Y int
}

// Status is the terminal outcome of one path query: every click ends in
// exactly one of these, never a hang.
type Status int

const (
	// StatusInvalid means the query itself was malformed (bad grid or
	// out-of-bounds cell).
	StatusInvalid Status = iota
	// StatusFound means a walkable cell chain reached the goal.
	StatusFound
	// StatusNoPath means both endpoints are legal but no walkable chain
	// connects them.
	StatusNoPath
	// StatusStartBlocked means the start cell is solid.
	StatusStartBlocked
	// StatusGoalBlocked means the goal cell is solid.
	StatusGoalBlocked
)

// String returns the stable status name used in JSON and testdata.
func (s Status) String() string {
	switch s {
	case StatusFound:
		return "found"
	case StatusNoPath:
		return "no_path"
	case StatusStartBlocked:
		return "start_blocked"
	case StatusGoalBlocked:
		return "goal_blocked"
	default:
		return "invalid"
	}
}

// ParseStatus maps a testdata want string to a Status.
func ParseStatus(name string) (Status, bool) {
	switch name {
	case "found":
		return StatusFound, true
	case "no_path":
		return StatusNoPath, true
	case "start_blocked":
		return StatusStartBlocked, true
	case "goal_blocked":
		return StatusGoalBlocked, true
	case "invalid":
		return StatusInvalid, true
	default:
		return StatusInvalid, false
	}
}

// octile is the admissible 8-way heuristic: max + (sqrt2-1)*min.
func octile(dx, dy int) float64 {
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	mn, mx := dx, dy
	if mn > mx {
		mn, mx = mx, mn
	}
	return float64(mx) + (math.Sqrt2-1)*float64(mn)
}

// openNode is one A* frontier entry.
type openNode struct {
	idx int
	f   float64
	h   float64
}

// openHeap is the deterministic frontier: smaller f first, then smaller
// h, then smaller cell index (top rows, then left columns win ties, so
// the same grid always yields the same path).
type openHeap []openNode

func (h openHeap) Len() int { return len(h) }
func (h openHeap) Less(i, j int) bool {
	if h[i].f != h[j].f {
		return h[i].f < h[j].f
	}
	if h[i].h != h[j].h {
		return h[i].h < h[j].h
	}
	return h[i].idx < h[j].idx
}
func (h openHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *openHeap) Push(x any)   { *h = append(*h, x.(openNode)) }
func (h *openHeap) Pop() any {
	old := *h
	n := len(old) - 1
	v := old[n]
	*h = old[:n]
	return v
}

// stepDir is one of the 8 fixed expansion directions (deterministic order:
// 4 straight first, then 4 diagonals).
type stepDir struct {
	dx, dy int
	diag   bool
	cost   float64
}

func stepDirs() [8]stepDir {
	return [8]stepDir{
		{1, 0, false, costStraight},
		{-1, 0, false, costStraight},
		{0, 1, false, costStraight},
		{0, -1, false, costStraight},
		{1, 1, true, costDiagonal},
		{-1, 1, true, costDiagonal},
		{1, -1, true, costDiagonal},
		{-1, -1, true, costDiagonal},
	}
}

// FindPath runs A* over g from start to goal (both inclusive in the
// returned chain). 8-way with corner-cut forbidden: a diagonal step is
// allowed only when both orthogonal neighbours are walkable, so paths
// never slip through wall corners (Godot AStarGrid2D default feel).
//
// Errors: invalid grids or out-of-bounds endpoints are InvalidArg with
// StatusInvalid and a nil path. Solid endpoints are InvalidArg with
// StatusStartBlocked/StatusGoalBlocked and a nil path (terminal, no
// retry). Legal-but-disconnected endpoints return StatusNoPath with a
// nil error (terminal, not a crash).
func FindPath(g Grid, start, goal Cell) ([]Cell, Status, error) {
	const op = "nav.FindPath"
	if !g.Valid() {
		return nil, StatusInvalid, core.InvalidArg(op, "grid")
	}
	if !g.InBounds(start.X, start.Y) || !g.InBounds(goal.X, goal.Y) {
		return nil, StatusInvalid, core.InvalidArg(op, "cell")
	}
	if g.solid[start.Y*g.w+start.X] {
		return nil, StatusStartBlocked, core.InvalidArg(op, "start")
	}
	if g.solid[goal.Y*g.w+goal.X] {
		return nil, StatusGoalBlocked, core.InvalidArg(op, "goal")
	}
	si := start.Y*g.w + start.X
	gi := goal.Y*g.w + goal.X
	if si == gi {
		return []Cell{start}, StatusFound, nil
	}
	n := g.w * g.h
	gScore := make([]float64, n)
	parent := make([]int, n)
	closed := make([]bool, n)
	for i := range gScore {
		gScore[i] = math.Inf(1)
		parent[i] = -1
	}
	gScore[si] = 0
	open := &openHeap{{idx: si, f: octile(goal.X-start.X, goal.Y-start.Y), h: octile(goal.X-start.X, goal.Y-start.Y)}}
	heap.Init(open)
	dirs := stepDirs()
	for open.Len() > 0 {
		cur := heap.Pop(open).(openNode).idx
		if closed[cur] {
			continue
		}
		if cur == gi {
			chain := []int{gi}
			for p := parent[gi]; p >= 0; p = parent[chain[len(chain)-1]] {
				chain = append(chain, p)
				if p == si {
					break
				}
			}
			out := make([]Cell, len(chain))
			for i, idx := range chain {
				out[len(chain)-1-i] = Cell{X: idx % g.w, Y: idx / g.w}
			}
			return out, StatusFound, nil
		}
		closed[cur] = true
		cx, cy := cur%g.w, cur/g.w
		for _, d := range dirs {
			nx, ny := cx+d.dx, cy+d.dy
			if nx < 0 || ny < 0 || nx >= g.w || ny >= g.h {
				continue
			}
			ni := ny*g.w + nx
			if closed[ni] || g.solid[ni] {
				continue
			}
			if d.diag {
				// Corner rule: both edge-adjacent cells must be walkable.
				o1x, o1y := cx+d.dx, cy
				o2x, o2y := cx, cy+d.dy
				if g.solid[o1y*g.w+o1x] || g.solid[o2y*g.w+o2x] {
					continue
				}
			}
			tent := gScore[cur] + d.cost
			if tent < gScore[ni] {
				gScore[ni] = tent
				parent[ni] = cur
				h := octile(goal.X-nx, goal.Y-ny)
				heap.Push(open, openNode{idx: ni, f: tent + h, h: h})
			}
		}
	}
	return nil, StatusNoPath, nil
}

// CellToWorld maps a cell to its center in world units: origin is the
// top-left of cell (0,0), cellSize scales one cell edge.
func CellToWorld(c Cell, cellSize float64, origin core.Vec2) core.Vec2 {
	return core.V2(origin.X+(float64(c.X)+0.5)*cellSize, origin.Y+(float64(c.Y)+0.5)*cellSize)
}

// WorldToCell maps a world point to its owning cell. Out-of-grid points
// report ok=false.
func WorldToCell(p core.Vec2, cellSize float64, origin core.Vec2, w, h int) (Cell, bool) {
	if !(cellSize > 0) || w < 1 || h < 1 {
		return Cell{}, false
	}
	fx := (p.X - origin.X) / cellSize
	fy := (p.Y - origin.Y) / cellSize
	x, y := int(math.Floor(fx)), int(math.Floor(fy))
	if x < 0 || y < 0 || x >= w || y >= h {
		return Cell{}, false
	}
	return Cell{X: x, Y: y}, true
}

// CellsToWorld converts a cell chain to world waypoints (cell centers).
func CellsToWorld(cells []Cell, cellSize float64, origin core.Vec2) []core.Vec2 {
	out := make([]core.Vec2, len(cells))
	for i, c := range cells {
		out[i] = CellToWorld(c, cellSize, origin)
	}
	return out
}

func finite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// Agent walks one waypoint list at a fixed speed: Pos is the live spot,
// Next is the next_path cursor into Waypoints, Done is the arrived
// terminal (within ArriveTol of the final point). The caller owns grid
// truth via FindPath; Agent only integrates motion.
type Agent struct {
	pos       core.Vec2
	speed     float64
	waypoints []core.Vec2
	next      int
	done      bool
}

// NewAgent builds an agent at pos walking at speed units/s. Pos must be
// finite, speed must be finite and > 0 (DefaultSpeed 200 matches the
// navigation demo), else InvalidArg.
func NewAgent(pos core.Vec2, speed float64) (Agent, error) {
	const op = "nav.NewAgent"
	if !finite(pos.X) || !finite(pos.Y) {
		return Agent{}, core.InvalidArg(op, "pos")
	}
	if !finite(speed) || speed <= 0 {
		return Agent{}, core.InvalidArg(op, "speed")
	}
	return Agent{pos: pos, speed: speed, done: true}, nil
}

// Pos returns the live spot (zero on nil).
func (a *Agent) Pos() core.Vec2 {
	if a == nil {
		return core.Vec2{}
	}
	return a.pos
}

// Speed returns the walk speed, or 0 on nil.
func (a *Agent) Speed() float64 {
	if a == nil {
		return 0
	}
	return a.speed
}

// Done reports whether the agent rests at its terminal (empty path counts
// as done). Nil reports done.
func (a *Agent) Done() bool {
	if a == nil {
		return true
	}
	return a.done
}

// Remaining counts waypoints not yet reached, or 0 on nil/done.
func (a *Agent) Remaining() int {
	if a == nil || a.done {
		return 0
	}
	return len(a.waypoints) - a.next
}

// Waypoints returns a fresh copy of the waypoint list.
func (a *Agent) Waypoints() []core.Vec2 {
	if a == nil || len(a.waypoints) == 0 {
		return nil
	}
	return append([]core.Vec2(nil), a.waypoints...)
}

// SetWaypoints installs a new route: the agent restarts from its current
// spot toward pts[0]. Empty/nil pts park the agent at done. Any
// non-finite point is InvalidArg and keeps the old route.
func (a *Agent) SetWaypoints(pts []core.Vec2) error {
	const op = "nav.Agent.SetWaypoints"
	if a == nil {
		return core.InvalidArg(op, "agent")
	}
	for _, p := range pts {
		if !finite(p.X) || !finite(p.Y) {
			return core.InvalidArg(op, "waypoint")
		}
	}
	a.waypoints = append([]core.Vec2(nil), pts...)
	a.next = 0
	a.done = len(a.waypoints) == 0
	return nil
}

// SetCells installs a cell chain as world waypoints (cell centers).
// Empty chains park at done; bad cellSize is InvalidArg.
func (a *Agent) SetCells(cells []Cell, cellSize float64, origin core.Vec2) error {
	const op = "nav.Agent.SetCells"
	if a == nil {
		return core.InvalidArg(op, "agent")
	}
	if len(cells) == 0 {
		a.waypoints = nil
		a.next = 0
		a.done = true
		return nil
	}
	if !(cellSize > 0) || !finite(cellSize) || !finite(origin.X) || !finite(origin.Y) {
		return core.InvalidArg(op, "mapping")
	}
	a.waypoints = CellsToWorld(cells, cellSize, origin)
	a.next = 0
	a.done = false
	return nil
}

// Step advances toward the current waypoint by speed*dt: reaching within
// ArriveTol (2 units) consumes the waypoint; consuming the last one parks
// at done and reports arrived=true. dt must be finite and >= 0, else
// InvalidArg keeps the agent still. Nil receivers report InvalidArg.
func (a *Agent) Step(dt float64) (bool, error) {
	const op = "nav.Agent.Step"
	if a == nil {
		return false, core.InvalidArg(op, "agent")
	}
	if !finite(dt) || dt < 0 {
		return false, core.InvalidArg(op, "dt")
	}
	if a.done || a.next >= len(a.waypoints) {
		a.done = true
		return true, nil
	}
	target := a.waypoints[a.next]
	dx, dy := target.X-a.pos.X, target.Y-a.pos.Y
	dist := math.Hypot(dx, dy)
	if dist <= ArriveTol {
		a.next++
		if a.next >= len(a.waypoints) {
			a.pos = target
			a.done = true
			return true, nil
		}
		return false, nil
	}
	travel := a.speed * dt
	if travel >= dist-ArriveTol {
		// Snap within tolerance instead of orbiting the point.
		a.pos = target
		a.next++
		if a.next >= len(a.waypoints) {
			a.done = true
			return true, nil
		}
		return false, nil
	}
	if dist > 0 {
		a.pos.X += dx / dist * travel
		a.pos.Y += dy / dist * travel
	}
	return false, nil
}

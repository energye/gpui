package tilemap

import (
	"math"

	"github.com/energye/gpui/game/core"
)

// Level selects which art a chunk draws: detailed near, simple far.
// The caller draws LevelNear chunks full and LevelFar chunks simplified
// with the existing render draws.
type Level int

const (
	// LevelNear is full detail: distance at or below the near line.
	LevelNear Level = 0
	// LevelFar is simplified: distance at or above the far line.
	LevelFar Level = 1
)

// String returns "near", "far", or "unknown" for logging and JSON.
func (l Level) String() string {
	switch l {
	case LevelNear:
		return "near"
	case LevelFar:
		return "far"
	default:
		return "unknown"
	}
}

// LOD switches chunks by distance with a hysteresis band: at or below
// Near stays detailed, at or above Far stays simple, inside (Near,Far)
// keeps the previous level so a jittering camera never flickers.
// It draws nothing; the caller feeds the camera focus (VisibleWorldRect
// center as core.Vec2, so this package never imports the camera package)
// and draws each chunk at its returned level.
type LOD struct {
	near float64
	far  float64
}

// NewLOD builds the switch. Distances must be finite with 0 < near < far.
// Bad sizes are InvalidArg, never a guessed band.
func NewLOD(near, far float64) (LOD, error) {
	const op = "tilemap.NewLOD"
	if !finite(near) || !finite(far) || near <= 0 || far <= 0 || !(near < far) {
		return LOD{}, core.InvalidArg(op, "range")
	}
	return LOD{near: near, far: far}, nil
}

// Near returns the detailed distance line.
func (l LOD) Near() float64 { return l.near }

// Far returns the simplified distance line.
func (l LOD) Far() float64 { return l.far }

// Band returns the hysteresis width: Far - Near, always > 0.
func (l LOD) Band() float64 { return l.far - l.near }

func validLevel(l Level) bool { return l == LevelNear || l == LevelFar }

// Select picks the level for dist keeping cur inside (Near,Far).
// At or below Near is Near, at or above Far is Far, bad distances keep
// cur. An invalid cur inside the band splits at the midpoint; outside
// the band the distance alone decides, so a fresh chunk still lands.
func (l LOD) Select(dist float64, cur Level) Level {
	if math.IsNaN(dist) || math.IsInf(dist, 0) {
		if validLevel(cur) {
			return cur
		}
		return LevelNear
	}
	if dist <= l.near {
		return LevelNear
	}
	if dist >= l.far {
		return LevelFar
	}
	if validLevel(cur) {
		return cur
	}
	if dist < (l.near+l.far)/2 {
		return LevelNear
	}
	return LevelFar
}

// ChunkDist measures focus to bounds: 0 inside or edge-touching, else the
// Euclidean gap to the nearest edge. Bad or empty inputs report ok=false.
func ChunkDist(focus core.Vec2, bounds core.Rect) (float64, bool) {
	if !finite(focus.X) || !finite(focus.Y) || !finiteRect(bounds) || bounds.IsEmpty() {
		return 0, false
	}
	if bounds.Contains(focus) {
		return 0, true
	}
	dx := 0.0
	if focus.X < bounds.X {
		dx = bounds.X - focus.X
	} else if focus.X >= bounds.X+bounds.W {
		dx = focus.X - (bounds.X + bounds.W)
	}
	dy := 0.0
	if focus.Y < bounds.Y {
		dy = bounds.Y - focus.Y
	} else if focus.Y >= bounds.Y+bounds.H {
		dy = focus.Y - (bounds.Y + bounds.H)
	}
	dist := math.Sqrt(dx*dx + dy*dy)
	if !finite(dist) {
		return 0, false
	}
	return dist, true
}

// ChunkLevel picks the level for one chunk box from the camera focus.
// Bad inputs keep cur and report ok=false, never panic.
func (l LOD) ChunkLevel(focus core.Vec2, bounds core.Rect, cur Level) (Level, bool) {
	dist, ok := ChunkDist(focus, bounds)
	if !ok {
		return cur, false
	}
	return l.Select(dist, cur), true
}

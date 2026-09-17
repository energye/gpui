package tilemap

import (
	"math"
	"sort"

	"github.com/energye/gpui/game/core"
)

// MaxChunks caps one grid so a bad size fails fast instead of hanging the
// Needed scan. Beyond it NewChunker reports OutOfMemory, never a guess.
const MaxChunks = 1 << 20

// ChunkID names one partition: CX/CY count chunks, not tiles.
// Chunk (0,0) covers tiles col [0,chunkW), row [0,chunkH).
type ChunkID struct {
	CX int
	CY int
}

// Chunk cuts a map into chunkW x chunkH tile blocks and tracks which
// blocks are loaded. It draws nothing; the caller draws Loaded chunks that
// intersect the camera view (see Visible) with the existing render draws.
// Only core numbers are used; the view is the camera VisibleWorldRect fed
// in as a core.Rect, so this package never imports the camera package.
type Chunk struct {
	orient Orientation
	mapW   int
	mapH   int
	tileW  float64
	tileH  float64
	chunkW int
	chunkH int
	cols   int
	rows   int
	loaded map[ChunkID]struct{}
}

// NewChunker builds a partition over a mapW x mapH tile map with the given
// tile size and chunk size (in tiles). Chunks bigger than the map collapse
// to one chunk; the tail row/column keeps the leftover tiles. Bad sizes
// are InvalidArg, a grid beyond MaxChunks is OutOfMemory.
func NewChunker(orient Orientation, mapW, mapH int, tileW, tileH float64, chunkW, chunkH int) (Chunk, error) {
	const op = "tilemap.NewChunker"
	if orient != OrientOrthogonal && orient != OrientIsometric {
		return Chunk{}, core.InvalidArg(op, "orient")
	}
	if mapW <= 0 || mapH <= 0 {
		return Chunk{}, core.InvalidArg(op, "size")
	}
	if !finite(tileW) || !finite(tileH) || tileW <= 0 || tileH <= 0 {
		return Chunk{}, core.InvalidArg(op, "tile")
	}
	if chunkW <= 0 || chunkH <= 0 {
		return Chunk{}, core.InvalidArg(op, "chunk")
	}
	cols := (mapW + chunkW - 1) / chunkW
	rows := (mapH + chunkH - 1) / chunkH
	if int64(cols)*int64(rows) > MaxChunks {
		return Chunk{}, core.OutOfMemory(op, "grid")
	}
	return Chunk{
		orient: orient, mapW: mapW, mapH: mapH,
		tileW: tileW, tileH: tileH,
		chunkW: chunkW, chunkH: chunkH, cols: cols, rows: rows,
	}, nil
}

// Orient returns the grid math selector.
func (c Chunk) Orient() Orientation { return c.orient }

// W returns the map width in tiles.
func (c Chunk) W() int { return c.mapW }

// H returns the map height in tiles.
func (c Chunk) H() int { return c.mapH }

// TileW returns the tile width in world units.
func (c Chunk) TileW() float64 { return c.tileW }

// TileH returns the tile height in world units.
func (c Chunk) TileH() float64 { return c.tileH }

// ChunkW returns the chunk width in tiles.
func (c Chunk) ChunkW() int { return c.chunkW }

// ChunkH returns the chunk height in tiles.
func (c Chunk) ChunkH() int { return c.chunkH }

// ChunkCols returns how many chunks span the map width.
func (c Chunk) ChunkCols() int { return c.cols }

// ChunkRows returns how many chunks span the map height.
func (c Chunk) ChunkRows() int { return c.rows }

// LoadedCount returns how many chunks are loaded now.
func (c *Chunk) LoadedCount() int {
	if c == nil || c.loaded == nil {
		return 0
	}
	return len(c.loaded)
}

func sortChunkIDs(ids []ChunkID) {
	sort.Slice(ids, func(i, j int) bool {
		if ids[i].CY != ids[j].CY {
			return ids[i].CY < ids[j].CY
		}
		return ids[i].CX < ids[j].CX
	})
}

// validView reports whether view can cull: finite with positive area.
// NaN/Inf is invalid (callers change nothing); empty is valid but sees
// nothing, exactly like ObjectsIn treats an empty rect.
func validView(view core.Rect) bool {
	return !view.IsEmpty() && finiteRect(view)
}

// ChunkOf maps one tile to its chunk. OOB tiles report ok=false.
func (c Chunk) ChunkOf(col, row int) (ChunkID, bool) {
	if c.cols <= 0 || c.rows <= 0 || col < 0 || row < 0 || col >= c.mapW || row >= c.mapH {
		return ChunkID{}, false
	}
	return ChunkID{CX: col / c.chunkW, CY: row / c.chunkH}, true
}

// isoChunkBounds unions the diamond boxes of tiles [c0,c1) x [r0,r1).
// Corner origins bound the linear projection, so four corners suffice.
func (c Chunk) isoChunkBounds(c0, r0, c1, r1 int) core.Rect {
	hw := c.tileW / 2
	hh := c.tileH / 2
	xs := [4]float64{
		(float64(c0) - float64(r0)) * hw,
		(float64(c1-1) - float64(r0)) * hw,
		(float64(c0) - float64(r1-1)) * hw,
		(float64(c1-1) - float64(r1-1)) * hw,
	}
	ys := [4]float64{
		(float64(c0) + float64(r0)) * hh,
		(float64(c1-1) + float64(r0)) * hh,
		(float64(c0) + float64(r1-1)) * hh,
		(float64(c1-1) + float64(r1-1)) * hh,
	}
	minX, maxX := xs[0], xs[0]
	minY, maxY := ys[0], ys[0]
	for i := 1; i < 4; i++ {
		minX = math.Min(minX, xs[i])
		maxX = math.Max(maxX, xs[i])
		minY = math.Min(minY, ys[i])
		maxY = math.Max(maxY, ys[i])
	}
	return core.NewRect(minX, minY, maxX+c.tileW-minX, maxY+c.tileH-minY)
}

func (c Chunk) chunkBounds(id ChunkID) (core.Rect, bool) {
	if id.CX < 0 || id.CY < 0 || id.CX >= c.cols || id.CY >= c.rows ||
		c.mapW <= 0 || c.mapH <= 0 {
		return core.Rect{}, false
	}
	c0 := id.CX * c.chunkW
	r0 := id.CY * c.chunkH
	c1 := min(c0+c.chunkW, c.mapW)
	r1 := min(r0+c.chunkH, c.mapH)
	if c.orient == OrientIsometric {
		return c.isoChunkBounds(c0, r0, c1, r1), true
	}
	return core.NewRect(float64(c0)*c.tileW, float64(r0)*c.tileH,
		float64(c1-c0)*c.tileW, float64(r1-r0)*c.tileH), true
}

// ChunkBounds returns the world box a chunk covers: orthogonal tile block,
// isometric union of member diamond boxes (conservative, neighbours
// overlap by design). Unknown ids report ok=false with zero.
func (c Chunk) ChunkBounds(id ChunkID) (core.Rect, bool) {
	return c.chunkBounds(id)
}

// Needed lists chunks touching view, sorted by (CY,CX) so replays match
// bit for bit. Edge-touching chunks stay out (positive-area overlap only).
// Bad or empty views need nothing, never panic.
func (c Chunk) Needed(view core.Rect) []ChunkID {
	if c.cols <= 0 || c.rows <= 0 || !validView(view) {
		return nil
	}
	var out []ChunkID
	for cy := 0; cy < c.rows; cy++ {
		for cx := 0; cx < c.cols; cx++ {
			id := ChunkID{CX: cx, CY: cy}
			if b, ok := c.chunkBounds(id); ok && b.Intersects(view) {
				out = append(out, id)
			}
		}
	}
	return out
}

// Load marks Needed(view) loaded and returns the newly loaded chunks
// (sorted, nil when nothing new). Invalid views change nothing.
func (c *Chunk) Load(view core.Rect) []ChunkID {
	if c == nil || !finiteRect(view) {
		return nil
	}
	need := c.Needed(view)
	if len(need) == 0 {
		return nil
	}
	if c.loaded == nil {
		c.loaded = make(map[ChunkID]struct{}, len(need))
	}
	var added []ChunkID
	for _, id := range need {
		if _, ok := c.loaded[id]; !ok {
			c.loaded[id] = struct{}{}
			added = append(added, id)
		}
	}
	return added
}

// Unload drops loaded chunks outside view and returns the dropped ones
// (sorted, nil when nothing dropped). An empty (but finite) view sees
// nothing, so everything unloads; an invalid view changes nothing.
func (c *Chunk) Unload(view core.Rect) []ChunkID {
	if c == nil || !finiteRect(view) || len(c.loaded) == 0 {
		return nil
	}
	need := c.Needed(view)
	keep := make(map[ChunkID]struct{}, len(need))
	for _, id := range need {
		keep[id] = struct{}{}
	}
	var gone []ChunkID
	for id := range c.loaded {
		if _, ok := keep[id]; !ok {
			gone = append(gone, id)
		}
	}
	for _, id := range gone {
		delete(c.loaded, id)
	}
	sortChunkIDs(gone)
	return gone
}

// Update loads what view needs and unloads the rest in one step, so a
// teleporting camera (跳跃镜头) never leaves stale chunks behind and never
// grows: afterwards Loaded == Needed(view). Invalid views change nothing
// and return nils.
func (c *Chunk) Update(view core.Rect) (loaded, unloaded []ChunkID) {
	if c == nil || !finiteRect(view) {
		return nil, nil
	}
	need := c.Needed(view)
	if c.loaded == nil {
		c.loaded = make(map[ChunkID]struct{}, len(need))
	}
	// Loads come straight from need in order; unloads scan need linearly
	// instead of a want map. Need sets stay tiny (tens of chunks), so the
	// scan is cheaper than a per-frame map and returns the same sets.
	for _, id := range need {
		if _, ok := c.loaded[id]; !ok {
			c.loaded[id] = struct{}{}
			loaded = append(loaded, id)
		}
	}
loadedLoop:
	for id := range c.loaded {
		for _, want := range need {
			if id == want {
				continue loadedLoop
			}
		}
		unloaded = append(unloaded, id)
	}
	for _, id := range unloaded {
		delete(c.loaded, id)
	}
	sortChunkIDs(unloaded)
	return loaded, unloaded
}

// Visible lists loaded chunks touching view: the draw list (镜头外不画).
// It never mutates the loaded set. Sorted, nil when nothing to draw.
func (c Chunk) Visible(view core.Rect) []ChunkID {
	if len(c.loaded) == 0 || !validView(view) {
		return nil
	}
	need := c.Needed(view)
	var out []ChunkID
	for _, id := range need {
		if _, ok := c.loaded[id]; ok {
			out = append(out, id)
		}
	}
	return out
}

// Loaded returns the loaded chunk ids, sorted. A fresh slice every call:
// writing it cannot alias the grid.
func (c *Chunk) Loaded() []ChunkID {
	if c == nil || len(c.loaded) == 0 {
		return nil
	}
	out := make([]ChunkID, 0, len(c.loaded))
	for id := range c.loaded {
		out = append(out, id)
	}
	sortChunkIDs(out)
	return out
}

// IsLoaded reports whether id is loaded. Unknown ids are false, nil grids
// are never loaded, never panic.
func (c *Chunk) IsLoaded(id ChunkID) bool {
	if c == nil || c.loaded == nil {
		return false
	}
	_, ok := c.loaded[id]
	return ok
}

// Reset unloads everything. Nil grids stay silent.
func (c *Chunk) Reset() {
	if c == nil {
		return
	}
	c.loaded = nil
}

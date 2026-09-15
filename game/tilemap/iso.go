package tilemap

import (
	"math"

	"github.com/energye/gpui/game/core"
)

// Iso is the 斜45度 helper: tile (col,row) bounding-box top-left is
// ((col-row)*TileW/2, (col+row)*TileH/2), diamond center adds half a tile.
// It draws nothing; TileToWorld feeds camera/projector math.
type Iso struct {
	tileW float64
	tileH float64
}

// NewIso builds the projector. Tiles must be finite and > 0.
func NewIso(tileW, tileH float64) (Iso, error) {
	if !finite(tileW) || !finite(tileH) || tileW <= 0 || tileH <= 0 {
		return Iso{}, core.InvalidArg("tilemap.NewIso", "tile")
	}
	return Iso{tileW: tileW, tileH: tileH}, nil
}

// TileW returns the tile width.
func (iso Iso) TileW() float64 { return iso.tileW }

// TileH returns the tile height.
func (iso Iso) TileH() float64 { return iso.tileH }

// TileToWorld returns the diamond bounding-box top-left for (col,row).
// Any integer cell maps fine; NaN can only come from overflow, which
// reports ok=false with zero instead.
func (iso Iso) TileToWorld(col, row int) (core.Vec2, bool) {
	x := (float64(col) - float64(row)) * iso.tileW / 2
	y := (float64(col) + float64(row)) * iso.tileH / 2
	if !finite(x) || !finite(y) {
		return core.Vec2{}, false
	}
	return core.V2(x, y), true
}

// TileCenter returns the diamond center for (col,row).
func (iso Iso) TileCenter(col, row int) (core.Vec2, bool) {
	origin, ok := iso.TileToWorld(col, row)
	if !ok {
		return core.Vec2{}, false
	}
	out := core.V2(origin.X+iso.tileW/2, origin.Y+iso.tileH/2)
	if !finite(out.X) || !finite(out.Y) {
		return core.Vec2{}, false
	}
	return out, true
}

// TileBounds returns the diamond bounding box (W=tileW,H=tileH).
func (iso Iso) TileBounds(col, row int) (core.Rect, bool) {
	origin, ok := iso.TileToWorld(col, row)
	if !ok {
		return core.Rect{}, false
	}
	return core.NewRect(origin.X, origin.Y, iso.tileW, iso.tileH), true
}

// DiamondContains reports whether world sits inside the (col,row) diamond
// (edges count as inside). Bad input is always false, never panics.
func (iso Iso) DiamondContains(col, row int, world core.Vec2) bool {
	if !finite(world.X) || !finite(world.Y) {
		return false
	}
	center, ok := iso.TileCenter(col, row)
	if !ok {
		return false
	}
	dx := math.Abs(world.X-center.X) / (iso.tileW / 2)
	dy := math.Abs(world.Y-center.Y) / (iso.tileH / 2)
	return dx+dy <= 1+1e-9
}

// WorldToTile floors world to the diamond that contains it.
// Bounding boxes overlap, so the inverse formula only seeds a 3x3 search
// around the candidate; the diamond test picks the owner. Shared edges
// belong to two diamonds and vertices to three: the candidate-first order
// above returns one owner deterministically, centers always return their
// own tile. Bad input reports ok=false with zeros.
func (iso Iso) WorldToTile(world core.Vec2) (col, row int, ok bool) {
	if !finite(world.X) || !finite(world.Y) {
		return 0, 0, false
	}
	hw := iso.tileW / 2
	hh := iso.tileH / 2
	if !(hw > 0) || !(hh > 0) {
		return 0, 0, false
	}
	fx := world.X / hw
	fy := world.Y / hh
	cc := int(math.Floor((fx + fy) / 2))
	rr := int(math.Floor((fy - fx) / 2))
	for _, cand := range [9][2]int{
		{cc, rr}, {cc + 1, rr}, {cc, rr + 1}, {cc - 1, rr}, {cc, rr - 1},
		{cc + 1, rr + 1}, {cc - 1, rr - 1}, {cc + 1, rr - 1}, {cc - 1, rr + 1},
	} {
		if iso.DiamondContains(cand[0], cand[1], world) {
			return cand[0], cand[1], true
		}
	}
	return 0, 0, false
}

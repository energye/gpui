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
	"github.com/energye/gpui/engine/core"
)

// Bake limits: a 1024x1024 grid is the largest single bake (same order as
// the tilemap large档). Past it Bake reports OutOfMemory, never a half grid.
const (
	MaxGridW     = 1024
	MaxGridH     = 1024
	MaxGridCells = 1048576
)

// Grid is one baked walkable grid: W/H cell counts plus row-major solid
// flags (true blocks). The zero Grid is invalid; build via NewGrid/Bake.
type Grid struct {
	w     int
	h     int
	solid []bool
}

// NewGrid builds an empty (all walkable) grid. W/H must sit in
// [1,1024] with W*H <= MaxGridCells, else InvalidArg (too large is
// OutOfMemory).
func NewGrid(w, h int) (Grid, error) {
	const op = "nav.NewGrid"
	if w < 1 || h < 1 || w > MaxGridW || h > MaxGridH {
		return Grid{}, core.InvalidArg(op, "size")
	}
	if int64(w)*int64(h) > int64(MaxGridCells) {
		return Grid{}, core.OutOfMemory(op, "cells")
	}
	return Grid{w: w, h: h, solid: make([]bool, w*h)}, nil
}

// Bake builds a grid by sampling solidAt per cell: nil predicate means all
// walkable. Same W/H plus same predicate answers always yield the same
// Grid and the same Encode bytes. Bad sizes follow NewGrid codes.
func Bake(w, h int, solidAt func(x, y int) bool) (Grid, error) {
	const op = "nav.Bake"
	g, err := NewGrid(w, h)
	if err != nil {
		return Grid{}, err
	}
	if solidAt == nil {
		return g, nil
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if solidAt(x, y) {
				g.solid[y*g.w+x] = true
			}
		}
	}
	return g, nil
}

// Valid reports whether g holds a usable grid.
func (g Grid) Valid() bool {
	return g.w >= 1 && g.h >= 1 && len(g.solid) == g.w*g.h
}

// W returns the grid width, or 0 on an invalid grid.
func (g Grid) W() int {
	if !g.Valid() {
		return 0
	}
	return g.w
}

// H returns the grid height, or 0 on an invalid grid.
func (g Grid) H() int {
	if !g.Valid() {
		return 0
	}
	return g.h
}

// InBounds reports whether (x,y) sits inside the grid.
func (g Grid) InBounds(x, y int) bool {
	return g.Valid() && x >= 0 && y >= 0 && x < g.w && y < g.h
}

// SolidAt reports whether (x,y) blocks. Out-of-bounds and invalid grids
// report blocked (fail closed) with ok=false.
func (g Grid) SolidAt(x, y int) (solid, ok bool) {
	if !g.InBounds(x, y) {
		return true, false
	}
	return g.solid[y*g.w+x], true
}

// SetSolid flips one cell. Out-of-bounds or invalid grids report
// InvalidArg and change nothing.
func (g *Grid) SetSolid(x, y int, solid bool) error {
	const op = "nav.Grid.SetSolid"
	if g == nil {
		return core.InvalidArg(op, "grid")
	}
	if !g.InBounds(x, y) {
		return core.InvalidArg(op, "cell")
	}
	g.solid[y*g.w+x] = solid
	return nil
}

// SolidCount counts blocked cells, or 0 on an invalid grid.
func (g Grid) SolidCount() int {
	if !g.Valid() {
		return 0
	}
	n := 0
	for _, s := range g.solid {
		if s {
			n++
		}
	}
	return n
}

// Encode renders the deterministic bake artifact: row-major 0/1 bytes
// (y*W+x), 1 means blocked. Same input always yields identical bytes;
// invalid grids encode to nil.
func (g Grid) Encode() []byte {
	if !g.Valid() {
		return nil
	}
	out := make([]byte, len(g.solid))
	for i, s := range g.solid {
		if s {
			out[i] = 1
		}
	}
	return out
}

// Decode rebuilds a grid from Encode bytes: length must equal W*H and
// every byte must be 0/1. Size faults are InvalidArg (too large is
// OutOfMemory); length/value faults are BadData. Bad writes build nothing.
func Decode(w, h int, raw []byte) (Grid, error) {
	const op = "nav.Decode"
	g, err := NewGrid(w, h)
	if err != nil {
		return Grid{}, err
	}
	if len(raw) != w*h {
		return Grid{}, core.BadData(op, "length")
	}
	for i, b := range raw {
		if b != 0 && b != 1 {
			return Grid{}, core.BadData(op, "value")
		}
		g.solid[i] = b == 1
	}
	return g, nil
}

// Equal reports whether two grids share size and every solid flag.
// Invalid grids never equal, even to each other.
func (g Grid) Equal(o Grid) bool {
	if !g.Valid() || !o.Valid() || g.w != o.w || g.h != o.h {
		return false
	}
	for i := range g.solid {
		if g.solid[i] != o.solid[i] {
			return false
		}
	}
	return true
}

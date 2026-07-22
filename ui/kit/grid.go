package kit

import (
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Grid is kit wrapper for primitive.Grid (Ant Row/Col simplified).
// https://ant.design/components/grid
type Grid struct {
	Root *primitive.Grid
}

// NewGridCols creates an n-column equal FR grid.
func NewGridCols(n int, children ...core.Node) *Grid {
	if n < 1 {
		n = 1
	}
	cols := make([]core.GridTrack, n)
	for i := range cols {
		cols[i] = core.GridTrack{Fr: 1}
	}
	return &Grid{Root: primitive.NewGrid(cols, children...)}
}

// Node returns root.
func (g *Grid) Node() core.Node {
	if g == nil {
		return nil
	}
	return g.Root
}

// SetGap sets column and row gap.
func (g *Grid) SetGap(gap float64) {
	if g == nil || g.Root == nil {
		return
	}
	g.Root.ColumnGap = gap
	g.Root.RowGap = gap
}

// SetCols rebuilds equal FR columns.
func (g *Grid) SetCols(n int) {
	if g == nil || g.Root == nil {
		return
	}
	if n < 1 {
		n = 1
	}
	cols := make([]core.GridTrack, n)
	for i := range cols {
		cols[i] = core.GridTrack{Fr: 1}
	}
	g.Root.Columns = cols
	g.Root.MarkNeedsLayout()
}

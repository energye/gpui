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

package core

// GridTrack is a column or row track size.
// Fr > 0 uses flex fraction of free space; Px > 0 is fixed; both 0 → auto (content).
type GridTrack struct {
	Px float64
	Fr float64
}

// GridCell describes a child placement in Grid layout.
type GridCell struct {
	Node Node
	// Col/Row are 0-based start indices.
	Col, Row int
	// ColSpan/RowSpan default 1.
	ColSpan, RowSpan int
}

// GridLayoutParams configures a CSS-grid-like single pass (simplified).
type GridLayoutParams struct {
	Columns []GridTrack
	Rows    []GridTrack // optional; if empty, rows auto-size to content
	// ColumnGap / RowGap between tracks.
	ColumnGap, RowGap float64
	// Cells if non-nil places specific children; else children flow row-major.
	Cells []GridCell
}

// LayoutGrid lays out children in a simple grid under constraints.
func LayoutGrid(parent *NodeBase, c Constraints, p GridLayoutParams) Size {
	kids := parent.children
	cols := p.Columns
	if len(cols) == 0 {
		cols = []GridTrack{{Fr: 1}, {Fr: 1}}
	}
	nCol := len(cols)

	// Build placement list
	type place struct {
		node     Node
		col, row int
		cs, rs   int
	}
	var places []place
	if len(p.Cells) > 0 {
		for _, cell := range p.Cells {
			cs, rs := cell.ColSpan, cell.RowSpan
			if cs < 1 {
				cs = 1
			}
			if rs < 1 {
				rs = 1
			}
			places = append(places, place{cell.Node, cell.Col, cell.Row, cs, rs})
		}
	} else {
		for i, child := range kids {
			places = append(places, place{child, i % nCol, i / nCol, 1, 1})
		}
	}

	// Determine row count
	nRow := 0
	for _, pl := range places {
		end := pl.row + pl.rs
		if end > nRow {
			nRow = end
		}
	}
	if nRow == 0 {
		out := c.Tighten(Size{})
		parent.SetSize(out)
		return out
	}

	// Available width
	maxW := c.MaxWidth
	if !isFinite(maxW) {
		maxW = 0
		for _, t := range cols {
			if t.Px > 0 {
				maxW += t.Px
			} else {
				maxW += 80 // guess for unbounded
			}
		}
		if nCol > 1 {
			maxW += p.ColumnGap * float64(nCol-1)
		}
	}
	gapW := 0.0
	if nCol > 1 {
		gapW = p.ColumnGap * float64(nCol-1)
	}
	freeW := maxW - gapW
	fixedW := 0.0
	totalFr := 0.0
	for _, t := range cols {
		if t.Fr > 0 {
			totalFr += t.Fr
		} else if t.Px > 0 {
			fixedW += t.Px
		}
	}
	freeForFr := freeW - fixedW
	if freeForFr < 0 {
		freeForFr = 0
	}
	colW := make([]float64, nCol)
	for i, t := range cols {
		if t.Fr > 0 && totalFr > 0 {
			colW[i] = freeForFr * (t.Fr / totalFr)
		} else if t.Px > 0 {
			colW[i] = t.Px
		} else {
			// auto: equal share of remaining or min 40
			colW[i] = 40
			if totalFr == 0 && freeForFr > 0 {
				colW[i] = freeForFr / float64(nCol)
			}
		}
	}
	// column x offsets
	colX := make([]float64, nCol)
	x := 0.0
	for i := 0; i < nCol; i++ {
		colX[i] = x
		x += colW[i]
		if i < nCol-1 {
			x += p.ColumnGap
		}
	}
	totalW := x

	// First pass: measure row heights (max content in row for auto rows)
	rowH := make([]float64, nRow)
	if len(p.Rows) >= nRow {
		for i := 0; i < nRow; i++ {
			if p.Rows[i].Px > 0 {
				rowH[i] = p.Rows[i].Px
			}
		}
	}

	// Layout each cell with its track size
	for _, pl := range places {
		if pl.node == nil {
			continue
		}
		// span width
		w := 0.0
		for c := pl.col; c < pl.col+pl.cs && c < nCol; c++ {
			w += colW[c]
			if c > pl.col {
				w += p.ColumnGap
			}
		}
		// provisional height: if fixed row use it else loose
		hMax := Unbounded
		if pl.rs == 1 && pl.row < len(rowH) && rowH[pl.row] > 0 {
			hMax = rowH[pl.row]
		}
		sz := pl.node.Layout(Constraints{MaxWidth: w, MaxHeight: hMax})
		// distribute height to rows if auto
		per := sz.Height / float64(pl.rs)
		for r := pl.row; r < pl.row+pl.rs && r < nRow; r++ {
			if rowH[r] < per {
				rowH[r] = per
			}
		}
	}

	// Apply fr rows if specified
	if len(p.Rows) >= nRow {
		// recompute with fr — simplified: if any Fr, distribute remaining height
		maxH := c.MaxHeight
		if isFinite(maxH) {
			gapH := 0.0
			if nRow > 1 {
				gapH = p.RowGap * float64(nRow-1)
			}
			fixedH, frSum := 0.0, 0.0
			for i := 0; i < nRow; i++ {
				if p.Rows[i].Fr > 0 {
					frSum += p.Rows[i].Fr
				} else if p.Rows[i].Px > 0 {
					fixedH += p.Rows[i].Px
					rowH[i] = p.Rows[i].Px
				} else {
					fixedH += rowH[i]
				}
			}
			freeH := maxH - gapH - fixedH
			if freeH > 0 && frSum > 0 {
				for i := 0; i < nRow; i++ {
					if p.Rows[i].Fr > 0 {
						rowH[i] = freeH * (p.Rows[i].Fr / frSum)
					}
				}
			}
		}
	}

	rowY := make([]float64, nRow)
	y := 0.0
	for i := 0; i < nRow; i++ {
		rowY[i] = y
		y += rowH[i]
		if i < nRow-1 {
			y += p.RowGap
		}
	}
	totalH := y

	// Position children (re-layout with final tight sizes for single-cell)
	for _, pl := range places {
		if pl.node == nil {
			continue
		}
		w := 0.0
		for c := pl.col; c < pl.col+pl.cs && c < nCol; c++ {
			w += colW[c]
			if c > pl.col {
				w += p.ColumnGap
			}
		}
		h := 0.0
		for r := pl.row; r < pl.row+pl.rs && r < nRow; r++ {
			h += rowH[r]
			if r > pl.row {
				h += p.RowGap
			}
		}
		_ = pl.node.Layout(Constraints{MinWidth: w, MaxWidth: w, MinHeight: h, MaxHeight: h})
		pl.node.Base().SetOffset(Point{X: colX[pl.col], Y: rowY[pl.row]})
	}

	out := c.Tighten(Size{Width: totalW, Height: totalH})
	parent.SetSize(out)
	return out
}

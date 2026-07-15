package render

// PathBooleanOp selects a path boolean operation (H.04 / SkPath::Op).
type PathBooleanOp int

const (
	PathOpUnion PathBooleanOp = iota
	PathOpIntersect
	PathOpDifference
	PathOpXor
)

// BooleanPath computes a path representing a boolean combination of a and b.
//
// Implementation: scanline membership via winding numbers, then emit filled
// horizontal runs as rectangles. Correct for fill semantics (Skia soft path);
// suitable for UI-level boolean regions.
func BooleanPath(a, b *Path, op PathBooleanOp) *Path {
	if a == nil || a.isEmpty() {
		if op == PathOpUnion || op == PathOpXor {
			if b == nil {
				return NewPath()
			}
			return b.Clone()
		}
		return NewPath()
	}
	if b == nil || b.isEmpty() {
		if op == PathOpUnion || op == PathOpDifference || op == PathOpXor {
			return a.Clone()
		}
		return NewPath()
	}

	ab := a.Bounds()
	bb := b.Bounds()
	minX := ab.Min.X
	if bb.Min.X < minX {
		minX = bb.Min.X
	}
	minY := ab.Min.Y
	if bb.Min.Y < minY {
		minY = bb.Min.Y
	}
	maxX := ab.Max.X
	if bb.Max.X > maxX {
		maxX = bb.Max.X
	}
	maxY := ab.Max.Y
	if bb.Max.Y > maxY {
		maxY = bb.Max.Y
	}
	minX--
	minY--
	maxX++
	maxY++
	if maxX <= minX || maxY <= minY {
		return NewPath()
	}

	// Cap extremely large regions to keep CPU bounded.
	const maxDim = 2048
	if maxX-minX > maxDim {
		maxX = minX + maxDim
	}
	if maxY-minY > maxDim {
		maxY = minY + maxDim
	}

	out := NewPath()
	for y := minY; y < maxY; y++ {
		py := float64(y) + 0.5
		runStart := -1
		for x := minX; x <= maxX; x++ {
			px := float64(x) + 0.5
			inA := a.Winding(Pt(px, py)) != 0
			inB := b.Winding(Pt(px, py)) != 0
			inside := false
			switch op {
			case PathOpUnion:
				inside = inA || inB
			case PathOpIntersect:
				inside = inA && inB
			case PathOpDifference:
				inside = inA && !inB
			case PathOpXor:
				inside = inA != inB
			}
			if inside {
				if runStart < 0 {
					runStart = x
				}
			} else if runStart >= 0 {
				out.Rectangle(float64(runStart), float64(y), float64(x-runStart), 1)
				runStart = -1
			}
		}
		if runStart >= 0 {
			out.Rectangle(float64(runStart), float64(y), float64(maxX-runStart), 1)
		}
	}
	return out
}

// Op returns BooleanPath(p, other, op).
func (p *Path) Op(other *Path, op PathBooleanOp) *Path {
	return BooleanPath(p, other, op)
}

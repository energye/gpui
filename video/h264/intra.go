package h264

import "fmt"

// Intra 4x4 modes (Table 8-2).
const (
	Intra4x4Vertical = iota
	Intra4x4Horizontal
	Intra4x4DC
	Intra4x4DiagDownLeft
	Intra4x4DiagDownRight
	Intra4x4VerticalRight
	Intra4x4HorizontalDown
	Intra4x4VerticalLeft
	Intra4x4HorizontalUp
)

// Intra 16x16 and chroma modes.
const (
	IntraPredVertical = iota
	IntraPredHorizontal
	IntraPredDC
	IntraPredPlane
)

// sampler reads a decoded neighbour sample; false outside the picture or
// from not-yet-decoded areas (the caller gates those).
type sampler func(x, y int32) (uint8, bool)

// clampIV is a temporary diagnostic switch (not for commit): fall back to
// DC instead of erroring on unavailable-top Vertical.
var clampIV = false

// PredIntra4x4 predicts one 4x4 luma block at pixel (bx, by).
// Pixel formulas follow the 4x4 mode equations one by one.
func PredIntra4x4(get sampler, bx, by int32, mode int) ([16]uint8, error) {
	var out [16]uint8
	set := func(x, y int, v int) { out[y*4+x] = uint8(v) }
	top := func(i int) (uint8, bool) { return get(bx+int32(i), by-1) }
	left := func(i int) (uint8, bool) { return get(bx-1, by+int32(i)) }
retry:
	switch mode {
	case Intra4x4Vertical:
		t := make([]uint8, 4)
		for i := range t {
			v, ok := top(i)
			if !ok {
				if clampIV {
					mode = Intra4x4DC
					goto retry
				}
				return out, fmt.Errorf("%w: intra4x4 v top missing", ErrBadSliceHeader)
			}
			t[i] = v
		}
		for y := 0; y < 4; y++ {
			for x := 0; x < 4; x++ {
				set(x, y, int(t[x]))
			}
		}
	case Intra4x4Horizontal:
		l := make([]uint8, 4)
		for i := range l {
			v, ok := left(i)
			if !ok {
				return out, fmt.Errorf("%w: intra4x4 h left missing", ErrBadSliceHeader)
			}
			l[i] = v
		}
		for y := 0; y < 4; y++ {
			for x := 0; x < 4; x++ {
				set(x, y, int(l[y]))
			}
		}
	case Intra4x4DC:
		sum, n := 0, 0
		for i := 0; i < 4; i++ {
			if v, ok := top(i); ok {
				sum += int(v)
				n++
			}
			if v, ok := left(i); ok {
				sum += int(v)
				n++
			}
		}
		mean := 128
		if n == 8 {
			mean = (sum + 4) >> 3
		} else if n == 4 {
			mean = (sum + 2) >> 2
		}
		for i := range out {
			out[i] = uint8(mean)
		}
	case Intra4x4DiagDownLeft:
		t := make([]uint8, 8)
		for i := 0; i < 4; i++ {
			v, ok := top(i)
			if !ok {
				return out, fmt.Errorf("%w: intra4x4 ddl top missing", ErrBadSliceHeader)
			}
			t[i] = v
		}
		// Missing top-right samples fall back to p[3,-1]: DDL stays
		// legal at the right picture edge.
		for i := 4; i < 8; i++ {
			v, ok := top(i)
			if !ok {
				v = t[3]
			}
			t[i] = v
		}
		i := func(n int) int { return int(t[n]) }
		set(0, 0, (i(0)+i(2)+2*i(1)+2)>>2)
		set(1, 0, (i(1)+i(3)+2*i(2)+2)>>2)
		set(0, 1, (i(1)+i(3)+2*i(2)+2)>>2)
		set(2, 0, (i(2)+i(4)+2*i(3)+2)>>2)
		set(1, 1, (i(2)+i(4)+2*i(3)+2)>>2)
		set(0, 2, (i(2)+i(4)+2*i(3)+2)>>2)
		set(3, 0, (i(3)+i(5)+2*i(4)+2)>>2)
		set(2, 1, (i(3)+i(5)+2*i(4)+2)>>2)
		set(1, 2, (i(3)+i(5)+2*i(4)+2)>>2)
		set(0, 3, (i(3)+i(5)+2*i(4)+2)>>2)
		set(3, 1, (i(4)+i(6)+2*i(5)+2)>>2)
		set(2, 2, (i(4)+i(6)+2*i(5)+2)>>2)
		set(1, 3, (i(4)+i(6)+2*i(5)+2)>>2)
		set(3, 2, (i(5)+i(7)+2*i(6)+2)>>2)
		set(2, 3, (i(5)+i(7)+2*i(6)+2)>>2)
		set(3, 3, (i(6)+3*i(7)+2)>>2)
	case Intra4x4DiagDownRight:
		t := make([]uint8, 4)
		l := make([]uint8, 4)
		for i := 0; i < 4; i++ {
			a, ok1 := top(i)
			b, ok2 := left(i)
			if !ok1 || !ok2 {
				return out, fmt.Errorf("%w: intra4x4 ddr neighbours missing", ErrBadSliceHeader)
			}
			t[i], l[i] = a, b
		}
		lt, ok := get(bx-1, by-1)
		if !ok {
			return out, fmt.Errorf("%w: intra4x4 ddr corner missing", ErrBadSliceHeader)
		}
		i := func(n int) int { return int(t[n]) }
		j := func(n int) int { return int(l[n]) }
		c := int(lt)
		set(0, 3, (j(3)+2*j(2)+j(1)+2)>>2)
		set(0, 2, (j(2)+2*j(1)+j(0)+2)>>2)
		set(1, 3, (j(2)+2*j(1)+j(0)+2)>>2)
		set(0, 1, (j(1)+2*j(0)+c+2)>>2)
		set(1, 2, (j(1)+2*j(0)+c+2)>>2)
		set(2, 3, (j(1)+2*j(0)+c+2)>>2)
		set(0, 0, (j(0)+2*c+i(0)+2)>>2)
		set(1, 1, (j(0)+2*c+i(0)+2)>>2)
		set(2, 2, (j(0)+2*c+i(0)+2)>>2)
		set(3, 3, (j(0)+2*c+i(0)+2)>>2)
		set(1, 0, (c+2*i(0)+i(1)+2)>>2)
		set(2, 1, (c+2*i(0)+i(1)+2)>>2)
		set(3, 2, (c+2*i(0)+i(1)+2)>>2)
		set(2, 0, (i(0)+2*i(1)+i(2)+2)>>2)
		set(3, 1, (i(0)+2*i(1)+i(2)+2)>>2)
		set(3, 0, (i(1)+2*i(2)+i(3)+2)>>2)
	case Intra4x4VerticalRight:
		t := make([]uint8, 4)
		l := make([]uint8, 4)
		for i := 0; i < 4; i++ {
			a, ok1 := top(i)
			b, ok2 := left(i)
			if !ok1 || !ok2 {
				return out, fmt.Errorf("%w: intra4x4 vr neighbours missing", ErrBadSliceHeader)
			}
			t[i], l[i] = a, b
		}
		lt, ok := get(bx-1, by-1)
		if !ok {
			return out, fmt.Errorf("%w: intra4x4 vr corner missing", ErrBadSliceHeader)
		}
		i := func(n int) int { return int(t[n]) }
		j := func(n int) int { return int(l[n]) }
		c := int(lt)
		set(0, 0, (c+i(0)+1)>>1)
		set(1, 2, (c+i(0)+1)>>1)
		set(1, 0, (i(0)+i(1)+1)>>1)
		set(2, 2, (i(0)+i(1)+1)>>1)
		set(2, 0, (i(1)+i(2)+1)>>1)
		set(3, 2, (i(1)+i(2)+1)>>1)
		set(3, 0, (i(2)+i(3)+1)>>1)
		set(0, 1, (j(0)+2*c+i(0)+2)>>2)
		set(1, 3, (j(0)+2*c+i(0)+2)>>2)
		set(1, 1, (c+2*i(0)+i(1)+2)>>2)
		set(2, 3, (c+2*i(0)+i(1)+2)>>2)
		set(2, 1, (i(0)+2*i(1)+i(2)+2)>>2)
		set(3, 3, (i(0)+2*i(1)+i(2)+2)>>2)
		set(3, 1, (i(1)+2*i(2)+i(3)+2)>>2)
		set(0, 2, (c+2*j(0)+j(1)+2)>>2)
		set(0, 3, (j(0)+2*j(1)+j(2)+2)>>2)
	case Intra4x4HorizontalDown:
		t := make([]uint8, 4)
		l := make([]uint8, 4)
		for i := 0; i < 4; i++ {
			a, ok1 := top(i)
			b, ok2 := left(i)
			if !ok1 || !ok2 {
				return out, fmt.Errorf("%w: intra4x4 hd neighbours missing", ErrBadSliceHeader)
			}
			t[i], l[i] = a, b
		}
		lt, ok := get(bx-1, by-1)
		if !ok {
			return out, fmt.Errorf("%w: intra4x4 hd corner missing", ErrBadSliceHeader)
		}
		i := func(n int) int { return int(t[n]) }
		j := func(n int) int { return int(l[n]) }
		c := int(lt)
		set(0, 0, (c+j(0)+1)>>1)
		set(2, 1, (c+j(0)+1)>>1)
		set(1, 0, (j(0)+2*c+i(0)+2)>>2)
		set(3, 1, (j(0)+2*c+i(0)+2)>>2)
		set(2, 0, (c+2*i(0)+i(1)+2)>>2)
		set(3, 0, (i(0)+2*i(1)+i(2)+2)>>2)
		set(0, 1, (j(0)+j(1)+1)>>1)
		set(2, 2, (j(0)+j(1)+1)>>1)
		set(1, 1, (c+2*j(0)+j(1)+2)>>2)
		set(3, 2, (c+2*j(0)+j(1)+2)>>2)
		set(0, 2, (j(1)+j(2)+1)>>1)
		set(2, 3, (j(1)+j(2)+1)>>1)
		set(1, 2, (j(0)+2*j(1)+j(2)+2)>>2)
		set(3, 3, (j(0)+2*j(1)+j(2)+2)>>2)
		set(0, 3, (j(2)+j(3)+1)>>1)
		set(1, 3, (j(1)+2*j(2)+j(3)+2)>>2)
	case Intra4x4VerticalLeft:
		t := make([]uint8, 8)
		for i := 0; i < 4; i++ {
			v, ok := top(i)
			if !ok {
				return out, fmt.Errorf("%w: intra4x4 vl top missing", ErrBadSliceHeader)
			}
			t[i] = v
		}
		for i := 4; i < 8; i++ {
			v, ok := top(i)
			if !ok {
				v = t[3]
			}
			t[i] = v
		}
		i := func(n int) int { return int(t[n]) }
		set(0, 0, (i(0)+i(1)+1)>>1)
		set(1, 0, (i(1)+i(2)+1)>>1)
		set(0, 2, (i(1)+i(2)+1)>>1)
		set(2, 0, (i(2)+i(3)+1)>>1)
		set(1, 2, (i(2)+i(3)+1)>>1)
		set(3, 0, (i(3)+i(4)+1)>>1)
		set(2, 2, (i(3)+i(4)+1)>>1)
		set(3, 2, (i(4)+i(5)+1)>>1)
		set(0, 1, (i(0)+2*i(1)+i(2)+2)>>2)
		set(1, 1, (i(1)+2*i(2)+i(3)+2)>>2)
		set(0, 3, (i(1)+2*i(2)+i(3)+2)>>2)
		set(2, 1, (i(2)+2*i(3)+i(4)+2)>>2)
		set(1, 3, (i(2)+2*i(3)+i(4)+2)>>2)
		set(3, 1, (i(3)+2*i(4)+i(5)+2)>>2)
		set(2, 3, (i(3)+2*i(4)+i(5)+2)>>2)
		set(3, 3, (i(4)+2*i(5)+i(6)+2)>>2)
	case Intra4x4HorizontalUp:
		l := make([]uint8, 4)
		for i := range l {
			v, ok := left(i)
			if !ok {
				return out, fmt.Errorf("%w: intra4x4 hu left missing", ErrBadSliceHeader)
			}
			l[i] = v
		}
		j := func(n int) int { return int(l[n]) }
		set(0, 0, (j(0)+j(1)+1)>>1)
		set(1, 0, (j(0)+2*j(1)+j(2)+2)>>2)
		set(2, 0, (j(1)+j(2)+1)>>1)
		set(0, 1, (j(1)+j(2)+1)>>1)
		set(3, 0, (j(1)+2*j(2)+j(3)+2)>>2)
		set(1, 1, (j(1)+2*j(2)+j(3)+2)>>2)
		set(2, 1, (j(2)+j(3)+1)>>1)
		set(0, 2, (j(2)+j(3)+1)>>1)
		set(3, 1, (j(2)+2*j(3)+j(3)+2)>>2)
		set(1, 2, (j(2)+2*j(3)+j(3)+2)>>2)
		set(3, 2, j(3))
		set(1, 3, j(3))
		set(0, 3, j(3))
		set(2, 2, j(3))
		set(2, 3, j(3))
		set(3, 3, j(3))
	default:
		return out, fmt.Errorf("%w: intra4x4 mode %d", ErrBadSliceHeader, mode)
	}
	return out, nil
}

// PredIntra16x16 predicts a 16x16 luma block at pixel (bx, by).
func PredIntra16x16(get sampler, bx, by int32, mode int) ([256]uint8, error) {
	var out [256]uint8
	top := make([]uint8, 16)
	left := make([]uint8, 16)
	hasTop, hasLeft := true, true
	for i := 0; i < 16; i++ {
		v, ok := get(bx+int32(i), by-1)
		if !ok {
			hasTop = false
			break
		}
		top[i] = v
	}
	for i := 0; i < 16; i++ {
		v, ok := get(bx-1, by+int32(i))
		if !ok {
			hasLeft = false
			break
		}
		left[i] = v
	}
	set := func(x, y int, v uint8) { out[y*16+x] = v }
	switch mode {
	case IntraPredVertical:
		if !hasTop {
			return out, fmt.Errorf("%w: intra16 top missing", ErrBadSliceHeader)
		}
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				set(x, y, top[x])
			}
		}
	case IntraPredHorizontal:
		if !hasLeft {
			return out, fmt.Errorf("%w: intra16 left missing", ErrBadSliceHeader)
		}
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				set(x, y, left[y])
			}
		}
	case IntraPredDC:
		sum, n := 0, 0
		if hasTop {
			for _, v := range top {
				sum += int(v)
			}
			n += 16
		}
		if hasLeft {
			for _, v := range left {
				sum += int(v)
			}
			n += 16
		}
		mean := 128
		if n == 32 {
			mean = (sum + 16) >> 5
		} else if n == 16 {
			mean = (sum + 8) >> 4
		}
		for i := range out {
			out[i] = uint8(mean)
		}
	case IntraPredPlane:
		if !hasTop || !hasLeft {
			return out, fmt.Errorf("%w: intra16 plane neighbours missing", ErrBadSliceHeader)
		}
		tl, ok := get(bx-1, by-1)
		if !ok {
			return out, fmt.Errorf("%w: intra16 corner missing", ErrBadSliceHeader)
		}
		H, V := 0, 0
		for i := 0; i < 8; i++ {
			t0, l0 := int(tl), int(tl)
			if 6-i >= 0 {
				t0, l0 = int(top[6-i]), int(left[6-i])
			}
			H += (i + 1) * (int(top[8+i]) - t0)
			V += (i + 1) * (int(left[8+i]) - l0)
		}
		a := 16 * (int(left[15]) + int(top[15]))
		b := (5*H + 32) >> 6
		c := (5*V + 32) >> 6
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				v := (a + b*(x-7) + c*(y-7) + 16) >> 5
				if v < 0 {
					v = 0
				}
				if v > 255 {
					v = 255
				}
				set(x, y, uint8(v))
			}
		}
	default:
		return out, fmt.Errorf("%w: intra16 mode %d", ErrBadSliceHeader, mode)
	}
	return out, nil
}

// PredIntraChroma predicts one 8x8 chroma block at pixel (bx, by) in the
// subsampled plane.
func PredIntraChroma(get sampler, bx, by int32, mode int) ([64]uint8, error) {
	var out [64]uint8
	top := make([]uint8, 8)
	left := make([]uint8, 8)
	hasTop, hasLeft := true, true
	for i := 0; i < 8; i++ {
		v, ok := get(bx+int32(i), by-1)
		if !ok {
			hasTop = false
			break
		}
		top[i] = v
	}
	for i := 0; i < 8; i++ {
		v, ok := get(bx-1, by+int32(i))
		if !ok {
			hasLeft = false
			break
		}
		left[i] = v
	}
	set := func(x, y int, v uint8) { out[y*8+x] = v }
	switch mode {
	case IntraPredDC:
		// Edge DC splits halves (matches reference 8x8 behaviour):
		// top-only averages left/right top groups per 4 columns,
		// left-only averages top/bottom left groups per 4 rows.
		if hasTop && hasLeft {
			tl, tr, bl := 0, 0, 0
			for i := 0; i < 4; i++ {
				tl += int(left[i]) + int(top[i])
				tr += int(top[4+i])
				bl += int(left[4+i])
			}
			mTL, mTR := (tl+4)>>3, (tr+2)>>2
			mBL, mBR := (bl+2)>>2, (tr+bl+4)>>3
			for y := 0; y < 4; y++ {
				for x := 0; x < 4; x++ {
					set(x, y, uint8(mTL))
					set(4+x, y, uint8(mTR))
				}
			}
			for y := 4; y < 8; y++ {
				for x := 0; x < 4; x++ {
					set(x, y, uint8(mBL))
					set(4+x, y, uint8(mBR))
				}
			}
		} else if hasTop {
			s0, s1 := 0, 0
			for i := 0; i < 4; i++ {
				s0 += int(top[i])
				s1 += int(top[4+i])
			}
			m0, m1 := uint8((s0+2)>>2), uint8((s1+2)>>2)
			for y := 0; y < 8; y++ {
				for x := 0; x < 4; x++ {
					set(x, y, m0)
					set(4+x, y, m1)
				}
			}
		} else if hasLeft {
			s0, s1 := 0, 0
			for i := 0; i < 4; i++ {
				s0 += int(left[i])
				s1 += int(left[4+i])
			}
			m0, m1 := uint8((s0+2)>>2), uint8((s1+2)>>2)
			for y := 0; y < 4; y++ {
				for x := 0; x < 8; x++ {
					set(x, y, m0)
				}
			}
			for y := 4; y < 8; y++ {
				for x := 0; x < 8; x++ {
					set(x, y, m1)
				}
			}
		} else {
			for i := range out {
				out[i] = 128
			}
		}
	case IntraPredHorizontal:
		if !hasLeft {
			return out, fmt.Errorf("%w: chroma left missing", ErrBadSliceHeader)
		}
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				set(x, y, left[y])
			}
		}
	case IntraPredVertical:
		if !hasTop {
			return out, fmt.Errorf("%w: chroma top missing", ErrBadSliceHeader)
		}
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				set(x, y, top[x])
			}
		}
	case IntraPredPlane:
		if !hasTop || !hasLeft {
			return out, fmt.Errorf("%w: chroma plane neighbours missing", ErrBadSliceHeader)
		}
		tl, ok := get(bx-1, by-1)
		if !ok {
			return out, fmt.Errorf("%w: chroma corner missing", ErrBadSliceHeader)
		}
		H, V := 0, 0
		for i := 0; i < 4; i++ {
			t0, l0 := int(tl), int(tl)
			if 2-i >= 0 {
				t0, l0 = int(top[2-i]), int(left[2-i])
			}
			H += (i + 1) * (int(top[4+i]) - t0)
			V += (i + 1) * (int(left[4+i]) - l0)
		}
		a := 16 * (int(left[7]) + int(top[7]))
		b := (17*H + 16) >> 5
		c := (17*V + 16) >> 5
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				v := (a + b*(x-3) + c*(y-3) + 16) >> 5
				if v < 0 {
					v = 0
				}
				if v > 255 {
					v = 255
				}
				set(x, y, uint8(v))
			}
		}
	default:
		return out, fmt.Errorf("%w: chroma mode %d", ErrBadSliceHeader, mode)
	}
	return out, nil
}

package h264

// P-frame inter layer (VR2b scope): Baseline P slices, single reference,
// CAVLC residual. Covers F3 I/P part, F7 partitions down to 4x4, F8
// quarter-pel with single ref, F18 skip, plus reference management hooks.
//
// Design mirrors the slice-data flow of the reference decoder
// (skip-run state machine, per-partition motion prediction, MC then
// residual): ideas only, all code written from scratch.

// golombToInterCBP maps CBP ue values for inter MBs.
var golombToInterCBP = [48]uint8{
	0, 16, 1, 2, 4, 8, 32, 3, 5, 10, 12, 15, 47, 7, 11, 13,
	14, 6, 9, 31, 35, 37, 42, 44, 33, 34, 36, 40, 39, 43, 45, 46,
	17, 18, 20, 24, 19, 21, 26, 28, 23, 27, 29, 30, 22, 25, 38, 41,
}

func median3(a, b, c int16) int16 {
	if a > b {
		a, b = b, a
	}
	if b > c {
		b = c
		if a > b {
			b = a
		}
	}
	return b
}

func clipInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// mvNeighbour reads one 4x4 motion slot. ref -1 means unavailable
// (outside picture, intra, or not yet decoded).
func (d *Decoder) mvNeighbour(bx, by int) (mx, my int16, ref int8) {
	if bx < 0 || by < 0 || bx >= d.mbW*4 || by >= d.mbH*4 {
		return 0, 0, -1
	}
	i := by*d.mbW*4 + bx
	r := d.refIdx[i]
	if r < 0 {
		return 0, 0, -1
	}
	return d.mvX[i], d.mvY[i], r
}

// predMotion predicts one partition at 4x4 origin (x0,y0) with width w4.
func (d *Decoder) predMotion(x0, y0, w4 int, ref int8) (int16, int16) {
	ax, ay, ar := d.mvNeighbour(x0-1, y0)
	bx, by, br := d.mvNeighbour(x0, y0-1)
	cx, cy, cr := d.mvNeighbour(x0+w4, y0-1)
	if cr < 0 {
		cx, cy, cr = d.mvNeighbour(x0-1, y0-1)
	}
	match := 0
	if ar == ref {
		match++
	}
	if br == ref {
		match++
	}
	if cr == ref {
		match++
	}
	switch {
	case match > 1:
		return median3(ax, bx, cx), median3(ay, by, cy)
	case match == 1:
		if ar == ref {
			return ax, ay
		}
		if br == ref {
			return bx, by
		}
		return cx, cy
	default:
		if br < 0 && cr < 0 && ar >= 0 {
			return ax, ay
		}
		return median3(ax, bx, cx), median3(ay, by, cy)
	}
}

// pred16x8Top predicts the top half of a P_16x8 MB at 4x4 origin (x0,y0).
func (d *Decoder) pred16x8Top(x0, y0 int, ref int8) (int16, int16) {
	if _, _, br := d.mvNeighbour(x0, y0-1); br == ref {
		mx, my, _ := d.mvNeighbour(x0, y0-1)
		return mx, my
	}
	return d.predMotion(x0, y0, 4, ref)
}

// pred16x8Bottom predicts the bottom half at origin (x0,y0+2).
func (d *Decoder) pred16x8Bottom(x0, y0 int, ref int8) (int16, int16) {
	if _, _, ar := d.mvNeighbour(x0-1, y0); ar == ref {
		mx, my, _ := d.mvNeighbour(x0-1, y0)
		return mx, my
	}
	return d.predMotion(x0, y0, 4, ref)
}

// pred8x16Left predicts the left half at origin (x0,y0).
func (d *Decoder) pred8x16Left(x0, y0 int, ref int8) (int16, int16) {
	if _, _, ar := d.mvNeighbour(x0-1, y0); ar == ref {
		mx, my, _ := d.mvNeighbour(x0-1, y0)
		return mx, my
	}
	return d.predMotion(x0, y0, 2, ref)
}

// pred8x16Right predicts the right half; px is its 4x4 origin (mbX0+2).
func (d *Decoder) pred8x16Right(px, py int, ref int8) (int16, int16) {
	if mx, my, cr := d.mvNeighbour(px+2, py-1); cr == ref {
		return mx, my
	}
	return d.predMotion(px, py, 2, ref)
}

// storeMV fills one luma rectangle (pixels) with a motion vector.
func (d *Decoder) storeMV(px, py, w, h int, mx, my int16, ref int8) {
	stride := d.mbW * 4
	for y := py / 4; y < (py+h)/4; y++ {
		for x := px / 4; x < (px+w)/4; x++ {
			i := y*stride + x
			d.mvX[i], d.mvY[i] = mx, my
			d.refIdx[i] = ref
		}
	}
}

// markIntraMB clears motion state for an intra MB in a P slice.
func (d *Decoder) markIntraMB(mbx, mby int) {
	stride := d.mbW * 4
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			d.refIdx[(mby*4+y)*stride+mbx*4+x] = -1
		}
	}
	d.mbIntra[mby*d.mbW+mbx] = true
}

// fullPel clips reference access with edge extension.
func fullPel(plane []uint8, w, h, x, y int) int {
	return int(plane[clipInt(y, 0, h-1)*w+clipInt(x, 0, w-1)])
}

// halfH is the 6-tap half-pel filter horizontally.
func halfH(plane []uint8, w, h, x, y int) int {
	p0 := fullPel(plane, w, h, x-2, y)
	p1 := fullPel(plane, w, h, x-1, y)
	p2 := fullPel(plane, w, h, x, y)
	p3 := fullPel(plane, w, h, x+1, y)
	p4 := fullPel(plane, w, h, x+2, y)
	p5 := fullPel(plane, w, h, x+3, y)
	return clipInt((p0+p5-5*(p1+p4)+20*(p2+p3)+16)>>5, 0, 255)
}

// halfV is the 6-tap half-pel filter vertically.
func halfV(plane []uint8, w, h, x, y int) int {
	p0 := fullPel(plane, w, h, x, y-2)
	p1 := fullPel(plane, w, h, x, y-1)
	p2 := fullPel(plane, w, h, x, y)
	p3 := fullPel(plane, w, h, x, y+1)
	p4 := fullPel(plane, w, h, x, y+2)
	p5 := fullPel(plane, w, h, x, y+3)
	return clipInt((p0+p5-5*(p1+p4)+20*(p2+p3)+16)>>5, 0, 255)
}

// centerJ is the half-half position: cascaded 6-tap without intermediate
// rounding, single +512>>10 step.
func centerJ(plane []uint8, w, h, x, y int) int {
	var tmp [6]int
	for i := 0; i < 6; i++ {
		yy := y - 2 + i
		p0 := fullPel(plane, w, h, x-2, yy)
		p1 := fullPel(plane, w, h, x-1, yy)
		p2 := fullPel(plane, w, h, x, yy)
		p3 := fullPel(plane, w, h, x+1, yy)
		p4 := fullPel(plane, w, h, x+2, yy)
		p5 := fullPel(plane, w, h, x+3, yy)
		tmp[i] = p0 + p5 - 5*(p1+p4) + 20*(p2+p3)
	}
	sum := tmp[0] + tmp[5] - 5*(tmp[1]+tmp[4]) + 20*(tmp[2]+tmp[3])
	return clipInt((sum+512)>>10, 0, 255)
}

func avg2(a, b int) int { return (a + b + 1) >> 1 }

// lumaSample interpolates one luma sample at quarter-pel frac (0..3).
func lumaSample(plane []uint8, w, h, x, y, fx, fy int) int {
	f := fullPel(plane, w, h, x, y)
	switch {
	case fx == 0 && fy == 0:
		return f
	case fy == 0:
		hh := halfH(plane, w, h, x, y)
		switch fx {
		case 1:
			return avg2(f, hh)
		case 2:
			return hh
		default:
			return avg2(hh, fullPel(plane, w, h, x+1, y))
		}
	case fx == 0:
		hv := halfV(plane, w, h, x, y)
		switch fy {
		case 1:
			return avg2(f, hv)
		case 2:
			return hv
		default:
			return avg2(hv, fullPel(plane, w, h, x, y+1))
		}
	}
	hh := halfH(plane, w, h, x, y)
	hv := halfV(plane, w, h, x, y)
	jj := centerJ(plane, w, h, x, y)
	hh1 := halfH(plane, w, h, x, y+1)
	hv1 := halfV(plane, w, h, x+1, y)
	switch {
	case fx == 2 && fy == 2:
		return jj
	case fy == 1 && fx == 1:
		return avg2(hh, hv)
	case fy == 1 && fx == 2:
		return avg2(hh, jj)
	case fy == 1:
		if fx == 3 {
			return avg2(hh, hv1)
		}
	case fy == 2 && fx == 1:
		return avg2(hv, jj)
	case fy == 2 && fx == 3:
		return avg2(jj, hv1)
	case fy == 3 && fx == 1:
		return avg2(hv, hh1)
	case fy == 3 && fx == 2:
		return avg2(hh1, jj)
	case fy == 3 && fx == 3:
		return avg2(hh1, hv1)
	}
	return avg2(hh, hv)
}

// predictLumaBlock copies one inter partition prediction from the reference.
func predictLumaBlock(ref *Picture, px, py, w, h int, mx, my int16) []uint8 {
	out := make([]uint8, w*h)
	if ref == nil {
		for i := range out {
			out[i] = 128
		}
		return out
	}
	rw, rh := int(ref.Width), int(ref.Height)
	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			qx := (px+dx)*4 + int(mx)
			qy := (py+dy)*4 + int(my)
			// Floor divide by 4 for possibly negative vectors.
			ix := qx >> 2
			iy := qy >> 2
			fx := qx - (ix << 2)
			fy := qy - (iy << 2)
			out[dy*w+dx] = uint8(lumaSample(ref.Y, rw, rh, ix, iy, fx, fy))
		}
	}
	return out
}

// predictChromaBlock interpolates one chroma partition. The chroma vector
// shares the luma integer (quarter units double as eighth units).
func predictChromaBlock(plane []uint8, w, h, px, py, cw, ch int, mx, my int16) []uint8 {
	out := make([]uint8, cw*ch)
	for dy := 0; dy < ch; dy++ {
		for dx := 0; dx < cw; dx++ {
			ex := (px+dx)*8 + int(mx)
			ey := (py+dy)*8 + int(my)
			ix := ex >> 3
			iy := ey >> 3
			fx := ex - (ix << 3)
			fy := ey - (iy << 3)
			a := fullPel(plane, w, h, ix, iy)
			b := fullPel(plane, w, h, ix+1, iy)
			c := fullPel(plane, w, h, ix, iy+1)
			d := fullPel(plane, w, h, ix+1, iy+1)
			v := ((8-fx)*(8-fy)*a + fx*(8-fy)*b + (8-fx)*fy*c + fx*fy*d + 32) >> 6
			out[dy*cw+dx] = uint8(clipInt(v, 0, 255))
		}
	}
	return out
}

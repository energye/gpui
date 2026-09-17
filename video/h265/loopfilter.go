package h265

// Loop filters for I slices: deblocking + SAO, applied in spec order
// (full-picture deblock first, then SAO from a deblocked snapshot).
//
// Peer (read-only, ideas only, no code copied):
//   hevc/filter.c deblocking_filter_CTB/boundary_strengths/
//   ff_hevc_hls_filter (Bs rules, beta/tc derivation, edge grids,
//   vertical-then-horizontal order, chroma gating) +
//   hevc/dsp_template.c hevc_loop_filter_luma/chroma +
//   h26x/h2656_deblock_template.c strong/weak formulas +
//   h26x/h2656_sao_template.c band/edge classification and offsets +
//   hevcdec.c hls_sao_param (offset inference: edge keeps first two
//   positive and negates the last two; band uses coded signs).
// Only table numbers and stage order travel; all code is fresh.
//
// Scope: single-slice I pictures, 8-bit 4:2:0, no tiles, PPS deblock
// control absent (zero beta/tc offsets, filter on). Anything else
// refuses honestly so other clips never silently misfilter.

import (
	"fmt"
)

// Deblock tables (standard HEVC beta/Tc tables; same discipline as
// the transform tables: numbers travel, logic is fresh).
var deblockBeta = [52]uint8{
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 6, 7, 8,
	9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 20, 22, 24, 26, 28, 30, 32, 34, 36,
	38, 40, 42, 44, 46, 48, 50, 52, 54, 56, 58, 60, 62, 64,
}

var deblockTc = [54]uint8{
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 4, 4, 4,
	5, 5, 6, 6, 7, 8, 9, 10, 11, 13, 14, 16, 18, 20, 22, 24,
}

func clipRange(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clipPixel(v int) byte {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return byte(v)
}

func absDiff(a, b int) int {
	if a < b {
		return b - a
	}
	return a - b
}

// buildQPYTab spreads each leaf's QP over the min-CB grid it covers.
// Leaves partition the picture, so every cell lands exactly once;
// the slice QP backs cells no leaf covers (never on this path).
func buildQPYTab(fs *FrameSyntax, s *SPS) ([]int, int, int, int) {
	minCB := 1 << s.Log2MinCB
	gw := (int(s.Width) + minCB - 1) / minCB
	gh := (int(s.Height) + minCB - 1) / minCB
	tab := make([]int, gw*gh)
	for i := range tab {
		tab[i] = int(fs.SH.SliceQP)
	}
	for _, l := range fs.Leaves {
		sz := 1 << l.Log2Size
		x1, y1 := l.X0+sz, l.Y0+sz
		if x1 > int(s.Width) {
			x1 = int(s.Width)
		}
		if y1 > int(s.Height) {
			y1 = int(s.Height)
		}
		for y := l.Y0; y < y1; y += minCB {
			for x := l.X0; x < x1; x += minCB {
				tab[(y/minCB)*gw+(x/minCB)] = int(l.QP)
			}
		}
	}
	return tab, gw, gh, minCB
}

func qpyAt(tab []int, gw, gh, minCB, w, h, x, y int) int {
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x >= w {
		x = w - 1
	}
	if y >= h {
		y = h - 1
	}
	cx, cy := x/minCB, y/minCB
	if cx >= gw {
		cx = gw - 1
	}
	if cy >= gh {
		cy = gh - 1
	}
	return tab[cy*gw+cx]
}

// chromaTcFor maps a luma QP average to a chroma Tc (PPS offsets are
// zero on this path; 4:2:0 mapping mirrors the pixel stage).
func chromaTcFor(qpY, cIdx int) int {
	qpi := clipRange(qpY, 0, 57)
	var qp int
	switch {
	case qpi < 30:
		qp = qpi
	case qpi > 43:
		qp = qpi - 6
	default:
		qp = chromaQP[qpi-30]
	}
	return int(deblockTc[clipRange(qp+2, 0, 53)])
}

// loopFilterChecks gates the fast path: I slice, 8-bit 4:2:0,
// default deblock control (on, zero offsets).
func loopFilterChecks(fs *FrameSyntax, s *SPS, q *PPS) error {
	if fs == nil || fs.SH == nil || s == nil || q == nil {
		return fmt.Errorf("%w: nil loop filter args", ErrBadSlice)
	}
	if fs.SH.Type != SliceI {
		return fmt.Errorf("%w: loop filter on non-I slice", ErrNotDecodable)
	}
	if s.BitDepth != 8 || s.BitDepthChroma != 8 {
		return fmt.Errorf("%w: loop filter bit depth %d/%d", ErrBadSPS, s.BitDepth, s.BitDepthChroma)
	}
	if s.ChromaFormat != 0 && s.ChromaFormat != 1 {
		return fmt.Errorf("%w: loop filter chroma %d", ErrBadSPS, s.ChromaFormat)
	}
	if q.DeblockControl {
		return fmt.Errorf("%w: deblock control on loop path", ErrBadSlice)
	}
	if q.TilesEnabled || q.EntropySync {
		return fmt.Errorf("%w: tiles on loop path", ErrBadSlice)
	}
	return nil
}

// LoopFilter runs deblocking then SAO over a reconstructed picture in
// place (spec order). The pre-filter pixels must already hold the
// Reconstruct output for fs.
func LoopFilter(pic *Picture, fs *FrameSyntax, s *SPS, q *PPS) error {
	if err := loopFilterChecks(fs, s, q); err != nil {
		return err
	}
	if pic == nil || pic.Width != int(s.Width) || pic.Height != int(s.Height) {
		return fmt.Errorf("%w: loop filter picture size", ErrBadSlice)
	}
	deblock(pic, fs, s)
	applySAO(pic, fs, s)
	return nil
}

// deblock filters luma on the 8-grid then chroma on the 16-grid.
// All-intra I slices carry Bs 2 on TU-boundary 8-grid edges (the peer
// assigns 2 whenever either side is intra) and 0 elsewhere, including
// 8-grid lines buried inside a large TU, so the boundary map comes
// from the leaf partition, not from the grid alone.
func deblock(pic *Picture, fs *FrameSyntax, s *SPS) {
	W, H := pic.Width, pic.Height
	tab, gw, gh, minCB := buildQPYTab(fs, s)
	at := func(x, y int) int { return qpyAt(tab, gw, gh, minCB, W, H, x, y) }
	// Leaf index grid at 4x4 (min TU): leaves partition the picture,
	// so an 8-grid line is a TU boundary iff the leaves on its two
	// sides differ. Chunk rows/cols sit on 4-multiples and leaf edges
	// sit on 4-multiples, so one probe row/col per chunk decides it.
	gw4 := (W + 3) / 4
	leafAt := make([]int, gw4*((H+3)/4))
	for i, l := range fs.Leaves {
		sz := 1 << l.Log2Size
		x1, y1 := l.X0+sz, l.Y0+sz
		if x1 > W {
			x1 = W
		}
		if y1 > H {
			y1 = H
		}
		for y := l.Y0; y < y1; y++ {
			for x := l.X0; x < x1; x++ {
				leafAt[(y/4)*gw4+(x/4)] = i
			}
		}
	}
	probe := func(x, y int) int {
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}
		if x >= W {
			x = W - 1
		}
		if y >= H {
			y = H - 1
		}
		return leafAt[(y/4)*gw4+(x/4)]
	}
	// bsV reports Bs for a vertical 4-row chunk beside edge x.
	bsV := func(x, y int) int {
		if probe(x-1, y) != probe(x, y) {
			return 2
		}
		return 0
	}
	// bsH reports Bs for a horizontal 4-col chunk below edge y.
	bsH := func(x, y int) int {
		if probe(x, y-1) != probe(x, y) {
			return 2
		}
		return 0
	}
	// Luma vertical edges.
	for x := 8; x < W; x += 8 {
		for y0 := 0; y0 < H; y0 += 8 {
			for j := 0; j < 2; j++ {
				ys := y0 + j*4
				if ys >= H || bsV(x, ys) == 0 {
					continue
				}
				qp := (at(x-1, ys) + at(x, ys) + 1) >> 1
				beta := int(deblockBeta[clipRange(qp, 0, 51)])
				tc := int(deblockTc[clipRange(qp+2, 0, 53)])
				n := 4
				if ys+n > H {
					n = H - ys
				}
				lumaChunkVert(pic.Y, W, x, ys, n, beta, tc)
			}
		}
	}
	// Luma horizontal edges (run after the full vertical pass).
	for y := 8; y < H; y += 8 {
		for x0 := 0; x0 < W; x0 += 8 {
			for j := 0; j < 2; j++ {
				xs := x0 + j*4
				if xs >= W || bsH(xs, y) == 0 {
					continue
				}
				qp := (at(xs, y-1) + at(xs, y) + 1) >> 1
				beta := int(deblockBeta[clipRange(qp, 0, 51)])
				tc := int(deblockTc[clipRange(qp+2, 0, 53)])
				n := 4
				if xs+n > W {
					n = W - xs
				}
				lumaChunkHoriz(pic.Y, W, H, xs, y, n, beta, tc)
			}
		}
	}
	if s.ChromaFormat != 1 {
		return
	}
	// Chroma only on the 16-grid, weak filter, Bs 2 implied.
	for c := 1; c <= 2; c++ {
		var plane []byte
		if c == 1 {
			plane = pic.Cb
		} else {
			plane = pic.Cr
		}
		cw := (W + 1) / 2
		ch := (H + 1) / 2
		for x := 16; x < W; x += 16 {
			for y0 := 0; y0 < H; y0 += 16 {
				for j := 0; j < 2; j++ {
					ys := y0 + j*8
					if ys >= H || bsV(x, ys) == 0 {
						continue
					}
					t := chromaTcFor((at(x-1, ys)+at(x, ys)+1)>>1, c)
					if t <= 0 {
						continue
					}
					// One 4-chroma-row chunk per 8 luma rows.
					cys := ys / 2
					n := 4
					if cys+n > ch {
						n = ch - cys
					}
					chromaChunkVert(plane, cw, x/2, cys, n, t)
				}
			}
		}
		for y := 16; y < H; y += 16 {
			for x0 := 0; x0 < W; x0 += 16 {
				for j := 0; j < 2; j++ {
					xs := x0 + j*8
					if xs >= W || bsH(xs, y) == 0 {
						continue
					}
					t := chromaTcFor((at(xs, y-1)+at(xs, y)+1)>>1, c)
					if t <= 0 {
						continue
					}
					cxs := xs / 2
					n := 4
					if cxs+n > cw {
						n = cw - cxs
					}
					chromaChunkHoriz(plane, cw, cxs, y/2, n, t)
				}
			}
		}
	}
}

// lumaChunkVert filters n rows (<=4) beside a vertical edge.
func lumaChunkVert(p []byte, W, x, y, n, beta, tc int) {
	if tc <= 0 && beta <= 0 {
		return
	}
	// Decision rows: first and last of the chunk.
	r0, r1 := y, y+n-1
	g := func(row, dx int) int { return int(p[row*W+x+dx]) }
	dp0 := absDiff(g(r0, -3)-2*g(r0, -2)+g(r0, -1), 0)
	dq0 := absDiff(g(r0, 2)-2*g(r0, 1)+g(r0, 0), 0)
	dp3 := absDiff(g(r1, -3)-2*g(r1, -2)+g(r1, -1), 0)
	dq3 := absDiff(g(r1, 2)-2*g(r1, 1)+g(r1, 0), 0)
	d0 := dp0 + dq0
	d3 := dp3 + dq3
	if d0+d3 >= beta {
		return
	}
	beta3, beta2 := beta>>3, beta>>2
	tc25 := (tc*5 + 1) >> 1
	strong := absDiff(g(r0, -4), g(r0, -1))+absDiff(g(r0, 3), g(r0, 0)) < beta3 &&
		absDiff(g(r0, -1), g(r0, 0)) < tc25 &&
		absDiff(g(r1, -4), g(r1, -1))+absDiff(g(r1, 3), g(r1, 0)) < beta3 &&
		absDiff(g(r1, -1), g(r1, 0)) < tc25 &&
		(d0<<1) < beta2 && (d3<<1) < beta2
	if strong {
		t2 := tc << 1
		for d := 0; d < n; d++ {
			row := y + d
			p3, p2, p1, p0 := g(row, -4), g(row, -3), g(row, -2), g(row, -1)
			q0, q1, q2, q3 := g(row, 0), g(row, 1), g(row, 2), g(row, 3)
			p[row*W+x-1] = clipPixel(p0 + clipRange(((p2+2*p1+2*p0+2*q0+q1+4)>>3)-p0, -t2, t2))
			p[row*W+x-2] = clipPixel(p1 + clipRange(((p2+p1+p0+q0+2)>>2)-p1, -t2, t2))
			p[row*W+x-3] = clipPixel(p2 + clipRange(((2*p3+3*p2+p1+p0+q0+4)>>3)-p2, -t2, t2))
			p[row*W+x] = clipPixel(q0 + clipRange(((p1+2*p0+2*q0+2*q1+q2+4)>>3)-q0, -t2, t2))
			p[row*W+x+1] = clipPixel(q1 + clipRange(((p0+q0+q1+q2+2)>>2)-q1, -t2, t2))
			p[row*W+x+2] = clipPixel(q2 + clipRange(((2*q3+3*q2+q1+q0+p0+4)>>3)-q2, -t2, t2))
		}
		return
	}
	ndP, ndQ := 1, 1
	if absDiff(g(r0, -3)-2*g(r0, -2)+g(r0, -1), 0)+absDiff(g(r1, -3)-2*g(r1, -2)+g(r1, -1), 0) < (beta+(beta>>1))>>3 {
		ndP = 2
	}
	if absDiff(g(r0, 2)-2*g(r0, 1)+g(r0, 0), 0)+absDiff(g(r1, 2)-2*g(r1, 1)+g(r1, 0), 0) < (beta+(beta>>1))>>3 {
		ndQ = 2
	}
	tc2 := tc >> 1
	for d := 0; d < n; d++ {
		row := y + d
		p2, p1, p0 := g(row, -3), g(row, -2), g(row, -1)
		q0, q1, q2 := g(row, 0), g(row, 1), g(row, 2)
		delta := (9*(q0-p0) - 3*(q1-p1) + 8) >> 4
		if absDiff(delta, 0) >= 10*tc {
			continue
		}
		delta = clipRange(delta, -tc, tc)
		p[row*W+x-1] = clipPixel(p0 + delta)
		p[row*W+x] = clipPixel(q0 - delta)
		if ndP > 1 {
			p[row*W+x-2] = clipPixel(p1 + clipRange((((p2+p0+1)>>1)-p1+delta)>>1, -tc2, tc2))
		}
		if ndQ > 1 {
			p[row*W+x+1] = clipPixel(q1 + clipRange((((q2+q0+1)>>1)-q1-delta)>>1, -tc2, tc2))
		}
	}
}

// lumaChunkHoriz filters n columns (<=4) below a horizontal edge.
func lumaChunkHoriz(p []byte, W, H, x, y, n, beta, tc int) {
	if tc <= 0 && beta <= 0 {
		return
	}
	_ = H
	g := func(dx, dy int) int { return int(p[(y+dy)*W+x+dx]) }
	c0, c1 := 0, n-1
	dp0 := absDiff(g(c0, -3)-2*g(c0, -2)+g(c0, -1), 0)
	dq0 := absDiff(g(c0, 2)-2*g(c0, 1)+g(c0, 0), 0)
	dp3 := absDiff(g(c1, -3)-2*g(c1, -2)+g(c1, -1), 0)
	dq3 := absDiff(g(c1, 2)-2*g(c1, 1)+g(c1, 0), 0)
	d0, d3 := dp0+dq0, dp3+dq3
	if d0+d3 >= beta {
		return
	}
	beta3, beta2 := beta>>3, beta>>2
	tc25 := (tc*5 + 1) >> 1
	strong := absDiff(g(c0, -4), g(c0, -1))+absDiff(g(c0, 3), g(c0, 0)) < beta3 &&
		absDiff(g(c0, -1), g(c0, 0)) < tc25 &&
		absDiff(g(c1, -4), g(c1, -1))+absDiff(g(c1, 3), g(c1, 0)) < beta3 &&
		absDiff(g(c1, -1), g(c1, 0)) < tc25 &&
		(d0<<1) < beta2 && (d3<<1) < beta2
	if strong {
		t2 := tc << 1
		for d := 0; d < n; d++ {
			col := x + d
			p3, p2, p1, p0 := g(d, -4), g(d, -3), g(d, -2), g(d, -1)
			q0, q1, q2, q3 := g(d, 0), g(d, 1), g(d, 2), g(d, 3)
			p[(y-1)*W+col] = clipPixel(p0 + clipRange(((p2+2*p1+2*p0+2*q0+q1+4)>>3)-p0, -t2, t2))
			p[(y-2)*W+col] = clipPixel(p1 + clipRange(((p2+p1+p0+q0+2)>>2)-p1, -t2, t2))
			p[(y-3)*W+col] = clipPixel(p2 + clipRange(((2*p3+3*p2+p1+p0+q0+4)>>3)-p2, -t2, t2))
			p[y*W+col] = clipPixel(q0 + clipRange(((p1+2*p0+2*q0+2*q1+q2+4)>>3)-q0, -t2, t2))
			p[(y+1)*W+col] = clipPixel(q1 + clipRange(((p0+q0+q1+q2+2)>>2)-q1, -t2, t2))
			p[(y+2)*W+col] = clipPixel(q2 + clipRange(((2*q3+3*q2+q1+q0+p0+4)>>3)-q2, -t2, t2))
		}
		return
	}
	ndP, ndQ := 1, 1
	if dp0+dp3 < (beta+(beta>>1))>>3 {
		ndP = 2
	}
	if dq0+dq3 < (beta+(beta>>1))>>3 {
		ndQ = 2
	}
	tc2 := tc >> 1
	for d := 0; d < n; d++ {
		col := x + d
		p2, p1, p0 := g(d, -3), g(d, -2), g(d, -1)
		q0, q1, q2 := g(d, 0), g(d, 1), g(d, 2)
		delta := (9*(q0-p0) - 3*(q1-p1) + 8) >> 4
		if absDiff(delta, 0) >= 10*tc {
			continue
		}
		delta = clipRange(delta, -tc, tc)
		p[(y-1)*W+col] = clipPixel(p0 + delta)
		p[y*W+col] = clipPixel(q0 - delta)
		if ndP > 1 {
			p[(y-2)*W+col] = clipPixel(p1 + clipRange((((p2+p0+1)>>1)-p1+delta)>>1, -tc2, tc2))
		}
		if ndQ > 1 {
			p[(y+1)*W+col] = clipPixel(q1 + clipRange((((q2+q0+1)>>1)-q1-delta)>>1, -tc2, tc2))
		}
	}
}

func chromaChunkVert(p []byte, cw, cx, cy, n, tc int) {
	if tc <= 0 {
		return
	}
	for d := 0; d < n; d++ {
		row := cy + d
		p1 := int(p[row*cw+cx-2])
		p0 := int(p[row*cw+cx-1])
		q0 := int(p[row*cw+cx])
		q1 := int(p[row*cw+cx+1])
		delta := clipRange(((q0-p0)*4+p1-q1+4)>>3, -tc, tc)
		p[row*cw+cx-1] = clipPixel(p0 + delta)
		p[row*cw+cx] = clipPixel(q0 - delta)
	}
}

func chromaChunkHoriz(p []byte, cw, cx, cy, n, tc int) {
	if tc <= 0 {
		return
	}
	for d := 0; d < n; d++ {
		col := cx + d
		p1 := int(p[(cy-2)*cw+col])
		p0 := int(p[(cy-1)*cw+col])
		q0 := int(p[cy*cw+col])
		q1 := int(p[(cy+1)*cw+col])
		delta := clipRange(((q0-p0)*4+p1-q1+4)>>3, -tc, tc)
		p[(cy-1)*cw+col] = clipPixel(p0 + delta)
		p[cy*cw+col] = clipPixel(q0 - delta)
	}
}

// saoOffsets infers the five filter offsets for one component: index
// 0 stays 0; edge keeps the first two positive and negates the last
// two; band applies the coded signs.
func saoOffsets(sa SAOParams, c int) [5]int {
	var o [5]int
	if sa.Type[c] == SAOBand {
		for i := 0; i < 4; i++ {
			v := int(sa.Abs[c][i])
			if sa.Sign[c][i] {
				v = -v
			}
			o[i+1] = v
		}
		return o
	}
	o[1] = int(sa.Abs[c][0])
	o[2] = int(sa.Abs[c][1])
	o[3] = -int(sa.Abs[c][2])
	o[4] = -int(sa.Abs[c][3])
	return o
}

var saoEdgeIdx = [5]int{1, 2, 0, 3, 4}

// applySAO filters each CTB from a deblocked snapshot (neighbors must
// read pre-SAO values, so the source stays frozen while dst fills).
func applySAO(pic *Picture, fs *FrameSyntax, s *SPS) {
	W, H := pic.Width, pic.Height
	ctb := 1 << s.Log2MaxCB
	ctbW := (W + ctb - 1) / ctb
	srcY := append([]byte(nil), pic.Y...)
	var srcCb, srcCr []byte
	if s.ChromaFormat == 1 {
		srcCb = append([]byte(nil), pic.Cb...)
		srcCr = append([]byte(nil), pic.Cr...)
	}
	for ry := 0; ry*ctb < H; ry++ {
		for rx := 0; rx*ctb < W; rx++ {
			sa := fs.SAO[ry*ctbW+rx]
			x0, y0 := rx*ctb, ry*ctb
			w := ctb
			if x0+w > W {
				w = W - x0
			}
			h := ctb
			if y0+h > H {
				h = H - y0
			}
			if fs.SH.SAOLuma && sa.Type[0] != SAOOff {
				saoCTB(pic.Y, srcY, W, H, x0, y0, w, h, sa, 0)
			}
			if s.ChromaFormat != 1 || !fs.SH.SAOChroma {
				continue
			}
			cw := (W + 1) / 2
			ch := (H + 1) / 2
			cx0, cy0 := x0/2, y0/2
			cw0, ch0 := w/2, h/2
			if sa.Type[1] != SAOOff {
				saoCTB(pic.Cb, srcCb, cw, ch, cx0, cy0, cw0, ch0, sa, 1)
			}
			// Cr shares Cb's type in syntax, but keeps its own
			// offsets; the stored type mirrors Cb by construction.
			if sa.Type[2] != SAOOff {
				saoCTB(pic.Cr, srcCr, cw, ch, cx0, cy0, cw0, ch0, sa, 2)
			}
		}
	}
}

// saoCTB filters one component rect reading src, writing dst.
func saoCTB(dst, src []byte, W, H, x0, y0, w, h int, sa SAOParams, c int) {
	off := saoOffsets(sa, c)
	if sa.Type[c] == SAOBand {
		pos := int(sa.BandPos[c])
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				v := int(src[(y0+y)*W+x0+x])
				band := (v >> 3) & 31
				if band >= pos && band < pos+4 {
					dst[(y0+y)*W+x0+x] = clipPixel(v + off[band-pos+1])
				}
			}
		}
		return
	}
	eo := int(sa.EOClass[c])
	var ax, ay, bx, by int
	switch eo {
	case 0:
		ax, ay, bx, by = -1, 0, 1, 0
	case 1:
		ax, ay, bx, by = 0, -1, 0, 1
	case 2:
		ax, ay, bx, by = -1, -1, 1, 1
	default:
		ax, ay, bx, by = 1, -1, -1, 1
	}
	cmp := func(a, b int) int {
		if a > b {
			return 1
		}
		if a < b {
			return -1
		}
		return 0
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			px, py := x0+x, y0+y
			nax, nay := px+ax, py+ay
			nbx, nby := px+bx, py+by
			// Picture borders keep the deblocked value (the
			// peer restores them with offset 0 on this path).
			if nax < 0 || nay < 0 || nax >= W || nay >= H ||
				nbx < 0 || nby < 0 || nbx >= W || nby >= H {
				continue
			}
			v := int(src[py*W+px])
			d := 2 + cmp(v, int(src[nay*W+nax])) + cmp(v, int(src[nby*W+nbx]))
			dst[py*W+px] = clipPixel(v + off[saoEdgeIdx[d]])
		}
	}
}

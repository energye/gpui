package h265

import (
	"fmt"
)

// P-slice loop filter: deblocking with motion/cbf boundary strengths
// plus the shared SAO stage, applied in spec order like the I path.
//
// Peer (read-only, ideas only, no code copied):
//   filter.c boundary_strength (ref-POC compare, quarter-pel >= 4
//   threshold, BI branches) + deblocking_boundary_strengths (TU-edge
//   vs internal-PU paths, cbf gate only on TU edges, chroma only on
//   Bs 2) + ff_hevc_hls_filter dispatch (luma on Bs>=1, chroma on
//   Bs==2). Only gates and numbers travel; all code is fresh.
//
// Scope: single-slice P pictures, 8-bit 4:2:0, L0-only motion, no
// tiles, default deblock control. Intra CUs inside P refuse honestly
// (this clip has none); B slices refuse at the RPL build.

// motionBs mirrors boundary_strength for L0-only P motion: different
// reference POCs give 1; same reference with a quarter-pel diff of 4
// or more (>= 1 pixel) gives 1; else 0. BI branches never fire here.
func (d *interDec) motionBs(a, b mvCand, rpl []refPic) int {
	if a.pred == motionIntra || b.pred == motionIntra {
		return 2
	}
	pa, pb := -1, -1
	if a.ref0 < len(rpl) {
		pa = rpl[a.ref0].poc
	}
	if b.ref0 < len(rpl) {
		pb = rpl[b.ref0].poc
	}
	if pa != pb {
		return 1
	}
	dx := int(a.x0) - int(b.x0)
	if dx < 0 {
		dx = -dx
	}
	dy := int(a.y0) - int(b.y0)
	if dy < 0 {
		dy = -dy
	}
	if dx >= 4 || dy >= 4 {
		return 1
	}
	return 0
}

// buildCbfGrid spreads CbfLuma over the min-TB grid (peer cbf_luma;
// skip CUs own no leaves, so their cells stay 0).
func (d *interDec) buildCbfGrid(fs *FrameSyntax) []bool {
	tb := 1 << d.log2MinTB
	gw := (int(d.sps.Width) + tb - 1) / tb
	gh := (int(d.sps.Height) + tb - 1) / tb
	grid := make([]bool, gw*gh)
	for _, l := range fs.Leaves {
		if !l.CbfLuma {
			continue
		}
		sz := 1 << l.Log2Size
		x1, y1 := l.X0+sz, l.Y0+sz
		if x1 > int(d.sps.Width) {
			x1 = int(d.sps.Width)
		}
		if y1 > int(d.sps.Height) {
			y1 = int(d.sps.Height)
		}
		for y := l.Y0; y < y1; y += tb {
			for x := l.X0; x < x1; x += tb {
				grid[(y/tb)*gw+(x/tb)] = true
			}
		}
	}
	return grid
}

// bsMaps holds boundary strengths per 4-pixel chunk, keyed like the
// peer's bs arrays: vertical[(x,y)] for the chunk right of edge x,
// horizontal[(x,y)] for the chunk below edge y. Later writes win, so
// callers replay decode order (peer overwrites in place).
type bsMaps struct {
	v, h map[[2]int]int
}

// bsCall mirrors ff_hevc_deblocking_boundary_strengths for one
// (x0,y0,log2size) block: TU-edge path on the 8-grid (cbf gate, else
// motion compare) plus internal-PU path for blocks wider than a min
// PU (pure motion compare, no cbf gate). Single-slice/single-tile
// pictures skip the slice/tile gating (no boundaries inside).
func (d *interDec) bsCall(m *bsMaps, fs *FrameSyntax, grid *mvGrid, rpl []refPic, cbf []bool, x0, y0, log2size int) {
	W, H := int(d.sps.Width), int(d.sps.Height)
	tb := 1 << d.log2MinTB
	tbW := (W + tb - 1) / tb
	cbfAt := func(x, y int) bool {
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
		return cbf[(y/tb)*tbW+(x/tb)]
	}
	motAt := func(x, y int) mvCand {
		if f := d.gridAt(grid, x, y); f != nil {
			return *f
		}
		return mvCand{}
	}
	size := 1 << log2size
	isIntra := false
	if f := d.gridAt(grid, x0, y0); f != nil && f.pred == motionIntra {
		isIntra = true
	}
	_ = fs
	if y0 > 0 && y0&7 == 0 {
		yp, yq := y0-1, y0
		for i := 0; i < size; i += 4 {
			x := x0 + i
			a, b := motAt(x, yp), motAt(x, yq)
			var bs int
			if a.pred == motionIntra || b.pred == motionIntra {
				bs = 2
			} else if cbfAt(x, yp) || cbfAt(x, yq) {
				bs = 1
			} else {
				bs = d.motionBs(a, b, rpl)
			}
			m.h[[2]int{x, y0}] = bs
		}
	}
	if x0 > 0 && x0&7 == 0 {
		xp, xq := x0-1, x0
		for i := 0; i < size; i += 4 {
			y := y0 + i
			a, b := motAt(xp, y), motAt(xq, y)
			var bs int
			if a.pred == motionIntra || b.pred == motionIntra {
				bs = 2
			} else if cbfAt(xp, y) || cbfAt(xq, y) {
				bs = 1
			} else {
				bs = d.motionBs(a, b, rpl)
			}
			m.v[[2]int{x0, y}] = bs
		}
	}
	if log2size > d.log2MinPU && !isIntra {
		for j := 8; j < size; j += 8 {
			for i := 0; i < size; i += 4 {
				x := x0 + i
				a, b := motAt(x, y0+j-1), motAt(x, y0+j)
				m.h[[2]int{x, y0 + j}] = d.motionBs(a, b, rpl)
			}
		}
		for j := 0; j < size; j += 4 {
			y := y0 + j
			for i := 8; i < size; i += 8 {
				a, b := motAt(x0+i-1, y), motAt(x0+i, y)
				m.v[[2]int{x0 + i, y}] = d.motionBs(a, b, rpl)
			}
		}
	}
}

// buildBs replays the peer's boundary-strength calls in decode order:
// skip and no-residual CUs file one CB-sized call; CUs with a tree
// file one call per transform leaf in walk order (later writes win,
// like the peer's in-place arrays).
func (d *interDec) buildBs(fs *FrameSyntax, grid *mvGrid, rpl []refPic) *bsMaps {
	m := &bsMaps{v: map[[2]int]int{}, h: map[[2]int]int{}}
	cbf := d.buildCbfGrid(fs)
	leavesOf := func(cu *CUInfo) []TULeaf {
		cb := 1 << cu.Log2Size
		var out []TULeaf
		for _, l := range fs.Leaves {
			if l.X0 >= cu.X0 && l.Y0 >= cu.Y0 && l.X0 < cu.X0+cb && l.Y0 < cu.Y0+cb {
				out = append(out, l)
			}
		}
		return out
	}
	for i := range fs.CUs {
		cu := &fs.CUs[i]
		cb := 1 << cu.Log2Size
		_ = cb
		if cu.Pred == PredSkip || cu.NoResid {
			d.bsCall(m, fs, grid, rpl, cbf, cu.X0, cu.Y0, int(cu.Log2Size))
			continue
		}
		for _, l := range leavesOf(cu) {
			d.bsCall(m, fs, grid, rpl, cbf, l.X0, l.Y0, l.Log2Size)
		}
	}
	return m
}

// filterPChecks gates the P fast path (mirrors loopFilterChecks plus
// the motion scope; intra CUs refuse so no block goes black).
func (d *interDec) filterPChecks(fs *FrameSyntax, q *PPS) error {
	if fs == nil || fs.SH == nil || q == nil {
		return fmt.Errorf("%w: nil P filter args", ErrBadSlice)
	}
	if fs.SH.Type != SliceP {
		return fmt.Errorf("%w: P filter on non-P slice", ErrNotDecodable)
	}
	if d.sps.BitDepth != 8 || d.sps.BitDepthChroma != 8 {
		return fmt.Errorf("%w: P filter bit depth", ErrBadSPS)
	}
	if d.sps.ChromaFormat != 1 {
		return fmt.Errorf("%w: P filter chroma %d", ErrBadSPS, d.sps.ChromaFormat)
	}
	if q.DeblockControl || q.TilesEnabled || q.EntropySync {
		return fmt.Errorf("%w: P filter control/tiles", ErrBadSlice)
	}
	for i := range fs.CUs {
		if fs.CUs[i].Pred == PredIntra {
			return fmt.Errorf("%w: intra CU in P filter", ErrNotDecodable)
		}
	}
	return nil
}

// FilterP runs deblocking (motion/cbf strengths) then SAO over a
// reconstructed P picture in place. grid holds the derived motion,
// rpl the frame's L0 list for ref-POC compares.
func (d *interDec) FilterP(pic *Picture, fs *FrameSyntax, q *PPS, grid *mvGrid, rpl []refPic) error {
	if err := d.filterPChecks(fs, q); err != nil {
		return err
	}
	if pic == nil || pic.Width != int(d.sps.Width) || pic.Height != int(d.sps.Height) {
		return fmt.Errorf("%w: P filter picture size", ErrBadSlice)
	}
	d.deblockP(pic, fs, grid, rpl)
	applySAO(pic, fs, d.sps)
	return nil
}

// deblockP filters luma on the 8-grid from the replayed Bs map (peer
// ff_hevc_hls_filter dispatch: luma runs on Bs>=1 per 4-row chunk,
// chroma only on Bs 2 — unreachable on P without intra, so luma
// only). Chroma never filters here.
func (d *interDec) deblockP(pic *Picture, fs *FrameSyntax, grid *mvGrid, rpl []refPic) {
	W, H := pic.Width, pic.Height
	tab, gw, gh, minCB := buildQPYTab(fs, d.sps)
	at := func(x, y int) int { return qpyAt(tab, gw, gh, minCB, W, H, x, y) }
	bs := d.buildBs(fs, grid, rpl)
	// Luma vertical edges.
	for x := 8; x < W; x += 8 {
		for y0 := 0; y0 < H; y0 += 8 {
			for j := 0; j < 2; j++ {
				ys := y0 + j*4
				if ys >= H {
					continue
				}
				b := bs.v[[2]int{x, ys}]
				if b == 0 {
					continue
				}
				qp := (at(x-1, ys) + at(x, ys) + 1) >> 1
				beta := int(deblockBeta[clipRange(qp, 0, 51)])
				tc := int(deblockTc[clipRange(qp+2*(b-1), 0, 53)])
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
				if xs >= W {
					continue
				}
				b := bs.h[[2]int{xs, y}]
				if b == 0 {
					continue
				}
				qp := (at(xs, y-1) + at(xs, y) + 1) >> 1
				beta := int(deblockBeta[clipRange(qp, 0, 51)])
				tc := int(deblockTc[clipRange(qp+2*(b-1), 0, 53)])
				n := 4
				if xs+n > W {
					n = W - xs
				}
				lumaChunkHoriz(pic.Y, W, H, xs, y, n, beta, tc)
			}
		}
	}
}

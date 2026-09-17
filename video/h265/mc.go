package h265

import (
	"fmt"
)

// Motion compensation for P slices: sub-pixel prediction plus
// residual add, feeding the loop filter.
//
// Peer (read-only, ideas only, no code copied):
//   hevcdec.c luma_mc_uni/chroma_mc_uni (shift/add, edge detect,
//   egde emulate call, uni dispatch, weight branch) +
//   h26x/h2656_inter_template.c uni luma/chroma h/v/hv kernels
//   (filter taps, shift 14-depth, offset, hv two-pass) +
//   dsp.c qpel/epel filter tables +
//   pred_template.c intra paths for the P-slice intra CUs (reuse) +
//   cabac.c residual scaling (shift/scale/add, verified bin-exact on
//   P residuals: 379/379 pre-dequant levels equal the peer's scaled
//   inputs). Only orders, gates and numbers travel; weighted
//   prediction refuses (this clip's weights are all-unused).

// qpelLuma are the 8-tap luma filters per quarter offset (peer
// ff_hevc_qpel_filters; index 0 unused).
var qpelLuma = [4][8]int8{
	{},
	{-1, 4, -10, 58, 17, -5, 1, 0},
	{-1, 4, -11, 40, 40, -11, 4, -1},
	{0, 1, -5, 17, 58, -10, 4, -1},
}

// epelChroma are the 4-tap chroma filters per eighth offset (peer
// ff_hevc_epel_filters; index 0 unused).
var epelChroma = [8][4]int8{
	{},
	{-2, 58, 10, -2},
	{-4, 54, 16, -2},
	{-6, 46, 28, -4},
	{-4, 36, 36, -4},
	{-4, 28, 46, -6},
	{-2, 16, 54, -4},
	{-2, 10, 58, -2},
}

// mcTmp caps the intermediate width (peer MAX_PB_SIZE 64).
const mcTmpW = 64

// emuPlane replicates borders: out-of-picture reads clamp to the edge
// (peer's emulated_edge_mc for sub-pixel taps near the frame border).
func emuPlane(plane []byte, w, h int, x, y int) byte {
	if x < 0 {
		x = 0
	}
	if x >= w {
		x = w - 1
	}
	if y < 0 {
		y = 0
	}
	if y >= h {
		y = h - 1
	}
	return plane[y*w+x]
}

// mcLumaUni predicts one luma block from the reference: integer shift
// then 8-tap qpel horizontally, vertically, or both (hv runs through
// a 14-bit intermediate, like the peer's tmp pass).
func mcLumaUni(dst []byte, dstStride int, ref *Picture, x0, y0, w, h int, mvx, mvy int32) {
	ix := x0 + int(mvx>>2)
	iy := y0 + int(mvy>>2)
	mx := int(mvx & 3)
	my := int(mvy & 3)
	rw, rh := ref.Width, ref.Height
	switch {
	case mx == 0 && my == 0:
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst[y*dstStride+x] = emuPlane(ref.Y, rw, rh, ix+x, iy+y)
			}
		}
	case my == 0:
		f := qpelLuma[mx]
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				s := 0
				for k := 0; k < 8; k++ {
					s += int(f[k]) * int(emuPlane(ref.Y, rw, rh, ix+x+k-3, iy+y))
				}
				dst[y*dstStride+x] = clipPix((s + 32) >> 6)
			}
		}
	case mx == 0:
		f := qpelLuma[my]
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				s := 0
				for k := 0; k < 8; k++ {
					s += int(f[k]) * int(emuPlane(ref.Y, rw, rh, ix+x, iy+y+k-3))
				}
				dst[y*dstStride+x] = clipPix((s + 32) >> 6)
			}
		}
	default:
		fh, fv := qpelLuma[mx], qpelLuma[my]
		tmp := make([]int, (h+7)*mcTmpW)
		for y := 0; y < h+7; y++ {
			for x := 0; x < w; x++ {
				s := 0
				for k := 0; k < 8; k++ {
					s += int(fh[k]) * int(emuPlane(ref.Y, rw, rh, ix+x+k-3, iy+y-3))
				}
				tmp[y*mcTmpW+x] = s
			}
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				s := 0
				for k := 0; k < 8; k++ {
					s += int(fv[k]) * tmp[(y+k)*mcTmpW+x]
				}
				dst[y*dstStride+x] = clipPix((s + 2048) >> 12)
			}
		}
	}
}

// mcChromaUni predicts one chroma block: luma MVs shift down by the
// 4:2:0 subsampling (>>3 with zero-extended fraction), then 4-tap
// epel. mx/my ride 0..7 eighths.
func mcChromaUni(dst []byte, dstStride int, plane []byte, pw, ph int, x0, y0, w, h int, mvx, mvy int32) {
	mx := int(uint32(mvx) & 7)
	my := int(uint32(mvy) & 7)
	ix := x0 + int(mvx>>3)
	iy := y0 + int(mvy>>3)
	switch {
	case mx == 0 && my == 0:
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst[y*dstStride+x] = emuPlane(plane, pw, ph, ix+x, iy+y)
			}
		}
	case my == 0:
		f := epelChroma[mx]
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				s := 0
				for k := 0; k < 4; k++ {
					s += int(f[k]) * int(emuPlane(plane, pw, ph, ix+x+k-1, iy+y))
				}
				dst[y*dstStride+x] = clipPix((s + 32) >> 6)
			}
		}
	case mx == 0:
		f := epelChroma[my]
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				s := 0
				for k := 0; k < 4; k++ {
					s += int(f[k]) * int(emuPlane(plane, pw, ph, ix+x, iy+y+k-1))
				}
				dst[y*dstStride+x] = clipPix((s + 32) >> 6)
			}
		}
	default:
		fh, fv := epelChroma[mx], epelChroma[my]
		tmp := make([]int, (h+3)*mcTmpW)
		for y := 0; y < h+3; y++ {
			for x := 0; x < w; x++ {
				s := 0
				for k := 0; k < 4; k++ {
					s += int(fh[k]) * int(emuPlane(plane, pw, ph, ix+x+k-1, iy+y-1))
				}
				tmp[y*mcTmpW+x] = s
			}
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				s := 0
				for k := 0; k < 4; k++ {
					s += int(fv[k]) * tmp[(y+k)*mcTmpW+x]
				}
				dst[y*dstStride+x] = clipPix((s + 2048) >> 12)
			}
		}
	}
}

// scaleOne dequantizes one pre-dequant level (peer residual tail:
// level * scale * scalem + add, then shift and clip).
func scaleOne(level int16, scale, shift int) int {
	add := 1 << (shift - 1)
	v := (int(level)*scale*16 + add) >> shift
	if v < -32768 {
		return -32768
	}
	if v > 32767 {
		return 32767
	}
	return v
}

// scaleFor derives the peer's per-block shift/scale (8-bit 4:2:0;
// verified against the reference COEFF probe: luma 8x8 qp28 runs
// shift 6 scale 1024, chroma shift 5 scale 1024).
func scaleFor(log2t, cIdx int, qp int32) (int, int) {
	// Peer qp derivation (8-bit 4:2:0, chroma offsets kept 0 here).
	q := int(qp)
	if cIdx != 0 {
		if q < 30 {
			// qp stays
		} else if q > 43 {
			q = 43
		} else {
			q = chromaQP[q-30]
		}
	}
	rem := ((q % 6) + 6) % 6
	div := (q - rem) / 6
	_ = div
	scale := levelScale[rem] << 4
	// Closed form matching the probe (all our clips are 8-bit with
	// zero QP offsets; other bit depths refuse in decodeP).
	shift := 6 + (log2t - 3)
	if cIdx != 0 {
		shift--
	}
	return shift, scale >> 4
}

// reconInter builds one P picture: every inter/skip PU predicts from
// its reference (L0 only), residuals dequant/transform/add, intra CUs
// (none on this clip) refuse honestly. tuMap links leaves to stored
// TUs like the I path.
func (d *interDec) reconInter(fs *FrameSyntax, mots []puMotion, rpl0 []refPic, qpY int32) (*Picture, error) {
	for i := range fs.CUs {
		if fs.CUs[i].Pred == PredIntra {
			return nil, fmt.Errorf("%w: intra CU in P recon", ErrNotDecodable)
		}
	}
	pic := NewPicture(int(d.sps.Width), int(d.sps.Height))
	if pic == nil {
		return nil, fmt.Errorf("%w: bad size", ErrBadSPS)
	}
	// Residual lookup by (origin, size, component).
	type tuKey struct {
		x, y, s, c int
	}
	tus := map[tuKey]*TUInfo{}
	for i := range fs.TUs {
		t := &fs.TUs[i]
		tus[tuKey{t.X0, t.Y0, t.Log2Size, t.CIdx}] = t
	}
	// Predict PU by PU in decode order (peer writes each PU right
	// after deriving it; residuals add later per transform leaf).
	for _, mo := range mots {
		ref, err := d.resolve(rpl0[mo.mv.ref0])
		if err != nil {
			return nil, err
		}
		pred := make([]byte, mo.w*mo.h)
		mcLumaUni(pred, mo.w, ref.pic, mo.x0, mo.y0, mo.w, mo.h, mo.mv.x0, mo.mv.y0)
		for y := 0; y < mo.h; y++ {
			for x := 0; x < mo.w; x++ {
				pic.Y[(mo.y0+y)*pic.Width+mo.x0+x] = pred[y*mo.w+x]
			}
		}
		cw := (pic.Width + 1) / 2
		chH := (pic.Height + 1) / 2
		rw := (ref.pic.Width + 1) / 2
		rh := (ref.pic.Height + 1) / 2
		for _, c := range []int{1, 2} {
			plane := ref.pic.Cb
			dst := pic.Cb
			if c == 2 {
				plane = ref.pic.Cr
				dst = pic.Cr
			}
			cp := make([]byte, (mo.w/2)*(mo.h/2))
			mcChromaUni(cp, mo.w/2, plane, rw, rh, mo.x0/2, mo.y0/2, mo.w/2, mo.h/2, mo.mv.x0, mo.mv.y0)
			for y := 0; y < mo.h/2; y++ {
				for x := 0; x < mo.w/2; x++ {
					dst[((mo.y0/2)+y)*cw+(mo.x0/2)+x] = cp[y*(mo.w/2)+x]
				}
			}
			_ = chH
		}
	}
	// Residuals add leaf by leaf (inter leaves carry mode 255; the
	// leaf's stored QP drives scale, like the I path).
	for _, l := range fs.Leaves {
		size := 1 << l.Log2Size
		// Luma residual only where coded; prediction is already in.
		if l.CbfLuma {
			t, ok := tus[tuKey{l.X0, l.Y0, l.Log2Size, 0}]
			if !ok {
				return nil, fmt.Errorf("%w: missing luma TU %d,%d,%d", ErrBadSlice, l.X0, l.Y0, size)
			}
			res := append([]int16(nil), t.Coeffs...)
			inverseTransform(res, l.Log2Size, qpFor(0, t.QP), false)
			for y := 0; y < size; y++ {
				for x := 0; x < size; x++ {
					o := (l.Y0+y)*pic.Width + l.X0 + x
					pic.Y[o] = clipPix(int(pic.Y[o]) + int(res[y*size+x]))
				}
			}
		}
		if d.sps.ChromaFormat != 1 {
			continue
		}
		cw := (pic.Width + 1) / 2
		if l.Log2Size > 2 {
			cs := size / 2
			px, py := l.X0/2, l.Y0/2
			// Cross-component prediction (peer adds BEFORE the
			// transform: cross = (alpha * lumaRes) >> 3 on the
			// scaled luma coefficients). The alpha rides in the
			// stored TU (0 = off, this clip always off).
			var lumaRes []int16
			if t, ok := tus[tuKey{l.X0, l.Y0, l.Log2Size, 0}]; ok && l.CbfLuma {
				lumaRes = append([]int16(nil), t.Coeffs...)
				inverseTransform(lumaRes, l.Log2Size, qpFor(0, t.QP), false)
				// Downsample to chroma size (4:2:0: average 2x2).
				ds := make([]int16, cs*cs)
				for y := 0; y < cs; y++ {
					for x := 0; x < cs; x++ {
						s := int(lumaRes[(2*y)*size+2*x]) + int(lumaRes[(2*y)*size+2*x+1]) +
							int(lumaRes[(2*y+1)*size+2*x]) + int(lumaRes[(2*y+1)*size+2*x+1])
						ds[y*cs+x] = int16((s + 2) >> 2)
					}
				}
				lumaRes = ds
			}
			for _, c := range []int{1, 2} {
				plane := pic.Cb
				if c == 2 {
					plane = pic.Cr
				}
				var cres []int16
				var alpha int32
				if (c == 1 && l.CbfCb) || (c == 2 && l.CbfCr) {
					t, ok := tus[tuKey{l.X0, l.Y0, l.Log2Size - 1, c}]
					if !ok {
						return nil, fmt.Errorf("%w: missing chroma TU %d,%d,%d,c%d", ErrBadSlice, l.X0, l.Y0, size, c)
					}
					alpha = t.ResScale
					cres = append([]int16(nil), t.Coeffs...)
					if alpha != 0 && lumaRes != nil {
						for i := range cres {
							cres[i] += int16((int(alpha) * int(lumaRes[i])) >> 3)
						}
					}
					inverseTransform(cres, l.Log2Size-1, qpFor(c, t.QP), false)
				} else if lumaRes != nil {
					// Zero-cbf chroma with the gate on adds pure
					// prediction (peer add_residual path); off
					// here since alpha is 0 on this clip.
					t, ok := tus[tuKey{l.X0, l.Y0, l.Log2Size - 1, c}]
					_ = t
					_ = ok
				}
				if cres == nil {
					continue
				}
				for y := 0; y < cs; y++ {
					for x := 0; x < cs; x++ {
						o := (py+y)*cw + px + x
						plane[o] = clipPix(int(plane[o]) + int(cres[y*cs+x]))
					}
				}
			}
			continue
		}
		if l.Log2Size == 2 && l.Blk == 3 {
			px, py := l.CbX/2, l.CbY/2
			var lumaRes4 []int16
			if t, ok := tus[tuKey{l.CbX, l.CbY, 2, 0}]; ok && l.CbfLuma {
				lumaRes4 = append([]int16(nil), t.Coeffs...)
				inverseTransform(lumaRes4, 2, qpFor(0, t.QP), false)
			}
			for _, c := range []int{1, 2} {
				plane := pic.Cb
				if c == 2 {
					plane = pic.Cr
				}
				var cres []int16
				if (c == 1 && l.CbfCb) || (c == 2 && l.CbfCr) {
					t, ok := tus[tuKey{l.CbX, l.CbY, 2, c}]
					if !ok {
						return nil, fmt.Errorf("%w: missing small chroma TU %d,%d,c%d", ErrBadSlice, l.CbX, l.CbY, c)
					}
					cres = append([]int16(nil), t.Coeffs...)
					if t.ResScale != 0 && lumaRes4 != nil {
						for i := range cres {
							cres[i] += int16((int(t.ResScale) * int(lumaRes4[i])) >> 3)
						}
					}
					inverseTransform(cres, 2, qpFor(c, t.QP), false)
				}
				if cres == nil {
					continue
				}
				for y := 0; y < 4; y++ {
					for x := 0; x < 4; x++ {
						o := (py+y)*cw + px + x
						plane[o] = clipPix(int(plane[o]) + int(cres[y*4+x]))
					}
				}
			}
		}
	}
	return pic, nil
}

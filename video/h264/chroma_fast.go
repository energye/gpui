package h264

// S1b-A1 chroma fast path (dispatch + interior scalar留守).
//
// Peer (ffmpeg, read-only, ideas only, no code copied):
//   libavcodec/h264chroma_template.c:29-70 FUNCC(ff_*_h264_chroma_mc4/8)
//   (A=(8-x)*(8-y) etc, (A*a+B*b+C*c+D*d+32)>>6, our weights below) vs
//   our chroma_amd64.s row kernel; oracle is the scalar留守 itself
//   (predictChromaBlock in inter.go, no new vectors here, VR2 exact
//   pins pixels).
// Difference: ffmpeg pads reference edges once (emulated-edge buffers);
// we take the fast path only for interior blocks (all four taps inside,
// no per-tap clipping) and fall back to the scalar留守 for edge blocks.
// Ours: inter.go predictChromaBlockScalar-equivalent (留守) + this file
// (dispatch); amd64 block code lives in chroma_amd64.go with its kernel
// in chroma_amd64.s; arm64 + other archs run the interior scalar fast
// path (no per-tap clipping, bit-identical on interior) via
// chroma_arm64.go / chroma_fallback.go. Output is bit-identical to the
// scalar留守 (chroma_s1_test.go pins all fracs). Set
// GPUI_SCALAR_CONVERT=1 to force scalar (same switch as qpel).

// chromaFast runs one sub-pel chroma partition through the arch block
// path. It reports false (caller falls back to the scalar留守) for
// integer motion (handled by the row-copy path), forced-scalar mode,
// edge-touching blocks, and bad sizes.
func chromaFast(plane []byte, w, h, px, py, cw, ch int, mx, my int16, out []byte) bool {
	if qpelScalarForced {
		return false
	}
	if cw <= 0 || ch <= 0 || cw > 16 || ch > 16 {
		return false
	}
	if len(out) < cw*ch || len(plane) < w*h {
		return false
	}
	ex0 := px*8 + int(mx)
	ey0 := py*8 + int(my)
	ix0 := ex0 >> 3
	iy0 := ey0 >> 3
	fx := ex0 - (ix0 << 3)
	fy := ey0 - (iy0 << 3)
	if fx == 0 && fy == 0 {
		return false
	}
	if ix0 < 0 || iy0 < 0 || ix0+cw+1 > w || iy0+ch+1 > h {
		return false
	}
	return chromaBlock(out, plane, w, ix0, iy0, cw, ch, fx, fy)
}

// chromaFastInto is the strided entry (S1-W direct-write): same gate
// as chromaFast, output rows land at dst[dy*dstStride:dy*dstStride+cw].
func chromaFastInto(plane []byte, w, h, px, py, cw, ch int, mx, my int16, dst []byte, dstStride int) bool {
	if qpelScalarForced {
		return false
	}
	if cw <= 0 || ch <= 0 || cw > 16 || ch > 16 {
		return false
	}
	ex0 := px*8 + int(mx)
	ey0 := py*8 + int(my)
	ix0 := ex0 >> 3
	iy0 := ey0 >> 3
	fx := ex0 - (ix0 << 3)
	fy := ey0 - (iy0 << 3)
	if fx == 0 && fy == 0 {
		return false
	}
	if ix0 < 0 || iy0 < 0 || ix0+cw+1 > w || iy0+ch+1 > h {
		return false
	}
	return chromaBlockInto(dst, dstStride, plane, w, ix0, iy0, cw, ch, fx, fy)
}

// chromaInteriorScalar is the interior留守 (C版留守): no per-tap
// clipping (caller guarantees interior), weights hoisted. Bit-exact
// with predictChromaBlock on interior blocks (v in 0..255, no clip
// needed: max (64*255+32)>>6 = 255). Do not optimize here; optimize
// in chroma_* arch paths.
func chromaInteriorScalar(out, plane []byte, stride, ix0, iy0, cw, ch, fx, fy int) {
	chromaInteriorScalarStride(out, cw, plane, stride, ix0, iy0, cw, ch, fx, fy)
}

// chromaInteriorScalarStride is the strided entry (S1-W direct-write):
// same weights, output row dy at out[dy*dstStride:dy*dstStride+cw].
func chromaInteriorScalarStride(out []byte, dstStride int, plane []byte, stride, ix0, iy0, cw, ch, fx, fy int) {
	aW := (8 - fx) * (8 - fy)
	bW := fx * (8 - fy)
	cW := (8 - fx) * fy
	dW := fx * fy
	for dy := 0; dy < ch; dy++ {
		s0 := (iy0+dy)*stride + ix0
		s1 := (iy0+dy+1)*stride + ix0
		o := dy * dstStride
		for dx := 0; dx < cw; dx++ {
			a := int(plane[s0+dx])
			b := int(plane[s0+dx+1])
			c := int(plane[s1+dx])
			d := int(plane[s1+dx+1])
			v := (aW*a + bW*b + cW*c + dW*d + 32) >> 6
			out[o+dx] = uint8(v)
		}
	}
}

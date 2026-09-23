//go:build amd64

package h264

import "unsafe"

// S1b-A1 amd64 chroma block path (SSE2 row kernel + interior scalar tail).
// The kernel (chroma_amd64.s) handles 8 pixels per call; the tail covers
// cw%8 with the interior scalar (same weights, no clipping).

//go:noescape
func chromaRow8(dst, src0, src1 unsafe.Pointer, a32, b32, c32, d32 int)

//go:noescape
func chromaRow4(dst, src0, src1 unsafe.Pointer, a32, b32, c32, d32 int)

//go:noescape
func chromaBlock8(dst unsafe.Pointer, dstStride uintptr, src unsafe.Pointer, srcStride uintptr, a32, b32, c32, d32 int, w, h int)

//go:noescape
func chromaBlock4(dst unsafe.Pointer, dstStride uintptr, src unsafe.Pointer, srcStride uintptr, a32, b32, c32, d32 int, h int)

// S1-D chroma deblock whole-edge kernels (one call filters an 8-line
// chroma edge with per-line tc; tc8[g]<0 skips group g). Bodies equal
// four filterChromaEdge calls bit for bit; only weak edges arrive here
// (no bS==4, checked by chromaDebEdge16). Intra/mixed edges stay on
// the per-segment scalar path.

//go:noescape
func chromaDebVWeak8(p unsafe.Pointer, stride, alpha, beta int, tc8 unsafe.Pointer)

//go:noescape
func chromaDebHWeak8(p unsafe.Pointer, stride, alpha, beta int, tc8 unsafe.Pointer)

// chromaDebEdge16 filters one whole 8-line chroma edge in a single asm
// call (luma E16 shape, chroma-sized). It reports false when the edge
// must run the per-segment scalar path: forced-scalar mode, any bS==4
// (intra formula differs), or a non-amd64 build (stubbed per arch).
// All-zero and closed-gate edges report true without touching the
// plane (every per-segment filter would skip: scalar gates each line
// on bS/alpha/beta first, and the kernels mask tc<=0 lanes).
// tc[g] must equal filterTC(qpc, fa, bS[g])+1 (positive for bS>=1).
// arm64/NEON twins are open work (same output via the per-segment
// path; perf differs); i386/32-bit arm stay scalar by design.
func chromaDebEdge16(p []uint8, stride, cx, cy int, vertical bool, bS, tc [4]int, alpha, beta int) bool {
	if deblockScalarForced {
		return false
	}
	any := false
	// S1b-Q pre-scan (bit-identical verdicts): the asm path touches
	// the plane only when at least one line both passes bS>=1 (no
	// bS==4 anywhere: intra formula differs) and holds tc>0 — a
	// tc<=0 line blends to identity inside the kernel, so an edge
	// whose every group has tc<=0 writes nothing. The old entry
	// reported true early only for all-zero bS; edges with bS>=1
	// but all-zero tc (QP-driven clip, ~15% of batched edges) still
	// paid the tc8 build + call + kernel setup for zero output.
	// Skip those here: the per-segment fallback below skips bS<1
	// segments and the batched caller marks tc<=0 groups -1, so an
	// all-tc<=0 edge filters nothing either way. Zero-alloc: one
	// linear scan, predictable branches.
	tcPos := false
	for _, v := range bS {
		if v == 4 {
			return false
		}
		if v > 0 {
			any = true
		}
	}
	if !any {
		return true
	}
	for _, v := range tc {
		if v > 0 {
			tcPos = true
			break
		}
	}
	if !tcPos {
		return true
	}
	if alpha == 0 || beta == 0 {
		return true
	}
	var tc8 [8]int16
	for g := 0; g < 4; g++ {
		tc8[2*g] = int16(tc[g])
		tc8[2*g+1] = int16(tc[g])
	}
	base := unsafe.Pointer(&p[cy*stride+cx])
	tcp := unsafe.Pointer(&tc8[0])
	if !vertical {
		chromaDebVWeak8(base, stride, alpha, beta, tcp)
	} else {
		chromaDebHWeak8(base, stride, alpha, beta, tcp)
	}
	return true
}

const (
	chromaMaxW = 16
	chromaMaxH = 16
)

// chromaBlock is the amd64 entry of the S1b-A1 dispatch (chromaFast in
// chroma_fast.go): one interior sub-pel chroma partition. Same gate as
// the shared dispatch; false means "run the scalar留守".
func chromaBlock(out, plane []byte, stride, ix0, iy0, cw, ch, fx, fy int) bool {
	return chromaBlockInto(out, cw, plane, stride, ix0, iy0, cw, ch, fx, fy)
}

// chromaBlockInto is the strided entry (S1-W direct-write): output row
// dy lands at out[dy*dstStride:dy*dstStride+cw]. Same weights, same
// bytes; the dense entry above is one call with dstStride == cw.
func chromaBlockInto(out []byte, dstStride int, plane []byte, stride, ix0, iy0, cw, ch, fx, fy int) bool {
	if cw <= 0 || ch <= 0 || cw > chromaMaxW || ch > chromaMaxH {
		return false
	}
	aW := (8 - fx) * (8 - fy)
	bW := fx * (8 - fy)
	cW := (8 - fx) * fy
	dW := fx * fy
	// Pack each weight twice into 32 bits so the kernel's MOVD+PSHUFD
	// broadcasts to 8 identical 16-bit lanes (same trick as
	// video/color/convert_amd64.s).
	a32 := aW | (aW << 16)
	b32 := bW | (bW << 16)
	c32 := cW | (cW << 16)
	d32 := dW | (dW << 16)
	// S1b-P fused block path (one call per block): the row kernels
	// above cost a call + a Go wrapper row + four weight broadcasts
	// per row; the wrapper matched the kernels' own flat. Exact
	// widths 8/16 (block8, groups in-asm) and 4 (block4) fuse;
	// cw==2 and odd widths keep the row path below (same weights,
	// same bytes).
	if cw == 8 || cw == 16 {
		chromaBlock8(
			unsafe.Pointer(&out[0]), uintptr(dstStride),
			unsafe.Pointer(&plane[iy0*stride+ix0]), uintptr(stride),
			a32, b32, c32, d32, cw, ch,
		)
		return true
	}
	if cw == 4 {
		chromaBlock4(
			unsafe.Pointer(&out[0]), uintptr(dstStride),
			unsafe.Pointer(&plane[iy0*stride+ix0]), uintptr(stride),
			a32, b32, c32, d32, ch,
		)
		return true
	}
	for dy := 0; dy < ch; dy++ {
		s0 := (iy0+dy)*stride + ix0
		s1 := (iy0+dy+1)*stride + ix0
		o := dy * dstStride
		done := 0
		bulk := cw &^ 7
		if bulk > 0 {
			chromaRow8(
				unsafe.Pointer(&out[o]),
				unsafe.Pointer(&plane[s0]),
				unsafe.Pointer(&plane[s1]),
				a32, b32, c32, d32,
			)
			// bulk is at most 8 here (cw <= 16); loop only if wider.
			for done = 8; done < bulk; done += 8 {
				chromaRow8(
					unsafe.Pointer(&out[o+done]),
					unsafe.Pointer(&plane[s0+done]),
					unsafe.Pointer(&plane[s1+done]),
					a32, b32, c32, d32,
				)
			}
			done = bulk
		}
		// 4-wide middle (cw=4 blocks and 4-remainders): exact 5-byte
		// loads, no overread past the interior guarantee.
		if cw-done >= 4 {
			chromaRow4(
				unsafe.Pointer(&out[o+done]),
				unsafe.Pointer(&plane[s0+done]),
				unsafe.Pointer(&plane[s1+done]),
				a32, b32, c32, d32,
			)
			done += 4
		}
		// Tail (cw%4, i.e. cw=2 blocks) with the interior scalar.
		for dx := done; dx < cw; dx++ {
			a := int(plane[s0+dx])
			b := int(plane[s0+dx+1])
			c := int(plane[s1+dx])
			d := int(plane[s1+dx+1])
			v := (aW*a + bW*b + cW*c + dW*d + 32) >> 6
			out[o+dx] = uint8(v)
		}
	}
	return true
}

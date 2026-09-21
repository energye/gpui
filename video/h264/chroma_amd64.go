//go:build amd64

package h264

import "unsafe"

// S1b-A1 amd64 chroma block path (SSE2 row kernel + interior scalar tail).
// The kernel (chroma_amd64.s) handles 8 pixels per call; the tail covers
// cw%8 with the interior scalar (same weights, no clipping).

//go:noescape
func chromaRow8(dst, src0, src1 unsafe.Pointer, a32, b32, c32, d32 int)

const (
	chromaMaxW = 16
	chromaMaxH = 16
)

// chromaBlock is the amd64 entry of the S1b-A1 dispatch (chromaFast in
// chroma_fast.go): one interior sub-pel chroma partition. Same gate as
// the shared dispatch; false means "run the scalar留守".
func chromaBlock(out, plane []byte, stride, ix0, iy0, cw, ch, fx, fy int) bool {
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
	for dy := 0; dy < ch; dy++ {
		s0 := (iy0+dy)*stride + ix0
		s1 := (iy0+dy+1)*stride + ix0
		o := dy * cw
		bulk := cw &^ 7
		if bulk > 0 {
			chromaRow8(
				unsafe.Pointer(&out[o]),
				unsafe.Pointer(&plane[s0]),
				unsafe.Pointer(&plane[s1]),
				a32, b32, c32, d32,
			)
			// bulk is at most 8 here (cw <= 16); loop only if wider.
			for done := 8; done < bulk; done += 8 {
				chromaRow8(
					unsafe.Pointer(&out[o+done]),
					unsafe.Pointer(&plane[s0+done]),
					unsafe.Pointer(&plane[s1+done]),
					a32, b32, c32, d32,
				)
			}
		}
		// Tail (cw%8) with the interior scalar, same weights.
		for dx := bulk; dx < cw; dx++ {
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

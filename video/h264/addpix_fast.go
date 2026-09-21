package h264

// S1b-A3 residual add (prediction + residual -> clipped picture).
//
// Peer (ffmpeg, read-only, ideas only, no code copied):
//   libavcodec/h264addpx_template.c:30-60 FUNCC(ff_h264_add_pixels4/8)
//   (dst[i] += src[i] lane add, memset src 0, our addRow4/8 kernels +
//   scalar tail) hooked via libavcodec/h264dsp.c:78-98 add_pixels/idct
//   function-pointer table (our addResid dispatch below); oracle is the
//   scalar loop itself (pred+residual+clipPixel, no new vectors here,
//   VR2 exact pins pixels).
// Ours: scalar留守 (clipPixel loop) + arch kernels (amd64 SSE2 row
// adds now, arm64 NEON follows the S1 staging). Interior blocks only
// (caller guarantees fit, no bounds checks beyond the dispatch gate).
// Output is bit-identical (addpix_s1_test.go pins luma/chroma). Set
// GPUI_SCALAR_CONVERT=1 to force scalar (same switch as qpel).

// addResidBlock adds res (int32 residual, 4 or 8 wide rows) onto pred
// bytes into the picture row at (bx, by) luma pixels, clipped 0..255.
// True means done (arch or scalar); false means caller falls back to
// the per-pixel SetY loop (forced-scalar off path never reports false:
// scalar always runs here).
// Edge guard: blocks touching the crop border stay on the SetY path
// (SetY clips per pixel); the fast path only runs fully inside.
func addResidBlock(pic []uint8, stride uint32, bx, by uint32, pred []uint8, predStride int, res []int32, w, h int) bool {
	if w != 4 && w != 8 {
		return false
	}
	if h <= 0 || len(res) < w*h || len(pred) < (h-1)*predStride+w {
		return false
	}
	if stride == 0 || uint64(by)*uint64(stride)+uint64(bx)+uint64(h-1)*uint64(stride)+uint64(w) > uint64(len(pic)) {
		return false
	}
	if qpelScalarForced {
		addResidScalar(pic, stride, bx, by, pred, predStride, res, w, h)
		return true
	}
	return addResidArch(pic, stride, bx, by, pred, predStride, res, w, h)
}

// addResidScalar is the scalar留守: pred+residual+clip per pixel.
func addResidScalar(pic []uint8, stride uint32, bx, by uint32, pred []uint8, predStride int, res []int32, w, h int) {
	for y := 0; y < h; y++ {
		base := (by+uint32(y))*stride + bx
		for x := 0; x < w; x++ {
			v := int32(pred[y*predStride+x]) + res[y*w+x]
			pic[base+uint32(x)] = clipPixel(v)
		}
	}
}

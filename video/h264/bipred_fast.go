package h264

// S1b-A2 bipred average (implicit weights).
//
// Peer (ffmpeg, read-only, ideas only, no code copied):
//   libavcodec/h264dsp.c:100-104 biweight_pixels_tab (w0/w1 denom 6,
//   (w0*a+w1*b+32)>>6 family, our bipred_amd64.s row kernel) vs our
//   bipred_amd64.go block skeleton; oracle is the scalar loop itself
//   (no new vectors here, VR2 exact pins pixels).
// Ours: scalar留守 below + arch kernels (amd64 SSE2 now, arm64 NEON
// follows the S1 staging); mcParts copy loops also row-copy (runtime
// memmove) instead of per-pixel stores. Output is bit-identical
// (bipred_s1_test.go pins weights). Set GPUI_SCALAR_CONVERT=1 to force
// scalar (same switch as qpel).

// bipredAvg mixes s1 into dst in place: dst[i] = (w*dst[i]+(64-w)*s1[i]+32)>>6.
// w is the implicit list-0 weight (0..64, denom 6); w==32 is the equal split.
func bipredAvg(dst, s1 []byte, w int32) {
	if len(dst) == 0 || len(dst) != len(s1) {
		return
	}
	if w == 32 {
		bipredAvgEqual(dst, s1)
		return
	}
	if !qpelScalarForced {
		if bipredFast(dst, s1, w) {
			return
		}
	}
	w1 := 64 - w
	for i := range dst {
		dst[i] = uint8((w*int32(dst[i]) + w1*int32(s1[i]) + 32) >> 6)
	}
}

// bipredAvgEqual is the w==32 fast lane (plain average, no weight mult).
func bipredAvgEqual(dst, s1 []byte) {
	if !qpelScalarForced {
		if bipredFast(dst, s1, 32) {
			return
		}
	}
	for i := range dst {
		dst[i] = uint8((int32(dst[i]) + int32(s1[i]) + 1) >> 1)
	}
}

// bipredFast runs one average through the arch kernel; false falls back.
func bipredFast(dst, s1 []byte, w int32) bool {
	n := len(dst)
	if n == 0 || n != len(s1) {
		return false
	}
	return bipredBlock(dst, s1, n, w)
}

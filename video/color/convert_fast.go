package color

import "os"

// S1 convert fast path, step T2 (SSE4.1 row kernel, scalar留守).
//
// Peer (ffmpeg, read-only, ideas only, no code copied):
//   libswscale/yuv2rgb.c:47 ff_yuv2rgb_coeffs table (our tableFor),
//   libswscale/yuv2rgb.c:137 YUV2RGBFUNC macro family (per-pair factored
//   taps, our convertBandScalar) and :520-522 YUV420FUNC yuv2rgb_c_32
//   (packed 32-bit RGBA output order),
//   libswscale/x86/yuv_2_rgb.asm (SIMD yuv2rgb instance, our
//   convert_amd64.s row kernel processes 4 pixels per iteration),
//   libswscale/output.c:1116 yuv2rgba64_X_c_template (row-band workers,
//   our convert420Into band split),
//   libswscale/utils.c:2447 ff_sws_thread_exec (slice threading idea,
//   our convWorker pool),
//   libswscale/swscale.c:1626 sws_scale (frame entry, our ConvertInto).
// Ours: video/color/color.go convertBandScalar (留守) + convert420Into
// (band split, untouched); this file is the dispatch (SIMD first,
// scalar fallback). amd64 assembly lives in convert_amd64.s with its Go
// wrapper in convert_amd64.go; other archs use convert_fallback.go.
// Tried and dropped (2026-09-15, same machine): lookup-table + 8-pixel
// unroll measured ~1.0x vs scalar on Go 1.25/Skylake (multiplies are
// pipelined, LUT loads add dependent latency), so tables are out and the
// vector kernel is the fast path. Output is bit-identical to the scalar
//留守 (vectors pin it). Requires SSE4.1 on amd64 (Penryn 2008+, every
// 1080p target has it); set GPUI_SCALAR_CONVERT=1 to force scalar.

// scalarForced skips the vector kernel (ancient-CPU escape hatch), read
// once from the environment so the hot path pays nothing.
var scalarForced = func() bool {
	switch os.Getenv("GPUI_SCALAR_CONVERT") {
	case "1", "true", "TRUE":
		return true
	}
	return false
}()

// convertBand is the S1 dispatch: vector kernel first, scalar留守 fallback.
func convertBand(dst, y, cb, cr []byte, w int, t coeffs, ys, ye int) {
	if !scalarForced && convertBandSIMD(dst, y, cb, cr, w, t, ys, ye) {
		return
	}
	convertBandScalar(dst, y, cb, cr, w, t, ys, ye)
}

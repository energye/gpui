// Package color implements VR3: YUV storage plus YUV to RGBA conversion.
//
// Scope is narrow on purpose: 8-bit 4:2:0 input (the only sampling the
// H.264 decoder emits this stage) to an owned RGBA frame, with studio
// (limited) versus full range and BT.601 versus BT.709 matrices. Anything
// else reports a readable error naming the sampling or matrix, so VR9 can
// plug new samplings through the registry without touching this flow.
//
// Design follows the对照思路 of ffmpeg's libswscale (yuv2rgb/input/output
// plus the manual): range expansion first, then the Kr/Kb matrix, fixed
// point with round-to-nearest. All logic is reimplemented in pure Go with
// only the standard library.
//
// S1-T2 (2026-09-15, same machine i5-6200U/Skylake): amd64 SSE4.1 row
// kernel (convert_amd64.s, 4 pixels/iteration) behind the convertBand
// dispatch, scalar留守 fallback (GPUI_SCALAR_CONVERT=1 forces it, other
// archs use it). Math-only 854x480 scalar ~4.1ms vs dispatch ~1.0ms
// (~3.9x, zero alloc); end-to-end 1080p ConvertInto ~8.2ms vs ~2.9ms
// (~2.8x, threading-capped); VR7-T avg 0.41 vs 0.15ms (budget 0.63),
// VR7-D convert p95 was 12.85 FAIL now PASS (budget 8.8). Output
// bit-identical to scalar (vectors pin it). Next:插值/去块 kernels + arm64
// NEON; H264 math segment untouched (see plan §12 S1).
package color

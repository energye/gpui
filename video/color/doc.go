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
package color

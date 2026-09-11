// Package h264 implements VR1: H.264 parameter sets and frame splitting.
//
// Scope is narrow on purpose: parse SPS/PPS (档位/等级/尺寸), split
// NAL units out of both MP4 packings (AVCC length-prefixed and Annex B
// start codes), collect in-band and out-of-band parameter sets, and cut
// NALUs into frames (access units). It does not reconstruct pictures;
// that belongs to VR2.
//
// Design follows the对照思路 of ffmpeg's H.264 helpers
// (libavcodec/cbs_h264.h for SPS/PPS field order, h2645_parse for NAL
// boundaries, golomb.h/get_bits.h for Exp-Golomb reading). All logic is
// reimplemented in pure Go with only the standard library.
package h264

// Package h265 implements V2-1: H.265/HEVC headers only (no pixels).
//
// Scope is narrow on purpose: parse the hvcC box (HEVCDecoderConfiguration
// Record per ISO 14496-15), report profile/level/size and cut MP4 sample
// payloads into NAL units. It does not reconstruct pictures; pixel decode
// is V2-2.
//
// Design follows the对照思路 of ffmpeg's HEVC helpers (read-only, no code
// copied): libavcodec/hevc/parse.c:79 ff_hevc_decode_extradata (hvcC box
// shape: skip 21, length-size byte, array count, per-array type/count and
// 2-byte length-prefixed units) + libavcodec/hevc/hevcdec.c:4190
// hevc_decode_init + :4271 ff_hevc_decoder (decoder entry shape,
// RECEIVE_FRAME_CB pull model) + libavformat/mov.c:3419 (hvcC read with
// avcC). All logic is reimplemented in pure Go with only the standard
// library.
package h265

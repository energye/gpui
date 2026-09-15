// Package mp4 implements the MP4 demuxer (VR0 shell + B1 fragments).
//
// Scope: read the file shell (boxes), find the video track, and report
// width, height, frame rate, duration, and the keyframe index. It does
// not decode pictures; that belongs to later stages (VR1/VR2).
//
// Plain clips use moov sample tables; fragmented clips (B1, phone-style
// 边录边存) assemble samples from moof segments (see frag.go). Both land in
// the same Track tables, so players and seeks never branch on provenance.
//
// Design follows the对照思路 of ffmpeg's mov demuxer
// (libavformat/mov.c, isom.h): nested boxes, sample tables
// (stts/stsc/stsz/stco), sync samples (stss), composition offsets
// (ctts), edit lists (elst), fragment tables
// (mvex/trex/moov, moof/traf/tfhd/tfdt/trun, sidx). All logic is
// reimplemented in pure Go with only the standard library.
package mp4

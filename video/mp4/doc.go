// Package mp4 implements a minimal MP4 demuxer for VR0.
//
// Scope is intentionally narrow: read the file shell (boxes), find the
// video track, and report width, height, frame rate, duration, and the
// keyframe index. It does not decode pictures; that belongs to later
// stages (VR1/VR2).
//
// Design follows the对照思路 of ffmpeg's mov demuxer
// (libavformat/mov.c, isom.h): nested boxes, sample tables
// (stts/stsc/stsz/stco), sync samples (stss), composition offsets
// (ctts), edit lists (elst). All logic is reimplemented in pure Go
// with only the standard library.
package mp4

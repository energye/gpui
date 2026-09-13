// Package video is the root API for VR4 playback, VR5 seeking, VR6
// faults and VR7 limits: open a file, pull frames by timestamp, pause,
// resume, jump to a time and close.
//
// Scope is narrow on purpose: the player wires the existing pieces
// (mp4 shell, h264 pictures, color conversion, clock queue) without
// reimplementing them. Decoding runs on a background thread; the caller
// only polls due frames. SeekTo lands on the last keyframe at or before
// the target and re-decodes forward (verified pixel-exact, never a head
// replay). Bad inputs fail or isolate readably: Classify names layer +
// tool, F20 frames are skipped with the tail kept playing. VR7 bounds
// the steady loop: single-frame-size pools (YUV/RGBA/work, video/pool.go)
// with hit/leak/cap stats, bounded queue, explicit memory cap, and a
// timer-free Poll fast path so steady polls cost no heap. Errors name
// the layer and the file, never a bare code. Pure Go, standard library
// only.
package video

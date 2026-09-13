// Package video is the root API for VR4 playback and VR5 seeking: open a
// file, pull frames by timestamp, pause, resume, jump to a time and close.
//
// Scope is narrow on purpose: the player wires the existing pieces
// (mp4 shell, h264 pictures, color conversion, clock queue) without
// reimplementing them. Decoding runs on a background thread; the caller
// only polls due frames. SeekTo lands on the last keyframe at or before
// the target and re-decodes forward (verified pixel-exact, never a head
// replay). Errors name the layer and the file, never a bare code. Pure
// Go, standard library only.
package video

// Package video is the root API for VR4 playback: open a file, pull
// frames by timestamp, pause, resume and close.
//
// Scope is narrow on purpose: the player wires the existing pieces
// (mp4 shell, h264 pictures, color conversion, clock queue) without
// reimplementing them. Decoding runs on a background thread; the caller
// only polls due frames. Errors name the layer and the file, never a
// bare code. Pure Go, standard library only.
package video

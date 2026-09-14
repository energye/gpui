// Package video is the root API for VR4 playback, VR5 seeking, VR6
// faults and VR7 limits: open a file, pull frames by timestamp, pause,
// resume, jump to a time and close.
//
// Seeks are requests, not chores (ffplay stream_seek model): SeekTo only
// reparks the read needle on the landing keyframe and returns the landing
// stamp at once; the background drops decoded frames until the landing
// shows, and a second seek supersedes the first (drag-friendly). Seeking
// reports a seek still travelling; PositionMs reports the current stamp.
// Companion controls: SeekFast (keyframe-only, for dragging), SeekBy
// (relative jump), Next/PrevKeyframe, StepFrame (paused single step) and
// SetRate/Rate (0 < rate <= 8, speed-scaled clock; video-only, no audio
// to keep in sync). True reverse decode does not exist.
//
// Scope is narrow on purpose: the player wires the existing pieces
// (mp4 shell, h264 pictures, color conversion, clock queue) without
// reimplementing them. Decoding runs on a background thread; the caller
// only polls due frames. SeekTo lands on the last keyframe at or before
// the target and the background re-decodes forward (verified pixel-exact
// on the buffered path, never a head replay). Bad inputs fail or isolate readably: Classify names layer +
// tool, F20 frames are skipped with the tail kept playing. Containers and
// codecs resolve through the registry (video/registry.go: shell probe +
// codec table + capability query); the player never names a format, so new
// shells/codecs arrive as new packages + Register calls. Streaming is
// ffmpeg-shaped: Open parses headers + first frame only (any length
// opens fast, any Source — file, memory, HTTP Range), the background
// decodes ahead through a bounded queue, so memory stays flat; small
// clips buffer fully to keep every gate deterministic. VR7 bounds
// the steady loop: single-frame-size pools (YUV/RGBA/work, video/pool.go)
// with hit/leak/cap stats, bounded queue, explicit memory cap, and a
// timer-free Poll fast path so steady polls cost no heap. Errors name
// the layer and the file, never a bare code. Pure Go, standard library
// only.
package video

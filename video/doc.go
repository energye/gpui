// Package video is the root API for VR4 playback, VR5 seeking, VR6
// faults and VR7 limits: open a file, pull frames by timestamp, pause,
// resume, jump to a time and close.
//
// Seeks are requests, not chores (ffplay stream_seek model): SeekTo only
// posts the target and returns the landing stamp at once; the background
// drops decoded frames until the landing shows, and a second seek
// supersedes the first (drag-friendly). Seeking reports a seek still
// travelling; PositionMs reports the current stamp.
// Companion controls: SeekFast (keyframe-only, for dragging), SeekBy
// (relative jump), Next/PrevKeyframe, StepFrame (paused single step) and
// SetRate/Rate (0 < rate <= 8, speed-scaled clock; sound follows the
// same clock). True reverse decode does not exist.
//
// Scope is narrow on purpose: the player wires the pieces without
// reimplementing them. Decode backend is ffmpeg only
// (video/ffmpeg: demux + decode + sws to RGBA); the Player only does
// queue/clock/seek orchestration (player_types/open/playback/seek) on
// top. Decoding runs on a background thread; the caller only polls due
// frames. Every clip streams: the background decodes ahead through a
// bounded queue, so memory stays flat. Capability answers and shell
// probing live in the registry (video/registry_probe.go); bad inputs
// fail readably through Classify (video/fault_classify.go); streaming
// buffers come from fixed-size pools (video/mem_pool.go) with hit/leak/
// cap stats. Data sources (file/memory/HTTP Range) are abstracted in
// video/io_source.go; audio sync math and the Player sound wiring
// live in video/sync_audio.go. Errors name the layer and the file,
// never a bare code. No CGO, standard library plus purego only.
package video

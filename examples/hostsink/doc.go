// Package hostsink carries decoded PCM to the host speaker for
// interactive windows (e.g. video_player).
//
// Say it plain: the engine (video.Player.PollAudio) hands us decoded PCM
// frames (float32 interleaved + stamp + waterline) and never touches the
// speaker; this package carries those frames to the OS sound service and
// reports back honestly. video/ stays device-free, so every device
// handle below lives window-side only.
//
// The A4 gate window (examples/video_a4_sink) keeps its own copy with
// gate extras (unplug-drill hooks, WAV chain proof, null sink); this
// package is the shared speaker path for windows that just play.
// Behavior matches A4's: paplay-pulse preferred, aplay-alsa fallback,
// device-paced blocking writes, pause + auto-resume on loss, pump thread
// that lingers past end-of-stream so replays keep sounding.
//
// ffmpeg peers (read-only, fftools/ffplay.c; Serial == Player generation):
//
//	audio_open (:2578-2640, SDL_OpenAudioDevice + fallback lists) ->
//	  ProbeHostAudio + NewHostSink (obtained spec reported, never faked)
//	sdl_audio_callback (:2533-2576, pull frame, pad silence on error,
//	  re-anchor clock) -> Pump.Once / PumpLoop (pull PollAudio,
//	  FloatToS16, bad packets never surface: the engine conceals them)
//	SDL_PauseAudioDevice (:2814, start) -> the first Write starts the
//	  stream; the pump thread and both clocks keep living across loss
//
// Platform table (one row per OS, no blank cells per repo discipline):
//
//	Linux/paplay-pulse | system Pulse client via raw stdin | default, tried first
//	Linux/aplay-alsa   | system ALSA writer via raw stdin  | fallback when paplay missing
//	darwin/*           | honest unsupported, readable err  | no speaker claim, video plays on
//	windows/*          | honest unsupported, readable err  | no speaker claim, video plays on
package hostsink

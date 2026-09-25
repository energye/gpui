// Audio probe stub, video-only backend (ffmpeg).
//
// Say it plain: sound decode is not wired yet (see t-audio-ffmpeg), so
// every clip reports no audio. This file keeps the ProbeAudio shape so
// windows and tools keep compiling against silence instead of branching
// on a removed function.
package video

// AudioInfo describes one audio track. Fields stay for the callers;
// values are always zero on the video-only backend.
type AudioInfo struct {
	Path       string
	Codec      string
	Profile    string
	SampleRate int
	Channels   int
	Samples    int
	DurationMs int64
	ASC        []byte
}

// ProbeAudio reports the audio track without decoding. Always ErrNoAudio
// on the video-only backend (honest unavailable, never faked).
func ProbeAudio(path string) (AudioInfo, error) {
	return AudioInfo{Path: path}, ErrNoAudio
}

// ProbeAudioSource is the Source twin of ProbeAudio. Always ErrNoAudio.
func ProbeAudioSource(src Source) (AudioInfo, error) {
	name := ""
	if src != nil {
		name = src.Name()
	}
	return AudioInfo{Path: name}, ErrNoAudio
}

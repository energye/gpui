package ffmpeg

import (
	"errors"
	"testing"
)

// TestAudioOpenInfo pins the oceans sound identity through the audio
// demuxer: AAC stereo 48kHz, the same numbers ffprobe reports in
// video/testdata/a2_ffmpeg.json.
func TestAudioOpenInfo(t *testing.T) {
	if !Available() {
		t.Skipf("lib missing: %s", LibPath())
	}
	a, err := OpenAudio("../testdata/vr_oceans.mp4")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	info := a.Info()
	if info.SampleRate != 48000 {
		t.Fatalf("sample rate = %d, want 48000", info.SampleRate)
	}
	if info.Channels != 2 {
		t.Fatalf("channels = %d, want 2", info.Channels)
	}
	if info.CodecID != 86018 {
		t.Fatalf("codec = %d, want 86018 (AAC)", info.CodecID)
	}
}

// TestAudioFirstFrames decodes 5 sound chunks: samples present,
// stamps rise, output shape is 48kHz stereo float, and the clip is
// audibly non-silent.
func TestAudioFirstFrames(t *testing.T) {
	if !Available() {
		t.Skipf("lib missing: %s", LibPath())
	}
	a, err := OpenAudio("../testdata/vr_oceans.mp4")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	// The oceans head opens with ~0.4s of digital silence (verified
	// against system ffmpeg: first energy lands near frame 20, full
	// level by frame 30). Prime past it so the non-silence assert
	// means something.
	for i := 0; i < 30; i++ {
		if _, err := a.Next(); err != nil {
			t.Fatalf("prime %d: %v", i, err)
		}
	}
	var last int64 = -1
	var energy float64
	for i := 0; i < 5; i++ {
		fr, err := a.Next()
		if err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
		if fr.SampleRate != OutRate || fr.Channels != OutChannels {
			t.Fatalf("frame %d shape = %dHz/%dch, want 48000/2", i, fr.SampleRate, fr.Channels)
		}
		if fr.Samples <= 0 || len(fr.Data) != fr.Samples*OutChannels {
			t.Fatalf("frame %d samples = %d data %d", i, fr.Samples, len(fr.Data))
		}
		if fr.PTSMs <= last {
			t.Fatalf("frame %d pts %d not after %d", i, fr.PTSMs, last)
		}
		last = fr.PTSMs
		for _, s := range fr.Data {
			energy += float64(s) * float64(s)
		}
		t.Logf("audio %d pts=%d samples=%d", i, fr.PTSMs, fr.Samples)
	}
	if energy <= 0 {
		t.Fatal("5 voiced frames decoded silence (energy 0)")
	}
}

// TestAudioSeek lands at/after the target and keeps decoding.
func TestAudioSeek(t *testing.T) {
	if !Available() {
		t.Skipf("lib missing: %s", LibPath())
	}
	a, err := OpenAudio("../testdata/vr_oceans.mp4")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if _, err := a.SeekTo(5000); err != nil {
		t.Fatal(err)
	}
	fr, err := a.Next()
	if err != nil {
		t.Fatal(err)
	}
	if fr.PTSMs < 4000 {
		t.Fatalf("seek landing pts = %d, want near 5000", fr.PTSMs)
	}
}

// TestAudioSilentClip keeps silence honest: no track, no frames.
func TestAudioSilentClip(t *testing.T) {
	if !Available() {
		t.Skipf("lib missing: %s", LibPath())
	}
	if _, err := OpenAudio("../testdata/vr_silent.mp4"); !errors.Is(err, errNoAudioTrack) {
		t.Fatalf("silent open err = %v, want no-audio-track", err)
	}
	if HasAudioTrack("../testdata/vr_silent.mp4") {
		t.Fatal("silent clip reports a track")
	}
	if !HasAudioTrack("../testdata/vr_oceans.mp4") {
		t.Fatal("oceans reports no track")
	}
}

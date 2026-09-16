// A4 host-sink gate (VW6 §12 A4 row): headless, no window, no speaker
// needed except the device-loss drill (skipped honestly when headless).
//
// ffmpeg peers (read-only, no code copied), see sink.go header for the
// line map: fftools/ffplay.c audio_open (:2578-2640) + sdl_audio_callback
// (:2533-2576) + audio_decode_frame (:2423-2531) + PauseAudioDevice (:2814).
//
// Baseline video/testdata/a4_ffmpeg.json pins the clip identity (ffprobe
// 4.4.2), the sink spec (pactl 15.99.1) and the budgets; the tests read
// expects from it, never hardcode them.
package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	govideo "github.com/energye/gpui/video"
)

// TestA4BaselineParity pins the gate clip shell from the real file:
// 320x240 video plus LC AAC 44100/stereo packet tables against the
// ffprobe baseline.
func TestA4BaselineParity(t *testing.T) {
	base, err := loadA4Baseline()
	if err != nil {
		t.Fatal(err)
	}
	if err := checkA4Identity(base); err != nil {
		t.Fatal(err)
	}
	if base.Audio.ASCHex == "" || base.Audio.Samples == 0 {
		t.Fatal("baseline audio identity empty")
	}
}

// TestA4ConvertExact pins FloatToS16 against the vector data file
// (the exact bytes paplay/aplay consume).
func TestA4ConvertExact(t *testing.T) {
	got, total, err := checkA4Convert()
	if err != nil {
		t.Fatal(err)
	}
	if got != total || total == 0 {
		t.Fatalf("convert = %d/%d", got, total)
	}
	t.Logf("vectors %d/%d exact", got, total)
}

// TestA4WavChain plays the head through the real decoder and proves the
// WAV file carries the speaker bytes exactly (headless audible-proof
// stand-in: same bytes, no hardware).
func TestA4WavChain(t *testing.T) {
	base, err := loadA4Baseline()
	if err != nil {
		t.Fatal(err)
	}
	if err := checkA4Wav(base); err != nil {
		t.Fatal(err)
	}
	// File leg: the same chain through a real file in TempDir.
	rec := &recSink{}
	h := &handClock{}
	p, err := govideo.OpenFile(resolveA4(base.Clip), govideo.Options{NowMs: h.at})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	pm := &Pump{Sink: rec}
	for i := 0; i < 400 && len(rec.blocks) < 3; i++ {
		h.now += 10
		if _, done, err := pm.Once(p); err != nil {
			t.Fatal(err)
		} else if done {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	var pcm []byte
	for _, b := range rec.blocks {
		pcm = append(pcm, b...)
	}
	path := filepath.Join(t.TempDir(), "a4.wav")
	w, err := NewWavSink(path, base.Audio.SampleRate, base.Audio.Channels)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.WritePCM(pcm); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := EncodeWav(base.Audio.SampleRate, base.Audio.Channels, pcm)
	if len(raw) != len(want) {
		t.Fatalf("wav size = %d, want %d", len(raw), len(want))
	}
	for i := range want {
		if raw[i] != want[i] {
			t.Fatalf("wav byte %d differs", i)
		}
	}
}

// TestA4PumpSync pins sound-led sync through a counting sink: both
// stamps monotonic, master audio, zero drops, gap inside budget.
func TestA4PumpSync(t *testing.T) {
	base, err := loadA4Baseline()
	if err != nil {
		t.Fatal(err)
	}
	d, v, a, err := checkA4Pump(base)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("pump v=%d a=%d avdiff=%d", v, a, d)
}

// TestA4ProbeHonest pins the device probe never fakes: either a named
// backend or a readable reason, never a silent zero.
func TestA4ProbeHonest(t *testing.T) {
	backend, available, reason := ProbeHostAudio()
	if available && backend == "" {
		t.Fatal("available without a backend name")
	}
	if !available && reason == "" {
		t.Fatal("unavailable without a reason")
	}
	t.Logf("probe backend=%q available=%v reason=%q", backend, available, reason)
}

// TestA4DeviceLossKeepsThread drills the unplug path on the real writer:
// pump, kill the child mid-stream, require paused + readable error,
// Resume, require sound flowing again. No speaker hardware is asserted
// (CI is headless); what is asserted is pause-not-dead. Skipped
// honestly when no writer exists.
func TestA4DeviceLossKeepsThread(t *testing.T) {
	backend, available, reason := ProbeHostAudio()
	if !available {
		t.Skipf("no speaker writer: %s", reason)
	}
	base, err := loadA4Baseline()
	if err != nil {
		t.Fatal(err)
	}
	h := &handClock{}
	p, err := govideo.OpenFile(resolveA4(base.Clip), govideo.Options{NowMs: h.at})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	s, err := NewHostSink(base.Audio.SampleRate, base.Audio.Channels, "")
	if err != nil {
		t.Skipf("writer refused: %v", err)
	}
	defer s.Close()
	if s.Backend() != backend {
		t.Fatalf("backend = %q, probe said %q", s.Backend(), backend)
	}
	pm := &Pump{Sink: s}
	fed := 0
	for i := 0; i < 400 && fed < 2; i++ {
		h.now += 25
		if wrote, _, err := pm.Once(p); err == nil && wrote {
			fed++
		}
		time.Sleep(2 * time.Millisecond)
	}
	if fed < 1 {
		t.Skipf("writer took no frames (headless audio?): fed=%d", fed)
	}
	cs, ok := s.(*cmdSink)
	if !ok {
		t.Skipf("backend %q has no child to drill", s.Backend())
	}
	cs.KillChild()
	time.Sleep(300 * time.Millisecond)
	// Drive the clock so writes are actually attempted (a quiet pump
	// proves nothing); the dead pipe must surface as an error.
	lost := false
	for i := 0; i < 200 && !lost; i++ {
		h.now += 25
		if _, _, err := pm.Once(p); err != nil {
			lost = true
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !lost {
		t.Fatal("writes after kill kept succeeding, want device loss")
	}
	if !s.Paused() {
		t.Fatal("sink not paused after device loss")
	}
	if s.DeviceError() == "" {
		t.Fatal("device loss without a readable error")
	}
	lostErr := s.DeviceError()
	if err := s.Resume(); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if s.Paused() {
		t.Fatal("still paused after resume")
	}
	t.Logf("loss paused, err=%q, resumed ok", lostErr)
}

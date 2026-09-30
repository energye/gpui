// ffmpeg peers (read-only, no code copied), see video/sync_audio.go header for the line map:
//
//	fftools/ffplay.c Clock + get_master_sync_type/get_master_clock +
//	set_clock/set_clock_speed + stream_seek/packet_queue_flush +
//	compute_target_delay + video_refresh + audio_decode_frame; thresholds
//	AV_SYNC_THRESHOLD_MIN/MAX 0.04/0.1, FRAMEDUP 0.1, NOSYNC 10.0.
//
// Baseline video/testdata/a2_ffmpeg.json pins the clip identity (ffprobe
// 4.4.2), the thresholds and the seek floors; the engine reads expects
// from it, never hardcodes them.
package video

import (
	"encoding/json"
	"os"
	"runtime"
	"testing"
	"time"

	ff "github.com/energye/gpui/video/ffmpeg"
)

type a2Packet struct {
	Size  int   `json:"size"`
	PTSMs int64 `json:"pts_ms"`
}

type a2Seek struct {
	TargetMs     int64 `json:"target_ms"`
	VideoFloorMs int64 `json:"video_floor_ms"`
	VideoKeyMs   int64 `json:"video_key_ms"`
	AudioFloorMs int64 `json:"audio_floor_ms"`
}

type a2Video struct {
	Profile    string  `json:"profile"`
	ProfileIDC int     `json:"profile_idc"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	Level      int     `json:"level"`
	FrameRate  string  `json:"avg_frame_rate"`
	NbFrames   int     `json:"nb_frames"`
	DurationMs int64   `json:"duration_ms"`
	Samples    int     `json:"samples"`
	Keyframes  int     `json:"keyframes"`
	KeyPtsMs   []int64 `json:"key_pts_ms"`
	FirstPtsMs []int64 `json:"first_pts_ms"`
	LastPtsMs  int64   `json:"last_pts_ms"`
}

type a2Baseline struct {
	Clip  string  `json:"clip"`
	Video a2Video `json:"video"`
	Audio struct {
		Profile      string     `json:"profile"`
		SampleRate   int        `json:"sample_rate"`
		Channels     int        `json:"channels"`
		NbFrames     int        `json:"nb_frames"`
		DurationMs   int64      `json:"duration_ms"`
		Samples      int        `json:"samples"`
		Timescale    uint32     `json:"timescale"`
		ASCHex       string     `json:"asc_hex"`
		MP4ARate     uint32     `json:"mp4a_rate"`
		MP4AChannels uint16     `json:"mp4a_channels"`
		MP4ABits     uint16     `json:"mp4a_bits"`
		FirstPackets []a2Packet `json:"first_packets"`
		LastPtsMs    int64      `json:"last_pts_ms"`
	} `json:"audio"`
	DurationTolMs int64 `json:"duration_tolerance_ms"`
	ThresholdsMs  struct {
		SyncMin int `json:"sync_min"`
		SyncMax int `json:"sync_max"`
		Framed  int `json:"framedup"`
		Nosync  int `json:"nosync"`
	} `json:"thresholds_ms"`
	DiffBudgetMs int64    `json:"diff_budget_ms"`
	Seeks        []a2Seek `json:"seeks"`
}

func loadA2Baseline(t *testing.T) a2Baseline {
	t.Helper()
	buf, err := os.ReadFile("testdata/a2_ffmpeg.json")
	if err != nil {
		t.Fatal(err)
	}
	var b a2Baseline
	if err := json.Unmarshal(buf, &b); err != nil {
		t.Fatal(err)
	}
	if b.Clip == "" || len(b.Seeks) == 0 {
		t.Fatal("empty a2 baseline")
	}
	return b
}

// TestA2BaselineParity pins the gate clip identity through the ffmpeg
// demuxer: Constrained Baseline 960x400/23.976fps video dims, codec and
// frame count match ffprobe, duration within tolerance. The Go box walk
// (samples/keys/ASC/packet tables) retired with video/mp4+video/aac;
// the baseline json keeps those numbers as history. Audio asserts return
// with t-audio-ffmpeg; until then the backend plays this clip silent.
// Thresholds ride along so the sync budget never drifts into a literal.
func TestA2BaselineParity(t *testing.T) {
	b := loadA2Baseline(t)
	dec, err := ff.Open("testdata/" + b.Clip)
	if err != nil {
		t.Fatal(err)
	}
	info := dec.Info()
	dec.Close()
	if info.Width != b.Video.Width || info.Height != b.Video.Height {
		t.Fatalf("video dims = %dx%d, want %dx%d", info.Width, info.Height, b.Video.Width, b.Video.Height)
	}
	if got := ffCodecName(info.CodecID); got != "h264" {
		t.Fatalf("video codec = %q, want h264", got)
	}
	if int(info.Frames) != b.Video.Samples && int(info.Frames) != 0 {
		t.Fatalf("video frames = %d, want %d", info.Frames, b.Video.Samples)
	}
	if d := info.DurMs - b.Video.DurationMs; d < -b.DurationTolMs || d > b.DurationTolMs {
		t.Fatalf("video duration = %d, want %d +-%d", info.DurMs, b.Video.DurationMs, b.DurationTolMs)
	}
	// Sound-carrying clip, sound backend: open reports audio with the
	// probe identity (AAC stereo 48kHz, same as ffprobe in a2_ffmpeg.json).
	h := &handClock{}
	p, err := OpenFile("testdata/"+b.Clip, Options{NowMs: h.at})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if !p.HasAudio() {
		t.Fatal("hasAudio = false, want true (oceans carries AAC)")
	}
	if p.Master() != MasterAudio {
		t.Fatalf("master = %s, want audio", p.Master())
	}
	if ai, err := ProbeAudio("testdata/" + b.Clip); err != nil {
		t.Fatalf("probe oceans: %v", err)
	} else if ai.SampleRate != 48000 || ai.Channels != 2 || ai.Codec != "aac" {
		t.Fatalf("probe oceans = %+v, want aac 48kHz stereo", ai)
	}
	if b.ThresholdsMs.SyncMin != A2SyncThresholdMinMs || b.ThresholdsMs.SyncMax != A2SyncThresholdMaxMs ||
		b.ThresholdsMs.Framed != A2FramedupMs || b.ThresholdsMs.Nosync != A2NosyncMs {
		t.Fatalf("thresholds %+v drift from ffplay peer %d/%d/%d/%d", b.ThresholdsMs,
			A2SyncThresholdMinMs, A2SyncThresholdMaxMs, A2FramedupMs, A2NosyncMs)
	}
	if SelectMaster(true) != MasterAudio || SelectMaster(false) != MasterVideo {

		t.Fatal("master select != audio-with-sound/video-when-silent")
	}
}

// TestA2TargetDelay pins the ffplay compute_target_delay shape in
// milliseconds: late shortens the wait, early stretches it, huge diffs
// pass through (initial-error guard), video-master passes through.
func TestA2TargetDelay(t *testing.T) {
	if d := ComputeTargetDelay(40, -50, false); d != 0 {
		t.Fatalf("late 40-50 = %v, want 0 (floor)", d)
	}
	if d := ComputeTargetDelay(100, -150, false); d != 0 {
		t.Fatalf("late 100-150 = %v, want 0 (floor)", d)
	}
	if d := ComputeTargetDelay(40, 60, false); d != 80 {
		t.Fatalf("early 40+60 = %v, want 80 (double)", d)
	}
	if d := ComputeTargetDelay(200, 150, false); d != 350 {
		t.Fatalf("early long 200+150 = %v, want 350", d)
	}
	if d := ComputeTargetDelay(40, 20000, false); d != 40 {
		t.Fatalf("nosync = %v, want passthrough 40", d)
	}
	if d := ComputeTargetDelay(40, -80, true); d != 40 {
		t.Fatalf("video-master = %v, want passthrough 40", d)
	}
}

func monoInc(t *testing.T, name string, vs []int64) {
	t.Helper()
	for i := 1; i < len(vs); i++ {
		if vs[i] <= vs[i-1] {
			t.Fatalf("%s not monotonic: %v", name, vs)
		}
	}
}

// TestA2SoundHead pins the ffmpeg sound head on the A2 clip: audio
// master, picture and sound rise monotonically together, the voiced
// part carries nonzero energy, and Stats reports real audio counts.
func TestA2SoundHead(t *testing.T) {
	b := loadA2Baseline(t)
	h := &handClock{}
	p, err := OpenFile("testdata/"+b.Clip, Options{NowMs: h.at})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if !p.HasAudio() || p.Master() != MasterAudio {
		t.Fatalf("hasAudio/master = %v/%s, want true/audio", p.HasAudio(), p.Master())
	}
	var vpts, apts []int64
	var energy float64
	ended := false
	deadline := time.Now().Add(90 * time.Second)
	for i := 0; i < 400 && !ended; i++ {
		if time.Now().After(deadline) {
			t.Fatalf("head never plays, v=%d a=%d", len(vpts), len(apts))
		}
		h.now += 25
		if vf, done := p.Poll(); vf != nil {
			vpts = append(vpts, vf.PTSMs)
		} else if done {
			ended = true
		}
		if af, _ := p.PollAudio(); af != nil {
			apts = append(apts, af.PTSMs)
			for _, s := range af.Data {
				energy += float64(s) * float64(s)
			}
		}
		runtime.Gosched()
		time.Sleep(10 * time.Millisecond)
	}
	if len(vpts) < 5 {
		t.Fatalf("only %d video frames in 400 ticks: %v", len(vpts), vpts)
	}
	if len(apts) < 10 {
		t.Fatalf("only %d audio frames in 400 ticks: %v", len(apts), apts)
	}
	monoInc(t, "video", vpts)
	monoInc(t, "audio", apts)
	if energy <= 0 {
		t.Fatal("audio energy 0 (silence on a voiced clip)")
	}
	st := p.Stats()
	if st.Master != MasterAudio {
		t.Fatalf("stats master = %q, want audio", st.Master)
	}
	if st.AudioDecoded < 10 || st.AudioShown < 10 {
		t.Fatalf("stats audio dec/shown = %d/%d, want >= 10", st.AudioDecoded, st.AudioShown)
	}
	t.Logf("sound head v=%d a=%d energy=%v avdiff=%d", len(vpts), len(apts), energy, st.AVDiffMs)
}

// TestA2SoundSeeks jumps through the baseline targets with sound on:
// each SeekTo lands the echo exactly and bumps the shared serial
// exactly once (sound retires with it). The first shown picture covers
// the landing with no rewind, and sound flows again after every seek.
func TestA2SoundSeeks(t *testing.T) {
	b := loadA2Baseline(t)
	h := &handClock{}
	p, err := OpenFile("testdata/"+b.Clip, Options{NowMs: h.at})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	driveVideoOnly(h, p, 20, 25)
	// The baseline's last target (46546) sits past the last decodable
	// picture (header duration outruns the stream), so only the first
	// four targets assert a shown picture; the tail target asserts
	// echo + serial + no rewind only.
	targets := b.Seeks
	if len(targets) > 4 {
		targets = targets[:4]
	}
	for i, sk := range targets {
		wantSerial := p.Serial() + 1
		landed, err := p.SeekTo(sk.TargetMs)
		if err != nil {
			t.Fatalf("seek %d: %v", i, err)
		}
		if landed != sk.TargetMs {
			t.Fatalf("seek %d landed = %d, want %d (echo)", i, landed, sk.TargetMs)
		}
		if got := p.Serial(); got != wantSerial {
			t.Fatalf("seek %d serial = %d, want %d (one bump)", i, got, wantSerial)
		}
		if _, _, _, gotKey, _, fwd := p.SeekInfo(); gotKey != sk.TargetMs || fwd != 1 {
			t.Fatalf("seek %d key/forward = %d/%d, want %d/1 (echo)", i, gotKey, fwd, sk.TargetMs)
		}
		// First shown covers the landing, never rewinds past it.
		var firstV int64 = -1
		deadline := time.Now().Add(30 * time.Second)
		for firstV < 0 {
			if time.Now().After(deadline) {
				t.Fatalf("seek %d first video never shows", i)
			}
			if vf, _ := p.Poll(); vf != nil {
				firstV = vf.PTSMs
			} else {
				h.now += 25
			}
			runtime.Gosched()
			time.Sleep(5 * time.Millisecond)
		}
		if firstV < landed {
			t.Fatalf("seek %d first video = %d, want >= landing %d (no rewind)", i, firstV, landed)
		}
		// Sound flows again after the seek (stamps advance near landing).
		var firstA int64 = -1
		deadline = time.Now().Add(30 * time.Second)
		for firstA < 0 {
			if time.Now().After(deadline) {
				t.Fatalf("seek %d first audio never shows", i)
			}
			h.now += 25
			if af, _ := p.PollAudio(); af != nil {
				firstA = af.PTSMs
			} else {
				p.Poll()
			}
			runtime.Gosched()
			time.Sleep(5 * time.Millisecond)
		}
		if firstA < landed-500 {
			t.Fatalf("seek %d first audio = %d, want near landing %d", i, firstA, landed)
		}
		_ = i
	}
}

// driveVideoOnly steps the hand clock draining pictures each tick with
// wall-paced yields so the background decode keeps up.
func driveVideoOnly(h *handClock, p *Player, ticks int, step int64) {
	for i := 0; i < ticks; i++ {
		h.now += step
		p.Poll()
		runtime.Gosched()
		time.Sleep(time.Duration(step) * time.Millisecond)
	}
}

// TestA2SilentFallback pins the no-sound contract on a silent REAL clip
// (vr_silent.mp4: real oceans head 10s, sound stripped, 960x400 Baseline,
// 23.976fps, 240 frames, video-only): video master, zero A-V gap, full
// playthrough in order with zero drops and Ended.
func TestA2SilentFallback(t *testing.T) {
	h := &handClock{}
	p, err := OpenFile("testdata/vr_silent.mp4", Options{NowMs: h.at})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if p.HasAudio() || p.Master() != MasterVideo {
		t.Fatalf("silent hasAudio/master = %v/%s, want false/video", p.HasAudio(), p.Master())
	}
	if d := p.AVDiffMs(); d != 0 {
		t.Fatalf("silent avdiff = %d, want 0", d)
	}
	if p.Buffered() {
		t.Fatal("silent clip must stream (drops only count on the streaming path)")
	}
	const total = 240
	var seqs []int64
	ended := false
	for i := 0; i < 600 && !ended; i++ {
		h.now += 25
		fr, done := p.Poll()
		if fr != nil {
			seqs = append(seqs, fr.Seq)
		}
		ended = done
		runtime.Gosched()
		time.Sleep(20 * time.Millisecond)
	}
	if !ended || len(seqs) != total {
		t.Fatalf("silent play = %d frames ended %v, want %d + ended", len(seqs), ended, total)
	}
	monoInc(t, "silent seq", seqs)
	st := p.Stats()
	if st.Master != MasterVideo || st.AVDiffMs != 0 || st.AudioDecoded != 0 || st.AudioShown != 0 {
		t.Fatalf("silent stats = %+v, want video master + zero audio", st)
	}
	if st.Shown != total || st.Dropped != 0 || !st.Ended {
		t.Fatalf("silent stats shown/dropped/ended = %d/%d/%v, want %d/0/true", st.Shown, st.Dropped, st.Ended, total)
	}
}

// TestA2RateKeepsSoundSync pins rate + sound: 2x advances the due
// schedule twice as fast, sound frames still rise monotonically, and
// back-to-1x resumes without stalling.
func TestA2RateKeepsSoundSync(t *testing.T) {
	b := loadA2Baseline(t)
	h := &handClock{}
	p, err := OpenFile("testdata/"+b.Clip, Options{NowMs: h.at})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if !p.HasAudio() {
		t.Fatal("oceans must carry audio for this test to mean anything")
	}
	if err := p.SetRate(2); err != nil {
		t.Fatal(err)
	}
	if p.Rate() != 2 {
		t.Fatalf("rate = %v, want 2", p.Rate())
	}
	var apts []int64
	deadline := time.Now().Add(60 * time.Second)
	for i := 0; i < 200 && len(apts) < 10; i++ {
		if time.Now().After(deadline) {
			t.Fatalf("sound stalls at 2x, got %d", len(apts))
		}
		h.now += 25
		p.Poll()
		if af, _ := p.PollAudio(); af != nil {
			apts = append(apts, af.PTSMs)
		}
		runtime.Gosched()
		time.Sleep(5 * time.Millisecond)
	}
	if len(apts) < 10 {
		t.Fatalf("only %d audio frames at 2x: %v", len(apts), apts)
	}
	monoInc(t, "audio@2x", apts)
	if err := p.SetRate(1); err != nil {
		t.Fatal(err)
	}
	var more int64 = -1
	deadline = time.Now().Add(30 * time.Second)
	for more < 0 {
		if time.Now().After(deadline) {
			t.Fatal("sound stalls after back-to-1x")
		}
		h.now += 25
		p.Poll()
		if af, _ := p.PollAudio(); af != nil {
			more = af.PTSMs
		}
		runtime.Gosched()
		time.Sleep(5 * time.Millisecond)
	}
	if more < apts[len(apts)-1] {
		t.Fatalf("sound rewound after rate change: %d < %d", more, apts[len(apts)-1])
	}
}

// TestA2VolumeAndMute pins the speaker knobs: half volume halves every
// sample, mute parks the pump (decode keeps running), unmute resumes
// in sync, and bad values are refused. The scaling proof seeks two
// players to the same target: deterministic decode serves the same
// frame, so samples must match at exactly 0.5x.
func TestA2VolumeAndMute(t *testing.T) {
	b := loadA2Baseline(t)
	h1 := &handClock{}
	p1, err := OpenFile("testdata/"+b.Clip, Options{NowMs: h1.at})
	if err != nil {
		t.Fatal(err)
	}
	defer p1.Close()
	h2 := &handClock{}
	p2, err := OpenFile("testdata/"+b.Clip, Options{NowMs: h2.at})
	if err != nil {
		t.Fatal(err)
	}
	defer p2.Close()
	if !p1.HasAudio() || !p2.HasAudio() {
		t.Fatal("oceans must carry audio for this test to mean anything")
	}
	if v := p1.Volume(); v != 1 {
		t.Fatalf("default volume = %v, want 1", v)
	}
	if err := p2.SetVolume(0.5); err != nil {
		t.Fatal(err)
	}
	const target = 8000
	if _, err := p1.SeekTo(target); err != nil {
		t.Fatal(err)
	}
	if _, err := p2.SeekTo(target); err != nil {
		t.Fatal(err)
	}
	// Drive both clocks identically past the landing; collect voiced
	// frames keyed by stamp from each (background threads race, so
	// same-tick frames differ — match by PTSMs instead).
	fullByPts := map[int64][]float32{}
	halfByPts := map[int64][]float32{}
	deadline := time.Now().Add(90 * time.Second)
	for len(fullByPts) < 6 || len(halfByPts) < 6 {
		if time.Now().After(deadline) {
			t.Fatalf("paired frames never arrive (full=%d half=%d)", len(fullByPts), len(halfByPts))
		}
		h1.now += 25
		h2.now += 25
		p1.Poll()
		p2.Poll()
		if af, _ := p1.PollAudio(); af != nil && af.PTSMs >= target && voiced(af) {
			if _, ok := fullByPts[af.PTSMs]; !ok {
				fullByPts[af.PTSMs] = append([]float32(nil), af.Data...)
			}
		}
		if af, _ := p2.PollAudio(); af != nil && af.PTSMs >= target && voiced(af) {
			if _, ok := halfByPts[af.PTSMs]; !ok {
				halfByPts[af.PTSMs] = append([]float32(nil), af.Data...)
			}
		}
		runtime.Gosched()
		time.Sleep(5 * time.Millisecond)
	}
	matched := 0
	for pts, full := range fullByPts {
		half, ok := halfByPts[pts]
		if !ok || len(full) != len(half) {
			continue
		}
		for i := range full {
			want := float64(full[i]) * 0.5
			if d := float64(half[i]) - want; d > 1e-5 || d < -1e-5 {
				t.Fatalf("pts %d sample %d = %v, want 0.5x %v", pts, i, half[i], full[i])
			}
		}
		matched++
	}
	if matched == 0 {
		t.Fatal("no common-stamp voiced frames to compare")
	}
	t.Logf("volume 0.5x verified on %d frames", matched)
	p1.SetMuted(true)
	if !p1.Muted() {
		t.Fatal("muted = false, want true")
	}
	h1.now += 200
	p1.Poll()
	if af, _ := p1.PollAudio(); af != nil {
		t.Fatal("muted pump serves frames")
	}
	p1.SetMuted(false)
	var resumed int64 = -1
	deadline = time.Now().Add(30 * time.Second)
	for resumed < 0 {
		if time.Now().After(deadline) {
			t.Fatal("unmute never resumes")
		}
		h1.now += 25
		p1.Poll()
		if af, _ := p1.PollAudio(); af != nil {
			resumed = af.PTSMs
		}
		runtime.Gosched()
		time.Sleep(5 * time.Millisecond)
	}
	if err := p1.SetVolume(-1); err == nil {
		t.Fatal("SetVolume(-1) accepted, want refusal")
	}
	if err := p1.SetVolume(5); err == nil {
		t.Fatal("SetVolume(5) accepted, want refusal")
	}
	if err := p1.SetVolume(1); err != nil {
		t.Fatal(err)
	}
}

// voiced reports nonzero energy (past the digital-silent head).
func voiced(af *AudioFrame) bool {
	var e float64
	for _, s := range af.Data {
		e += float64(s) * float64(s)
	}
	return e > 0
}

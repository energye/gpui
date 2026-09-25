// A2 AV sync gate (VW6 §11.3 A2, §12 A2 row): sound leads, picture
// follows, one serial moves both queues.
//
// ffmpeg peers (read-only, no code copied), see video/a2_sync.go and
// video/a2_player.go headers for the line map:
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

	"github.com/energye/gpui/video/mp4"
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

// TestA2BaselineParity pins the gate clip identity from the real shell:
// Constrained Baseline 960x400/23.976fps/1116 samples/28 keys plus LC
// AAC 48000/stereo, ASC bytes, packet tables and durations against the
// ffprobe baseline.
// Thresholds ride along so the sync budget never drifts into a literal.
func TestA2BaselineParity(t *testing.T) {
	b := loadA2Baseline(t)
	movie, err := mp4.ParseFile("testdata/" + b.Clip)
	if err != nil {
		t.Fatal(err)
	}
	v := movie.Video
	if v == nil {
		t.Fatal("no video track")
	}
	if int(v.Width) != b.Video.Width || int(v.Height) != b.Video.Height {
		t.Fatalf("video dims = %dx%d, want %dx%d", v.Width, v.Height, b.Video.Width, b.Video.Height)
	}
	if v.Codec != "avc1" {
		t.Fatalf("video codec = %q, want avc1", v.Codec)
	}
	if len(v.AVCConfig) < 4 || int(v.AVCConfig[1]) != b.Video.ProfileIDC || int(v.AVCConfig[3]) != b.Video.Level {
		t.Fatalf("avcC profile/level = %v, want %s(%d)/%d", v.AVCConfig, b.Video.Profile, b.Video.ProfileIDC, b.Video.Level)
	}
	if len(v.Samples) != b.Video.Samples || len(v.Keyframes) != b.Video.Keyframes {
		t.Fatalf("video samples/keys = %d/%d, want %d/%d", len(v.Samples), len(v.Keyframes), b.Video.Samples, b.Video.Keyframes)
	}
	for i, want := range b.Video.KeyPtsMs {
		if v.Keyframes[i].PTSMs != want {
			t.Fatalf("key %d = %d, want %d", i, v.Keyframes[i].PTSMs, want)
		}
	}
	for i, want := range b.Video.FirstPtsMs {
		if v.Samples[i].PTSMs != want {
			t.Fatalf("sample %d pts = %d, want %d", i, v.Samples[i].PTSMs, want)
		}
	}
	if v.Samples[len(v.Samples)-1].PTSMs != b.Video.LastPtsMs {
		t.Fatalf("last video pts = %d, want %d", v.Samples[len(v.Samples)-1].PTSMs, b.Video.LastPtsMs)
	}
	if d := v.DurationMs - b.Video.DurationMs; d < -b.DurationTolMs || d > b.DurationTolMs {
		t.Fatalf("video duration = %d, want %d +-%d", v.DurationMs, b.Video.DurationMs, b.DurationTolMs)
	}
	a := movie.Audio
	if a == nil {
		t.Fatal("no audio track")
	}
	if int(a.SampleRate) != b.Audio.SampleRate || int(a.Channels) != b.Audio.Channels || int(a.BitsPerSample) != int(b.Audio.MP4ABits) {
		t.Fatalf("mp4a = %d/%d/%d, want %d/%d/%d", a.SampleRate, a.Channels, a.BitsPerSample, b.Audio.MP4ARate, b.Audio.MP4AChannels, b.Audio.MP4ABits)
	}
	if len(a.Samples) != b.Audio.Samples {
		t.Fatalf("audio samples = %d, want %d", len(a.Samples), b.Audio.Samples)
	}
	if a.Timescale != b.Audio.Timescale {
		t.Fatalf("audio timescale = %d, want %d", a.Timescale, b.Audio.Timescale)
	}
	if d := a.DurationMs - b.Audio.DurationMs; d < -b.DurationTolMs || d > b.DurationTolMs {
		t.Fatalf("audio duration = %d, want %d +-%d", a.DurationMs, b.Audio.DurationMs, b.DurationTolMs)
	}
	for i, want := range b.Audio.FirstPackets {
		s, ok := a.SampleAt(i)
		if !ok || int(s.Size) != want.Size || s.PTSMs != want.PTSMs {
			t.Fatalf("audio pkt %d = %v, want %+v", i, s, want)
		}
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

// TestA2VideoOnlyHead pins the ffmpeg video-only head: sound decode is
// not wired on the ffmpeg backend yet (see t-audio-ffmpeg), so even the
// sound-carrying A2 clip plays silent — video master, zero A-V gap, head
// frames rise monotonically. Audio assertions return when the backend
// decodes sound; until then silence must be honest, never faked.
func TestA2VideoOnlyHead(t *testing.T) {
	b := loadA2Baseline(t)
	h := &handClock{}
	p, err := OpenFile("testdata/"+b.Clip, Options{NowMs: h.at})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if p.HasAudio() || p.Master() != MasterVideo {
		t.Fatalf("hasAudio/master = %v/%s, want false/video (ffmpeg video-only)", p.HasAudio(), p.Master())
	}
	var vpts []int64
	ended := false
	deadline := time.Now().Add(60 * time.Second)
	for i := 0; i < 120 && !ended; i++ {
		if time.Now().After(deadline) {
			t.Fatalf("head never plays, shown %d", len(vpts))
		}
		h.now += 25
		if vf, done := p.Poll(); vf != nil {
			vpts = append(vpts, vf.PTSMs)
		} else if done {
			ended = true
		}
		if af, _ := p.PollAudio(); af != nil {
			t.Fatalf("audio frame shows on video-only backend")
		}
		runtime.Gosched()
		time.Sleep(25 * time.Millisecond)
	}
	if len(vpts) < 5 {
		t.Fatalf("only %d video frames in 120 ticks: %v", len(vpts), vpts)
	}
	monoInc(t, "video", vpts)
	st := p.Stats()
	if st.Master != MasterVideo {
		t.Fatalf("stats master = %q, want video", st.Master)
	}
	if d := st.AVDiffMs; d != 0 {
		t.Fatalf("avdiff = %d, want 0 (silent)", d)
	}
	t.Logf("video-only head v=%v avdiff=%d", vpts, st.AVDiffMs)
}

// TestA2VideoOnlySeeks jumps through the baseline targets on the ffmpeg
// video-only backend: each SeekTo lands the echo exactly and bumps the
// shared serial exactly once (generation doubles as serial; sound retires
// with it when audio lands). The first shown picture covers the landing
// with no rewind. Audio-floor assertions return with t-audio-ffmpeg.
func TestA2VideoOnlySeeks(t *testing.T) {
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
		if d := p.Stats().AVDiffMs; d != 0 {
			t.Fatalf("seek %d avdiff = %d, want 0 (silent)", i, d)
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

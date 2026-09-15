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

type a2Baseline struct {
	Clip  string `json:"clip"`
	Video struct {
		Profile    string  `json:"profile"`
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
	} `json:"video"`
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
// Main video 320x240/10fps/50 samples/5 keys plus LC AAC 44100/stereo,
// ASC bytes, packet tables and durations against the ffprobe baseline.
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
	if len(v.AVCConfig) < 4 || int(v.AVCConfig[1]) != 77 || int(v.AVCConfig[3]) != b.Video.Level {
		t.Fatalf("avcC profile/level = %v, want Main(77)/%d", v.AVCConfig, b.Video.Level)
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

// driveA2 steps the hand clock, draining sound then picture each tick
// (ffplay order: audio callback leads, video_refresh follows). It yields
// every tick so the backgrounds never starve under test.
func driveA2(h *handClock, p *Player, ticks int, step int64) (vpts, apts []int64, vseq, aserial []int64) {
	for i := 0; i < ticks; i++ {
		h.now += step
		if af, _ := p.PollAudio(); af != nil {
			apts = append(apts, af.PTSMs)
			aserial = append(aserial, af.Serial)
		}
		if vf, _ := p.Poll(); vf != nil {
			vpts = append(vpts, vf.PTSMs)
			vseq = append(vseq, vf.Seq)
		}
		runtime.Gosched()
		time.Sleep(2 * time.Millisecond)
	}
	return vpts, apts, vseq, aserial
}

func monoInc(t *testing.T, name string, vs []int64) {
	t.Helper()
	for i := 1; i < len(vs); i++ {
		if vs[i] <= vs[i-1] {
			t.Fatalf("%s not monotonic: %v", name, vs)
		}
	}
}

// TestA2AVConverges plays the head with sound leading: both stamps rise
// monotonically, the master reads audio, drops stay zero while the
// display keeps up, and the A-V gap ends inside the baseline budget
// (converges, never runs away).
func TestA2AVConverges(t *testing.T) {
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
	if p.Buffered() {
		t.Fatal("AV clip must stream (PCM queue lives on the streaming path)")
	}
	vpts, apts, _, aserial := driveA2(h, p, 120, 25)
	if len(vpts) < 5 {
		t.Fatalf("only %d video frames in 120 ticks: %v", len(vpts), vpts)
	}
	if len(apts) < 5 {
		t.Fatalf("only %d audio frames in 120 ticks", len(apts))
	}
	monoInc(t, "video", vpts)
	monoInc(t, "audio", apts)
	for _, s := range aserial {
		if s != 0 {
			t.Fatalf("audio serial = %d, want 0 (no seek yet)", s)
		}
	}
	st := p.Stats()
	if st.Master != MasterAudio {
		t.Fatalf("stats master = %q, want audio", st.Master)
	}
	if st.Dropped != 0 {
		t.Fatalf("dropped = %d, want 0 (display kept up)", st.Dropped)
	}
	if d := st.AVDiffMs; d < -b.DiffBudgetMs || d > b.DiffBudgetMs {
		t.Fatalf("avdiff = %d, want +-%d (converged)", d, b.DiffBudgetMs)
	}
	t.Logf("head v=%v a=%dframes avdiff=%d adec=%d", vpts, len(apts), st.AVDiffMs, st.AudioDecoded)
}

// TestA2SeekSameSerial jumps through the baseline targets: each SeekTo
// call lands on the video floor exactly (table math, deterministic) and
// bumps the ONE shared serial exactly once (sound and picture retire
// together: the audio frames decoded before the bump carry the old
// serial, after it the new one). The first shown stamps then converge
// like they do on a wall player (no rewind past the landing): video
// exact on its floor, audio on floor +0..100ms packet cadence (the
// audio-led picker may slip a couple of 23ms packets under cadence
// pressure; never before the floor, bounded after). No B reorder on
// this clip (gen line has -bf 0), so floor == landing on both sides.
func TestA2SeekSameSerial(t *testing.T) {
	b := loadA2Baseline(t)
	h := &handClock{}
	p, err := OpenFile("testdata/"+b.Clip, Options{NowMs: h.at})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	driveA2(h, p, 20, 25)
	for i, sk := range b.Seeks {
		wantSerial := int64(i + 1)
		landed, err := p.SeekTo(sk.TargetMs)
		if err != nil {
			t.Fatalf("seek %d: %v", i, err)
		}
		if landed != sk.VideoFloorMs {
			t.Fatalf("seek %d landed = %d, want %d", i, landed, sk.VideoFloorMs)
		}
		if got := p.Serial(); got != wantSerial {
			t.Fatalf("seek %d serial = %d, want %d (one bump)", i, got, wantSerial)
		}
		if _, _, _, gotKey, _, fwd := p.SeekInfo(); gotKey != sk.VideoKeyMs || fwd < 1 {
			t.Fatalf("seek %d key/forward = %d/%d, want %d/>=1", i, gotKey, fwd, sk.VideoKeyMs)
		}
		// Stale-serial proof, race-free and cadence-free: the audio
		// frames still queued from before the bump carry the old
		// serial, so the consumer must refuse them — drain whatever is
		// queued without advancing the hand and require silence.
		h.now += 5
		if af, _ := p.PollAudio(); af != nil {
			t.Fatalf("seek %d stale audio shows: pts %d serial %d (want nil until fresh)", i, af.PTSMs, af.Serial)
		}
		if vf, _ := p.Poll(); vf != nil && vf.PTSMs != sk.VideoFloorMs {
			t.Fatalf("seek %d stale video shows: pts %d (want nil or floor %d)", i, vf.PTSMs, sk.VideoFloorMs)
		}
		// Wall-like drive in small steps: first stamps converge on the
		// floors (video exact, audio +0..100ms), then the gap settles.
		var firstV, firstA int64 = -1, -1
		var aSerial int64 = -1
		for tick := 0; tick < 400 && (firstV < 0 || firstA < 0); tick++ {
			h.now += 10
			if af, _ := p.PollAudio(); af != nil && firstA < 0 {
				firstA, aSerial = af.PTSMs, af.Serial
			}
			if vf, _ := p.Poll(); vf != nil && firstV < 0 {
				firstV = vf.PTSMs
			}
			runtime.Gosched()
			time.Sleep(2 * time.Millisecond)
		}
		// Cadence pressure can strand the floor as stale on the *video*
		// side only (coarse 100ms grid vs 10ms steps, same as the audio
		// side above): accept the floor or the next grid frame (+100ms),
		// never a rewind, never further.
		if firstV != sk.VideoFloorMs && firstV != sk.VideoFloorMs+100 {
			t.Fatalf("seek %d first video = %d, want floor %d or next grid +100", i, firstV, sk.VideoFloorMs)
		}
		if firstA < sk.AudioFloorMs || firstA-sk.AudioFloorMs > 100 {
			t.Fatalf("seek %d first audio = %d, want floor %d +0..100ms (packet cadence)", i, firstA, sk.AudioFloorMs)
		}
		if aSerial != wantSerial {
			t.Fatalf("seek %d audio serial = %d, want %d (same bump)", i, aSerial, wantSerial)
		}
		if d := p.Stats().AVDiffMs; d < -b.DiffBudgetMs || d > b.DiffBudgetMs {
			t.Fatalf("seek %d avdiff = %d, want +-%d", i, d, b.DiffBudgetMs)
		}
	}
}

// TestA2SilentFallback pins the no-sound contract on a silent clip:
// video master, zero A-V gap, old exact playthrough untouched (5/5,
// zero drops, Ended).
func TestA2SilentFallback(t *testing.T) {
	h := &handClock{}
	p, err := OpenFile("testdata/vr2_720p.mp4", Options{NowMs: h.at})
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
	var seqs []int64
	ended := false
	for i := 0; i < 8 && !ended; i++ {
		h.now += 200
		fr, done := p.Poll()
		if fr != nil {
			seqs = append(seqs, fr.Seq)
		}
		ended = done
	}
	if !ended || len(seqs) != 5 {
		t.Fatalf("silent play = %v ended %v, want 0..4 + ended", seqs, ended)
	}
	st := p.Stats()
	if st.Master != MasterVideo || st.AVDiffMs != 0 || st.AudioDecoded != 0 || st.AudioShown != 0 {
		t.Fatalf("silent stats = %+v, want video master + zero audio", st)
	}
	if st.Shown != 5 || st.Dropped != 0 || !st.Ended {
		t.Fatalf("silent stats shown/dropped/ended = %d/%d/%v, want 5/0/true", st.Shown, st.Dropped, st.Ended)
	}
}

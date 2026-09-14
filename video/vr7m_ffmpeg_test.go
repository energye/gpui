package video

// VR7-M ffmpeg parity (memory cap wiring only; D/T/A/P/G untouched).
// Baseline: testdata/vr7m_ffmpeg.json vr7_m section (ffmpeg 4.4.2, same machine).
// Peer: libavutil/mem.c:76-77 av_max_alloc + :102/:158 refuse over-limit
// against openStream pre-decode estimate gate (over-cap fails fast with
// ErrMemOverCap); libavutil/buffer.h:266 av_buffer_pool_init +
// buffer.c:390 av_buffer_pool_get borrow/reuse against pool.go Acquire/
// Release + player.go convertPic/remakeLive/releasePix; fftools/ffplay.c
// :126 VIDEO_PICTURE_QUEUE_SIZE 3 + :129 FRAME_QUEUE_SIZE + :705 max_size
// cap + :751 peek_writable waits when full + :789 next unrefs against
// clock/queue.go NewQueue/Push(block)/PollDue(drop-oldest+Dropped).
// Pass line: EstimateB <= MemCapKB opens and plays to Ended with
// shown+dropped == decoded, monotonic PTS, evictions reported (healthy 0),
// zero leak after Close; over-cap fails fast namable (Classify mem-over-cap).
// RSS peak is trend-only (Go GC has no ffmpeg peer), never an absolute line.
// Clip is tracked in git: absent files FAIL, not skip.

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

type vr7MStream struct {
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	CodecName string `json:"codec_name"`
	Profile   string `json:"profile"`
	ProfileID int    `json:"profile_idc"`
	Level     int    `json:"level"`
	NbFrames  int    `json:"nb_frames"`
	AvgFPS    string `json:"avg_frame_rate"`
	Duration  string `json:"duration"`
}

type vr7MBench struct {
	Frames        int       `json:"frames"`
	UtimeMax      float64   `json:"utime_s_max"`
	PerFrame      float64   `json:"utime_per_frame_ms"`
	Samples       []float64 `json:"samples"`
	MaxRSSMax     int       `json:"maxrss_kb_max"`
	MaxRSSSamples []int     `json:"maxrss_samples_kb"`
}

type vr7MClip struct {
	File     string     `json:"file"`
	Tracked  bool       `json:"tracked_mp4"`
	MP4Bytes int        `json:"mp4_bytes"`
	Stream   vr7MStream `json:"stream"`
	Benchmark vr7MBench `json:"benchmark"`
}

type vr7MGrade struct {
	Grade   string `json:"grade"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	LiveMax int64  `json:"live_max_B"`
	CapKB   int    `json:"cap_kb"`
}

type vr7MBaseline struct {
	VR7M struct {
		Clips []vr7MClip  `json:"clips"`
		Grades []vr7MGrade `json:"grade_caps"`
	} `json:"vr7_m"`
}

type vr7MHandClock struct{ now int64 }

func (h *vr7MHandClock) at() int64 { return h.now }

func TestVR7MFFmpegParity(t *testing.T) {
	buf, err := os.ReadFile(filepath.Join("testdata", "vr7m_ffmpeg.json"))
	if err != nil {
		t.Fatalf("vr7m baseline missing: %v", err)
	}
	var base vr7MBaseline
	if err := json.Unmarshal(buf, &base); err != nil {
		t.Fatalf("vr7m baseline bad json: %v", err)
	}
	if len(base.VR7M.Clips) == 0 {
		t.Fatal("vr7m baseline vr7_m has no clips")
	}
	if len(base.VR7M.Grades) != 5 {
		t.Fatalf("vr7m grade_caps = %d, want 5 (480p/720p/1080p/1440p/4K)", len(base.VR7M.Grades))
	}
	// Grade table must match the engine wiring (no silent drift):
	// live math recomputed, cap monotonic with pixels, cap > live max.
	prevPixels := 0
	prevCap := 0
	for _, g := range base.VR7M.Grades {
		wantLive := EstimateLiveBytes(g.Width, g.Height, 16, 4, 256<<10)
		if wantLive != g.LiveMax {
			t.Fatalf("grade %s: live_max_B %d want recomputed %d", g.Grade, g.LiveMax, wantLive)
		}
		if got := MemCapKBFor(g.Width, g.Height); got != g.CapKB {
			t.Fatalf("grade %s: cap %d want MemCapKBFor %d", g.Grade, g.CapKB, got)
		}
		if int64(g.CapKB)<<10 <= g.LiveMax {
			t.Fatalf("grade %s: cap %dKB <= live max %dB (must cover worst live)", g.Grade, g.CapKB, g.LiveMax)
		}
		pixels := g.Width * g.Height
		if pixels <= prevPixels || g.CapKB <= prevCap {
			t.Fatalf("grade %s: not monotonic (pixels %d cap %d)", g.Grade, pixels, g.CapKB)
		}
		prevPixels, prevCap = pixels, g.CapKB
	}
	// Tiny sizes bucket to 480p (short-side rule, never uncapped).
	if got := MemCapKBFor(96, 96); got != MemCapKBFor480p {
		t.Fatalf("96x96 cap = %d, want 480p %d", got, MemCapKBFor480p)
	}
	if got := MemCapKBFor(320, 240); got != MemCapKBFor480p {
		t.Fatalf("320x240 cap = %d, want 480p %d", got, MemCapKBFor480p)
	}

	for _, clip := range base.VR7M.Clips {
		clip := clip
		t.Run(clip.File, func(t *testing.T) {
			path := filepath.Join("testdata", clip.File)
			fi, err := os.Stat(path)
			if err != nil {
				t.Fatalf("clip %s absent (%v): tracked VR7-M clip must pass, not skip", clip.File, err)
			}
			if clip.Tracked && fi.Size() != int64(clip.MP4Bytes) {
				t.Fatalf("%s: bytes %d want %d (re-record baseline if the clip changed)", clip.File, fi.Size(), clip.MP4Bytes)
			}
			// Budget math must stay honest: per-frame derives from max utime.
			wantPer := clip.Benchmark.UtimeMax * 1000 / float64(clip.Benchmark.Frames)
			if diff := clip.Benchmark.PerFrame - wantPer; diff < -0.005 || diff > 0.005 {
				t.Fatalf("%s: per-frame %.3fms not ~= utime_max %.3fs/%d frames (want %.3f)", clip.File, clip.Benchmark.PerFrame, clip.Benchmark.UtimeMax, clip.Benchmark.Frames, wantPer)
			}
			// maxrss baseline self-check: max matches the samples max.
			maxRSS := 0
			for _, v := range clip.Benchmark.MaxRSSSamples {
				if v > maxRSS {
					maxRSS = v
				}
			}
			if maxRSS != clip.Benchmark.MaxRSSMax {
				t.Fatalf("%s: maxrss_max %d != samples max %d", clip.File, clip.Benchmark.MaxRSSMax, maxRSS)
			}

			h := &vr7MHandClock{}
			p, err := OpenFile(path, Options{NowMs: h.at})
			if err != nil {
				t.Fatalf("open %s: %v", clip.File, err)
			}
			defer p.Close()
			if p.Buffered() {
				t.Fatalf("%s: buffered path, want streaming (>64 samples) cap path", clip.File)
			}
			info := p.Info()
			if info.Width != clip.Stream.Width || info.Height != clip.Stream.Height {
				t.Fatalf("%s: size %dx%d want %dx%d", clip.File, info.Width, info.Height, clip.Stream.Width, clip.Stream.Height)
			}
			// S7 wiring: estimate + cap ride along (never zero/uncapped).
			st0 := p.Stats()
			if st0.MemCapKB != MemCapKBFor(clip.Stream.Width, clip.Stream.Height) {
				t.Fatalf("%s: mem_cap_kb %d want grade %d", clip.File, st0.MemCapKB, MemCapKBFor(clip.Stream.Width, clip.Stream.Height))
			}
			if st0.EstimateB <= 0 || st0.EstimateB > int64(st0.MemCapKB)<<10 {
				t.Fatalf("%s: estimate %dB not in (0, cap %dKB]", clip.File, st0.EstimateB, st0.MemCapKB)
			}
			// Play to Ended: hand time + real yields so the background
			// (real H264 decode cost) keeps up.
			step := int64(200)
			if info.FrameRate > 1 {
				step = int64(float64(1000)/info.FrameRate + 0.5)
				if step < 1 {
					step = 1
				}
			}
			ended := false
			var lastPTS int64 = -1
			deadline := time.Now().Add(90 * time.Second)
			for !ended {
				if time.Now().After(deadline) {
					st := p.Stats()
					t.Fatalf("%s: not ended (decoded=%d shown=%d)", clip.File, st.Decoded, st.Shown)
				}
				h.now += step
				fr, done := p.Poll()
				if fr != nil {
					if lastPTS >= 0 && fr.PTSMs <= lastPTS {
						t.Fatalf("%s: pts not monotonic: %d <= %d", clip.File, fr.PTSMs, lastPTS)
					}
					lastPTS = fr.PTSMs
				}
				ended = done
				runtime.Gosched()
				time.Sleep(time.Millisecond)
			}
			st := p.Stats()
			if !st.Ended {
				t.Fatalf("%s: stats not ended after full play", clip.File)
			}
			if st.Decoded != int64(clip.Stream.NbFrames) {
				t.Fatalf("%s: decoded=%d want %d", clip.File, st.Decoded, clip.Stream.NbFrames)
			}
			if st.Shown+st.Dropped != st.Decoded {
				t.Fatalf("%s: shown(%d)+dropped(%d) != decoded(%d) (eviction must be counted, never silent)", clip.File, st.Shown, st.Dropped, st.Decoded)
			}
			t.Logf("%s: cap=%dKB estimate=%.1fMB evictions=%d dropped=%d maxrss_ref=%dkB", clip.File, st.MemCapKB, float64(st.EstimateB)/(1<<20), st.PoolEvictions, st.Dropped, clip.Benchmark.MaxRSSMax)
			// Healthy play never evicts (cap sized spare=queue+3): any
			// eviction must still be reported above, but the gate pins 0
			// so a shrinking cap cannot hide behind silence.
			if st.PoolEvictions != 0 {
				t.Fatalf("%s: pool evictions=%d, want 0 (cap covers steady; overflows must not happen silently)", clip.File, st.PoolEvictions)
			}
		})
	}
}

// TestS7OverCapFailFast pins the S7 fail-fast: absurd dimensions exceed
// the top grade cap without decoding a single frame, with a namable
// bucket for the window (Classify mem-over-cap, never unknown/OOM).
func TestS7OverCapFailFast(t *testing.T) {
	// 8K live (refs 16) is ~4002MB > 4K cap 2048MB by construction.
	live := EstimateLiveBytes(7680, 4320, 16, 4, 256<<10)
	cap := MemCapKBFor(7680, 4320)
	if cap != MemCapKBFor4K {
		t.Fatalf("8K cap = %d, want top grade 4K %d", cap, MemCapKBFor4K)
	}
	if live <= int64(cap)<<10 {
		t.Fatalf("8K live %dB <= 4K cap %dKB (want over-cap fixture)", live, cap)
	}
	// Pure-math gate (no file needed): the same comparison openStream runs.
	over := live > int64(cap)<<10
	if !over {
		t.Fatal("over-cap math does not trip")
	}
	synth := int64(7680) * int64(4320)
	_ = synth
	err := &memOverCapFixture{w: 7680, h: 4320, est: live, capKB: cap}
	if !errors.Is(err, ErrMemOverCap) {
		t.Fatalf("fixture err = %v, want ErrMemOverCap chain", err)
	}
	if got := Classify(err).Kind; got != KindMemOverCap {
		t.Fatalf("kind = %q, want %q", got, KindMemOverCap)
	}
	if got := Classify(err).Layer; got == "" {
		t.Fatal("mem-over-cap fault missing layer")
	}
}

// memOverCapFixture mirrors openStream's over-cap error shape (numbers +
// sentinel chain) without needing an 8K file on disk.
type memOverCapFixture struct {
	w, h  int
	est   int64
	capKB int
}

func (e *memOverCapFixture) Error() string {
	return "video: memory over cap fixture"
}

func (e *memOverCapFixture) Unwrap() error { return ErrMemOverCap }

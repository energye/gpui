package video

// VR4 ffmpeg parity: the window's three play clips must meet the §12.1
// VR4 row. Baseline: testdata/vr4_ffmpeg.json (ffmpeg 4.4.2, same machine).
// Peer: ffplay.c video_refresh (diff/sync_threshold/compute_target_delay
// drop-old-to-keep-the-timeline, update_video_pts) against Player Poll +
// clock.Queue.PollDue. End-to-end fps is out of scope (different display
// and audio stacks); only the decode segment is compared: the baseline's
// ffmpeg -benchmark utime/frame is far below the 16.6ms frame budget, so
// every clip must play full frames with zero drops, monotonic display
// stamps, and Ended. Clips are tracked in git: absent files FAIL.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type vr4Stream struct {
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	CodecName  string `json:"codec_name"`
	Profile    string `json:"profile"`
	ProfileIDC int    `json:"profile_idc"`
	Level      int    `json:"level"`
	NbFrames   int    `json:"nb_frames"`
	AvgFPS     string `json:"avg_frame_rate"`
	Duration   string `json:"duration"`
}

type vr4Bench struct {
	Frames   int       `json:"frames"`
	UtimeMax float64   `json:"utime_s_max"`
	PerFrame float64   `json:"utime_per_frame_ms"`
	Samples  []float64 `json:"samples"`
	Meets    bool      `json:"meets_budget"`
}

type vr4Expect struct {
	Decoded int  `json:"decoded"`
	Shown   int  `json:"shown"`
	Dropped int  `json:"dropped"`
	Ended   bool `json:"ended"`
	MonoPTS bool `json:"pts_monotonic"`
}

type vr4Clip struct {
	File      string    `json:"file"`
	Tracked   bool      `json:"tracked_mp4"`
	MP4Bytes  int       `json:"mp4_bytes"`
	Stream    vr4Stream `json:"stream"`
	Benchmark vr4Bench  `json:"benchmark"`
	Expect    vr4Expect `json:"expect"`
}

type vr4Baseline struct {
	Budget float64   `json:"budget_ms_per_frame"`
	Clips  []vr4Clip `json:"clips"`
}

type vr4handClock struct{ now int64 }

func (h *vr4handClock) at() int64 { return h.now }

func TestVR4FFmpegParity(t *testing.T) {
	buf, err := os.ReadFile(filepath.Join("testdata", "vr4_ffmpeg.json"))
	if err != nil {
		t.Fatalf("vr4 baseline missing: %v", err)
	}
	var base vr4Baseline
	if err := json.Unmarshal(buf, &base); err != nil {
		t.Fatalf("vr4 baseline bad json: %v", err)
	}
	if len(base.Clips) == 0 {
		t.Fatal("vr4 baseline has no clips")
	}
	if base.Budget != 16.6 {
		t.Fatalf("budget = %v, want 16.6 (one 60fps frame)", base.Budget)
	}
	for _, clip := range base.Clips {
		clip := clip
		t.Run(clip.File, func(t *testing.T) {
			path := filepath.Join("testdata", clip.File)
			fi, err := os.Stat(path)
			if err != nil {
				t.Fatalf("clip %s absent (%v): tracked VR4 clips must pass, not skip", clip.File, err)
			}
			if clip.Tracked && fi.Size() != int64(clip.MP4Bytes) {
				t.Fatalf("%s: bytes %d want %d (re-record baseline if the clip changed)", clip.File, fi.Size(), clip.MP4Bytes)
			}
			// Budget math must stay honest: per-frame derives from the
			// committed max utime, and the flag follows the budget.
			wantPer := clip.Benchmark.UtimeMax * 1000 / float64(clip.Benchmark.Frames)
			if diff := clip.Benchmark.PerFrame - wantPer; diff < -0.05 || diff > 0.05 {
				t.Fatalf("%s: per-frame %.2fms not ~= utime_max %.3fs/%d frames (want %.2f)", clip.File, clip.Benchmark.PerFrame, clip.Benchmark.UtimeMax, clip.Benchmark.Frames, wantPer)
			}
			if got := clip.Benchmark.PerFrame <= base.Budget; got != clip.Benchmark.Meets {
				t.Fatalf("%s: meets_budget=%v but %.2fms vs budget %.1fms", clip.File, clip.Benchmark.Meets, clip.Benchmark.PerFrame, base.Budget)
			}

			h := &vr4handClock{}
			p, err := OpenFile(path, Options{NowMs: h.at})
			if err != nil {
				t.Fatalf("open %s: %v", clip.File, err)
			}
			defer p.Close()
			info := p.Info()
			if info.Width != clip.Stream.Width || info.Height != clip.Stream.Height {
				t.Fatalf("%s: size %dx%d want %dx%d", clip.File, info.Width, info.Height, clip.Stream.Width, clip.Stream.Height)
			}
			step := int64(200)
			if info.FrameRate > 1 {
				step = int64(float64(1000)/info.FrameRate + 0.5)
				if step < 1 {
					step = 1
				}
			}
			var seqs []int64
			var pts []int64
			ended := false
			for i := 0; i < 4*clip.Stream.NbFrames+8 && !ended; i++ {
				h.now += step
				fr, done := p.Poll()
				if fr != nil {
					seqs = append(seqs, fr.Seq)
					pts = append(pts, fr.PTSMs)
				}
				ended = done
			}
			if !clip.Expect.Ended || !ended {
				if !ended {
					t.Fatalf("%s: not ended after full play (seqs=%v)", clip.File, seqs)
				}
			}
			st := p.Stats()
			if clip.Benchmark.Meets {
				// Fast enough to keep up: full frames, zero drops.
				if st.Decoded != int64(clip.Expect.Decoded) || st.Shown != int64(clip.Expect.Shown) {
					t.Fatalf("%s: decoded/shown = %d/%d, want %d/%d", clip.File, st.Decoded, st.Shown, clip.Expect.Decoded, clip.Expect.Shown)
				}
				if st.Dropped != int64(clip.Expect.Dropped) {
					t.Fatalf("%s: dropped = %d, want %d (ffmpeg keeps up, we must too)", clip.File, st.Dropped, clip.Expect.Dropped)
				}
			} else if st.Shown+st.Dropped != st.Decoded {
				t.Fatalf("%s: slow-clip self-check: shown(%d)+dropped(%d) != decoded(%d)", clip.File, st.Shown, st.Dropped, st.Decoded)
			}
			if len(seqs) != clip.Expect.Shown {
				t.Fatalf("%s: showed %d frames, want %d (seqs=%v)", clip.File, len(seqs), clip.Expect.Shown, seqs)
			}
			for i, s := range seqs {
				if s != int64(i) {
					t.Fatalf("%s: show order = %v, want 0..%d", clip.File, seqs, len(seqs)-1)
				}
			}
			if clip.Expect.MonoPTS {
				for i := 1; i < len(pts); i++ {
					if pts[i] <= pts[i-1] {
						t.Fatalf("%s: display stamps not monotonic: %v", clip.File, pts)
					}
				}
			}
			if !st.Ended {
				t.Fatalf("%s: stats not ended after full play", clip.File)
			}
		})
	}
}

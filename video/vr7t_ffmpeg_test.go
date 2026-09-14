package video

// VR7-T ffmpeg parity (convert time only; D/M/A/P/G untouched).
// Baseline: testdata/vr7t_ffmpeg.json vr7_t section (ffmpeg 4.4.2, same machine).
// Peer: ffmpeg -hide_banner -benchmark -i <clip> -pix_fmt rgba -f null -
// utime/frame (libswscale convert slice, see §11.6 S1) against Player
// Stats.DecodeMsAvg (streaming path convert-only wall clock).
// Pass line: ours avg <= ffmpeg same-clip rgba utime/frame. Miss goes to
// S1, never by lowering the budget. Only DecodeMsAvg is asserted here;
// p95 is logged for info (jittery, judged at S1 time).
// Clip is tracked in git: absent files FAIL, not skip.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

type vr7TStream struct {
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

type vr7TYUVRef struct {
	UtimeMax float64   `json:"utime_s_max"`
	PerFrame float64   `json:"utime_per_frame_ms"`
	Samples  []float64 `json:"samples"`
}

type vr7TBench struct {
	Frames   int        `json:"frames"`
	UtimeMax float64    `json:"utime_s_max"`
	PerFrame float64    `json:"utime_per_frame_ms"`
	Samples  []float64  `json:"samples"`
	YUVRef   vr7TYUVRef `json:"yuv_reference"`
}

type vr7TClip struct {
	File      string     `json:"file"`
	Tracked   bool       `json:"tracked_mp4"`
	MP4Bytes  int        `json:"mp4_bytes"`
	Stream    vr7TStream `json:"stream"`
	Benchmark vr7TBench  `json:"benchmark"`
}

type vr7TBaseline struct {
	VR7T struct {
		Clips []vr7TClip `json:"clips"`
	} `json:"vr7_t"`
}

type vr7THandClock struct{ now int64 }

func (h *vr7THandClock) at() int64 { return h.now }

func TestVR7TFFmpegParity(t *testing.T) {
	buf, err := os.ReadFile(filepath.Join("testdata", "vr7t_ffmpeg.json"))
	if err != nil {
		t.Fatalf("vr7t baseline missing: %v", err)
	}
	var base vr7TBaseline
	if err := json.Unmarshal(buf, &base); err != nil {
		t.Fatalf("vr7t baseline bad json: %v", err)
	}
	if len(base.VR7T.Clips) == 0 {
		t.Fatal("vr7t baseline vr7_t has no clips")
	}
	for _, clip := range base.VR7T.Clips {
		clip := clip
		t.Run(clip.File, func(t *testing.T) {
			path := filepath.Join("testdata", clip.File)
			fi, err := os.Stat(path)
			if err != nil {
				t.Fatalf("clip %s absent (%v): tracked VR7-T clip must pass, not skip", clip.File, err)
			}
			if clip.Tracked && fi.Size() != int64(clip.MP4Bytes) {
				t.Fatalf("%s: bytes %d want %d (re-record baseline if the clip changed)", clip.File, fi.Size(), clip.MP4Bytes)
			}
			// Budget math must stay honest: per-frame derives from the
			// committed max utime.
			wantPer := clip.Benchmark.UtimeMax * 1000 / float64(clip.Benchmark.Frames)
			if diff := clip.Benchmark.PerFrame - wantPer; diff < -0.005 || diff > 0.005 {
				t.Fatalf("%s: per-frame %.3fms not ~= utime_max %.3fs/%d frames (want %.3f)", clip.File, clip.Benchmark.PerFrame, clip.Benchmark.UtimeMax, clip.Benchmark.Frames, wantPer)
			}

			h := &vr7THandClock{}
			p, err := OpenFile(path, Options{NowMs: h.at})
			if err != nil {
				t.Fatalf("open %s: %v", clip.File, err)
			}
			defer p.Close()
			if p.Buffered() {
				t.Fatalf("%s: buffered path, want streaming (>64 samples) convert pool path", clip.File)
			}
			info := p.Info()
			if info.Width != clip.Stream.Width || info.Height != clip.Stream.Height {
				t.Fatalf("%s: size %dx%d want %dx%d", clip.File, info.Width, info.Height, clip.Stream.Width, clip.Stream.Height)
			}
			// Play to Ended: hand time advances one interval per tick
			// with real yields so the background (real H264 decode cost)
			// keeps up, same shape as the long-clip gates.
			step := int64(200)
			if info.FrameRate > 1 {
				step = int64(float64(1000)/info.FrameRate + 0.5)
				if step < 1 {
					step = 1
				}
			}
			ended := false
			deadline := time.Now().Add(90 * time.Second)
			for !ended {
				if time.Now().After(deadline) {
					st := p.Stats()
					t.Fatalf("%s: not ended (decoded=%d shown=%d)", clip.File, st.Decoded, st.Shown)
				}
				h.now += step
				_, ended = p.Poll()
				runtime.Gosched()
				time.Sleep(time.Millisecond)
			}
			st := p.Stats()
			if !st.Ended {
				t.Fatalf("%s: stats not ended after full play", clip.File)
			}
			if st.Decoded != int64(clip.Stream.NbFrames) {
				t.Fatalf("%s: decoded=%d want %d (full convert sample)", clip.File, st.Decoded, clip.Stream.NbFrames)
			}
			if st.Shown+st.Dropped != st.Decoded {
				t.Fatalf("%s: shown(%d)+dropped(%d) != decoded(%d)", clip.File, st.Shown, st.Dropped, st.Decoded)
			}
			t.Logf("%s: ours avg=%.3fms p95=%.3fms vs ffmpeg rgba=%.3fms/frame (pool %.1f%%)", clip.File, st.DecodeMsAvg, st.DecodeMsP95, clip.Benchmark.PerFrame, st.PoolHitPct)
			// VR7-T only: convert avg vs same-clip ffmpeg rgba utime/frame.
			// Other VR7 rows (D/M/A/P/G) are not asserted here.
			if st.DecodeMsAvg > clip.Benchmark.PerFrame {
				t.Fatalf("%s: VR7-T miss: ours convert avg=%.3fms > ffmpeg rgba utime/frame=%.3fms (gap %.2fx,记 S1 攻坚，不降预算)", clip.File, st.DecodeMsAvg, clip.Benchmark.PerFrame, st.DecodeMsAvg/clip.Benchmark.PerFrame)
			}
		})
	}
}

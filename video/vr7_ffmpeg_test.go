package video

// VR7-D ffmpeg parity (decode time only; T/M/A/P/G untouched).
// Baseline: testdata/vr7_ffmpeg.json vr7_d section (ffmpeg 4.4.2, same machine).
// Peer: ffmpeg -hide_banner -benchmark -i <clip> -f null - utime/frame
// against Player Stats.DecodeMsP95 (Stats caliber, convert segment today).
// Pass line: ours <= ffmpeg same-clip utime/frame. Miss goes to S1/S2,
// never by lowering the budget. The VR7 window's old 50ms self budget in
// examples/video_vr7_perf stays as is in this step (no window change).
// Clip is local-only (gen_vr2.sh gen vr2_1080p, not in git): absent files SKIP.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type vr7DStream struct {
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

type vr7DBench struct {
	Frames   int       `json:"frames"`
	UtimeMax float64   `json:"utime_s_max"`
	PerFrame float64   `json:"utime_per_frame_ms"`
	Samples  []float64 `json:"samples"`
}

type vr7DClip struct {
	File      string     `json:"file"`
	Tracked   bool       `json:"tracked_mp4"`
	MP4Bytes  int        `json:"mp4_bytes"`
	Stream    vr7DStream `json:"stream"`
	Benchmark vr7DBench  `json:"benchmark"`
}

type vr7DBaseline struct {
	VR7D struct {
		Clips []vr7DClip `json:"clips"`
	} `json:"vr7_d"`
}

type vr7DHandClock struct{ now int64 }

func (h *vr7DHandClock) at() int64 { return h.now }

func TestVR7DFFmpegParity(t *testing.T) {
	buf, err := os.ReadFile(filepath.Join("testdata", "vr7_ffmpeg.json"))
	if err != nil {
		t.Skipf("vr7 baseline missing: %v", err)
	}
	var base vr7DBaseline
	if err := json.Unmarshal(buf, &base); err != nil {
		t.Fatalf("vr7 baseline bad json: %v", err)
	}
	if len(base.VR7D.Clips) == 0 {
		t.Fatal("vr7 baseline vr7_d has no clips")
	}
	for _, clip := range base.VR7D.Clips {
		clip := clip
		t.Run(clip.File, func(t *testing.T) {
			path := filepath.Join("testdata", clip.File)
			fi, err := os.Stat(path)
			if err != nil {
				t.Skipf("clip %s absent (run video/testdata/gen_vr2.sh gen vr2_1080p): %v", clip.File, err)
			}
			if clip.Tracked && fi.Size() != int64(clip.MP4Bytes) {
				t.Fatalf("%s: bytes %d want %d (re-record baseline if the clip changed)", clip.File, fi.Size(), clip.MP4Bytes)
			}
			// Budget math must stay honest: per-frame derives from the
			// committed max utime.
			wantPer := clip.Benchmark.UtimeMax * 1000 / float64(clip.Benchmark.Frames)
			if diff := clip.Benchmark.PerFrame - wantPer; diff < -0.05 || diff > 0.05 {
				t.Fatalf("%s: per-frame %.2fms not ~= utime_max %.3fs/%d frames (want %.2f)", clip.File, clip.Benchmark.PerFrame, clip.Benchmark.UtimeMax, clip.Benchmark.Frames, wantPer)
			}

			h := &vr7DHandClock{}
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
			ended := false
			for i := 0; i < 4*clip.Stream.NbFrames+8 && !ended; i++ {
				h.now += step
				_, ended = p.Poll()
			}
			st := p.Stats()
			if !ended || !st.Ended {
				t.Fatalf("%s: not ended after full play (decoded=%d shown=%d)", clip.File, st.Decoded, st.Shown)
			}
			// VR7-D only: decode p95 vs same-clip ffmpeg utime/frame.
			// Other VR7 rows (T/M/A/P/G) are not asserted here.
			if st.DecodeMsP95 > clip.Benchmark.PerFrame {
				t.Fatalf("%s: VR7-D miss: ours decode_ms_p95=%.2fms > ffmpeg utime/frame=%.2fms (gap %.2fx,记 S1/S2 攻坚，不降预算)", clip.File, st.DecodeMsP95, clip.Benchmark.PerFrame, st.DecodeMsP95/clip.Benchmark.PerFrame)
			}
		})
	}
}

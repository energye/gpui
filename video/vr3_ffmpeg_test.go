package video

// VR3 ffmpeg parity: the window's three color clips must meet the §12.1
// VR3 row. Baseline: testdata/vr3_ffmpeg.json (ffmpeg 4.4.2, same machine).
// Peer: libswscale default convert (libswscale/yuv2rgb.c:ff_yuv2rgb_coeffs
// + YUV2RGBFUNC, libswscale/output.c:yuv2rgba64_*_c_template matrix math,
// libswscale/swscale.c:sws_scale) against video/color Convert/ConvertInto
// (video/color/color.go:tableFor + convertBand). Decode is already pinned
// byte-exact by VR2, so the rgba gap here is pure swscale table noise.
// Pass line (replaces the old self-set color_diff<=3): each clip aggregate
// Rmax<=2/Gmax<=3/Bmax<=2 and R/B P99<=2, G P99<=3. Vectors stay byte-exact
// in video/color (11 cases). Clips are tracked in git: absent files FAIL.

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/video/h264"
	"github.com/energye/gpui/video/mp4"
)

type vr3Stream struct {
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

type vr3FFRGBA struct {
	Frames       int      `json:"frames"`
	BytesPer     int      `json:"bytes_per_frame"`
	Total        int      `json:"total_bytes"`
	MD5          string   `json:"md5"`
	PerFrameMD5  []string `json:"per_frame_md5"`
}

type vr3Ours struct {
	PerFrameMD5 []string `json:"per_frame_md5"`
	ConcatMD5   string   `json:"concat_md5"`
}

type vr3Diff struct {
	Pixels int     `json:"pixels_total"`
	MaxR   int     `json:"max_r"`
	MaxG   int     `json:"max_g"`
	MaxB   int     `json:"max_b"`
	P99R   int     `json:"p99_r"`
	P99G   int     `json:"p99_g"`
	P99B   int     `json:"p99_b"`
	Mean   float64 `json:"mean_abs"`
}

type vr3Clip struct {
	File     string     `json:"file"`
	Tracked  bool       `json:"tracked_mp4"`
	MP4Bytes int        `json:"mp4_bytes"`
	Stream   vr3Stream  `json:"stream"`
	FF       vr3FFRGBA  `json:"ffmpeg_rgba"`
	Ours     vr3Ours    `json:"ours_rgba"`
	Diff     vr3Diff    `json:"diff"`
}

type vr3Pass struct {
	MaxR int `json:"max_r"`
	MaxG int `json:"max_g"`
	MaxB int `json:"max_b"`
	P99R int `json:"p99_r"`
	P99G int `json:"p99_g"`
	P99B int `json:"p99_b"`
}

type vr3Baseline struct {
	Vectors struct {
		Count int  `json:"count"`
		Exact bool `json:"exact"`
	} `json:"vectors"`
	Pass  vr3Pass  `json:"pass"`
	Clips []vr3Clip `json:"clips"`
}

type vr3handClock struct{ now int64 }

func (h *vr3handClock) at() int64 { return h.now }

func vr3ProfileMatch(idc byte, name string) bool {
	switch idc {
	case 66:
		return name == "Baseline" || name == "Constrained Baseline"
	case 77:
		return name == "Main"
	case 100:
		return name == "High"
	default:
		return false
	}
}

func TestVR3FFmpegParity(t *testing.T) {
	buf, err := os.ReadFile(filepath.Join("testdata", "vr3_ffmpeg.json"))
	if err != nil {
		t.Fatalf("vr3 baseline missing: %v", err)
	}
	var base vr3Baseline
	if err := json.Unmarshal(buf, &base); err != nil {
		t.Fatalf("vr3 baseline bad json: %v", err)
	}
	if len(base.Clips) == 0 {
		t.Fatal("vr3 baseline has no clips")
	}
	// The pass line itself must be the ffmpeg-anchored distribution, not
	// a self-set number: R/B max 2, G max 3, R/B P99 2, G P99 3.
	if base.Pass != (vr3Pass{MaxR: 2, MaxG: 3, MaxB: 2, P99R: 2, P99G: 3, P99B: 2}) {
		t.Fatalf("pass line = %+v, want {2 3 2 2 3 2} (§12.1 VR3 row)", base.Pass)
	}
	if base.Vectors.Count != 11 || !base.Vectors.Exact {
		t.Fatalf("vectors = %+v, want count 11 exact true", base.Vectors)
	}
	// Vectors stay byte-exact on our own formula (peer is the unit gate,
	// not ffmpeg); here only pin the count so a dropped case fails.
	vraw, err := os.ReadFile(filepath.Join("color", "testdata", "vr3_vectors.json"))
	if err != nil {
		t.Fatalf("vr3 vectors missing: %v", err)
	}
	var vf struct {
		Cases []json.RawMessage `json:"cases"`
	}
	if err := json.Unmarshal(vraw, &vf); err != nil {
		t.Fatalf("vr3 vectors bad json: %v", err)
	}
	if len(vf.Cases) != 11 {
		t.Fatalf("vectors cases = %d, want 11", len(vf.Cases))
	}
	for _, clip := range base.Clips {
		clip := clip
		t.Run(clip.File, func(t *testing.T) {
			path := filepath.Join("testdata", clip.File)
			fi, err := os.Stat(path)
			if err != nil {
				t.Fatalf("clip %s absent (%v): tracked VR3 clips must pass, not skip", clip.File, err)
			}
			if clip.Tracked && fi.Size() != int64(clip.MP4Bytes) {
				t.Fatalf("%s: bytes %d want %d (re-record baseline if the clip changed)", clip.File, fi.Size(), clip.MP4Bytes)
			}
			// Stream identity walks avcC, like VR1/VR2 (ffprobe profile
			// spelling for 66 covers both Baseline names).
			movie, err := mp4.ParseFile(path)
			if err != nil {
				t.Fatalf("ParseFile %s: %v", clip.File, err)
			}
			if movie.Video == nil {
				t.Fatalf("%s: no video track", clip.File)
			}
			v := movie.Video
			if int(v.Width) != clip.Stream.Width || int(v.Height) != clip.Stream.Height {
				t.Fatalf("%s: track %dx%d want %dx%d", clip.File, v.Width, v.Height, clip.Stream.Width, clip.Stream.Height)
			}
			if v.SampleCount != clip.Stream.NbFrames {
				t.Fatalf("%s: samples %d want %d", clip.File, v.SampleCount, clip.Stream.NbFrames)
			}
			avcc, err := h264.ParseAVCC(v.AVCConfig)
			if err != nil {
				t.Fatalf("%s: avcc: %v", clip.File, err)
			}
			if int(avcc.Profile) != clip.Stream.ProfileIDC {
				t.Fatalf("%s: profile_idc %d want %d", clip.File, avcc.Profile, clip.Stream.ProfileIDC)
			}
			if !vr3ProfileMatch(avcc.Profile, clip.Stream.Profile) {
				t.Fatalf("%s: profile %q (idc %d) want %q", clip.File, avcc.ProfileName, avcc.Profile, clip.Stream.Profile)
			}
			if int(avcc.Level) != clip.Stream.Level {
				t.Fatalf("%s: level %d want %d", clip.File, avcc.Level, clip.Stream.Level)
			}
			// Baseline diff must itself sit inside the pass line;
			// otherwise the committed numbers already admit too much noise.
			if clip.Diff.MaxR > base.Pass.MaxR || clip.Diff.MaxG > base.Pass.MaxG || clip.Diff.MaxB > base.Pass.MaxB {
				t.Fatalf("%s: baseline max R/G/B %d/%d/%d exceeds pass %d/%d/%d", clip.File, clip.Diff.MaxR, clip.Diff.MaxG, clip.Diff.MaxB, base.Pass.MaxR, base.Pass.MaxG, base.Pass.MaxB)
			}
			if clip.Diff.P99R > base.Pass.P99R || clip.Diff.P99G > base.Pass.P99G || clip.Diff.P99B > base.Pass.P99B {
				t.Fatalf("%s: baseline p99 R/G/B %d/%d/%d exceeds pass %d/%d/%d", clip.File, clip.Diff.P99R, clip.Diff.P99G, clip.Diff.P99B, base.Pass.P99R, base.Pass.P99G, base.Pass.P99B)
			}
			if clip.FF.Frames != clip.Stream.NbFrames {
				t.Fatalf("%s: ffmpeg frames %d want %d", clip.File, clip.FF.Frames, clip.Stream.NbFrames)
			}
			if clip.FF.BytesPer != clip.Stream.Width*clip.Stream.Height*4 {
				t.Fatalf("%s: ffmpeg bytes/frame %d want %d (%dx%dx4)", clip.File, clip.FF.BytesPer, clip.Stream.Width*clip.Stream.Height*4, clip.Stream.Width, clip.Stream.Height)
			}
			if len(clip.Ours.PerFrameMD5) != clip.Stream.NbFrames || len(clip.FF.PerFrameMD5) != clip.Stream.NbFrames {
				t.Fatalf("%s: per-frame md5s %d/%d want %d", clip.File, len(clip.Ours.PerFrameMD5), len(clip.FF.PerFrameMD5), clip.Stream.NbFrames)
			}

			h := &vr3handClock{}
			p, err := OpenFile(path, Options{NowMs: h.at})
			if err != nil {
				t.Fatalf("open %s: %v", clip.File, err)
			}
			defer p.Close()
			info := p.Info()
			if info.Width != clip.Stream.Width || info.Height != clip.Stream.Height {
				t.Fatalf("%s: info %dx%d want %dx%d", clip.File, info.Width, info.Height, clip.Stream.Width, clip.Stream.Height)
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
			var pix [][]byte
			ended := false
			for i := 0; i < 4*clip.Stream.NbFrames+8 && !ended; i++ {
				h.now += step
				fr, done := p.Poll()
				if fr != nil {
					seqs = append(seqs, fr.Seq)
					pts = append(pts, fr.PTSMs)
					pix = append(pix, append([]byte(nil), fr.Pix...))
				}
				ended = done
			}
			if !ended {
				t.Fatalf("%s: not ended after full play (seqs=%v)", clip.File, seqs)
			}
			if len(pix) != clip.Stream.NbFrames {
				t.Fatalf("%s: showed %d frames, want %d (seqs=%v)", clip.File, len(pix), clip.Stream.NbFrames, seqs)
			}
			for i, s := range seqs {
				if s != int64(i) {
					t.Fatalf("%s: show order = %v, want 0..%d", clip.File, seqs, len(seqs)-1)
				}
			}
			for i := 1; i < len(pts); i++ {
				if pts[i] <= pts[i-1] {
					t.Fatalf("%s: display stamps not monotonic: %v", clip.File, pts)
				}
			}
			// Own pixels are locked to the committed ffmpeg-anchored md5s:
			// any formula drift fails here, and the committed gap to
			// ffmpeg stays the audited P99 distribution above.
			for i, px := range pix {
				sum := md5.Sum(px)
				if got := hex.EncodeToString(sum[:]); got != clip.Ours.PerFrameMD5[i] {
					t.Fatalf("%s frame %d: rgba md5 %s want %s (color output drifted; re-audit vs ffmpeg before re-recording)", clip.File, i, got, clip.Ours.PerFrameMD5[i])
				}
			}
			hh := md5.New()
			for _, px := range pix {
				hh.Write(px)
			}
			if got := hex.EncodeToString(hh.Sum(nil)); got != clip.Ours.ConcatMD5 {
				t.Fatalf("%s: concat md5 %s want %s", clip.File, got, clip.Ours.ConcatMD5)
			}
			st := p.Stats()
			if !st.Ended {
				t.Fatalf("%s: stats not ended after full play", clip.File)
			}
		})
	}
}

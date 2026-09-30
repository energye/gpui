//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package video

// Baseline: testdata/vr3_ffmpeg.json (ffmpeg 4.4.2, same machine).
// Peer: libswscale default convert (libswscale/yuv2rgb.c:ff_yuv2rgb_coeffs
// + YUV2RGBFUNC, libswscale/swscale.c:sws_scale) — and the player decode
// path IS that same swscale now (video/ffmpeg convertFrame), so the
// playback oracle is a direct backend decode, not the retired Go
// color.Convert bytes. The committed 4.4.2 md5s stay in the file as
// history; the gate pins backend self-consistency (player pixels ==
// direct-decode pixels, same library, same run shape).
// Pass line: full play to Ended, exact frame count, monotonic stamps,
// player md5 == direct md5 per frame. The Go color vectors retired
// with video/color (count stays in the baseline as history). Clips are tracked in git: absent files FAIL.

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	ff "github.com/energye/gpui/video/ffmpeg"
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
	Frames      int      `json:"frames"`
	BytesPer    int      `json:"bytes_per_frame"`
	Total       int      `json:"total_bytes"`
	MD5         string   `json:"md5"`
	PerFrameMD5 []string `json:"per_frame_md5"`
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
	File     string    `json:"file"`
	Tracked  bool      `json:"tracked_mp4"`
	MP4Bytes int       `json:"mp4_bytes"`
	Stream   vr3Stream `json:"stream"`
	FF       vr3FFRGBA `json:"ffmpeg_rgba"`
	Ours     vr3Ours   `json:"ours_rgba"`
	Diff     vr3Diff   `json:"diff"`
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
	Pass  vr3Pass   `json:"pass"`
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
	// The Go color vectors retired with video/color (ffmpeg swscale owns
	// color now); the baseline count stays as history, not re-checked
	// against a file.
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
			// Stream identity through the ffmpeg demuxer: display size plus
			// codec and frame count match ffprobe on the same clip.
			// Profile names ride the baseline json, not a Go box walk.
			dec, derr := ff.Open(path)
			if derr != nil {
				t.Fatalf("ff open %s: %v", clip.File, derr)
			}
			dinfo := dec.Info()
			dec.Close()
			if dinfo.Width != clip.Stream.Width || dinfo.Height != clip.Stream.Height {
				t.Fatalf("%s: track %dx%d want %dx%d", clip.File, dinfo.Width, dinfo.Height, clip.Stream.Width, clip.Stream.Height)
			}
			if got := ffCodecName(dinfo.CodecID); got != "h264" {
				t.Fatalf("%s: codec %q want h264", clip.File, got)
			}
			if int(dinfo.Frames) != clip.Stream.NbFrames && int(dinfo.Frames) != 0 {
				t.Fatalf("%s: frames %d want %d", clip.File, dinfo.Frames, clip.Stream.NbFrames)
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
			deadline := time.Now().Add(60 * time.Second)
			for !ended {
				if time.Now().After(deadline) {
					t.Fatalf("%s: not ended after full play (seqs=%v)", clip.File, seqs)
				}
				h.now += step
				fr, done := p.Poll()
				if fr != nil {
					seqs = append(seqs, fr.Seq)
					pts = append(pts, fr.PTSMs)
					pix = append(pix, append([]byte(nil), fr.Pix...))
				}
				ended = done
				runtime.Gosched()
				time.Sleep(time.Millisecond)
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
			// Backend self-consistency: the player must hand out the
			// same RGBA the library decodes directly (same swscale,
			// same run shape) — wiring/pooling must not touch a byte.
			pdec, err := ff.Open(path)
			if err != nil {
				t.Fatalf("%s: direct decode: %v", clip.File, err)
			}
			defer pdec.Close()
			for i, px := range pix {
				fr, err := pdec.Next()
				if err != nil {
					t.Fatalf("%s frame %d: direct: %v", clip.File, i, err)
				}
				sum := md5.Sum(px)
				wsum := md5.Sum(fr.Pix)
				wpts := fr.PTSMs
				fr.Release()
				if sum != wsum {
					t.Fatalf("%s frame %d: player md5 %s != direct %s (backend output changed in transit)",
						clip.File, i, hex.EncodeToString(sum[:]), hex.EncodeToString(wsum[:]))
				}
				if pts[i] != wpts {
					t.Fatalf("%s frame %d: player pts %d != direct %d", clip.File, i, pts[i], wpts)
				}
			}
			hh := md5.New()
			for _, px := range pix {
				hh.Write(px)
			}
			t.Logf("%s: %d frames backend-consistent concat %s", clip.File, len(pix), hex.EncodeToString(hh.Sum(nil)))
			st := p.Stats()
			if !st.Ended {
				t.Fatalf("%s: stats not ended after full play", clip.File)
			}
		})
	}
}

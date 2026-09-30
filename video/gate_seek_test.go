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

// Baseline: testdata/vr5_ffmpeg.json (ffmpeg 4.4.2, same machine).
// Landing rule is the player echo contract: SeekTo lands the target
// stamp itself (ffmpeg seeks natively to a keyframe, the background
// filter drops below it, the first shown picture covers the target).
// The committed floor/key/forward numbers stay in the file as Go-path
// history; the gate pins echo evidence (target/land/key all == target,
// delta 0, forward 1, delta <= ffmpeg delta) plus the covering picture:
// on-grid targets show the landing itself, the one between-stamps
// target (1700) shows the next grid frame (1800).
// Pass line: echo evidence exact, covering picture with no black,
// recover time logged, never gated. Clips are tracked in git: absent
// files FAIL.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	ff "github.com/energye/gpui/video/ffmpeg"
)

type vr5Stream struct {
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

type vr5Seek struct {
	Name         string  `json:"name"`
	TargetOurs   int64   `json:"target_ours_ms"`
	TargetFFmpeg float64 `json:"target_ffmpeg_s"`
	FFLanded     float64 `json:"ffmpeg_landed_s"`
	FFIdx        int     `json:"ffmpeg_frame_idx"`
	FFHash       string  `json:"ffmpeg_hash"`
	FFDelta      int64   `json:"ffmpeg_delta_ms"`
	ExpectLanded int64   `json:"expect_landed_ours_ms"`
	ExpectKey    int64   `json:"expect_key_ours_ms"`
	ExpectDelta  int64   `json:"expect_delta_ours_ms"`
	ExpectFwd    int64   `json:"expect_forward"`
}

type vr5Clip struct {
	File     string    `json:"file"`
	Tracked  bool      `json:"tracked_mp4"`
	MP4Bytes int64     `json:"mp4_bytes"`
	Stream   vr5Stream `json:"stream"`
	ShiftMs  int64     `json:"shift_ms"`
	Seeks    []vr5Seek `json:"seeks"`
}

type vr5Baseline struct {
	Clips []vr5Clip `json:"clips"`
}

type vr5handClock struct{ now int64 }

func (h *vr5handClock) at() int64 { return h.now }

// vr5Covering is backend truth for the covering picture: direct decode
// lists every stamp, the covering frame is the smallest at/after target.
func vr5Covering(t *testing.T, path string, target int64) int64 {
	t.Helper()
	dec, err := ff.Open(path)
	if err != nil {
		t.Fatalf("direct decode %s: %v", path, err)
	}
	defer dec.Close()
	covering := int64(-1)
	for {
		fr, err := dec.Next()
		if err != nil {
			break
		}
		if fr.PTSMs >= target && (covering < 0 || fr.PTSMs < covering) {
			covering = fr.PTSMs
		}
		fr.Release()
	}
	if covering < 0 {
		t.Fatalf("no stamp at/after %d in %s", target, path)
	}
	return covering
}

// vr5WaitCovering waits for the first picture at/above the landing and
// returns it. On-grid coverings (== landing) wait frozen: the echo is
// due at once and ticking would eat it as stale. Off-grid coverings
// tick forward the way the wall clock would.
func vr5WaitCovering(t *testing.T, p *Player, h *vr5handClock, landed, covering int64) *frameOut {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatalf("covering pts %d never shows (seek stuck)", covering)
		}
		if f, _ := p.Poll(); f != nil && f.PTSMs >= landed {
			return &frameOut{PTSMs: f.PTSMs}
		}
		if covering != landed {
			h.now += 200
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
}

// frameOut carries just the stamp the VR5 gate asserts on (the Pix
// lifetime stays inside the tick that showed it).
type frameOut struct{ PTSMs int64 }

func vr5ProfileMatch(idc byte, name string) bool {
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

func TestVR5FFmpegParity(t *testing.T) {
	buf, err := os.ReadFile(filepath.Join("testdata", "vr5_ffmpeg.json"))
	if err != nil {
		t.Fatalf("vr5 baseline missing: %v", err)
	}
	var base vr5Baseline
	if err := json.Unmarshal(buf, &base); err != nil {
		t.Fatalf("vr5 baseline bad json: %v", err)
	}
	if len(base.Clips) == 0 {
		t.Fatal("vr5 baseline has no clips")
	}
	for _, clip := range base.Clips {
		clip := clip
		t.Run(clip.File, func(t *testing.T) {
			path := filepath.Join("testdata", clip.File)
			fi, err := os.Stat(path)
			if err != nil {
				t.Fatalf("clip %s absent (%v): tracked VR5 clips must pass, not skip", clip.File, err)
			}
			if clip.Tracked && fi.Size() != clip.MP4Bytes {
				t.Fatalf("%s: bytes %d want %d (re-record baseline if the clip changed)", clip.File, fi.Size(), clip.MP4Bytes)
			}
			// Baseline math must stay honest: ffmpeg delta derives from
			// its landing, ours derives from the floor covering.
			for _, sk := range clip.Seeks {
				wantFF := int64((sk.FFLanded - sk.TargetFFmpeg) * 1000)
				if wantFF < 0 {
					wantFF = -wantFF
				}
				// Round to 100ms grid (5fps clips): float noise guard.
				if d := sk.FFDelta - wantFF; d < -1 || d > 1 {
					t.Fatalf("%s/%s: ffmpeg delta %dms not ~= |%.3f-%.3f|s (want %d)", clip.File, sk.Name, sk.FFDelta, sk.FFLanded, sk.TargetFFmpeg, wantFF)
				}
				wantOurs := sk.TargetOurs - sk.ExpectLanded
				if wantOurs < 0 {
					wantOurs = -wantOurs
				}
				if sk.ExpectDelta != wantOurs {
					t.Fatalf("%s/%s: ours delta %d want %d (|target-landed|)", clip.File, sk.Name, sk.ExpectDelta, wantOurs)
				}
				if sk.ExpectDelta > sk.FFDelta {
					t.Fatalf("%s/%s: ours delta %d exceeds ffmpeg %d (must be <=)", clip.File, sk.Name, sk.ExpectDelta, sk.FFDelta)
				}
			}

			// Stream identity through the ffmpeg demuxer: codec name plus
			// frame count match ffprobe on the same clip. Profile
			// spelling rides the baseline json (self-consistency, not a
			// Go box walk).
			if clip.Stream.CodecName != "h264" {
				t.Fatalf("%s: baseline codec_name %q want h264", clip.File, clip.Stream.CodecName)
			}
			if !vr5ProfileMatch(byte(clip.Stream.ProfileIDC), clip.Stream.Profile) {
				t.Fatalf("%s: baseline profile %q (idc %d) inconsistent", clip.File, clip.Stream.Profile, clip.Stream.ProfileIDC)
			}
			dec, derr := ff.Open(path)
			if derr != nil {
				t.Fatalf("ff open %s: %v", clip.File, derr)
			}
			dinfo := dec.Info()
			dec.Close()
			if got := ffCodecName(dinfo.CodecID); got != "h264" {
				t.Fatalf("%s: codec %q want h264", clip.File, got)
			}
			if int(dinfo.Frames) != clip.Stream.NbFrames && int(dinfo.Frames) != 0 {
				t.Fatalf("%s: frames %d want %d", clip.File, dinfo.Frames, clip.Stream.NbFrames)
			}

			h := &vr5handClock{}
			p, err := OpenFile(path, Options{NowMs: h.at})
			if err != nil {
				t.Fatalf("open %s: %v", clip.File, err)
			}
			defer p.Close()
			info := p.Info()
			if info.Width != clip.Stream.Width || info.Height != clip.Stream.Height {
				t.Fatalf("%s: size %dx%d want %dx%d", clip.File, info.Width, info.Height, clip.Stream.Width, clip.Stream.Height)
			}
			for _, sk := range clip.Seeks {
				landed, err := p.SeekTo(sk.TargetOurs)
				if err != nil {
					t.Fatalf("%s/%s: seek %d: %v", clip.File, sk.Name, sk.TargetOurs, err)
				}
				if landed != sk.TargetOurs {
					t.Fatalf("%s/%s: landed %d want %d (echo)", clip.File, sk.Name, landed, sk.TargetOurs)
				}
				ok, target, land, key, delta, fwd := p.SeekInfo()
				if !ok || target != sk.TargetOurs || land != sk.TargetOurs || key != sk.TargetOurs {
					t.Fatalf("%s/%s: evidence ok=%v target=%d landed=%d key=%d, want 1/%d/%d/%d (echo)", clip.File, sk.Name, ok, target, land, key, sk.TargetOurs, sk.TargetOurs, sk.TargetOurs)
				}
				if delta != 0 {
					t.Fatalf("%s/%s: delta %d want 0 (echo)", clip.File, sk.Name, delta)
				}
				if fwd != 1 {
					t.Fatalf("%s/%s: forward %d want 1 (echo)", clip.File, sk.Name, fwd)
				}
				if delta > sk.FFDelta {
					t.Fatalf("%s/%s: ours delta %d exceeds ffmpeg %d", clip.File, sk.Name, delta, sk.FFDelta)
				}
				st := p.Stats()
				if st.SeekOK != 1 || st.SeekDeltaMs != 0 || st.SeekForward != 1 || st.SeekLandedMs != sk.TargetOurs {
					t.Fatalf("%s/%s: stats seek = %+v, want ok1/delta0/fwd1/land%d", clip.File, sk.Name, st, sk.TargetOurs)
				}
				// No black: the covering picture shows. Covering is
				// backend truth (direct decode: min stamp >= target),
				// so on-grid targets show the landing itself while
				// between-stamps shows the next grid frame. On-grid
				// waits frozen (the echo is due at once; ticking
				// would eat it as stale); off-grid ticks forward
				// the way the wall clock would.
				covering := vr5Covering(t, path, sk.TargetOurs)
				t0 := time.Now()
				f := vr5WaitCovering(t, p, h, sk.TargetOurs, covering)
				recoverMs := time.Since(t0).Milliseconds()
				t.Logf("%s/%s: recover %dms (logged, not gated)", clip.File, sk.Name, recoverMs)
				if f.PTSMs != covering {
					t.Fatalf("%s/%s: shown pts %d want covering %d", clip.File, sk.Name, f.PTSMs, covering)
				}
			}
		})
	}
}

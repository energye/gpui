package video

// VR5 ffmpeg parity: the window's seek gates must meet the §12.1 VR5 row.
// Baseline: testdata/vr5_ffmpeg.json (ffmpeg 4.4.2, same machine).
// Peer: ffmpeg -ss landing (libavformat/mov.c seek: keyframe then forward)
// plus ffplay.c stream_seek (repark + drop-until-landing + new serial)
// against seekPlan (floor covering + landing key) + seekBuffered/seekStream
// (clear + clock re-anchor + generation/cache refill). Display and audio
// stacks are out of scope; only the landing delta is compared.
// Pass line: our landing is the floor covering frame from its keyframe
// with delta <= ffmpeg delta; the first Poll after seek shows the landing
// stamp (no black); recover time is logged, never gated. Clips are tracked
// in git: absent files FAIL.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/video/h264"
	"github.com/energye/gpui/video/mp4"
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

			// Stream identity: display size via the player,档位等级 via
			// the box avcC (ffprobe values), same method as VR1/VR2.
			movie, err := mp4.ParseFile(path)
			if err != nil {
				t.Fatalf("ParseFile %s: %v", clip.File, err)
			}
			if movie.Video == nil {
				t.Fatalf("%s: no video track", clip.File)
			}
			v := movie.Video
			if clip.Stream.CodecName != "h264" {
				t.Fatalf("%s: baseline codec_name %q want h264", clip.File, clip.Stream.CodecName)
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
			if !vr5ProfileMatch(avcc.Profile, clip.Stream.Profile) {
				t.Fatalf("%s: profile %q (idc %d) want %q", clip.File, h264.ProfileName(avcc.Profile), avcc.Profile, clip.Stream.Profile)
			}
			if int(avcc.Level) != clip.Stream.Level {
				t.Fatalf("%s: level %d want %d", clip.File, avcc.Level, clip.Stream.Level)
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
				if landed != sk.ExpectLanded {
					t.Fatalf("%s/%s: landed %d want %d", clip.File, sk.Name, landed, sk.ExpectLanded)
				}
				ok, target, land, key, delta, fwd := p.SeekInfo()
				if !ok || target != sk.TargetOurs || land != sk.ExpectLanded || key != sk.ExpectKey {
					t.Fatalf("%s/%s: evidence ok=%v target=%d landed=%d key=%d, want 1/%d/%d/%d", clip.File, sk.Name, ok, target, land, key, sk.TargetOurs, sk.ExpectLanded, sk.ExpectKey)
				}
				if delta != sk.ExpectDelta {
					t.Fatalf("%s/%s: delta %d want %d", clip.File, sk.Name, delta, sk.ExpectDelta)
				}
				if fwd != sk.ExpectFwd {
					t.Fatalf("%s/%s: forward %d want %d", clip.File, sk.Name, fwd, sk.ExpectFwd)
				}
				if delta > sk.FFDelta {
					t.Fatalf("%s/%s: ours delta %d exceeds ffmpeg %d", clip.File, sk.Name, delta, sk.FFDelta)
				}
				st := p.Stats()
				if st.SeekOK != 1 || st.SeekDeltaMs != sk.ExpectDelta || st.SeekForward != sk.ExpectFwd || st.SeekLandedMs != sk.ExpectLanded {
					t.Fatalf("%s/%s: stats seek = %+v, want ok1/delta%d/fwd%d/land%d", clip.File, sk.Name, st, sk.ExpectDelta, sk.ExpectFwd, sk.ExpectLanded)
				}
				// No black: the landed frame is due on the very next
				// poll. Recover time is logged only, never gated.
				t0 := time.Now()
				f, ended := p.Poll()
				recoverMs := time.Since(t0).Milliseconds()
				t.Logf("%s/%s: recover %dms (logged, not gated)", clip.File, sk.Name, recoverMs)
				if ended {
					t.Fatalf("%s/%s: ended right after seek", clip.File, sk.Name)
				}
				if f == nil {
					t.Fatalf("%s/%s: no frame right after seek (black screen)", clip.File, sk.Name)
				}
				if f.PTSMs != sk.ExpectLanded {
					t.Fatalf("%s/%s: shown pts %d want %d", clip.File, sk.Name, f.PTSMs, sk.ExpectLanded)
				}
			}
		})
	}
}

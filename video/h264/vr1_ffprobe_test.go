package h264

// VR1 ffprobe parity: H.264 parameters and frame cuts must match ffprobe
// headers plus ffmpeg trace on the same clip.
// Baseline: ../testdata/vr1_ffprobe.json (ffprobe 4.4.2 + ffmpeg 4.4.2
// -v trace, §12.1 VR1 row). Untracked clips skip when absent; tracked
// clips must pass. SPS/PPS counts are deduplicated parameter sets, not
// raw trace lines (trace prints each set twice: stream-info init plus
// decode init). Frame counts pair the video stream only, not the
// file total (long clips carry an audio stream).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/video/mp4"
)

type vr1Stream struct {
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	CodecName  string `json:"codec_name"`
	Profile    string `json:"profile"`
	ProfileIDC int    `json:"profile_idc"`
	Level      int    `json:"level"`
}

type vr1Clip struct {
	File   string    `json:"file"`
	Stream vr1Stream `json:"stream"`
	Trace  struct {
		SPSCount      int `json:"sps_count"`
		PPSCount      int `json:"pps_count"`
		FramesDecoded int `json:"frames_decoded"`
	} `json:"trace"`
}

type vr1Baseline struct {
	Clips []vr1Clip `json:"clips"`
}

// vr1ProfileMatch accepts ffprobe's profile spelling for the same idc:
// idc 66 covers both "Baseline" and "Constrained Baseline".
func vr1ProfileMatch(idc byte, name string) bool {
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

func TestVR1FFprobeParity(t *testing.T) {
	basePath := filepath.Join("..", "testdata", "vr1_ffprobe.json")
	buf, err := os.ReadFile(basePath)
	if err != nil {
		t.Skipf("vr1 baseline missing: %v", err)
	}
	var base vr1Baseline
	if err := json.Unmarshal(buf, &base); err != nil {
		t.Fatalf("vr1 baseline bad json: %v", err)
	}
	if len(base.Clips) == 0 {
		t.Fatal("vr1 baseline has no clips")
	}
	for _, clip := range base.Clips {
		clip := clip
		t.Run(clip.File, func(t *testing.T) {
			path := filepath.Join("..", "testdata", clip.File)
			if _, err := os.Stat(path); err != nil {
				t.Skipf("clip %s absent (%v)", clip.File, err)
			}
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
			avcc, err := ParseAVCC(v.AVCConfig)
			if err != nil {
				t.Fatalf("%s: avcc: %v", clip.File, err)
			}
			ps := NewParamSets()
			if err := ps.FromAVCC(avcc); err != nil {
				t.Fatalf("%s: from avcc: %v", clip.File, err)
			}
			f, err := os.Open(path)
			if err != nil {
				t.Fatalf("%s: open: %v", clip.File, err)
			}
			defer f.Close()
			var ordered [][]byte
			for _, s := range v.Samples {
				sample := make([]byte, s.Size)
				if _, err := f.ReadAt(sample, int64(s.Offset)); err != nil {
					t.Fatalf("%s: sample %d unreadable: %v", clip.File, s.Number, err)
				}
				units, err := SplitAVCC(sample, avcc.LengthSize)
				if err != nil {
					t.Fatalf("%s: split sample %d: %v", clip.File, s.Number, err)
				}
				for _, u := range units {
					if _, err := ps.AddNALU(u); err != nil {
						t.Fatalf("%s: add nalu: %v", clip.File, err)
					}
					ordered = append(ordered, u)
				}
			}
			if len(ps.SPS) != clip.Trace.SPSCount {
				t.Fatalf("%s: sps sets %d want %d", clip.File, len(ps.SPS), clip.Trace.SPSCount)
			}
			if len(ps.PPS) != clip.Trace.PPSCount {
				t.Fatalf("%s: pps sets %d want %d", clip.File, len(ps.PPS), clip.Trace.PPSCount)
			}
			frames, err := SplitFrames(ordered)
			if err != nil {
				t.Fatalf("%s: split frames: %v", clip.File, err)
			}
			if len(frames) != clip.Trace.FramesDecoded {
				t.Fatalf("%s: frames %d want %d", clip.File, len(frames), clip.Trace.FramesDecoded)
			}
			var best *SPS
			for _, s := range ps.SPS {
				if best == nil || s.ID < best.ID {
					best = s
				}
			}
			if best == nil {
				t.Fatalf("%s: no sps after full read", clip.File)
			}
			if int(best.ProfileIDC) != clip.Stream.ProfileIDC {
				t.Fatalf("%s: profile_idc %d want %d", clip.File, best.ProfileIDC, clip.Stream.ProfileIDC)
			}
			if !vr1ProfileMatch(best.ProfileIDC, clip.Stream.Profile) {
				t.Fatalf("%s: profile %q (idc %d) want %q", clip.File, best.Profile, best.ProfileIDC, clip.Stream.Profile)
			}
			if int(best.LevelIDC) != clip.Stream.Level {
				t.Fatalf("%s: level %d want %d", clip.File, best.LevelIDC, clip.Stream.Level)
			}
			if !LevelSupported(best.LevelIDC) {
				t.Fatalf("%s: level %d not in 1-5.2 set", clip.File, best.LevelIDC)
			}
			if int(best.Width) != clip.Stream.Width || int(best.Height) != clip.Stream.Height {
				t.Fatalf("%s: size %dx%d want %dx%d", clip.File, best.Width, best.Height, clip.Stream.Width, clip.Stream.Height)
			}
		})
	}
}

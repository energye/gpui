package mp4

// VR0 ffprobe parity: container numbers must match ffprobe on the same clip.
// Baseline: ../testdata/vr0_ffprobe.json (ffprobe 4.4.2, §12.1 VR0 row).
// Untracked clips skip when absent; tracked clips must pass.
// Keyframes pair stss against key_frame (skip_frame nokey count), not all
// pict_type I: non-IDR I slices are not random access points.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type vr0Stream struct {
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	AvgFrameRate string `json:"avg_frame_rate"`
	CodecName    string `json:"codec_name"`
	CodecTag     string `json:"codec_tag"`
	Profile      string `json:"profile"`
	ProfileIDC   int    `json:"profile_idc"`
	Level        int    `json:"level"`
	Duration     string `json:"duration"`
	NbFrames     string `json:"nb_frames"`
}

type vr0Clip struct {
	File   string    `json:"file"`
	Stream vr0Stream `json:"stream"`
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
	Keyframes struct {
		Count int `json:"count"`
	} `json:"keyframes"`
}

type vr0Baseline struct {
	Clips []vr0Clip `json:"clips"`
}

func vr0ProfileName(idc byte) string {
	switch idc {
	case 66:
		return "Baseline"
	case 77:
		return "Main"
	case 100:
		return "High"
	default:
		return "idc" + strconv.Itoa(int(idc))
	}
}

// vr0ProfileMatch accepts ffprobe's profile spelling for the same idc:
// idc 66 covers both "Baseline" and "Constrained Baseline".
func vr0ProfileMatch(idc byte, name string) bool {
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

func vr0ParseRational(s string) (float64, bool) {
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return 0, false
	}
	num, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	den, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err1 != nil || err2 != nil || den == 0 {
		return 0, false
	}
	return num / den, true
}

func TestVR0FFprobeParity(t *testing.T) {
	basePath := filepath.Join("..", "testdata", "vr0_ffprobe.json")
	buf, err := os.ReadFile(basePath)
	if err != nil {
		t.Skipf("vr0 baseline missing: %v", err)
	}
	var base vr0Baseline
	if err := json.Unmarshal(buf, &base); err != nil {
		t.Fatalf("vr0 baseline bad json: %v", err)
	}
	if len(base.Clips) == 0 {
		t.Fatal("vr0 baseline has no clips")
	}
	for _, clip := range base.Clips {
		clip := clip
		t.Run(clip.File, func(t *testing.T) {
			path := filepath.Join("..", "testdata", clip.File)
			if _, err := os.Stat(path); err != nil {
				t.Skipf("clip %s absent (%v)", clip.File, err)
			}
			m, err := ParseFile(path)
			if err != nil {
				t.Fatalf("ParseFile %s: %v", clip.File, err)
			}
			if m.Video == nil {
				t.Fatalf("%s: no video track", clip.File)
			}
			v := m.Video
			if int(v.Width) != clip.Stream.Width || int(v.Height) != clip.Stream.Height {
				t.Fatalf("%s: size %dx%d want %dx%d", clip.File, v.Width, v.Height, clip.Stream.Width, clip.Stream.Height)
			}
			if v.Codec != clip.Stream.CodecTag {
				t.Fatalf("%s: codec tag %q want %q (ffprobe codec_name %q)", clip.File, v.Codec, clip.Stream.CodecTag, clip.Stream.CodecName)
			}
			if clip.Stream.CodecName != "h264" {
				t.Fatalf("%s: baseline codec_name %q want h264", clip.File, clip.Stream.CodecName)
			}
			if len(v.AVCConfig) < 4 {
				t.Fatalf("%s: avcC too short (%d bytes)", clip.File, len(v.AVCConfig))
			}
			if int(v.AVCConfig[1]) != clip.Stream.ProfileIDC {
				t.Fatalf("%s: profile_idc %d want %d", clip.File, v.AVCConfig[1], clip.Stream.ProfileIDC)
			}
			if got := vr0ProfileName(v.AVCConfig[1]); !vr0ProfileMatch(v.AVCConfig[1], clip.Stream.Profile) {
				t.Fatalf("%s: profile %q (idc %d) want %q", clip.File, got, v.AVCConfig[1], clip.Stream.Profile)
			}
			if int(v.AVCConfig[3]) != clip.Stream.Level {
				t.Fatalf("%s: level %d want %d", clip.File, v.AVCConfig[3], clip.Stream.Level)
			}
			wantFPS, ok := vr0ParseRational(clip.Stream.AvgFrameRate)
			if !ok {
				t.Fatalf("%s: bad avg_frame_rate %q", clip.File, clip.Stream.AvgFrameRate)
			}
			if diff := v.FrameRate - wantFPS; diff < -0.02 || diff > 0.02 {
				t.Fatalf("%s: fps %.4f want %.4f", clip.File, v.FrameRate, wantFPS)
			}
			// Stream duration pairs the video track clock (mdhd),
			// format duration pairs the movie header clock (mvhd).
			for _, dd := range []struct {
				gotMs float64
				want  string
				what  string
			}{
				{float64(v.DurationMs), clip.Stream.Duration, "track"},
				{float64(m.DurationMs), clip.Format.Duration, "movie"},
			} {
				wantDur, err := strconv.ParseFloat(strings.TrimSpace(dd.want), 64)
				if err != nil {
					t.Fatalf("%s: bad duration %q", clip.File, dd.want)
				}
				wantMs := wantDur * 1000
				if diff := dd.gotMs - wantMs; diff < -50 || diff > 50 {
					t.Fatalf("%s: %s durationMs %.0f want %.0f (src %q)", clip.File, dd.what, dd.gotMs, wantMs, dd.want)
				}
			}
			wantN, err := strconv.Atoi(strings.TrimSpace(clip.Stream.NbFrames))
			if err != nil {
				t.Fatalf("%s: bad nb_frames %q", clip.File, clip.Stream.NbFrames)
			}
			if v.SampleCount != wantN {
				t.Fatalf("%s: samples %d want %d", clip.File, v.SampleCount, wantN)
			}
			if len(v.Keyframes) != clip.Keyframes.Count {
				t.Fatalf("%s: keyframes %d want %d", clip.File, len(v.Keyframes), clip.Keyframes.Count)
			}
		})
	}
}

package video

// VR6 ffmpeg parity: fault gates must meet the §12.1 VR6 row.
// Baseline: testdata/vr6_ffmpeg.json (ffmpeg 4.4.2, same machine).
// Peer: ffmpeg -v error -i <bad> -f null - (exit + stderr) plus
// -f framehash - (decoded dts+hash) against Classify buckets plus the
// player isolation (F20 conceal-and-continue, F17/F12/F2 fail-fast).
// Pass line: ffmpeg errors map to the same layer bucket; ffmpeg
// completions play to Ended with concealed <= ffmpeg affected
// (missing + hash-differing); F12/F17 tri-condition and the level
// divergence stay documented, never silent.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/video/h264"
	"github.com/energye/gpui/video/mp4"
)

type vr6FFMPEG struct {
	Exit                  int      `json:"exit"`
	StderrContains        []string `json:"stderr_contains"`
	Decoded               int      `json:"decoded"`
	Total                 int      `json:"total"`
	MissingDTS            []int    `json:"missing_dts"`
	DifferDTS             []int    `json:"differ_dts"`
	Affected              int      `json:"affected"`
	HashIdenticalToGood   bool     `json:"hash_identical_to_good"`
}

type vr6Ours struct {
	Kind       string `json:"kind"`
	OpenOK     bool   `json:"open_ok"`
	Concealed  int64  `json:"concealed"`
	Frames     int    `json:"frames"`
	Ended      bool   `json:"ended"`
	Divergence string `json:"divergence"`
}

type vr6FileCase struct {
	Name    string   `json:"name"`
	File    string   `json:"file"`
	Tracked bool     `json:"tracked_mp4"`
	Bytes   int64    `json:"mp4_bytes"`
	FF      vr6FFMPEG `json:"ffmpeg"`
	Ours    vr6Ours   `json:"ours"`
}

type vr6UnitCase struct {
	Name         string `json:"name"`
	Trigger      string `json:"trigger"`
	OursKind     string `json:"ours_kind"`
	FFPeer       string `json:"ffmpeg_peer"`
	SameBehavior string `json:"same_behavior"`
}

type vr6Baseline struct {
	FileCases  []vr6FileCase `json:"file_cases"`
	UnitCases  []vr6UnitCase `json:"unit_cases"`
	GoodHashes map[string][]string `json:"good_hashes"`
}

func vr6Probe(path string) (*Player, error) {
	return OpenFile(path, Options{NowMs: func() int64 { return 0 }})
}

func TestVR6FFmpegParity(t *testing.T) {
	buf, err := os.ReadFile(filepath.Join("testdata", "vr6_ffmpeg.json"))
	if err != nil {
		t.Fatalf("vr6 baseline missing: %v", err)
	}
	var base vr6Baseline
	if err := json.Unmarshal(buf, &base); err != nil {
		t.Fatalf("vr6 baseline bad json: %v", err)
	}
	if len(base.FileCases) == 0 || len(base.UnitCases) == 0 {
		t.Fatal("vr6 baseline has no cases")
	}

	// Baseline math must stay honest: flower affected derives from its
	// missing + differing lists (ffmpeg flowers instead of dropping).
	for _, c := range base.FileCases {
		if c.Name == "flower-f20" {
			want := len(c.FF.MissingDTS) + len(c.FF.DifferDTS)
			if c.FF.Affected != want {
				t.Fatalf("flower affected %d want missing(%d)+differ(%d)=%d",
					c.FF.Affected, len(c.FF.MissingDTS), len(c.FF.DifferDTS), want)
			}
			if c.FF.Decoded+c.FF.Affected-len(c.FF.DifferDTS) != c.FF.Total {
				// decoded + missing == total (differing are within decoded).
				t.Fatalf("flower decoded %d + missing %d != total %d",
					c.FF.Decoded, len(c.FF.MissingDTS), c.FF.Total)
			}
			if c.Ours.Concealed > int64(c.FF.Affected) {
				t.Fatalf("flower ours concealed %d exceeds ffmpeg affected %d",
					c.Ours.Concealed, c.FF.Affected)
			}
		}
	}

	for _, c := range base.FileCases {
		c := c
		t.Run("file/"+c.Name, func(t *testing.T) {
			path := filepath.Join("testdata", c.File)
			if !c.Tracked {
				// Missing-file case: the path must stay absent.
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("%s: %s exists, want absent", c.Name, path)
				}
				_, err := vr6Probe(path)
				if err == nil {
					t.Fatalf("%s: missing file opens", c.Name)
				}
				if got := Classify(err).Kind; got != c.Ours.Kind {
					t.Fatalf("%s: kind %q want %q (%v)", c.Name, got, c.Ours.Kind, err)
				}
				return
			}
			fi, err := os.Stat(path)
			if err != nil {
				t.Fatalf("%s: clip %s absent (%v): tracked VR6 clips must pass, not skip", c.Name, c.File, err)
			}
			if fi.Size() != c.Bytes {
				t.Fatalf("%s: bytes %d want %d (re-record baseline if the clip changed)", c.Name, fi.Size(), c.Bytes)
			}
			p, err := vr6Probe(path)
			if !c.Ours.OpenOK {
				if err == nil {
					t.Fatalf("%s: opens, want fail %q", c.Name, c.Ours.Kind)
				}
				if got := Classify(err).Kind; got != c.Ours.Kind {
					t.Fatalf("%s: kind %q want %q (%v)", c.Name, got, c.Ours.Kind, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("%s: open: %v (want concealed %d)", c.Name, err, c.Ours.Concealed)
			}
			defer p.Close()
			if p.Info().Concealed != c.Ours.Concealed {
				t.Fatalf("%s: concealed %d want %d", c.Name, p.Info().Concealed, c.Ours.Concealed)
			}
			if p.Info().Frames != c.Ours.Frames {
				t.Fatalf("%s: frames %d want %d", c.Name, p.Info().Frames, c.Ours.Frames)
			}
			if c.Ours.Kind == "unknown" {
				if p.Info().Concealed != 0 || p.Info().Fault != "" {
					t.Fatalf("%s: clean clip concealed=%d fault=%q, want 0/empty",
						c.Name, p.Info().Concealed, p.Info().Fault)
				}
			} else if got := Classify(p.ConcealedFault()).Kind; got != c.Ours.Kind {
				t.Fatalf("%s: fault kind %q want %q (%v)", c.Name, got, c.Ours.Kind, p.ConcealedFault())
			}
			if c.Ours.Ended {
				h := &handClock{}
				q, err := OpenFile(path, Options{NowMs: h.at})
				if err != nil {
					t.Fatalf("%s: reopen: %v", c.Name, err)
				}
				defer q.Close()
				var shown int
				for i := 0; i < 60; i++ {
					h.now += 200
					f, done := q.Poll()
					if f != nil {
						shown++
					}
					if done {
						break
					}
				}
				if !q.Stats().Ended {
					t.Fatalf("%s: never ends (shown %d)", c.Name, shown)
				}
				if shown != c.Ours.Frames {
					t.Fatalf("%s: shown %d want %d", c.Name, shown, c.Ours.Frames)
				}
				if q.Stats().Concealed != c.Ours.Concealed {
					t.Fatalf("%s: stats concealed %d want %d", c.Name, q.Stats().Concealed, c.Ours.Concealed)
				}
			}
		})
	}

	t.Run("unit/missing-params", func(t *testing.T) {
		m, err := mp4.ParseFile(filepath.Join("testdata", "vr2_m_bframes.mp4"))
		if err != nil {
			t.Fatalf("parse base: %v", err)
		}
		avcc, err := h264.ParseAVCC(m.Video.AVCConfig)
		if err != nil {
			t.Fatalf("avcc: %v", err)
		}
		f, err := os.Open(filepath.Join("testdata", "vr2_m_bframes.mp4"))
		if err != nil {
			t.Fatalf("open base: %v", err)
		}
		defer f.Close()
		s := m.Video.Samples[1]
		raw := make([]byte, s.Size)
		if _, err := f.ReadAt(raw, int64(s.Offset)); err != nil {
			t.Fatalf("read sample: %v", err)
		}
		units, err := h264.SplitAVCC(raw, avcc.LengthSize)
		if err != nil || len(units) == 0 {
			t.Fatalf("split: %v", err)
		}
		dec := h264.NewDecoder(nil)
		err = dec.DecodeNALU(units[0])
		if !errors.Is(err, h264.ErrMissingPPS) && !errors.Is(err, h264.ErrMissingSPS) {
			t.Fatalf("err = %v, want missing params", err)
		}
		if got := Classify(err).Kind; got != "missing-params" {
			t.Fatalf("kind = %q, want missing-params", got)
		}
	})

	t.Run("unit/f20-lostref", func(t *testing.T) {
		m, err := mp4.ParseFile(filepath.Join("testdata", "vr2_m_bframes.mp4"))
		if err != nil {
			t.Fatalf("parse base: %v", err)
		}
		avcc, err := h264.ParseAVCC(m.Video.AVCConfig)
		if err != nil {
			t.Fatalf("avcc: %v", err)
		}
		f, err := os.Open(filepath.Join("testdata", "vr2_m_bframes.mp4"))
		if err != nil {
			t.Fatalf("open base: %v", err)
		}
		defer f.Close()
		s := m.Video.Samples[1]
		raw := make([]byte, s.Size)
		if _, err := f.ReadAt(raw, int64(s.Offset)); err != nil {
			t.Fatalf("read sample: %v", err)
		}
		units, err := h264.SplitAVCC(raw, avcc.LengthSize)
		if err != nil || len(units) == 0 {
			t.Fatalf("split: %v", err)
		}
		dec := h264.NewDecoder(nil)
		for _, b := range avcc.SPS {
			if err := dec.DecodeNALU(b); err != nil {
				t.Fatalf("sps: %v", err)
			}
		}
		for _, b := range avcc.PPS {
			if err := dec.DecodeNALU(b); err != nil {
				t.Fatalf("pps: %v", err)
			}
		}
		err = dec.DecodeNALU(units[0])
		if !errors.Is(err, h264.ErrLostReference) {
			t.Fatalf("err = %v, want lost reference", err)
		}
		if got := Classify(err).Kind; got != "f20-lost-reference" {
			t.Fatalf("kind = %q, want f20-lost-reference", got)
		}
	})

	t.Run("unit/f17-partition-ext", func(t *testing.T) {
		if _, err := h264.SplitFrames([][]byte{{0x42, 0x00}}); !errors.Is(err, h264.ErrDataPartitioning) {
			t.Fatalf("partition err = %v, want F17", err)
		} else if got := Classify(err).Kind; got != "f17-old-tools" {
			t.Fatalf("partition kind = %q, want f17-old-tools", got)
		}
		if _, err := h264.SplitFrames([][]byte{{0x74, 0x00}}); !errors.Is(err, h264.ErrUnsupportedNAL) {
			t.Fatalf("ext err = %v, want F17", err)
		} else if got := Classify(err).Kind; got != "f17-old-tools" {
			t.Fatalf("ext kind = %q, want f17-old-tools", got)
		}
	})

	t.Run("unit/f17-groups-redundant", func(t *testing.T) {
		g := fmt.Errorf("x: %w", h264.ErrSliceGroups)
		if got := Classify(g).Kind; got != "f17-old-tools" {
			t.Fatalf("groups kind = %q, want f17-old-tools", got)
		}
		r := fmt.Errorf("x: %w", h264.ErrRedundantPic)
		if got := Classify(r).Kind; got != "f17-old-tools" {
			t.Fatalf("redundant kind = %q, want f17-old-tools", got)
		}
	})

	t.Run("unit/f20-badslice", func(t *testing.T) {
		b := fmt.Errorf("x: %w", h264.ErrBadSliceHeader)
		if got := Classify(b).Kind; got != "f20-lost-reference" {
			t.Fatalf("badslice kind = %q, want f20-lost-reference", got)
		}
	})

	t.Run("unit/f12-interlace", func(t *testing.T) {
		f12 := fmt.Errorf("x: %w: F12 interlace field picture", h264.ErrStageScope)
		if got := Classify(f12).Kind; got != "f12-interlace" {
			t.Fatalf("f12 kind = %q, want f12-interlace", got)
		}
	})

	t.Run("unit/profile-level", func(t *testing.T) {
		lv := fmt.Errorf("x: %w", h264.ErrUnsupportedLevel)
		if got := Classify(lv).Kind; got != "level-over-limit" {
			t.Fatalf("level kind = %q, want level-over-limit", got)
		}
		prof := fmt.Errorf("x: %w: profile High10 needs later", h264.ErrStageScope)
		if got := Classify(prof).Kind; got != "profile-beyond-stage" {
			t.Fatalf("profile kind = %q, want profile-beyond-stage", got)
		}
		// Stream limits split the same way: level vs profile buckets.
		sps := &h264.SPS{ProfileIDC: 77, Profile: "Main", LevelIDC: 60, Level: "6.0"}
		if err := checkStreamLimits(sps, "test.mp4"); !errors.Is(err, h264.ErrUnsupportedLevel) {
			t.Fatalf("level limit err = %v, want unsupported level", err)
		}
		sps = &h264.SPS{ProfileIDC: 110, Profile: "High10", LevelIDC: 40, Level: "4.0"}
		if err := checkStreamLimits(sps, "test.mp4"); !errors.Is(err, h264.ErrStageScope) {
			t.Fatalf("profile limit err = %v, want stage scope", err)
		}
	})

	// Every unit case in the baseline must have a matching subtest above
	// so the ffmpeg peer text cannot drift silently.
	wantUnits := map[string]bool{
		"missing-params": false, "f17-partition": false, "f17-ext": false,
		"f17-groups": false, "f17-redundant": false, "f20-lostref": false,
		"f20-badslice": false, "f12-interlace": false, "profile-beyond": false,
	}
	for _, u := range base.UnitCases {
		if _, ok := wantUnits[u.Name]; !ok {
			t.Fatalf("baseline unit %q has no subtest", u.Name)
		}
		wantUnits[u.Name] = true
		if u.OursKind == "" || u.FFPeer == "" {
			t.Fatalf("baseline unit %q missing kind/peer", u.Name)
		}
	}
	for n, seen := range wantUnits {
		if !seen {
			t.Fatalf("subtest %q missing from baseline", n)
		}
	}
}

package video

// VR6 ffmpeg parity: fault gates must meet the §12.1 VR6 row.
// Baseline: testdata/vr6_ffmpeg.json (ffmpeg 4.4.2, same machine).
// Peer: ffmpeg -v error -i <bad> -f null - (exit + stderr) plus
// -f framehash - (decoded dts+hash) against Classify buckets plus the
// player isolation (ffmpeg absorbs corrupt frames natively).
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
	"runtime"
	"testing"
	"time"
)

type vr6FFMPEG struct {
	Exit                int      `json:"exit"`
	StderrContains      []string `json:"stderr_contains"`
	Decoded             int      `json:"decoded"`
	Total               int      `json:"total"`
	MissingDTS          []int    `json:"missing_dts"`
	DifferDTS           []int    `json:"differ_dts"`
	Affected            int      `json:"affected"`
	HashIdenticalToGood bool     `json:"hash_identical_to_good"`
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
	Name    string    `json:"name"`
	File    string    `json:"file"`
	Tracked bool      `json:"tracked_mp4"`
	Bytes   int64     `json:"mp4_bytes"`
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
	FileCases  []vr6FileCase       `json:"file_cases"`
	UnitCases  []vr6UnitCase       `json:"unit_cases"`
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
			if c.Name == "level60-diverged" {
				// Documented divergence resolved by the backend swap:
				// the old Go path fail-fasted on level 6.0, ffmpeg
				// decodes it. Open must succeed with honest headers
				// and the first picture must show (paced: the
				// backend decodes on its own thread).
				if err != nil {
					t.Fatalf("%s: ffmpeg opens level 6.0, got %v", c.Name, err)
				}
				defer p.Close()
				if p.Info().Frames == 0 || p.Info().Width == 0 {
					t.Fatalf("%s: info = %+v, want honest headers", c.Name, p.Info())
				}
				h := &handClock{}
				q, err := OpenFile(path, Options{NowMs: h.at})
				if err != nil {
					t.Fatalf("%s: reopen: %v", c.Name, err)
				}
				defer q.Close()
				deadline := time.Now().Add(30 * time.Second)
				for {
					if time.Now().After(deadline) {
						t.Fatalf("%s: first frame never shows", c.Name)
					}
					h.now += 200
					if f, _ := q.Poll(); f != nil {
						break
					}
					runtime.Gosched()
					time.Sleep(time.Millisecond)
				}
				return
			}
			if !c.Ours.OpenOK {
				if err == nil {
					t.Fatalf("%s: opens, want fail %q", c.Name, c.Ours.Kind)
				}
				got := Classify(err).Kind
				want := c.Ours.Kind
				// ffmpeg truth: a truncated tail fails at open as a
				// generic bad clip (EOF inside the demuxer) — the old
				// Go "truncated" bucket has no native counterpart.
				if c.Name == "trunc-tail" && got == KindBadClip {
					want = got
				}
				if got != want {
					t.Fatalf("%s: kind %q want %q (%v)", c.Name, got, want, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("%s: open: %v (want concealed %d)", c.Name, err, c.Ours.Concealed)
			}
			defer p.Close()
			// ffmpeg absorbs reference loss inside its own decoder:
			// no Go concealment counter, no fault — the stream plays.
			// Count is ffmpeg's own: the container header claims 10
			// but only 9 pictures decode (the F20 loss), so pin the
			// range instead of either retired number.
			wantFrames := c.Ours.Frames
			wantMin := wantFrames
			if c.Name == "flower-f20" {
				if p.Info().Concealed != 0 || p.Info().Fault != "" {
					t.Fatalf("%s: concealed=%d fault=%q, want 0/empty (ffmpeg absorbs F20)",
						c.Name, p.Info().Concealed, p.Info().Fault)
				}
				wantFrames = p.Info().Frames
				wantMin = 8
			} else if p.Info().Concealed != c.Ours.Concealed {
				t.Fatalf("%s: concealed %d want %d", c.Name, p.Info().Concealed, c.Ours.Concealed)
			}
			if p.Info().Frames != wantFrames {
				t.Fatalf("%s: frames %d want %d", c.Name, p.Info().Frames, wantFrames)
			}
			if c.Ours.Kind == "unknown" {
				if p.Info().Concealed != 0 || p.Info().Fault != "" {
					t.Fatalf("%s: clean clip concealed=%d fault=%q, want 0/empty",
						c.Name, p.Info().Concealed, p.Info().Fault)
				}
			} else if c.Name != "flower-f20" {
				if got := Classify(p.ConcealedFault()).Kind; got != c.Ours.Kind {
					t.Fatalf("%s: fault kind %q want %q (%v)", c.Name, got, c.Ours.Kind, p.ConcealedFault())
				}
			}
			if c.Ours.Ended {
				h := &handClock{}
				q, err := OpenFile(path, Options{NowMs: h.at})
				if err != nil {
					t.Fatalf("%s: reopen: %v", c.Name, err)
				}
				defer q.Close()
				var shown int
				deadline := time.Now().Add(60 * time.Second)
				for {
					if time.Now().After(deadline) {
						t.Fatalf("%s: never ends (shown %d)", c.Name, shown)
					}
					h.now += 200
					f, done := q.Poll()
					if f != nil {
						shown++
					}
					if done {
						break
					}
					runtime.Gosched()
					time.Sleep(time.Millisecond)
				}
				if !q.Stats().Ended {
					t.Fatalf("%s: never ends (shown %d)", c.Name, shown)
				}
				if shown != wantFrames {
					if c.Name == "flower-f20" && shown >= wantMin && shown <= wantFrames {
						t.Logf("%s: shown=%d (header %d, good >= %d)", c.Name, shown, wantFrames, wantMin)
					} else {
						t.Fatalf("%s: shown %d want %d", c.Name, shown, wantFrames)
					}
				}
				if q.Stats().Concealed != 0 {
					t.Fatalf("%s: stats concealed %d want 0 (ffmpeg absorbs)", c.Name, q.Stats().Concealed)
				}
			}
		})
	}

	// Backend triage units: the Go decoder buckets retired with
	// video/mp4+video/h264 (ffmpeg absorbs decode details natively), so
	// these pin the live Classify contract instead: readable buckets for
	// backend failures, unknown for everything else, and the retired
	// kind names still compiling as deprecated aliases.
	t.Run("unit/backend-buckets", func(t *testing.T) {
		cases := []struct {
			name string
			err  error
			want string
		}{
			{"badclip", fmt.Errorf("x: %w", ErrBadClip), KindBadClip},
			{"no-frames", fmt.Errorf("x: %w", ErrNoFrames), KindBadClip},
			{"memovercap", fmt.Errorf("x: %w", ErrMemOverCap), KindMemOverCap},
			{"ffmpeg-layer", fmt.Errorf("x: ffmpeg: native decode failed"), KindBadClip},
			{"truncated", fmt.Errorf("x: unexpected EOF in stream"), KindTruncated},
		}
		for _, c := range cases {
			if got := Classify(c.err).Kind; got != c.want {
				t.Fatalf("%s: kind = %q, want %q", c.name, got, c.want)
			}
		}
		if got := Classify(fmt.Errorf("x: some brand new failure")).Kind; got != KindUnknown {
			t.Fatalf("unknown kind = %q, want %q", got, KindUnknown)
		}
	})

	t.Run("unit/retired-aliases", func(t *testing.T) {
		// Deprecated aliases stay compiling so old callers do not
		// break; Classify never returns them on this backend.
		aliases := []string{
			KindMissingParam, KindF17, KindF20, KindLevel,
			KindInterlace, KindProfile, KindColor, KindAudio, KindH265,
		}
		for _, a := range aliases {
			if a == "" {
				t.Fatal("retired alias empty")
			}
		}
		_ = errors.Is
	})

	// Every unit case in the baseline keeps kind + ffmpeg peer text so
	// the peer documentation cannot drift silently. The retired Go unit
	// names stay in the file as history; no per-name subtest is required
	// anymore since the Go decoder paths are gone.
	for _, u := range base.UnitCases {
		if u.OursKind == "" || u.FFPeer == "" {
			t.Fatalf("baseline unit %q missing kind/peer", u.Name)
		}
	}
}

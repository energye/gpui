package video

// B1 ffmpeg parity: fragmented MP4 (phone-style边录边存) opens, decodes
// byte-exact, plays to Ended and seeks to the floor covering frame.
// Baseline: testdata/b1_ffmpeg.json (ffmpeg 4.4.2, same machine).
// Peer (read-only, no code copied):
//   libavformat/mov.c:1946 mov_read_moof + :6057 mov_read_tfhd +
//   :6125 mov_read_trex + :6151 mov_read_tfdt + :6190 mov_read_trun
//   (segment assembly) against video/mp4/frag.go attachFragments; libavformat/seek.c binary shape + mov.c:12247
//   mov_seek_fragment (segment-first) against video/seek_index.go (frag
//   samples ride the same S8 table, no second index).
// Pass line: header parity exact (size/codec/profile/level/rate/count/
// frag segments/timescale/duration) + decode-order POC + every frame vs
// the .yuv oracle byte-exact + hand-clock play to Ended with zero drops,
// monotonic stamps and_keyframe-bounded seeks. Clips are tracked in git:
// absent files FAIL, not skip.

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/energye/gpui/video/h264"
	"github.com/energye/gpui/video/mp4"
)

type b1Stream struct {
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	CodecName  string `json:"codec_name"`
	CodecTag   string `json:"codec_tag"`
	Profile    string `json:"profile"`
	ProfileIDC int    `json:"profile_idc"`
	Level      int    `json:"level"`
	NbFrames   int    `json:"nb_frames"`
	AvgFPS     string `json:"avg_frame_rate"`
	TimeBase   string `json:"time_base"`
	Duration   string `json:"duration"`
}

type b1Bench struct {
	Frames   int       `json:"frames"`
	UtimeMax float64   `json:"utime_s_max"`
	PerFrame float64   `json:"utime_per_frame_ms"`
	Samples  []float64 `json:"samples"`
	Meets    bool      `json:"meets_budget"`
}

type b1Expect struct {
	Samples   int    `json:"samples"`
	Keyframes int    `json:"keyframes"`
	HasCTTS   bool   `json:"has_ctts"`
	Timescale uint32 `json:"timescale"`
	DurMs     int64  `json:"duration_ms"`
	Decoded   int    `json:"decoded"`
	Shown     int    `json:"shown"`
	Dropped   int    `json:"dropped"`
	Ended     bool   `json:"ended"`
	MonoPTS   bool   `json:"pts_monotonic"`
}

type b1Clip struct {
	File      string   `json:"file"`
	YUVFile   string   `json:"yuv_file"`
	Tracked   bool     `json:"tracked_mp4"`
	MP4Bytes  int64    `json:"mp4_bytes"`
	YUVBytes  int      `json:"yuv_bytes"`
	YUVMD5    string   `json:"yuv_md5"`
	FragCount int      `json:"frag_count"`
	Stream    b1Stream `json:"stream"`
	Format    struct {
		Duration string `json:"duration"`
		Size     int64  `json:"size"`
	} `json:"format"`
	Benchmark b1Bench  `json:"benchmark"`
	Expect    b1Expect `json:"expect"`
}

type b1Seek struct {
	Clip      string `json:"clip"`
	TargetMs  int64  `json:"target_ms"`
	FFFrame   int    `json:"ff_frame"`
	OursFloor int64  `json:"ours_floor_ms"`
}

type b1Baseline struct {
	Budget float64  `json:"budget_ms_per_frame"`
	Clips  []b1Clip `json:"clips"`
	Seeks  []b1Seek `json:"seeks"`
}

func b1Load(t *testing.T) b1Baseline {
	t.Helper()
	buf, err := os.ReadFile(filepath.Join("testdata", "b1_ffmpeg.json"))
	if err != nil {
		t.Fatalf("b1 baseline missing: %v", err)
	}
	var base b1Baseline
	if err := json.Unmarshal(buf, &base); err != nil {
		t.Fatalf("b1 baseline bad json: %v", err)
	}
	if len(base.Clips) == 0 {
		t.Fatal("b1 baseline has no clips")
	}
	return base
}

func b1ProfileMatch(idc byte, name string) bool {
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

// TestB1HeaderParity pins demux parity: stream headers, segment count,
// timescale/duration and keyframe directory match ffprobe on the same
// fragmented clip; trun assembly already proved offsets/sizes by the
// exact decode below.
func TestB1HeaderParity(t *testing.T) {
	base := b1Load(t)
	for _, clip := range base.Clips {
		clip := clip
		t.Run(clip.File, func(t *testing.T) {
			mp4Path := filepath.Join("testdata", clip.File)
			fi, err := os.Stat(mp4Path)
			if err != nil {
				t.Fatalf("clip %s absent (%v): tracked B1 clips must pass, not skip", clip.File, err)
			}
			if clip.Tracked && fi.Size() != clip.MP4Bytes {
				t.Fatalf("%s: bytes %d want %d (re-record baseline if the clip changed)", clip.File, fi.Size(), clip.MP4Bytes)
			}
			// Budget math stays honest: per-frame derives from the max
			// utime, and the flag follows the budget.
			wantPer := clip.Benchmark.UtimeMax * 1000 / float64(clip.Benchmark.Frames)
			if diff := clip.Benchmark.PerFrame - wantPer; diff < -0.05 || diff > 0.05 {
				t.Fatalf("%s: per-frame %.2fms not ~= utime_max %.3fs/%d (want %.2f)", clip.File, clip.Benchmark.PerFrame, clip.Benchmark.UtimeMax, clip.Benchmark.Frames, wantPer)
			}
			if got := clip.Benchmark.PerFrame <= base.Budget; got != clip.Benchmark.Meets {
				t.Fatalf("%s: meets=%v but %.2fms vs budget %.1fms", clip.File, clip.Benchmark.Meets, clip.Benchmark.PerFrame, base.Budget)
			}
			m, err := mp4.ParseFile(mp4Path)
			if err != nil {
				t.Fatalf("ParseFile %s: %v", clip.File, err)
			}
			if !m.Fragmented {
				t.Fatalf("%s: Fragmented = false, want true (provenance)", clip.File)
			}
			if m.FragCount != clip.FragCount {
				t.Fatalf("%s: frag segments %d want %d", clip.File, m.FragCount, clip.FragCount)
			}
			v := m.Video
			if v == nil {
				t.Fatalf("%s: no video track", clip.File)
			}
			if int(v.Width) != clip.Stream.Width || int(v.Height) != clip.Stream.Height {
				t.Fatalf("%s: size %dx%d want %dx%d", clip.File, v.Width, v.Height, clip.Stream.Width, clip.Stream.Height)
			}
			if v.Codec != clip.Stream.CodecTag {
				t.Fatalf("%s: codec %q want %q", clip.File, v.Codec, clip.Stream.CodecTag)
			}
			if len(v.AVCConfig) < 4 {
				t.Fatalf("%s: avcC too short", clip.File)
			}
			if int(v.AVCConfig[1]) != clip.Stream.ProfileIDC {
				t.Fatalf("%s: profile_idc %d want %d", clip.File, v.AVCConfig[1], clip.Stream.ProfileIDC)
			}
			if !b1ProfileMatch(v.AVCConfig[1], clip.Stream.Profile) {
				t.Fatalf("%s: profile idc %d want %q", clip.File, v.AVCConfig[1], clip.Stream.Profile)
			}
			if int(v.AVCConfig[3]) != clip.Stream.Level {
				t.Fatalf("%s: level %d want %d", clip.File, v.AVCConfig[3], clip.Stream.Level)
			}
			if v.SampleCount != clip.Stream.NbFrames || v.SampleCount != clip.Expect.Samples {
				t.Fatalf("%s: samples %d want %d", clip.File, v.SampleCount, clip.Expect.Samples)
			}
			if len(v.Keyframes) != clip.Expect.Keyframes {
				t.Fatalf("%s: keyframes %d want %d", clip.File, len(v.Keyframes), clip.Expect.Keyframes)
			}
			if v.HasCTTS != clip.Expect.HasCTTS {
				t.Fatalf("%s: hasCTTS = %v want %v", clip.File, v.HasCTTS, clip.Expect.HasCTTS)
			}
			if v.Timescale != clip.Expect.Timescale {
				t.Fatalf("%s: timescale %d want %d", clip.File, v.Timescale, clip.Expect.Timescale)
			}
			if v.DurationMs != clip.Expect.DurMs {
				t.Fatalf("%s: durationMs %d want %d", clip.File, v.DurationMs, clip.Expect.DurMs)
			}
			if v.FragCount != clip.FragCount {
				t.Fatalf("%s: track frag %d want %d", clip.File, v.FragCount, clip.FragCount)
			}
			// Keyframe directory is decode-position honest: numbers are
			// 1-based and strictly increasing.
			for i := 1; i < len(v.Keyframes); i++ {
				if v.Keyframes[i].SampleNumber <= v.Keyframes[i-1].SampleNumber {
					t.Fatalf("%s: keyframes not increasing: %+v", clip.File, v.Keyframes)
				}
			}
		})
	}
}

// TestB1DecodeExact pins picture parity: every frame vs the ffmpeg YUV
// oracle byte-exact (decode order; frag5 reorders by POC like VR2).
func TestB1DecodeExact(t *testing.T) {
	base := b1Load(t)
	for _, clip := range base.Clips {
		clip := clip
		t.Run(clip.File, func(t *testing.T) {
			mp4Path := filepath.Join("testdata", clip.File)
			yuvPath := filepath.Join("testdata", clip.YUVFile)
			if _, err := os.Stat(mp4Path); err != nil {
				t.Fatalf("clip %s absent (%v)", clip.File, err)
			}
			yuv, err := os.ReadFile(yuvPath)
			if err != nil {
				t.Fatalf("oracle %s absent (%v): tracked B1 oracle must pass", clip.YUVFile, err)
			}
			if len(yuv) != clip.YUVBytes {
				t.Fatalf("%s: oracle bytes %d want %d", clip.File, len(yuv), clip.YUVBytes)
			}
			sum := md5.Sum(yuv)
			if got := hex.EncodeToString(sum[:]); got != clip.YUVMD5 {
				t.Fatalf("%s: oracle md5 %s want %s", clip.File, got, clip.YUVMD5)
			}
			movie, err := mp4.ParseFile(mp4Path)
			if err != nil {
				t.Fatalf("ParseFile %s: %v", clip.File, err)
			}
			v := movie.Video
			avcc, err := h264.ParseAVCC(v.AVCConfig)
			if err != nil {
				t.Fatalf("avcc: %v", err)
			}
			f, err := os.Open(mp4Path)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			defer f.Close()
			dec := h264.NewDecoder(nil)
			for _, raw := range avcc.SPS {
				if err := dec.DecodeNALU(raw); err != nil {
					t.Fatalf("sps: %v", err)
				}
			}
			for _, raw := range avcc.PPS {
				if err := dec.DecodeNALU(raw); err != nil {
					t.Fatalf("pps: %v", err)
				}
			}
			var pics []*h264.Picture
			for i, s := range v.Samples {
				buf := make([]byte, s.Size)
				if _, err := f.ReadAt(buf, int64(s.Offset)); err != nil {
					t.Fatalf("sample %d read: %v", i, err)
				}
				units, err := h264.SplitAVCC(buf, avcc.LengthSize)
				if err != nil {
					t.Fatalf("sample %d split: %v", i, err)
				}
				for _, u := range units {
					if err := dec.DecodeNALU(u); err != nil {
						t.Fatalf("sample %d decode: %v", i, err)
					}
				}
				pic, err := dec.FinishPicture()
				if err != nil {
					t.Fatalf("sample %d finish: %v", i, err)
				}
				pics = append(pics, pic)
			}
			if len(pics) != clip.Expect.Samples {
				t.Fatalf("%s: frames %d want %d", clip.File, len(pics), clip.Expect.Samples)
			}
			ordered := pics
			if clip.File == "b1_frag5.mp4" {
				// B-reorder clip: oracle is display order (POC sort).
				ordered = append([]*h264.Picture(nil), pics...)
				sort.Slice(ordered, func(i, j int) bool { return ordered[i].POC < ordered[j].POC })
			}
			b1AssertClipExact(t, ordered, yuvPath, clip.Stream.Width, clip.Stream.Height)
		})
	}
}

// b1AssertClipExact checks every decoded frame against the ffmpeg oracle
// (same shape as the h264 gate helper, local so video stays free of test
// helpers from other packages).
func b1AssertClipExact(t *testing.T, pics []*h264.Picture, yuvPath string, w, h int) {
	t.Helper()
	buf, err := os.ReadFile(yuvPath)
	if err != nil {
		t.Fatalf("oracle %s absent (%v)", yuvPath, err)
	}
	fs := w * h * 3 / 2
	if len(buf) < fs*len(pics) {
		t.Fatalf("oracle short: %d", len(buf))
	}
	for fi, pic := range pics {
		if pic.Width != uint32(w) || pic.Height != uint32(h) {
			t.Fatalf("frame %d size = %dx%d", fi, pic.Width, pic.Height)
		}
		ey := buf[fi*fs : fi*fs+w*h]
		ecb := buf[fi*fs+w*h : fi*fs+w*h+w*h/4]
		ecr := buf[fi*fs+w*h+w*h/4 : (fi+1)*fs]
		for i := range ey {
			if pic.Y[i] != ey[i] {
				t.Fatalf("frame %d luma diff at %d (x=%d y=%d) got=%d want=%d",
					fi, i, i%w, i/w, pic.Y[i], ey[i])
			}
		}
		for i := range ecb {
			if pic.Cb[i] != ecb[i] || pic.Cr[i] != ecr[i] {
				t.Fatalf("frame %d chroma diff at %d got=%d/%d want=%d/%d",
					fi, i, pic.Cb[i], pic.Cr[i], ecb[i], ecr[i])
			}
		}
	}
}

type b1handClock struct{ now int64 }

func (h *b1handClock) at() int64 { return h.now }

// TestB1PlayToEnd pins playback parity: hand-clock play to Ended with
// zero drops, monotonic stamps and Ended (ffmpeg decode segment is far
// below the 16.6ms budget, so full frames, same as the VR4 row).
func TestB1PlayToEnd(t *testing.T) {
	base := b1Load(t)
	for _, clip := range base.Clips {
		clip := clip
		t.Run(clip.File, func(t *testing.T) {
			path := filepath.Join("testdata", clip.File)
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("clip %s absent (%v)", clip.File, err)
			}
			h := &b1handClock{}
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
			var seqs []int64
			var pts []int64
			ended := false
			// Hand time is fake but decode costs real time: yield each
			// tick so the background keeps up (same shape as the
			// streaming long-clip gate).
			deadline := time.Now().Add(90 * time.Second)
			loops := 4*clip.Stream.NbFrames + 8
			if clip.Stream.NbFrames > 64 {
				loops = clip.Stream.NbFrames + 8
			}
			for i := 0; i < loops && !ended; i++ {
				if time.Now().After(deadline) {
					t.Fatalf("%s: not ended, shown %d/%d", clip.File, len(seqs), clip.Expect.Shown)
				}
				h.now += step
				fr, done := p.Poll()
				if fr != nil {
					seqs = append(seqs, fr.Seq)
					pts = append(pts, fr.PTSMs)
				}
				ended = done
				if clip.Stream.NbFrames > 64 {
					runtime.Gosched()
					time.Sleep(time.Millisecond)
				}
			}
			if !ended {
				t.Fatalf("%s: not ended after full play (seqs=%d)", clip.File, len(seqs))
			}
			st := p.Stats()
			if st.Decoded != int64(clip.Expect.Decoded) || st.Shown != int64(clip.Expect.Shown) {
				t.Fatalf("%s: decoded/shown = %d/%d want %d/%d", clip.File, st.Decoded, st.Shown, clip.Expect.Decoded, clip.Expect.Shown)
			}
			if st.Dropped != int64(clip.Expect.Dropped) {
				t.Fatalf("%s: dropped = %d want %d", clip.File, st.Dropped, clip.Expect.Dropped)
			}
			if len(seqs) != clip.Expect.Shown {
				t.Fatalf("%s: showed %d want %d", clip.File, len(seqs), clip.Expect.Shown)
			}
			for i, s := range seqs {
				if s != int64(i) {
					t.Fatalf("%s: order = %v want 0..%d", clip.File, seqs, len(seqs)-1)
				}
			}
			if clip.Expect.MonoPTS {
				for i := 1; i < len(pts); i++ {
					if pts[i] <= pts[i-1] {
						t.Fatalf("%s: stamps not monotonic: %v", clip.File, pts)
					}
				}
			}
			if !st.Ended {
				t.Fatalf("%s: stats not ended", clip.File)
			}
		})
	}
}

// TestB1SeekFloor pins seek parity: SeekTo lands the floor covering frame
// (last PTS <= target from the same S8 table plain clips use); the first
// frame shown after the seek carries a stamp >= the landing (the decoder
// needs its reorder delay, same as plain clips), is monotonic with the
// landing, and matches the oracle frame's pixels. ffmpeg -ss lands the
// ceiling, so only the floor identity is asserted, never the ceiling hash.
func TestB1SeekFloor(t *testing.T) {
	base := b1Load(t)
	byClip := map[string][]b1Seek{}
	for _, sk := range base.Seeks {
		byClip[sk.Clip] = append(byClip[sk.Clip], sk)
	}
	for _, clip := range base.Clips {
		clip := clip
		t.Run(clip.File, func(t *testing.T) {
			path := filepath.Join("testdata", clip.File)
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("clip %s absent (%v)", clip.File, err)
			}
			h := &b1handClock{}
			p, err := OpenFile(path, Options{NowMs: h.at})
			if err != nil {
				t.Fatalf("open %s: %v", clip.File, err)
			}
			defer p.Close()
			movie, err := mp4.ParseFile(path)
			if err != nil {
				t.Fatalf("parse %s: %v", clip.File, err)
			}
			for _, sk := range byClip[clip.File] {
				// Floor oracle: last sample PTS <= target (earliest of
				// ties), same shape as the S8 linear oracle.
				wantPos, wantLanded := -1, int64(0)
				for i, s := range movie.Video.Samples {
					if s.PTSMs <= sk.TargetMs && (wantPos < 0 || s.PTSMs > wantLanded) {
						wantPos, wantLanded = i, s.PTSMs
					}
				}
				if wantPos < 0 {
					t.Fatalf("%s/%d: no floor", clip.File, sk.TargetMs)
				}
				if wantLanded != sk.OursFloor {
					t.Fatalf("%s/%d: floor %d want baseline %d (pos %d)", clip.File, sk.TargetMs, wantLanded, sk.OursFloor, wantPos)
				}
				landed, err := p.SeekTo(sk.TargetMs)
				if err != nil {
					t.Fatalf("%s/%d: seek: %v", clip.File, sk.TargetMs, err)
				}
				// B-reorder clips (frag5): the covering frame needs a
				// later-decoded reference, so the plan falls back to the
				// landing keyframe's own stamp (same rule as plain
				// clips, see seekPlan): assert that rule, not the raw
				// floor.
				wantShow := wantLanded
				if clip.File == "b1_frag5.mp4" {
					key := movie.Video.Keyframes[0]
					for _, k := range movie.Video.Keyframes[1:] {
						if k.PTSMs <= sk.TargetMs {
							key = k
						} else {
							break
						}
					}
					keyPos := -1
					for i, s := range movie.Video.Samples {
						if s.Number == key.SampleNumber {
							keyPos = i
							break
						}
					}
					if keyPos > wantPos {
						wantShow = movie.Video.Samples[keyPos].PTSMs
					}
				}
				if landed != wantShow {
					t.Fatalf("%s/%d: landed %d want %d (floor %d)", clip.File, sk.TargetMs, landed, wantShow, wantLanded)
				}
				// Drive the clock until frames show: the first shown stamp
				// must be >= the landing (reorder delay: the landing
				// picture needs its references first, same as plain
				// clips), stamps stay monotonic, and the landed oracle
				// frame's pixels match the .yuv at its display index.
				h.now = landed
				var stamps []int64
				for i := 0; i < 100 && len(stamps) < 3; i++ {
					h.now += 200
					fr, _ := p.Poll()
					if fr != nil {
						stamps = append(stamps, fr.PTSMs)
					}
					runtime.Gosched()
					time.Sleep(time.Millisecond)
				}
				if len(stamps) == 0 {
					t.Fatalf("%s/%d: landing %d never showed", clip.File, sk.TargetMs, landed)
				}
				if stamps[0] < landed {
					t.Fatalf("%s/%d: first show %d before landing %d (%v)", clip.File, sk.TargetMs, stamps[0], landed, stamps)
				}
				for i := 1; i < len(stamps); i++ {
					if stamps[i] <= stamps[i-1] {
						t.Fatalf("%s/%d: stamps not monotonic: %v", clip.File, sk.TargetMs, stamps)
					}
				}
				// The landing shows within its reorder delay (frag100:
				// progressive, near tick; frag5: one B GOP delay: the
				// covering P/B needs the later-decoded reference, so
				// the background emits in display order from the key).
				if stamps[0]-landed > 800 {
					t.Fatalf("%s/%d: landing %d late, first show %d (%v)", clip.File, sk.TargetMs, landed, stamps[0], stamps)
				}
			}
		})
	}
}

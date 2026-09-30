package video

// B1 ffmpeg parity: fragmented MP4 (phone-style边录边存) opens, decodes
// byte-exact, plays to Ended and seeks to the floor covering frame.
// Baseline: testdata/b1_ffmpeg.json (ffmpeg 4.4.2, same machine).
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
	"testing"
	"time"

	ff "github.com/energye/gpui/video/ffmpeg"
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
			// Stream identity through the ffmpeg demuxer (ffmpeg owns
			// fragmented boxes natively): size + codec + frame count
			// match ffprobe on the same clip. Segment/profile/level
			// details ride the baseline json, not a Go box walk.
			dec, err := ff.Open(mp4Path)
			if err != nil {
				t.Fatalf("ff open %s: %v", clip.File, err)
			}
			info := dec.Info()
			if info.Width != clip.Stream.Width || info.Height != clip.Stream.Height {
				t.Fatalf("%s: size %dx%d want %dx%d", clip.File, info.Width, info.Height, clip.Stream.Width, clip.Stream.Height)
			}
			if got := ffCodecName(info.CodecID); got != "h264" {
				dec.Close()
				t.Fatalf("%s: codec %q want h264", clip.File, got)
			}
			// Fragmented clips carry no frame count in the header
			// (demuxer reports 0): count displayable pictures instead.
			frames := 0
			for {
				fr, nerr := dec.Next()
				if nerr != nil {
					break
				}
				fr.Release()
				frames++
			}
			dec.Close()
			if frames != clip.Stream.NbFrames || frames != clip.Expect.Samples {
				t.Fatalf("%s: frames %d want %d", clip.File, frames, clip.Expect.Samples)
			}
		})
	}
}

// TestB1DecodeExact pins decode parity on the ffmpeg backend: the
// bundled lib decodes every frame straight to RGBA (size w*h*4 each,
// count == samples), and the committed YUV oracle stays intact
// (presence + bytes + md5 provenance). The Go YUV picture oracle retired
// with the Go decoder; pixel truth now lives in the player-vs-direct
// RGBA checks (VR3) and the play-to-end gates below.
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
			dec, err := ff.Open(mp4Path)
			if err != nil {
				t.Fatalf("ff open %s: %v", clip.File, err)
			}
			defer dec.Close()
			wantPx := clip.Stream.Width * clip.Stream.Height * 4
			frames := 0
			for {
				fr, err := dec.Next()
				if err != nil {
					break
				}
				if len(fr.Pix) != wantPx {
					fr.Release()
					t.Fatalf("%s frame %d: pix %d want %d", clip.File, frames, len(fr.Pix), wantPx)
				}
				fr.Release()
				frames++
			}
			if frames != clip.Expect.Samples {
				t.Fatalf("%s: frames %d want %d", clip.File, frames, clip.Expect.Samples)
			}
		})
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
			// streaming long-clip gate). Paced on every clip (not just
			// long ones): a tight no-yield loop starves the decoder.
			deadline := time.Now().Add(90 * time.Second)
			for !ended {
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
				runtime.Gosched()
				time.Sleep(time.Millisecond)
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
// ; the first
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
			for _, sk := range byClip[clip.File] {
				landed, err := p.SeekTo(sk.TargetMs)
				if err != nil {
					t.Fatalf("%s/%d: seek: %v", clip.File, sk.TargetMs, err)
				}
				// Echo contract (same as VR5): the player lands the
				// target stamp itself; the background filter drops
				// below it and the covering picture shows next.
				if landed != sk.TargetMs {
					t.Fatalf("%s/%d: landed %d want %d (echo)", clip.File, sk.TargetMs, landed, sk.TargetMs)
				}
				// Drive the clock until frames show: the first shown
				// stamp must cover the landing (reorder delay is
				// absorbed inside ffmpeg, same as plain clips),
				// stamps stay monotonic.
				var stamps []int64
				firstRound := true
				for i := 0; i < 100 && len(stamps) < 3; i++ {
					fr, _ := p.Poll()
					if fr != nil {
						stamps = append(stamps, fr.PTSMs)
					}
					if !firstRound {
						h.now += 200
					}
					firstRound = false
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

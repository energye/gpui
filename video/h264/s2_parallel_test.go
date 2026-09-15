package h264

// S2 GOP gate: parallel GOP decode is byte-exact vs sequential and really
// overlaps on multi-GOP clips. Player wiring follows; this gate pins the
// kernel only.
//
// Peer: ffmpeg frame threading (libavcodec/pthread_frame.c:917-923
// thread_count, :948-949 delay, :551/:569-585 submit loop;
// pthread_internal.h:26 cap 16; h264dec.c:660-667 IDR clears refs) against
// s2_parallel.go decodeGOPsParallel. Throughput reference (log only):
// video/testdata/vr7t_ffmpeg.json yuv_reference utime_per_frame_ms on the
// same long clip (ffmpeg -benchmark -i clip -f null -); the hard asserts
// are exactness + overlap, never an invented speedup line.
// Clip: ../testdata/vr_stream_long.mp4 (tracked 230K, 320x240/Main/200
// samples/40 IDR GOPs x5; missing file FAILs, never Skips).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/video/mp4"
)

func TestS2GOPParallelExactAndThroughput(t *testing.T) {
	mp4Path := filepath.Join("..", "testdata", "vr_stream_long.mp4")
	fi, err := os.Stat(mp4Path)
	if err != nil {
		t.Fatalf("long clip absent (tracked, want 234721B): %v", err)
	}
	if fi.Size() != 234721 {
		t.Fatalf("long clip bytes %d want 234721 (re-record baseline if the clip changed)", fi.Size())
	}
	movie, err := mp4.ParseFile(mp4Path)
	if err != nil {
		t.Fatalf("demux: %v", err)
	}
	v := movie.Video
	if v == nil {
		t.Fatal("no video track")
	}
	if len(v.Samples) != 200 {
		t.Fatalf("samples %d want 200", len(v.Samples))
	}
	if len(v.Keyframes) != 40 {
		t.Fatalf("keyframes %d want 40", len(v.Keyframes))
	}
	if int(v.Width) != 320 || int(v.Height) != 240 {
		t.Fatalf("track %dx%d want 320x240", v.Width, v.Height)
	}
	avcc, err := ParseAVCC(v.AVCConfig)
	if err != nil {
		t.Fatalf("avcc: %v", err)
	}
	f, err := os.Open(mp4Path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	blobs := make([][]byte, len(v.Samples))
	for i, s := range v.Samples {
		buf := make([]byte, s.Size)
		if _, err := f.ReadAt(buf, int64(s.Offset)); err != nil {
			t.Fatalf("sample %d: %v", s.Number, err)
		}
		blobs[i] = buf
	}
	keyIdx := map[int]bool{}
	for _, k := range v.Keyframes {
		keyIdx[k.SampleNumber] = true
	}
	var starts []int
	for i, s := range v.Samples {
		if keyIdx[s.Number] {
			starts = append(starts, i)
		}
	}
	if len(starts) != 40 || starts[0] != 0 {
		t.Fatalf("gop starts %d want 40 from 0", len(starts))
	}
	// Sequential reference (single Decoder, decode order).
	t0 := time.Now()
	seqDec := NewDecoder(nil)
	for _, raw := range avcc.SPS {
		if err := seqDec.DecodeNALU(raw); err != nil {
			t.Fatalf("seq sps: %v", err)
		}
	}
	for _, raw := range avcc.PPS {
		if err := seqDec.DecodeNALU(raw); err != nil {
			t.Fatalf("seq pps: %v", err)
		}
	}
	var seqPics []*Picture
	for i, blob := range blobs {
		units, err := SplitAVCC(blob, avcc.LengthSize)
		if err != nil {
			t.Fatalf("seq split %d: %v", i, err)
		}
		for _, u := range units {
			if err := seqDec.DecodeNALU(u); err != nil {
				t.Fatalf("seq decode %d: %v", i, err)
			}
		}
		pic, err := seqDec.FinishPicture()
		if err != nil {
			t.Fatalf("seq finish %d: %v", i, err)
		}
		seqPics = append(seqPics, pic)
	}
	seqWall := time.Since(t0)
	// Parallel GOPs.
	var stats s2GOPStats
	t1 := time.Now()
	parPics, err := decodeGOPsParallel(blobs, starts, avcc, 0, &stats)
	if err != nil {
		t.Fatalf("parallel: %v", err)
	}
	parWall := time.Since(t1)
	if len(parPics) != len(seqPics) {
		t.Fatalf("count parallel=%d want %d", len(parPics), len(seqPics))
	}
	for i := range seqPics {
		a, b := seqPics[i], parPics[i]
		if a.Width != b.Width || a.Height != b.Height {
			t.Fatalf("frame %d size %dx%d vs %dx%d", i, a.Width, a.Height, b.Width, b.Height)
		}
		if len(a.Y) != len(b.Y) || len(a.Cb) != len(b.Cb) || len(a.Cr) != len(b.Cr) {
			t.Fatalf("frame %d plane sizes differ", i)
		}
		for j := range a.Y {
			if a.Y[j] != b.Y[j] {
				t.Fatalf("frame %d luma diff at %d (%d vs %d)", i, j, a.Y[j], b.Y[j])
			}
		}
		for j := range a.Cb {
			if a.Cb[j] != b.Cb[j] {
				t.Fatalf("frame %d cb diff at %d", i, j)
			}
		}
		for j := range a.Cr {
			if a.Cr[j] != b.Cr[j] {
				t.Fatalf("frame %d cr diff at %d", i, j)
			}
		}
	}
	if stats.MaxInFlight <= 1 {
		t.Fatalf("max_in_flight=%d want >1 (40 GOPs over %d workers)", stats.MaxInFlight, s2WorkerCount(0))
	}
	// Throughput reference only (ffmpeg yuv utime/frame on the same clip).
	ffmpegPerFrame := 0.0
	if buf, err := os.ReadFile(filepath.Join("..", "testdata", "vr7t_ffmpeg.json")); err == nil {
		var vr7t struct {
			VR7T struct {
				Clips []struct {
					File      string `json:"file"`
					Benchmark struct {
						PerFrame     float64 `json:"utime_per_frame_ms"`
						YUVReference struct {
							PerFrame float64 `json:"utime_per_frame_ms"`
						} `json:"yuv_reference"`
					} `json:"benchmark"`
				} `json:"clips"`
			} `json:"vr7_t"`
		}
		if json.Unmarshal(buf, &vr7t) == nil {
			for _, c := range vr7t.VR7T.Clips {
				if c.File == "vr_stream_long.mp4" {
					ffmpegPerFrame = c.Benchmark.YUVReference.PerFrame
				}
			}
		}
	}
	t.Logf("s2 long 200f/40gops: seq=%v par=%v speedup=%.2fx workers=%d max_in_flight=%d ffmpeg_yuv_per_frame=%.2fms (ref only)",
		seqWall, parWall, float64(seqWall)/float64(parWall), s2WorkerCount(0), stats.MaxInFlight, ffmpegPerFrame)
}

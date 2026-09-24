package h264

// S2xB nested gate: a long-GOP clip decodes pixel-exact nested vs
// sequential (the nested lane only runs at/above s2NestedMinFrames, so
// the small-clip S2 gate cannot cover it). Player wiring follows; this
// gate pins the kernel only.
// Clip: ../testdata/vr_b_long.mp4 (tracked 20110B, 96x96/Main/80
// samples/2 IDR GOPs x40; missing file FAILs, never Skips).

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/video/mp4"
)

func TestS2NestedExactLongGOP(t *testing.T) {
	mp4Path := filepath.Join("..", "testdata", "vr_b_long.mp4")
	fi, err := os.Stat(mp4Path)
	if err != nil {
		t.Fatalf("long-GOP clip absent (tracked, want 20110B): %v", err)
	}
	if fi.Size() != 20110 {
		t.Fatalf("long-GOP clip bytes %d want 20110 (re-record baseline if the clip changed)", fi.Size())
	}
	movie, err := mp4.ParseFile(mp4Path)
	if err != nil {
		t.Fatalf("demux: %v", err)
	}
	v := movie.Video
	if v == nil {
		t.Fatal("no video track")
	}
	if len(v.Samples) != 80 {
		t.Fatalf("samples %d want 80", len(v.Samples))
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
	if len(starts) != 2 || starts[0] != 0 {
		t.Fatalf("gop starts %v want 2 from 0", starts)
	}
	for g := range starts {
		g1 := len(blobs)
		if g+1 < len(starts) {
			g1 = starts[g+1]
		}
		if g1-starts[g] < s2NestedMinFrames {
			t.Fatalf("gop %d len %d below nested lane %d (gate would not cover nesting)",
				g, g1-starts[g], s2NestedMinFrames)
		}
	}
	// Sequential reference (the fallback lane, one decoder).
	seqPics, err := decodeS2GOP(blobs, avcc)
	if err != nil {
		t.Fatalf("sequential: %v", err)
	}
	defer func() {
		for _, pic := range seqPics {
			pic.Release()
		}
	}()
	// Nested (GOP fan-out outside, frame DAG inside long GOPs).
	var stats s2GOPStats
	nestedPics, err := decodeGOPsParallel(blobs, starts, avcc, 0, &stats)
	if err != nil {
		t.Fatalf("nested: %v", err)
	}
	defer func() {
		for _, pic := range nestedPics {
			pic.Release()
		}
	}()
	if len(nestedPics) != len(seqPics) {
		t.Fatalf("count nested=%d want %d", len(nestedPics), len(seqPics))
	}
	for i := range seqPics {
		a, b := seqPics[i], nestedPics[i]
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
		t.Fatalf("max_in_flight=%d want >1 (2 GOPs over %d workers)", stats.MaxInFlight, s2WorkerCount(0))
	}
	t.Logf("s2 nested long-GOP 80f/2gops: exact, max_in_flight=%d workers=%d",
		stats.MaxInFlight, s2WorkerCount(0))
}

package video

// S8 ffmpeg parity (seek index only; B1 fragmented and N3 infinite-live
// stay untouched).
// Peer: libavformat/mov.c:12247 mov_seek_fragment (search_frag_timestamp
// picks the segment, then drills into it) + :12310 mov_seek_stream +
// :12401 mov_read_seek against seekPlan/seekKeyframe (floor covering +
// landing key + next/prev); libavformat/seek.c:132
// ff_index_search_timestamp + :245 av_index_search_timestamp (binary
// search shape) against seek_index.go covering/keyAtOrBefore/nextKey/
// prevKey; fftools/ffplay.c:1527 stream_seek (repark + drop until
// landing, unchanged — only the table lookup got faster).
// Pass line: indexed answers equal the old linear scans exactly on every
// tracked clip (same landed/key/forward, so VR5 parity and long-clip
// seeks cannot drift); per-seek table steps stay logarithmic; small clips
// (<=64) keep the buffered path with no index. Clips are tracked in git:
// absent files FAIL, not skip.

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/video/mp4"
)

// s8LinearCovering is the pre-S8 scan (floor covering + earliest-of-ties)
// kept as the oracle: the index must answer identically.
func s8LinearCovering(samples []mp4.Sample, target int64) (spos int, landed int64) {
	spos = -1
	for i, s := range samples {
		if s.PTSMs <= target && (spos < 0 || s.PTSMs > landed) {
			spos = i
			landed = s.PTSMs
		}
	}
	if spos < 0 {
		best := 0
		for i := 1; i < len(samples); i++ {
			if samples[i].PTSMs < samples[best].PTSMs {
				best = i
			}
		}
		return best, samples[best].PTSMs
	}
	return spos, landed
}

func s8LinearKey(keyframes []mp4.Keyframe, target int64) mp4.Keyframe {
	key := keyframes[0]
	if target < key.PTSMs {
		return keyframes[0]
	}
	best := keyframes[0]
	for _, k := range keyframes[1:] {
		if k.PTSMs <= target {
			best = k
		} else {
			break
		}
	}
	return best
}

func s8LinearNext(keyframes []mp4.Keyframe, ref int64) mp4.Keyframe {
	var key mp4.Keyframe
	found := false
	for _, k := range keyframes {
		if k.PTSMs > ref && (!found || k.PTSMs < key.PTSMs) {
			key, found = k, true
		}
	}
	if !found {
		return keyframes[len(keyframes)-1]
	}
	return key
}

func s8LinearPrev(keyframes []mp4.Keyframe, ref int64) mp4.Keyframe {
	var key mp4.Keyframe
	found := false
	for _, k := range keyframes {
		if k.PTSMs < ref && (!found || k.PTSMs > key.PTSMs) {
			key, found = k, true
		}
	}
	if !found {
		return keyframes[0]
	}
	return key
}

func s8LoadTables(t *testing.T, file string) ([]mp4.Sample, []mp4.Keyframe) {
	t.Helper()
	path := filepath.Join("testdata", file)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("clip %s absent (%v): tracked S8 clip must pass, not skip", file, err)
	}
	movie, err := mp4.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile %s: %v", file, err)
	}
	if movie.Video == nil || len(movie.Video.Samples) == 0 || len(movie.Video.Keyframes) == 0 {
		t.Fatalf("%s: no video tables", file)
	}
	return movie.Video.Samples, movie.Video.Keyframes
}

// TestS8IndexMatchesLinear pins exact parity: every sample stamp and its
// neighbours, plus out-of-range targets, answer identically through the
// index and the old scans (covering/key/next/prev/keyPos).
func TestS8IndexMatchesLinear(t *testing.T) {
	for _, file := range []string{
		"vr5_seek.mp4", "vr2_m_bframes.mp4", "vr_stream_long.mp4",
		"vr_f42906.mp4", "vr_oceans.mp4",
	} {
		file := file
		t.Run(file, func(t *testing.T) {
			samples, keyframes := s8LoadTables(t, file)
			si := buildSeekIndex(samples, keyframes)
			if si.n != len(samples) {
				t.Fatalf("index n = %d, want %d", si.n, len(samples))
			}
			targets := make([]int64, 0, 3*len(samples)+4)
			for _, s := range samples {
				targets = append(targets, s.PTSMs-1, s.PTSMs, s.PTSMs+1)
			}
			minPTS, maxPTS := samples[0].PTSMs, samples[0].PTSMs
			for _, s := range samples[1:] {
				if s.PTSMs < minPTS {
					minPTS = s.PTSMs
				}
				if s.PTSMs > maxPTS {
					maxPTS = s.PTSMs
				}
			}
			targets = append(targets, minPTS-1, minPTS-100000, maxPTS+1, maxPTS+100000)
			for _, target := range targets {
				wantPos, wantLanded := s8LinearCovering(samples, target)
				gotPos, gotLanded, _ := si.covering(target)
				if gotPos != wantPos || gotLanded != wantLanded {
					t.Fatalf("covering(%d) = pos %d pts %d, want pos %d pts %d",
						target, gotPos, gotLanded, wantPos, wantLanded)
				}
				wantKey := s8LinearKey(keyframes, target)
				gotKey, _ := si.keyAtOrBefore(target)
				if gotKey != wantKey {
					t.Fatalf("key(%d) = %+v, want %+v", target, gotKey, wantKey)
				}
				if got, ok := keyDecodePos(samples, gotKey); !ok || samples[got].Number != gotKey.SampleNumber {
					t.Fatalf("keyPos(%d) = %d ok=%v, want sample %d", target, got, ok, gotKey.SampleNumber)
				}
				if got, _ := si.nextKey(target); got != s8LinearNext(keyframes, target) {
					t.Fatalf("next(%d) = %+v, want %+v", target, got, s8LinearNext(keyframes, target))
				}
				{
					got, _ := si.prevKey(target)
					want := s8LinearPrev(keyframes, target)
					if got != want {
						t.Fatalf("prev(%d) = %+v, want %+v", target, got, want)
					}
				}
			}
		})
	}
}

// TestS8StepsLogarithmic pins the S8 complexity: per-seek table steps
// stay within twice the binary-search depth on every tracked clip with
// more than one segment, so hours-long clips seek in tens of steps.
func TestS8StepsLogarithmic(t *testing.T) {
	for _, file := range []string{"vr_stream_long.mp4", "vr_f42906.mp4", "vr_oceans.mp4"} {
		file := file
		t.Run(file, func(t *testing.T) {
			samples, keyframes := s8LoadTables(t, file)
			si := buildSeekIndex(samples, keyframes)
			if len(si.segs) < 2 {
				t.Fatalf("%s: segments = %d, want >= 2 (else the seek path is untested)", file, len(si.segs))
			}
			for i := 1; i < len(si.segs); i++ {
				if si.segs[i].firstPTS < si.segs[i-1].firstPTS || si.segs[i].start != si.segs[i-1].end {
					t.Fatalf("%s: segments not contiguous/monotonic at %d: %+v %+v",
						file, i, si.segs[i-1], si.segs[i])
				}
			}
			bound := 2*(int(math.Ceil(math.Log2(float64(len(samples)))))+1) + 2
			maxSteps := 0
			for _, s := range samples {
				for _, target := range []int64{s.PTSMs - 1, s.PTSMs, s.PTSMs + 1} {
					_, _, cs := si.covering(target)
					_, ks := si.keyAtOrBefore(target)
					if cs > maxSteps {
						maxSteps = cs
					}
					if ks > bound {
						t.Fatalf("%s: key steps %d exceed log bound %d (n=%d)",
							file, ks, bound, len(samples))
					}
				}
			}
			if maxSteps > bound {
				t.Fatalf("%s: covering steps %d exceed log bound %d (n=%d segs=%d)",
					file, maxSteps, bound, len(samples), len(si.segs))
			}
			t.Logf("%s: n=%d segs=%d maxCoverSteps=%d bound=%d", file, len(samples), len(si.segs), maxSteps, bound)
		})
	}
}

// TestS8PlayerWiring pins the paths: streaming clips build the index and
// seek through it with identical evidence, small clips keep the buffered
// path with no index (fast path untouched).
func TestS8PlayerWiring(t *testing.T) {
	h := &handClock{}
	long, err := OpenFile("testdata/vr_stream_long.mp4", Options{NowMs: h.at})
	if err != nil {
		t.Fatalf("open long: %v", err)
	}
	defer long.Close()
	if long.Buffered() {
		t.Fatal("long clip buffered, want streaming index path")
	}
	if long.sidx == nil {
		t.Fatal("streaming player has no seek index")
	}
	for _, target := range []int64{0, 199, 200, 399, 4000, 20000, 39999, 40000, 100000} {
		wantPos, wantLanded := s8LinearCovering(long.samples, target)
		wantKey := s8LinearKey(long.keyframes, target)
		gotPos, gotLanded, _, gotKey, gotKeyPos, err := long.seekPlan(target)
		if err != nil {
			t.Fatalf("seekPlan(%d): %v", target, err)
		}
		if gotPos != wantPos || gotLanded != wantLanded || gotKey != wantKey {
			t.Fatalf("seekPlan(%d) = pos %d pts %d key %+v, want pos %d pts %d key %+v",
				target, gotPos, gotLanded, gotKey, wantPos, wantLanded, wantKey)
		}
		if wantKeyPos, ok := keyDecodePos(long.samples, wantKey); !ok || gotKeyPos != wantKeyPos {
			t.Fatalf("seekPlan(%d) keyPos = %d, want %d", target, gotKeyPos, wantKeyPos)
		}
	}
	landed, err := long.SeekTo(20000)
	if err != nil {
		t.Fatalf("SeekTo 20000: %v", err)
	}
	if wantPos, wantLanded := s8LinearCovering(long.samples, 20000); landed != wantLanded {
		t.Fatalf("SeekTo landed = %d, want linear %d (pos %d)", landed, wantLanded, wantPos)
	}

	small, err := OpenFile("testdata/vr5_seek.mp4", Options{NowMs: h.at})
	if err != nil {
		t.Fatalf("open small: %v", err)
	}
	defer small.Close()
	if !small.Buffered() {
		t.Fatal("small clip streaming, want buffered fast path")
	}
	if small.sidx != nil {
		t.Fatal("buffered player built an index, want fast path untouched")
	}
}

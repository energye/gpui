package video

import (
	"sort"

	"github.com/energye/gpui/video/mp4"
)

// S8 streaming seek index (section 11.7 S8, section 12 S8 row).
// ffmpeg peers (read-only, no code copied):
//   libavformat/mov.c:12247 mov_seek_fragment (search_frag_timestamp picks
//     the segment, then drills into it) + :12310 mov_seek_stream +
//     :12401 mov_read_seek
//   libavformat/seek.c:132 ff_index_search_timestamp (binary search over
//     the index) + :245 av_index_search_timestamp
//   fftools/ffplay.c:1527 stream_seek (repark + drop until landing)
//
// Shape copied: segment-first, sample-second, both binary. Sample tables
// stay resident (ffmpeg plain-MP4 mov.c keeps its small index too);
// on-demand sample paging stays a later N3/B1 item. Small clips (<=64)
// keep the buffered full-cache path and never build this index.

const seekSegSize = 64

type seekSeg struct {
	start, end int // [start,end) over order
	firstPTS   int64
	lastPTS    int64
}

type seekIndex struct {
	n        int
	order    []int   // decode positions by (PTS, pos)
	orderPTS []int64 // samples[order[i]].PTSMs
	segs     []seekSeg
	keys     []mp4.Keyframe // keyframes by (PTS, SampleNumber)
}

func buildSeekIndex(samples []mp4.Sample, keyframes []mp4.Keyframe) *seekIndex {
	si := &seekIndex{n: len(samples)}
	if len(samples) == 0 {
		return si
	}
	si.order = make([]int, len(samples))
	for i := range si.order {
		si.order[i] = i
	}
	sort.Slice(si.order, func(a, b int) bool {
		pa, pb := samples[si.order[a]].PTSMs, samples[si.order[b]].PTSMs
		if pa != pb {
			return pa < pb
		}
		return si.order[a] < si.order[b]
	})
	si.orderPTS = make([]int64, len(si.order))
	for i, pos := range si.order {
		si.orderPTS[i] = samples[pos].PTSMs
	}
	for s := 0; s < len(si.order); s += seekSegSize {
		e := s + seekSegSize
		if e > len(si.order) {
			e = len(si.order)
		}
		si.segs = append(si.segs, seekSeg{start: s, end: e, firstPTS: si.orderPTS[s], lastPTS: si.orderPTS[e-1]})
	}
	if len(keyframes) > 0 {
		si.keys = append([]mp4.Keyframe(nil), keyframes...)
		sort.Slice(si.keys, func(a, b int) bool {
			if si.keys[a].PTSMs != si.keys[b].PTSMs {
				return si.keys[a].PTSMs < si.keys[b].PTSMs
			}
			return si.keys[a].SampleNumber < si.keys[b].SampleNumber
		})
	}
	return si
}

// covering finds the floor frame: last display stamp <= target (first of
// ties, matching the old linear scan). steps counts binary comparisons.
func (si *seekIndex) covering(target int64) (spos int, landed int64, steps int) {
	lo, hi := 0, len(si.segs)
	for lo < hi {
		m := (lo + hi) >> 1
		steps++
		if si.segs[m].firstPTS <= target {
			lo = m + 1
		} else {
			hi = m
		}
	}
	s := lo - 1
	if s < 0 {
		return si.order[0], si.orderPTS[0], steps
	}
	sg := si.segs[s]
	lo, hi = sg.start, sg.end
	for lo < hi {
		m := (lo + hi) >> 1
		steps++
		if si.orderPTS[m] <= target {
			lo = m + 1
		} else {
			hi = m
		}
	}
	i := lo - 1
	landed = si.orderPTS[i]
	for i > 0 && si.orderPTS[i-1] == landed {
		i--
	}
	return si.order[i], landed, steps
}

// keyAtOrBefore finds the last keyframe at or before target (first one
// when the target precedes them all), mirroring KeyframeNear.
func (si *seekIndex) keyAtOrBefore(target int64) (key mp4.Keyframe, steps int) {
	lo, hi := 0, len(si.keys)
	for lo < hi {
		m := (lo + hi) >> 1
		steps++
		if si.keys[m].PTSMs <= target {
			lo = m + 1
		} else {
			hi = m
		}
	}
	if lo-1 < 0 {
		return si.keys[0], steps
	}
	return si.keys[lo-1], steps
}

// nextKey finds the first keyframe after ref, clamped to the last one.
func (si *seekIndex) nextKey(ref int64) (key mp4.Keyframe, steps int) {
	lo, hi := 0, len(si.keys)
	for lo < hi {
		m := (lo + hi) >> 1
		steps++
		if si.keys[m].PTSMs <= ref {
			lo = m + 1
		} else {
			hi = m
		}
	}
	if lo >= len(si.keys) {
		return si.keys[len(si.keys)-1], steps
	}
	return si.keys[lo], steps
}

// prevKey finds the last keyframe strictly before ref, clamped to the
// first one.
func (si *seekIndex) prevKey(ref int64) (key mp4.Keyframe, steps int) {
	lo, hi := 0, len(si.keys)
	for lo < hi {
		m := (lo + hi) >> 1
		steps++
		if si.keys[m].PTSMs < ref {
			lo = m + 1
		} else {
			hi = m
		}
	}
	if lo-1 < 0 {
		return si.keys[0], steps
	}
	return si.keys[lo-1], steps
}

// keyDecodePos maps a keyframe to its decode-order position. Sample
// numbers are 1-based decode indexes by construction (mp4 buildTrack),
// so this is arithmetic; the scan fallback stays for safety.
func keyDecodePos(samples []mp4.Sample, key mp4.Keyframe) (int, bool) {
	pos := key.SampleNumber - 1
	if pos >= 0 && pos < len(samples) && samples[pos].Number == key.SampleNumber {
		return pos, true
	}
	for i, s := range samples {
		if s.Number == key.SampleNumber {
			return i, true
		}
	}
	return 0, false
}

package main

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/energye/gpui/video"
	"github.com/energye/gpui/video/mp4"
)

// S8 gate clips: one buffered small clip (fast path, no index) plus one
// streaming long clip (segment + binary index). Same two files as
// video/s8_index_test.go's player-wiring half, so the window never drifts
// from the gate. Steps stay pinned by the gate (log bound 20/24/26,
// actual 10/11/12); the window pins landing parity plus visible recovery.

type s8Target struct {
	Clip   string
	Target int64
}

type s8Group struct {
	Name   string `json:"name"`
	Passed int    `json:"passed"`
	Total  int    `json:"total"`
	Note   string `json:"note"`
}

type s8Evidence struct {
	Groups     []s8Group `json:"groups"`
	Passed     int       `json:"passed"`
	Total      int       `json:"total"`
	Failed     int       `json:"failed_items"`
	ForwardMax int64     `json:"forward_max"`
	Clips      string    `json:"clips"`
	Profile    string    `json:"profile"`
	ErrText    string    `json:"err"`
}

func resolveTestdata(name string) string {
	for _, p := range []string{"video/testdata/" + name, "../../video/testdata/" + name} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "video/testdata/" + name
}

func s8Targets() []s8Target {
	return []s8Target{
		{Clip: "vr5_seek.mp4", Target: 600},
		{Clip: "vr5_seek.mp4", Target: 1800},
		{Clip: "vr_stream_long.mp4", Target: 0},
		{Clip: "vr_stream_long.mp4", Target: 200},
		{Clip: "vr_stream_long.mp4", Target: 5000},
		{Clip: "vr_stream_long.mp4", Target: 20000},
		{Clip: "vr_stream_long.mp4", Target: 35000},
		{Clip: "vr_stream_long.mp4", Target: 100000},
	}
}

// linearWant mirrors the pre-S8 scan (floor covering + earliest-of-ties)
// plus the B-reorder fallback (key after target lands on the key itself),
// identical to video.seekPlan's contract. The index must answer the same.
func linearWant(samples []mp4.Sample, keyframes []mp4.Keyframe, target int64) (wantPos int, wantLanded int64, wantKey mp4.Keyframe, wantKeyPos int, wantShow int64) {
	wantPos = -1
	for i, s := range samples {
		if s.PTSMs <= target && (wantPos < 0 || s.PTSMs > wantLanded) {
			wantPos, wantLanded = i, s.PTSMs
		}
	}
	if wantPos < 0 {
		best := 0
		for i := 1; i < len(samples); i++ {
			if samples[i].PTSMs < samples[best].PTSMs {
				best = i
			}
		}
		wantPos, wantLanded = best, samples[best].PTSMs
	}
	wantKey = keyframes[0]
	if target >= wantKey.PTSMs {
		best := keyframes[0]
		for _, k := range keyframes[1:] {
			if k.PTSMs <= target {
				best = k
			} else {
				break
			}
		}
		wantKey = best
	}
	wantKeyPos = -1
	for i, s := range samples {
		if s.Number == wantKey.SampleNumber {
			wantKeyPos = i
			break
		}
	}
	wantShow = wantLanded
	if wantKeyPos > wantPos {
		wantShow = samples[wantKeyPos].PTSMs
	}
	return wantPos, wantLanded, wantKey, wantKeyPos, wantShow
}

type handClock struct{ now int64 }

func (h *handClock) at() int64 { return h.now }

// checkSeekOne pins one jump: landing equals the linear oracle (indexed
// and linear scans identical), forward sane (no head replay), and the
// tail shows promptly from at/after the landing (never black, never
// backwards). The first shown stamp may lag the landing by the stream's
// reorder delay (same rule as video/b1_ffmpeg_test.go TestB1SeekFloor:
// stamps[0] >= landed, stamps monotonic, stamps[0]-landed <= 800ms),
// because a covering P/B picture needs its later-decoded reference first.
// Buffered clips emit the landing itself; streaming delay frames still
// prove the index put the needle on the right key.
func checkSeekOne(clip string, target int64) (landed, keyMs, delta, forward int64, shown bool, err error) {
	path := resolveTestdata(clip)
	movie, err := mp4.ParseFile(path)
	if err != nil {
		return 0, 0, 0, 0, false, fmt.Errorf("拆盒失败: %v", err)
	}
	v := movie.Video
	if v == nil || len(v.Samples) == 0 || len(v.Keyframes) == 0 {
		return 0, 0, 0, 0, false, fmt.Errorf("没视频表")
	}
	_, _, wantKey, _, wantShow := linearWant(v.Samples, v.Keyframes, target)
	h := &handClock{}
	p, err := video.OpenFile(path, video.Options{NowMs: h.at})
	if err != nil {
		return 0, 0, 0, 0, false, fmt.Errorf("打不开: %v", err)
	}
	defer p.Close()
	landed, err = p.SeekTo(target)
	if err != nil {
		return 0, 0, 0, 0, false, fmt.Errorf("跳不动: %v", err)
	}
	if landed != wantShow {
		return landed, 0, 0, 0, false, fmt.Errorf("落点%d要%d(键%d)", landed, wantShow, wantKey.PTSMs)
	}
	_, _, _, keyMs, delta, forward = p.SeekInfo()
	if keyMs != wantKey.PTSMs {
		return landed, keyMs, delta, forward, false, fmt.Errorf("落键%d要%d", keyMs, wantKey.PTSMs)
	}
	if forward < 1 || forward > int64(len(v.Samples)) {
		return landed, keyMs, delta, forward, false, fmt.Errorf("前解%d越界(共%d)", forward, len(v.Samples))
	}
	// Head-replay guard: even the longest jump must decode far fewer
	// samples than the whole clip (long 200 frames, GOP 5).
	if forward >= int64(len(v.Samples)) {
		return landed, keyMs, delta, forward, false, fmt.Errorf("疑似从头解: 前解%d不小于全片%d", forward, len(v.Samples))
	}
	// Show: drive the clock until frames appear; the first shown stamp
	// must be at/after the landing (reorder delay allowed), stamps stay
	// monotonic, and the landing arrives within 800ms (B1's line).
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
		return landed, keyMs, delta, forward, false, fmt.Errorf("跳后黑屏: 落点%d没播出来", landed)
	}
	if stamps[0] < landed {
		return landed, keyMs, delta, forward, false, fmt.Errorf("首现%d早于落点%d", stamps[0], landed)
	}
	for i := 1; i < len(stamps); i++ {
		if stamps[i] <= stamps[i-1] {
			return landed, keyMs, delta, forward, false, fmt.Errorf("跳后时间不单调")
		}
	}
	if stamps[0]-landed > 800 {
		return landed, keyMs, delta, forward, false, fmt.Errorf("落点恢复太慢: 首现%d落点%d", stamps[0], landed)
	}
	return landed, keyMs, delta, forward, true, nil
}

// loadS8 runs the S8 gates and folds them into groups/items counts:
// seek 8 jumps + show 8 tails + path 2 clips = 18.
func loadS8() s8Evidence {
	ev := s8Evidence{Clips: "vr5_seek.mp4+vr_stream_long.mp4", Profile: "Main"}
	targets := s8Targets()

	seekPass := 0
	seekNote := ""
	var forwardMax int64
	showPass := 0
	showNote := ""
	for _, tg := range targets {
		landed, _, _, forward, shown, err := checkSeekOne(tg.Clip, tg.Target)
		_ = landed
		if err != nil {
			if seekNote == "" {
				seekNote = fmt.Sprintf("%s/%d:%s", tg.Clip, tg.Target, err.Error())
			}
			if showNote == "" && !shown {
				showNote = fmt.Sprintf("%s/%d没播出来", tg.Clip, tg.Target)
			}
			continue
		}
		seekPass++
		if forward > forwardMax {
			forwardMax = forward
		}
		if shown {
			showPass++
		} else if showNote == "" {
			showNote = fmt.Sprintf("%s/%d没播出来", tg.Clip, tg.Target)
		}
	}
	if seekNote == "" {
		seekNote = "落点全对线性"
	}
	if showNote == "" {
		showNote = "跳后尾帧即现"
	}
	ev.Groups = append(ev.Groups, s8Group{Name: "seek", Passed: seekPass, Total: len(targets), Note: seekNote})
	ev.Groups = append(ev.Groups, s8Group{Name: "show", Passed: showPass, Total: len(targets), Note: showNote})
	ev.ForwardMax = forwardMax

	// Group path: small stays buffered (fast path untouched), long streams
	// (index path built). Buffered() is the public witness for sidx.
	pathPass := 0
	pathNote := ""
	func() {
		small, err := video.OpenFile(resolveTestdata("vr5_seek.mp4"), video.Options{NowMs: (&handClock{}).at})
		if err != nil {
			pathNote = "小片打不开:" + err.Error()
			return
		}
		defer small.Close()
		if !small.Buffered() {
			pathNote = "小片走了流式,要走缓冲快路"
			return
		}
		pathPass++
		long, err := video.OpenFile(resolveTestdata("vr_stream_long.mp4"), video.Options{NowMs: (&handClock{}).at})
		if err != nil {
			pathNote = "长片打不开:" + err.Error()
			return
		}
		defer long.Close()
		if long.Buffered() {
			pathNote = "长片走了缓冲,要走索引流式"
			return
		}
		pathPass++
	}()
	if pathNote == "" {
		pathNote = "小缓冲长索引"
	}
	ev.Groups = append(ev.Groups, s8Group{Name: "path", Passed: pathPass, Total: 2, Note: pathNote})

	for _, g := range ev.Groups {
		ev.Passed += g.Passed
		ev.Total += g.Total
	}
	ev.Failed = ev.Total - ev.Passed
	if ev.Failed < 0 {
		ev.Failed = 0
	}
	if ev.ErrText == "" {
		for _, g := range ev.Groups {
			if g.Passed != g.Total {
				ev.ErrText = g.Name + ":" + g.Note
				break
			}
		}
	}
	return ev
}

func (ev s8Evidence) infoLines() []string {
	var out []string
	for _, g := range ev.Groups {
		out = append(out, g.Name+" "+itoa(g.Passed)+"/"+itoa(g.Total)+" "+g.Note)
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

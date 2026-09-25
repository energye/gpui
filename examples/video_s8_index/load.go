package main

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/energye/gpui/video"
)

// S8 gate clips, ffmpeg backend: every clip streams (no Go seek index;
// ffmpeg owns demux + seek natively). Same two files as the player-wiring
// gate, so the window never drifts from it. The window pins echo landing
// plus visible recovery.

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

type handClock struct{ now int64 }

func (h *handClock) at() int64 { return h.now }

// checkSeekOne pins one jump on the echo contract: SeekTo lands the
// target itself; SeekInfo echoes it back; the tail shows promptly from
// at/after the landing (never black, never backwards). The first shown
// stamp may lag by reorder delay (stamps[0] >= landed, monotonic,
// stamps[0]-landed <= 800ms).
func checkSeekOne(clip string, target int64) (landed, keyMs, delta, forward int64, shown bool, err error) {
	path := resolveTestdata(clip)
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
	if landed != target {
		return landed, 0, 0, 0, false, fmt.Errorf("落点%d要回声%d", landed, target)
	}
	_, _, _, keyMs, delta, forward = p.SeekInfo()
	if keyMs != target || delta != 0 || forward != 1 {
		return landed, keyMs, delta, forward, false, fmt.Errorf("回声证据不对(键%d差%d前解%d)", keyMs, delta, forward)
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
		seekNote = "落点全对回声"
	}
	if showNote == "" {
		showNote = "跳后尾帧即现"
	}
	ev.Groups = append(ev.Groups, s8Group{Name: "seek", Passed: seekPass, Total: len(targets), Note: seekNote})
	ev.Groups = append(ev.Groups, s8Group{Name: "show", Passed: showPass, Total: len(targets), Note: showNote})
	ev.ForwardMax = forwardMax

	// Group path: every clip streams on the ffmpeg backend (no Go seek
	// index; ffmpeg owns seek). Buffered() stays false as the witness.
	pathPass := 0
	pathNote := ""
	func() {
		small, err := video.OpenFile(resolveTestdata("vr5_seek.mp4"), video.Options{NowMs: (&handClock{}).at})
		if err != nil {
			pathNote = "小片打不开:" + err.Error()
			return
		}
		defer small.Close()
		if small.Buffered() {
			pathNote = "小片走了缓冲,要走流式"
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
			pathNote = "长片走了缓冲,要走流式"
			return
		}
		pathPass++
	}()
	if pathNote == "" {
		pathNote = "全片流式"
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

package main

import (
	"fmt"
	"os"

	govideo "github.com/energye/gpui/video"
)

// seekState is one VR5 gate: open a real clip, jump to a time, verify the
// landing (keyframe + forward decode proven pixel-exact inside SeekTo),
// then confirm the picture shows at once (no black).
type seekState struct {
	name     string
	path     string
	targetMs int64
	info     govideo.Info
	landedMs int64
	keyMs    int64
	deltaMs  int64
	forward  int64
	recover  bool
	err      error
}

func seekOne(name, mp4Path string, targetMs int64) *seekState {
	st := &seekState{name: name, path: mp4Path, targetMs: targetMs}
	p, err := govideo.OpenFile(mp4Path, govideo.Options{})
	if err != nil {
		st.err = fmt.Errorf("打不开: %w", err)
		return st
	}
	defer p.Close()
	st.info = p.Info()
	landed, err := p.SeekTo(targetMs)
	if err != nil {
		st.err = fmt.Errorf("跳不动: %w", err)
		return st
	}
	st.landedMs = landed
	_, _, _, key, delta, forward := p.SeekInfo()
	st.keyMs, st.deltaMs, st.forward = key, delta, forward
	// Recovery: the landed frame must be due on the very next poll
	// (instant, well inside the 3s budget) — otherwise black screen.
	if f, _ := p.Poll(); f != nil && f.PTSMs == landed {
		st.recover = true
	} else {
		st.err = fmt.Errorf("跳后黑屏: 目标%d 落点%d 没立刻播出来", targetMs, landed)
	}
	return st
}

func resolveClip(name string) string {
	for _, p := range []string{"video/testdata/" + name, "../../video/testdata/" + name} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "video/testdata/" + name
}

func loadSeek() []*seekState {
	return []*seekState{
		seekOne("双关键帧跳第二GOP", resolveClip("vr5_seek.mp4"), 1800),
		seekOne("双关键帧跳第一GOP", resolveClip("vr5_seek.mp4"), 600),
		seekOne("B帧重排96x96", resolveClip("vr2_m_bframes.mp4"), 800),
		seekOne("480p裁边", resolveClip("vr2_480p.mp4"), 800),
	}
}

func (st *seekState) infoLine() string {
	if st.err != nil {
		return fmt.Sprintf("%s 跳失败：%s", st.name, shortErr(st.err.Error(), 44))
	}
	return fmt.Sprintf("%s 目标%d 落点%d 键%d 前解%d 恢复%v", st.name,
		st.targetMs, st.landedMs, st.keyMs, st.forward, st.recover)
}

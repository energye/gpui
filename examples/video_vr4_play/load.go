package main

import (
	"fmt"
	"os"

	govideo "github.com/energye/gpui/video"
)

// playState is one VR4 gate clip played end to end through the real
// player: open, poll by wall clock, count frames. Numbers always come
// from the real play, never hand-written.
type playState struct {
	name      string
	path      string
	info      govideo.Info
	decoded   int64
	shown     int64
	dropped   int64
	decodeAvg float64
	decodeP95 float64
	driftMs   int64
	ended     bool
	firstPix  []byte
	firstW    int
	firstH    int
	err       error
}

func playOne(name, mp4Path string) *playState {
	st := &playState{name: name, path: mp4Path}
	p, err := govideo.OpenFile(mp4Path, govideo.Options{Loop: true})
	if err != nil {
		st.err = fmt.Errorf("打不开: %w", err)
		return st
	}
	defer p.Close()
	st.info = p.Info()
	// Let the 5-frame 5fps clip loop a few passes; the wall clock paces
	// display, so Shown grows in real time.
	deadline := govideo.WallDeadline(6000)
	for {
		f, _ := p.Poll()
		if f != nil && st.firstPix == nil {
			st.firstPix = append([]byte(nil), f.Pix...)
			st.firstW, st.firstH = f.Width, f.Height
		}
		if govideo.WallPast(deadline) {
			break
		}
		govideo.WallSleep(5)
	}
	s := p.Stats()
	st.decoded, st.shown, st.dropped = s.Decoded, s.Shown, s.Dropped
	st.decodeAvg, st.decodeP95, st.driftMs = s.DecodeMsAvg, s.DecodeMsP95, s.DriftMs
	st.ended = s.Ended
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

func loadPlay() []*playState {
	return []*playState{
		playOne("B帧重排96x96", resolveClip("vr2_m_bframes.mp4")),
		playOne("480p裁边", resolveClip("vr2_480p.mp4")),
		playOne("720p", resolveClip("vr2_720p.mp4")),
	}
}

func (st *playState) infoLine() string {
	if st.err != nil {
		return fmt.Sprintf("%s 播放失败：%s", st.name, shortErr(translatePlayError(st.err.Error()), 44))
	}
	return fmt.Sprintf("%s %s 解码%d 显示%d 丢%d 耗时%.2f毫秒", st.name, st.info.Profile,
		st.decoded, st.shown, st.dropped, st.decodeAvg)
}

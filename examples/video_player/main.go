// Command video_player is a clickable playback demo, not a gate window.
//
//	go run ./examples/video_player [clip.mp4]
//
// Window: 1200x800, stays open until you close it (RUN_SECONDS only caps
// automated runs). Opens one clip through the public video API, shows it
// as one scene rect, and lets you play/pause/seek with mouse + keyboard.
// No JSON gate, no FAIL lines: this is the usage sample, the VR/VC
// windows next door are the ones that judge.
//
// Controls:
//   - Space or P, or click the left button: play / pause.
//   - Left / Right: seek -/+ 1s. Home / End: head / tail. R: replay.
//   - Click the progress bar: jump there. Q or Esc: quit.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"
	govideo "github.com/energye/gpui/video"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

// Layout in window logical px (hit math below uses the same numbers).
const (
	videoX, videoY   = 120.0, 90.0
	videoW, videoH   = 960.0, 540.0
	playX, playY     = 120.0, 650.0
	playW, playH     = 110.0, 36.0
	replayX, replayY = 240.0, 650.0
	replayW, replayH = 110.0, 36.0
	barX, barY       = 120.0, 700.0
	barW, barH       = 960.0, 14.0
)

// X11 keysyms for non-printable keys (Wayland xkb uses the same values).
const (
	keyEscape = 65307
	keyHome   = 65360
	keyLeft   = 65361
	keyUp     = 65362
	keyRight  = 65363
	keyDown   = 65364
	keyEnd    = 65367
)

func main() {
	wrkit.EnsureUIFace()

	clip := clipArg()
	_, _, probeErr := govideo.ProbeFile(clip)
	player, openErr := govideo.OpenFile(clip, govideo.Options{})
	if player != nil {
		defer player.Close()
	}

	root := rendering.NewAbsoluteBox(winW, winH)
	root.Background = &rendering.Color{R: 0.08, G: 0.09, B: 0.11, A: 1}
	root.Place(wrkit.Label("视频播放小样（能点能拖，只演示用法）", 20, 0.92, 0.94, 0.98), 20, 14)

	fileLabel := wrkit.Label("", 13, 0.72, 0.8, 0.9)
	root.Place(fileLabel, 20, 46)

	img := rendering.NewRenderImage(videoW, videoH)
	root.Place(img, videoX, videoY)

	playBox := rendering.NewRenderColorBox(playW, playH, 0.2, 0.5, 0.9, 1)
	root.Place(playBox, playX, playY)
	playLabel := wrkit.Label("暂停", 14, 0.95, 0.97, 1)
	root.Place(playLabel, playX+34, playY+9)

	replayBox := rendering.NewRenderColorBox(replayW, replayH, 0.25, 0.28, 0.33, 1)
	root.Place(replayBox, replayX, replayY)
	root.Place(wrkit.Label("重播", 14, 0.9, 0.93, 0.96), replayX+34, replayY+9)

	statusLabel := wrkit.Label("", 13, 0.75, 0.85, 0.9)
	root.Place(statusLabel, 370, 658)

	barBg := rendering.NewRenderColorBox(barW, barH, 0.2, 0.22, 0.26, 1)
	root.Place(barBg, barX, barY)
	barFill := rendering.NewRenderColorBox(1, barH, 0.3, 0.8, 0.5, 1)
	root.Place(barFill, barX, barY)

	timeLabel := wrkit.Label("", 12, 0.72, 0.8, 0.9)
	root.Place(timeLabel, barX, barY+22)
	root.Place(wrkit.Label("空格/P=播/停 ←/→=±1秒 Home/End=头/尾 R=重播 点进度条=跳 Q=退出", 12, 0.55, 0.65, 0.75), barX, barY+46)

	st := &state{clip: clip}
	if player != nil {
		st.player = player
		st.info = player.Info()
		st.durMs = st.info.DurMs
		if st.durMs <= 0 && st.info.Frames > 0 {
			st.durMs = int64(st.info.Frames) * 200
		}
		var bufErr error
		st.buf, bufErr = render.NewImageBuf(st.info.Width, st.info.Height, render.FormatRGBA8)
		if bufErr != nil {
			st.fatal(fmt.Sprintf("显存建不起：%v", bufErr))
		}
		fileLabel.SetText(fmt.Sprintf("%s  %dx%d  %.1ffps  %s  %d帧  %s/%s",
			shortName(clip), st.info.Width, st.info.Height, st.info.FrameRate,
			fmtMs(st.durMs), st.info.Frames, st.info.Container, st.info.Codec))
		st.status("播放中")
		playLabel.SetText("暂停")
	} else {
		msg := fmt.Sprintf("打不开：%v", openErr)
		if probeErr != nil && openErr == nil {
			msg = fmt.Sprintf("打不开：%v", probeErr)
		}
		if openErr != nil {
			msg = fmt.Sprintf("打不开：%s", govideo.Classify(openErr).Readable())
		}
		fileLabel.SetText(shortName(clip))
		st.fatal(msg)
		statusLabel.SetText(msg)
		statusLabel.SetColor(0.95, 0.4, 0.35, 1)
		timeLabel.SetText(fmt.Sprintf("支持：盒子%v 编码%v", govideo.SupportedContainers(), govideo.SupportedCodecs()))
		playLabel.SetText("无片")
	}
	st.fileLabel, st.statusLabel = fileLabel, statusLabel
	st.timeLabel, st.playLabel = timeLabel, playLabel
	st.img, st.barFill = img, barFill

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui video_player — 完整播放小样", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "窗口打开失败(需要真机窗口):", err)
		os.Exit(1)
	}
	host := win.Host()

	// RUN_SECONDS caps automated runs only; unset = stay until close.
	runFor := time.Duration(0)
	if secs, set := wrkit.RunSecondsOpt(); set {
		runFor = time.Duration(secs) * time.Second
	}
	var app *embedder.PipelineApp
	app = embedder.NewPipelineApp(host, root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: runFor,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "video_player: 关闭 (%s)\n", win.Backend())
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					root.FixedWidth, root.FixedHeight = float64(ev.Width), float64(ev.Height)
					root.MarkNeedsLayout()
				}
				app.ScheduleFrame()
			case platform.EventPointer:
				if ev.Pointer == platform.PointerDown && ev.Button == 1 {
					st.click(ev.X, ev.Y, app)
				}
				app.ScheduleFrame()
			case platform.EventKey:
				if st.key(ev, app, win) {
					app.ScheduleFrame()
				}
			}
		},
	})

	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		_ = dt
		app.ScheduleFrame()
		st.tick()
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "打开失败:", err)
		os.Exit(1)
	}
	// Re-anchor the clock onto the head frame now that setup is done:
	// the player clock starts at OpenFile, and window setup costs a few
	// hundred ms — on a 5fps clip that already eats the head frame
	// (PollDue skips stale). Seeking to 0 right before Run re-anchors
	// so the first shown frame is frame 0, like the VC0 gate (5/5).
	if st.player != nil && st.bad == "" {
		if landed, err := st.player.SeekTo(0); err == nil {
			st.lastPTS = 0
			_ = landed
			st.refreshTime()
		}
	}
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "运行失败:", err)
		os.Exit(1)
	}
	elapsed := time.Since(t0).Seconds()
	presents := app.PresentCount()
	fps := 0.0
	if elapsed > 0.001 {
		fps = float64(presents) / elapsed
	}
	app.Close()
	win.Close()
	fmt.Fprintf(os.Stderr, "video_player: 关窗 %s 显示=%d 上屏=%d fps=%.1f 用时=%.1fs\n",
		shortName(clip), st.shown, presents, fps, elapsed)
}

// state is the demo playback state. All player calls happen on the UI
// loop thread (events + ticker share it), so no extra lock is needed.
type state struct {
	clip   string
	player *govideo.Player
	info   govideo.Info
	durMs  int64
	buf    *render.ImageBuf

	paused  bool
	ended   bool
	lastPTS int64
	shown   int64
	note    string
	bad     string

	img                      *rendering.RenderImage
	barFill                  *rendering.RenderColorBox
	fileLabel, statusLabel  *rendering.RenderText
	timeLabel, playLabel    *rendering.RenderText
}

func (st *state) fatal(msg string) {
	st.bad = msg
	st.note = msg
}

func (st *state) status(s string) {
	if st.bad != "" {
		return
	}
	st.note = s
	st.refreshStatus()
}

func (st *state) refreshStatus() {
	if st.statusLabel == nil {
		return
	}
	if st.bad != "" {
		st.statusLabel.SetText(st.bad)
		return
	}
	s := st.note
	if st.player == nil {
		st.statusLabel.SetText(s)
		return
	}
	stats := st.player.Stats()
	extra := fmt.Sprintf("解码=%d 显示=%d 丢=%d 队列=%d", stats.Decoded, stats.Shown, stats.Dropped, stats.QueueDepth)
	if s == "" {
		st.statusLabel.SetText(extra)
	} else {
		st.statusLabel.SetText(s + "  " + extra)
	}
}

func (st *state) refreshTime() {
	if st.timeLabel == nil || st.barFill == nil {
		return
	}
	cur := st.lastPTS
	if cur < 0 {
		cur = 0
	}
	if st.durMs > 0 && cur > st.durMs {
		cur = st.durMs
	}
	st.timeLabel.SetText(fmt.Sprintf("%s / %s  (%d帧)", fmtMs(cur), fmtMs(st.durMs), st.shown))
	ratio := 0.0
	if st.durMs > 0 {
		ratio = float64(cur) / float64(st.durMs)
	}
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	st.barFill.Width = 1 + ratio*(barW-1)
	st.barFill.MarkNeedsLayout()
}

// toggle pauses, resumes, or replays from the end.
func (st *state) toggle() {
	p := st.player
	if p == nil || st.bad != "" {
		return
	}
	if st.ended {
		if _, err := p.SeekTo(0); err != nil {
			st.status("重播失败：" + errTail(err))
			return
		}
		st.ended = false
		st.paused = false
		p.Resume()
		st.status("播放中")
		if st.playLabel != nil {
			st.playLabel.SetText("暂停")
		}
		return
	}
	if st.paused {
		st.paused = false
		p.Resume()
		st.status("播放中")
		if st.playLabel != nil {
			st.playLabel.SetText("暂停")
		}
	} else {
		st.paused = true
		p.Pause()
		st.status("已暂停")
		if st.playLabel != nil {
			st.playLabel.SetText("播放")
		}
	}
}

func (st *state) replay() {
	p := st.player
	if p == nil || st.bad != "" {
		return
	}
	if _, err := p.SeekTo(0); err != nil {
		st.status("重播失败：" + errTail(err))
		return
	}
	st.ended = false
	st.paused = false
	p.Resume()
	st.status("播放中")
	if st.playLabel != nil {
		st.playLabel.SetText("暂停")
	}
}

// seekTo jumps to targetMs, clamped into the clip.
func (st *state) seekTo(target int64) {
	p := st.player
	if p == nil || st.bad != "" {
		return
	}
	if st.durMs > 0 {
		if target < 0 {
			target = 0
		}
		if target >= st.durMs {
			target = st.durMs - 1
		}
	}
	landed, err := p.SeekTo(target)
	if err != nil {
		st.status("跳失败：" + errTail(err))
		return
	}
	st.ended = false
	_, _, _, _, delta, fwd := p.SeekInfo()
	st.lastPTS = landed
	st.status(fmt.Sprintf("跳到%s", fmtMs(landed)))
	_ = delta
	_ = fwd
	st.refreshTime()
}

// tick polls one due frame and paints it. Pause freezes via the player
// clock, so polling while paused just yields nil and holds the picture.
func (st *state) tick() {
	p := st.player
	if p == nil || st.bad != "" || st.buf == nil {
		return
	}
	f, ended := p.Poll()
	if f != nil {
		st.shown++
		st.lastPTS = f.PTSMs
		fastBlitRGBA(st.buf, f.Pix, f.Width, f.Height)
		if st.img != nil {
			st.img.SetImageShared(st.buf)
		}
		if st.ended {
			st.ended = false
		}
		st.refreshTime()
		st.refreshStatus()
	}
	if ended && !st.ended {
		st.ended = true
		st.status("播完（空格/R重播）")
		st.refreshTime()
		if st.playLabel != nil {
			st.playLabel.SetText("重播")
		}
	}
}

// click routes left-button hits: buttons first, then the progress bar.
func (st *state) click(x, y float64, app *embedder.PipelineApp) {
	_ = app
	if inside(x, y, playX, playY, playW, playH) {
		st.toggle()
		return
	}
	if inside(x, y, replayX, replayY, replayW, replayH) {
		st.replay()
		return
	}
	if st.player == nil || st.bad != "" || st.durMs <= 0 {
		return
	}
	if inside(x, y, barX, barY-8, barW, barH+16) {
		ratio := (x - barX) / barW
		st.seekTo(int64(ratio * float64(st.durMs)))
	}
}

// key routes keyboard. True = state changed, schedule a frame.
func (st *state) key(ev platform.Event, app *embedder.PipelineApp, win *platform.Window) bool {
	_ = app
	if !ev.Pressed {
		return false
	}
	if ev.Rune == 'q' || ev.Rune == 'Q' || ev.KeyCode == keyEscape {
		win.Close()
		return false
	}
	if ev.Rune == ' ' || ev.KeyCode == ' ' || ev.Rune == 'p' || ev.Rune == 'P' {
		if ev.Repeat {
			return false
		}
		st.toggle()
		return true
	}
	if ev.Rune == 'r' || ev.Rune == 'R' {
		if ev.Repeat {
			return false
		}
		st.replay()
		return true
	}
	step := int64(0)
	switch ev.KeyCode {
	case keyLeft:
		step = -1000
	case keyRight:
		step = 1000
	case keyUp:
		step = 5000
	case keyDown:
		step = -5000
	case keyHome:
		st.seekTo(0)
		return true
	case keyEnd:
		st.seekTo(st.durMs - 200)
		return true
	}
	if step != 0 {
		st.seekTo(st.lastPTS + step)
		return true
	}
	return false
}

// fastBlitRGBA copies a player frame into the display buffer row by row
// (one copy per row instead of w*h SetRGBA calls) and flags it for GPU
// reupload. Window side only; video core never imports render.
func fastBlitRGBA(dst *render.ImageBuf, pix []byte, w, h int) {
	if dst == nil || len(pix) < w*h*4 {
		return
	}
	data := dst.Data()
	rowLen := w * 4
	if len(data) >= h*rowLen {
		for y := 0; y < h; y++ {
			copy(data[y*rowLen:(y+1)*rowLen], pix[y*rowLen:(y+1)*rowLen])
		}
		dst.MarkPixelsDirty()
		dst.InvalidatePremulCache()
		return
	}
	for yy := 0; yy < h; yy++ {
		for xx := 0; xx < w; xx++ {
			o := (yy*w + xx) * 4
			_ = dst.SetRGBA(xx, yy, pix[o], pix[o+1], pix[o+2], pix[o+3])
		}
	}
	dst.MarkPixelsDirty()
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

func inside(x, y, rx, ry, rw, rh float64) bool {
	return x >= rx && x < rx+rw && y >= ry && y < ry+rh
}

func fmtMs(ms int64) string {
	if ms < 0 {
		ms = 0
	}
	s := ms / 1000
	m := s / 60
	s %= 60
	return fmt.Sprintf("%d:%02d.%03d", m, s, ms%1000)
}

func errTail(err error) string {
	s := err.Error()
	r := []rune(s)
	if len(r) > 60 {
		return string(r[:60]) + "..."
	}
	return s
}

func shortName(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[i+1:]
		}
	}
	return p
}

// clipArg picks the clip: argv[1], then $CLIP, then the bundled 720p.
func clipArg() string {
	if len(os.Args) > 1 && os.Args[1] != "" {
		return os.Args[1]
	}
	if v := os.Getenv("CLIP"); v != "" {
		return v
	}
	for _, p := range []string{"video/testdata/vr2_720p.mp4", "../../video/testdata/vr2_720p.mp4"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "video/testdata/vr2_720p.mp4"
}

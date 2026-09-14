// Command video_player is a clickable playback demo, not a gate window.
//
//	go run ./examples/video_player [clip.mp4]
//
// Window: 1200x800, stays open until you close it (RUN_SECONDS only caps
// automated runs). Opens one clip through the public video API, shows it
// as one scene rect, and exposes the standard player controls:
// play/pause, progress scrub (drag = fast keyframe jumps, release =
// exact landing), rate 0.25x-4x, frame step, prev/next keyframe,
// relative jumps. No JSON gate, no FAIL lines: this is the usage sample,
// the VR/VC windows next door are the ones that judge.
//
// Controls:
//   - Space or P, or click 播/停: play / pause.
//   - O or click 打开: pick a file (zenity/kdialog) or drag a file in.
//   - Progress bar: press-move = SeekFast scrub, release = SeekTo exact.
//   - Left / Right: seek -/+ 1s (SeekBy). Home / End: head / tail.
//   - R: replay. . : frame step. , / . : prev/next keyframe.
//   - 1/2/3/4: rate 0.5x/1x/2x/4x (Shift+1 = 0.25x).
//   - Q or Esc: quit.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
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

// Layout defaults for 1200x800 (hit math uses st.* at runtime, these are
// just the first frame before applyLayout runs).
const (
	videoX, videoY   = 120.0, 90.0
	videoW, videoH   = 960.0, 540.0
	playW, playH     = 110.0, 36.0
	replayW, replayH = 110.0, 36.0
	openW, openH     = 110.0, 36.0
	barH             = 14.0
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
	root.Place(playBox, 120, 650)
	playLabel := wrkit.Label("暂停", 14, 0.95, 0.97, 1)
	root.Place(playLabel, 120+34, 650+9)

	replayBox := rendering.NewRenderColorBox(replayW, replayH, 0.25, 0.28, 0.33, 1)
	root.Place(replayBox, 240, 650)
	replayLabel := wrkit.Label("重播", 14, 0.9, 0.93, 0.96)
	root.Place(replayLabel, 240+34, 650+9)

	openBox := rendering.NewRenderColorBox(openW, openH, 0.16, 0.42, 0.32, 1)
	root.Place(openBox, 360, 650)
	openLabel := wrkit.Label("打开", 14, 0.95, 0.97, 1)
	root.Place(openLabel, 360+34, 650+9)

	statusLabel := wrkit.Label("", 13, 0.75, 0.85, 0.9)
	root.Place(statusLabel, 490, 658)

	barBg := rendering.NewRenderColorBox(videoW, barH, 0.2, 0.22, 0.26, 1)
	root.Place(barBg, 120, 700)
	barFill := rendering.NewRenderColorBox(1, barH, 0.3, 0.8, 0.5, 1)
	root.Place(barFill, 120, 700)

	timeLabel := wrkit.Label("", 12, 0.72, 0.8, 0.9)
	root.Place(timeLabel, 120, 722)
	helpLabel := wrkit.Label("空格=播/停 ←/→=±1秒 ,/.=上下关键帧 .=单步 1/2/3/4=0.5/1/2/4x 拖条=快 scrub Q=退出", 12, 0.55, 0.65, 0.75)
	root.Place(helpLabel, 120, 746)

	st := &state{clip: clip, rateIdx: 1}
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
	st.root = root
	st.playBox, st.replayBox, st.barBg = playBox, replayBox, barBg
	st.openBox = openBox
	st.replayLabel, st.helpLabel = replayLabel, helpLabel
	st.openLabel = openLabel
	st.applyLayout(float64(winW), float64(winH))

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
					st.applyLayout(float64(ev.Width), float64(ev.Height))
				}
				app.ScheduleFrame()
			case platform.EventPointer:
				switch ev.Pointer {
				case platform.PointerDown:
					if ev.Button == 1 {
						st.press(ev.X, ev.Y)
					}
				case platform.PointerMove:
					st.drag(ev.X)
				case platform.PointerUp:
					if ev.Button == 1 {
						st.release(ev.X)
					}
				}
				app.ScheduleFrame()
			case platform.EventDrop:
				if len(ev.Files) > 0 {
					st.openPath(cleanDropPath(ev.Files[0]))
				} else {
					st.status("拖进来的东西拿不到路径，换个文件管理器再拖一次")
				}
				app.ScheduleFrame()
			case platform.EventDragEnter, platform.EventDragOver:
				st.status("松手就换片播")
				app.ScheduleFrame()
			case platform.EventDragLeave:
				st.refreshStatus()
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

// state is the demo playback state. The engine SeekTo is async by
// design (repark-and-return, background forward discard), so every
// control calls it directly on the UI thread — no background patch, no
// generation, clicks never freeze.
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

	// Progress scrub: press-move fires fast keyframe jumps, release
	// fires the exact landing (standard scrub model).
	scrubbing bool
	rateIdx   int

	img                      *rendering.RenderImage
	barFill                  *rendering.RenderColorBox
	fileLabel, statusLabel  *rendering.RenderText
	timeLabel, playLabel    *rendering.RenderText
	root                     *rendering.AbsoluteBox
	playBox, replayBox, barBg *rendering.RenderColorBox
	openBox                  *rendering.RenderColorBox
	replayLabel, helpLabel    *rendering.RenderText
	openLabel                *rendering.RenderText
	// Current layout in window logical px (applyLayout owns these;
	// click/paint math reads them, never the defaults above).
	winW, winH                         float64
	videoX, videoY, videoW, videoH     float64
	playX, playY, replayX, replayY     float64
	openX, openY                       float64
	barX, barY, barW                   float64
	statusX, statusY, timeY, helpY     float64
}

// applyLayout fits the video into the window keeping its aspect ratio
// (letterbox, never stretched), and pins the controls to the bottom.
// Called once at startup and on every resize.
func (st *state) applyLayout(winW, winH float64) {
	if st == nil || st.root == nil {
		return
	}
	const side, top, bottomReserve = 20.0, 80.0, 150.0
	availW := winW - 2*side
	availH := winH - top - bottomReserve - 10
	if availW < 200 {
		availW = 200
	}
	if availH < 150 {
		availH = 150
	}
	srcW, srcH := float64(st.info.Width), float64(st.info.Height)
	if srcW <= 0 || srcH <= 0 {
		srcW, srcH = 1280, 720
	}
	aspect := srcW / srcH
	dispW, dispH := availW, availW/aspect
	if dispH > availH {
		dispH, dispW = availH, availH*aspect
	}
	st.winW, st.winH = winW, winH
	st.videoW, st.videoH = dispW, dispH
	st.videoX = side + (availW-dispW)/2
	st.videoY = top + (availH-dispH)/2
	st.playX, st.playY = side, winH-150
	st.replayX, st.replayY = side+120, winH-150
	st.openX, st.openY = side+240, winH-150
	st.statusX, st.statusY = side+370, winH-142
	st.barX, st.barY = side, winH-100
	st.barW = availW
	st.timeY, st.helpY = st.barY+22, st.barY+46

	if st.img != nil {
		st.img.Width, st.img.Height = dispW, dispH
		st.img.MarkNeedsLayout()
		st.root.Place(st.img, st.videoX, st.videoY)
	}
	if st.playBox != nil {
		st.root.Place(st.playBox, st.playX, st.playY)
	}
	if st.playLabel != nil {
		st.root.Place(st.playLabel, st.playX+34, st.playY+9)
	}
	if st.replayBox != nil {
		st.root.Place(st.replayBox, st.replayX, st.replayY)
	}
	if st.openBox != nil {
		st.root.Place(st.openBox, st.openX, st.openY)
	}
	if st.openLabel != nil {
		st.root.Place(st.openLabel, st.openX+34, st.openY+9)
	}
	if st.replayLabel != nil {
		st.root.Place(st.replayLabel, st.replayX+34, st.replayY+9)
	}
	if st.statusLabel != nil {
		st.root.Place(st.statusLabel, st.statusX, st.statusY)
	}
	if st.barBg != nil {
		st.barBg.Width = st.barW
		st.barBg.MarkNeedsLayout()
		st.root.Place(st.barBg, st.barX, st.barY)
	}
	if st.barFill != nil {
		st.root.Place(st.barFill, st.barX, st.barY)
	}
	if st.timeLabel != nil {
		st.root.Place(st.timeLabel, st.barX, st.timeY)
	}
	if st.helpLabel != nil {
		st.root.Place(st.helpLabel, st.barX, st.helpY)
	}
	st.refreshTime()
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
	extra := fmt.Sprintf("解码=%d 显示=%d 丢=%d 队列=%d %.1fx%s", stats.Decoded, stats.Shown, stats.Dropped, stats.QueueDepth, stats.Rate, seekingMark(stats.Seeking))
	if s == "" {
		st.statusLabel.SetText(extra)
	} else {
		st.statusLabel.SetText(s + "  " + extra)
	}
}

func seekingMark(v int) string {
	if v != 0 {
		return " 跳转中"
	}
	return ""
}

func (st *state) refreshTime() {
	if st.timeLabel == nil || st.barFill == nil {
		return
	}
	cur := st.lastPTS
	// While a seek travels, the progress knob already tracks the request
	// (standard scrub feel); the picture catches up via Poll.
	if st.player != nil && st.player.Seeking() {
		_, _, landed, _, _, _ := st.player.SeekInfo()
		cur = landed
	}
	if cur < 0 {
		cur = 0
	}
	if st.durMs > 0 && cur > st.durMs {
		cur = st.durMs
	}
	rate := 1.0
	if st.player != nil {
		rate = st.player.Rate()
	}
	st.timeLabel.SetText(fmt.Sprintf("%s / %s  (%d帧 %.1fx)", fmtMs(cur), fmtMs(st.durMs), st.shown, rate))
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
	st.barFill.Width = 1 + ratio*(st.barW-1)
	st.barFill.MarkNeedsLayout()
}

// toggle pauses, resumes, or replays from the end.
func (st *state) toggle() {
	p := st.player
	if p == nil || st.bad != "" {
		return
	}
	if st.ended {
		st.seekTo(0)
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
	if st.player == nil || st.bad != "" {
		return
	}
	st.seekTo(0)
}

// seekTo jumps to targetMs, clamped into the clip. The engine reparks
// and returns in milliseconds, so this runs directly on the UI thread.
func (st *state) seekTo(target int64) {
	p := st.player
	if p == nil || st.bad != "" {
		return
	}
	target = st.clampSeek(target)
	landed, err := p.SeekTo(target)
	if err != nil {
		st.status("跳失败：" + errTail(err))
		return
	}
	st.afterSeek(landed, "跳到%s")
}

// seekFastTo jumps keyframe-only (scrub drag): instant across huge GOPs.
func (st *state) seekFastTo(target int64) {
	p := st.player
	if p == nil || st.bad != "" {
		return
	}
	target = st.clampSeek(target)
	landed, err := p.SeekFast(target)
	if err != nil {
		st.status("跳失败：" + errTail(err))
		return
	}
	st.afterSeek(landed, "快移到%s")
}

func (st *state) clampSeek(target int64) int64 {
	if st.durMs > 0 {
		if target < 0 {
			target = 0
		}
		if target >= st.durMs {
			target = st.durMs - 1
		}
	}
	return target
}

// afterSeek refreshes the resumed line after any jump (exact or fast):
// the clock is already re-anchored, the picture arrives via Poll.
func (st *state) afterSeek(landed int64, format string) {
	st.ended = false
	st.paused = false
	st.player.Resume()
	st.lastPTS = landed
	st.status(fmt.Sprintf(format, fmtMs(landed)))
	if st.playLabel != nil {
		st.playLabel.SetText("暂停")
	}
	st.refreshTime()
	st.refreshStatus()
}

// setRate cycles the standard ladder 0.5x/1x/2x/4x (Shift+1 = 0.25x via key).
func (st *state) setRate(r float64) {
	p := st.player
	if p == nil || st.bad != "" {
		return
	}
	if err := p.SetRate(r); err != nil {
		st.status("变速失败：" + errTail(err))
		return
	}
	for i, v := range rateLadder {
		if v == r {
			st.rateIdx = i
		}
	}
	st.status(fmt.Sprintf("%.2fx", r))
	st.refreshTime()
	st.refreshStatus()
}

// rateLadder is the standard playback ladder (0.25x via Shift+1).
var rateLadder = []float64{0.5, 1, 2, 4}

func (st *state) cycleRate() {
	next := rateLadder[0]
	if st.rateIdx+1 < len(rateLadder) {
		next = rateLadder[st.rateIdx+1]
	}
	st.setRate(next)
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

// press routes left-button down: buttons act at once, the progress bar
// starts a scrub (fast jumps while held).
func (st *state) press(x, y float64) {
	if inside(x, y, st.playX, st.playY, playW, playH) {
		st.toggle()
		return
	}
	if inside(x, y, st.replayX, st.replayY, replayW, replayH) {
		st.replay()
		return
	}
	if inside(x, y, st.openX, st.openY, openW, openH) {
		st.promptOpen()
		return
	}
	if st.player == nil || st.bad != "" || st.durMs <= 0 {
		return
	}
	if inside(x, y, st.barX, st.barY-8, st.barW, barH+16) {
		st.scrubbing = true
		st.seekFastTo(st.barTarget(x))
	}
}

// drag fires fast keyframe jumps while the bar stays held (standard
// scrub: superseding requests, never a freeze).
func (st *state) drag(x float64) {
	if !st.scrubbing {
		return
	}
	if st.player == nil || st.bad != "" || st.durMs <= 0 {
		return
	}
	st.seekFastTo(st.barTarget(x))
}

// release ends a scrub with the exact landing frame.
func (st *state) release(x float64) {
	if !st.scrubbing {
		return
	}
	st.scrubbing = false
	if st.player == nil || st.bad != "" || st.durMs <= 0 {
		return
	}
	st.seekTo(st.barTarget(x))
}

func (st *state) barTarget(x float64) int64 {
	ratio := (x - st.barX) / st.barW
	return int64(ratio * float64(st.durMs))
}

// click routes left-button hits: kept for headless tests (buttons then
// the progress bar, exact landing like release).
func (st *state) click(x, y float64, app *embedder.PipelineApp) {
	_ = app
	if inside(x, y, st.playX, st.playY, playW, playH) {
		st.toggle()
		return
	}
	if inside(x, y, st.replayX, st.replayY, replayW, replayH) {
		st.replay()
		return
	}
	if inside(x, y, st.openX, st.openY, openW, openH) {
		st.promptOpen()
		return
	}
	if st.player == nil || st.bad != "" || st.durMs <= 0 {
		return
	}
	if inside(x, y, st.barX, st.barY-8, st.barW, barH+16) {
		ratio := (x - st.barX) / st.barW
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
	if ev.Rune == 'o' || ev.Rune == 'O' {
		if ev.Repeat {
			return false
		}
		st.promptOpen()
		return true
	}
	// Frame step (paused single step) and keyframe walk.
	if ev.Rune == '.' {
		if ev.Repeat {
			return false
		}
		st.stepFrame()
		return true
	}
	if ev.Rune == ',' {
		if ev.Repeat {
			return false
		}
		st.prevKeyframe()
		return true
	}
	// Rate ladder: 1/2/3/4 = 0.5x/1x/2x/4x (Shift+1 = 0.25x), 0 = cycle.
	if ev.Rune == '1' || ev.Rune == '2' || ev.Rune == '3' || ev.Rune == '4' || ev.Rune == '0' {
		if ev.Repeat {
			return false
		}
		switch ev.Rune {
		case '1':
			st.setRate(0.5)
		case '2':
			st.setRate(1)
		case '3':
			st.setRate(2)
		case '4':
			st.setRate(4)
		case '0':
			st.cycleRate()
		}
		return true
	}
	if ev.Rune == '!' {
		if ev.Repeat {
			return false
		}
		st.setRate(0.25)
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
		st.seekBy(step)
		return true
	}
	return false
}

// seekBy jumps relative to the current picture (J/L style rewind).
func (st *state) seekBy(delta int64) {
	p := st.player
	if p == nil || st.bad != "" {
		return
	}
	landed, err := p.SeekBy(delta)
	if err != nil {
		st.status("跳失败：" + errTail(err))
		return
	}
	st.afterSeek(landed, "跳到%s")
}

// stepFrame advances one frame while paused (frame-step key).
func (st *state) stepFrame() {
	p := st.player
	if p == nil || st.bad != "" {
		return
	}
	landed, err := p.StepFrame()
	if err != nil {
		st.status("单步失败：" + errTail(err))
		return
	}
	st.paused = true
	if st.playLabel != nil {
		st.playLabel.SetText("播放")
	}
	st.ended = false
	st.lastPTS = landed
	st.status(fmt.Sprintf("单步%s", fmtMs(landed)))
	st.refreshTime()
	st.refreshStatus()
}

// prevKeyframe / nextKeyframe walk the keyframe table.
func (st *state) prevKeyframe() {
	p := st.player
	if p == nil || st.bad != "" {
		return
	}
	landed, err := p.PrevKeyframe()
	if err != nil {
		st.status("跳失败：" + errTail(err))
		return
	}
	st.afterSeek(landed, "上个关键帧%s")
}

func (st *state) nextKeyframe() {
	p := st.player
	if p == nil || st.bad != "" {
		return
	}
	landed, err := p.NextKeyframe()
	if err != nil {
		st.status("跳失败：" + errTail(err))
		return
	}
	st.afterSeek(landed, "下个关键帧%s")
}

// openPath swaps the clip while the window stays open: probe first so a
// bad file never kills the current playback, then rebuild the display
// buffer for the new size and re-fit the layout (aspect may change).
// Runs on the UI loop thread (events share it with the ticker).
func (st *state) openPath(path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	if _, err := os.Stat(path); err != nil {
		st.status("找不到文件：" + shortName(path))
		return
	}
	if _, _, err := govideo.ProbeFile(path); err != nil {
		st.status("打不开：" + govideo.Classify(err).Readable())
		return
	}
	next, err := govideo.OpenFile(path, govideo.Options{})
	if err != nil {
		st.status("打不开：" + govideo.Classify(err).Readable())
		return
	}
	nbuf, err := render.NewImageBuf(next.Info().Width, next.Info().Height, render.FormatRGBA8)
	if err != nil {
		next.Close()
		st.status(fmt.Sprintf("显存建不起：%v", err))
		return
	}
	if st.player != nil {
		st.player.Close()
	}
	if st.buf != nil {
		st.buf.Dispose()
	}
	st.player, st.buf = next, nbuf
	st.clip = path
	st.info = next.Info()
	st.durMs = st.info.DurMs
	if st.durMs <= 0 && st.info.Frames > 0 {
		st.durMs = int64(st.info.Frames) * 200
	}
	st.paused, st.ended = false, false
	st.lastPTS, st.shown = 0, 0
	st.bad, st.note = "", ""
	if st.fileLabel != nil {
		st.fileLabel.SetText(fmt.Sprintf("%s  %dx%d  %.1ffps  %s  %d帧  %s/%s",
			shortName(path), st.info.Width, st.info.Height, st.info.FrameRate,
			fmtMs(st.durMs), st.info.Frames, st.info.Container, st.info.Codec))
	}
	if st.statusLabel != nil {
		st.statusLabel.SetColor(0.75, 0.85, 0.9, 1)
	}
	if st.playLabel != nil {
		st.playLabel.SetText("暂停")
	}
	if _, err := st.player.SeekTo(0); err == nil {
		st.refreshTime()
	}
	st.applyLayout(st.winW, st.winH)
	st.status("播放中")
}

// promptOpen asks the desktop for a file via zenity/kdialog when present.
// No CGO, no new dependency: plain os/exec from the example layer only.
// Cancel or missing tool is never an error, just a status hint.
func (st *state) promptOpen() {
	if path, ok := systemPickFile(); ok {
		st.openPath(path)
		return
	}
	st.status("没找到系统选片框，把片子直接拖进窗口，或按 O 再试")
}

// systemPickFile tries zenity then kdialog. ok=false means cancelled or
// no tool installed (caller shows the drag hint instead).
func systemPickFile() (path string, ok bool) {
	if _, err := exec.LookPath("zenity"); err == nil {
		cmd := exec.Command("zenity", "--file-selection",
			"--title=打开视频",
			"--file-filter=视频 | *.mp4 *.m4v *.mov",
			"--file-filter=全部文件 | *")
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err != nil {
			return "", false
		}
		if p := strings.TrimSpace(out.String()); p != "" {
			return p, true
		}
		return "", false
	}
	if _, err := exec.LookPath("kdialog"); err == nil {
		cmd := exec.Command("kdialog", "--title", "打开视频",
			"--getopenfilename", os.Getenv("HOME"),
			"*.mp4 *.m4v *.mov | 视频文件 (*.mp4 *.m4v *.mov)")
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err != nil {
			return "", false
		}
		if p := strings.TrimSpace(out.String()); p != "" {
			return p, true
		}
	}
	return "", false
}

// cleanDropPath defends against raw file:// URLs in case a backend ever
// passes one through unparsed (normal path: Files are already clean).
func cleanDropPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.Trim(p, "<>")
	if strings.HasPrefix(p, "file://") {
		p = strings.TrimPrefix(p, "file://")
	}
	return p
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

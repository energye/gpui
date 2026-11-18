//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package wrkit

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"time"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/raster"
)

// DirectApp 是「手动帧控」真窗骨架（O12 批 1a：framepresent / gpuframe / gputexture）。
//
// 与 PipelineApp 窗的区别：帧循环由骨架自己跑——platform.Open 打窗、
// embedder.OpenPresentTarget 建呈现链、场景帧用 Target.PresentWith*（内部走
// Context.PresentFramePlan → PresentFrameDamage*/Full 的真实 swapchain 视图）直接驱动。
// 被测能力就是 render 的呈现/GPU 帧直控面本身；骨架只做循环与记账，不替能力做决定。
//
// GL 线程纪律（C7/P2.4e，与 PipelineApp 同规）：建链、每次 present、快照重绘、
// WaitIdle/Close 全部经 raster.RunGLSync 上 GL 枢纽——GL 后端的 EGL 绑定被帧路径
// 持有，别的线程 MakeCurrent 会 EGL_BAD_ACCESS 静默 no-op。
//
// 证据链复用 wrkit 既有 PNG 基建：快照=停帧后的重绘 SavePNG（present 走零回读，
// present 后 pixmap 是陈旧的——pipeline_app.go 同款「合成重绘」语义）；
// 抓屏=GrabClientArea（live 窗真实上屏像素）；黄金图=CompareGoldenPNG 逐位。
type DirectApp struct {
	Win    *platform.Window
	Host   platform.Host
	Ctl    platform.WindowController
	Target *render.PresentTarget
	DC     *render.Context

	Title string

	// 自记呈现记账（喂 wrgate.BuildReport 的 PresentCount 与探针）。
	Presents    int
	FullCount   int
	IdleCount   int
	DamageCount int
	LastOutcome render.PresentOutcome
	PresentErrs int

	quit bool
}

// OpenDirect opens a platform window plus its render present target.
// The present chain is opened as one GL-hub job, mirroring PipelineApp.Open.
func OpenDirect(title string, w, h int) (*DirectApp, error) {
	win, err := platform.Open(platform.Options{Width: w, Height: h,
		Title: title, Decorations: true, Resizable: true})
	if err != nil {
		return nil, fmt.Errorf("directdrive: window open (needs_gpu_window): %w", err)
	}
	a := &DirectApp{
		Win:   win,
		Host:  win.Host(),
		Ctl:   win.Controls(),
		Title: title,
	}
	a.Target, err = embedder.OpenPresentTarget(win.Host())
	if err != nil {
		win.Close()
		return nil, fmt.Errorf("directdrive: open present target: %w", err)
	}
	if err := raster.RunGLSync(func() error {
		a.DC = a.Target.Context()
		if a.DC == nil {
			return errors.New("directdrive: present target has no context")
		}
		// 首帧必须全量（swapchain 缓冲未写），与 PipelineApp 的 WarmUp 同语义。
		return a.Target.PresentWith(func(dc *render.Context) {
			dc.SetRGBA(0.08, 0.09, 0.11, 1)
			dc.DrawRectangle(0, 0, float64(dc.Width()), float64(dc.Height()))
			_ = dc.Fill()
			dc.MarkFullRedraw()
		})
	}); err != nil {
		_ = raster.RunGLSync(func() error { _ = a.Target.Close(); return nil })
		win.Close()
		return nil, fmt.Errorf("directdrive: warm-up present: %w", err)
	}
	a.Presents++
	a.FullCount++
	return a, nil
}

// Quit requests loop exit (close event or scripted end).
func (a *DirectApp) Quit() { a.quit = true }

// QuitRequested reports whether Quit was called.
func (a *DirectApp) QuitRequested() bool { return a.quit }

// WaitEvents blocks for platform events (timeout < 0 = infinite).
func (a *DirectApp) WaitEvents(timeout time.Duration) []platform.Event {
	return a.Host.WaitEvents(timeout)
}

// PresentFull runs one forced full present (Target.PresentWith → Context.PresentFrameFull
// inside the present chain). One GL-hub job, synchronous.
func (a *DirectApp) PresentFull(draw func(dc *render.Context)) error {
	var err error
	gerr := raster.RunGLSync(func() error {
		err = a.Target.PresentWith(draw)
		a.LastOutcome = a.Target.LastPresentOutcome()
		return nil
	})
	if gerr != nil {
		return gerr
	}
	if err != nil {
		a.PresentErrs++
		return err
	}
	a.Presents++
	a.FullCount++
	return nil
}

// PresentAuto runs one plan-driven present (Target.PresentWithAuto → Context.PresentFramePlan:
// idle / damage_union / damage_multi / full dispatch on the real swapchain view).
// One GL-hub job, synchronous. Returns the plan's PresentOutcome.
func (a *DirectApp) PresentAuto(draw func(dc *render.Context)) (render.PresentOutcome, error) {
	var (
		out render.PresentOutcome
		err error
	)
	gerr := raster.RunGLSync(func() error {
		out, err = a.Target.PresentWithAuto(draw)
		a.LastOutcome = a.Target.LastPresentOutcome()
		return nil
	})
	if gerr != nil {
		return out, gerr
	}
	if err != nil {
		a.PresentErrs++
		return out, err
	}
	a.Presents++
	switch out.Mode {
	case render.PresentModeIdle:
		a.IdleCount++
	case render.PresentModeFull:
		a.FullCount++
	default:
		a.DamageCount++
	}
	return out, nil
}

// SnapshotRepaint repaints draw into the context and saves a PNG readback.
//
// 必须在停止 present 之后调用（关窗前的合成重绘）：present 走零回读路径，
// present 之后直接 Image()/SavePNG 拿到的是陈旧 pixmap——与 PipelineApp 的
// SnapshotPath「停帧重绘」同语义。One GL-hub job.
func (a *DirectApp) SnapshotRepaint(draw func(dc *render.Context), path string) error {
	return raster.RunGLSync(func() error {
		a.DC.BeginFrame()
		if draw != nil {
			draw(a.DC)
		}
		return a.DC.SavePNG(path)
	})
}

// Close tears down in the 纪律 order: WaitIdle → Target.Close → Win.Close.
// WaitIdle 等飞行中的 GPU 提交排空，再销毁 surface/swapchain（940MX NVIDIA
// Xorg 的 in-flight present 拆 surface 会崩 X server——AGENTS.md 已知事故基线）。
func (a *DirectApp) Close() error {
	var firstErr error
	_ = raster.RunGLSync(func() error {
		if err := a.Target.WaitIdle(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("directdrive: wait idle: %w", err)
		}
		if err := a.Target.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("directdrive: target close: %w", err)
		}
		return nil
	})
	a.Win.Close()
	return firstErr
}

// DumpDirectEnv prints the one-line GPU backend banner the evidence reports use.
func (a *DirectApp) GPUBackendName() string {
	if a.Target == nil {
		return ""
	}
	return a.Target.GPUBackend()
}

// RunGL runs fn on the GL hub and waits (offscreen GPU legs, dc state jobs).
func (a *DirectApp) RunGL(fn func()) {
	_ = raster.RunGLSync(func() error {
		fn()
		return nil
	})
}

// OSPresentStats reports the accumulated OS damage-present counters.
func (a *DirectApp) OSPresentStats() (partial, full uint64) {
	return a.Target.OSPresentStats()
}

// GrabLiveWindow grabs the live client area of the direct window (X11/xwd path).
// Returns (w, h, err); wrkit.ScreenUnsupported(err) marks environments without
// an X11 window to grab (native Wayland) — the evidence honestly skips.
func (a *DirectApp) GrabLiveWindow(outPath string) (int, int, error) {
	// Present* returns once the GPU submit is queued, but the X window shows the
	// new swapchain image only after the display consumes it (wgpu-native is
	// async; the GL hub is synchronous). Grabbing immediately read the PREVIOUS
	// image — canvascontract lost the final present's card text (screen diff
	// 11.07%, while a debug print's extra time made the same grab 0.31%). Drain
	// the queue, then let one 60Hz frame land before reading the live window.
	if a != nil && a.Target != nil {
		_ = a.Target.WaitIdle()
	}
	time.Sleep(34 * time.Millisecond)
	return GrabClientArea(a.Title, outPath)
}

// EnsureSnapDir mkdir -p the testdata dir for a window.
func EnsureSnapDir(dir string) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}
}

// ----------------------------------------
// PNG 区域对照（抓屏 vs 快照 / 抓屏 vs 抓屏）。逻辑坐标 rects，容差 ±2/通道。
// ----------------------------------------

// LoadPNG decodes a PNG file (evidence helper).
func LoadPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

// GrabDiffPct compares two PNGs over the rects (physical px). Returns -1 when
// either image is unavailable or the sizes disagree.
func GrabDiffPct(pathA, pathB string, rects []GoldenRect) float64 {
	a, err := LoadPNG(pathA)
	if err != nil {
		return -1
	}
	b, err := LoadPNG(pathB)
	if err != nil {
		return -1
	}
	ab, bb := a.Bounds(), b.Bounds()
	if ab.Dx() != bb.Dx() || ab.Dy() != bb.Dy() {
		return -1
	}
	var total, diffPx int64
	for _, gr := range rects {
		r := image.Rect(int(gr.X), int(gr.Y), int(gr.X+gr.W), int(gr.Y+gr.H))
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				ar, ag, ab2, aa := a.At(x, y).RGBA()
				br, bg, bb2, ba := b.At(x, y).RGBA()
				total++
				if chanDiff2(ar>>8, br>>8) || chanDiff2(ag>>8, bg>>8) ||
					chanDiff2(ab2>>8, bb2>>8) || chanDiff2(aa>>8, ba>>8) {
					diffPx++
				}
			}
		}
	}
	if total == 0 {
		return -1
	}
	return float64(diffPx) / float64(total) * 100
}

func chanDiff2(a, b uint32) bool {
	d := int(a) - int(b)
	if d < 0 {
		d = -d
	}
	return d > 2
}

// ----------------------------------------
// 直驱窗共用 chrome（1200×800：顶栏 + 图例 + 2×2 卡 + 底部动态状态带）。
// 三扇直驱窗同一骨架：卡片内容各窗自绘，Golden 区 = 顶栏+图例+四卡。
// ----------------------------------------

const (
	DWinW, DWinH = 1200.0, 800.0
	DTopH        = 48.0

	DLegX, DLegY, DLegW, DLegH = 12.0, 60.0, 260.0, 656.0
	DBodyX, DBodyY             = 284.0, 60.0
	DGap                       = 12.0
	DCardW                     = 446.0
	DCardH                     = 322.0

	DStatusY = 726.0 // 动态状态带顶（金区外）
)

// DirectCardAt returns the top-left of card i (0=A,1=B,2=C,3=D).
func DirectCardAt(i int) (float64, float64) {
	switch i {
	case 0:
		return DBodyX, DBodyY
	case 1:
		return DBodyX + DCardW + DGap, DBodyY
	case 2:
		return DBodyX, DBodyY + DCardH + DGap
	default:
		return DBodyX + DCardW + DGap, DBodyY + DCardH + DGap
	}
}

// DirectGoldenRects returns the golden compare region (status band excluded).
func DirectGoldenRects() []GoldenRect {
	rects := []GoldenRect{
		{X: 0, Y: 0, W: DWinW, H: DTopH},
		{X: DLegX, Y: DLegY, W: DLegW, H: DLegH},
	}
	for i := 0; i < 4; i++ {
		x, y := DirectCardAt(i)
		rects = append(rects, GoldenRect{X: x, Y: y, W: DCardW, H: DCardH})
	}
	return rects
}

var (
	DColBG      = [3]float64{0.08, 0.09, 0.11}
	DColTop     = [3]float64{0.13, 0.15, 0.19}
	DColLegend  = [3]float64{0.11, 0.12, 0.15}
	DColCardBG  = [3]float64{0.10, 0.11, 0.13}
	DColBorder  = [3]float64{0.32, 0.36, 0.44}
	DColTextDim = [3]float64{0.62, 0.66, 0.74}
	DColTextHi  = [3]float64{0.88, 0.90, 0.94}
)

// DirectFillRect fills a rect with a color at alpha (device-space helper).
func DirectFillRect(dc *render.Context, x, y, w, h float64, c [3]float64, a float64) {
	dc.SetRGBA(c[0], c[1], c[2], a)
	dc.DrawRectangle(x, y, w, h)
	_ = dc.Fill()
}

// DirectStrokeRect strokes a rect (device-space helper).
func DirectStrokeRect(dc *render.Context, x, y, w, h float64, lw float64, c [3]float64) {
	dc.SetRGBA(c[0], c[1], c[2], 1)
	dc.SetLineWidth(lw)
	dc.DrawRectangle(x, y, w, h)
	_ = dc.Stroke()
}

// DirectDrawText draws one label (caller supplies the cached face).
func DirectDrawText(dc *render.Context, s string, x, y float64, face text.Face, c [3]float64) {
	dc.SetFont(face)
	dc.SetRGBA(c[0], c[1], c[2], 1)
	dc.DrawString(s, x, y)
}

// PaintDirectChrome paints the top band + legend panel (static strings only).
func PaintDirectChrome(dc *render.Context, title string, legendLines []string, faceTitle, faceLeg text.Face) {
	DirectFillRect(dc, 0, 0, DWinW, DTopH, DColTop, 1)
	DirectDrawText(dc, title, 16, 30, faceTitle, DColTextHi)

	DirectFillRect(dc, DLegX, DLegY, DLegW, DLegH, DColLegend, 1)
	DirectStrokeRect(dc, DLegX+0.5, DLegY+0.5, DLegW-1, DLegH-1, 1, DColBorder)
	for i, ln := range legendLines {
		DirectDrawText(dc, ln, DLegX+10, DLegY+24+float64(i)*22, faceLeg, DColTextDim)
	}
}

// PaintDirectCardFrame paints card bg + border + title.
func PaintDirectCardFrame(dc *render.Context, i int, title string, faceCard text.Face) {
	x, y := DirectCardAt(i)
	DirectFillRect(dc, x, y, DCardW, DCardH, DColCardBG, 1)
	DirectStrokeRect(dc, x+0.5, y+0.5, DCardW-1, DCardH-1, 1, DColBorder)
	DirectDrawText(dc, title, x+12, y+22, faceCard, DColTextHi)
}

// PaintDirectStatus paints the dynamic status band (outside golden rects).
func PaintDirectStatus(dc *render.Context, lines []string, faceSmall text.Face) {
	DirectFillRect(dc, 0, DStatusY, DWinW, DWinH-DStatusY, DColBG, 1)
	for i, ln := range lines {
		DirectDrawText(dc, ln, DLegX, DStatusY+18+float64(i)*20, faceSmall, DColTextDim)
	}
}

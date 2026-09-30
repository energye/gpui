//go:build linux && !nogpu

package main

// P2-1 同进程双窗复现：第一扇鹈鹕窗走完在线链路并关窗后，同进程再开
// 第二扇，看是否堵在标题图块烘焙的字形管线（bakeTitleLine→bakeTile→
// Image→FlushGPU→ensureGlyphMaskPipeline）。任一阶段超时即打印全
// goroutine 栈后 FAIL，不干等 90 秒。无 DISPLAY 即 Skip，不假绿。

import (
	"bytes"
	"fmt"
	"runtime"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

func p2DumpStacks(t *testing.T, stage string) {
	t.Helper()
	var buf bytes.Buffer
	_ = pprof.Lookup("goroutine").WriteTo(&buf, 2)
	t.Logf("P2-1 repro stacks at stage %s:\n%s", stage, buf.String())
}

func p2RunWithTimeout(t *testing.T, stage string, d time.Duration, fn func() error) error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		done <- fn()
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(d):
		p2DumpStacks(t, stage)
		t.Fatalf("P2-1 repro stage %s timed out after %s (see goroutine dump above)", stage, d)
		return nil
	}
}

func p2PresentPelican(t *testing.T, w, h int) {
	t.Helper()
	xw := p1OpenX11(t, w, h)
	defer xw.close()

	pt, err := render.NewPresentTarget(render.PresentNativeSurface{
		Platform: render.PresentPlatformX11,
		Display:  xw.display,
		Window:   xw.window,
	}, 1200, 700, 1)
	if err != nil {
		t.Fatalf("NewPresentTarget(GL online): %v", err)
	}
	defer pt.Close()

	var sc *pelicanScene
	if err := p2RunWithTimeout(t, "bake-scene", 60*time.Second, func() error {
		sc = newPelicanScene(1200, 700)
		return nil
	}); err != nil {
		t.Fatalf("newPelicanScene: %v", err)
	}
	draw := func(dc *render.Context) {
		pc := rendering.NewPaintContext(dc, 1)
		size := rendering.Size{Width: stageW, Height: stageH}
		sc.paintStatic(pc, size)
		sc.paintStage(pc, size)
	}
	if err := p2RunWithTimeout(t, "present-2frames", 60*time.Second, func() error {
		for i := 0; i < 2; i++ {
			if err := pt.PresentWith(draw); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("PresentWith: %v", err)
	}
	stats := pt.Context().RenderPathStats()
	t.Logf("P2-1 repro window done %s backend=%s", stats.LogLine(), pt.GPUBackend())
	if stats.GPUOps == 0 {
		t.Fatalf("repro window drew nothing on GPU: %s", stats.LogLine())
	}
}

func TestP2TwoWindowRepro(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	t.Setenv("GPUI_BACKEND", "go")
	t.Setenv("PELICAN_T", "5")

	t.Log("P2-1 repro: opening first window")
	p2PresentPelican(t, 1200, 700)
	t.Log("P2-1 repro: first window closed, opening second window in same process")
	p2PresentPelican(t, 1200, 700)
	t.Log("P2-1 repro: second window done, no hang")
}

func TestP2TwoWindowSequentialWebGPU(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	t.Setenv("PELICAN_T", "5")

	t.Log("P2-1 repro WebGPU: opening first window")
	p2PresentPelican(t, 1200, 700)
	t.Log("P2-1 repro WebGPU: first window closed, opening second window in same process")
	p2PresentPelican(t, 1200, 700)
	t.Log("P2-1 repro WebGPU: second window done, no hang")
}

// TestP2TwoWindowOverlapped keeps two windows alive in the same process and
// presents them alternately. This is the real three-window shape (windows
// coexist); sequential open/close/open alone does not cover it.
func TestP2TwoWindowOverlapped(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	t.Setenv("GPUI_BACKEND", "go")
	t.Setenv("PELICAN_T", "5")

	xw1 := p1OpenX11(t, 1200, 700)
	defer xw1.close()
	var pt1 *render.PresentTarget
	if err := p2RunWithTimeout(t, "open-target-1", 60*time.Second, func() error {
		var err error
		pt1, err = render.NewPresentTarget(render.PresentNativeSurface{
			Platform: render.PresentPlatformX11,
			Display:  xw1.display,
			Window:   xw1.window,
		}, 1200, 700, 1)
		return err
	}); err != nil {
		t.Fatalf("NewPresentTarget 1: %v", err)
	}
	defer pt1.Close()

	xw2 := p1OpenX11(t, 1200, 700)
	defer xw2.close()
	var pt2 *render.PresentTarget
	if err := p2RunWithTimeout(t, "open-target-2", 60*time.Second, func() error {
		var err error
		pt2, err = render.NewPresentTarget(render.PresentNativeSurface{
			Platform: render.PresentPlatformX11,
			Display:  xw2.display,
			Window:   xw2.window,
		}, 1200, 700, 1)
		return err
	}); err != nil {
		t.Fatalf("NewPresentTarget 2: %v", err)
	}
	defer pt2.Close()

	var sc1, sc2 *pelicanScene
	if err := p2RunWithTimeout(t, "bake-scene-2-while-1-open", 60*time.Second, func() error {
		sc2 = newPelicanScene(1200, 700)
		return nil
	}); err != nil {
		t.Fatalf("newPelicanScene 2: %v", err)
	}
	sc1 = newPelicanScene(1200, 700)

	draw1 := func(dc *render.Context) {
		pc := rendering.NewPaintContext(dc, 1)
		size := rendering.Size{Width: stageW, Height: stageH}
		sc1.paintStatic(pc, size)
		sc1.paintStage(pc, size)
	}
	draw2 := func(dc *render.Context) {
		pc := rendering.NewPaintContext(dc, 1)
		size := rendering.Size{Width: stageW, Height: stageH}
		sc2.paintStatic(pc, size)
		sc2.paintStage(pc, size)
	}
	if err := p2RunWithTimeout(t, "alternate-present", 90*time.Second, func() error {
		for i := 0; i < 4; i++ {
			t.Logf("overlapped iter %d: presenting win1", i)
			if err := pt1.PresentWith(draw1); err != nil {
				return fmt.Errorf("win1 iter %d: %w", i, err)
			}
			t.Logf("overlapped iter %d: presenting win2", i)
			if err := pt2.PresentWith(draw2); err != nil {
				return fmt.Errorf("win2 iter %d: %w", i, err)
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("alternate PresentWith: %v", err)
	}
	t.Logf("P2-1 overlapped: win1 %s backend=%s", pt1.Context().RenderPathStats().LogLine(), pt1.GPUBackend())
	t.Logf("P2-1 overlapped: win2 %s backend=%s", pt2.Context().RenderPathStats().LogLine(), pt2.GPUBackend())
	if pt1.Context().RenderPathStats().GPUOps == 0 || pt2.Context().RenderPathStats().GPUOps == 0 {
		t.Fatalf("overlapped windows must both draw on GPU")
	}
}

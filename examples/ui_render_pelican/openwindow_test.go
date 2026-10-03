//go:build linux && !nogpu

package main

// 开窗钮（HUD 药丸右端 + 号）的命中与回调：纯逻辑断言，不开窗。
// 覆盖：点中 + 钮恰好开一个窗；暂停钮/滑条行为不变且不误开窗；
// 未接入回调时静默无动作；悬停态各自独立。

import (
	"testing"

	"github.com/energye/gpui/ui/platform"
)

func openTestScene(t *testing.T) *pelicanScene {
	t.Helper()
	sc := newPelicanScene(winW, winH)
	sc.relayout(winW, winH)
	return sc
}

func TestOpenButtonHitOpensOne(t *testing.T) {
	sc := openTestScene(t)
	var opened int
	sc.OnOpenWindow = func() { opened++ }
	ox, oy := sc.openBtnCenter()
	sc.onPointer(platform.Event{Type: platform.EventPointer, Pointer: platform.PointerDown, X: ox, Y: oy})
	if opened != 1 {
		t.Fatalf("open button press: opened = %d, want 1", opened)
	}
	if sc.sim.paused {
		t.Fatal("open button must not toggle pause")
	}
	if sc.draggingSlider {
		t.Fatal("open button must not start slider drag")
	}
}

func TestOpenButtonNilCallbackSilent(t *testing.T) {
	sc := openTestScene(t)
	sc.OnOpenWindow = nil
	ox, oy := sc.openBtnCenter()
	sc.onPointer(platform.Event{Type: platform.EventPointer, Pointer: platform.PointerDown, X: ox, Y: oy})
	// 无回调不断言计数，只要求不 panic、不改暂停/滑条状态。
	if sc.sim.paused || sc.draggingSlider {
		t.Fatal("nil callback press must change nothing")
	}
}

func TestPauseAndSliderUnaffected(t *testing.T) {
	sc := openTestScene(t)
	var opened int
	sc.OnOpenWindow = func() { opened++ }

	// 暂停钮：照常暂停，不开窗。
	bx, by := sc.btnCenter()
	was := sc.sim.paused
	sc.onPointer(platform.Event{Type: platform.EventPointer, Pointer: platform.PointerDown, X: bx, Y: by})
	if sc.sim.paused == was {
		t.Fatal("pause button must still toggle pause")
	}
	if opened != 0 {
		t.Fatal("pause button must not open a window")
	}

	// 滑条：照常调速，不开窗。
	sx := sc.sliderX0() + 10
	sy := sc.hudY + hudH/2
	speedBefore := sc.sim.speed
	sc.onPointer(platform.Event{Type: platform.EventPointer, Pointer: platform.PointerDown, X: sx, Y: sy})
	if !sc.draggingSlider {
		t.Fatal("slider press must start dragging")
	}
	if sc.sim.speed == speedBefore {
		t.Fatal("slider press must change speed")
	}
	if opened != 0 {
		t.Fatal("slider must not open a window")
	}
	sc.onPointer(platform.Event{Type: platform.EventPointer, Pointer: platform.PointerUp, X: sx, Y: sy})
	if sc.draggingSlider {
		t.Fatal("pointer up must stop slider drag")
	}
}

func TestOpenButtonHoverIndependent(t *testing.T) {
	sc := openTestScene(t)
	ox, oy := sc.openBtnCenter()
	sc.onPointer(platform.Event{Type: platform.EventPointer, Pointer: platform.PointerMove, X: ox, Y: oy})
	if !sc.openHover {
		t.Fatal("hovering the open button must set openHover")
	}
	if sc.btnHover {
		t.Fatal("open button hover must not set pause-button hover")
	}
	bx, by := sc.btnCenter()
	sc.onPointer(platform.Event{Type: platform.EventPointer, Pointer: platform.PointerMove, X: bx, Y: by})
	if !sc.btnHover {
		t.Fatal("hovering the pause button must set btnHover")
	}
	if sc.openHover {
		t.Fatal("pause button hover must clear openHover")
	}
}

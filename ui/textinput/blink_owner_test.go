package textinput

import (
	"testing"
	"time"

	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/rendering"
)

// 聚焦盒自注册进所属渲染树的 ticker 注册表，失焦或禁用不注册、零成本。
// 嵌道层泵经 owner.TickAllTickers 在 UI 线程喂真实流逝时间。
// 帧由脏态驱动，不由 ticker 活着驱动。
// 契约（scheduler.Ticker）：Tick 返回 false 表示不再继续驱动，注册表自动摘除。
func TestBlinkFocusRegistration(t *testing.T) {
	ed := New()
	box := NewMultiLineInputBox(ed, 880, 90, 16)
	root := rendering.NewAbsoluteBox(1200, 800)
	root.Place(box, 296, 312)
	owner := rendering.NewPipelineOwner(root)
	fm := focus.NewManager()
	fm.Register(box.FocusNode())
	if owner.HasTickers() {
		t.Fatal("unfocused box must not register for blink dt")
	}
	if !box.FocusNode().RequestFocus() {
		t.Fatal("focus request failed")
	}
	if !owner.HasTickers() {
		t.Fatal("focused box must register for blink dt")
	}
	// Fresh focus starts caret-on; one half period toggles it off.
	if !box.IsCaretOn() {
		t.Fatal("fresh focus must start caret-on")
	}
	owner.TickAllTickers(0.6)
	if box.IsCaretOn() {
		t.Fatal("caret must be off after one blink half-period")
	}
	owner.TickAllTickers(0.6)
	if !box.IsCaretOn() {
		t.Fatal("caret must be on after two half-periods")
	}
	// 契约断言：聚焦期间 Tick 持续返回 true，注册表不摘它。
	if !owner.HasTickers() {
		t.Fatal("focused box must stay registered while Tick keeps returning true")
	}
	// Disabling a focused box drops it: no dt, no frames for it.
	box.SetDisabled(true)
	if owner.HasTickers() {
		t.Fatal("disabled box must drop blink registration")
	}
}

// 大步长保持真奇偶：按截止睡住的循环醒来拿到真实流逝时间，
// 一次 Tick 可能跨过多个半周期。
// 契约合一后 Tick 返回「是否继续驱动」，不再表达「是否翻转」；
// 净可见变化改由 IsCaretOn 真值断言承载，不丢真值。
func TestBlinkParity_OversizedStep(t *testing.T) {
	ed := New()
	box := NewMultiLineInputBox(ed, 880, 90, 16)
	root := rendering.NewAbsoluteBox(1200, 800)
	root.Place(box, 296, 312)
	_ = rendering.NewPipelineOwner(root)
	fm := focus.NewManager()
	fm.Register(box.FocusNode())
	if !box.FocusNode().RequestFocus() {
		t.Fatal("focus request failed")
	}
	box.SetCaretOn(true)
	if !box.Tick(1.2) {
		t.Fatal("focused box must keep driving even when a step nets no toggle")
	}
	if !box.IsCaretOn() {
		t.Fatal("even toggles must leave caret on")
	}
	if !box.Tick(0.6) {
		t.Fatal("focused box must keep driving")
	}
	if box.IsCaretOn() {
		t.Fatal("caret must be off after odd toggles")
	}
}

// NextWake 跟随闪烁相位余量；未聚焦的盒子报不出截止点（false）。
func TestBlinkDeadline_TracksPhase(t *testing.T) {
	ed := New()
	box := NewMultiLineInputBox(ed, 880, 90, 16)
	if _, ok := box.NextWake(); ok {
		t.Fatal("unfocused box must report no deadline")
	}
	fm := focus.NewManager()
	fm.Register(box.FocusNode())
	if !box.FocusNode().RequestFocus() {
		t.Fatal("focus request failed")
	}
	box.SetCaretOn(true)
	if d, ok := box.NextWake(); !ok || d != 500*time.Millisecond {
		t.Fatalf("NextWake=%v,%v want 500ms,true", d, ok)
	}
	box.TickCaret(0.2)
	if d, ok := box.NextWake(); !ok || d < 290*time.Millisecond || d > 310*time.Millisecond {
		t.Fatalf("NextWake=%v,%v want ~300ms", d, ok)
	}
}

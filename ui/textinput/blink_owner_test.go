package textinput

import (
	"testing"
	"time"

	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/rendering"
)

// 聚焦盒自注册进所属渲染树的 ticker 注册表，失焦或禁用不注册、零成本。
// 嵌道层泵经 owner.TickAllTickers 在 UI 线程推进。
// 帧由脏态驱动，不由 ticker 活着驱动。
// 契约（scheduler.Ticker）：Tick 返回 false 表示不再继续驱动，注册表自动摘除。
//
// 闪烁相位是墙钟相位（对齐 Flutter EditableText reset 语义）：Tick 喂进的
// dt 不参与相位推进——重置（焦点/编辑）发生在间隔中段时，下一次 Tick 即使
// 拿到整段 slept 间隔，也不得把刚清零的相位推过翻转点。
// 回归背景：切换输入框后光标先隐藏约半秒才恢复闪烁（tickPump 喂自上次
// tick 以来的整段墙钟间隔，dispatch 中段的重置被一步冲掉）。
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
	// Fresh focus starts caret-on; a mid-gap reset must survive a whole-gap
	// tick (the pump feeds the entire slept interval on the wake).
	if !box.IsCaretOn() {
		t.Fatal("fresh focus must start caret-on")
	}
	owner.TickAllTickers(0.6)
	if !box.IsCaretOn() {
		t.Fatal("mid-gap reset must survive a whole-gap tick: caret stays on")
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

// 墙钟相位映射纯函数表：偶数半周期亮、奇数半周期灭；NextWake 报到下一个
// 边界的剩余时长。大步长奇偶性由墙钟相位天然承载，不再需要 dt 累积模型。
func TestBlinkPhaseMapping(t *testing.T) {
	cases := []struct {
		phase  float64
		wantOn bool
	}{
		{0, true},
		{0.2, true},
		{0.499, true},
		{0.5, false},
		{0.6, false},
		{0.999, false},
		{1.0, true},
		{1.1, true},
		{1.5, false},
		{2.0, true},
		{-0.1, true}, // 负相位钳到 0
	}
	for _, c := range cases {
		if got := blinkStateAt(c.phase); got != c.wantOn {
			t.Fatalf("blinkStateAt(%v) = %v, want %v", c.phase, got, c.wantOn)
		}
	}
	wakes := []struct {
		phase float64
		want  time.Duration
	}{
		{0, 500 * time.Millisecond},
		{0.2, 300 * time.Millisecond},
		{0.5, 500 * time.Millisecond}, // 边界后进入第二个半周期
		{1.1, 400 * time.Millisecond},
	}
	for _, c := range wakes {
		d, ok := blinkNextWakeFrom(c.phase)
		if !ok || d < c.want-time.Millisecond || d > c.want+time.Millisecond {
			t.Fatalf("blinkNextWakeFrom(%v) = %v,%v want %v,true", c.phase, d, ok, c.want)
		}
	}
}

// 墙钟驱动实测：不喂 dt（Tick(0)），只靠真实流逝跨过半周期就翻转——
// 闪烁按墙钟走，不按循环唤醒节奏走。
func TestBlinkWallClock_TogglesByWallTime(t *testing.T) {
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
	time.Sleep(560 * time.Millisecond)
	box.Tick(0)
	if box.IsCaretOn() {
		t.Fatal("wall phase past half period must flip caret off")
	}
	time.Sleep(500 * time.Millisecond)
	box.Tick(0)
	if !box.IsCaretOn() {
		t.Fatal("second half period must flip caret back on")
	}
}

// 编辑常亮契约：每次编辑重置墙钟锚点，相位走不到翻转点，光标常亮
// （Flutter EditableText：编辑时 _cursorTimer.reset()）。
func TestBlinkSolidWhileEditing_WallClock(t *testing.T) {
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
	// 连续编辑（每次重置锚点）跨过 1.5 个半周期，光标必须全程常亮。
	for i := 0; i < 4; i++ {
		time.Sleep(180 * time.Millisecond)
		box.SetCaretOn(true) // 编辑重置
		box.Tick(0)
		if !box.IsCaretOn() {
			t.Fatalf("caret must stay solid while editing (round %d)", i)
		}
	}
}

// NextWake 跟随墙钟相位余量；未聚焦的盒子报不出截止点（false）。
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
	if d, ok := box.NextWake(); !ok || d <= 400*time.Millisecond || d > 500*time.Millisecond {
		t.Fatalf("NextWake=%v,%v want ~500ms", d, ok)
	}
	time.Sleep(200 * time.Millisecond)
	// 不喂 dt：墙钟相位自己走了 200ms，截止点必须跟着缩短。
	if d, ok := box.NextWake(); !ok || d <= 200*time.Millisecond || d > 400*time.Millisecond {
		t.Fatalf("NextWake=%v,%v want ~300ms (wall phase must advance without dt)", d, ok)
	}
}

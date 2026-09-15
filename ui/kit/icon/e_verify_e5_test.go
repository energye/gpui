package icon_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit/icon"
	"github.com/energye/gpui/ui/scheduler"
)

// E5（docs/antd/icon.md §6.13）：闲图标自动摘除。
// 修前 Tick 在 !spin 时仍回 true，占着注册表不烧帧；修后回 false，
// TickAll 当场摘除（与 Button 惯例一致）。本测试锁修后语义。
func TestEVerify_E5_IdleIconAutoDrop(t *testing.T) {
	// 从没转过的图标：挂上后第一拍就摘除。
	reg := &scheduler.TickerRegistry{}
	ic := icon.NewIcon("sync")
	ic.Attach(reg)
	if !reg.HasActive() {
		t.Fatal("attached icon must register")
	}
	reg.TickAll(1.0 / 60)
	if reg.HasActive() {
		t.Fatal("idle icon must auto-drop after TickAll (E5)")
	}
	if ic.Tick(1.0 / 60) {
		t.Fatal("idle Tick must return false")
	}

	// 转着的留下。
	reg2 := &scheduler.TickerRegistry{}
	sp := icon.NewIcon("sync")
	sp.SetSpin(true)
	sp.Attach(reg2)
	reg2.TickAll(1.0 / 60)
	if !reg2.HasActive() {
		t.Fatal("spinning icon must stay registered")
	}
	if sp.Phase() == 0 && !sp.WantsFrame() {
		t.Fatal("spinning icon must advance phase and want frames")
	}

	// 关掉转圈：下一拍摘除。
	sp.SetSpin(false)
	reg2.TickAll(1.0 / 60)
	if reg2.HasActive() {
		t.Fatal("icon with SetSpin(false) must drop on next TickAll (E5)")
	}

	// 省动态开着：即使允许转也不占位。
	reg3 := &scheduler.TickerRegistry{}
	rm := icon.NewIcon("sync")
	rm.SetSpin(true)
	rm.SetReduceMotion(true)
	rm.Attach(reg3)
	reg3.TickAll(1.0 / 60)
	if reg3.HasActive() {
		t.Fatal("reduce-motion icon must not hold the registry")
	}
}

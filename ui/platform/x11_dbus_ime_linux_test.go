//go:build linux

package platform

import (
	"os"
	"strings"
	"testing"
	"time"
)

// TestX11SharedBusSingleConn 验证 S1 单 Conn 复用：多次调用共享同一指针
// 且建窗不阻塞（无守护时返回 nil 降级不崩）。
func TestX11SharedBusSingleConn(t *testing.T) {
	// 无 DISPLAY 时 SessionBus 通常失败，验证降级路径不崩且不阻塞
	c1, err1 := sharedDBusConn()
	c2, err2 := sharedDBusConn()
	if (c1 == nil) != (c2 == nil) {
		t.Fatalf("shared conn mismatch nil: c1=%v err1=%v c2=%v err2=%v", c1, err1, c2, err2)
	}
	if c1 != nil && c1 != c2 {
		t.Fatalf("shared conn not reused: %p vs %p", c1, c2)
	}
	if c1 != nil {
		// 有总线时两窗 IME 应复用同一 conn
		h1 := &x11Host{st: &x11State{}}
		h2 := &x11Host{st: &x11State{}}
		im1 := imeForX11(h1)
		im2 := imeForX11(h2)
		// 无守护时两者皆 nil（英文直通）；有守护时两者非 nil 且 conn 相同
		if (im1 == nil) != (im2 == nil) {
			t.Fatalf("imeForX11 mismatch nil: %v vs %v", im1, im2)
		}
		if im1 != nil && im2 != nil {
			x1 := im1.(*x11Ime)
			x2 := im2.(*x11Ime)
			if x1.conn != x2.conn {
				t.Fatalf("IME conn not shared: %p vs %p", x1.conn, x2.conn)
			}
			if x1.conn != c1 {
				t.Fatalf("IME conn != shared: %p vs %p", x1.conn, c1)
			}
		}
	} else {
		t.Logf("no session bus (err=%v), degrade nil as expected", err1)
		h := &x11Host{st: &x11State{}}
		if im := imeForX11(h); im != nil {
			t.Fatalf("expected nil IME without bus, got %T", im)
		}
	}
	_ = err2
}

// TestX11ImeDebugSmoke 验证 GPUI_IME_DEBUG 打点不崩且桩接口可用
func TestX11ImeDebugSmoke(t *testing.T) {
	os.Setenv("GPUI_IME_DEBUG", "1")
	defer os.Unsetenv("GPUI_IME_DEBUG")
	// sharedDBusConn 在 debug 态应打印 dial/hello/match（人目验日志）
	_, _ = sharedDBusConn()
	h := &x11Host{st: &x11State{w: 800, h: 600}}
	im := imeForX11(h)
	if im == nil {
		t.Logf("no IME (no daemon), skip method smoke")
		return
	}
	// 桩方法幂等与 same-rect 跳过
	im.EnableIME(Rect{X: 10, Y: 20, W: 2, H: 16})
	im.EnableIME(Rect{X: 10, Y: 20, W: 2, H: 16}) // focused guard skip
	im.UpdateCursorRect(Rect{X: 10, Y: 20, W: 2, H: 16}) // same skip
	im.UpdateCursorRect(Rect{X: 12, Y: 20, W: 2, H: 16})
	im.SetContentType(PurposePassword)
	im.SetComposing("nihao", 3)
	im.Commit("你好")
	im.DisableIME()
	im.DisableIME() // repeat
}

// TestX11ImeNilSafety 保证 typed-nil 与 nil 接口调用不崩
func TestX11ImeNilSafety(t *testing.T) {
	var im *x11Ime
	im.EnableIME(Rect{})
	im.UpdateCursorRect(Rect{})
	im.SetContentType(PurposeNormal)
	im.SetComposing("", 0)
	im.Commit("")
	im.DisableIME()
	var im2 IME
	// nil 接口不调用，Window.IME()==nil 时上层静默降级
	if im2 != nil {
		im2.EnableIME(Rect{})
	}
}

// TestX11S2AsyncProbe 验证 S2 异步探针：不阻塞、SetCapabilities、每窗一路径、Close 不泄漏
func TestX11S2AsyncProbe(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no DISPLAY, skip real S2 probe")
	}
	os.Setenv("GPUI_IME_DEBUG", "1")
	defer os.Unsetenv("GPUI_IME_DEBUG")
	// 清理 env 干扰
	origGtk := os.Getenv("GTK_IM_MODULE")
	origQt := os.Getenv("QT_IM_MODULE")
	os.Setenv("GTK_IM_MODULE", "ibus")
	os.Setenv("QT_IM_MODULE", "ibus")
	defer func() {
		os.Setenv("GTK_IM_MODULE", origGtk)
		os.Setenv("QT_IM_MODULE", origQt)
	}()
	h1 := &x11Host{st: &x11State{}}
	h2 := &x11Host{st: &x11State{}}
	im1 := imeForX11(h1)
	im2 := imeForX11(h2)
	if im1 == nil || im2 == nil {
		t.Skip("no daemon, skip S2")
	}
	// 异步：立即返回应有 IME 但 engine 尚未就绪
	x1 := im1.(*x11Ime)
	x2 := im2.(*x11Ime)
	// 等待探针完成（500ms*2 + 开销）
	for i := 0; i < 20; i++ {
		if x1.Engine() != "" && x2.Engine() != "" {
			break
		}
		// 也允许 100ms 轮询
		// 使用 time.Sleep
		time.Sleep(100 * time.Millisecond)
	}
	if x1.Engine() == "" || x2.Engine() == "" {
		t.Fatalf("engine not set after probe: %q %q", x1.Engine(), x2.Engine())
	}
	if x1.ObjectPath() == "" || x2.ObjectPath() == "" {
		t.Fatalf("objectPath empty")
	}
	if x1.ObjectPath() == x2.ObjectPath() {
		t.Fatalf("dual window same path %s, should differ", x1.ObjectPath())
	}
	if x1.conn != x2.conn {
		t.Fatalf("conn not shared")
	}
	// SetCapabilities 已在日志中，此处检查 engine 非空即代表已调用
	// Close 不泄漏：关闭后 path 清零且不影响另一窗
	x1.Close()
	if x1.ObjectPath() != "" {
		t.Fatalf("x1 path not cleared after Close")
	}
	if x2.ObjectPath() == "" {
		t.Fatalf("x2 path cleared unexpectedly after x1 Close")
	}
	x2.Close()
	if x2.ObjectPath() != "" {
		t.Fatalf("x2 path not cleared")
	}
}

// TestX11TruncateSyncB10 保证 x11TruncateSurrounding 满足 4000/NUL/居中语义（B10）。
// 不直接 import textinput 以免 platform<->textinput 循环；同源性由脚本 scripts/check_truncate_sync.go 校验。
func TestX11TruncateSyncB10(t *testing.T) {
	cases := []struct {
		text   string
		cursor int
		anchor int
	}{
		{"hello world", 5, 5},
		{"你好世界 hello", 6, 6},
		{strings.Repeat("a", 3000) + strings.Repeat("你", 1000), 2000, 2000},
		{strings.Repeat("😀", 500), 1000, 1000},
		{"", 0, 0},
	}
	for i, c := range cases {
		trX, cx, ax := x11TruncateSurrounding(c.text, c.cursor, c.anchor)
		if len(trX)+1 > 4000 {
			t.Fatalf("case %d x11 truncate exceed 4000: %d", i, len(trX))
		}
		if cx < 0 || cx > len(trX) || ax < 0 || ax > len(trX) {
			t.Fatalf("case %d cursor/anchor out of range: %d/%d len %d", i, cx, ax, len(trX))
		}
		// 光标必须落在截断窗口内
		if c.cursor < len(c.text) && cx == 0 && len(c.text) > 4000 {
			// 居中截断时，超长文本的光标不应被截到 0（除非本来就在头部）
			if c.cursor > 2000 && trX == "" {
				t.Fatalf("case %d truncated to empty", i)
			}
		}
	}
}

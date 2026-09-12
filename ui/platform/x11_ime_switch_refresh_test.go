//go:build linux

package platform

import (
	"sync"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

// fakeSwitchEngine 扮演输入法引擎：按键是否被吃掉可按脚本指定，
// SetCursorLocation 只记录不发送（不断言外部守护行为）。
type fakeSwitchEngine struct {
	mu       sync.Mutex
	handled  bool
	cursorHx []cursorCall
}

type cursorCall struct {
	x, y, w, h int
}

func (f *fakeSwitchEngine) Name() string { return "fcitx5" }
func (f *fakeSwitchEngine) Caps() uint32 { return 0 }
func (f *fakeSwitchEngine) Bus(force bool) (*dbus.Conn, error) {
	return nil, nil
}
func (f *fakeSwitchEngine) CreateInputContext(conn *dbus.Conn, timeout time.Duration) (dbus.ObjectPath, error) {
	return "", nil
}
func (f *fakeSwitchEngine) SetCapabilities(conn *dbus.Conn, obj dbus.ObjectPath, caps uint32) error {
	return nil
}
func (f *fakeSwitchEngine) Destroy(conn *dbus.Conn, obj dbus.ObjectPath) error { return nil }
func (f *fakeSwitchEngine) FocusIn(conn *dbus.Conn, obj dbus.ObjectPath) error { return nil }
func (f *fakeSwitchEngine) FocusOut(conn *dbus.Conn, obj dbus.ObjectPath) error {
	return nil
}
func (f *fakeSwitchEngine) SetCursorLocation(conn *dbus.Conn, obj dbus.ObjectPath, x, y, w, h int) error {
	f.mu.Lock()
	f.cursorHx = append(f.cursorHx, cursorCall{x, y, w, h})
	f.mu.Unlock()
	return nil
}
func (f *fakeSwitchEngine) SetSurroundingText(conn *dbus.Conn, obj dbus.ObjectPath, text string, cursor, anchor int) error {
	return nil
}
func (f *fakeSwitchEngine) SetContentType(conn *dbus.Conn, obj dbus.ObjectPath, purpose ContentPurpose) error {
	return nil
}
func (f *fakeSwitchEngine) ProcessKeyEvent(conn *dbus.Conn, obj dbus.ObjectPath, keysym, keycode, state, xTime uint32, isPress bool) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.handled, nil
}
func (f *fakeSwitchEngine) cursorCalls() []cursorCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]cursorCall(nil), f.cursorHx...)
}

// newSwitchRefreshIME 搭一个“会话就绪”的 x11Ime：连接取自会话总线（仅用于
// sessionUsable 的零往返存活判断，不产生任何 D-Bus 调用），实际发送全走桩引擎。
// 无会话总线时跳过（外部依赖缺失不清零不断言）。
func newSwitchRefreshIME(t *testing.T, eng *fakeSwitchEngine) *x11Ime {
	t.Helper()
	conn, err := dbus.SessionBus()
	if err != nil {
		t.Skipf("no session bus: %v", err)
	}
	return &x11Ime{
		conn:       conn,
		host:       &x11Host{st: &x11State{scale: 1}},
		engine:     "fcitx5",
		engineImpl: eng,
		objectPath: "/org/fcitx/Fcitx5/InputContext_9",
		focused:    true,
		hasRect:    true,
		lastRect:   Rect{X: 100, Y: 200, W: 2, H: 22},
	}
}

// TestX11SwitchRefreshHandledPressBurstsOnce：非组合期按键被输入法吃掉
//（如切中/拼/en 的触发键），同步路径补刷一次缓存矩形。
func TestX11SwitchRefreshHandledPressBurstsOnce(t *testing.T) {
	eng := &fakeSwitchEngine{handled: true}
	im := newSwitchRefreshIME(t, eng)
	if im.ProcessKeyEvent(65, 0, true) != true {
		t.Fatalf("handled key should stay handled")
	}
	got := eng.cursorCalls()
	if len(got) != 1 {
		t.Fatalf("cursor sends = %d, want 1 (switch burst)", len(got))
	}
	if got[0] != (cursorCall{x: 100, y: 200, w: 2, h: 22}) {
		t.Fatalf("cursor send = %+v, want cached rect {100 200 2 22}", got[0])
	}
}

// TestX11SwitchRefreshComposingStaysQuiet：组合期被吃掉的按键不补刷
//（preedit 变更自带上报，补刷是重复）。
func TestX11SwitchRefreshComposingStaysQuiet(t *testing.T) {
	eng := &fakeSwitchEngine{handled: true}
	im := newSwitchRefreshIME(t, eng)
	im.mu.Lock()
	im.composing = true
	im.mu.Unlock()
	im.ProcessKeyEvent(65, 0, true)
	if got := eng.cursorCalls(); len(got) != 0 {
		t.Fatalf("cursor sends while composing = %d, want 0", len(got))
	}
}

// TestX11SwitchRefreshReleaseAndPassthroughQuiet：释放事件与未被吃掉的按键不补刷。
func TestX11SwitchRefreshReleaseAndPassthroughQuiet(t *testing.T) {
	eng := &fakeSwitchEngine{handled: true}
	im := newSwitchRefreshIME(t, eng)
	im.ProcessKeyEvent(65, 0, false) // release：只刷一次，release 不刷
	eng.mu.Lock()
	eng.handled = false
	eng.mu.Unlock()
	im.ProcessKeyEvent(65, 0, true) // 未被吃掉：本地直通，不刷
	if got := eng.cursorCalls(); len(got) != 0 {
		t.Fatalf("cursor sends = %d, want 0 (release + passthrough)", len(got))
	}
}

// TestX11SwitchRefreshUnfocusedQuiet：失焦时补刷静默（无会话可言）。
func TestX11SwitchRefreshUnfocusedQuiet(t *testing.T) {
	eng := &fakeSwitchEngine{handled: true}
	im := newSwitchRefreshIME(t, eng)
	im.mu.Lock()
	im.focused = false
	im.mu.Unlock()
	im.ProcessKeyEvent(65, 0, true)
	if got := eng.cursorCalls(); len(got) != 0 {
		t.Fatalf("cursor sends while unfocused = %d, want 0", len(got))
	}
}

// TestX11SwitchRefreshAsyncBurstsOnce：生产走的异步路径同样只刷一次。
func TestX11SwitchRefreshAsyncBurstsOnce(t *testing.T) {
	eng := &fakeSwitchEngine{handled: true}
	im := newSwitchRefreshIME(t, eng)
	im.ProcessKeyEventAsync(65, 0, true, 0, Event{Type: EventKey})
	deadline := time.Now().Add(2 * time.Second)
	for {
		if len(eng.cursorCalls()) >= 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("async switch burst never sent")
		}
		time.Sleep(5 * time.Millisecond)
	}
	// 再等一小段，确认没有第二次（只刷一次）。
	time.Sleep(100 * time.Millisecond)
	if got := eng.cursorCalls(); len(got) != 1 {
		t.Fatalf("async cursor sends = %d, want exactly 1", len(got))
	}
}

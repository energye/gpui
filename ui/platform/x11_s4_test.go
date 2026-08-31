//go:build linux

package platform

import (
	"os"
	"testing"
	"time"
)

func TestX11S4ProcessKeyEvent(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no DISPLAY")
	}
	os.Setenv("GPUI_IME_DEBUG", "1")
	defer os.Unsetenv("GPUI_IME_DEBUG")
	h := &x11Host{st: &x11State{
		display: 1, // dummy non-zero to allow keysym lookup, but we use nil keycodeToKeysym so keysym will be 0
	}}
	// need a real host with display and keycodeToKeysym for accurate keysym, but we can test with mock
	// Use a real window for accurate test
	w, err := Open(Options{Width: 320, Height: 240, Title: "s4 test"})
	if err != nil {
		t.Skipf("open fail: %v", err)
	}
	defer w.Close()
	ime := w.IME()
	if ime == nil {
		t.Skip("IME nil")
	}
	// wait for probe
	time.Sleep(800 * time.Millisecond)
	x, ok := ime.(*x11Ime)
	if !ok {
		t.Fatalf("not x11Ime")
	}
	if x.ObjectPath() == "" {
		t.Fatalf("no objectPath")
	}
	// Test 1: Unset should pass through (we test with a new x11Ime with empty path)
	empty := &x11Ime{conn: x.conn, host: h}
	if empty.ProcessKeyEvent(38, 0, true) {
		t.Fatalf("empty path should not be handled")
	}
	// Test 2: 50ms timeout - call should return quickly
	t0 := time.Now()
	handled := x.ProcessKeyEvent(38, 0, true)
	elapsed := time.Since(t0)
	if elapsed > 100*time.Millisecond {
		t.Fatalf("ProcessKeyEvent took %v >100ms", elapsed)
	}
	t.Logf("ProcessKeyEvent a handled=%v elapsed=%v", handled, elapsed)
	// Test 3: composing true vs false
	x.SetComposing("ni", 2)
	time.Sleep(100 * time.Millisecond)
	// When composing, a key should be handled (depending on daemon, but at least it should not panic)
	// We can't assert exact handled value without knowing daemon state, but we can ensure it doesn't block and returns bool
	t0 = time.Now()
	handled2 := x.ProcessKeyEvent(38, 0, true)
	elapsed = time.Since(t0)
	if elapsed > 100*time.Millisecond {
		t.Fatalf("composing ProcessKeyEvent took %v", elapsed)
	}
	t.Logf("composing a handled=%v", handled2)
	// Test Home key when composing - should be handled if daemon says true
	// Home keycode is typically 110, keysym 0xff50
	handledHome := x.ProcessKeyEvent(110, 0, true)
	t.Logf("Home handled=%v", handledHome)
	// Clear composing
	x.SetComposing("", 0)
	time.Sleep(50 * time.Millisecond)
	handled3 := x.ProcessKeyEvent(38, 0, true)
	t.Logf("after clear composing a handled=%v", handled3)
}

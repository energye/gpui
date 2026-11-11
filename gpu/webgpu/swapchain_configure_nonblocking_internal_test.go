//go:build !(js && wasm)

package webgpu

import "testing"

// TestPickPresentMode_NonBlocking pins the Wayland I5 v2 present order:
// Mailbox replaces the pending image without waiting for compositor release;
// Immediate is the next non-blocking fallback; FifoRelaxed still waits for a
// free image, so it only ranks above legacy FIFO. A surface that advertises
// only FIFO must still select FIFO (callers surface the residual risk).
func TestPickPresentMode_NonBlocking(t *testing.T) {
	sc := NewSwapchain(nil, nil, 1, 1)
	sc.SetPreferNonBlocking()
	want := []PresentMode{PresentModeMailbox, PresentModeImmediate, PresentModeFifoRelaxed, PresentModeFifo}
	if len(sc.PreferPresentModes) != len(want) {
		t.Fatalf("prefer list=%v want %v", sc.PreferPresentModes, want)
	}
	for i := range want {
		if sc.PreferPresentModes[i] != want[i] {
			t.Fatalf("prefer[%d]=%v want %v (list=%v)", i, sc.PreferPresentModes[i], want[i], sc.PreferPresentModes)
		}
	}

	cases := []struct {
		name      string
		available []PresentMode
		want      PresentMode
	}{
		{"all", []PresentMode{PresentModeFifo, PresentModeImmediate, PresentModeFifoRelaxed, PresentModeMailbox}, PresentModeMailbox},
		{"no-mailbox", []PresentMode{PresentModeFifo, PresentModeImmediate, PresentModeFifoRelaxed}, PresentModeImmediate},
		{"only-relaxed", []PresentMode{PresentModeFifo, PresentModeFifoRelaxed}, PresentModeFifoRelaxed},
		{"only-fifo", []PresentMode{PresentModeFifo}, PresentModeFifo},
		{"empty", nil, PresentModeFifo},
	}
	for _, c := range cases {
		if got := pickPresentMode(c.available, sc.PreferPresentModes); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

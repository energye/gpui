package platform

import (
	"sync/atomic"
	"testing"
	"time"
)

// fakeBackend is a test backend recording Create/Adopt calls.
type fakeBackend struct {
	kind    PlatformKind
	created atomic.Int32
	adopted atomic.Int32
	win     *Window
}

func (f *fakeBackend) Kind() PlatformKind { return f.kind }

func (f *fakeBackend) Create(opts Options) (*Window, error) {
	f.created.Add(1)
	if f.win != nil {
		return f.win, nil
	}
	h := NewStubHost(opts.Width, opts.Height)
	return newWindow(h, f.kind, nil, nil, nil), nil
}

func (f *fakeBackend) Adopt(ns NativeSurface) (*Window, error) {
	f.adopted.Add(1)
	h := NewStubHost(1, 1)
	h.SetNativeSurface(ns)
	return newWindow(h, f.kind, nil, nil, nil), nil
}

// testKind avoids colliding with real backends on any OS.
const testKind = PlatformKind(100)

func withFakeBackend(t *testing.T, f *fakeBackend) {
	t.Helper()
	Register(testKind, f)
	t.Cleanup(func() { Register(testKind, nil) })
}

func TestRegisterAndBackendFor(t *testing.T) {
	if _, err := backendFor(testKind); err == nil {
		t.Fatal("backendFor unregistered kind should error")
	}
	f := &fakeBackend{kind: testKind}
	Register(testKind, f)
	defer Register(testKind, nil)

	b, err := backendFor(testKind)
	if err != nil {
		t.Fatalf("backendFor: %v", err)
	}
	if b.Kind() != testKind {
		t.Fatalf("kind = %d, want %d", b.Kind(), testKind)
	}
}

func TestRegisteredKindsOrder(t *testing.T) {
	// Register out of order; registeredKinds must return sorted by value.
	Register(testKind, &fakeBackend{kind: testKind})
	defer Register(testKind, nil)
	// testKind(100) sorts after all real kinds; just check it's present and sorted.
	ks := registeredKinds()
	found := false
	for i, k := range ks {
		if i > 0 && ks[i-1] > k {
			t.Fatalf("registeredKinds not sorted: %v", ks)
		}
		if k == testKind {
			found = true
		}
	}
	if !found {
		t.Fatalf("testKind not in registeredKinds: %v", ks)
	}
}

func TestRegisterNilRemoves(t *testing.T) {
	f := &fakeBackend{kind: testKind}
	Register(testKind, f)
	Register(testKind, nil)
	if _, err := backendFor(testKind); err == nil {
		t.Fatal("Register(nil) should remove the backend")
	}
}

// withOverriddenX11 swaps the PlatformX11 backend for the test and restores
// the original (or removes it if none) on cleanup. Platform-independent.
func withOverriddenX11(t *testing.T, f Backend) {
	t.Helper()
	orig, _ := backendFor(PlatformX11)
	Register(PlatformX11, f)
	t.Cleanup(func() {
		if orig != nil {
			Register(PlatformX11, orig)
		} else {
			Register(PlatformX11, nil)
		}
	})
}

func TestOpenCallsCreate(t *testing.T) {
	f := &fakeBackend{kind: testKind}
	withFakeBackend(t, f)
	withOverriddenX11(t, f) // route Open(DisplayX11) to the fake

	win, err := Open(Options{Width: 800, Height: 600, Title: "t", Backend: DisplayX11})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if f.created.Load() != 1 {
		t.Fatalf("created = %d, want 1", f.created.Load())
	}
	if win == nil || win.Kind() != testKind {
		t.Fatalf("win kind = %v, want testKind", win.Kind())
	}
	// Defaults applied.
	w, h := win.Host().Size()
	if w != 800 || h != 600 {
		t.Fatalf("size = %dx%d", w, h)
	}
}

func TestOpenDefaults(t *testing.T) {
	f := &fakeBackend{kind: testKind}
	withFakeBackend(t, f)
	withOverriddenX11(t, f)

	win, err := Open(Options{Backend: DisplayX11})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	w, h := win.Host().Size()
	if w != 640 || h != 480 {
		t.Fatalf("default size = %dx%d, want 640x480", w, h)
	}
}

func TestOpenNoBackendErrors(t *testing.T) {
	// Remove the x11 backend; Open(DisplayX11) must error via backendFor.
	withOverriddenX11(t, nil)
	if _, err := Open(Options{Backend: DisplayX11}); err == nil {
		t.Fatal("Open with no backend should error")
	}
}

func TestAdoptRoutesByKind(t *testing.T) {
	f := &fakeBackend{kind: testKind}
	withFakeBackend(t, f)

	ns := NativeSurface{Kind: testKind, Display: 0x1234, Window: 0x5678}
	win, err := Adopt(ns)
	if err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	if f.adopted.Load() != 1 {
		t.Fatalf("adopted = %d, want 1", f.adopted.Load())
	}
	got := win.Host().NativeSurface()
	if got.Display != 0x1234 || got.Window != 0x5678 {
		t.Fatalf("surface = %+v", got)
	}
}

func TestAdoptUnknownKind(t *testing.T) {
	if _, err := Adopt(NativeSurface{Kind: PlatformNone}); err == nil {
		t.Fatal("Adopt PlatformNone should error")
	}
	if _, err := Adopt(NativeSurface{Kind: PlatformKind(999)}); err == nil {
		t.Fatal("Adopt unregistered kind should error")
	}
}

func TestWindowCloseOnce(t *testing.T) {
	var n atomic.Int32
	h := NewStubHost(10, 10)
	w := newWindow(h, testKind, nil, nil, func() { n.Add(1) })
	if w.Closed() {
		t.Fatal("new window should be open")
	}
	w.Close()
	w.Close()
	if n.Load() != 1 {
		t.Fatalf("close fn ran %d times, want 1", n.Load())
	}
	if !w.Closed() {
		t.Fatal("window should be closed")
	}
}

func TestWindowNilSafety(t *testing.T) {
	var w *Window
	if w.Host() != nil || w.Kind() != PlatformNone || w.IME() != nil || w.Clipboard() != nil {
		t.Fatal("nil Window should return zero values")
	}
	if w.Closed() != true {
		t.Fatal("nil Window.Closed should be true")
	}
	w.Close() // must not panic
}

func TestWindowCapabilities(t *testing.T) {
	// No IME/Clipboard in the fake → nil (silent degrade).
	f := &fakeBackend{kind: testKind}
	withFakeBackend(t, f)
	withOverriddenX11(t, f)

	win, err := Open(Options{Backend: DisplayX11})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if win.IME() != nil || win.Clipboard() != nil {
		t.Fatal("fake backend has no capabilities; must be nil")
	}
}

func TestToPlatformKind(t *testing.T) {
	if toPlatformKind(DisplayX11) != PlatformX11 {
		t.Fatal("DisplayX11 → PlatformX11")
	}
	if toPlatformKind(DisplayWayland) != PlatformWayland {
		t.Fatal("DisplayWayland → PlatformWayland")
	}
	if toPlatformKind(DisplayAuto) != PlatformNone {
		t.Fatal("DisplayAuto → PlatformNone (must be resolved by detection)")
	}
}

func TestWindowHostEventPump(t *testing.T) {
	// StubHost round-trips events through the Window wrapper.
	h := NewStubHost(100, 100)
	w := newWindow(h, testKind, nil, nil, nil)
	h.Push(Event{Type: EventResize, Width: 200, Height: 150, Scale: 2})
	evs := w.Host().WaitEvents(0)
	if len(evs) != 1 || evs[0].Type != EventResize || evs[0].Width != 200 {
		t.Fatalf("events = %+v", evs)
	}
}

// keep the time import referenced for builds where no test uses it directly
var _ = time.Second

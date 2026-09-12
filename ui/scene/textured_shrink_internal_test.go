package scene

import (
	"image"
	"testing"
	"unsafe"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/render"
)

// fakeLiveToken backs fakeLiveView: the pointer only needs to be non-nil so
// headless cache logic treats the slot as a live GPU view (never sampled).
var fakeLiveToken byte

// fakeLiveView fakes a live GPU texture view without a device.
func fakeLiveView() gpucontext.TextureView {
	return gpucontext.NewTextureView(unsafe.Pointer(&fakeLiveToken))
}

// A shrink re-record must keep the previous bounds for the damage union:
// damage for a refreshed layer is previous ∪ current, otherwise the
// cleared tail is never repainted and a LoadOpLoad present shows stale
// pixels (IME backspace stall: "hi"→"h" kept showing "hi").
func TestRecordLocal_ShrinkKeepsPrevBounds(t *testing.T) {
	dc := render.NewContext(100, 50)
	defer dc.Close()
	tex := NewPictureTextureCache(dc, 0)
	alloc, _, _ := fakeSlotFactory(t)
	tex.slotAlloc = alloc

	big := image.Rect(0, 0, 80, 20)
	picBig := RecordPicture(func(r *PictureRecorder) {
		r.FillRect(0, 0, 80, 20, 1, 0, 0, 1)
	})
	small := image.Rect(0, 0, 40, 20)
	picSmall := RecordPicture(func(r *PictureRecorder) {
		r.FillRect(0, 0, 40, 20, 1, 0, 0, 1)
	})

	tex.BeginFrame()
	if _, ok := tex.recordLocalWith(7, &picBig, big, nil); !ok {
		t.Fatal("grow record must succeed")
	}
	if got := tex.PrevBounds(7); !got.Empty() {
		t.Fatalf("first record has no previous bounds, got %v", got)
	}

	tex.BeginFrame()
	if _, ok := tex.recordLocalWith(7, &picSmall, small, nil); !ok {
		t.Fatal("shrink record must succeed")
	}
	if got := tex.Bounds(7); got != small {
		t.Fatalf("current bounds=%v want %v", got, small)
	}
	if got := tex.PrevBounds(7); got != big {
		t.Fatalf("prev bounds=%v want %v (cleared tail would miss damage)", got, big)
	}
	if u := tex.Bounds(7).Union(tex.PrevBounds(7)); u != big {
		t.Fatalf("damage union=%v want %v", u, big)
	}
	if !tex.RecordedThisFrame(7) {
		t.Fatal("shrink record must count as recorded this frame")
	}

	// Frame scoping: without a new re-record the previous bounds expire,
	// so later frames never accumulate stale damage.
	tex.BeginFrame()
	if got := tex.PrevBounds(7); !got.Empty() {
		t.Fatalf("prev bounds must expire after the frame, got %v", got)
	}
	if got := tex.Bounds(7); got != small {
		t.Fatalf("current bounds must survive, got %v", got)
	}
}

// Growing must not change damage: union(previous, current) == current.
func TestRecordLocal_GrowUnionIsCurrent(t *testing.T) {
	dc := render.NewContext(120, 50)
	defer dc.Close()
	tex := NewPictureTextureCache(dc, 0)
	alloc, _, _ := fakeSlotFactory(t)
	tex.slotAlloc = alloc

	small := image.Rect(0, 0, 40, 20)
	picSmall := RecordPicture(func(r *PictureRecorder) {
		r.FillRect(0, 0, 40, 20, 1, 0, 0, 1)
	})
	big := image.Rect(0, 0, 80, 20)
	picBig := RecordPicture(func(r *PictureRecorder) {
		r.FillRect(0, 0, 80, 20, 1, 0, 0, 1)
	})

	tex.BeginFrame()
	if _, ok := tex.recordLocalWith(7, &picSmall, small, nil); !ok {
		t.Fatal("first record must succeed")
	}
	tex.BeginFrame()
	if _, ok := tex.recordLocalWith(7, &picBig, big, nil); !ok {
		t.Fatal("grow record must succeed")
	}
	if u := tex.Bounds(7).Union(tex.PrevBounds(7)); u != big {
		t.Fatalf("grow damage union=%v want current %v", u, big)
	}
}

// Clearing a layer to empty must keep a viewless shell carrying the previous
// region for phase-2 damage: dropping the shell silently (or skipping its
// damage in the composite) leaves stale pixels until an incidental full
// repaint (IME second-backspace ghost: "h"→"" kept showing "h" ~500ms).
func TestDropTransparent_KeepsShellForDamage(t *testing.T) {
	dc := render.NewContext(100, 50)
	defer dc.Close()
	tex := NewPictureTextureCache(dc, 0)
	alloc, _, _ := fakeSlotFactory(t)
	tex.slotAlloc = alloc

	small := image.Rect(0, 0, 40, 20)
	picSmall := RecordPicture(func(r *PictureRecorder) {
		r.FillRect(0, 0, 40, 20, 1, 0, 0, 1)
	})
	tex.BeginFrame()
	if _, ok := tex.recordLocalWith(7, &picSmall, small, nil); !ok {
		t.Fatal("first record must succeed")
	}
	// Fake a live GPU view so the clear keeps a shell instead of evicting
	// (headless has no device; the view is never sampled in this test).
	tex.mu.Lock()
	e := tex.entries[7]
	if e == nil || e.contentSlot < 0 {
		tex.mu.Unlock()
		t.Fatal("entry must exist after record")
	}
	e.slots[e.contentSlot].view = fakeLiveView()
	tex.mu.Unlock()

	tex.BeginFrame()
	tex.mu.Lock()
	tex.dropTransparentLocked(7, image.Rectangle{})
	tex.mu.Unlock()
	if tex.Has(7) {
		t.Fatal("cleared layer must have no texture to blit")
	}
	if !tex.RecordedThisFrame(7) {
		t.Fatal("shell transition must count as recorded this frame or phase 2 emits no damage")
	}
	if got := tex.Bounds(7); got != small {
		t.Fatalf("shell bounds=%v want previous %v", got, small)
	}
	if got := tex.PrevBounds(7); got != small {
		t.Fatalf("shell prev bounds=%v want previous %v", got, small)
	}
	if u := tex.Bounds(7).Union(tex.PrevBounds(7)); u != small {
		t.Fatalf("shell damage union=%v want %v", u, small)
	}
	// Expiry like normal records: no accumulation into later frames.
	tex.BeginFrame()
	if got := tex.PrevBounds(7); !got.Empty() {
		t.Fatalf("shell prev bounds must expire after the frame, got %v", got)
	}
}

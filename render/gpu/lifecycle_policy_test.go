//go:build !nogpu

package gpu

import (
	"os"
	"testing"
)

func TestResolveSurfaceLifecycle_EnvAndOOM(t *testing.T) {
	ResetTextureOOMCount()
	t.Cleanup(func() {
		_ = os.Unsetenv("GPUI_LIFECYCLE")
		ResetTextureOOMCount()
	})

	_ = os.Setenv("GPUI_LIFECYCLE", "normal")
	if got := ResolveSurfaceLifecycle(nil); got != LifecycleNormal {
		t.Fatalf("normal: got %v", got)
	}
	_ = os.Setenv("GPUI_LIFECYCLE", "purge")
	if got := ResolveSurfaceLifecycle(nil); got != LifecyclePurge {
		t.Fatalf("purge: got %v", got)
	}
	_ = os.Setenv("GPUI_LIFECYCLE", "recreate")
	if got := ResolveSurfaceLifecycle(nil); got != LifecycleRecreate {
		t.Fatalf("recreate: got %v", got)
	}

	_ = os.Setenv("GPUI_LIFECYCLE", "auto")
	ResetTextureOOMCount()
	if got := ResolveSurfaceLifecycle(nil); got != LifecyclePurge {
		t.Fatalf("auto default purge: got %v want purge", got)
	}
	NoteTextureOOM()
	if got := ResolveSurfaceLifecycle(nil); got != LifecycleRecreate {
		t.Fatalf("auto after OOM: got %v want recreate", got)
	}
}

func TestNoteTextureOOM_Increments(t *testing.T) {
	ResetTextureOOMCount()
	if TextureOOMCount() != 0 {
		t.Fatal("want 0")
	}
	NoteTextureOOM()
	NoteTextureOOM()
	if TextureOOMCount() != 2 {
		t.Fatalf("got %d", TextureOOMCount())
	}
	ResetTextureOOMCount()
}

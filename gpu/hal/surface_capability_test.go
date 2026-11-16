//go:build !(js && wasm)

package hal_test

import (
	"testing"

	"github.com/energye/gpui/gpu/hal"
)

// plainSurface implements hal.Surface but not the optional capability.
type plainSurface struct{ hal.Surface }

// persistentSurface opts into the optional capability.
type persistentSurface struct {
	hal.Surface
	value bool
}

func (p persistentSurface) PersistentBackBuffer() bool { return p.value }

func TestSurfacePersistentBackBuffer(t *testing.T) {
	if hal.SurfacePersistentBackBuffer(nil) {
		t.Fatal("nil surface must report false")
	}
	var plain plainSurface
	if hal.SurfacePersistentBackBuffer(plain) {
		t.Fatal("surface without the optional capability must report false")
	}
	if !hal.SurfacePersistentBackBuffer(persistentSurface{value: true}) {
		t.Fatal("capability=true surface must report true")
	}
	if hal.SurfacePersistentBackBuffer(persistentSurface{value: false}) {
		t.Fatal("capability=false surface must report false")
	}
}

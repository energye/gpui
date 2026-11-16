//go:build linux || windows

package gles

import "testing"

func TestSurfacePersistentBackBuffer(t *testing.T) {
	var s Surface
	if !s.PersistentBackBuffer() {
		t.Fatal("GLES swapchain FBO keeps its previous frame; capability must be true")
	}
}

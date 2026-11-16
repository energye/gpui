//go:build !(js && wasm)

package webgpu

import "testing"

func TestSurfacePersistentBackBuffer_False(t *testing.T) {
	var s Surface
	if s.PersistentBackBuffer() {
		t.Fatal("wgpu-native rotates swapchain images; capability must be false")
	}
}

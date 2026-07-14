//go:build !(js && wasm)

package webgpu

import rwgpu "github.com/energye/gpui/gpu/rwgpu"

// Texture represents a GPU texture.
// On the wgpu-native backend, this wraps rwgpu Texture.
type Texture struct {
	r        *rwgpu.Texture
	device   *Device
	format   TextureFormat
	released bool
}

// Format returns the texture format.
func (t *Texture) Format() TextureFormat { return t.format }

// Release destroys the texture.
func (t *Texture) Release() {
	if t.released {
		return
	}
	t.released = true
	if t.r != nil {
		t.r.Release()
	}
}

// TextureView represents a view into a texture.
// On the wgpu-native backend, this wraps rwgpu TextureView.
type TextureView struct {
	r        *rwgpu.TextureView
	device   *Device
	texture  *Texture
	released bool
}

// Texture returns the parent Texture that this view was created from.
// Returns nil if the view has been released.
func (v *TextureView) Texture() *Texture { return v.texture }

// Release marks the texture view for destruction.
func (v *TextureView) Release() {
	if v.released {
		return
	}
	v.released = true
	if v.r != nil {
		v.r.Release()
	}
}

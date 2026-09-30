//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !(js && wasm)

package webgpu

import (
	"github.com/energye/gpui/gpu/hal"
	rwgpu "github.com/energye/gpui/gpu/rwgpu"
)

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

// Released reports whether Release has been called (diagnostics/tests).
func (t *Texture) Released() bool { return t != nil && t.released }

// Release frees the texture. On native this maps to wgpuTextureDestroy +
// Release so GPU memory is reclaimed immediately.
// Release-only left device heaps pinned across AutoRecover on libwgpu_native
// (process VRAM ≈ old+new RequestDevice reserve → CreateTexture OOM).
func (t *Texture) Release() {
	if t.released {
		return
	}
	t.released = true
	if t.r != nil {
		t.r.Destroy()
		t.r = nil
	}
}

// Destroy implements hal.Texture: same as Release.
func (t *Texture) Destroy() { t.Release() }

func (t *Texture) NativeHandle() uintptr { return 0 }

func (t *Texture) CurrentUsage() TextureUsage { return 0 }

func (t *Texture) AddPendingRef() {}

func (t *Texture) DecPendingRef() {}

// TextureView represents a view into a texture.
// On the wgpu-native backend, this wraps rwgpu TextureView.
type TextureView struct {
	r        *rwgpu.TextureView
	device   *Device
	texture  *Texture
	released bool
}

// Texture returns the parent Texture that this view was created from.
// Implements hal.TextureView (returns hal.Texture interface).
// Callers needing the concrete type can assert to *Texture.
func (v *TextureView) Texture() hal.Texture {
	if v.texture == nil {
		return nil
	}
	return v.texture
}

// Released reports whether Release has been called (diagnostics/tests).
func (v *TextureView) Released() bool { return v != nil && v.released }

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

// Destroy implements hal.TextureView: same as Release.
func (v *TextureView) Destroy() { v.Release() }

func (v *TextureView) NativeHandle() uintptr { return 0 }

var (
	_ hal.Texture     = (*Texture)(nil)
	_ hal.TextureView = (*TextureView)(nil)
)

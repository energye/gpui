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

// Sampler represents a texture sampler.
// On the wgpu-native backend, this wraps rwgpu Sampler.
type Sampler struct {
	r        *rwgpu.Sampler
	device   *Device
	released bool
}

// Release destroys the sampler.
func (s *Sampler) Release() {
	if s == nil || s.released {
		return
	}
	s.released = true
	if s.r != nil {
		s.r.Release()
	}
}

// Destroy implements hal.Sampler: same as Release.
func (s *Sampler) Destroy() { s.Release() }

func (s *Sampler) NativeHandle() uintptr { return 0 }

var _ hal.Sampler = (*Sampler)(nil)

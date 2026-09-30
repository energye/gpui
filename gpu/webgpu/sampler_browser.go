//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build js && wasm

package webgpu

import "github.com/energye/gpui/gpu/webgpu/internal/browser"

// Sampler represents a texture sampler.
type Sampler struct {
	browser  *browser.Sampler
	released bool
}

// Release destroys the sampler.
func (s *Sampler) Release() {
	if s.released {
		return
	}
	s.released = true
}

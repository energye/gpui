// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

//go:build windows && !(js && wasm)

package gles

import "github.com/energye/gpui/gpu/hal"

// Windows-only type assertions (Device/Queue/Adapter/Instance/Surface live in windows files).
var (
	_ hal.Backend        = Backend{}
	_ hal.Instance       = (*Instance)(nil)
	_ hal.Adapter        = (*Adapter)(nil)
	_ hal.Device         = (*Device)(nil)
	_ hal.Queue          = (*Queue)(nil)
	_ hal.Surface        = (*Surface)(nil)
	_ hal.SurfaceTexture = (*SurfaceTexture)(nil)
)

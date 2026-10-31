//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package hal

// Native release-on-lost switch.
//
// The rwgpu-backed native build skips child-resource native Release while the
// parent device is sticky-lost (calling into a stock fatal .so after device
// loss aborts the process). Soft-native recover paths must flip that behavior
// for the duration of a reclaim, otherwise VRAM stays pinned across
// Device.Destroy → RequestDevice and the new device OOMs on CreateTexture.
//
// The semantics live in the FFI binding layer, so hal only exposes the switch
// as an init-time hook: the native backend registers it at package init and
// render-side recover code calls WithForceNativeReleaseOnLost here instead of
// importing a concrete backend (P2.5 D2). Backends without skip-on-lost
// semantics (gles/metal/noop) never register a hook and the fallback runs fn
// unchanged — same observable behavior as before the switch existed.

var forceNativeReleaseOnLostHook func(func())

// SetForceNativeReleaseOnLostHook registers the backend implementation of
// WithForceNativeReleaseOnLost. Init-time wiring by the owning backend, like
// the Vram ledger's env read; not goroutine-safe by design.
func SetForceNativeReleaseOnLostHook(hook func(func())) {
	forceNativeReleaseOnLostHook = hook
}

// WithForceNativeReleaseOnLost runs fn with native child Release enabled even
// when the parent device is sticky-lost. With no hook registered (backends
// without skip-on-lost semantics) this is just fn().
func WithForceNativeReleaseOnLost(fn func()) {
	if fn == nil {
		return
	}
	if forceNativeReleaseOnLostHook != nil {
		forceNativeReleaseOnLostHook(fn)
		return
	}
	fn()
}

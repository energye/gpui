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

import "log/slog"

// SetLogger configures the logger for the wgpu stack.
// On the wgpu-native backend, logging is handled by wgpu-native internally.
func SetLogger(_ *slog.Logger) {
	// wgpu-native backend: wgpu-native has its own logging.
}

// Logger returns the current logger used by the wgpu stack.
// On the wgpu-native backend, returns nil.
func Logger() *slog.Logger {
	return nil
}

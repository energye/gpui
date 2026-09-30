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

import "log/slog"

// SetLogger configures the logger for the wgpu stack.
func SetLogger(l *slog.Logger) {
	// Browser: logging not yet implemented
}

// Logger returns the current logger used by the wgpu stack.
func Logger() *slog.Logger {
	return nil
}

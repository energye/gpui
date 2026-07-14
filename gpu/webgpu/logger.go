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

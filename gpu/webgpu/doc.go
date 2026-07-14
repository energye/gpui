// Package webgpu provides a safe, ergonomic WebGPU API for Go applications.
//
// This package is backed by wgpu-native through gpu/rwgpu.
//
// # Quick Start
//
// Import this package:
//
//	import (
//	    "github.com/energye/gpui/gpu/webgpu"
//	)
//
//	instance, err := webgpu.CreateInstance(nil)
//	// ...
//
// # Resource Lifecycle
//
// All GPU resources must be explicitly released with Release().
// Resources are reference-counted internally. Using a released resource panics.
//
// # Thread Safety
//
// Instance, Adapter, and Device are safe for concurrent use.
// Encoders (CommandEncoder, RenderPassEncoder, ComputePassEncoder) are NOT thread-safe.
package webgpu

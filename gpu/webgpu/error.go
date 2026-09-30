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
	"errors"
)

// Public API sentinel errors.
var (
	// ErrReleased is returned when operating on a released resource.
	ErrReleased = errors.New("wgpu: resource already released")

	// ErrNoAdapters is returned when no GPU adapters are found.
	ErrNoAdapters = errors.New("wgpu: no GPU adapters available")

	// ErrNoBackends is returned when no backends are registered.
	ErrNoBackends = errors.New("wgpu: no backends registered (import a backend package)")

	// ErrDeviceLost 见 hal.ErrDeviceLost（去别名：直接用 hal. 前缀）。

	// ErrRecovered: one skip after AutoRecover (VRAM/driver settle).
	ErrRecovered = errors.New("wgpu: device recovered, skip frame")

	// ErrOutOfMemory is returned when the GPU is out of memory.
	ErrOutOfMemory = errors.New("wgpu: out of memory")

	// ErrInvalidHandle is returned when operating on a nil/released/zero handle.
	ErrInvalidHandle = errors.New("wgpu: invalid handle")

	// ErrFrameInFlight is returned when BeginFrame is called while a prior
	// frame has not been EndFrame/DiscardFrame'd (pairing violation).
	ErrFrameInFlight = errors.New("wgpu: frame already in flight (BeginFrame/Present unpaired)")

	// ErrNoFrame is returned when EndFrame/Present is called without a live frame.
	ErrNoFrame = errors.New("wgpu: no frame in flight")

	// ErrSurfaceLost / ErrSurfaceOutdated 见 hal 同名（去别名：直接用 hal. 前缀）。

	// ErrSurfaceOccluded is returned when the window is minimized/covered and
	// no surface texture is available. Callers should skip the frame (do not reconfigure).
	ErrSurfaceOccluded = errors.New("wgpu: surface occluded")

	// ErrTimeout 见 hal.ErrTimeout（去别名：直接用 hal. 前缀）。

	// ErrSubmitCommandBufferInvalid is returned when a command buffer is submitted twice.
	ErrSubmitCommandBufferInvalid = errors.New("wgpu: command buffer already submitted")

	// ErrSubmitBufferDestroyed is returned when a submitted command buffer references a destroyed buffer.
	ErrSubmitBufferDestroyed = errors.New("wgpu: submitted command buffer references destroyed buffer")

	// ErrSubmitBufferMapped is returned when a submitted command buffer references a mapped buffer.
	ErrSubmitBufferMapped = errors.New("wgpu: submitted command buffer references mapped buffer")

	// ErrSubmitTextureDestroyed is returned when a submitted command buffer references a destroyed texture.
	ErrSubmitTextureDestroyed = errors.New("wgpu: submitted command buffer references destroyed texture")

	// ErrSubmitBindGroupDestroyed is returned when a submitted command buffer references a destroyed bind group.
	ErrSubmitBindGroupDestroyed = errors.New("wgpu: submitted command buffer references destroyed bind group")
)

// Draw-time validation sentinel errors.
var (
	ErrDrawMissingPipeline            = errors.New("wgpu: draw called without SetPipeline")
	ErrDrawMissingBindGroup           = errors.New("wgpu: draw called with missing bind group")
	ErrDrawIncompatibleBindGroup      = errors.New("wgpu: draw called with incompatible bind group layout")
	ErrDrawMissingVertexBuffer        = errors.New("wgpu: draw called with insufficient vertex buffers")
	ErrDrawMissingIndexBuffer         = errors.New("wgpu: DrawIndexed called without SetIndexBuffer")
	ErrDrawMissingBlendConstant       = errors.New("wgpu: draw called without SetBlendConstant (pipeline uses constant blend factor)")
	ErrDrawLateBufferTooSmall         = errors.New("wgpu: bound buffer smaller than shader-required minimum")
	ErrDispatchMissingPipeline        = errors.New("wgpu: dispatch called without SetPipeline")
	ErrDispatchMissingBindGroup       = errors.New("wgpu: dispatch called with missing bind group")
	ErrDispatchIncompatibleBindGroup  = errors.New("wgpu: dispatch called with incompatible bind group layout")
	ErrDispatchLateBufferTooSmall     = errors.New("wgpu: dispatch: bound buffer smaller than shader-required minimum")
	ErrDispatchWorkgroupCountExceeded = errors.New("wgpu: dispatch workgroup count exceeds device limit")

	ErrDrawIndexFormatMismatch         = errors.New("wgpu: index buffer format does not match pipeline strip index format")
	ErrDrawIndirectBufferUsage         = errors.New("wgpu: indirect draw buffer missing INDIRECT usage")
	ErrDrawIndirectOffsetAlignment     = errors.New("wgpu: indirect draw buffer offset not 4-byte aligned")
	ErrDispatchIndirectBufferUsage     = errors.New("wgpu: indirect dispatch buffer missing INDIRECT usage")
	ErrDispatchIndirectOffsetAlignment = errors.New("wgpu: indirect dispatch buffer offset not 4-byte aligned")
	ErrDrawIndirectBufferOverrun       = errors.New("wgpu: indirect draw args exceed buffer size")
	ErrDispatchIndirectBufferOverrun   = errors.New("wgpu: indirect dispatch args exceed buffer size")
)

// GPUError / ErrorFilter（含 Validation/OutOfMemory/Internal）见 hal 同名
//（去别名：直接用 hal. 前缀；wasm 侧见 error_browser.go 独立定义）。

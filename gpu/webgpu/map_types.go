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

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
)

const (
	MapModeRead  = types.MapModeRead
	MapModeWrite = types.MapModeWrite
)

// MapState reports the current mapping state of a buffer.
type MapState uint8

const (
	MapStateUnmapped MapState = iota
	MapStatePending
	MapStateMapped
	MapStateDestroyed
)

const (
	PollPoll = hal.PollPoll
	PollWait = hal.PollWait
)

// Typed buffer mapping errors.
var (
	ErrMapAlreadyPending = errors.New("wgpu: buffer map already pending")
	ErrMapAlreadyMapped  = errors.New("wgpu: buffer is already mapped")
	ErrMapNotMapped      = errors.New("wgpu: buffer is not mapped")
	ErrMapRangeOverlap   = errors.New("wgpu: mapped range overlaps existing")
	ErrMapRangeDetached  = errors.New("wgpu: mapped range detached (buffer unmapped)")
	ErrMapAlignment      = errors.New("wgpu: map offset/size not aligned")
	ErrMapCanceled       = errors.New("wgpu: map canceled")
	ErrBufferDestroyed   = errors.New("wgpu: buffer destroyed")
	ErrMapDeviceLost     = errors.New("wgpu: device lost during map")
	ErrMapInvalidMode    = errors.New("wgpu: buffer not created with required MAP_READ/MAP_WRITE usage")
	ErrMapRangeOverflow  = errors.New("wgpu: map range exceeds buffer size")
)

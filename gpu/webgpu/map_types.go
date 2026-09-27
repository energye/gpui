//go:build !(js && wasm)

package webgpu

import (
	"errors"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
)

// MapMode selects the type of access requested for a buffer mapping.
// Canonical form lives in gpu/types (片4已对齐，以 webgpu/map_types.go:8 为准);
// this alias keeps webgpu callers compiling while render uses types.* directly.
// Full removal waits for slice 7f收尾.
type MapMode = types.MapMode

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

// PollType selects the blocking behavior of Device.Poll.
// Canonical form lives in gpu/hal (mirrors webgpu PollType);
// alias keeps webgpu callers compiling while render uses hal.* directly.
// Full removal waits for slice 7f收尾.
type PollType = hal.PollType

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

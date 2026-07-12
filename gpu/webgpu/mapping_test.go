//go:build !rust && !(js && wasm)

package webgpu_test

import (
	"context"
	"errors"
	"testing"

	"github.com/energye/gpui/gpu/webgpu"
)

// =============================================================================
// MapPending lifecycle tests
// Covers map_pending.go missed lines (Status, Wait, Release, nil guards)
// =============================================================================

func TestMapPendingStatusOnNil(t *testing.T) {
	// A nil MapPending should return ready=true with ErrMapCanceled.
	var pending *webgpu.MapPending
	ready, err := pending.Status()
	if !ready {
		t.Error("Status on nil MapPending should return ready=true")
	}
	if !errors.Is(err, webgpu.ErrMapCanceled) {
		t.Errorf("Status on nil: got %v, want ErrMapCanceled", err)
	}
}

func TestMapPendingWaitOnNil(t *testing.T) {
	// A nil MapPending.Wait should return ErrMapCanceled.
	var pending *webgpu.MapPending
	err := pending.Wait(context.Background())
	if !errors.Is(err, webgpu.ErrMapCanceled) {
		t.Errorf("Wait on nil: got %v, want ErrMapCanceled", err)
	}
}

func TestMapPendingReleaseOnNil(t *testing.T) {
	// Release on nil should not panic.
	var pending *webgpu.MapPending
	pending.Release()
}

func TestMapPendingStatusResolvesImmediately(t *testing.T) {
	_, _, device := newDevice(t)
	defer device.Release()
	requireHAL(t, device)

	buf, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label:            "pending-status-buf",
		Size:             64,
		Usage:            webgpu.BufferUsageMapRead | webgpu.BufferUsageCopySrc,
		MappedAtCreation: true,
	})
	if err != nil {
		t.Fatalf("CreateBuffer: %v", err)
	}
	defer buf.Release()

	// Unmap so we can MapAsync.
	if err := buf.Unmap(); err != nil {
		t.Fatalf("Unmap: %v", err)
	}

	pending, err := buf.MapAsync(webgpu.MapModeRead, 0, 64)
	if err != nil {
		t.Fatalf("MapAsync: %v", err)
	}
	defer pending.Release()

	// Poll to drive the map to completion.
	device.Poll(webgpu.PollWait)

	ready, mapErr := pending.Status()
	if !ready {
		t.Error("Status should be ready after PollWait")
	}
	if mapErr != nil {
		t.Errorf("Status error: %v", mapErr)
	}
}

func TestMapPendingStatusCachedAfterResolve(t *testing.T) {
	_, _, device := newDevice(t)
	defer device.Release()
	requireHAL(t, device)

	buf, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label:            "pending-cached-buf",
		Size:             64,
		Usage:            webgpu.BufferUsageMapRead | webgpu.BufferUsageCopySrc,
		MappedAtCreation: true,
	})
	if err != nil {
		t.Fatalf("CreateBuffer: %v", err)
	}
	defer buf.Release()

	if err := buf.Unmap(); err != nil {
		t.Fatalf("Unmap: %v", err)
	}

	pending, err := buf.MapAsync(webgpu.MapModeRead, 0, 64)
	if err != nil {
		t.Fatalf("MapAsync: %v", err)
	}
	defer pending.Release()

	device.Poll(webgpu.PollWait)

	// First call resolves.
	ready1, _ := pending.Status()
	// Second call returns cached result.
	ready2, _ := pending.Status()

	if !ready1 || !ready2 {
		t.Error("both Status calls should return ready=true")
	}
}

func TestMapPendingWaitResolvesImmediately(t *testing.T) {
	_, _, device := newDevice(t)
	defer device.Release()
	requireHAL(t, device)

	buf, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label:            "pending-wait-buf",
		Size:             64,
		Usage:            webgpu.BufferUsageMapRead | webgpu.BufferUsageCopySrc,
		MappedAtCreation: true,
	})
	if err != nil {
		t.Fatalf("CreateBuffer: %v", err)
	}
	defer buf.Release()

	if err := buf.Unmap(); err != nil {
		t.Fatalf("Unmap: %v", err)
	}

	pending, err := buf.MapAsync(webgpu.MapModeRead, 0, 64)
	if err != nil {
		t.Fatalf("MapAsync: %v", err)
	}
	defer pending.Release()

	// Poll first so it resolves.
	device.Poll(webgpu.PollWait)

	// Wait after resolve returns cached result.
	err = pending.Wait(context.Background())
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
}

func TestMapPendingWaitContextCancel(t *testing.T) {
	_, _, device := newDevice(t)
	defer device.Release()
	requireHAL(t, device)

	buf, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "pending-cancel-buf",
		Size:  64,
		Usage: webgpu.BufferUsageMapRead | webgpu.BufferUsageCopySrc,
	})
	if err != nil {
		t.Fatalf("CreateBuffer: %v", err)
	}
	defer buf.Release()

	pending, err := buf.MapAsync(webgpu.MapModeRead, 0, 64)
	if err != nil {
		t.Fatalf("MapAsync: %v", err)
	}
	defer pending.Release()

	// Use an already-canceled context.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Poll once to check.
	device.Poll(webgpu.PollPoll)

	err = pending.Wait(ctx)
	// Should return context cancellation or map success (if resolved during poll).
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Errorf("Wait with canceled ctx: got %v, want context.Canceled or nil", err)
	}
}

// =============================================================================
// MapMode conversion tests
// Covers map_types.go MapMode.toInternal and mapStateFromCore
// =============================================================================

func TestMapModeConstants(t *testing.T) {
	if webgpu.MapModeRead != 1 {
		t.Errorf("MapModeRead = %d, want 1", webgpu.MapModeRead)
	}
	if webgpu.MapModeWrite != 2 {
		t.Errorf("MapModeWrite = %d, want 2", webgpu.MapModeWrite)
	}
}

func TestMapStateConstants(t *testing.T) {
	// Verify the enum values are stable.
	if webgpu.MapStateUnmapped != 0 {
		t.Errorf("MapStateUnmapped = %d, want 0", webgpu.MapStateUnmapped)
	}
	if webgpu.MapStatePending != 1 {
		t.Errorf("MapStatePending = %d, want 1", webgpu.MapStatePending)
	}
	if webgpu.MapStateMapped != 2 {
		t.Errorf("MapStateMapped = %d, want 2", webgpu.MapStateMapped)
	}
	if webgpu.MapStateDestroyed != 3 {
		t.Errorf("MapStateDestroyed = %d, want 3", webgpu.MapStateDestroyed)
	}
}

func TestPollTypeConstants(t *testing.T) {
	if webgpu.PollPoll != 0 {
		t.Errorf("PollPoll = %d, want 0", webgpu.PollPoll)
	}
	if webgpu.PollWait != 1 {
		t.Errorf("PollWait = %d, want 1", webgpu.PollWait)
	}
}

// =============================================================================
// Typed error mapping tests (coreErrToTyped coverage)
// Covers map_types.go lines 142-173
// =============================================================================

func TestMappingErrorSentinels(t *testing.T) {
	// Verify all mapping error sentinels are distinct and non-nil.
	errs := []error{
		webgpu.ErrMapAlreadyPending,
		webgpu.ErrMapAlreadyMapped,
		webgpu.ErrMapNotMapped,
		webgpu.ErrMapRangeOverlap,
		webgpu.ErrMapRangeDetached,
		webgpu.ErrMapAlignment,
		webgpu.ErrMapCanceled,
		webgpu.ErrBufferDestroyed,
		webgpu.ErrMapDeviceLost,
		webgpu.ErrMapInvalidMode,
		webgpu.ErrMapRangeOverflow,
	}

	for i, err := range errs {
		if err == nil {
			t.Errorf("mapping error sentinel [%d] is nil", i)
		}
		for j := i + 1; j < len(errs); j++ {
			if errors.Is(errs[i], errs[j]) {
				t.Errorf("mapping error sentinel [%d] (%v) equals [%d] (%v)", i, errs[i], j, errs[j])
			}
		}
	}
}

func TestMappingErrorMessages(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantSub string
	}{
		{"AlreadyPending", webgpu.ErrMapAlreadyPending, "already pending"},
		{"AlreadyMapped", webgpu.ErrMapAlreadyMapped, "already mapped"},
		{"NotMapped", webgpu.ErrMapNotMapped, "not mapped"},
		{"RangeOverlap", webgpu.ErrMapRangeOverlap, "overlaps"},
		{"RangeDetached", webgpu.ErrMapRangeDetached, "detached"},
		{"Alignment", webgpu.ErrMapAlignment, "aligned"},
		{"Canceled", webgpu.ErrMapCanceled, "canceled"},
		{"Destroyed", webgpu.ErrBufferDestroyed, "destroyed"},
		{"DeviceLost", webgpu.ErrMapDeviceLost, "device lost"},
		{"InvalidMode", webgpu.ErrMapInvalidMode, "MAP_READ/MAP_WRITE"},
		{"RangeOverflow", webgpu.ErrMapRangeOverflow, "exceeds"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.err.Error()
			if msg == "" {
				t.Error("error message is empty")
			}
		})
	}
}

// =============================================================================
// MappedRange tests — Bytes(), Len(), Offset(), Release()
// Covers mapped_range.go missed lines
// =============================================================================

func TestMappedRangeOnNil(t *testing.T) {
	var mr *webgpu.MappedRange

	if got := mr.Bytes(); got != nil {
		t.Errorf("nil MappedRange.Bytes() = %v, want nil", got)
	}
	if got := mr.Len(); got != 0 {
		t.Errorf("nil MappedRange.Len() = %d, want 0", got)
	}
	if got := mr.Offset(); got != 0 {
		t.Errorf("nil MappedRange.Offset() = %d, want 0", got)
	}

	// Release on nil should not panic.
	mr.Release()
}

func TestMappedRangeValid(t *testing.T) {
	_, _, device := newDevice(t)
	defer device.Release()
	requireHAL(t, device)

	buf, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label:            "mr-valid",
		Size:             64,
		Usage:            webgpu.BufferUsageMapWrite | webgpu.BufferUsageCopySrc,
		MappedAtCreation: true,
	})
	if err != nil {
		t.Fatalf("CreateBuffer: %v", err)
	}
	defer buf.Release()

	rng, err := buf.MappedRange(0, 64)
	if err != nil {
		t.Fatalf("MappedRange: %v", err)
	}

	if rng.Len() != 64 {
		t.Errorf("Len() = %d, want 64", rng.Len())
	}
	if rng.Offset() != 0 {
		t.Errorf("Offset() = %d, want 0", rng.Offset())
	}

	data := rng.Bytes()
	if data == nil {
		t.Fatal("Bytes() returned nil on valid mapped range")
	}
	if len(data) != 64 {
		t.Errorf("len(Bytes()) = %d, want 64", len(data))
	}

	rng.Release()
}

func TestMappedRangeDetachedAfterUnmap(t *testing.T) {
	_, _, device := newDevice(t)
	defer device.Release()
	requireHAL(t, device)

	buf, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label:            "mr-detach",
		Size:             64,
		Usage:            webgpu.BufferUsageMapWrite | webgpu.BufferUsageCopySrc,
		MappedAtCreation: true,
	})
	if err != nil {
		t.Fatalf("CreateBuffer: %v", err)
	}
	defer buf.Release()

	rng, err := buf.MappedRange(0, 64)
	if err != nil {
		t.Fatalf("MappedRange: %v", err)
	}

	// Bytes should be valid before unmap.
	if rng.Bytes() == nil {
		t.Fatal("Bytes() should be non-nil before Unmap")
	}

	if err := buf.Unmap(); err != nil {
		t.Fatalf("Unmap: %v", err)
	}

	// After unmap, generation mismatch should cause Bytes() to return nil.
	if rng.Bytes() != nil {
		t.Error("Bytes() should return nil after Unmap (generation mismatch)")
	}

	rng.Release()
}

func TestMappedRangeAfterRelease(t *testing.T) {
	_, _, device := newDevice(t)
	defer device.Release()
	requireHAL(t, device)

	buf, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label:            "mr-after-release",
		Size:             32,
		Usage:            webgpu.BufferUsageMapWrite | webgpu.BufferUsageCopySrc,
		MappedAtCreation: true,
	})
	if err != nil {
		t.Fatalf("CreateBuffer: %v", err)
	}
	defer buf.Release()

	rng, err := buf.MappedRange(0, 32)
	if err != nil {
		t.Fatalf("MappedRange: %v", err)
	}

	rng.Release()

	// After Release, the MappedRange is pooled and zeroed.
	// Bytes/Len/Offset should return zero values.
	if rng.Bytes() != nil {
		t.Error("Bytes() should return nil after Release")
	}
}

func TestMappedRangeAlignmentError(t *testing.T) {
	_, _, device := newDevice(t)
	defer device.Release()
	requireHAL(t, device)

	buf, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label:            "mr-align",
		Size:             64,
		Usage:            webgpu.BufferUsageMapWrite | webgpu.BufferUsageCopySrc,
		MappedAtCreation: true,
	})
	if err != nil {
		t.Fatalf("CreateBuffer: %v", err)
	}
	defer buf.Release()

	// Offset not multiple of 8 should fail.
	_, err = buf.MappedRange(3, 4)
	if !errors.Is(err, webgpu.ErrMapAlignment) {
		t.Errorf("MappedRange(offset=3): got %v, want ErrMapAlignment", err)
	}

	// Size not multiple of 4 should fail.
	_, err = buf.MappedRange(0, 5)
	if !errors.Is(err, webgpu.ErrMapAlignment) {
		t.Errorf("MappedRange(size=5): got %v, want ErrMapAlignment", err)
	}
}

func TestMappedRangeOnUnmappedBuffer(t *testing.T) {
	_, _, device := newDevice(t)
	defer device.Release()
	requireHAL(t, device)

	buf, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "mr-unmapped",
		Size:  64,
		Usage: webgpu.BufferUsageVertex,
	})
	if err != nil {
		t.Fatalf("CreateBuffer: %v", err)
	}
	defer buf.Release()

	// Buffer is not mapped — MappedRange should fail.
	_, err = buf.MappedRange(0, 64)
	if err == nil {
		t.Fatal("MappedRange on unmapped buffer should fail")
	}
}

func TestMappedRangeOnNilBuffer(t *testing.T) {
	var buf *webgpu.Buffer
	_, err := buf.MappedRange(0, 64)
	if !errors.Is(err, webgpu.ErrReleased) {
		t.Errorf("MappedRange on nil buffer: got %v, want ErrReleased", err)
	}
}

// =============================================================================
// Buffer.Map error paths
// Covers buffer.go Map/MapAsync missed lines
// =============================================================================

func TestBufferMapOnNilBuffer(t *testing.T) {
	var buf *webgpu.Buffer
	err := buf.Map(context.Background(), webgpu.MapModeRead, 0, 64)
	if !errors.Is(err, webgpu.ErrReleased) {
		t.Errorf("Map on nil buffer: got %v, want ErrReleased", err)
	}
}

func TestBufferMapAsyncOnNilBuffer(t *testing.T) {
	var buf *webgpu.Buffer
	_, err := buf.MapAsync(webgpu.MapModeRead, 0, 64)
	if !errors.Is(err, webgpu.ErrReleased) {
		t.Errorf("MapAsync on nil buffer: got %v, want ErrReleased", err)
	}
}

func TestBufferUnmapOnNilBuffer(t *testing.T) {
	var buf *webgpu.Buffer
	err := buf.Unmap()
	if !errors.Is(err, webgpu.ErrReleased) {
		t.Errorf("Unmap on nil buffer: got %v, want ErrReleased", err)
	}
}

func TestBufferMapWrongUsage(t *testing.T) {
	_, _, device := newDevice(t)
	defer device.Release()
	requireHAL(t, device)

	// Buffer without MapRead usage.
	buf, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "map-wrong-usage",
		Size:  64,
		Usage: webgpu.BufferUsageVertex,
	})
	if err != nil {
		t.Fatalf("CreateBuffer: %v", err)
	}
	defer buf.Release()

	_, err = buf.MapAsync(webgpu.MapModeRead, 0, 64)
	if !errors.Is(err, webgpu.ErrMapInvalidMode) {
		t.Errorf("MapAsync wrong usage: got %v, want ErrMapInvalidMode", err)
	}
}

func TestBufferMapAlreadyMapped(t *testing.T) {
	_, _, device := newDevice(t)
	defer device.Release()
	requireHAL(t, device)

	buf, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label:            "map-already-mapped",
		Size:             64,
		Usage:            webgpu.BufferUsageMapWrite | webgpu.BufferUsageCopySrc,
		MappedAtCreation: true,
	})
	if err != nil {
		t.Fatalf("CreateBuffer: %v", err)
	}
	defer buf.Release()

	// Buffer is already mapped from MappedAtCreation.
	_, err = buf.MapAsync(webgpu.MapModeWrite, 0, 64)
	if !errors.Is(err, webgpu.ErrMapAlreadyMapped) {
		t.Errorf("MapAsync on already mapped: got %v, want ErrMapAlreadyMapped", err)
	}
}

func TestBufferMapContextAlreadyCanceled(t *testing.T) {
	_, _, device := newDevice(t)
	defer device.Release()
	requireHAL(t, device)

	buf, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "map-ctx-cancel",
		Size:  64,
		Usage: webgpu.BufferUsageMapRead | webgpu.BufferUsageCopySrc,
	})
	if err != nil {
		t.Fatalf("CreateBuffer: %v", err)
	}
	defer buf.Release()

	// Already-canceled context should return error immediately.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = buf.Map(ctx, webgpu.MapModeRead, 0, 64)
	if err == nil {
		t.Fatal("Map with already-canceled context should fail")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Map with canceled ctx: got %v, want context.Canceled", err)
	}
}

func TestBufferMapNilContext(t *testing.T) {
	_, _, device := newDevice(t)
	defer device.Release()
	requireHAL(t, device)

	buf, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label:            "map-nil-ctx",
		Size:             64,
		Usage:            webgpu.BufferUsageMapRead | webgpu.BufferUsageCopySrc,
		MappedAtCreation: true,
	})
	if err != nil {
		t.Fatalf("CreateBuffer: %v", err)
	}
	defer buf.Release()

	// Unmap first so we can map.
	if err := buf.Unmap(); err != nil {
		t.Fatalf("Unmap: %v", err)
	}

	// nil context should be replaced with Background.
	//nolint:staticcheck // intentionally testing nil context
	err = buf.Map(nil, webgpu.MapModeRead, 0, 64)
	if err != nil {
		t.Fatalf("Map with nil context should succeed: %v", err)
	}
}

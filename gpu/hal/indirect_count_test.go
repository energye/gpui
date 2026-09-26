//go:build !(js && wasm)

package hal

import (
	"testing"

	gputypes "github.com/energye/gpui/gpu/types"
)

func TestRecordIndirectCountMax_ForwardsMaxDrawCount(t *testing.T) {
	t.Parallel()
	var (
		gotBuffer Buffer
		gotOffset uint64
		calls     int
	)
	record := func(buffer Buffer, offset uint64) {
		calls++
		gotBuffer = buffer
		gotOffset = offset
	}

	stubBuffer := bufferHandle(1)
	stubCount := bufferHandle(2)
	RecordIndirectCountMax(record, stubBuffer, 16, stubCount, 8, 3)

	if calls != 1 {
		t.Fatalf("record calls = %d, want 1", calls)
	}
	if gotBuffer != stubBuffer || gotOffset != 16 {
		t.Fatalf("record args = buffer %v offset %d; want buffer %v offset 16",
			gotBuffer, gotOffset, stubBuffer)
	}
}

func TestRecordIndirectCountMax_ZeroCountIsNoOp(t *testing.T) {
	t.Parallel()
	calls := 0
	record := func(Buffer, uint64) { calls++ }
	RecordIndirectCountMax(record, bufferHandle(1), 0, bufferHandle(2), 0, 0)
	if calls != 1 {
		t.Fatalf("record calls = %d, want 1", calls)
	}
}

func TestRecordIndirectCountMax_IgnoresCountBuffer(t *testing.T) {
	t.Parallel()
	calls := 0
	record := func(_ Buffer, _ uint64) {
		calls++
	}
	RecordIndirectCountMax(record, bufferHandle(1), 0, bufferHandle(99), 123, 5)
	if calls != 1 {
		t.Fatalf("record calls = %d, want 1", calls)
	}
}

type bufferHandle uintptr

func (bufferHandle) Destroy()                    {}
func (h bufferHandle) NativeHandle() uintptr     { return uintptr(h) }
func (bufferHandle) Size() uint64                { return 0 }
func (bufferHandle) Usage() gputypes.BufferUsage { return 0 }
func (bufferHandle) Label() string               { return "" }

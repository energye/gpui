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

package hal

// IndirectCountRecorder issues a multi-draw indirect call.
type IndirectCountRecorder func(buffer Buffer, offset uint64)

// RecordIndirectCountMax records an indirect draw using maxDrawCount when the
// backend cannot consume a GPU-provided count buffer.
func RecordIndirectCountMax(
	record IndirectCountRecorder,
	buffer Buffer,
	offset uint64,
	countBuffer Buffer,
	countOffset uint64,
	maxDrawCount uint32,
) {
	_ = countBuffer
	_ = countOffset
	_ = maxDrawCount
	record(buffer, offset)
}

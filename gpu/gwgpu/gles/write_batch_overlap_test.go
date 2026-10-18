//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build (windows || linux) && !(js && wasm)

package gles

import "testing"

// TestCountStagedWriteOverlaps pins the upload guard's decision table. The
// merged-present regression staged 46 fills at the same offset inside one
// drain; the drain uploaded one value and every earlier fill read it. Per
// command-buffer drains and merely touching ranges are healthy.
func TestCountStagedWriteOverlaps(t *testing.T) {
	write := func(off uint64, n int) pendingWrite {
		return pendingWrite{offset: off, data: make([]byte, n)}
	}
	sameOffset46 := make([]pendingWrite, 46)
	for i := range sameOffset46 {
		sameOffset46[i] = write(0, 384)
	}
	cases := []struct {
		name string
		in   []pendingWrite
		want int
	}{
		{"single write", []pendingWrite{write(0, 384)}, 0},
		{"disjoint ranges", []pendingWrite{write(0, 128), write(128, 128)}, 0},
		{"touching ranges", []pendingWrite{write(0, 384), write(384, 384)}, 0},
		{"partial overlap", []pendingWrite{write(0, 384), write(128, 384)}, 1},
		{"contained write", []pendingWrite{write(0, 384), write(64, 16)}, 1},
		{"gap then overlap", []pendingWrite{write(0, 128), write(512, 128), write(512, 64)}, 1},
		{"regression shape: 46 writes at offset 0", sameOffset46, 45},
	}
	for _, tc := range cases {
		if got := countStagedWriteOverlaps(tc.in); got != tc.want {
			t.Errorf("%s: countStagedWriteOverlaps = %d, want %d", tc.name, got, tc.want)
		}
	}
}

// TestNoteStagedWriteOverlapsCounts checks the counter accumulates the guard
// value and survives the nil-buffer path (diagnostic must never panic).
func TestNoteStagedWriteOverlapsCounts(t *testing.T) {
	buf := &Buffer{id: 7, label: "test_buf"}
	before := stagedWriteOverlapCount()
	noteStagedWriteOverlaps(buf, 46, 45, 384)
	if got := stagedWriteOverlapCount() - before; got != 45 {
		t.Fatalf("counter delta = %d, want 45", got)
	}
	before = stagedWriteOverlapCount()
	noteStagedWriteOverlaps(nil, 2, 1, 16)
	if got := stagedWriteOverlapCount() - before; got != 1 {
		t.Fatalf("nil-buffer counter delta = %d, want 1", got)
	}
}

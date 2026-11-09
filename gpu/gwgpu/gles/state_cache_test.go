//go:build linux || windows

package gles

import "testing"

func TestBaseVertexAttribOffset(t *testing.T) {
	cases := []struct {
		name       string
		baseOffset uintptr
		baseVertex int32
		stride     int32
		want       uintptr
	}{
		{"zero", 0, 0, 16, 0},
		{"one vertex", 0, 1, 16, 16},
		{"offset plus two", 32, 2, 16, 64},
		{"negative is not folded", 32, -1, 16, 32},
		{"zero stride", 32, 4, 0, 32},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := baseVertexAttribOffset(tc.baseOffset, tc.baseVertex, tc.stride); got != tc.want {
				t.Fatalf("baseVertexAttribOffset(%d,%d,%d) = %d want %d", tc.baseOffset, tc.baseVertex, tc.stride, got, tc.want)
			}
		})
	}
}

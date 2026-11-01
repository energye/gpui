package main

import "fmt"

// Replicates the walk arithmetic of path1DEffect.FilterPath and the 2D grid
// loop, counting iterations WITHOUT allocating stamps.
func walk1D(offset, length, advance float64, cap int) (iters int, stalled bool) {
	d := offset
	prev := d
	for d < length {
		iters++
		if iters > cap {
			return iters, true
		}
		d += advance
		if d == prev { // float step cannot progress
			return iters, true
		}
		prev = d
	}
	return iters, false
}

func main() {
	for _, c := range [][3]float64{
		{0, 20, 1e-9},   // absurdly small advance on a short contour
		{0, 1000, 1e-6}, // typical "hairline stamp" typo
		{1 << 30, 1 << 31, 1e-3}, // advance below ULP(distance): never progresses
	} {
		it, stall := walk1D(c[0], c[1], c[2], 5_000_000)
		fmt.Printf("1D offset=%g length=%g advance=%g -> iterations until done: %d (capped/stalled=%v)\n",
			c[0], c[1], c[2], it, stall)
	}
}

package h264

import (
	"testing"
)

// S1b-I gate: PrimeFrame retains the new roster BEFORE releasing the
// old one. Consecutive tasks share anchors (same picture in both
// rosters); releasing first drops a shared picture to zero, recycles
// it into the pool, and a later acquire hands its buffers to another
// frame while the roster still points at them (1080p parallel caught
// it: duplicated snapshots, "skip without reference"). This test
// primes overlapping rosters and asserts the pool never hands out a
// live roster picture.
func TestPoolPrimeOverlapKeepsShared(t *testing.T) {
	mk := func() *Picture {
		p, err := NewPicture(64, 64)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	a, b := mk(), mk()
	d := NewDecoder(NewParamSets())
	d.PrimeFrame([]*Picture{a}, POCSeed{})
	d.PrimeFrame([]*Picture{a, b}, POCSeed{})
	c, err := acquirePicture(64, 64)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Release()
	if len(c.Y) == 0 || len(a.Y) == 0 || len(b.Y) == 0 {
		t.Fatalf("empty planes")
	}
	if &c.Y[0] == &a.Y[0] || &c.Y[0] == &b.Y[0] {
		t.Fatalf("pool handed out a live roster picture")
	}
	d.DropBuffered()
}

// S1b-I gate: the pool actually recycles (release returns buffers, the
// next acquire of the same size reuses them instead of mallocing).
func TestPoolRecyclesBuffers(t *testing.T) {
	p, err := acquirePicture(64, 64)
	if err != nil {
		t.Fatal(err)
	}
	addr := &p.Y[0]
	p.Release()
	q, err := acquirePicture(64, 64)
	if err != nil {
		t.Fatal(err)
	}
	defer q.Release()
	if &q.Y[0] != addr {
		t.Fatalf("pool did not recycle released buffers")
	}
}

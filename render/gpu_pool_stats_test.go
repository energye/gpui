package render

import "testing"

// gpu_pool_stats_test.go pins the probe bridge the GPU layer publishes its pool
// reuse counters through (P2.4d): add / remove by owner identity, snapshot
// aggregation, and the hit-rate reading a real-window gate asserts on.

func stubPoolStats(name string, hits, misses uint64) GPUPoolStatsFunc {
	return func() []PoolDomainStats {
		return []PoolDomainStats{{Device: "test-device", Name: name, Hits: hits, Misses: misses}}
	}
}

func TestGPUPoolStats_RegisterSnapshotUnregister(t *testing.T) {
	ownerA, ownerB := new(any), new(any)
	RegisterGPUPoolStatsCollector(ownerA, stubPoolStats("domain-a", 7, 3))
	RegisterGPUPoolStatsCollector(ownerB, stubPoolStats("domain-b", 1, 1))
	t.Cleanup(func() {
		UnregisterGPUPoolStatsCollector(ownerA)
		UnregisterGPUPoolStatsCollector(ownerB)
	})

	got := GPUPoolStats()
	if len(got) < 2 {
		t.Fatalf("GPUPoolStats returned %d domains, want both collectors", len(got))
	}
	var a, b bool
	for _, st := range got {
		switch st.Name {
		case "domain-a":
			a = true
			if st.Hits != 7 || st.Misses != 3 || st.Device != "test-device" {
				t.Fatalf("domain-a snapshot = %+v, want hits=7 misses=3 device=test-device", st)
			}
		case "domain-b":
			b = true
		}
	}
	if !a || !b {
		t.Fatalf("missing domain in snapshot (a=%v b=%v): %+v", a, b, got)
	}

	UnregisterGPUPoolStatsCollector(ownerA)
	for _, st := range GPUPoolStats() {
		if st.Name == "domain-a" {
			t.Fatal("unregistered collector still reporting")
		}
	}
}

func TestRegisterGPUPoolStatsCollector_NilIgnored(t *testing.T) {
	owner := new(any)
	RegisterGPUPoolStatsCollector(owner, nil)
	RegisterGPUPoolStatsCollector(nil, stubPoolStats("domain-nil", 1, 0))
	before := len(GPUPoolStats())
	UnregisterGPUPoolStatsCollector(owner)
	if after := len(GPUPoolStats()); after != before {
		t.Fatalf("nil registration changed the domain count: %d → %d", before, after)
	}
}

func TestPoolDomainStats_HitRate(t *testing.T) {
	cases := []struct {
		hits, misses uint64
		want         float64
	}{
		{0, 0, 0}, // never used: not a perfect score
		{3, 1, 0.75},
		{0, 4, 0},
		{4, 0, 1},
	}
	for _, c := range cases {
		st := PoolDomainStats{Hits: c.hits, Misses: c.misses}
		if got := st.HitRate(); got != c.want {
			t.Errorf("hits=%d misses=%d: HitRate=%v, want %v", c.hits, c.misses, got, c.want)
		}
	}
}

//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/render"
)

// pool_stats_probe_test.go pins the P2.4d reuse probe: every res.Cache domain
// of a device holder reports through render.GPUPoolStats(), the numbers are the
// domain's real counters (not a re-derivation), and a closed holder stops
// reporting so a stale pool can never be mistaken for a live one.

func surfaceTrioOf(s *GPUShared) *surfaceCache {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.surfaceCacheLocked()
	c.setDevice(s.device)
	return c
}

func findDomain(stats []render.PoolDomainStats, name string) (render.PoolDomainStats, bool) {
	for _, st := range stats {
		if st.Name == name {
			return st, true
		}
	}
	return render.PoolDomainStats{}, false
}

// TestGPUShared_PoolStats_ReportsEveryDomainWithRealCounters drives each shared
// domain through one reuse and checks the probe reads hits > 0, misses > 0 and
// the declared budget — the three numbers a real-window gate asserts on.
func TestGPUShared_PoolStats_ReportsEveryDomainWithRealCounters(t *testing.T) {
	device, s := newPooledShared(t)
	t.Cleanup(s.Close)

	trios := surfaceTrioOf(s)
	key := surfaceKey(64, 48, 1, false, false)
	refA, _, err := trios.acquire(device, key)
	if err != nil {
		t.Fatalf("surface acquire: %v", err)
	}
	trios.release(refA)
	refB, _, err := trios.acquire(device, key)
	if err != nil {
		t.Fatalf("surface acquire #2: %v", err)
	}
	trios.release(refB)

	recycleSnap(t, s, poolSnap(t, s, device, 256, 256))
	recycleSnap(t, s, poolSnap(t, s, device, 256, 256))
	recycleRT(t, s, poolRT(t, s, device, 128, 128))
	recycleRT(t, s, poolRT(t, s, device, 128, 128))

	stats := s.poolStats()
	wantBudget := map[string]uint64{
		"device-surface": uint64(render.DefaultBudget(render.BudgetSurfacePool).MaxBytes),
		"dualtex-snap":   uint64(render.DefaultBudget(render.BudgetBlendDualTex).MaxBytes),
		"filter-graph":   uint64(render.DefaultBudget(render.BudgetFilterGraph).MaxBytes),
	}
	for name, budget := range wantBudget {
		st, ok := findDomain(stats, name)
		if !ok {
			t.Fatalf("domain %q missing from probe: %+v", name, stats)
		}
		if st.Device != s.poolLabel {
			t.Fatalf("domain %q device label = %q, want %q", name, st.Device, s.poolLabel)
		}
		if st.Hits == 0 {
			t.Errorf("domain %q reports hits=0 after a same-key reuse: %+v", name, st)
		}
		if st.Misses == 0 {
			t.Errorf("domain %q reports misses=0: a fresh allocation must be counted", name)
		}
		if st.Budget != budget {
			t.Errorf("domain %q budget = %d, want the declared %d", name, st.Budget, budget)
		}
	}

	// The per-context layer pool is a separate domain and only shows up once a
	// context actually built it.
	rc := s.NewRenderContext()
	defer s.unregisterContext(rc)
	if _, owner := rc.offscreenCacheFor(); owner.IsNil() {
		t.Fatal("offscreen cache has no registry owner")
	}
	st, ok := findDomain(s.poolStats(), "layer-offscreen")
	if !ok {
		t.Fatal("layer-offscreen domain missing from probe")
	}
	if st.Device != s.poolLabel || st.Hits != 0 || st.Entries != 0 {
		t.Fatalf("fresh layer-offscreen domain = %+v, want this holder with zeros", st)
	}
}

// TestGPUShared_PoolStats_ClosedHolderStopsReporting is the stale-pool pin:
// after Close the natives are gone, so a snapshot that still listed this
// holder would let a real window assert on a dead pool.
func TestGPUShared_PoolStats_ClosedHolderStopsReporting(t *testing.T) {
	device, s := newPooledShared(t)
	label := s.poolLabel
	if label == "" {
		t.Fatal("device holder has no probe label")
	}
	recycleRT(t, s, poolRT(t, s, device, 64, 64))
	if _, ok := findDomain(s.poolStats(), "filter-graph"); !ok {
		t.Fatal("filter-graph domain missing before Close")
	}
	found := false
	for _, st := range render.GPUPoolStats() {
		if st.Device == label {
			found = true
		}
	}
	if !found {
		t.Fatal("holder not reachable through the public probe")
	}

	s.Close()
	for _, st := range render.GPUPoolStats() {
		if st.Device == label {
			t.Fatalf("closed holder %q still reporting: %+v", label, st)
		}
	}
}

// TestGPUShared_PoolStats_LabelsArePerHolder keeps the multi-window diagnostic
// honest: two device holders must not fold into one label.
func TestGPUShared_PoolStats_LabelsArePerHolder(t *testing.T) {
	_, a := newPooledShared(t)
	_, b := newPooledShared(t)
	t.Cleanup(a.Close)
	t.Cleanup(b.Close)
	if a.poolLabel == b.poolLabel {
		t.Fatalf("two holders share the probe label %q", a.poolLabel)
	}
}

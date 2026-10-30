//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import "sync"

// PoolDomainStats is one GPU pool domain's reuse snapshot (P2.4d probe).
// The GPU layer owns its pools internally; this is the read-only shape the
// engine publishes so a real window can assert that same-size reuse actually
// happens (hits > 0) instead of trusting a pool that silently allocates every
// frame.
//
// Entries counts checked-out plus pooled resources; Idle counts pooled entries
// nothing is using. Bytes is the domain's estimated live footprint (checked-out
// entries count against the budget too, so a burst cannot dodge the cap).
type PoolDomainStats struct {
	Device    string `json:"device"` // device holder label (multi-window diagnostics)
	Name      string `json:"name"`   // domain name: surface-pool, filter-graph, ...
	Hits      uint64 `json:"hits"`
	Misses    uint64 `json:"misses"`
	Evictions uint64 `json:"evictions"`
	Purged    uint64 `json:"purged"`
	Entries   uint64 `json:"entries"`
	Idle      uint64 `json:"idle"`
	Bytes     uint64 `json:"bytes"`
	PeakBytes uint64 `json:"peak_bytes"`
	Budget    uint64 `json:"budget"`
}

// HitRate reports reuse as a fraction of acquisitions. Zero hits with zero
// misses means the domain was never used — not that it worked perfectly.
func (s PoolDomainStats) HitRate() float64 {
	total := s.Hits + s.Misses
	if total == 0 {
		return 0
	}
	return float64(s.Hits) / float64(total)
}

// GPUPoolStatsFunc snapshots the pool domains of one live GPU device holder.
// The GPU layer registers one per device holder (render never imports the
// internal GPU package, so the bridge is a callback, like the OOM purge chain).
type GPUPoolStatsFunc func() []PoolDomainStats

var poolStatsRegistry = struct {
	sync.Mutex
	entries []namedPoolStats
}{}

type namedPoolStats struct {
	owner any
	fn    GPUPoolStatsFunc
}

// RegisterGPUPoolStatsCollector adds a device holder's pool snapshot source,
// keyed by owner identity so it can be removed again. A nil func is ignored.
func RegisterGPUPoolStatsCollector(owner any, fn GPUPoolStatsFunc) {
	if owner == nil || fn == nil {
		return
	}
	poolStatsRegistry.Lock()
	defer poolStatsRegistry.Unlock()
	poolStatsRegistry.entries = append(poolStatsRegistry.entries, namedPoolStats{owner: owner, fn: fn})
}

// UnregisterGPUPoolStatsCollector drops a device holder's collector at
// teardown, so a closed window never reports pools whose natives are gone.
func UnregisterGPUPoolStatsCollector(owner any) {
	if owner == nil {
		return
	}
	poolStatsRegistry.Lock()
	defer poolStatsRegistry.Unlock()
	kept := poolStatsRegistry.entries[:0]
	for _, e := range poolStatsRegistry.entries {
		if e.owner != owner {
			kept = append(kept, e)
		}
	}
	poolStatsRegistry.entries = kept
}

// GPUPoolStats collects every registered device holder's pool domain
// snapshots. Empty when no GPU device is live (CPU rendering).
func GPUPoolStats() []PoolDomainStats {
	poolStatsRegistry.Lock()
	entries := make([]namedPoolStats, len(poolStatsRegistry.entries))
	copy(entries, poolStatsRegistry.entries)
	poolStatsRegistry.Unlock()
	var out []PoolDomainStats
	for _, e := range entries {
		if e.fn == nil {
			continue
		}
		out = append(out, e.fn()...)
	}
	return out
}

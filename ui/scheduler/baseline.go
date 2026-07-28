package scheduler

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
)

// BaselineTolerance holds absolute deltas allowed when comparing a run to a
// saved baseline (M-BASELINE-DELTA). Zero fields mean "ignore that metric".
type BaselineTolerance struct {
	// HitchRatePerMin max allowed increase (current - baseline).
	HitchRatePerMin float64
	// P95IntervalMs max allowed increase in frame_interval_p95_ms.
	P95IntervalMs float64
	// RSSSlopeKBPerMin max allowed increase in rss_slope_kb_per_min.
	RSSSlopeKBPerMin float64
	// CPUFallbackOps max allowed increase in cumulative cpu_fallback_ops.
	CPUFallbackOps int64
	// HitchCount max allowed increase in hitch_count.
	HitchCount int64
}

// DefaultBaselineTolerance is a loose soak-friendly band (not a product SLO).
// Tighten per scenario JSON once machines stabilize.
func DefaultBaselineTolerance() BaselineTolerance {
	return BaselineTolerance{
		HitchRatePerMin:  5,    // +5 hitches/min
		P95IntervalMs:    8,    // +8ms p95
		RSSSlopeKBPerMin: 2000, // +2 MB/min (GPU init noise on short runs)
		CPUFallbackOps:   50,
		HitchCount:       10,
	}
}

// BaselineDelta is the result of CompareToBaseline (I 回归).
type BaselineDelta struct {
	// OK is true when every checked field is within tolerance.
	OK bool `json:"ok"`
	// Failures lists human-readable reasons when OK is false.
	Failures []string `json:"failures,omitempty"`

	HitchRateDelta     float64 `json:"hitch_rate_delta,omitempty"`
	P95IntervalDeltaMs float64 `json:"p95_interval_delta_ms,omitempty"`
	RSSSlopeDelta      float64 `json:"rss_slope_delta,omitempty"`
	CPUFallbackDelta   int64   `json:"cpu_fallback_delta,omitempty"`
	HitchCountDelta    int64   `json:"hitch_count_delta,omitempty"`

	// Current / baseline snapshots of key fields (for JSON logs).
	Current  FrameMetrics `json:"current,omitempty"`
	Baseline FrameMetrics `json:"baseline,omitempty"`
}

// CompareToBaseline diffs cur against base using tol. Only fields with
// non-zero tolerance are enforced; deltas are always filled for logging.
//
// Direction: "worse" means higher hitch/p95/slope/fallback/hitch_count.
// Improvements (negative deltas) always pass.
func CompareToBaseline(cur, base FrameMetrics, tol BaselineTolerance) BaselineDelta {
	d := BaselineDelta{
		OK:       true,
		Current:  cur,
		Baseline: base,
	}
	d.HitchRateDelta = cur.HitchRatePerMin - base.HitchRatePerMin
	d.P95IntervalDeltaMs = cur.P95FrameIntervalMs - base.P95FrameIntervalMs
	d.RSSSlopeDelta = cur.RSSSlopeKBPerMin - base.RSSSlopeKBPerMin
	d.CPUFallbackDelta = cur.CPUFallbackOps - base.CPUFallbackOps
	d.HitchCountDelta = cur.HitchCount - base.HitchCount

	check := func(name string, delta, limit float64) {
		if limit <= 0 {
			return
		}
		if delta > limit {
			d.OK = false
			d.Failures = append(d.Failures,
				fmt.Sprintf("%s delta=%.3f exceeds +%.3f", name, delta, limit))
		}
	}
	checkInt := func(name string, delta, limit int64) {
		if limit <= 0 {
			return
		}
		if delta > limit {
			d.OK = false
			d.Failures = append(d.Failures,
				fmt.Sprintf("%s delta=%d exceeds +%d", name, delta, limit))
		}
	}

	check("hitch_rate_per_min", d.HitchRateDelta, tol.HitchRatePerMin)
	check("frame_interval_p95_ms", d.P95IntervalDeltaMs, tol.P95IntervalMs)
	check("rss_slope_kb_per_min", d.RSSSlopeDelta, tol.RSSSlopeKBPerMin)
	checkInt("cpu_fallback_ops", d.CPUFallbackDelta, tol.CPUFallbackOps)
	checkInt("hitch_count", d.HitchCountDelta, tol.HitchCount)
	return d
}

// SaveBaseline writes m as pretty JSON to path (for hand-curated / CI artifacts).
func SaveBaseline(path string, m FrameMetrics) error {
	if path == "" {
		return fmt.Errorf("scheduler: empty baseline path")
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

// LoadBaseline reads a FrameMetrics JSON file previously written by SaveBaseline
// or MetricsStore.JSON (same schema).
func LoadBaseline(path string) (FrameMetrics, error) {
	var m FrameMetrics
	if path == "" {
		return m, fmt.Errorf("scheduler: empty baseline path")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, err
	}
	return m, nil
}

// NearlyEqual is a small helper for tests (absolute epsilon).
func NearlyEqual(a, b, eps float64) bool {
	return math.Abs(a-b) <= eps
}

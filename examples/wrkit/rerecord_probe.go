package wrkit

// Key-level re-record attribution probe for dirty-region windows (R4b/R8/R21).
//
// scene.PictureTextureCache.RerecordByKeySnapshot exposes cumulative per-key
// re-record counts; the key is the ui/rendering Base.CacheID of the boundary
// node owning the picture (non-boundary carriers have CacheKey 0 and never
// appear). A window-side probe diffs consecutive snapshots on the UI ticker
// and attributes every changed key against an explicit allowlist:
//   - hot keys MUST gain re-records on (nearly) every steady tick — they are
//     the only legitimate continuous re-records in the scene;
//   - forbidden keys (static dense cells, shell chrome) must NEVER appear;
//   - HUD is a bounded background re-record (≤10 Hz against a 60 Hz ticker).
//
// The raster thread updates the attribution map serially; the UI-side read is
// one mutex-consistent copy per tick (same posture as ui_wr_r10_async_image).

import (
	"fmt"
	"os"

	"github.com/energye/gpui/ui/embedder"
)

// RerecordKeyProbe accumulates key-level re-record deltas between snapshots.
type RerecordKeyProbe struct {
	app *embedder.PipelineApp
	// prev is the last snapshot; nil until Start.
	prev map[uint64]map[string]int64
	// Keys maps key → cumulative re-records gained across the probe window.
	Keys map[uint64]int64
	// MaxChangedPerTick is the worst observed changed-key count in one diff.
	MaxChangedPerTick int64
	// Ticks is the number of accounted diff windows.
	Ticks int64
	dbg   bool
}

// NewRerecordKeyProbe builds a probe. WR_RERECORD_DBG=1 logs every early tick
// and every 60th tick afterwards (probe calibration, same pattern as R10).
func NewRerecordKeyProbe(app *embedder.PipelineApp) *RerecordKeyProbe {
	return &RerecordKeyProbe{app: app, dbg: os.Getenv("WR_RERECORD_DBG") == "1"}
}

// Start takes the baseline snapshot. Records already on the raster thread when
// Start runs land in the baseline, so callers should Start ≥1 tick after the
// last legitimate non-hot re-record (freeze mark, overlay insert) settles.
func (p *RerecordKeyProbe) Start() {
	if p == nil || p.app == nil {
		return
	}
	p.prev = p.app.PictureTextures().RerecordByKeySnapshot()
}

// Tick diffs the current snapshot against the previous one, accumulates per-key
// gains, and returns the keys that gained re-records in this window.
func (p *RerecordKeyProbe) Tick() []uint64 {
	if p == nil || p.prev == nil {
		return nil
	}
	cur := p.app.PictureTextures().RerecordByKeySnapshot()
	changed := make([]uint64, 0, len(cur))
	for k, causes := range cur {
		prevCauses := p.prev[k]
		gained := int64(0)
		for cause, n := range causes {
			if d := n - prevCauses[cause]; d > 0 {
				gained += d
			}
		}
		if gained > 0 {
			changed = append(changed, k)
			if p.Keys == nil {
				p.Keys = make(map[uint64]int64)
			}
			p.Keys[k] += gained
		}
	}
	p.prev = cur
	p.Ticks++
	if int64(len(changed)) > p.MaxChangedPerTick {
		p.MaxChangedPerTick = int64(len(changed))
	}
	if p.dbg && (p.Ticks <= 90 || p.Ticks%60 == 0) {
		fmt.Fprintf(os.Stderr, "rerecord-probe: tick=%d changed=%d keys=%v\n", p.Ticks, len(changed), changed)
	}
	return changed
}

// Total returns the cumulative gained re-record count for one key.
func (p *RerecordKeyProbe) Total(key uint64) int64 {
	if p == nil {
		return 0
	}
	return p.Keys[key]
}

// Violations returns per-key totals for every key outside the allowlist.
// A non-empty result names exactly which layers leaked into the re-record
// attribution (shell chrome, dense static cells, …).
func (p *RerecordKeyProbe) Violations(allow map[uint64]bool) map[uint64]int64 {
	if p == nil {
		return nil
	}
	out := make(map[uint64]int64)
	for k, n := range p.Keys {
		if !allow[k] {
			out[k] = n
		}
	}
	return out
}

// Engaged reports whether the probe actually observed any re-record. A disengaged
// probe (texture cache inert for this window's policy) makes key-level gates
// vacuous — callers must downgrade those gates instead of passing them silently.
func (p *RerecordKeyProbe) Engaged() bool {
	return p != nil && p.Ticks > 0 && len(p.Keys) > 0
}

// BaselineSize is the number of keys already present when Start ran. A window
// can be engaged through the baseline alone (cold records happened before the
// probe window) even when the probe window itself saw zero re-records.
func (p *RerecordKeyProbe) BaselineSize() int {
	if p == nil {
		return 0
	}
	return len(p.prev)
}

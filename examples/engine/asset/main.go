// Command game_asset is the 12.3 hot-reload independent window.
//
// One watched slot plus its dependency chain, all through the real
// engine/asset ledger (Manager plus Watcher, read-only from here).
// The window owns its watch file under its own testdata and never
// watches repo source files: edit that one file and only that id swaps,
// bad bytes keep the last good picture, and the chain report names the
// broken link.
//
// Modes:
//
//	go run ./examples/engine/asset --case=reload -auto-only
//	  headless probes + ~8s window (JSON on stdout, exit 1 on fail).
//	go run ./examples/engine/asset --case=reload -manual-seconds 30
//	  manual for 30s (events logged, title shows the count), then summary.
//	go run ./examples/engine/asset --case=reload
//	  probes, then resident until close (RUN_SECONDS sets a timed run).
//
// Window: 1200x800, title game_asset. First run writes the golden baseline
// into testdata/; later runs compare it with zero tolerance.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/energye/gpui/engine/asset"
	"github.com/energye/gpui/engine/core"
	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH = 1200, 800
	abilityID  = "asset-reload"
	scenario   = "game_asset--case=reload"
	goldenPath = "examples/engine/asset/testdata/asset_reload_golden.png"

	// frozenPath is the engine-side frozen hotreload table. The window
	// never hardcodes hashes or sizes; it replays this file through the
	// real engine/asset API.
	frozenPath = "engine/asset/testdata/hotreload_cases.json"
	// engineData is the engine testdata dir the probes stage copies from.
	// Staging copies live in a process temp dir; testdata stays frozen.
	engineData = "engine/asset/testdata"
	// liveRel is the one file the live window watches. It lives in the
	// window's own testdata and is owned by this window; repo source
	// files are never watched.
	liveRel = "examples/engine/asset/testdata/live_slot.ktx2"
	liveID  = "tex/live"

	// probePixelTol is the hardcoded per-channel tolerance (0-255 steps)
	// for the offscreen pixel asserts. The golden mask compare stays at
	// zero tolerance.
	probePixelTol = 4

	// S59 targets (informational, from the frozen 136B/112B swap pair):
	// a 136B swap lands in ~40us, a steady poll in ~14us. The auto gate
	// below uses generous caps so slow hosts do not false-red; the
	// measured numbers ride the JSON for the record.
	pollTargetUs   = 14.0
	reloadTargetUs = 40.0
	maxPollUs      = 100.0
	maxReloadUs    = 5000.0

	swapReps  = 100
	steadyN   = 2000
	neighbour = "map/level1"
)

// Shared scene colors (window paint and offscreen probes use the same).
const (
	bgR, bgG, bgB = 0.08, 0.09, 0.11

	redR, redG, redB = 0.90, 0.15, 0.12
	chkR, chkG, chkB = 0.20, 0.60, 0.60

	barR, barG, barB = 0.35, 0.55, 0.75
	barBgR, barBgG   = 0.25, 0.27
	barBgB           = 0.32
)

// Body-local layout (body is ~904x656 under the shell chrome).
const (
	liveX, liveY   = 16.0, 44.0
	depX, depY     = 312.0, 44.0
	countX, countY = 608.0, 44.0
	swW, swH       = 280.0, 180.0
	noteY          = 480.0

	offW, offH       = 480, 270
	offCardW, offCH  = 140, 200
	offCardY         = 30
	offRedX, offChkX = 10, 170
	offBarY, offBarH = 244, 10
	offBarW, offFill = 460, 300
)

// probeResult is the three-evidence headless verdict (no GPU needed).
type probeResult struct {
	LogicOK, PixOK, GoldenOK bool
	SwapOK                   bool
	IdenticalOK              bool
	BadKeptOK                bool
	DepsOK                   bool
	StableOK                 bool
	Reloads                  int
	Events                   int
	PollUs                   float64
	ReloadUs                 float64
	FromHash, ToHash         uint64
	FromSize, ToSize         int
	DepChain                 string
	Dependents               string
	Detail                   string
	PixDetail                string
	GoldenChanged            int
	GoldenWrote              bool
	OK                       bool
}

type frozenWatch struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`
	File    string   `json:"file"`
	Version string   `json:"version"`
	Hash    uint64   `json:"hash"`
	Size    int      `json:"size"`
	Upload  int      `json:"upload"`
	Pixels  int      `json:"pixels"`
	Deps    []string `json:"deps"`
}

type frozenSwap struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	From     string `json:"from"`
	To       string `json:"to"`
	Version  string `json:"version"`
	FromHash uint64 `json:"from_hash"`
	ToHash   uint64 `json:"to_hash"`
}

type frozenBad struct {
	File     string `json:"file"`
	Kind     string `json:"kind"`
	WantCode string `json:"want_code"`
}

type frozenFile struct {
	Version        string        `json:"version"`
	MaxWatches     int           `json:"max_watches"`
	MaxWatchIDLen  int           `json:"max_watch_id_len"`
	MaxWatchPath   int           `json:"max_watch_path_len"`
	PollIntervalMs int64         `json:"poll_interval_ms"`
	Tolerance      int           `json:"tolerance"`
	Watches        []frozenWatch `json:"watches"`
	Swap           frozenSwap    `json:"swap"`
	BadFile        frozenBad     `json:"bad_file"`
	Unsupported    []string      `json:"unsupported_kinds"`
}

func loadFrozen() (frozenFile, error) {
	var f frozenFile
	raw, err := os.ReadFile(frozenPath)
	if err != nil {
		return f, err
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		return f, err
	}
	return f, nil
}

func mustKind(name string) (asset.Kind, error) { return asset.ParseKind(name) }

func mustVer(s string) (core.Version, error) { return core.ParseVersion(s) }

func idsOf(deps []string) []core.AssetID {
	if len(deps) == 0 {
		return nil
	}
	out := make([]core.AssetID, len(deps))
	for i, d := range deps {
		out[i] = core.AssetID(d)
	}
	return out
}

func wantCode(s string) core.Code {
	switch s {
	case "not-found":
		return core.CodeNotFound
	case "bad-data":
		return core.CodeBadData
	case "out-of-memory":
		return core.CodeOutOfMemory
	case "unsupported":
		return core.CodeUnsupported
	case "invalid-arg":
		return core.CodeInvalidArg
	case "version-mismatch":
		return core.CodeVersionMismatch
	default:
		return core.CodeUnknown
	}
}

// stageDir copies the frozen files into a process temp dir. Probes edit
// the copies; testdata stays frozen on disk.
func stageDir(f frozenFile) (string, error) {
	dir, err := os.MkdirTemp("", "game_asset_probe")
	if err != nil {
		return "", err
	}
	seen := map[string]bool{}
	add := func(name string) error {
		if seen[name] || name == "" {
			return nil
		}
		seen[name] = true
		raw, err := os.ReadFile(filepath.Join(engineData, name))
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, name), raw, 0o644)
	}
	for _, w := range f.Watches {
		if err := add(w.File); err != nil {
			return "", err
		}
	}
	if err := add(f.Swap.From); err != nil {
		return "", err
	}
	if err := add(f.Swap.To); err != nil {
		return "", err
	}
	return dir, add(f.BadFile.File)
}

// rewriteStaged copies engine bytes over a staged path and bumps the
// modtime past the filesystem tick so stat-based Poll sees the change.
func rewriteStaged(dir, watched, src string) error {
	raw, err := os.ReadFile(filepath.Join(engineData, src))
	if err != nil {
		return err
	}
	dst := filepath.Join(dir, watched)
	if err := os.WriteFile(dst, raw, 0o644); err != nil {
		return err
	}
	at := time.Now().Add(2 * time.Second)
	return os.Chtimes(dst, at, at)
}

func watchAll(dir string, f frozenFile, w *asset.Watcher) error {
	for _, want := range f.Watches {
		k, err := mustKind(want.Kind)
		if err != nil {
			return fmt.Errorf("%s kind: %w", want.ID, err)
		}
		v, err := mustVer(want.Version)
		if err != nil {
			return fmt.Errorf("%s version: %w", want.ID, err)
		}
		if _, err := w.Watch(core.AssetID(want.ID), filepath.Join(dir, want.File), k, v, idsOf(want.Deps)); err != nil {
			return fmt.Errorf("%s watch: %w", want.ID, err)
		}
	}
	return nil
}

func containsID(list []core.AssetID, want core.AssetID) bool {
	for _, got := range list {
		if got == want {
			return true
		}
	}
	return false
}

// probeLogic drives the real Manager plus Watcher against the frozen
// file: one edit swaps one id, watcher matches direct load bit for bit,
// bad files keep the old picture, the chain names its links, and 100
// round trips hold bytes steady.
func probeLogic() probeResult {
	var p probeResult
	f, err := loadFrozen()
	if err != nil {
		p.Detail = "frozen read: " + err.Error()
		return p
	}
	if len(f.Watches) == 0 || f.Swap.From == "" || f.BadFile.File == "" || len(f.Unsupported) == 0 {
		p.Detail = "frozen file has no watches/swap/bad/unsupported"
		return p
	}
	if f.Tolerance != 0 {
		p.Detail = fmt.Sprintf("tolerance = %d, want frozen 0", f.Tolerance)
		return p
	}
	if asset.MaxWatches != f.MaxWatches || asset.MaxWatchIDLen != f.MaxWatchIDLen ||
		asset.MaxWatchPathLen != f.MaxWatchPath || int64(asset.PollInterval.Milliseconds()) != f.PollIntervalMs {
		p.Detail = "hotreload budgets diverge from frozen file"
		return p
	}
	if asset.CurrentVersion.String() != f.Version {
		p.Detail = fmt.Sprintf("version = %v, want %v", asset.CurrentVersion, f.Version)
		return p
	}
	dir, err := stageDir(f)
	if err != nil {
		p.Detail = "stage: " + err.Error()
		return p
	}
	defer func() { _ = os.RemoveAll(dir) }()

	// A: one edit swaps one id; neighbours stay put.
	m := asset.NewManager()
	w := asset.NewWatcher(m)
	if err := watchAll(dir, f, w); err != nil {
		p.Detail = err.Error()
		return p
	}
	if dirty, err := w.Poll(); err != nil || len(dirty) != 0 {
		p.Detail = fmt.Sprintf("steady poll = %v/%v, want empty", dirty, err)
		return p
	}
	target := ""
	for _, want := range f.Watches {
		if want.File == f.Swap.From {
			target = want.ID
			break
		}
	}
	if target == "" {
		p.Detail = fmt.Sprintf("swap from %q matches no watch", f.Swap.From)
		return p
	}
	beforeOther, err := m.Wait(core.AssetID(neighbour))
	if err != nil {
		p.Detail = "neighbour wait: " + err.Error()
		return p
	}
	if err := rewriteStaged(dir, f.Swap.From, f.Swap.To); err != nil {
		p.Detail = "rewrite: " + err.Error()
		return p
	}
	dirty, err := w.Poll()
	if err != nil || len(dirty) != 1 || dirty[0] != core.AssetID(target) {
		p.Detail = fmt.Sprintf("dirty = %v/%v, want [%s]", dirty, err, target)
		return p
	}
	if w.StateOf(core.AssetID(target)) != asset.HotDirty {
		p.Detail = "state is not dirty after edit"
		return p
	}
	got, err := w.Reload(core.AssetID(target))
	if err != nil || got.Hash() != f.Swap.ToHash {
		p.Detail = fmt.Sprintf("reload hash/err = %v/%v, want %d", got.Hash(), err, f.Swap.ToHash)
		return p
	}
	afterOther, err := m.Wait(core.AssetID(neighbour))
	if err != nil || !beforeOther.Equal(afterOther) {
		p.Detail = "neighbour moved during the swap"
		return p
	}
	if st := w.Stats(); st.Reloads != 1 || st.Events != 1 {
		p.Detail = fmt.Sprintf("stats = %+v, want 1 reload 1 event", st)
		return p
	}
	if out, err := w.ReloadDirty(); err != nil || len(out) != 0 {
		p.Detail = fmt.Sprintf("quiet reload-dirty = %d/%v, want empty", len(out), err)
		return p
	}
	if err := rewriteStaged(dir, f.Swap.From, f.Swap.From); err != nil {
		p.Detail = "rewrite back: " + err.Error()
		return p
	}
	if _, err := w.Poll(); err != nil {
		p.Detail = "second poll: " + err.Error()
		return p
	}
	out, err := w.ReloadDirty()
	if err != nil || len(out) != 1 || out[0].Hash() != f.Swap.FromHash {
		p.Detail = "reload-dirty did not swap back"
		return p
	}
	p.SwapOK = true
	p.FromHash, p.ToHash = f.Swap.FromHash, f.Swap.ToHash

	// B: watcher matches a direct load bit for bit; copies stay isolated.
	for _, want := range f.Watches {
		other := asset.NewManager()
		k, _ := mustKind(want.Kind)
		v, _ := mustVer(want.Version)
		direct, err := other.LoadFile(core.AssetID(want.ID), filepath.Join(engineData, want.File), k, v, idsOf(want.Deps))
		if err != nil {
			p.Detail = fmt.Sprintf("%s direct: %v", want.ID, err)
			return p
		}
		viaWatch, err := m.Wait(core.AssetID(want.ID))
		if err != nil || !viaWatch.Equal(direct) {
			p.Detail = fmt.Sprintf("%s watch vs direct diverged: %v", want.ID, err)
			return p
		}
		probe := viaWatch.Bytes()
		if len(probe) == 0 {
			p.Detail = fmt.Sprintf("%s bytes empty", want.ID)
			return p
		}
		probe[0] ^= 0xFF
		if again, _ := m.Wait(core.AssetID(want.ID)); !again.Equal(direct) {
			p.Detail = fmt.Sprintf("%s copy probe corrupted the watch", want.ID)
			return p
		}
		if w.PathOf(core.AssetID(want.ID)) == "" || w.CauseOf(core.AssetID(want.ID)) != nil {
			p.Detail = fmt.Sprintf("%s path/cause not clean", want.ID)
			return p
		}
		gotIDs := w.WatchedIDs()
		for i := 1; i < len(gotIDs); i++ {
			if gotIDs[i-1] >= gotIDs[i] {
				p.Detail = "watched ids unsorted"
				return p
			}
		}
	}
	for _, name := range f.Unsupported {
		if _, err := asset.ParseKind(name); core.CodeOf(err) != core.CodeUnsupported {
			p.Detail = fmt.Sprintf("unsupported %q not rejected", name)
			return p
		}
	}
	p.IdenticalOK = true

	// C: bad files never wipe the art; the old picture stays readable.
	var redWatch frozenWatch
	for _, want := range f.Watches {
		if want.ID == "tex/red" {
			redWatch = want
		}
	}
	if redWatch.ID == "" {
		p.Detail = "frozen file has no tex/red watch"
		return p
	}
	goodBefore, err := m.Wait(core.AssetID(redWatch.ID))
	if err != nil {
		p.Detail = "good wait: " + err.Error()
		return p
	}
	victim := filepath.Join(dir, redWatch.File)
	if err := os.Remove(victim); err != nil {
		p.Detail = "remove: " + err.Error()
		return p
	}
	dirty, err = w.Poll()
	if err != nil || len(dirty) != 1 {
		p.Detail = fmt.Sprintf("delete dirty = %v/%v", dirty, err)
		return p
	}
	if _, err := w.Reload(core.AssetID(redWatch.ID)); core.CodeOf(err) != core.CodeNotFound {
		p.Detail = fmt.Sprintf("delete reload code = %v, want not-found", core.CodeOf(err))
		return p
	}
	if w.StateOf(core.AssetID(redWatch.ID)) != asset.HotFailed || w.CauseOf(core.AssetID(redWatch.ID)) == nil {
		p.Detail = "failed watch did not park with a cause"
		return p
	}
	if still, err := m.Wait(core.AssetID(redWatch.ID)); err != nil || !still.Equal(goodBefore) {
		p.Detail = "delete wiped the picture"
		return p
	}
	badRaw, err := os.ReadFile(filepath.Join(engineData, f.BadFile.File))
	if err != nil {
		p.Detail = "read bad: " + err.Error()
		return p
	}
	if err := os.WriteFile(victim, badRaw, 0o644); err != nil {
		p.Detail = "write torn: " + err.Error()
		return p
	}
	at := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(victim, at, at)
	if _, err := w.Poll(); err != nil {
		p.Detail = "torn poll: " + err.Error()
		return p
	}
	if _, err := w.Reload(core.AssetID(redWatch.ID)); core.CodeOf(err) != wantCode(f.BadFile.WantCode) {
		p.Detail = fmt.Sprintf("torn reload code = %v, want %v", core.CodeOf(err), f.BadFile.WantCode)
		return p
	}
	if kept, err := m.Wait(core.AssetID(redWatch.ID)); err != nil || !kept.Equal(goodBefore) {
		p.Detail = "torn wiped the picture"
		return p
	}
	if err := rewriteStaged(dir, redWatch.File, f.Swap.From); err != nil {
		p.Detail = "fix rewrite: " + err.Error()
		return p
	}
	if _, err := w.Poll(); err != nil {
		p.Detail = "fix poll: " + err.Error()
		return p
	}
	fixed, err := w.Reload(core.AssetID(redWatch.ID))
	if err != nil || !fixed.Equal(goodBefore) {
		p.Detail = fmt.Sprintf("good after bad diverged: %v", err)
		return p
	}
	p.BadKeptOK = true

	// D: the chain report names its links both ways.
	deps, ok := m.Dependencies(core.AssetID(neighbour))
	if !ok || !containsID(deps, "tex/red") || !containsID(deps, "tex/checker") {
		p.Detail = fmt.Sprintf("deps of %s = %v, want tex/red+tex/checker", neighbour, deps)
		return p
	}
	dependents := m.Dependents("tex/red")
	if !containsID(dependents, "map/level1") || !containsID(dependents, "bone/slime") {
		p.Detail = fmt.Sprintf("dependents of tex/red = %v, want map/level1+bone/slime", dependents)
		return p
	}
	if _, ok := m.Dependencies("ghost/missing"); ok {
		p.Detail = "missing chain reported found"
		return p
	}
	names := make([]string, len(deps))
	for i, d := range deps {
		names[i] = string(d)
	}
	p.DepChain = neighbour + "->" + strings.Join(names, ",")
	dnames := make([]string, len(dependents))
	for i, d := range dependents {
		dnames[i] = string(d)
	}
	p.Dependents = strings.Join(dnames, ",")
	p.DepsOK = true

	// E: 100 round trips hold bytes steady on a fresh ledger.
	m2 := asset.NewManager()
	w2 := asset.NewWatcher(m2)
	if err := watchAll(dir, f, w2); err != nil {
		p.Detail = "stable " + err.Error()
		return p
	}
	// Re-stage the swap endpoints: earlier sections rewrote them.
	if err := rewriteStaged(dir, f.Swap.From, f.Swap.From); err != nil {
		p.Detail = "stable reset: " + err.Error()
		return p
	}
	if _, err := w2.Poll(); err != nil {
		p.Detail = "stable reset poll: " + err.Error()
		return p
	}
	if _, err := w2.ReloadDirty(); err != nil {
		p.Detail = "stable reset reload: " + err.Error()
		return p
	}
	baseBytes := m2.TotalBytes()
	baseLive := m2.LiveCount("tex/red")
	baseSt := w2.Stats()
	first, err := m2.Wait("tex/red")
	if err != nil {
		p.Detail = "stable first: " + err.Error()
		return p
	}
	fromRaw, err := os.ReadFile(filepath.Join(engineData, f.Swap.From))
	if err != nil {
		p.Detail = "read from: " + err.Error()
		return p
	}
	toRaw, err := os.ReadFile(filepath.Join(engineData, f.Swap.To))
	if err != nil {
		p.Detail = "read to: " + err.Error()
		return p
	}
	p.FromSize, p.ToSize = len(fromRaw), len(toRaw)
	fromTotal := baseBytes
	toTotal := baseBytes - int64(len(fromRaw)) + int64(len(toRaw))
	for i := 0; i < swapReps; i++ {
		if i%2 == 0 {
			if err := rewriteStaged(dir, f.Swap.From, f.Swap.To); err != nil {
				p.Detail = fmt.Sprintf("rep %d rewrite: %v", i, err)
				return p
			}
		} else if err := rewriteStaged(dir, f.Swap.From, f.Swap.From); err != nil {
			p.Detail = fmt.Sprintf("rep %d rewrite: %v", i, err)
			return p
		}
		if _, err := w2.Poll(); err != nil {
			p.Detail = fmt.Sprintf("rep %d poll: %v", i, err)
			return p
		}
		got, err := w2.Reload("tex/red")
		if err != nil {
			p.Detail = fmt.Sprintf("rep %d reload: %v", i, err)
			return p
		}
		wantHash, wantTotal := f.Swap.ToHash, toTotal
		if i%2 == 1 {
			wantHash, wantTotal = f.Swap.FromHash, fromTotal
		}
		if got.Hash() != wantHash || m2.TotalBytes() != wantTotal || m2.LiveCount("tex/red") != baseLive {
			p.Detail = fmt.Sprintf("rep %d drifted", i)
			return p
		}
	}
	if back, err := m2.Wait("tex/red"); err != nil || !back.Equal(first) {
		p.Detail = "after 100 swaps drifted"
		return p
	}
	if st := w2.Stats(); st.Reloads-baseSt.Reloads != swapReps || st.Events-baseSt.Events != swapReps {
		p.Detail = fmt.Sprintf("stats = %+v, want %d reloads %d events since reset", st, swapReps, swapReps)
		return p
	}
	p.StableOK = true
	p.Reloads, p.Events = swapReps, swapReps

	// F: steady polls stay microsecond-scale; one 136B swap is timed.
	start := time.Now()
	for i := 0; i < steadyN; i++ {
		if _, err := w2.Poll(); err != nil {
			p.Detail = fmt.Sprintf("perf poll %d: %v", i, err)
			return p
		}
	}
	el := time.Since(start)
	p.PollUs = float64(el.Microseconds()) / steadyN
	if err := rewriteStaged(dir, f.Swap.From, f.Swap.To); err != nil {
		p.Detail = "perf rewrite: " + err.Error()
		return p
	}
	if _, err := w2.Poll(); err != nil {
		p.Detail = "perf poll: " + err.Error()
		return p
	}
	rstart := time.Now()
	perfGot, err := w2.Reload("tex/red")
	if err != nil || perfGot.Hash() != f.Swap.ToHash {
		p.Detail = fmt.Sprintf("perf reload: %v", err)
		return p
	}
	p.ReloadUs = float64(time.Since(rstart).Microseconds())
	p.Detail = fmt.Sprintf("swap=1->1 stable=%d identical=%d bad_kept=1 chain=%s poll=%.1fus reload=%.1fus",
		swapReps, len(f.Watches), p.DepChain, p.PollUs, p.ReloadUs)
	p.LogicOK = true
	return p
}

// paintAssetFrame draws the deterministic probe frame offscreen: the red
// slot, the checker slot, and the dependency chain strip.
func paintAssetFrame(dc *render.Context) {
	dc.ClearWithColor(render.RGBA{R: bgR, G: bgG, B: bgB, A: 1})
	dc.SetRGBA(redR, redG, redB, 1)
	dc.DrawRectangle(float64(offRedX), float64(offCardY), float64(offCardW), float64(offCH))
	_ = dc.Fill()
	dc.SetRGBA(chkR, chkG, chkB, 1)
	dc.DrawRectangle(float64(offChkX), float64(offCardY), float64(offCardW), float64(offCH))
	_ = dc.Fill()
	dc.SetRGBA(barBgR, barBgG, barBgB, 1)
	dc.DrawRectangle(10, float64(offBarY), float64(offBarW), float64(offBarH))
	_ = dc.Fill()
	dc.SetRGBA(barR, barG, barB, 1)
	dc.DrawRectangle(10, float64(offBarY), float64(offFill), float64(offBarH))
	_ = dc.Fill()
}

func sample8(img image.Image, x, y int) (uint8, uint8, uint8) {
	r, g, b, _ := img.At(x, y).RGBA()
	return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)
}

func closeEnough(got, want uint8) bool {
	d := int(got) - int(want)
	if d < 0 {
		d = -d
	}
	return d <= probePixelTol
}

func want8(v float64) uint8 { return uint8(v*255 + 0.5) }

// probePixels asserts the slot and chain colors offscreen.
func probePixels() (bool, string) {
	prev, _ := os.LookupEnv("GOGPU_RENDER_MODE")
	_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	defer func() {
		if prev == "" {
			_ = os.Unsetenv("GOGPU_RENDER_MODE")
		} else {
			_ = os.Setenv("GOGPU_RENDER_MODE", prev)
		}
	}()
	dc := render.NewContext(offW, offH)
	paintAssetFrame(dc)
	img := dc.Image()
	_ = dc.Close()

	rr, rg, rb := sample8(img, offRedX+offCardW/2, offCardY+offCH/2)
	cr, cg, cb := sample8(img, offChkX+offCardW/2, offCardY+offCH/2)
	br, bg, bb := sample8(img, 400, 130)
	qr, qg, qb := sample8(img, 100, offBarY+offBarH/2)
	ok := closeEnough(rr, want8(redR)) && closeEnough(rg, want8(redG)) && closeEnough(rb, want8(redB)) &&
		closeEnough(cr, want8(chkR)) && closeEnough(cg, want8(chkG)) && closeEnough(cb, want8(chkB)) &&
		closeEnough(br, want8(bgR)) && closeEnough(bg, want8(bgG)) && closeEnough(bb, want8(bgB)) &&
		closeEnough(qr, want8(barR)) && closeEnough(qg, want8(barG)) && closeEnough(qb, want8(barB))
	detail := fmt.Sprintf("red=(%d,%d,%d) chk=(%d,%d,%d) bg=(%d,%d,%d) chain=(%d,%d,%d) tol=%d",
		rr, rg, rb, cr, cg, cb, br, bg, bb, qr, qg, qb, probePixelTol)
	return ok, detail
}

// probeGolden compares the deterministic frame against the frozen mask
// with zero tolerance; the first run produces the baseline.
func probeGolden() (ok bool, changed int, wrote bool) {
	prev, _ := os.LookupEnv("GOGPU_RENDER_MODE")
	_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	defer func() {
		if prev == "" {
			_ = os.Unsetenv("GOGPU_RENDER_MODE")
		} else {
			_ = os.Setenv("GOGPU_RENDER_MODE", prev)
		}
	}()
	dc := render.NewContext(offW, offH)
	paintAssetFrame(dc)
	img := dc.Image()
	_ = dc.Close()

	f, err := os.Open(goldenPath)
	if err != nil {
		if err := os.MkdirAll("examples/engine/asset/testdata", 0o755); err != nil {
			return false, 0, false
		}
		out, err := os.Create(goldenPath)
		if err != nil {
			return false, 0, false
		}
		encErr := png.Encode(out, img)
		_ = out.Close()
		return encErr == nil, 0, encErr == nil
	}
	defer func() { _ = f.Close() }()
	want, err := png.Decode(f)
	if err != nil {
		return false, 0, false
	}
	if !img.Bounds().Eq(want.Bounds()) {
		return false, 1, false
	}
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			ar, ag, ab, aa := img.At(x, y).RGBA()
			br, bg, bb, ba := want.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				changed++
			}
		}
	}
	return changed == 0, changed, false
}

// runProbes collects the three evidences: logic, pixels, golden mask.
func runProbes() probeResult {
	p := probeLogic()
	p.PixOK, p.PixDetail = probePixels()
	var wrote bool
	p.GoldenOK, p.GoldenChanged, wrote = probeGolden()
	p.GoldenWrote = wrote
	p.OK = p.LogicOK && p.SwapOK && p.IdenticalOK && p.BadKeptOK && p.DepsOK && p.StableOK &&
		p.PixOK && p.GoldenOK && p.PollUs <= maxPollUs && p.ReloadUs <= maxReloadUs
	return p
}

// assetSim is the live window state: one real Watcher polls the owned
// slot file every frame and swaps only that id when it changes.
type assetSim struct {
	mgr     *asset.Manager
	watcher *asset.Watcher
	app     *embedder.PipelineApp
	shell   *wrkit.ShellChrome
	phase   *wrkit.PhaseClock
	redBox  *rendering.RenderColorBox
	chkBox  *rendering.RenderColorBox
	hashL   *rendering.RenderText
	sizeL   *rendering.RenderText
	stateL  *rendering.RenderText
	chainL  *rendering.RenderText
	reloadL *rendering.RenderText
	eventL  *rendering.RenderText
	perfL   *rendering.RenderText
	fpsL    *rendering.RenderText
	frames  int
}

type ticker struct{ s *assetSim }

func (t *ticker) Tick(dt float64) bool {
	s := t.s
	if s == nil {
		return true
	}
	s.frames++
	if dt < 0 {
		dt = 0
	}
	if dt > 0.05 {
		dt = 0.05
	}
	// Real logic only: poll the owned slot, swap when dirty.
	reloaded, _ := s.watcher.ReloadDirty()
	_ = reloaded
	cur, _ := s.mgr.Wait(core.AssetID(liveID))
	var hash uint64
	var size int
	var state asset.State
	if cur != nil {
		hash, size, state = cur.Hash(), cur.Size(), cur.State()
	}
	st := s.watcher.Stats()
	// Toggle the visible slot: red bytes show red, checker bytes show
	// checker, so a manual file edit visibly swaps the swatch.
	if size == 136 {
		s.redBox.SetAlpha(0)
		s.chkBox.SetAlpha(1)
	} else {
		s.redBox.SetAlpha(1)
		s.chkBox.SetAlpha(0)
	}
	s.hashL.SetText(fmt.Sprintf("哈希 %d", hash))
	s.sizeL.SetText(fmt.Sprintf("字节 %d", size))
	s.stateL.SetText(fmt.Sprintf("看门 %s 总字节 %d", s.watcher.StateOf(core.AssetID(liveID)), s.mgr.TotalBytes()))
	s.reloadL.SetText(fmt.Sprintf("重载数 %d", st.Reloads))
	s.eventL.SetText(fmt.Sprintf("事件数 %d", st.Events))
	s.perfL.SetText(fmt.Sprintf("轮询目标 %.0fus 重载目标 %.0fus", pollTargetUs, reloadTargetUs))
	_ = state

	phase := s.phase.Advance(dt)
	snap := s.app.Metrics().Snapshot()
	fps := 0.0
	if snap.AvgFrameIntervalMs > 1e-6 {
		fps = 1000.0 / snap.AvgFrameIntervalMs
	}
	s.fpsL.SetText(fmt.Sprintf("帧率 %.0f", fps))
	gateOK := s.frames > 0
	s.shell.NoteHUDTick(dt)
	s.shell.UpdateHUD("asset-reload", phase, s.app, gateOK,
		fmt.Sprintf("reloads=%d events=%d", st.Reloads, st.Events),
		fmt.Sprintf("hash=%d size=%d", hash, size))
	s.app.ScheduleFrame()
	return true
}

type manualSummary struct {
	Pointer, Key, Resize int
	Timed                bool
	Note                 string
}

func runSeconds(def int) int {
	if v := os.Getenv("RUN_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func failJSON(probe probeResult) {
	b, _ := json.Marshal(map[string]any{
		"ability_id": abilityID,
		"scenario":   scenario,
		"probe_ok":   0,
		"pass":       false,
		"pixels":     probe.PixDetail,
		"golden":     probe.GoldenChanged,
	})
	fmt.Fprintln(os.Stdout, string(b))
}

func main() {
	caseFlag := flag.String("case", "reload", "scenario case (only reload)")
	autoOnly := flag.Bool("auto-only", false, "probes + short window, JSON gate on stdout")
	manualSeconds := flag.Int("manual-seconds", 0, "manual phase seconds (0 = until close)")
	flag.Parse()

	if *caseFlag != "reload" {
		fmt.Fprintf(os.Stderr, "FAIL: --case=%q want reload (only hot-reload scene)\n", *caseFlag)
		os.Exit(1)
	}
	wrkit.EnsureUIFace()

	probe := runProbes()
	fmt.Fprintf(os.Stderr, "game_asset: probes ok=%v logic=%v swap=%v identical=%v bad=%v deps=%v stable=%v pix=%v golden=%v(wrote=%v changed=%d) poll=%.1fus reload=%.1fus %s | %s\n",
		probe.OK, probe.LogicOK, probe.SwapOK, probe.IdenticalOK, probe.BadKeptOK, probe.DepsOK,
		probe.StableOK, probe.PixOK, probe.GoldenOK, probe.GoldenWrote, probe.GoldenChanged,
		probe.PollUs, probe.ReloadUs, probe.Detail, probe.PixDetail)
	if !probe.OK {
		if *autoOnly {
			failJSON(probe)
		} else {
			fmt.Fprintln(os.Stderr, "game_asset: selftest FAIL, not opening window")
		}
		os.Exit(1)
	}

	var secs int
	if *autoOnly {
		secs = runSeconds(8)
		wrkit.RequireMinRun(secs, abilityID)
	} else if *manualSeconds > 0 {
		secs = *manualSeconds
	} else {
		secs, _ = wrkit.RunSecondsOpt()
	}
	manualMode := !*autoOnly

	var runFor time.Duration
	if secs > 0 {
		runFor = time.Duration(secs) * time.Second
	}

	shell := wrkit.NewShell(winW, winH, "game_asset — 12.3 热重载 (asset-reload)", []string{
		"改一个文件只换一个资源",
		"坏文件保旧图不崩",
		"看门人与直装逐位一致",
		"依赖断报哪条链",
		"右栏 重载/事件/轮询",
		"JSON见 ability_extra",
	})

	// Live ledger: the window watches only its own slot file.
	mgr := asset.NewManager()
	watcher := asset.NewWatcher(mgr)
	liveAsset, err := watcher.Watch(core.AssetID(liveID), liveRel, asset.KindTextureKTX2, asset.CurrentVersion, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: live watch:", err)
		os.Exit(1)
	}
	_ = liveAsset

	sim := &assetSim{
		mgr:     mgr,
		watcher: watcher,
		shell:   shell,
	}
	if secs > 0 {
		sim.phase = wrkit.NewPhaseClock(float64(secs)*0.5, float64(secs)*0.8)
	} else {
		sim.phase = wrkit.NewPhaseClock(0, 0)
	}

	// Left: live slot (two swatches stacked, toggle by loaded size).
	shell.Body.Place(wrkit.Label("LIVE 看门文件·改即换", 13, 0.55, 0.75, 0.95), liveX, liveY-24)
	sim.redBox = rendering.NewRenderColorBox(swW, swH, redR, redG, redB, 1)
	shell.Body.Place(sim.redBox, liveX, liveY)
	sim.chkBox = rendering.NewRenderColorBox(swW, swH, chkR, chkG, chkB, 1)
	sim.chkBox.SetAlpha(0)
	shell.Body.Place(sim.chkBox, liveX, liveY)
	sim.hashL = wrkit.Label("哈希 0", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.hashL, liveX, liveY+swH+10)
	sim.sizeL = wrkit.Label("字节 0", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.sizeL, liveX, liveY+swH+36)
	sim.stateL = wrkit.Label("看门 watching", 12, 0.70, 0.78, 0.88)
	shell.Body.Place(sim.stateL, liveX, liveY+swH+62)

	// Middle: dependency chain served by the real ledger.
	shell.Body.Place(wrkit.Label("DEP 依赖链", 13, 0.55, 0.75, 0.95), depX, depY-24)
	shell.Body.Place(wrkit.Label("map/level1 需两张贴图", 12, 0.70, 0.78, 0.88), depX, depY+10)
	shell.Body.Place(wrkit.Label("tex/red 被两处引用", 12, 0.70, 0.78, 0.88), depX, depY+36)
	sim.chainL = wrkit.Label("断链报哪条链见 JSON", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.chainL, depX, depY+62)
	shell.Body.Place(wrkit.Label("坏文件保旧不崩", 12, 0.70, 0.78, 0.88), depX, depY+88)
	shell.Body.Place(wrkit.Label("看门人与直装逐位一致", 12, 0.70, 0.78, 0.88), depX, depY+114)

	// Right: live counters.
	shell.Body.Place(wrkit.Label("COUNTERS 计数器", 13, 0.55, 0.75, 0.95), countX, countY-24)
	sim.reloadL = wrkit.Label("重载数 0", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.reloadL, countX, countY+10)
	sim.eventL = wrkit.Label("事件数 0", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.eventL, countX, countY+36)
	sim.perfL = wrkit.Label("轮询/重载目标", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.perfL, countX, countY+62)
	sim.fpsL = wrkit.Label("帧率 0", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.fpsL, countX, countY+88)

	shell.Body.Place(wrkit.Label("改 testdata/live_slot.ktx2 即换 · 红112B/青136B来回 · 右栏为live计数", 12, 0.70, 0.78, 0.88), liveX, noteY)

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "game_asset", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	defer win.Close()
	ctl := win.Controls()

	var summary manualSummary
	app := embedder.NewPipelineApp(win.Host(), shell.Root, embedder.PipelineOptions{
		ClearR: bgR, ClearG: bgG, ClearB: bgB, ClearA: 1,
		RunFor: runFor,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				fmt.Fprintf(os.Stderr, "game_asset: close (%s)\n", win.Backend())
				return
			case platform.EventPointer:
				summary.Pointer++
				if manualMode {
					fmt.Fprintf(os.Stderr, "game_asset: pointer %s (%.0f,%.0f) n=%d\n",
						ev.Pointer, ev.X, ev.Y, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_asset events=%d", summary.Pointer+summary.Key+summary.Resize))
					}
				}
				return
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					if manualMode {
						fmt.Fprintf(os.Stderr, "game_asset: key n=%d\n", summary.Pointer+summary.Key+summary.Resize)
						if ctl != nil {
							ctl.SetTitle(fmt.Sprintf("game_asset events=%d", summary.Pointer+summary.Key+summary.Resize))
						}
					}
				}
				return
			case platform.EventResize:
				summary.Resize++
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
				if manualMode {
					fmt.Fprintf(os.Stderr, "game_asset: resize %dx%d n=%d\n", ev.Width, ev.Height, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_asset events=%d", summary.Pointer+summary.Key+summary.Resize))
					}
				}
				return
			default:
				return
			}
		},
	})
	sim.app = app
	app.Scheduler().Tickers().Add(&ticker{s: sim})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	shell.Root.MarkNeedsPaint()
	app.ScheduleFrame()

	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}
	app.Close()
	elapsed := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	presents := app.PresentCount()
	probeOK := 0
	if probe.OK {
		probeOK = 1
	}
	liveSt := watcher.Stats()
	liveCur, _ := mgr.Wait(core.AssetID(liveID))
	var liveHash uint64
	var liveSize int
	if liveCur != nil {
		liveHash, liveSize = liveCur.Hash(), liveCur.Size()
	}
	extra := map[string]any{
		"case":             "reload",
		"probe_ok":         probeOK,
		"swap_ok":          probe.SwapOK,
		"identical_ok":     probe.IdenticalOK,
		"bad_kept_ok":      probe.BadKeptOK,
		"deps_ok":          probe.DepsOK,
		"stable_ok":        probe.StableOK,
		"reloads":          probe.Reloads,
		"events":           probe.Events,
		"poll_us":          probe.PollUs,
		"reload_us":        probe.ReloadUs,
		"poll_target_us":   pollTargetUs,
		"reload_target_us": reloadTargetUs,
		"from_hash":        probe.FromHash,
		"to_hash":          probe.ToHash,
		"from_size":        probe.FromSize,
		"to_size":          probe.ToSize,
		"dep_chain":        probe.DepChain,
		"dependents":       probe.Dependents,
		"live_reloads":     liveSt.Reloads,
		"live_events":      liveSt.Events,
		"live_hash":        liveHash,
		"live_size":        liveSize,
		"live_total":       mgr.TotalBytes(),
		"boundary_skip":    snap.BoundarySkip,
		"pixels":           probe.PixDetail,
		"golden":           probe.GoldenChanged,
	}

	if *autoOnly {
		report := wrgate.BuildReport(wrgate.BuildInput{
			AbilityID:     abilityID,
			Scenario:      scenario,
			Snap:          snap,
			PresentCount:  presents,
			ElapsedSec:    elapsed,
			SurfaceAreaPx: winW * winH,
			Warmup:        true,
			Extra:         extra,
		})
		raw, _ := json.Marshal(report)
		fmt.Println(string(raw))
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1}); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL:", err)
			os.Exit(1)
		}
		if presents < 1 || !probe.OK {
			fmt.Fprintf(os.Stderr, "FAIL: presents=%d probe=%v (want >=1, true)\n", presents, probe.OK)
			os.Exit(1)
		}
		if !probe.SwapOK || !probe.IdenticalOK || !probe.BadKeptOK || !probe.DepsOK || !probe.StableOK {
			fmt.Fprintf(os.Stderr, "FAIL: subgates swap=%v identical=%v bad=%v deps=%v stable=%v (want all true)\n",
				probe.SwapOK, probe.IdenticalOK, probe.BadKeptOK, probe.DepsOK, probe.StableOK)
			os.Exit(1)
		}
		if probe.PollUs > maxPollUs || probe.ReloadUs > maxReloadUs {
			fmt.Fprintf(os.Stderr, "FAIL: poll=%.1fus reload=%.1fus (want <=%.0f/%.0f)\n",
				probe.PollUs, probe.ReloadUs, maxPollUs, maxReloadUs)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "game_asset: OK presents=%d reloads=%d events=%d poll=%.1fus reload=%.1fus chain=%s elapsed=%.1fs\n",
			presents, probe.Reloads, probe.Events, probe.PollUs, probe.ReloadUs, probe.DepChain, elapsed)
		return
	}
	summary.Timed = secs > 0
	b, _ := json.Marshal(map[string]any{
		"ability_id": abilityID,
		"scenario":   scenario,
		"backend":    win.Backend().String(),
		"events": map[string]any{
			"pointer": summary.Pointer, "key": summary.Key, "resize": summary.Resize,
		},
		"presents":     presents,
		"elapsed_sec":  elapsed,
		"reloads":      probe.Reloads,
		"events_watch": liveSt.Events,
		"live_hash":    liveHash,
		"live_size":    liveSize,
		"dep_chain":    probe.DepChain,
		"probe_ok":     probeOK,
		"timed":        summary.Timed,
		"note":         summary.Note,
	})
	fmt.Println(string(b))
	fmt.Fprintf(os.Stderr, "game_asset: backend=%s presents=%d reloads=%d live_hash=%d elapsed=%.1fs\n",
		win.Backend(), presents, probe.Reloads, liveHash, elapsed)
}

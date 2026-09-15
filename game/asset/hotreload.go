package asset

import (
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/energye/gpui/game/core"
)

// Frozen hot-reload budgets. Beyond budget is OutOfMemory, never a guess.
const (
	// MaxWatches caps watched ids in one Watcher.
	MaxWatches = 1024
	// MaxWatchIDLen caps the watch id text length in bytes.
	MaxWatchIDLen = 128
	// MaxWatchPathLen caps the watch path text length in bytes.
	MaxWatchPathLen = 1024
	// PollInterval is the fallback poll step used when the caller drives
	// Poll without a file event. File events always win over the tick.
	PollInterval = 50 * time.Millisecond
)

// HotState names the lifecycle of one watched id.
type HotState int

const (
	// HotEmpty means no watch for the id.
	HotEmpty HotState = iota
	// HotWatching means the file is watched and in sync.
	HotWatching
	// HotDirty means the file changed on disk and waits for Reload.
	HotDirty
	// HotFailed is the placeholder for a failed Reload (bad file, missing
	// file, over budget). The last good snapshot stays readable.
	HotFailed
)

var hotStateNames = []string{"empty", "watching", "dirty", "failed"}

// String returns the stable log name of s.
func (s HotState) String() string {
	if s >= HotEmpty && int(s) < len(hotStateNames) {
		return hotStateNames[int(s)]
	}
	return "empty"
}

// HotStats reports Watcher counters. Watches is the watched id count;
// Reloads counts successful Reload calls; Failures counts failed Reload
// calls; Events counts file changes noticed through Poll.
type HotStats struct {
	Watches  int
	Reloads  int64
	Failures int64
	Events   int64
}

// watchEntry pins one id to one file plus its load shape. The manager
// keeps the payload; the watcher only remembers how to reload it.
type watchEntry struct {
	id    core.AssetID
	path  string
	kind  Kind
	ver   core.Version
	deps  []core.AssetID
	size  int64
	mod   time.Time
	state HotState
	cause error
}

// Watcher watches files for one Manager and reloads only the changed
// id through Manager.LoadFile, so a texture edit never touches maps and
// saves. It draws nothing; the caller draws the reloaded bytes with the
// existing render draws. Only core numbers are used; the old asset and
// tex paths stay untouched (LoadFile is only read, never modified).
//
// Threading: every method locks. Poll compares size plus modtime plus a
// content hash probe only when size/modtime moved, so the hot path stays
// a stat call. Reload re-reads the file through the manager, keeping the
// extra claim drained so the ledger does not grow.
type Watcher struct {
	mu      sync.Mutex
	mgr     *Manager
	watches map[core.AssetID]*watchEntry
	reloads int64
	fails   int64
	events  int64
}

// NewWatcher builds a watcher over mgr. A nil mgr reports InvalidArg on
// Watch and Reload, queries report zero values.
func NewWatcher(mgr *Manager) *Watcher {
	return &Watcher{mgr: mgr, watches: map[core.AssetID]*watchEntry{}}
}

// Manager returns the watched asset ledger, or nil when unset.
func (w *Watcher) Manager() *Manager {
	if w == nil {
		return nil
	}
	return w.mgr
}

func checkWatchID(id core.AssetID) error {
	if id.Empty() {
		return core.InvalidArg("hotreload", "")
	}
	if len(string(id)) > MaxWatchIDLen {
		return core.InvalidArg("hotreload", string(id))
	}
	return nil
}

func checkWatchPath(path string) error {
	if path == "" {
		return core.InvalidArg("hotreload", "path")
	}
	if len(path) > MaxWatchPathLen {
		return core.InvalidArg("hotreload", "path")
	}
	return nil
}

func fileFingerprint(path string) (size int64, mod time.Time, ok bool) {
	fi, err := os.Stat(filepath.Clean(path))
	if err != nil || fi.IsDir() {
		return 0, time.Time{}, false
	}
	return fi.Size(), fi.ModTime(), true
}

func sameFingerprint(aSize, bSize int64, aMod, bMod time.Time) bool {
	return aSize == bSize && aMod.Equal(bMod)
}

// Watch pins id to path with the load shape Reload replays. The current
// file loads immediately through Manager.LoadFile, so Watch both starts
// watching and fills the first snapshot. Empty ids/paths are InvalidArg;
// unknown kinds are InvalidArg; bad deps are InvalidArg/OutOfMemory and
// store nothing. Missing or torn files report the manager cause and
// store no watch: fix the file, then Watch again.
func (w *Watcher) Watch(id core.AssetID, path string, kind Kind, ver core.Version, deps []core.AssetID) (*Asset, error) {
	const op = "hotreload.Watch"
	if w == nil || w.mgr == nil {
		return nil, core.InvalidArg(op, "watcher")
	}
	if err := checkWatchID(id); err != nil {
		return nil, err
	}
	if err := checkWatchPath(path); err != nil {
		return nil, err
	}
	if !kind.Valid() {
		return nil, core.InvalidArg(op, string(id))
	}
	if err := checkDeps(deps, id); err != nil {
		return nil, err
	}
	got, err := w.mgr.LoadFile(id, path, kind, ver, deps)
	if err != nil {
		return got, err
	}
	size, mod, ok := fileFingerprint(path)
	if !ok {
		// Bytes cached but the file vanished mid-watch: drop the
		// claim so the ledger stays balanced, store no watch.
		_ = w.mgr.Unload(id)
		return got, core.NotFound(op, string(id))
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	_, dup := w.watches[id]
	if !dup && len(w.watches) >= MaxWatches {
		// Over budget: drop the claim, store no watch.
		_ = w.mgr.Unload(id)
		return got, core.OutOfMemory(op, string(id))
	}
	w.watches[id] = &watchEntry{
		id: id, path: filepath.Clean(path), kind: kind, ver: ver,
		deps: cloneIDs(deps), size: size, mod: mod, state: HotWatching,
	}
	if dup {
		// Re-watch replays the shape like Reload: drain the extra
		// claim so repeated Watch calls hold one claim.
		_ = w.mgr.Unload(id)
	}
	return got, nil
}

// Unwatch drops the watch on id but keeps the manager bytes for quick
// re-watch. Unknown ids are NotFound. Nil watchers report InvalidArg.
func (w *Watcher) Unwatch(id core.AssetID) error {
	const op = "hotreload.Unwatch"
	if w == nil {
		return core.InvalidArg(op, "watcher")
	}
	if err := checkWatchID(id); err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.watches[id]; !ok {
		return core.NotFound(op, string(id))
	}
	delete(w.watches, id)
	return nil
}

// Watched reports whether id is watched now.
func (w *Watcher) Watched(id core.AssetID) bool {
	if w == nil || id.Empty() {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	_, ok := w.watches[id]
	return ok
}

// WatchedIDs lists watched ids sorted for replay.
func (w *Watcher) WatchedIDs() []core.AssetID {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]core.AssetID, 0, len(w.watches))
	for id := range w.watches {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// StateOf returns the lifecycle state of id.
func (w *Watcher) StateOf(id core.AssetID) HotState {
	if w == nil || id.Empty() {
		return HotEmpty
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if e, ok := w.watches[id]; ok {
		return e.state
	}
	return HotEmpty
}

// CauseOf replays the stored Reload failure for id, or nil when the last
// Reload succeeded or none ran.
func (w *Watcher) CauseOf(id core.AssetID) error {
	if w == nil || id.Empty() {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if e, ok := w.watches[id]; ok {
		return e.cause
	}
	return core.NotFound("hotreload.CauseOf", string(id))
}

// PathOf returns the watched path of id, or "" when unwatched.
func (w *Watcher) PathOf(id core.AssetID) string {
	if w == nil || id.Empty() {
		return ""
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if e, ok := w.watches[id]; ok {
		return e.path
	}
	return ""
}

// Poll stats every watched file and marks changed ids Dirty. It never
// loads: call Reload to swap bytes. Missing files mark Dirty too, so the
// delete shows up and Reload reports NotFound instead of silence. Nil
// watchers report InvalidArg. It returns the newly dirty ids sorted.
//
// Change detection is size plus modtime (one stat per file, no reads),
// so the steady state never pays file I/O. Callers that rewrite a file
// faster than the filesystem timestamp tick must bump the modtime (see
// TouchFile in the test) or the change stays invisible until the next
// tick, exactly like classic make-style watchers.
func (w *Watcher) Poll() ([]core.AssetID, error) {
	const op = "hotreload.Poll"
	if w == nil {
		return nil, core.InvalidArg(op, "watcher")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	var dirty []core.AssetID
	for id, e := range w.watches {
		size, mod, ok := fileFingerprint(e.path)
		if !ok {
			if e.state != HotDirty {
				e.state = HotDirty
				e.cause = nil
				w.events++
				dirty = append(dirty, id)
			}
			continue
		}
		if sameFingerprint(size, e.size, mod, e.mod) {
			continue
		}
		// Do not adopt the new fingerprint here: only a successful
		// Reload adopts it. A failed Reload leaves the old one, so
		// the next Poll reports the id Dirty again until fixed.
		if e.state != HotDirty {
			w.events++
		}
		e.state = HotDirty
		e.cause = nil
		dirty = append(dirty, id)
	}
	sort.Slice(dirty, func(i, j int) bool { return dirty[i] < dirty[j] })
	return dirty, nil
}

// Reload re-reads the watched file of id through Manager.LoadFile and
// swaps only that id: other ids keep their bytes and claims. A Dirty id
// that loads clean returns to Watching; a failed Reload keeps the last
// good snapshot readable and parks the watch at Failed with CauseOf set.
// A failed Reload never adopts the bad fingerprint, so the next Poll
// reports the id Dirty again until the file is fixed. Clean ids reload
// anyway (explicit refresh) and stay Watching on success. Unknown ids
// are NotFound. Nil watchers/managers report InvalidArg.
func (w *Watcher) Reload(id core.AssetID) (*Asset, error) {
	const op = "hotreload.Reload"
	if w == nil || w.mgr == nil {
		return nil, core.InvalidArg(op, "watcher")
	}
	if err := checkWatchID(id); err != nil {
		return nil, err
	}
	w.mu.Lock()
	e, ok := w.watches[id]
	if !ok {
		w.mu.Unlock()
		return nil, core.NotFound(op, string(id))
	}
	path, kind, ver, deps := e.path, e.kind, e.ver, cloneIDs(e.deps)
	w.mu.Unlock()
	// Snapshot the last good bytes first: a torn file must never wipe
	// the art on screen, so a failed swap restores them. Manager.Load
	// overwrites Ready bytes on decoding success only; a torn KTX2
	// leaves Ready intact, but the restore closes the gap anyway. A
	// missing file stores a Missing placeholder and drops Ready, so the
	// snapshot below is the only way the picture survives.
	var good []byte
	var goodDeps []core.AssetID
	if cur, cerr := w.mgr.Wait(id); cerr == nil && cur.State() == StateReady {
		good, goodDeps = cur.Bytes(), cur.Deps()
	}
	got, err := w.mgr.LoadFile(id, path, kind, ver, deps)
	if err != nil {
		if len(good) > 0 {
			// Restore the last good payload through the same Load
			// path, then drain the restore claim so the ledger
			// holds the same single claim as before the failure.
			if _, rerr := w.mgr.Load(id, kind, ver, good, goodDeps); rerr == nil {
				_ = w.mgr.Unload(id)
				got, _ = w.mgr.Wait(id)
			}
		}
		w.mu.Lock()
		if cur, still := w.watches[id]; still {
			cur.state = HotFailed
			cur.cause = err
		}
		w.fails++
		w.mu.Unlock()
		// Drain the extra claim a failed store may hold so the ledger
		// does not grow: Failed placeholders carry no claim.
		return got, err
	}
	// Drain the extra Load claim so repeated reloads hold one claim.
	_ = w.mgr.Unload(id)
	size, mod, _ := fileFingerprint(path)
	w.mu.Lock()
	defer w.mu.Unlock()
	if cur, still := w.watches[id]; still {
		cur.size, cur.mod = size, mod
		cur.state = HotWatching
		cur.cause = nil
	}
	w.reloads++
	return got, nil
}

// ReloadDirty polls once then reloads every Dirty id, returning the
// reloaded snapshots in sorted id order. Failed ids stay Failed with
// CauseOf set and do not stop the rest. Nil watchers report InvalidArg.
func (w *Watcher) ReloadDirty() ([]*Asset, error) {
	const op = "hotreload.ReloadDirty"
	if w == nil {
		return nil, core.InvalidArg(op, "watcher")
	}
	dirty, err := w.Poll()
	if err != nil {
		return nil, err
	}
	out := make([]*Asset, 0, len(dirty))
	for _, id := range dirty {
		got, rerr := w.Reload(id)
		if rerr != nil {
			continue
		}
		out = append(out, got)
	}
	return out, nil
}

// Stats returns a copy of the watcher counters.
func (w *Watcher) Stats() HotStats {
	if w == nil {
		return HotStats{}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return HotStats{Watches: len(w.watches), Reloads: w.reloads, Failures: w.fails, Events: w.events}
}

// Count returns the watched id count.
func (w *Watcher) Count() int {
	if w == nil {
		return 0
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.watches)
}

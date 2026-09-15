package asset

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/energye/gpui/game/core"
)

// Window intent: game_asset --case=reload rides P2 (W3 gate needs an
// independent window per ability; the window is not built yet). Until
// then TestHotReloadOffscreenGolden is the offscreen comparison plus
// the auto verdict: one edit swaps one id, bad files never wipe art,
// both sides stay separate, reload cost and leak count are logged.

type hotWatchCase struct {
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

type hotSwapCase struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	From     string `json:"from"`
	To       string `json:"to"`
	Version  string `json:"version"`
	FromHash uint64 `json:"from_hash"`
	ToHash   uint64 `json:"to_hash"`
}

type hotBadCase struct {
	File     string `json:"file"`
	Kind     string `json:"kind"`
	WantCode string `json:"want_code"`
}

type hotFile struct {
	Version        string         `json:"version"`
	MaxWatches     int            `json:"max_watches"`
	MaxWatchIDLen  int            `json:"max_watch_id_len"`
	MaxWatchPath   int            `json:"max_watch_path_len"`
	PollIntervalMs int64          `json:"poll_interval_ms"`
	Tolerance      int            `json:"tolerance"`
	Watches        []hotWatchCase `json:"watches"`
	Swap           hotSwapCase    `json:"swap"`
	BadFile        hotBadCase     `json:"bad_file"`
	Unsupported    []string       `json:"unsupported_kinds"`
}

func loadHotCases(t *testing.T) hotFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "hotreload_cases.json"))
	if err != nil {
		t.Fatalf("read hotreload_cases.json: %v", err)
	}
	var c hotFile
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("decode hotreload_cases.json: %v", err)
	}
	if len(c.Watches) == 0 || c.Swap.ID == "" || c.BadFile.File == "" {
		t.Fatal("hotreload_cases.json has no watches/swap/bad file")
	}
	if len(c.Unsupported) == 0 {
		t.Fatal("hotreload_cases.json has no unsupported kinds")
	}
	return c
}

func mustFindHot(t *testing.T, c hotFile, id string) hotWatchCase {
	t.Helper()
	for _, w := range c.Watches {
		if w.ID == id {
			return w
		}
	}
	t.Fatalf("hotreload_cases.json has no watch %q", id)
	return hotWatchCase{}
}

func mustParseHotKind(t *testing.T, name string) Kind {
	t.Helper()
	k, err := ParseKind(name)
	if err != nil {
		t.Fatalf("ParseKind %q: %v", name, err)
	}
	return k
}

func mustParseHotVer(t *testing.T, s string) core.Version {
	t.Helper()
	v, err := core.ParseVersion(s)
	if err != nil {
		t.Fatalf("ParseVersion %q: %v", s, err)
	}
	return v
}

func hotDeps(deps []string) []core.AssetID {
	if len(deps) == 0 {
		return nil
	}
	out := make([]core.AssetID, len(deps))
	for i, d := range deps {
		out[i] = core.AssetID(d)
	}
	return out
}

func hotCode(s string) core.Code {
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

func checkHotWatch(t *testing.T, want hotWatchCase, got *Asset) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s: got nil asset", want.ID)
	}
	if got.ID() != core.AssetID(want.ID) {
		t.Errorf("%s: id = %q", want.ID, got.ID())
	}
	if got.Kind().String() != want.Kind {
		t.Errorf("%s: kind = %v, want %v", want.ID, got.Kind(), want.Kind)
	}
	if got.Hash() != want.Hash {
		t.Errorf("%s: hash = %d, want %d", want.ID, got.Hash(), want.Hash)
	}
	if got.Size() != want.Size {
		t.Errorf("%s: size = %d, want %d", want.ID, got.Size(), want.Size)
	}
	if got.UploadSize() != want.Upload || got.PixelSize() != want.Pixels {
		t.Errorf("%s: upload/pixels = %d/%d, want %d/%d", want.ID, got.UploadSize(), got.PixelSize(), want.Upload, want.Pixels)
	}
	if got.State() != StateReady {
		t.Errorf("%s: state = %v, want ready", want.ID, got.State())
	}
}

// stageWatchDir copies the golden files into a TempDir work area and
// returns the dir. Tests edit the copies, never testdata: the golden
// stays frozen on disk, edits live only in the process temp dir.
func stageWatchDir(t *testing.T, c hotFile) string {
	t.Helper()
	dir := t.TempDir()
	for _, w := range c.Watches {
		raw, err := os.ReadFile(filepath.Join("testdata", w.File))
		if err != nil {
			t.Fatalf("read %s: %v", w.File, err)
		}
		if err := os.WriteFile(filepath.Join(dir, w.File), raw, 0644); err != nil {
			t.Fatalf("stage %s: %v", w.File, err)
		}
	}
	for _, f := range []string{c.Swap.From, c.Swap.To, c.BadFile.File} {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			continue
		}
		raw, err := os.ReadFile(filepath.Join("testdata", f))
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if err := os.WriteFile(filepath.Join(dir, f), raw, 0644); err != nil {
			t.Fatalf("stage %s: %v", f, err)
		}
	}
	return dir
}

func mustWatchHot(t *testing.T, w *Watcher, dir string, want hotWatchCase) *Asset {
	t.Helper()
	got, err := w.Watch(core.AssetID(want.ID), filepath.Join(dir, want.File),
		mustParseHotKind(t, want.Kind), mustParseHotVer(t, want.Version), hotDeps(want.Deps))
	if err != nil {
		t.Fatalf("%s: Watch: %v", want.ID, err)
	}
	checkHotWatch(t, want, got)
	return got
}

// touchFile bumps the modtime past the filesystem tick so a same-size
// rewrite is visible to stat-based Poll (classic make caveat). Same-size
// content swaps with different bytes but identical size still surface
// because the test bumps the clock; production editors always bump it.
func touchFile(t *testing.T, path string, mod time.Time) {
	t.Helper()
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatalf("chtimes %s: %v", path, err)
	}
}

// rewriteHot copies src testdata bytes over the watched path and bumps
// the modtime, so Poll sees one stat-level change for the swap.
func rewriteHot(t *testing.T, dir, watched, src string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", src))
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	dst := filepath.Join(dir, watched)
	if err := os.WriteFile(dst, raw, 0644); err != nil {
		t.Fatalf("rewrite %s: %v", watched, err)
	}
	touchFile(t, dst, time.Now().Add(2*time.Second))
}

// expectHotCode fails the test unless err carries want.
func expectHotCode(t *testing.T, name string, err error, want core.Code) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: want error", name)
		return
	}
	if core.CodeOf(err) != want {
		t.Errorf("%s code = %v, want %v", name, core.CodeOf(err), want)
	}
}

// A:一改就换:Watch装首快照,Poll标脏,Reload只换那块.
func TestHotReloadSwapFromCases(t *testing.T) {
	c := loadHotCases(t)
	if MaxWatches != c.MaxWatches || MaxWatchIDLen != c.MaxWatchIDLen ||
		MaxWatchPathLen != c.MaxWatchPath || PollInterval.Milliseconds() != c.PollIntervalMs {
		t.Fatal("hotreload budgets diverge from hotreload_cases.json")
	}
	if HotWatching.String() != "watching" || HotDirty.String() != "dirty" || HotFailed.String() != "failed" {
		t.Fatal("hot state names drifted")
	}
	dir := stageWatchDir(t, c)
	m := NewManager()
	w := NewWatcher(m)
	for _, want := range c.Watches {
		mustWatchHot(t, w, dir, want)
	}
	if w.Count() != len(c.Watches) {
		t.Fatalf("count = %d, want %d", w.Count(), len(c.Watches))
	}
	// Steady Poll sees nothing: no spurious events without edits.
	if dirty, err := w.Poll(); err != nil || len(dirty) != 0 {
		t.Fatalf("steady Poll = %v/%v, want empty", dirty, err)
	}
	// Rewrite one watched file with the other golden: exactly one id
	// goes dirty, the rest stay Watching.
	sw := c.Swap
	target := ""
	for _, want := range c.Watches {
		if want.File == sw.From {
			target = want.ID
			break
		}
	}
	if target == "" {
		t.Fatalf("swap from %q matches no watch", sw.From)
	}
	rewriteHot(t, dir, sw.From, sw.To)
	dirty, err := w.Poll()
	if err != nil {
		t.Fatalf("Poll after edit: %v", err)
	}
	if len(dirty) != 1 || dirty[0] != core.AssetID(target) {
		t.Fatalf("dirty = %v, want [%s]", dirty, target)
	}
	if w.StateOf(core.AssetID(target)) != HotDirty {
		t.Fatalf("state = %v, want dirty", w.StateOf(core.AssetID(target)))
	}
	// Snapshot the neighbour before the swap: only the edited id moves.
	neighbour := mustFindHot(t, c, "map/level1")
	beforeOther, err := m.Wait(core.AssetID(neighbour.ID))
	if err != nil {
		t.Fatalf("neighbour Wait: %v", err)
	}
	got, err := w.Reload(core.AssetID(target))
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if got.Hash() != sw.ToHash {
		t.Fatalf("hash = %d, want %d", got.Hash(), sw.ToHash)
	}
	if w.StateOf(core.AssetID(target)) != HotWatching {
		t.Fatalf("after reload state = %v, want watching", w.StateOf(core.AssetID(target)))
	}
	afterOther, err := m.Wait(core.AssetID(neighbour.ID))
	if err != nil {
		t.Fatalf("neighbour re-Wait: %v", err)
	}
	if !beforeOther.Equal(afterOther) {
		t.Fatal("neighbour moved during the swap, want only one id swapped")
	}
	if st := w.Stats(); st.Reloads != 1 || st.Events != 1 || st.Watches != len(c.Watches) {
		t.Fatalf("stats = %+v, want 1 reload 1 event %d watches", st, len(c.Watches))
	}
	// ReloadDirty on the quiet path reloads nothing.
	if out, err := w.ReloadDirty(); err != nil || len(out) != 0 {
		t.Fatalf("quiet ReloadDirty = %d/%v, want empty", len(out), err)
	}
	// Second edit plus ReloadDirty swaps without naming the id.
	rewriteHot(t, dir, sw.From, sw.From)
	if _, err := w.Poll(); err != nil {
		t.Fatalf("second Poll: %v", err)
	}
	out, err := w.ReloadDirty()
	if err != nil {
		t.Fatalf("ReloadDirty: %v", err)
	}
	if len(out) != 1 || out[0].Hash() != sw.FromHash {
		t.Fatalf("ReloadDirty hash = %v, want back to %d", out, sw.FromHash)
	}
}

// B:坏文件不崩:缺文件坏字节删文件全占位加报错,好图不花.
func TestHotReloadEdgesNoCrash(t *testing.T) {
	c := loadHotCases(t)
	var nilW *Watcher
	if _, err := nilW.Poll(); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Poll code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := nilW.Reload("x"); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Reload code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := nilW.ReloadDirty(); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil ReloadDirty code = %v, want invalid-arg", core.CodeOf(err))
	}
	if err := nilW.Unwatch("x"); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Unwatch code = %v, want invalid-arg", core.CodeOf(err))
	}
	if nilW.Watched("x") || nilW.Count() != 0 || nilW.StateOf("x") != HotEmpty {
		t.Error("nil queries want unwatched/0/empty")
	}
	if nilW.Stats() != (HotStats{}) || nilW.PathOf("x") != "" || nilW.Manager() != nil {
		t.Error("nil stats/path/manager want zeros")
	}
	if nilW.CauseOf("x") != nil {
		t.Error("nil CauseOf want nil")
	}
	// Nil manager builds but refuses to watch or reload.
	nm := NewWatcher(nil)
	if nm.Manager() != nil {
		t.Error("nil-mgr Manager want nil")
	}
	if _, err := nm.Watch("a", "p", KindRaw, CurrentVersion, nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil-mgr Watch code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := nm.Reload("a"); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil-mgr Reload code = %v, want invalid-arg", core.CodeOf(err))
	}
	m := NewManager()
	w := NewWatcher(m)
	dir := stageWatchDir(t, c)
	if _, err := w.Watch("", filepath.Join(dir, "x"), KindRaw, CurrentVersion, nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty id Watch code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := w.Watch("a", "", KindRaw, CurrentVersion, nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty path Watch code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := w.Watch("a", filepath.Join(dir, "x"), KindUnknown, CurrentVersion, nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("unknown kind Watch code = %v, want invalid-arg", core.CodeOf(err))
	}
	longID := core.AssetID(strings.Repeat("x", MaxWatchIDLen+1))
	if _, err := w.Watch(longID, filepath.Join(dir, "x"), KindRaw, CurrentVersion, nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("long id Watch code = %v, want invalid-arg", core.CodeOf(err))
	}
	longPath := strings.Repeat("x", MaxWatchPathLen+1)
	if _, err := w.Watch("a", longPath, KindRaw, CurrentVersion, nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("long path Watch code = %v, want invalid-arg", core.CodeOf(err))
	}
	self := core.AssetID("self/loop")
	if _, err := w.Watch(self, filepath.Join(dir, "x"), KindRaw, CurrentVersion, []core.AssetID{self}); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("self dep Watch code = %v, want invalid-arg", core.CodeOf(err))
	}
	// Missing file never stores a watch: fix it, then Watch again.
	if _, err := w.Watch("ghost/tex", filepath.Join(dir, "no_such.ktx2"), KindTextureKTX2, CurrentVersion, nil); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("missing Watch code = %v, want not-found", core.CodeOf(err))
	}
	if w.Watched("ghost/tex") {
		t.Error("missing file stored a watch, want none")
	}
	// Torn file never stores a watch either.
	badRaw, err := os.ReadFile(filepath.Join("testdata", c.BadFile.File))
	if err != nil {
		t.Fatalf("read %s: %v", c.BadFile.File, err)
	}
	badPath := filepath.Join(dir, "torn.ktx2")
	if err := os.WriteFile(badPath, badRaw, 0644); err != nil {
		t.Fatalf("stage torn: %v", err)
	}
	if _, err := w.Watch("bad/tex", badPath, KindTextureKTX2, CurrentVersion, nil); hotCode(c.BadFile.WantCode) != core.CodeOf(err) {
		t.Errorf("torn Watch code = %v, want %v", core.CodeOf(err), c.BadFile.WantCode)
	}
	if w.Watched("bad/tex") {
		t.Error("torn file stored a watch, want none")
	}
	// Unknown Reload/Unwatch report NotFound and change nothing.
	if _, err := w.Reload("never/watched"); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("unknown Reload code = %v, want not-found", core.CodeOf(err))
	}
	if err := w.Unwatch("never/watched"); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("unknown Unwatch code = %v, want not-found", core.CodeOf(err))
	}
	if _, err := w.Reload(""); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty Reload code = %v, want invalid-arg", core.CodeOf(err))
	}
	// A watched file deleted on disk goes Dirty; Reload reports NotFound
	// but the last good picture stays readable (art never wipes).
	want := mustFindHot(t, c, "tex/red")
	mustWatchHot(t, w, dir, want)
	victim := filepath.Join(dir, want.File)
	goodBefore, err := m.Wait(core.AssetID(want.ID))
	if err != nil {
		t.Fatalf("good Wait: %v", err)
	}
	if err := os.Remove(victim); err != nil {
		t.Fatalf("remove: %v", err)
	}
	dirty, err := w.Poll()
	if err != nil {
		t.Fatalf("Poll after delete: %v", err)
	}
	if len(dirty) != 1 || dirty[0] != core.AssetID(want.ID) {
		t.Fatalf("delete dirty = %v, want [%s]", dirty, want.ID)
	}
	if _, err := w.Reload(core.AssetID(want.ID)); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("delete Reload code = %v, want not-found", core.CodeOf(err))
	}
	if w.StateOf(core.AssetID(want.ID)) != HotFailed {
		t.Errorf("after failed reload state = %v, want failed", w.StateOf(core.AssetID(want.ID)))
	}
	if w.CauseOf(core.AssetID(want.ID)) == nil {
		t.Error("failed reload CauseOf nil, want the cause")
	}
	still, err := m.Wait(core.AssetID(want.ID))
	if err != nil {
		t.Fatalf("art after delete: %v", err)
	}
	if !still.Equal(goodBefore) {
		t.Fatal("delete wiped the picture, want the last good snapshot")
	}
	// A torn overwrite behaves the same: Failed parks, art survives, the
	// next Poll keeps reporting Dirty until the file is fixed.
	if err := os.WriteFile(victim, badRaw, 0644); err != nil {
		t.Fatalf("write torn: %v", err)
	}
	touchFile(t, victim, time.Now().Add(2*time.Second))
	if _, err := w.Poll(); err != nil {
		t.Fatalf("Poll torn: %v", err)
	}
	if _, err := w.Reload(core.AssetID(want.ID)); core.CodeOf(err) != hotCode(c.BadFile.WantCode) {
		t.Errorf("torn Reload code = %v, want %v", core.CodeOf(err), c.BadFile.WantCode)
	}
	if kept, err := m.Wait(core.AssetID(want.ID)); err != nil || !kept.Equal(goodBefore) {
		t.Fatalf("torn wiped the picture: %v", err)
	}
	// Unwatch drops the watch but keeps the manager bytes for re-watch.
	if err := w.Unwatch(core.AssetID(want.ID)); err != nil {
		t.Fatalf("Unwatch: %v", err)
	}
	if w.Watched(core.AssetID(want.ID)) || w.StateOf(core.AssetID(want.ID)) != HotEmpty {
		t.Error("after Unwatch still watched, want dropped")
	}
	if !m.Ready(core.AssetID(want.ID)) {
		t.Error("Unwatch dropped manager bytes, want them kept")
	}
}

// C不适用画画(纯管线):文件双路重放逐位一致,拷贝隔离即两边同数.
func TestHotReloadBoundaryIdentical(t *testing.T) {
	c := loadHotCases(t)
	dir := stageWatchDir(t, c)
	m := NewManager()
	w := NewWatcher(m)
	for _, want := range c.Watches {
		mustWatchHot(t, w, dir, want)
	}
	// Watch returns the same snapshot a direct manager load holds.
	for _, want := range c.Watches {
		raw, err := os.ReadFile(filepath.Join("testdata", want.File))
		if err != nil {
			t.Fatalf("read %s: %v", want.File, err)
		}
		other := NewManager()
		direct, err := other.LoadFile(core.AssetID(want.ID), filepath.Join("testdata", want.File),
			mustParseHotKind(t, want.Kind), mustParseHotVer(t, want.Version), hotDeps(want.Deps))
		if err != nil {
			t.Fatalf("%s: direct LoadFile: %v", want.ID, err)
		}
		if HashBytes(raw) != want.Hash || HashBytes(raw) != direct.Hash() {
			t.Fatalf("%s: hash boundary drifted", want.ID)
		}
		viaWatch, err := m.Wait(core.AssetID(want.ID))
		if err != nil {
			t.Fatalf("%s: watch Wait: %v", want.ID, err)
		}
		if !viaWatch.Equal(direct) {
			t.Fatalf("%s: watch vs direct diverged", want.ID)
		}
		// Copies never alias: mutating a return cannot corrupt replay.
		probe := viaWatch.Bytes()
		if len(probe) == 0 {
			t.Fatalf("%s: bytes empty, golden invalid", want.ID)
		}
		probe[0] ^= 0xFF
		if again, _ := m.Wait(core.AssetID(want.ID)); !again.Equal(direct) {
			t.Fatalf("%s: copy probe corrupted the watch", want.ID)
		}
	}
	// Watched ids sort for replay; paths survive clean.
	gotIDs := w.WatchedIDs()
	for i := 1; i < len(gotIDs); i++ {
		if gotIDs[i-1] >= gotIDs[i] {
			t.Fatalf("watched ids unsorted: %v", gotIDs)
		}
	}
	for _, want := range c.Watches {
		if p := w.PathOf(core.AssetID(want.ID)); p == "" {
			t.Errorf("%s: PathOf empty", want.ID)
		}
		if w.CauseOf(core.AssetID(want.ID)) != nil {
			t.Errorf("%s: clean CauseOf non-nil", want.ID)
		}
	}
	// Same swap twice replays the same bytes through both managers.
	sw := c.Swap
	rewriteHot(t, dir, sw.From, sw.To)
	if _, err := w.Poll(); err != nil {
		t.Fatalf("Poll: %v", err)
	}
	first, err := w.Reload(core.AssetID("tex/red"))
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	again := NewManager()
	againRaw, err := os.ReadFile(filepath.Join("testdata", sw.To))
	if err != nil {
		t.Fatalf("read %s: %v", sw.To, err)
	}
	second, err := again.Load(core.AssetID("tex/red"), KindTextureKTX2, mustParseHotVer(t, sw.Version), againRaw, nil)
	if err != nil {
		t.Fatalf("cross-manager Load: %v", err)
	}
	if !first.Equal(second) {
		t.Fatal("cross-manager replay diverged")
	}
}

// D:重载跑得动:批量轮询微秒级,单次重载耗时有数.
func TestHotReloadPerfReload(t *testing.T) {
	c := loadHotCases(t)
	dir := stageWatchDir(t, c)
	m := NewManager()
	w := NewWatcher(m)
	for _, want := range c.Watches {
		mustWatchHot(t, w, dir, want)
	}
	// Steady polls stay microsecond-scale: one stat per file, no reads.
	const polls = 2000
	start := time.Now()
	for i := 0; i < polls; i++ {
		if _, err := w.Poll(); err != nil {
			t.Fatalf("poll %d: %v", i, err)
		}
	}
	el := time.Since(start)
	t.Logf("hotreload-poll: %d steady polls on %d watches in %v (%.1f us/poll)", polls, len(c.Watches), el, float64(el.Microseconds())/polls)
	if el > 5*time.Second {
		t.Fatalf("steady polls took %v, play would stall", el)
	}
	// One edit plus one reload carries the measured swap cost.
	sw := c.Swap
	rewriteHot(t, dir, sw.From, sw.To)
	if _, err := w.Poll(); err != nil {
		t.Fatalf("Poll: %v", err)
	}
	rstart := time.Now()
	got, err := w.Reload(core.AssetID("tex/red"))
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	rel := time.Since(rstart)
	t.Logf("hotreload-reload: tex/red %dB swap in %v (hash %d)", got.Size(), rel, got.Hash())
	if got.Hash() != sw.ToHash {
		t.Fatalf("hash = %d, want %d", got.Hash(), sw.ToHash)
	}
}

// E:反复改不涨:百次来回重载字节恒定,计数器对账,坏档不粘.
func TestHotReloadLongRunStable(t *testing.T) {
	c := loadHotCases(t)
	dir := stageWatchDir(t, c)
	m := NewManager()
	w := NewWatcher(m)
	for _, want := range c.Watches {
		mustWatchHot(t, w, dir, want)
	}
	sw := c.Swap
	baseBytes := m.TotalBytes()
	baseLive := m.LiveCount("tex/red")
	first, err := m.Wait("tex/red")
	if err != nil {
		t.Fatalf("first Wait: %v", err)
	}
	// The swap endpoints differ in size (112B vs 136B), so the total
	// oscillates between two frozen values instead of one: swapping in
	// To grows the ledger by the size delta, swapping back shrinks it.
	swFromRaw, err := os.ReadFile(filepath.Join("testdata", sw.From))
	if err != nil {
		t.Fatalf("read %s: %v", sw.From, err)
	}
	swToRaw, err := os.ReadFile(filepath.Join("testdata", sw.To))
	if err != nil {
		t.Fatalf("read %s: %v", sw.To, err)
	}
	fromTotal := baseBytes
	toTotal := baseBytes - int64(len(swFromRaw)) + int64(len(swToRaw))
	for i := 0; i < 100; i++ {
		if i%2 == 0 {
			rewriteHot(t, dir, sw.From, sw.To)
		} else {
			rewriteHot(t, dir, sw.From, sw.From)
		}
		if _, err := w.Poll(); err != nil {
			t.Fatalf("rep %d Poll: %v", i, err)
		}
		got, err := w.Reload("tex/red")
		if err != nil {
			t.Fatalf("rep %d Reload: %v", i, err)
		}
		wantHash := sw.ToHash
		wantTotal := toTotal
		if i%2 == 1 {
			wantHash = sw.FromHash
			wantTotal = fromTotal
		}
		if got.Hash() != wantHash {
			t.Fatalf("rep %d hash = %d, want %d", i, got.Hash(), wantHash)
		}
		if m.TotalBytes() != wantTotal {
			t.Fatalf("rep %d total = %d, want %d", i, m.TotalBytes(), wantTotal)
		}
		if m.LiveCount("tex/red") != baseLive {
			t.Fatalf("rep %d live = %d, want %d", i, m.LiveCount("tex/red"), baseLive)
		}
	}
	if back, err := m.Wait("tex/red"); err != nil || !back.Equal(first) {
		t.Fatalf("after 100 swaps drifted: %v", err)
	}
	// 100 reps end on From (even i=0 holds To, odd i=99 holds From), so
	// the ledger must sit exactly on the From baseline, not drift.
	if m.TotalBytes() != fromTotal {
		t.Fatalf("after 100 swaps total = %d, want %d", m.TotalBytes(), fromTotal)
	}
	if m.LiveCount("tex/red") != baseLive {
		t.Fatalf("after 100 swaps live = %d, want %d", m.LiveCount("tex/red"), baseLive)
	}
	if st := w.Stats(); st.Reloads != 100 || st.Events != 100 {
		t.Fatalf("stats = %+v, want 100 reloads 100 events", st)
	}
	// Bad overwrites never poison the next good reload.
	badRaw, err := os.ReadFile(filepath.Join("testdata", c.BadFile.File))
	if err != nil {
		t.Fatalf("read bad: %v", err)
	}
	victim := filepath.Join(dir, "tex_red_4x4.ktx2")
	// tex/red currently holds the From bytes after the even loop above
	// (100 reps end on From), so snapshot them as the good baseline.
	goodBefore, err := m.Wait("tex/red")
	if err != nil {
		t.Fatalf("good Wait: %v", err)
	}
	if err := os.WriteFile(victim, badRaw, 0644); err != nil {
		t.Fatalf("write bad: %v", err)
	}
	touchFile(t, victim, time.Now().Add(2*time.Second))
	if _, err := w.Poll(); err != nil {
		t.Fatalf("bad Poll: %v", err)
	}
	if _, err := w.Reload("tex/red"); core.CodeOf(err) != hotCode(c.BadFile.WantCode) {
		t.Fatalf("bad Reload code = %v, want %v", core.CodeOf(err), c.BadFile.WantCode)
	}
	if kept, err := m.Wait("tex/red"); err != nil || !kept.Equal(goodBefore) {
		t.Fatalf("bad wiped art: %v", err)
	}
	rewriteHot(t, dir, "tex_red_4x4.ktx2", sw.From)
	if _, err := w.Poll(); err != nil {
		t.Fatalf("fix Poll: %v", err)
	}
	fixed, err := w.Reload("tex/red")
	if err != nil {
		t.Fatalf("good after bad: %v", err)
	}
	if !fixed.Equal(goodBefore) {
		t.Fatal("good after bad diverged")
	}
	if m.TotalBytes() != fromTotal || m.LiveCount("tex/red") != baseLive {
		t.Fatalf("after fix total/live = %d/%d, want %d/%d", m.TotalBytes(), m.LiveCount("tex/red"), fromTotal, baseLive)
	}
}

// F:离屏金对照窗(窗随P2建,先离屏对比加自动判意向):冻结数加形状断言.
func TestHotReloadOffscreenGolden(t *testing.T) {
	c := loadHotCases(t)
	if MaxWatches != c.MaxWatches || MaxWatchIDLen != c.MaxWatchIDLen ||
		MaxWatchPathLen != c.MaxWatchPath || PollInterval.Milliseconds() != c.PollIntervalMs {
		t.Fatalf("budgets diverge from hotreload_cases.json")
	}
	if c.Tolerance != 0 {
		t.Fatalf("tolerance = %d, want frozen 0", c.Tolerance)
	}
	if CurrentVersion.String() != c.Version {
		t.Fatalf("version = %v, want %v", CurrentVersion, c.Version)
	}
	dir := stageWatchDir(t, c)
	m := NewManager()
	w := NewWatcher(m)
	for _, want := range c.Watches {
		mustWatchHot(t, w, dir, want)
	}
	// Golden pins the anchors through files, not code.
	for _, want := range c.Watches {
		raw, err := os.ReadFile(filepath.Join("testdata", want.File))
		if err != nil {
			t.Fatalf("read %s: %v", want.File, err)
		}
		if len(raw) != want.Size || HashBytes(raw) != want.Hash {
			t.Fatalf("%s: file size/hash = %d/%d, want %d/%d", want.ID, len(raw), HashBytes(raw), want.Size, want.Hash)
		}
		got, err := m.Wait(core.AssetID(want.ID))
		if err != nil {
			t.Fatalf("%s: Wait: %v", want.ID, err)
		}
		checkHotWatch(t, want, got)
	}
	// Shape: ids distinct, kinds loadable, versions all current.
	seen := map[string]bool{}
	var total int64
	biggest := 0
	for _, want := range c.Watches {
		if seen[want.ID] {
			t.Errorf("id %q collides", want.ID)
		}
		seen[want.ID] = true
		if _, err := ParseKind(want.Kind); err != nil {
			t.Errorf("%s kind %q: %v", want.ID, want.Kind, err)
		}
		if want.Version != CurrentVersion.String() {
			t.Errorf("%s version %v, want current %v", want.ID, want.Version, CurrentVersion)
		}
		got, _ := m.Wait(core.AssetID(want.ID))
		if got.Size() <= 0 || got.Size() > MaxAssetBytes {
			t.Errorf("%s size %d out of 1..%d", want.ID, got.Size(), MaxAssetBytes)
		}
		if len(got.Deps()) > MaxDeps {
			t.Errorf("%s deps %d exceeds MaxDeps", want.ID, len(got.Deps()))
		}
		total += int64(got.Size())
		if got.Size() > biggest {
			biggest = got.Size()
		}
	}
	if total != m.TotalBytes() {
		t.Errorf("total %d != manager %d", total, m.TotalBytes())
	}
	// Shape: tex leaves carry pixels, raw carriers carry deps.
	for _, want := range c.Watches {
		got, _ := m.Wait(core.AssetID(want.ID))
		if want.Kind == "ktx2" && (got.UploadSize() <= 0 || got.PixelSize() <= 0) {
			t.Errorf("%s: upload/pixels = %d/%d, want > 0", want.ID, got.UploadSize(), got.PixelSize())
		}
		if want.Kind == "raw" && (got.UploadSize() != 0 || got.PixelSize() != 0) {
			t.Errorf("%s: raw upload/pixels = %d/%d, want 0/0", want.ID, got.UploadSize(), got.PixelSize())
		}
	}
	// Shape: dependency edges point at loaded tex leaves, never self.
	for _, want := range c.Watches {
		for _, d := range want.Deps {
			found := false
			for _, other := range c.Watches {
				if other.ID == d {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s: dep %q not in golden", want.ID, d)
			}
			if d == want.ID {
				t.Errorf("%s: self-dep", want.ID)
			}
		}
	}
	// Shape: swap endpoints differ (else the A-swap proves nothing) and
	// both land inside the frozen budgets.
	if c.Swap.FromHash == c.Swap.ToHash {
		t.Error("swap endpoints share one hash, A proves nothing")
	}
	swFrom, err := os.ReadFile(filepath.Join("testdata", c.Swap.From))
	if err != nil {
		t.Fatalf("read %s: %v", c.Swap.From, err)
	}
	swTo, err := os.ReadFile(filepath.Join("testdata", c.Swap.To))
	if err != nil {
		t.Fatalf("read %s: %v", c.Swap.To, err)
	}
	if HashBytes(swFrom) != c.Swap.FromHash || HashBytes(swTo) != c.Swap.ToHash {
		t.Fatal("swap hashes drifted from files")
	}
	if len(swFrom) > MaxAssetBytes || len(swTo) > MaxAssetBytes {
		t.Error("swap file exceeds MaxAssetBytes")
	}
	// Shape: bad and unfrozen stay out of the Ready set.
	badRaw, _ := os.ReadFile(filepath.Join("testdata", c.BadFile.File))
	if _, err := m.Load("golden/bad", mustParseHotKind(t, c.BadFile.Kind), CurrentVersion, badRaw, nil); err == nil {
		t.Errorf("%s want error", c.BadFile.File)
	}
	for _, name := range c.Unsupported {
		if _, err := ParseKind(name); core.CodeOf(err) != core.CodeUnsupported {
			t.Errorf("golden unsupported %q code = %v", name, core.CodeOf(err))
		}
	}
}

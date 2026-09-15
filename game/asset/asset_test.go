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

type assetCase struct {
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

type badFileCase struct {
	File     string `json:"file"`
	Kind     string `json:"kind"`
	WantCode string `json:"want_code"`
}

type casesFile struct {
	Version       string      `json:"version"`
	MaxAssets     int         `json:"max_assets"`
	MaxBytes      int         `json:"max_bytes"`
	MaxAssetBytes int         `json:"max_asset_bytes"`
	MaxDeps       int         `json:"max_deps"`
	MaxIDLen      int         `json:"max_id_len"`
	Assets        []assetCase `json:"assets"`
	Unsupported   []string    `json:"unsupported_kinds"`
	BadFiles      []badFileCase `json:"bad_files"`
}

func loadAssetCases(t *testing.T) casesFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "asset_cases.json"))
	if err != nil {
		t.Fatalf("read asset_cases.json: %v", err)
	}
	var c casesFile
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("decode asset_cases.json: %v", err)
	}
	if len(c.Assets) == 0 {
		t.Fatal("asset_cases.json has no assets")
	}
	if len(c.Unsupported) == 0 || len(c.BadFiles) == 0 {
		t.Fatal("asset_cases.json has no unsupported/bad files")
	}
	return c
}

func mustFindAsset(t *testing.T, c casesFile, id string) assetCase {
	t.Helper()
	for _, a := range c.Assets {
		if a.ID == id {
			return a
		}
	}
	t.Fatalf("asset_cases.json has no asset %q", id)
	return assetCase{}
}

func mustReadTestdata(t *testing.T, file string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", file))
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	return raw
}

func mustParseKind(t *testing.T, name string) Kind {
	t.Helper()
	k, err := ParseKind(name)
	if err != nil {
		t.Fatalf("ParseKind %q: %v", name, err)
	}
	return k
}

func mustParseVersion(t *testing.T, s string) core.Version {
	t.Helper()
	v, err := core.ParseVersion(s)
	if err != nil {
		t.Fatalf("ParseVersion %q: %v", s, err)
	}
	return v
}

func depsToIDs(deps []string) []core.AssetID {
	if len(deps) == 0 {
		return nil
	}
	out := make([]core.AssetID, len(deps))
	for i, d := range deps {
		out[i] = core.AssetID(d)
	}
	return out
}

func expectCode(t *testing.T, name string, err error, want core.Code) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: want error", name)
		return
	}
	if core.CodeOf(err) != want {
		t.Errorf("%s code = %v, want %v", name, core.CodeOf(err), want)
	}
}

func codeFromName(s string) core.Code {
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

func checkAsset(t *testing.T, want assetCase, got *Asset, raw []byte) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s: got nil asset", want.ID)
	}
	if got.ID() != core.AssetID(want.ID) {
		t.Errorf("%s: id = %q, want %q", want.ID, got.ID(), want.ID)
	}
	if got.Kind().String() != want.Kind {
		t.Errorf("%s: kind = %v, want %v", want.ID, got.Kind(), want.Kind)
	}
	if got.Version().String() != want.Version {
		t.Errorf("%s: version = %v, want %v", want.ID, got.Version(), want.Version)
	}
	if got.Hash() != want.Hash {
		t.Errorf("%s: hash = %d, want %d", want.ID, got.Hash(), want.Hash)
	}
	if got.Size() != want.Size || len(raw) != want.Size {
		t.Errorf("%s: size = %d/file %d, want %d", want.ID, got.Size(), len(raw), want.Size)
	}
	if got.UploadSize() != want.Upload {
		t.Errorf("%s: upload = %d, want %d", want.ID, got.UploadSize(), want.Upload)
	}
	if got.PixelSize() != want.Pixels {
		t.Errorf("%s: pixels = %d, want %d", want.ID, got.PixelSize(), want.Pixels)
	}
	gotDeps := got.Deps()
	if len(gotDeps) != len(want.Deps) {
		t.Errorf("%s: deps len = %d, want %d", want.ID, len(gotDeps), len(want.Deps))
	} else {
		for i, d := range want.Deps {
			if string(gotDeps[i]) != d {
				t.Errorf("%s: deps[%d] = %q, want %q", want.ID, i, gotDeps[i], d)
			}
		}
	}
	if got.State() != StateReady {
		t.Errorf("%s: state = %v, want ready", want.ID, got.State())
	}
	if len(got.Bytes()) != want.Size {
		t.Errorf("%s: bytes len = %d, want %d", want.ID, len(got.Bytes()), want.Size)
	}
}

func mustLoadFile(t *testing.T, m *Manager, want assetCase) *Asset {
	t.Helper()
	raw := mustReadTestdata(t, want.File)
	if HashBytes(raw) != want.Hash {
		t.Fatalf("%s: file hash = %d, want %d", want.ID, HashBytes(raw), want.Hash)
	}
	got, err := m.LoadFile(core.AssetID(want.ID), filepath.Join("testdata", want.File),
		mustParseKind(t, want.Kind), mustParseVersion(t, want.Version), depsToIDs(want.Deps))
	if err != nil {
		t.Fatalf("%s: LoadFile: %v", want.ID, err)
	}
	checkAsset(t, want, got, raw)
	return got
}

func mustWait(t *testing.T, m *Manager, id string) *Asset {
	t.Helper()
	got, err := m.Wait(core.AssetID(id))
	if err != nil {
		t.Fatalf("%s: Wait: %v", id, err)
	}
	return got
}

// A:异步引用计数依赖跟踪版本哈希全对.
func TestAssetLoadFromCases(t *testing.T) {
	c := loadAssetCases(t)
	if CurrentVersion.String() != c.Version {
		t.Fatalf("version = %v, want %v", CurrentVersion, c.Version)
	}
	if MaxAssets != c.MaxAssets || MaxBytes != c.MaxBytes ||
		MaxAssetBytes != c.MaxAssetBytes || MaxDeps != c.MaxDeps || MaxIDLen != c.MaxIDLen {
		t.Fatalf("limits diverge from asset_cases.json")
	}
	if KindRaw.String() != "raw" || KindTextureKTX2.String() != "ktx2" || KindUnknown.String() != "unknown" {
		t.Fatal("kind names drifted")
	}
	if !KindRaw.Valid() || !KindTextureKTX2.Valid() || KindUnknown.Valid() {
		t.Fatal("kind valid drifted")
	}
	m := NewManager()
	for _, want := range c.Assets {
		raw := mustReadTestdata(t, want.File)
		if len(raw) != want.Size {
			t.Errorf("%s: file size = %d, want %d", want.ID, len(raw), want.Size)
		}
		if HashBytes(raw) != want.Hash {
			t.Errorf("%s: hash = %d, want %d", want.ID, HashBytes(raw), want.Hash)
		}
		ver := mustParseVersion(t, want.Version)
		if !ver.CompatibleWith(CurrentVersion) {
			t.Errorf("%s: version %v not compatible with %v", want.ID, ver, CurrentVersion)
		}
		got := mustLoadFile(t, m, want)
		if m.LiveCount(core.AssetID(want.ID)) != 1 || !m.Loaded(core.AssetID(want.ID)) {
			t.Errorf("%s: live = %d loaded = %v, want 1/true", want.ID, m.LiveCount(core.AssetID(want.ID)), m.Loaded(core.AssetID(want.ID)))
		}
		if v, ok := m.VersionOf(core.AssetID(want.ID)); !ok || v != ver {
			t.Errorf("%s: VersionOf = %v/%v, want %v/true", want.ID, v, ok, ver)
		}
		if h, ok := m.HashOf(core.AssetID(want.ID)); !ok || h != want.Hash {
			t.Errorf("%s: HashOf = %d/%v, want %d/true", want.ID, h, ok, want.Hash)
		}
		_ = got
	}
	// Ref adds one claim; two Unloads drain Load plus Ref.
	red := mustFindAsset(t, c, "tex/red")
	if _, err := m.Ref(core.AssetID(red.ID)); err != nil {
		t.Fatalf("Ref tex/red: %v", red.ID)
	}
	if m.LiveCount(core.AssetID(red.ID)) != 2 {
		t.Fatalf("after Ref live = %d, want 2", m.LiveCount(core.AssetID(red.ID)))
	}
	if err := m.Unload(core.AssetID(red.ID)); err != nil {
		t.Fatalf("Unload1: %v", err)
	}
	if err := m.Unload(core.AssetID(red.ID)); err != nil {
		t.Fatalf("Unload2: %v", err)
	}
	if m.Loaded(core.AssetID(red.ID)) || m.LiveCount(core.AssetID(red.ID)) != 0 {
		t.Fatal("after full unload still loaded")
	}
	// Reload to keep the ledger balanced for later checks.
	mustLoadFile(t, m, red)
	// Dependencies pair with the golden; Dependents invert them sorted.
	for _, want := range c.Assets {
		got, ok := m.Dependencies(core.AssetID(want.ID))
		if !ok {
			t.Fatalf("%s: Dependencies ok=false", want.ID)
		}
		if len(got) != len(want.Deps) {
			t.Fatalf("%s: deps len = %d, want %d", want.ID, len(got), len(want.Deps))
		}
		for i, d := range want.Deps {
			if string(got[i]) != d {
				t.Fatalf("%s: deps[%d] = %q, want %q", want.ID, i, got[i], d)
			}
		}
	}
	redDeps := m.Dependents(core.AssetID("tex/red"))
	if len(redDeps) != 2 || redDeps[0] != core.AssetID("bone/slime") || redDeps[1] != core.AssetID("map/level1") {
		t.Errorf("dependents tex/red = %v, want [bone/slime map/level1]", redDeps)
	}
	checkerDeps := m.Dependents(core.AssetID("tex/checker"))
	if len(checkerDeps) != 1 || checkerDeps[0] != core.AssetID("map/level1") {
		t.Errorf("dependents tex/checker = %v, want [map/level1]", checkerDeps)
	}
	if got := m.Dependents(core.AssetID("map/level1")); len(got) != 0 {
		t.Errorf("dependents map/level1 = %v, want empty", got)
	}
	// Async chain lands on the same snapshots: Request plus Wait.
	am := NewManager()
	for _, want := range c.Assets {
		if err := am.RequestFile(core.AssetID(want.ID), filepath.Join("testdata", want.File),
			mustParseKind(t, want.Kind), mustParseVersion(t, want.Version), depsToIDs(want.Deps)); err != nil {
			t.Fatalf("%s: RequestFile: %v", want.ID, err)
		}
	}
	for _, want := range c.Assets {
		st, err := am.Poll(core.AssetID(want.ID))
		if err != nil && st != StateLoading && st != StateReady {
			t.Fatalf("%s: Poll = %v/%v", want.ID, st, err)
		}
		got := mustWait(t, am, want.ID)
		raw := mustReadTestdata(t, want.File)
		checkAsset(t, want, got, raw)
	}
	// In-memory Request covers the same path without files.
	bm := NewManager()
	for _, want := range c.Assets {
		raw := mustReadTestdata(t, want.File)
		if err := bm.Request(core.AssetID(want.ID), mustParseKind(t, want.Kind),
			mustParseVersion(t, want.Version), raw, depsToIDs(want.Deps)); err != nil {
			t.Fatalf("%s: Request: %v", want.ID, err)
		}
	}
	for _, want := range c.Assets {
		got := mustWait(t, bm, want.ID)
		raw := mustReadTestdata(t, want.File)
		checkAsset(t, want, got, raw)
	}
}

// B:空零超大坏数据缺资产全不崩不卡死,占位加报错.
func TestAssetEdgesNoCrash(t *testing.T) {
	c := loadAssetCases(t)
	var nilM *Manager
	if _, err := nilM.Load(core.AssetID("x"), KindRaw, CurrentVersion, []byte("x"), nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Load code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := nilM.Ref(core.AssetID("x")); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Ref code = %v, want invalid-arg", core.CodeOf(err))
	}
	if err := nilM.Unload(core.AssetID("x")); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Unload code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := nilM.Poll(core.AssetID("x")); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Poll code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := nilM.Wait(core.AssetID("x")); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Wait code = %v, want invalid-arg", core.CodeOf(err))
	}
	if nilM.Placeholder(core.AssetID("x")) != nil {
		t.Error("nil Placeholder want nil")
	}
	if nilM.Stats() != (Stats{}) || nilM.Count() != 0 || nilM.TotalBytes() != 0 {
		t.Error("nil stats want zero")
	}
	m := NewManager()
	if _, err := m.Load("", KindRaw, CurrentVersion, []byte("x"), nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty id Load code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := m.Load(core.AssetID("x"), KindUnknown, CurrentVersion, []byte("x"), nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("unknown kind Load code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := m.Load(core.AssetID("x"), KindRaw, CurrentVersion, nil, nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil data Load code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := m.LoadFile(core.AssetID("x"), "", KindRaw, CurrentVersion, nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty path LoadFile code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := m.Poll(""); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty Poll code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := m.Wait(""); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty Wait code = %v, want invalid-arg", core.CodeOf(err))
	}
	if err := m.Request("", KindRaw, CurrentVersion, []byte("x"), nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty Request code = %v, want invalid-arg", core.CodeOf(err))
	}
	if err := m.Request(core.AssetID("x"), KindRaw, CurrentVersion, nil, nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Request data code = %v, want invalid-arg", core.CodeOf(err))
	}
	// Unfrozen kinds name the kind then stop with Unsupported.
	if _, err := ParseKind(""); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty ParseKind code = %v, want invalid-arg", core.CodeOf(err))
	}
	for _, name := range c.Unsupported {
		if _, err := ParseKind(name); core.CodeOf(err) != core.CodeUnsupported {
			t.Errorf("ParseKind %q code = %v, want unsupported", name, core.CodeOf(err))
		}
	}
	// Missing file yields a Missing placeholder, never nil, never panic.
	miss, err := m.LoadFile(core.AssetID("tex/ghost"), filepath.Join("testdata", "no_such.ktx2"), KindTextureKTX2, CurrentVersion, nil)
	expectCode(t, "missing file", err, core.CodeNotFound)
	if miss == nil || miss.State() != StateMissing || miss.Size() != 0 || len(miss.Bytes()) != 0 {
		t.Errorf("missing placeholder = %+v, want missing empty", miss)
	}
	if st, perr := m.Poll(core.AssetID("tex/ghost")); st != StateMissing || core.CodeOf(perr) != core.CodeNotFound {
		t.Errorf("missing Poll = %v/%v, want missing/not-found", st, perr)
	}
	if _, werr := m.Wait(core.AssetID("tex/ghost")); core.CodeOf(werr) != core.CodeNotFound {
		t.Errorf("missing Wait code = %v, want not-found", core.CodeOf(werr))
	}
	if _, rerr := m.Ref(core.AssetID("tex/ghost")); core.CodeOf(rerr) != core.CodeNotFound {
		t.Errorf("missing Ref code = %v, want not-found", core.CodeOf(rerr))
	}
	// Frozen bad files fail with their frozen codes and stay queryable.
	for _, b := range c.BadFiles {
		raw := mustReadTestdata(t, b.File)
		got, berr := m.Load(core.AssetID("bad/"+b.File), mustParseKind(t, b.Kind), CurrentVersion, raw, nil)
		expectCode(t, b.File, berr, codeFromName(b.WantCode))
		if got == nil || got.State() != StateFailed {
			t.Errorf("%s: state = %v, want failed", b.File, got.State())
		}
		if st, perr := m.Poll(core.AssetID("bad/" + b.File)); st != StateFailed || core.CodeOf(perr) != codeFromName(b.WantCode) {
			t.Errorf("%s: Poll = %v/%v, want failed/%v", b.File, st, perr, b.WantCode)
		}
	}
	// Version gaps never load: Failed placeholder plus VersionMismatch.
	raw := mustReadTestdata(t, "tex_red_4x4.ktx2")
	future := core.Version{Major: CurrentVersion.Major + 1, Minor: 0}
	if got, verr := m.Load(core.AssetID("tex/future"), KindTextureKTX2, future, raw, nil); core.CodeOf(verr) != core.CodeVersionMismatch {
		t.Errorf("future version code = %v, want version-mismatch", core.CodeOf(verr))
	} else if got.State() != StateFailed {
		t.Errorf("future state = %v, want failed", got.State())
	}
	// Bad values never store: overlong, self-dep, too many deps, huge.
	longID := core.AssetID(strings.Repeat("x", MaxIDLen+1))
	if _, err := m.Load(longID, KindRaw, CurrentVersion, []byte("x"), nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("long id code = %v, want invalid-arg", core.CodeOf(err))
	}
	self := core.AssetID("self/loop")
	if _, err := m.Load(self, KindRaw, CurrentVersion, []byte("x"), []core.AssetID{self}); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("self dep code = %v, want invalid-arg", core.CodeOf(err))
	}
	many := make([]core.AssetID, MaxDeps+1)
	for i := range many {
		many[i] = core.AssetID("dep/" + string(rune('a'+i%26)) + string(rune('0'+i%10)))
	}
	if _, err := m.Load(core.AssetID("many/deps"), KindRaw, CurrentVersion, []byte("x"), many); core.CodeOf(err) != core.CodeOutOfMemory {
		t.Errorf("many deps code = %v, want out-of-memory", core.CodeOf(err))
	}
	huge := make([]byte, MaxAssetBytes+1)
	if _, err := m.Load(core.AssetID("huge/blob"), KindRaw, CurrentVersion, huge, nil); core.CodeOf(err) != core.CodeOutOfMemory {
		t.Errorf("huge code = %v, want out-of-memory", core.CodeOf(err))
	}
	// Over-release and live-evict fail closed without touching counts.
	one := core.AssetID("edge/one")
	if _, err := m.Load(one, KindRaw, CurrentVersion, []byte("one"), nil); err != nil {
		t.Fatalf("edge load: %v", err)
	}
	if err := m.Unload(one); err != nil {
		t.Fatalf("edge unload: %v", err)
	}
	if err := m.Unload(one); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("over-release code = %v, want invalid-arg", core.CodeOf(err))
	}
	if err := m.Unload(core.AssetID("edge/never")); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("unknown unload code = %v, want not-found", core.CodeOf(err))
	}
	if _, err := m.Ref(core.AssetID("edge/never")); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("unknown ref code = %v, want not-found", core.CodeOf(err))
	}
	live := core.AssetID("edge/live")
	if _, err := m.Load(live, KindRaw, CurrentVersion, []byte("live"), nil); err != nil {
		t.Fatalf("live load: %v", err)
	}
	if err := m.Evict(live); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("evict live code = %v, want invalid-arg", core.CodeOf(err))
	}
	if err := m.Evict(core.AssetID("edge/never")); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("evict unknown code = %v, want not-found", core.CodeOf(err))
	}
	// Nil snapshots never panic.
	var nilA *Asset
	if nilA.ID() != "" || nilA.Kind() != KindUnknown || nilA.Hash() != 0 || nilA.Size() != 0 {
		t.Error("nil asset zeros drifted")
	}
	if nilA.State() != StateEmpty || len(nilA.Bytes()) != 0 || len(nilA.Deps()) != 0 {
		t.Error("nil asset state/bytes/deps drifted")
	}
	if nilA.UploadSize() != 0 || nilA.PixelSize() != 0 {
		t.Error("nil asset upload/pixels drifted")
	}
	if !nilA.Equal(nil) {
		t.Error("nil Equal(nil) = false, want true")
	}
	good := mustLoadFile(t, NewManager(), mustFindAsset(t, c, "tex/red"))
	if nilA.Equal(good) || !good.Equal(good) {
		t.Error("nil/good Equal wrong")
	}
}

// C不适用画画(纯管线):数路无损加逐位重放即两边同数.
func TestAssetBoundaryIdentical(t *testing.T) {
	c := loadAssetCases(t)
	for _, want := range c.Assets {
		raw := mustReadTestdata(t, want.File)
		input := append([]byte(nil), raw...)
		m := NewManager()
		first, err := m.LoadFile(core.AssetID(want.ID), filepath.Join("testdata", want.File),
			mustParseKind(t, want.Kind), mustParseVersion(t, want.Version), depsToIDs(want.Deps))
		if err != nil {
			t.Fatalf("%s: LoadFile: %v", want.ID, err)
		}
		// Input bytes cross the boundary intact.
		if len(input) != len(raw) {
			t.Fatalf("%s: input aliased", want.ID)
		}
		for i := range input {
			if input[i] != raw[i] {
				t.Fatalf("%s: input byte %d mutated", want.ID, i)
			}
		}
		// Same file parses to the same snapshot twice.
		second, err := m.Load(core.AssetID(want.ID), mustParseKind(t, want.Kind),
			mustParseVersion(t, want.Version), raw, depsToIDs(want.Deps))
		if err != nil {
			t.Fatalf("%s: second Load: %v", want.ID, err)
		}
		// Reload overwrites with identical bytes: snapshots stay equal
		// apart from the extra claim, so compare payload fields.
		if !first.Equal(second) {
			// Load acquires a new claim but the snapshot payload must match;
			// Equal covers payload only, so any drift here is real.
			t.Errorf("%s: reload diverged", want.ID)
		}
		other := NewManager()
		again, err := other.Load(core.AssetID(want.ID), mustParseKind(t, want.Kind),
			mustParseVersion(t, want.Version), raw, depsToIDs(want.Deps))
		if err != nil {
			t.Fatalf("%s: other Load: %v", want.ID, err)
		}
		// Cross-manager replay is bitwise identical.
		if !first.Equal(again) {
			t.Errorf("%s: cross-manager replay diverged", want.ID)
		}
		// Version boundary round-trips through its string form.
		if back, err := core.ParseVersion(first.Version().String()); err != nil || back != first.Version() {
			t.Errorf("%s: version boundary = %v/%v", want.ID, back, err)
		}
		// Hash boundary is deterministic.
		if HashBytes(raw) != first.Hash() || HashBytes(raw) != want.Hash {
			t.Errorf("%s: hash boundary drifted", want.ID)
		}
	}
	// Copies never alias: mutating a return cannot corrupt the replay.
	m := NewManager()
	want := mustFindAsset(t, c, "map/level1")
	first := mustLoadFile(t, m, want)
	probe := first.Bytes()
	if len(probe) == 0 {
		t.Fatal("map bytes empty, golden invalid")
	}
	probe[0] ^= 0xFF
	if first.Bytes()[0] == probe[0] {
		t.Fatal("Bytes aliases the asset")
	}
	deps := first.Deps()
	if len(deps) == 0 {
		t.Fatal("map deps empty, golden invalid")
	}
	deps[0] = core.AssetID("hacked/dep")
	if again, _ := m.Dependencies(first.ID()); len(again) > 0 && again[0] == core.AssetID("hacked/dep") {
		t.Fatal("Deps aliases the asset")
	}
	if !first.Equal(mustWait(t, m, want.ID)) {
		t.Fatal("copy probe corrupted the asset")
	}
	// Dependents order is stable across calls.
	a := m.Dependents(core.AssetID("tex/red"))
	// m holds map/level1 which needs tex/red, so the dependent shows
	// even though tex/red itself is not cached here.
	if len(a) != 1 || a[0] != core.AssetID("map/level1") {
		t.Fatalf("single-asset dependents = %v, want [map/level1]", a)
	}
	full := NewManager()
	for _, w := range c.Assets {
		mustLoadFile(t, full, w)
	}
	firstOrder := full.Dependents(core.AssetID("tex/red"))
	secondOrder := full.Dependents(core.AssetID("tex/red"))
	if len(firstOrder) != len(secondOrder) {
		t.Fatal("dependents order unstable")
	}
	for i := range firstOrder {
		if firstOrder[i] != secondOrder[i] {
			t.Fatal("dependents order unstable")
		}
	}
}

// D:百资产跑得动,数量内存时长有数.
func TestAssetPerfMany(t *testing.T) {
	c := loadAssetCases(t)
	want := mustFindAsset(t, c, "map/level1")
	template := mustReadTestdata(t, want.File)
	// Synthetic load only (no golden): golden stays in asset_cases.json.
	m := NewManager()
	const n = 100
	start := time.Now()
	for i := 0; i < n; i++ {
		id := core.AssetID("perf/" + itoaAsset(i))
		if _, err := m.Load(id, KindRaw, CurrentVersion, template, nil); err != nil {
			t.Fatalf("perf %d: %v", i, err)
		}
	}
	el := time.Since(start)
	st := m.Stats()
	t.Logf("asset-many: %d loads %dB total in %v (%.1f us/load)", n, st.TotalBytes, el, float64(el.Microseconds())/n)
	if st.Count != n {
		t.Fatalf("count = %d, want %d", st.Count, n)
	}
	if st.TotalBytes != int64(n*len(template)) {
		t.Fatalf("total = %d, want %d", st.TotalBytes, n*len(template))
	}
	if st.Loads != n {
		t.Fatalf("loads = %d, want %d", st.Loads, n)
	}
	if st.TotalBytes > MaxBytes {
		t.Fatalf("total %d exceeds MaxBytes %d", st.TotalBytes, MaxBytes)
	}
	for i := 0; i < n; i++ {
		id := core.AssetID("perf/" + itoaAsset(i))
		if !m.Loaded(id) || m.LiveCount(id) != 1 {
			t.Fatalf("perf %d live = %d loaded = %v", i, m.LiveCount(id), m.Loaded(id))
		}
	}
}

func itoaAsset(i int) string {
	const digits = "0123456789"
	if i == 0 {
		return "000"
	}
	var out [16]byte
	p := len(out)
	n := i
	for n > 0 {
		p--
		out[p] = digits[n%10]
		n /= 10
	}
	// Pad to 3 digits for stable ids.
	for p > len(out)-3 {
		p--
		out[p] = '0'
	}
	return string(out[p:])
}

// E:反复加载长跑不涨不漂,坏档不粘.
func TestAssetLongRunStable(t *testing.T) {
	c := loadAssetCases(t)
	want := mustFindAsset(t, c, "tex/red")
	raw := mustReadTestdata(t, want.File)
	m := NewManager()
	first := mustLoadFile(t, m, want)
	firstBytes := m.TotalBytes()
	for i := 0; i < 1000; i++ {
		got, err := m.Load(core.AssetID(want.ID), mustParseKind(t, want.Kind),
			mustParseVersion(t, want.Version), raw, depsToIDs(want.Deps))
		if err != nil {
			t.Fatalf("rep %d: %v", i, err)
		}
		if !got.Equal(first) {
			// Snapshots carry identical payloads; claims differ but Equal
			// covers payload only, so drift here is real.
			t.Fatalf("rep %d diverged", i)
		}
		// Drain the extra claim from this rep so the ledger does not grow.
		if err := m.Unload(core.AssetID(want.ID)); err != nil {
			t.Fatalf("rep %d unload: %v", i, err)
		}
	}
	if m.TotalBytes() != firstBytes {
		t.Fatalf("total = %d, want stable %d", m.TotalBytes(), firstBytes)
	}
	if again := mustWait(t, m, want.ID); !again.Equal(first) {
		t.Fatal("wait after long run diverged")
	}
	// Ref/Unload walk returns to the start.
	base := m.LiveCount(core.AssetID(want.ID))
	refed, err := m.Ref(core.AssetID(want.ID))
	if err != nil {
		t.Fatalf("walk ref: %v", err)
	}
	if m.LiveCount(core.AssetID(want.ID)) != base+1 {
		t.Fatalf("walk live = %d, want %d", m.LiveCount(core.AssetID(want.ID)), base+1)
	}
	if !refed.Equal(first) {
		t.Fatal("walk ref diverged")
	}
	if err := m.Unload(core.AssetID(want.ID)); err != nil {
		t.Fatalf("walk unload: %v", err)
	}
	if m.LiveCount(core.AssetID(want.ID)) != base {
		t.Fatalf("walk back = %d, want %d", m.LiveCount(core.AssetID(want.ID)), base)
	}
	// Bad data never poisons the next good load.
	for _, b := range c.BadFiles {
		bad := mustReadTestdata(t, b.File)
		if _, err := m.Load(core.AssetID("stable/bad"), mustParseKind(t, b.Kind), CurrentVersion, bad, nil); err == nil {
			t.Errorf("%s want error", b.File)
		}
	}
	if again, err := m.Load(core.AssetID("stable/bad"), KindRaw, CurrentVersion, []byte("good"), nil); err != nil {
		t.Fatalf("good after bad: %v", err)
	} else if again.State() != StateReady {
		t.Fatal("good after bad not ready")
	}
	if again := mustWait(t, m, want.ID); !again.Equal(first) {
		t.Fatal("good asset drifted after bad files")
	}
	// Clean the probe so the file round-trip starts from the first total.
	if err := m.Unload(core.AssetID("stable/bad")); err != nil {
		t.Fatalf("stable unload: %v", err)
	}
	if err := m.Evict(core.AssetID("stable/bad")); err != nil {
		t.Fatalf("stable evict: %v", err)
	}
	if m.TotalBytes() != firstBytes {
		t.Fatalf("after evict total = %d, want %d", m.TotalBytes(), firstBytes)
	}
	// File round-trips stay byte-stable.
	for i := 0; i < 200; i++ {
		got, err := m.LoadFile(core.AssetID(want.ID), filepath.Join("testdata", want.File),
			mustParseKind(t, want.Kind), mustParseVersion(t, want.Version), depsToIDs(want.Deps))
		if err != nil {
			t.Fatalf("file rep %d: %v", i, err)
		}
		if !got.Equal(first) {
			t.Fatalf("file rep %d diverged", i)
		}
		if err := m.Unload(core.AssetID(want.ID)); err != nil {
			t.Fatalf("file rep %d unload: %v", i, err)
		}
	}
	if m.TotalBytes() != firstBytes {
		t.Fatalf("file total = %d, want stable %d", m.TotalBytes(), firstBytes)
	}
}

// F:离屏金对照窗(窗免,纯管线):冻结数加形状断言,账即证据.
func TestAssetOffscreenGolden(t *testing.T) {
	c := loadAssetCases(t)
	m := NewManager()
	for _, want := range c.Assets {
		mustLoadFile(t, m, want)
	}
	// Golden pins the anchors through files, not code.
	for _, want := range c.Assets {
		raw := mustReadTestdata(t, want.File)
		got, err := m.Wait(core.AssetID(want.ID))
		if err != nil {
			t.Fatalf("%s: Wait: %v", want.ID, err)
		}
		checkAsset(t, want, got, raw)
	}
	// Shape: ids distinct, kinds loadable, versions all current.
	seen := map[string]bool{}
	for _, want := range c.Assets {
		if seen[want.ID] {
			t.Errorf("id %q collides", want.ID)
		}
		seen[want.ID] = true
		if want.Version != CurrentVersion.String() {
			t.Errorf("%s version %v, want current %v", want.ID, want.Version, CurrentVersion)
		}
		if _, err := ParseKind(want.Kind); err != nil {
			t.Errorf("%s kind %q: %v", want.ID, want.Kind, err)
		}
	}
	// Shape: tex leaves carry pixels, raw carriers carry deps.
	for _, want := range c.Assets {
		got := mustWait(t, m, want.ID)
		if want.Kind == "ktx2" && (got.UploadSize() <= 0 || got.PixelSize() <= 0) {
			t.Errorf("%s: upload/pixels = %d/%d, want > 0", want.ID, got.UploadSize(), got.PixelSize())
		}
		if want.Kind == "raw" && (got.UploadSize() != 0 || got.PixelSize() != 0) {
			t.Errorf("%s: raw upload/pixels = %d/%d, want 0/0", want.ID, got.UploadSize(), got.PixelSize())
		}
		if len(want.Deps) == 0 && len(got.Deps()) != 0 {
			t.Errorf("%s: deps = %v, want empty", want.ID, got.Deps())
		}
	}
	// Shape: dependency edges point at loaded tex leaves, never self.
	for _, want := range c.Assets {
		for _, d := range want.Deps {
			found := false
			for _, other := range c.Assets {
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
			if _, ok := m.VersionOf(core.AssetID(d)); !ok {
				t.Errorf("%s: dep %q not ready", want.ID, d)
			}
		}
	}
	// Shape: every stored number stays inside its frozen budget.
	var total int64
	hashes := map[uint64]string{}
	for _, want := range c.Assets {
		got := mustWait(t, m, want.ID)
		if got.Size() <= 0 || got.Size() > MaxAssetBytes {
			t.Errorf("%s size %d out of 1..%d", want.ID, got.Size(), MaxAssetBytes)
		}
		if len(got.Deps()) > MaxDeps {
			t.Errorf("%s deps %d exceeds MaxDeps", want.ID, len(got.Deps()))
		}
		total += int64(got.Size())
		if prev, dup := hashes[got.Hash()]; dup {
			t.Errorf("%s hash %d dup with %s", want.ID, got.Hash(), prev)
		}
		hashes[got.Hash()] = want.ID
	}
	if total != m.TotalBytes() {
		t.Errorf("total %d != manager %d", total, m.TotalBytes())
	}
	if m.Count() != len(c.Assets) {
		t.Errorf("count = %d, want %d", m.Count(), len(c.Assets))
	}
	// Shape: largest frozen payload still far under budget, so headroom is measured.
	biggest := 0
	for _, want := range c.Assets {
		if want.Size > biggest {
			biggest = want.Size
		}
	}
	if biggest <= 0 || biggest > MaxAssetBytes {
		t.Errorf("biggest %d out of budget", biggest)
	}
	if total > MaxBytes {
		t.Errorf("total %d exceeds MaxBytes", total)
	}
	// Shape: bad and unfrozen stay out of the Ready set.
	for _, b := range c.BadFiles {
		raw := mustReadTestdata(t, b.File)
		if _, err := m.Load(core.AssetID("golden/"+b.File), mustParseKind(t, b.Kind), CurrentVersion, raw, nil); err == nil {
			t.Errorf("%s want error", b.File)
		}
	}
	for _, name := range c.Unsupported {
		if _, err := ParseKind(name); core.CodeOf(err) != core.CodeUnsupported {
			t.Errorf("golden unsupported %q code = %v", name, core.CodeOf(err))
		}
	}
}

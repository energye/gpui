package asset

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/energye/gpui/engine/core"
)

type s70MemberJSON struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`
	File    string   `json:"file"`
	Version string   `json:"version"`
	Hash    uint64   `json:"hash"`
	Size    int      `json:"size"`
	Deps    []string `json:"deps"`
}

type s70SetJSON struct {
	Name    string          `json:"name"`
	Members []s70MemberJSON `json:"members"`
}

type s70Cases struct {
	Sets []s70SetJSON `json:"sets"`
	Swap struct {
		Set      string `json:"set"`
		ID       string `json:"id"`
		From     string `json:"from"`
		To       string `json:"to"`
		FromHash uint64 `json:"from_hash"`
		ToHash   uint64 `json:"to_hash"`
	} `json:"swap"`
	Bad struct {
		Set      string `json:"set"`
		ID       string `json:"id"`
		File     string `json:"file"`
		Kind     string `json:"kind"`
		WantCode string `json:"want_code"`
	} `json:"bad"`
	MissingID struct {
		Set      string `json:"set"`
		ID       string `json:"id"`
		WantCode string `json:"want_code"`
	} `json:"missing_id"`
	MissingFile string `json:"missing_file"`
	Logic       struct {
		Set      string `json:"set"`
		ID       string `json:"id"`
		Kind     string `json:"kind"`
		WantCode string `json:"want_code"`
	} `json:"logic"`
}

func loadS70Cases(t *testing.T) s70Cases {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "s70_reload_cases.json"))
	if err != nil {
		t.Fatalf("read s70_reload_cases.json: %v", err)
	}
	var c s70Cases
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("decode s70_reload_cases.json: %v", err)
	}
	if len(c.Sets) == 0 {
		t.Fatal("s70 cases have no sets")
	}
	return c
}

func s70Kind(t *testing.T, name string) Kind {
	t.Helper()
	k, err := ParseKind(name)
	if err != nil {
		t.Fatalf("ParseKind %q: %v", name, err)
	}
	return k
}

func s70Ver(t *testing.T, s string) core.Version {
	t.Helper()
	v, err := core.ParseVersion(s)
	if err != nil {
		t.Fatalf("ParseVersion %q: %v", s, err)
	}
	return v
}

func s70IDs(deps []string) []core.AssetID {
	if len(deps) == 0 {
		return nil
	}
	out := make([]core.AssetID, len(deps))
	for i, d := range deps {
		out[i] = core.AssetID(d)
	}
	return out
}

func s70Code(s string) core.Code {
	switch s {
	case "not-found":
		return core.CodeNotFound
	case "bad-data":
		return core.CodeBadData
	case "invalid-arg":
		return core.CodeInvalidArg
	default:
		return core.CodeUnknown
	}
}

func s70Read(t *testing.T, file string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", file))
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	return raw
}

func mustRegisterS70(t *testing.T, r *SetReloader, c s70Cases) {
	t.Helper()
	for _, set := range c.Sets {
		members := make([]SetMember, len(set.Members))
		for i, m := range set.Members {
			members[i] = SetMember{ID: core.AssetID(m.ID), Kind: s70Kind(t, m.Kind), Ver: s70Ver(t, m.Version), Deps: s70IDs(m.Deps)}
		}
		if err := r.RegisterSet(set.Name, members); err != nil {
			t.Fatalf("RegisterSet %q: %v", set.Name, err)
		}
	}
}

func mustReloadS70Member(t *testing.T, r *SetReloader, set string, m s70MemberJSON) {
	t.Helper()
	raw := s70Read(t, m.File)
	if HashBytes(raw) != m.Hash || len(raw) != m.Size {
		t.Fatalf("%s: file hash/size = %d/%d, want %d/%d", m.ID, HashBytes(raw), len(raw), m.Hash, m.Size)
	}
	got, err := r.ReloadOne(set, core.AssetID(m.ID), s70Kind(t, m.Kind), s70Ver(t, m.Version), raw, s70IDs(m.Deps))
	if err != nil {
		t.Fatalf("%s: ReloadOne: %v", m.ID, err)
	}
	if got.Hash() != m.Hash {
		t.Fatalf("%s: hash = %d, want %d", m.ID, got.Hash(), m.Hash)
	}
}

func mustLoadAllS70(t *testing.T, r *SetReloader, c s70Cases) {
	t.Helper()
	mustRegisterS70(t, r, c)
	for _, set := range c.Sets {
		for _, m := range set.Members {
			mustReloadS70Member(t, r, set.Name, m)
		}
	}
}

func s70FindMember(t *testing.T, c s70Cases, set, id string) s70MemberJSON {
	t.Helper()
	for _, s := range c.Sets {
		if s.Name != set {
			continue
		}
		for _, m := range s.Members {
			if m.ID == id {
				return m
			}
		}
	}
	t.Fatalf("s70 cases have no member %s/%s", set, id)
	return s70MemberJSON{}
}

// A:改一张只重载该集,其他集不动.
func TestS70SetReloadIsolated(t *testing.T) {
	c := loadS70Cases(t)
	m := NewManager()
	r := NewSetReloader(m)
	mustLoadAllS70(t, r, c)
	if got := r.Sets(); len(got) != 2 || got[0] != "map" || got[1] != "ui" {
		t.Fatalf("sets = %v, want [map ui]", got)
	}
	if set, ok := r.SetOf("tex/red"); !ok || set != "ui" {
		t.Fatalf("SetOf tex/red = %q/%v, want ui/true", set, ok)
	}
	sw := c.Swap
	before, err := m.Wait("map/level1")
	if err != nil {
		t.Fatalf("neighbour Wait: %v", err)
	}
	toRaw := s70Read(t, sw.To)
	got, err := r.ReloadOne(sw.Set, core.AssetID(sw.ID), s70Kind(t, "ktx2"), s70Ver(t, "1.0"), toRaw, nil)
	if err != nil {
		t.Fatalf("ReloadOne swap: %v", err)
	}
	if got.Hash() != sw.ToHash {
		t.Fatalf("hash = %d, want %d", got.Hash(), sw.ToHash)
	}
	after, err := m.Wait("map/level1")
	if err != nil {
		t.Fatalf("neighbour re-Wait: %v", err)
	}
	if !before.Equal(after) {
		t.Fatal("neighbour moved during the swap, want only one set reloaded")
	}
	found := false
	for _, l := range r.Logs() {
		if l.Set == sw.Set && l.ID == core.AssetID(sw.ID) && l.OK {
			found = true
		}
	}
	if !found {
		t.Fatalf("logs miss %s/%s ok entry", sw.Set, sw.ID)
	}
	if len(r.Reports()) != 0 {
		t.Fatalf("reports = %d, want 0 on clean swap", len(r.Reports()))
	}
}

// B:缺图坏图错码对,不断链不崩.
func TestS70BrokenLinkCodes(t *testing.T) {
	c := loadS70Cases(t)
	m := NewManager()
	r := NewSetReloader(m)
	mustLoadAllS70(t, r, c)
	good, err := m.Wait(core.AssetID(c.Swap.ID))
	if err != nil {
		t.Fatalf("good Wait: %v", err)
	}
	ver := s70Ver(t, "1.0")
	if _, err := r.ReloadOne(c.Bad.Set, core.AssetID(c.Bad.ID), s70Kind(t, c.Bad.Kind), ver, nil, nil); core.CodeOf(err) != core.CodeNotFound {
		t.Fatalf("empty reload code = %v, want not-found", core.CodeOf(err))
	}
	if kept, err := m.Wait(core.AssetID(c.Bad.ID)); err != nil || !kept.Equal(good) {
		t.Fatalf("empty wiped art: %v", err)
	}
	badRaw := s70Read(t, c.Bad.File)
	if _, err := r.ReloadOne(c.Bad.Set, core.AssetID(c.Bad.ID), s70Kind(t, c.Bad.Kind), ver, badRaw, nil); core.CodeOf(err) != s70Code(c.Bad.WantCode) {
		t.Fatalf("bad reload code = %v, want %v", core.CodeOf(err), c.Bad.WantCode)
	}
	if kept, err := m.Wait(core.AssetID(c.Bad.ID)); err != nil || !kept.Equal(good) {
		t.Fatalf("bad wiped art: %v", err)
	}
	missPath := filepath.Join(t.TempDir(), c.MissingFile)
	if _, err := r.ReloadOneFile(c.Bad.Set, core.AssetID(c.Bad.ID), missPath, s70Kind(t, c.Bad.Kind), ver, nil); core.CodeOf(err) != core.CodeNotFound {
		t.Fatalf("missing file code = %v, want not-found", core.CodeOf(err))
	}
	if kept, err := m.Wait(core.AssetID(c.Bad.ID)); err != nil || !kept.Equal(good) {
		t.Fatalf("missing wiped art: %v", err)
	}
	if _, err := r.ReloadOne(c.MissingID.Set, core.AssetID(c.MissingID.ID), s70Kind(t, "ktx2"), ver, []byte("x"), nil); core.CodeOf(err) != s70Code(c.MissingID.WantCode) {
		t.Fatalf("unknown id code = %v, want %v", core.CodeOf(err), c.MissingID.WantCode)
	}
	if _, err := r.ReloadOne("nope/set", "tex/red", s70Kind(t, "ktx2"), ver, []byte("x"), nil); core.CodeOf(err) != core.CodeNotFound {
		t.Fatalf("unknown set code = %v, want not-found", core.CodeOf(err))
	}
	if got := r.VerifyLinks("ui"); len(got) != 0 {
		t.Fatalf("clean VerifyLinks = %d, want 0", len(got))
	}
	onlyMap := NewManager()
	onlyR := NewSetReloader(onlyMap)
	mustRegisterS70(t, onlyR, c)
	lvl := s70FindMember(t, c, "map", "map/level1")
	mustReloadS70Member(t, onlyR, "map", lvl)
	found := onlyR.VerifyLinks("map")
	if len(found) != 2 {
		t.Fatalf("broken deps = %d, want 2", len(found))
	}
	for _, b := range found {
		if b.Code != core.CodeNotFound || b.Need.Empty() {
			t.Fatalf("broken entry = %+v, want not-found plus need", b)
		}
	}
	var nilR *SetReloader
	if _, err := nilR.ReloadOne("ui", "tex/red", s70Kind(t, "ktx2"), ver, []byte("x"), nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Fatalf("nil reload code = %v, want invalid-arg", core.CodeOf(err))
	}
	if nilR.Logs() != nil || nilR.Reports() != nil || nilR.Sets() != nil || nilR.Members("ui") != nil {
		t.Fatal("nil queries want nil")
	}
	if nilR.VerifyLinks("ui") != nil {
		t.Fatal("nil verify want nil")
	}
}

// C:上报带集名图名,日志位可查.
func TestS70ReportHasSet(t *testing.T) {
	c := loadS70Cases(t)
	m := NewManager()
	r := NewSetReloader(m)
	mustLoadAllS70(t, r, c)
	ver := s70Ver(t, "1.0")
	badRaw := s70Read(t, c.Bad.File)
	_, badErr := r.ReloadOne(c.Bad.Set, core.AssetID(c.Bad.ID), s70Kind(t, c.Bad.Kind), ver, badRaw, nil)
	if badErr == nil {
		t.Fatal("bad reload want error")
	}
	if !strings.Contains(badErr.Error(), c.Bad.Set) || !strings.Contains(badErr.Error(), c.Bad.ID) {
		t.Fatalf("bad error %q misses set/id %s/%s", badErr.Error(), c.Bad.Set, c.Bad.ID)
	}
	missPath := filepath.Join(t.TempDir(), c.MissingFile)
	_, missErr := r.ReloadOneFile(c.Bad.Set, core.AssetID(c.Bad.ID), missPath, s70Kind(t, c.Bad.Kind), ver, nil)
	if !strings.Contains(missErr.Error(), c.Bad.Set) || !strings.Contains(missErr.Error(), c.Bad.ID) {
		t.Fatalf("missing error %q misses set/id", missErr.Error())
	}
	reps := r.Reports()
	if len(reps) == 0 {
		t.Fatal("reports empty, want broken links logged")
	}
	for _, b := range reps {
		if b.Set != c.Bad.Set {
			t.Fatalf("report set = %q, want %q", b.Set, c.Bad.Set)
		}
		if strings.Contains(BrokenLink{Set: b.Set, ID: b.ID, Need: b.Need, Code: b.Code, Cause: b.Cause}.Error(), b.Set) == false {
			t.Fatalf("report error misses set: %+v", b)
		}
	}
	logs := r.Logs()
	if len(logs) == 0 {
		t.Fatal("logs empty, want reload slot")
	}
	seen := false
	for _, l := range logs {
		if l.Set == "" || l.ID.Empty() {
			t.Fatalf("log misses set/id: %+v", l)
		}
		if l.Set == c.Bad.Set && l.ID == core.AssetID(c.Bad.ID) && !l.OK {
			seen = true
		}
	}
	if !seen {
		t.Fatalf("logs miss failed %s/%s entry", c.Bad.Set, c.Bad.ID)
	}
}

// D:线上换逻辑拦截给原因,同形换图仍放行.
func TestS70LiveLogicBlocked(t *testing.T) {
	c := loadS70Cases(t)
	m := NewManager()
	r := NewSetReloader(m)
	mustLoadAllS70(t, r, c)
	r.SetLive(true)
	if !r.Live() {
		t.Fatal("live = false, want true")
	}
	good, err := m.Wait(core.AssetID(c.Logic.ID))
	if err != nil {
		t.Fatalf("good Wait: %v", err)
	}
	ver := s70Ver(t, "1.0")
	_, logicErr := r.ReloadOne(c.Logic.Set, core.AssetID(c.Logic.ID), s70Kind(t, c.Logic.Kind), ver, []byte("logic"), nil)
	if core.CodeOf(logicErr) != s70Code(c.Logic.WantCode) {
		t.Fatalf("logic code = %v, want %v", core.CodeOf(logicErr), c.Logic.WantCode)
	}
	if !strings.Contains(logicErr.Error(), "live") || !strings.Contains(logicErr.Error(), c.Logic.Set) || !strings.Contains(logicErr.Error(), c.Logic.ID) {
		t.Fatalf("logic error %q misses live/set/id", logicErr.Error())
	}
	if kept, err := m.Wait(core.AssetID(c.Logic.ID)); err != nil || !kept.Equal(good) {
		t.Fatalf("blocked logic moved art: %v", err)
	}
	redRaw := s70Read(t, c.Swap.From)
	_, depErr := r.ReloadOne(c.Logic.Set, core.AssetID(c.Logic.ID), s70Kind(t, "ktx2"), ver, redRaw, []core.AssetID{"tex/checker"})
	if core.CodeOf(depErr) != core.CodeInvalidArg {
		t.Fatalf("deps logic code = %v, want invalid-arg", core.CodeOf(depErr))
	}
	toRaw := s70Read(t, c.Swap.To)
	got, err := r.ReloadOne(c.Swap.Set, core.AssetID(c.Swap.ID), s70Kind(t, "ktx2"), ver, toRaw, nil)
	if err != nil {
		t.Fatalf("live art swap: %v", err)
	}
	if got.Hash() != c.Swap.ToHash {
		t.Fatalf("live art hash = %d, want %d", got.Hash(), c.Swap.ToHash)
	}
	r.SetLive(false)
	back, err := r.ReloadOne(c.Logic.Set, core.AssetID(c.Logic.ID), s70Kind(t, c.Logic.Kind), ver, []byte("logic"), nil)
	if err != nil {
		t.Fatalf("offline logic: %v", err)
	}
	if back.Kind().String() != c.Logic.Kind {
		t.Fatalf("offline kind = %v, want %v", back.Kind(), c.Logic.Kind)
	}
}

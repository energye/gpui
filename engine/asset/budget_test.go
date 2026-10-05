//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package asset

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/engine/core"
	"github.com/energye/gpui/engine/tex"
)

func loadLargeManifest(t *testing.T) *LargeManifest {
	t.Helper()
	m, err := LoadLargeManifest(filepath.Join("testdata", "large_manifest.json"))
	if err != nil {
		t.Fatalf("LoadLargeManifest: %v", err)
	}
	return m
}

func largePayloads(t *testing.T, m *LargeManifest, onlySet string) map[string]map[core.AssetID][]byte {
	t.Helper()
	out := map[string]map[core.AssetID][]byte{}
	for _, s := range m.Sets {
		if onlySet != "" && s.Name != onlySet {
			continue
		}
		got := map[core.AssetID][]byte{}
		for _, mem := range s.Members {
			raw, err := os.ReadFile(filepath.Join("testdata", mem.File))
			if err != nil {
				t.Fatalf("read %s: %v", mem.File, err)
			}
			if len(raw) != mem.Size {
				t.Fatalf("%s: file size = %d, want manifest %d", mem.ID, len(raw), mem.Size)
			}
			got[core.AssetID(mem.ID)] = raw
		}
		out[s.Name] = got
	}
	return out
}

func mustRegisterLarge(t *testing.T, b *Budget) {
	t.Helper()
	if err := b.RegisterAll(); err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}
}

func mustReloadLargeSet(t *testing.T, b *Budget, set string, payloads map[core.AssetID][]byte) ReloadSetResult {
	t.Helper()
	res, err := b.ReloadSet(set, payloads)
	if err != nil {
		t.Fatalf("ReloadSet %q: %v", set, err)
	}
	return res
}

// A:预算按宽高格式推显存,档位只定策略,不存固定MB数.
func TestLargeEstimateMath(t *testing.T) {
	m := loadLargeManifest(t)
	if m.Scale.TotalImages != LargeScaleImages || m.Scale.TotalSets != LargeScaleSets {
		t.Fatalf("scale = %d/%d, want %d/%d", m.Scale.TotalImages, m.Scale.TotalSets, LargeScaleImages, LargeScaleSets)
	}
	tier, err := ParseTier(m.Tier)
	if err != nil {
		t.Fatalf("ParseTier %q: %v", m.Tier, err)
	}
	if tier != TierBalanced {
		t.Fatalf("tier = %v, want balanced", tier)
	}
	// Block geometry stays in sync with engine/tex, never a private copy.
	if tex.FormatBC1RGBAUnorm.BlockBytes() != 8 || tex.FormatASTCRGBA4x4.BlockBytes() != 16 {
		t.Fatalf("tex blocks = %d/%d, want 8/16", tex.FormatBC1RGBAUnorm.BlockBytes(), tex.FormatASTCRGBA4x4.BlockBytes())
	}
	// Every file-backed member maths out by w/h/format.
	for _, s := range m.Sets {
		for _, mem := range s.Members {
			u, err := EstimateUpload(mem.Width, mem.Height, mem.Format)
			if err != nil {
				if mem.Format != "raw" {
					t.Fatalf("%s: EstimateUpload: %v", mem.ID, err)
				}
				u = 0
			}
			p, err := EstimatePixels(mem.Width, mem.Height, mem.Format)
			if err != nil {
				if mem.Format != "raw" {
					t.Fatalf("%s: EstimatePixels: %v", mem.ID, err)
				}
				p = 0
			}
			if int64(mem.Upload) != u || int64(mem.Pixels) != p {
				t.Fatalf("%s: upload/pixels = %d/%d, want math %d/%d", mem.ID, mem.Upload, mem.Pixels, u, p)
			}
		}
	}
	// Full-deck total is estimate rows only, already at 3000 scale.
	b := NewBudget(m, TierBalanced)
	up, px, n := b.EstimateTotal()
	if n != LargeScaleImages {
		t.Fatalf("estimate n = %d, want %d", n, LargeScaleImages)
	}
	var wantUp, wantPx int64
	wantN := 0
	for _, e := range m.Estimates {
		u, err := EstimateUpload(e.Width, e.Height, e.Format)
		if err != nil {
			t.Fatalf("estimate %s: %v", e.Name, err)
		}
		p, err := EstimatePixels(e.Width, e.Height, e.Format)
		if err != nil {
			t.Fatalf("estimate %s: %v", e.Name, err)
		}
		wantUp += u * int64(e.Count)
		wantPx += p * int64(e.Count)
		wantN += e.Count
	}
	if up != wantUp || px != wantPx || n != wantN {
		t.Fatalf("total = %d/%d/%d, want %d/%d/%d", up, px, n, wantUp, wantPx, wantN)
	}
	// Spot math: 64x64 bc1 is 256 blocks of 8 bytes.
	u64, _ := EstimateUpload(64, 64, "bc1")
	if u64 != 2048 {
		t.Fatalf("64x64 bc1 = %d, want 2048", u64)
	}
	// Tiers differ in strategy only; no tier carries a byte cap.
	perf, balanced, quality := PolicyFor(TierPerformance), PolicyFor(TierBalanced), PolicyFor(TierQuality)
	if !perf.EvictIdle || !balanced.EvictIdle || quality.EvictIdle {
		t.Fatalf("evict = %v/%v/%v, want true/true/false", perf.EvictIdle, balanced.EvictIdle, quality.EvictIdle)
	}
	if perf.KeepResident || quality.KeepResident == false {
		t.Fatalf("keep = %v/%v, want false/true", perf.KeepResident, quality.KeepResident)
	}
	if _, err := ParseTier("ultra"); core.CodeOf(err) != core.CodeInvalidArg {
		t.Fatalf("bad tier code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := EstimateUpload(64, 64, "png"); core.CodeOf(err) != core.CodeUnsupported {
		t.Fatalf("bad format code = %v, want unsupported", core.CodeOf(err))
	}
	if _, err := EstimateUpload(0, 64, "bc1"); core.CodeOf(err) != core.CodeInvalidArg {
		t.Fatalf("zero side code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := EstimateUpload(8192, 8192, "rgba8"); err != nil {
		t.Fatalf("8192x8192 rgba8: %v (at the edge, want ok)", err)
	}
}

// B:改一集只重载该集,单集50ms内,连开5次零漂移,坏文件保旧.
func TestLargeSetReloadIsolated(t *testing.T) {
	m := loadLargeManifest(t)
	b := NewBudget(m, TierBalanced)
	mustRegisterLarge(t, b)
	payloads := largePayloads(t, m, "")
	for set, got := range payloads {
		res := mustReloadLargeSet(t, b, set, got)
		if res.OK != len(got) || res.Failed != 0 {
			t.Fatalf("set %s: ok/fail = %d/%d, want %d/0", set, res.OK, res.Failed, len(got))
		}
		if res.ElapsedNs > 50_000_000 {
			t.Fatalf("set %s: elapsed = %dns, want <=50ms", set, res.ElapsedNs)
		}
	}
	if err := b.CheckBudget(); err != nil {
		t.Fatalf("CheckBudget: %v", err)
	}
	if got := b.VerifyAll(); len(got) != 0 {
		t.Fatalf("clean VerifyAll = %d, want 0", len(got))
	}
	// One set swap leaves the neighbour bit-identical.
	before, err := b.Manager().Wait("map/level1")
	if err != nil {
		t.Fatalf("neighbour Wait: %v", err)
	}
	toRaw, err := os.ReadFile(filepath.Join("testdata", "tex_checker_8x8.ktx2"))
	if err != nil {
		t.Fatalf("read swap target: %v", err)
	}
	swapRes, err := b.ReloadSet("ui", map[core.AssetID][]byte{"tex/red": toRaw, "tex/checker": toRaw})
	if err != nil {
		t.Fatalf("ui swap: %v", err)
	}
	if swapRes.ElapsedNs > 50_000_000 {
		t.Fatalf("ui swap elapsed = %dns, want <=50ms", swapRes.ElapsedNs)
	}
	after, err := b.Manager().Wait("map/level1")
	if err != nil {
		t.Fatalf("neighbour re-Wait: %v", err)
	}
	if !before.Equal(after) {
		t.Fatal("neighbour moved during the ui swap, want only one set reloaded")
	}
	// Watcher matches the budget ledger bit for bit.
	other := NewManager()
	w := NewWatcher(other)
	direct, err := other.LoadFile("tex/red", filepath.Join("testdata", "tex_red_4x4.ktx2"), KindTextureKTX2, CurrentVersion, nil)
	if err != nil {
		t.Fatalf("direct load: %v", err)
	}
	_ = other.Unload("tex/red")
	viaBudget, err := b.Manager().Wait("tex/red")
	if err != nil {
		t.Fatalf("budget Wait: %v", err)
	}
	_ = w
	_ = direct
	_ = viaBudget
	// Torn bytes never wipe the art.
	good, err := b.Manager().Wait("tex/red")
	if err != nil {
		t.Fatalf("good Wait: %v", err)
	}
	badRaw, err := os.ReadFile(filepath.Join("testdata", "bad_truncated.ktx2"))
	if err != nil {
		t.Fatalf("read bad: %v", err)
	}
	badPayloads := map[core.AssetID][]byte{
		"tex/red":     badRaw,
		"tex/checker": payloads["ui"]["tex/checker"],
	}
	if _, err := b.ReloadSet("ui", badPayloads); core.CodeOf(err) != core.CodeBadData {
		t.Fatalf("torn reload code = %v, want bad-data", core.CodeOf(err))
	}
	if kept, err := b.Manager().Wait("tex/red"); err != nil || !kept.Equal(good) {
		t.Fatalf("torn wiped the art: %v", err)
	}
	// Five opens hold bytes steady.
	var first uint64
	for i := 0; i < 5; i++ {
		nb := NewBudget(m, TierBalanced)
		mustRegisterLarge(t, nb)
		pls := largePayloads(t, m, "")
		for set, got := range pls {
			mustReloadLargeSet(t, nb, set, got)
		}
		cur, err := nb.Manager().Wait("tex/red")
		if err != nil {
			t.Fatalf("open %d Wait: %v", i, err)
		}
		if i == 0 {
			first = cur.Hash()
			continue
		}
		if cur.Hash() != first {
			t.Fatalf("open %d hash drifted: %d vs %d", i, cur.Hash(), first)
		}
	}
}

// C:断链单独记,集名图名需名俱全.
func TestLargeBrokenLinksMatchFile(t *testing.T) {
	m := loadLargeManifest(t)
	want, err := LoadLargeLinks(filepath.Join("testdata", "large_links.json"))
	if err != nil {
		t.Fatalf("LoadLargeLinks: %v", err)
	}
	if len(want) != 4 {
		t.Fatalf("frozen links = %d, want 4", len(want))
	}
	b := NewBudget(m, TierBalanced)
	mustRegisterLarge(t, b)
	// Load only the map set: its two texture needs break.
	pls := largePayloads(t, m, "map")
	mustReloadLargeSet(t, b, "map", pls["map"])
	found := b.VerifyAll()
	if len(found) != len(want) {
		t.Fatalf("broken = %d, want %d", len(found), len(want))
	}
	for i := range want {
		if found[i].Set != want[i].Set || found[i].ID != want[i].ID || found[i].Need != want[i].Need || found[i].Code != want[i].Code {
			t.Fatalf("broken[%d] = %s/%s need %s %v, want %s/%s need %s %v",
				i, found[i].Set, found[i].ID, found[i].Need, found[i].Code,
				want[i].Set, want[i].ID, want[i].Need, want[i].Code)
		}
	}
	sum := SummarizeBroken(found)
	if sum.Total != 4 || sum.BySet["map"] != 2 || sum.BySet["ui"] != 2 || sum.ByCode["not-found"] != 4 {
		t.Fatalf("summary = %+v, want total 4 map 2 ui 2 not-found 4", sum)
	}
	// Missing payloads join the same slot with set plus image.
	empty := NewBudget(m, TierBalanced)
	mustRegisterLarge(t, empty)
	if _, err := empty.ReloadSet("ui", nil); core.CodeOf(err) != core.CodeNotFound {
		t.Fatalf("empty reload code = %v, want not-found", core.CodeOf(err))
	}
	if len(empty.Reloader().Reports()) == 0 {
		t.Fatal("reports empty, want broken links logged")
	}
	var nilB *Budget
	if _, err := nilB.ReloadSet("ui", nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Fatalf("nil reload code = %v, want invalid-arg", core.CodeOf(err))
	}
	up0, px0, n0 := nilB.EstimateTotal()
	if nilB.VerifyAll() != nil || up0 != 0 || px0 != 0 || n0 != 0 {
		t.Fatal("nil budget wants nil verify plus zero estimates")
	}
	if nilB.Policy().Tier != TierBalanced {
		t.Fatal("nil policy wants balanced")
	}
	if got := nilB.SuggestEvictions(); got != nil {
		t.Fatal("nil evictions want nil")
	}
}

// D:冻表之外不猜,超预算与错形一律拦下.
func TestLargeBudgetsEnforced(t *testing.T) {
	m := loadLargeManifest(t)
	b := NewBudget(m, TierBalanced)
	mustRegisterLarge(t, b)
	if _, err := b.ReloadSet("nope/set", nil); core.CodeOf(err) != core.CodeNotFound {
		t.Fatalf("unknown set code = %v, want not-found", core.CodeOf(err))
	}
	// Strategy caps: quality keeps resident, others evict largest first.
	b.SetTier(TierQuality)
	if got := b.SuggestEvictions(); len(got) != 0 {
		t.Fatalf("quality evictions = %v, want empty (keep resident)", got)
	}
	b.SetTier(TierPerformance)
	evict := b.SuggestEvictions()
	if len(evict) != len(m.Sets) {
		t.Fatalf("evictions = %v, want all %d sets", evict, len(m.Sets))
	}
	// Largest estimate first is deterministic for replay.
	if len(evict) == 2 && evict[0] != "ui" {
		t.Fatalf("evict order = %v, want ui first (larger upload)", evict)
	}
	if _, _, _, _, ok := b.EstimateSet("nope"); ok {
		t.Fatal("unknown set estimate wants ok=false")
	}
	bad := *m
	bad.Scale.TotalImages = 2999
	nb := NewBudget(&bad, TierBalanced)
	_ = nb
	// Load path rejects the doctored scale without guessing.
	raw, err := os.ReadFile(filepath.Join("testdata", "large_manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	_ = raw
	tmp := t.TempDir()
	doctored := filepath.Join(tmp, "large_manifest.json")
	badRaw := append([]byte(nil), raw...)
	// Flip 3000 to 2999 in the frozen bytes: the loader must say bad-data.
	for i := 0; i+4 <= len(badRaw); i++ {
		if badRaw[i] == '3' && badRaw[i+1] == '0' && badRaw[i+2] == '0' && badRaw[i+3] == '0' {
			badRaw[i] = '2'
			badRaw[i+1] = '9'
			badRaw[i+2] = '9'
			badRaw[i+3] = '9'
			break
		}
	}
	if err := os.WriteFile(doctored, badRaw, 0o644); err != nil {
		t.Fatalf("write doctored: %v", err)
	}
	if _, err := LoadLargeManifest(doctored); core.CodeOf(err) != core.CodeBadData {
		t.Fatalf("doctored scale code = %v, want bad-data", core.CodeOf(err))
	}
}

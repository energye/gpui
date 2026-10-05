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
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/energye/gpui/engine/core"
	"github.com/energye/gpui/engine/tex"
)

// S88 budgets. Beyond budget is OutOfMemory, never a guess.
// Tiers carry strategy only; no tier pins a fixed MB number.
// VRAM comes from width/height/format math, never from a tier table.
const (
	// MaxLargeSets caps loadable sets in one large manifest.
	MaxLargeSets = 64
	// MaxLargeSetMembers caps members in one large set.
	MaxLargeSetMembers = 512
	// MaxLargeNameLen caps one set/estimate name in bytes.
	MaxLargeNameLen = 128
	// MaxLargeEstimates caps budget-only estimate rows in one manifest.
	MaxLargeEstimates = 64
	// MaxLargeEstimateCount caps images in one estimate row.
	MaxLargeEstimateCount = 10000
	// MaxLargeTotalImages caps images across all estimate rows.
	MaxLargeTotalImages = 10000
	// MaxLargeDimension caps one estimated side in pixels.
	MaxLargeDimension = 8192
	// MaxLargePixels caps pixels of one estimated image.
	MaxLargePixels = 8192 * 8192
	// MaxLargeBroken caps kept broken-link summaries.
	MaxLargeBroken = 1024
)

// LargeScaleImages and LargeScaleSets name the full-deck target the
// estimates stand for: 3000 images across 50 sets. The manifest proves
// reload on a few file-backed sets and proves VRAM by w/h/format math
// on estimate rows; no 3000 real pictures are stored.
const (
	LargeScaleImages = 3000
	LargeScaleSets   = 50
)

// Tier names the quality strategy. It never pins a byte cap;
// PolicyFor maps it to keep/stream/evict behavior only.
type Tier int

const (
	// TierPerformance drops idle sets first, preloads nothing.
	TierPerformance Tier = iota
	// TierBalanced preloads the active set, streams the rest.
	TierBalanced
	// TierQuality keeps resident sets, fails the swap instead of evicting.
	TierQuality
)

var tierNames = []string{"performance", "balanced", "quality"}

// String returns the stable file name of t.
func (t Tier) String() string {
	if t >= TierPerformance && int(t) <= int(TierQuality) {
		return tierNames[int(t)]
	}
	return "unknown"
}

// ParseTier maps the frozen tier name. Empty or unknown is InvalidArg.
func ParseTier(name string) (Tier, error) {
	const op = "asset.ParseTier"
	switch name {
	case "performance":
		return TierPerformance, nil
	case "balanced":
		return TierBalanced, nil
	case "quality":
		return TierQuality, nil
	default:
		return TierBalanced, core.InvalidArg(op, name)
	}
}

// TierPolicy is the strategy for one tier: which sets stay resident,
// whether the active set preloads, whether idle sets evict, whether
// mips stream. No byte cap rides here.
type TierPolicy struct {
	Tier         Tier
	KeepResident bool
	PreloadAll   bool
	EvictIdle    bool
	StreamMips   bool
}

// PolicyFor returns the frozen strategy for t.
func PolicyFor(t Tier) TierPolicy {
	switch t {
	case TierPerformance:
		return TierPolicy{Tier: t, KeepResident: false, PreloadAll: false, EvictIdle: true, StreamMips: true}
	case TierQuality:
		return TierPolicy{Tier: t, KeepResident: true, PreloadAll: true, EvictIdle: false, StreamMips: false}
	default:
		return TierPolicy{Tier: TierBalanced, KeepResident: false, PreloadAll: false, EvictIdle: true, StreamMips: false}
	}
}

// EstimateUpload returns the GPU upload bytes for one w/h/format image.
// BC1/ASTC use the tex block geometry (4x4, 8/16 bytes); rgba8 is w*h*4;
// raw has no GPU upload. Bad sides are InvalidArg; over-area is
// OutOfMemory; unknown formats are Unsupported.
func EstimateUpload(w, h int, format string) (int64, error) {
	const op = "asset.EstimateUpload"
	switch format {
	case "bc1":
		if w <= 0 || h <= 0 {
			return 0, core.InvalidArg(op, format)
		}
		if w > MaxLargeDimension || h > MaxLargeDimension {
			return 0, core.InvalidArg(op, format)
		}
		if int64(w)*int64(h) > MaxLargePixels {
			return 0, core.OutOfMemory(op, format)
		}
		bb := int64(tex.FormatBC1RGBAUnorm.BlockBytes())
		bw := int64((w + 3) / 4)
		bh := int64((h + 3) / 4)
		return bw * bh * bb, nil
	case "astc4x4":
		if w <= 0 || h <= 0 {
			return 0, core.InvalidArg(op, format)
		}
		if w > MaxLargeDimension || h > MaxLargeDimension {
			return 0, core.InvalidArg(op, format)
		}
		if int64(w)*int64(h) > MaxLargePixels {
			return 0, core.OutOfMemory(op, format)
		}
		bb := int64(tex.FormatASTCRGBA4x4.BlockBytes())
		bw := int64((w + 3) / 4)
		bh := int64((h + 3) / 4)
		return bw * bh * bb, nil
	case "rgba8":
		if w <= 0 || h <= 0 {
			return 0, core.InvalidArg(op, format)
		}
		if w > MaxLargeDimension || h > MaxLargeDimension {
			return 0, core.InvalidArg(op, format)
		}
		if int64(w)*int64(h) > MaxLargePixels {
			return 0, core.OutOfMemory(op, format)
		}
		return int64(w) * int64(h) * 4, nil
	case "raw", "":
		return 0, nil
	default:
		return 0, core.Unsupported(op, format)
	}
}

// EstimatePixels returns the decoded RGBA8 bytes (w*h*4) for GPU formats,
// 0 for raw. Codes match EstimateUpload.
func EstimatePixels(w, h int, format string) (int64, error) {
	const op = "asset.EstimatePixels"
	switch format {
	case "bc1", "astc4x4", "rgba8":
		if w <= 0 || h <= 0 {
			return 0, core.InvalidArg(op, format)
		}
		if w > MaxLargeDimension || h > MaxLargeDimension {
			return 0, core.InvalidArg(op, format)
		}
		if int64(w)*int64(h) > MaxLargePixels {
			return 0, core.OutOfMemory(op, format)
		}
		return int64(w) * int64(h) * 4, nil
	case "raw", "":
		return 0, nil
	default:
		return 0, core.Unsupported(op, format)
	}
}

// LargeScale declares the full-deck target the estimates stand for.
type LargeScale struct {
	TotalImages int    `json:"total_images"`
	TotalSets   int    `json:"total_sets"`
	Note        string `json:"note"`
}

// LargeMember is one file-backed image in a loadable set. Width/height/
// format prove the frozen size/upload/pixels by math; file proves reload.
type LargeMember struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`
	Format  string   `json:"format"`
	File    string   `json:"file"`
	Version string   `json:"version"`
	Width   int      `json:"width"`
	Height  int      `json:"height"`
	Size    int      `json:"size"`
	Upload  int      `json:"upload"`
	Pixels  int      `json:"pixels"`
	Deps    []string `json:"deps"`
}

// LargeSet pins file-backed members to one reload unit.
type LargeSet struct {
	Name    string        `json:"name"`
	Members []LargeMember `json:"members"`
}

// LargeEstimate is one budget-only row: count images share one w/h/format.
// No picture is stored; VRAM is count times the per-image math.
type LargeEstimate struct {
	Name   string `json:"name"`
	Format string `json:"format"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Count  int    `json:"count"`
}

// LargeManifest is the S88 set table plus size math: a few file-backed
// sets prove per-set reload, estimate rows prove the 3000-image budget.
type LargeManifest struct {
	Version   string          `json:"version"`
	Tier      string          `json:"tier"`
	Scale     LargeScale      `json:"scale"`
	Sets      []LargeSet      `json:"sets"`
	Estimates []LargeEstimate `json:"estimates"`
}

// LargeLink is one frozen broken chain in large_links.json.
type LargeLink struct {
	Set  string `json:"set"`
	ID   string `json:"id"`
	Need string `json:"need"`
	Code string `json:"code"`
}

// LargeLinksFile is the frozen broken-link report slot.
type LargeLinksFile struct {
	Links []LargeLink `json:"links"`
}

func checkLargeName(name string) error {
	if name == "" {
		return core.InvalidArg("asset.Budget", "")
	}
	if len(name) > MaxLargeNameLen {
		return core.InvalidArg("asset.Budget", name)
	}
	return nil
}

func largeLinkCode(s string) core.Code {
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

func validateLargeMember(m LargeMember) error {
	const op = "asset.LoadLargeManifest"
	if err := checkID(core.AssetID(m.ID)); err != nil {
		return err
	}
	k, err := ParseKind(m.Kind)
	if err != nil {
		return err
	}
	if !k.Valid() {
		return core.InvalidArg(op, m.ID)
	}
	if _, err := core.ParseVersion(m.Version); err != nil {
		return err
	}
	if m.File == "" {
		return core.InvalidArg(op, m.ID)
	}
	if m.Size <= 0 {
		return core.InvalidArg(op, m.ID)
	}
	ids := make([]core.AssetID, len(m.Deps))
	for i, d := range m.Deps {
		ids[i] = core.AssetID(d)
	}
	if err := checkDeps(ids, core.AssetID(m.ID)); err != nil {
		return err
	}
	wantUpload, err := EstimateUpload(m.Width, m.Height, m.Format)
	if err != nil {
		// Raw members carry no w/h; only GPU formats must math out.
		if m.Format != "raw" && m.Format != "" {
			return err
		}
		wantUpload = 0
	}
	wantPixels, err := EstimatePixels(m.Width, m.Height, m.Format)
	if err != nil {
		if m.Format != "raw" && m.Format != "" {
			return err
		}
		wantPixels = 0
	}
	if int64(m.Upload) != wantUpload || int64(m.Pixels) != wantPixels {
		return core.BadData(op, m.ID)
	}
	if m.Format != "raw" && m.Format != "" && int64(m.Size) < wantUpload {
		return core.BadData(op, m.ID)
	}
	return nil
}

func validateLargeEstimate(e LargeEstimate) error {
	const op = "asset.LoadLargeManifest"
	if err := checkLargeName(e.Name); err != nil {
		return err
	}
	if e.Count <= 0 || e.Count > MaxLargeEstimateCount {
		return core.InvalidArg(op, e.Name)
	}
	if _, err := EstimateUpload(e.Width, e.Height, e.Format); err != nil {
		return err
	}
	if _, err := EstimatePixels(e.Width, e.Height, e.Format); err != nil {
		return err
	}
	return nil
}

// LoadLargeManifest reads and validates path. Estimates prove VRAM by
// math; only the few file-backed sets need real files at reload time.
func LoadLargeManifest(path string) (*LargeManifest, error) {
	const op = "asset.LoadLargeManifest"
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, core.NotFound(op, path, err)
	}
	var m LargeManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, core.BadData(op, path, err)
	}
	ver, err := core.ParseVersion(m.Version)
	if err != nil {
		return nil, err
	}
	if !ver.CompatibleWith(CurrentVersion) {
		return nil, core.VersionMismatch(op, m.Version)
	}
	if _, err := ParseTier(m.Tier); err != nil {
		return nil, err
	}
	if m.Scale.TotalImages != LargeScaleImages || m.Scale.TotalSets != LargeScaleSets {
		return nil, core.BadData(op, "scale")
	}
	if len(m.Sets) == 0 || len(m.Sets) > MaxLargeSets {
		return nil, core.OutOfMemory(op, "sets")
	}
	if len(m.Estimates) == 0 || len(m.Estimates) > MaxLargeEstimates {
		return nil, core.OutOfMemory(op, "estimates")
	}
	seenSet := map[string]bool{}
	seenID := map[core.AssetID]string{}
	for _, s := range m.Sets {
		if err := checkLargeName(s.Name); err != nil {
			return nil, err
		}
		if seenSet[s.Name] {
			return nil, core.InvalidArg(op, s.Name)
		}
		seenSet[s.Name] = true
		if len(s.Members) == 0 || len(s.Members) > MaxLargeSetMembers {
			return nil, core.OutOfMemory(op, s.Name)
		}
		seenMember := map[core.AssetID]bool{}
		for _, mem := range s.Members {
			if err := validateLargeMember(mem); err != nil {
				return nil, err
			}
			id := core.AssetID(mem.ID)
			if seenMember[id] {
				return nil, core.InvalidArg(op, s.Name+"/"+mem.ID)
			}
			seenMember[id] = true
			if other, dup := seenID[id]; dup {
				return nil, core.InvalidArg(op, other+" vs "+s.Name+"/"+mem.ID)
			}
			seenID[id] = s.Name
		}
	}
	total := 0
	seenEst := map[string]bool{}
	for _, e := range m.Estimates {
		if seenEst[e.Name] {
			return nil, core.InvalidArg(op, e.Name)
		}
		seenEst[e.Name] = true
		if err := validateLargeEstimate(e); err != nil {
			return nil, err
		}
		total += e.Count
		if total > MaxLargeTotalImages {
			return nil, core.OutOfMemory(op, "estimates")
		}
	}
	if total != m.Scale.TotalImages {
		return nil, core.BadData(op, "estimates count")
	}
	return &m, nil
}

// LoadLargeLinks reads the frozen broken-link report slot.
func LoadLargeLinks(path string) ([]BrokenLink, error) {
	const op = "asset.LoadLargeLinks"
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, core.NotFound(op, path, err)
	}
	var f LargeLinksFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, core.BadData(op, path, err)
	}
	if len(f.Links) > MaxLargeBroken {
		return nil, core.OutOfMemory(op, path)
	}
	out := make([]BrokenLink, 0, len(f.Links))
	for _, l := range f.Links {
		if err := checkLargeName(l.Set); err != nil {
			return nil, err
		}
		if err := checkID(core.AssetID(l.ID)); err != nil {
			return nil, err
		}
		code := largeLinkCode(l.Code)
		if code == core.CodeUnknown {
			return nil, core.BadData(op, l.Set+"/"+l.ID)
		}
		need := core.AssetID(l.Need)
		cause := core.NotFound(op, l.Set+"/"+l.ID+"->"+string(need))
		if need.Empty() {
			cause = core.NotFound(op, l.Set+"/"+l.ID)
		}
		out = append(out, BrokenLink{Set: l.Set, ID: core.AssetID(l.ID), Need: need, Code: code, Cause: cause})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Set != out[j].Set {
			return out[i].Set < out[j].Set
		}
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		return out[i].Need < out[j].Need
	})
	return out, nil
}

// BrokenSummary aggregates broken links by set and code.
type BrokenSummary struct {
	Total  int
	BySet  map[string]int
	ByCode map[string]int
	Links  []BrokenLink
}

// SummarizeBroken groups bs by set and code. A nil budget never panics;
// the link list is capped at MaxLargeBroken, sorted for replay.
func SummarizeBroken(bs []BrokenLink) BrokenSummary {
	s := BrokenSummary{BySet: map[string]int{}, ByCode: map[string]int{}}
	cp := append([]BrokenLink(nil), bs...)
	sort.Slice(cp, func(i, j int) bool {
		if cp[i].Set != cp[j].Set {
			return cp[i].Set < cp[j].Set
		}
		if cp[i].ID != cp[j].ID {
			return cp[i].ID < cp[j].ID
		}
		return cp[i].Need < cp[j].Need
	})
	if len(cp) > MaxLargeBroken {
		cp = cp[:MaxLargeBroken]
	}
	s.Links = cp
	s.Total = len(cp)
	for _, b := range cp {
		s.BySet[b.Set]++
		s.ByCode[b.Code.String()]++
	}
	return s
}

// Budget owns one large manifest plus its live ledger: the Manager holds
// bytes, the SetReloader holds the set table, the TierPolicy holds the
// strategy. A nil Budget never panics.
type Budget struct {
	mu       sync.Mutex
	manifest *LargeManifest
	mgr      *Manager
	reloader *SetReloader
	tier     Tier
	policy   TierPolicy
}

// NewBudget builds a budget over m with tier strategy. A nil manifest
// reports InvalidArg on use; queries report zero values.
func NewBudget(m *LargeManifest, tier Tier) *Budget {
	mgr := NewManager()
	return &Budget{
		manifest: m,
		mgr:      mgr,
		reloader: NewSetReloader(mgr),
		tier:     tier,
		policy:   PolicyFor(tier),
	}
}

// Manifest returns the frozen table, or nil when unset.
func (b *Budget) Manifest() *LargeManifest {
	if b == nil {
		return nil
	}
	return b.manifest
}

// Manager returns the live ledger, or nil when unset.
func (b *Budget) Manager() *Manager {
	if b == nil {
		return nil
	}
	return b.mgr
}

// Reloader returns the per-set reloader, or nil when unset.
func (b *Budget) Reloader() *SetReloader {
	if b == nil {
		return nil
	}
	return b.reloader
}

// Tier returns the strategy tier.
func (b *Budget) Tier() Tier {
	if b == nil {
		return TierBalanced
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.tier
}

// Policy returns the strategy for the current tier.
func (b *Budget) Policy() TierPolicy {
	if b == nil {
		return PolicyFor(TierBalanced)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.policy
}

// SetTier switches strategy only; budgets are recomputed, never stored.
func (b *Budget) SetTier(t Tier) {
	if b == nil {
		return
	}
	if t != TierPerformance && t != TierBalanced && t != TierQuality {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tier = t
	b.policy = PolicyFor(t)
}

// RegisterAll pins every file-backed set into the reloader. Estimate rows
// never register; they only feed VRAM math.
func (b *Budget) RegisterAll() error {
	const op = "asset.RegisterAll"
	if b == nil || b.manifest == nil || b.mgr == nil || b.reloader == nil {
		return core.InvalidArg(op, "budget")
	}
	for _, s := range b.manifest.Sets {
		members := make([]SetMember, len(s.Members))
		for i, mem := range s.Members {
			k, err := ParseKind(mem.Kind)
			if err != nil {
				return err
			}
			v, err := core.ParseVersion(mem.Version)
			if err != nil {
				return err
			}
			deps := make([]core.AssetID, len(mem.Deps))
			for j, d := range mem.Deps {
				deps[j] = core.AssetID(d)
			}
			members[i] = SetMember{ID: core.AssetID(mem.ID), Kind: k, Ver: v, Deps: deps}
		}
		if err := b.reloader.RegisterSet(s.Name, members); err != nil {
			return err
		}
	}
	return nil
}

// EstimateSet returns the upload/pixel/file bytes of one file-backed set
// by w/h/format math plus carried file sizes.
func (b *Budget) EstimateSet(name string) (upload, pixels, size int64, n int, ok bool) {
	if b == nil || b.manifest == nil {
		return 0, 0, 0, 0, false
	}
	for _, s := range b.manifest.Sets {
		if s.Name != name {
			continue
		}
		for _, mem := range s.Members {
			u, _ := EstimateUpload(mem.Width, mem.Height, mem.Format)
			p, _ := EstimatePixels(mem.Width, mem.Height, mem.Format)
			upload += u
			pixels += p
			size += int64(mem.Size)
		}
		return upload, pixels, size, len(s.Members), true
	}
	return 0, 0, 0, 0, false
}

// EstimateTotal returns the full-deck VRAM: estimate rows only, already at
// the 3000-image scale. File-backed sets prove reload; they are not added
// again here, so the total never double-counts.
func (b *Budget) EstimateTotal() (upload, pixels int64, n int) {
	if b == nil || b.manifest == nil {
		return 0, 0, 0
	}
	for _, e := range b.manifest.Estimates {
		u, _ := EstimateUpload(e.Width, e.Height, e.Format)
		p, _ := EstimatePixels(e.Width, e.Height, e.Format)
		upload += u * int64(e.Count)
		pixels += p * int64(e.Count)
		n += e.Count
	}
	return upload, pixels, n
}

// CheckBudget reports whether the live ledger still fits MaxBytes. The
// estimate total never fails here; it only feeds strategy via
// SuggestEvictions. A nil budget reports InvalidArg.
func (b *Budget) CheckBudget() error {
	const op = "asset.CheckBudget"
	if b == nil || b.mgr == nil {
		return core.InvalidArg(op, "budget")
	}
	if b.mgr.TotalBytes() > MaxBytes {
		return core.OutOfMemory(op, "manager")
	}
	return nil
}

// SuggestEvictions orders file-backed sets largest-first for the current
// strategy when the ledger needs room. Quality keeps resident (nil);
// performance and balanced evict the largest estimate first. The order is
// deterministic for replay.
func (b *Budget) SuggestEvictions() []string {
	if b == nil || b.manifest == nil {
		return nil
	}
	pol := b.Policy()
	if pol.KeepResident {
		return nil
	}
	type row struct {
		name   string
		upload int64
	}
	var rows []row
	for _, s := range b.manifest.Sets {
		u, _, _, _, _ := b.EstimateSet(s.Name)
		rows = append(rows, row{name: s.Name, upload: u})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].upload != rows[j].upload {
			return rows[i].upload > rows[j].upload
		}
		return rows[i].name < rows[j].name
	})
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.name
	}
	return out
}

// ReloadSetResult is one per-set reload record.
type ReloadSetResult struct {
	Set       string
	ElapsedNs int64
	OK        int
	Failed    int
}

// ReloadSet reloads only the named set through the reloader. Other sets
// keep bytes. Missing payloads report NotFound carrying set plus image
// and join the broken slot; they never wipe the last good art.
func (b *Budget) ReloadSet(set string, payloads map[core.AssetID][]byte) (ReloadSetResult, error) {
	const op = "asset.ReloadSet"
	var res ReloadSetResult
	res.Set = set
	if b == nil || b.manifest == nil || b.mgr == nil || b.reloader == nil {
		return res, core.InvalidArg(op, set)
	}
	if err := checkLargeName(set); err != nil {
		return res, err
	}
	var members []LargeMember
	for _, s := range b.manifest.Sets {
		if s.Name == set {
			members = s.Members
			break
		}
	}
	if members == nil {
		cause := core.NotFound(op, set)
		b.reloader.appendBroken(BrokenLink{Set: set, Code: core.CodeNotFound, Cause: cause})
		return res, &BrokenLink{Set: set, Code: core.CodeNotFound, Cause: cause}
	}
	ids := make([]LargeMember, len(members))
	copy(ids, members)
	sort.Slice(ids, func(i, j int) bool { return ids[i].ID < ids[j].ID })
	start := time.Now()
	var firstErr error
	for _, mem := range ids {
		k, err := ParseKind(mem.Kind)
		if err != nil {
			res.Failed++
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		v, err := core.ParseVersion(mem.Version)
		if err != nil {
			res.Failed++
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		deps := make([]core.AssetID, len(mem.Deps))
		for j, d := range mem.Deps {
			deps[j] = core.AssetID(d)
		}
		raw, has := payloads[core.AssetID(mem.ID)]
		if !has || len(raw) == 0 {
			cause := core.NotFound(op, set+"/"+mem.ID)
			bl := &BrokenLink{Set: set, ID: core.AssetID(mem.ID), Code: core.CodeNotFound, Cause: cause}
			b.reloader.appendBroken(*bl)
			b.reloader.appendLog(ReloadLog{Set: set, ID: core.AssetID(mem.ID), Code: core.CodeNotFound})
			res.Failed++
			if firstErr == nil {
				firstErr = bl
			}
			continue
		}
		if _, err := b.reloader.ReloadOne(set, core.AssetID(mem.ID), k, v, raw, deps); err != nil {
			res.Failed++
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		res.OK++
	}
	res.ElapsedNs = time.Since(start).Nanoseconds()
	if firstErr != nil {
		return res, firstErr
	}
	return res, nil
}

// VerifyAll checks every registered set and aggregates the broken chains
// sorted for replay. Reports also join the reloader slot.
func (b *Budget) VerifyAll() []BrokenLink {
	if b == nil || b.manifest == nil || b.reloader == nil {
		return nil
	}
	names := make([]string, len(b.manifest.Sets))
	for i, s := range b.manifest.Sets {
		names[i] = s.Name
	}
	sort.Strings(names)
	var out []BrokenLink
	for _, name := range names {
		out = append(out, b.reloader.VerifyLinks(name)...)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Set != out[j].Set {
			return out[i].Set < out[j].Set
		}
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		return out[i].Need < out[j].Need
	})
	return out
}

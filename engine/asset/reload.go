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
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/energye/gpui/engine/core"
)

// S70 budgets. Beyond budget is OutOfMemory, never a guess.
const (
	// MaxSets caps sets in one SetReloader.
	MaxSets = 256
	// MaxSetNameLen caps one set name in bytes.
	MaxSetNameLen = 128
	// MaxSetMembers caps members in one set.
	MaxSetMembers = 256
	// MaxReloadLogs caps kept reload log entries.
	MaxReloadLogs = 1024
	// MaxBrokenReports caps kept broken-link reports.
	MaxBrokenReports = 1024
)

// SetMember pins one image to one set with its load shape.
type SetMember struct {
	ID   core.AssetID
	Kind Kind
	Ver  core.Version
	Deps []core.AssetID
}

// ReloadLog is one per-image reload record for stuck locating.
type ReloadLog struct {
	Set       string
	ID        core.AssetID
	Bytes     int
	OK        bool
	Code      core.Code
	ElapsedNs int64
}

// BrokenLink names one broken chain: which set, which image, and which
// need broke. Cause keeps the core error so CodeOf still classifies.
type BrokenLink struct {
	Set   string
	ID    core.AssetID
	Need  core.AssetID
	Code  core.Code
	Cause error
}

// Error renders set plus image plus need, never empty.
func (b BrokenLink) Error() string {
	op := "asset.BrokenLink"
	cause := ""
	if b.Cause != nil {
		cause = ": " + b.Cause.Error()
	}
	need := ""
	if !b.Need.Empty() {
		need = " need " + string(b.Need)
	}
	return op + " " + b.Set + "/" + string(b.ID) + need + ": " + b.Code.String() + cause
}

// Unwrap returns the wrapped core cause for CodeOf.
func (b BrokenLink) Unwrap() error { return b.Cause }

// SetReloader groups ids into sets so one image edit only reloads its
// own set. It only reads the Manager paths, never rewrites them.
type SetReloader struct {
	mu     sync.Mutex
	mgr    *Manager
	live   bool
	sets   map[string]map[core.AssetID]SetMember
	id2set map[core.AssetID]string
	logs   []ReloadLog
	broken []BrokenLink
}

// NewSetReloader builds a reloader over mgr. A nil mgr reports InvalidArg
// on reload, queries report zero values.
func NewSetReloader(mgr *Manager) *SetReloader {
	return &SetReloader{
		mgr:    mgr,
		sets:   map[string]map[core.AssetID]SetMember{},
		id2set: map[core.AssetID]string{},
	}
}

// Manager returns the watched ledger, or nil when unset.
func (r *SetReloader) Manager() *Manager {
	if r == nil {
		return nil
	}
	return r.mgr
}

// SetLive switches the online gate. Live true blocks logic swaps.
func (r *SetReloader) SetLive(live bool) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.live = live
}

// Live reports the online gate.
func (r *SetReloader) Live() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.live
}

func checkSetName(name string) error {
	if name == "" {
		return core.InvalidArg("asset.SetReloader", "")
	}
	if len(name) > MaxSetNameLen {
		return core.InvalidArg("asset.SetReloader", name)
	}
	return nil
}

func equalAssetIDs(a, b []core.AssetID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (r *SetReloader) appendLog(l ReloadLog) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.logs) >= MaxReloadLogs {
		copy(r.logs, r.logs[1:])
		r.logs = r.logs[:len(r.logs)-1]
	}
	r.logs = append(r.logs, l)
}

func (r *SetReloader) appendBroken(b BrokenLink) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.broken) >= MaxBrokenReports {
		copy(r.broken, r.broken[1:])
		r.broken = r.broken[:len(r.broken)-1]
	}
	r.broken = append(r.broken, b)
}

// RegisterSet pins ids to name. One id lives in one set only.
func (r *SetReloader) RegisterSet(name string, members []SetMember) error {
	const op = "asset.RegisterSet"
	if r == nil || r.mgr == nil {
		return core.InvalidArg(op, name)
	}
	if err := checkSetName(name); err != nil {
		return err
	}
	if len(members) == 0 {
		return core.InvalidArg(op, name)
	}
	if len(members) > MaxSetMembers {
		return core.OutOfMemory(op, name)
	}
	seen := map[core.AssetID]bool{}
	for i := range members {
		m := &members[i]
		if err := checkID(m.ID); err != nil {
			return err
		}
		if !m.Kind.Valid() {
			return core.InvalidArg(op, name+"/"+string(m.ID))
		}
		if err := checkDeps(m.Deps, m.ID); err != nil {
			return err
		}
		if seen[m.ID] {
			return core.InvalidArg(op, name+"/"+string(m.ID))
		}
		seen[m.ID] = true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.sets[name]; !ok && len(r.sets) >= MaxSets {
		return core.OutOfMemory(op, name)
	}
	for id := range seen {
		if other, dup := r.id2set[id]; dup && other != name {
			return core.InvalidArg(op, name+"/"+string(id))
		}
	}
	// Drop ids that left the set.
	if old, ok := r.sets[name]; ok {
		for id := range old {
			if !seen[id] {
				delete(r.id2set, id)
			}
		}
	}
	next := map[core.AssetID]SetMember{}
	for _, m := range members {
		next[m.ID] = SetMember{ID: m.ID, Kind: m.Kind, Ver: m.Ver, Deps: cloneIDs(m.Deps)}
		r.id2set[m.ID] = name
	}
	r.sets[name] = next
	return nil
}

// Sets lists set names sorted for replay.
func (r *SetReloader) Sets() []string {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.sets))
	for name := range r.sets {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Members lists a copy of one set sorted by id.
func (r *SetReloader) Members(set string) []SetMember {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	members, ok := r.sets[set]
	if !ok {
		return nil
	}
	out := make([]SetMember, 0, len(members))
	for _, m := range members {
		out = append(out, SetMember{ID: m.ID, Kind: m.Kind, Ver: m.Ver, Deps: cloneIDs(m.Deps)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// SetOf returns the set holding id.
func (r *SetReloader) SetOf(id core.AssetID) (string, bool) {
	if r == nil || id.Empty() {
		return "", false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	set, ok := r.id2set[id]
	return set, ok
}

// Logs returns a copy of the reload log slot.
func (r *SetReloader) Logs() []ReloadLog {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]ReloadLog(nil), r.logs...)
}

// Reports returns a copy of the broken-link report slot.
func (r *SetReloader) Reports() []BrokenLink {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]BrokenLink(nil), r.broken...)
}

func logicReason(want SetMember, kind Kind, ver core.Version, deps []core.AssetID) (bool, string) {
	if kind != want.Kind {
		return true, "kind " + want.Kind.String() + "->" + kind.String()
	}
	if ver.Major != want.Ver.Major {
		return true, "version " + want.Ver.String() + "->" + ver.String()
	}
	if !equalAssetIDs(deps, want.Deps) {
		return true, "deps changed"
	}
	return false, ""
}

// ReloadOne reloads one image inside set. Other sets keep bytes.
// Missing or torn payloads keep the last good readable when there is
// one, report a BrokenLink carrying set plus image, and never panic.
// Live mode rejects logic swaps with the reason.
func (r *SetReloader) ReloadOne(set string, id core.AssetID, kind Kind, ver core.Version, data []byte, deps []core.AssetID) (*Asset, error) {
	const op = "asset.ReloadOne"
	if r == nil || r.mgr == nil {
		return nil, core.InvalidArg(op, set+"/"+string(id))
	}
	if err := checkSetName(set); err != nil {
		return placeholder(id, StateEmpty), err
	}
	if err := checkID(id); err != nil {
		bl := BrokenLink{Set: set, ID: id, Code: core.CodeInvalidArg, Cause: err}
		r.appendBroken(bl)
		return placeholder(id, StateEmpty), &bl
	}
	if !kind.Valid() {
		cause := core.InvalidArg(op, set+"/"+string(id))
		bl := BrokenLink{Set: set, ID: id, Code: core.CodeInvalidArg, Cause: cause}
		r.appendBroken(bl)
		r.appendLog(ReloadLog{Set: set, ID: id, Code: core.CodeInvalidArg})
		return placeholder(id, StateFailed), &bl
	}
	r.mu.Lock()
	members, ok := r.sets[set]
	if !ok {
		r.mu.Unlock()
		cause := core.NotFound(op, set+"/"+string(id))
		bl := BrokenLink{Set: set, ID: id, Code: core.CodeNotFound, Cause: cause}
		r.appendBroken(bl)
		r.appendLog(ReloadLog{Set: set, ID: id, Code: core.CodeNotFound})
		return placeholder(id, StateMissing), &bl
	}
	want, ok := members[id]
	if !ok {
		r.mu.Unlock()
		cause := core.NotFound(op, set+"/"+string(id))
		bl := BrokenLink{Set: set, ID: id, Code: core.CodeNotFound, Cause: cause}
		r.appendBroken(bl)
		r.appendLog(ReloadLog{Set: set, ID: id, Code: core.CodeNotFound})
		return placeholder(id, StateMissing), &bl
	}
	live := r.live
	r.mu.Unlock()
	if live {
		if changed, reason := logicReason(want, kind, ver, deps); changed {
			cause := core.InvalidArg(op, set+"/"+string(id), errors.New("live: logic swap forbidden, use full package update: "+reason))
			bl := BrokenLink{Set: set, ID: id, Code: core.CodeInvalidArg, Cause: cause}
			r.appendBroken(bl)
			r.appendLog(ReloadLog{Set: set, ID: id, Bytes: len(data), Code: core.CodeInvalidArg})
			got := r.currentSnapshot(id)
			return got, &bl
		}
	}
	if err := checkDeps(deps, id); err != nil {
		code := core.CodeOf(err)
		bl := BrokenLink{Set: set, ID: id, Code: code, Cause: err}
		r.appendBroken(bl)
		r.appendLog(ReloadLog{Set: set, ID: id, Bytes: len(data), Code: code})
		return placeholder(id, StateFailed), &bl
	}
	if len(data) == 0 {
		cause := core.NotFound(op, set+"/"+string(id))
		bl := BrokenLink{Set: set, ID: id, Code: core.CodeNotFound, Cause: cause}
		r.appendBroken(bl)
		r.appendLog(ReloadLog{Set: set, ID: id, Code: core.CodeNotFound})
		if cur := r.currentSnapshot(id); cur.State() == StateReady {
			return cur, &bl
		}
		return placeholder(id, StateMissing), &bl
	}
	var good []byte
	var goodDeps []core.AssetID
	var goodKind Kind
	var goodVer core.Version
	hadGood := false
	if cur := r.currentSnapshot(id); cur.State() == StateReady {
		good, goodDeps, goodKind, goodVer = cur.Bytes(), cur.Deps(), cur.Kind(), cur.Version()
		hadGood = len(good) > 0
	}
	start := time.Now()
	got, err := r.mgr.Load(id, kind, ver, data, deps)
	elapsed := time.Since(start)
	if err != nil {
		code := core.CodeOf(err)
		if hadGood {
			if _, rerr := r.mgr.Load(id, goodKind, goodVer, good, goodDeps); rerr == nil {
				_ = r.mgr.Unload(id)
				if restored, werr := r.mgr.Wait(id); werr == nil {
					got = restored
				}
			}
		}
		bl := BrokenLink{Set: set, ID: id, Code: code, Cause: err}
		r.appendBroken(bl)
		r.appendLog(ReloadLog{Set: set, ID: id, Bytes: len(data), Code: code, ElapsedNs: elapsed.Nanoseconds()})
		return got, &bl
	}
	_ = r.mgr.Unload(id)
	r.appendLog(ReloadLog{Set: set, ID: id, Bytes: len(data), OK: true, ElapsedNs: elapsed.Nanoseconds()})
	return got, nil
}

// ReloadOneFile reloads one image inside set from path. Missing files
// report NotFound carrying set plus image and keep the last good.
func (r *SetReloader) ReloadOneFile(set string, id core.AssetID, path string, kind Kind, ver core.Version, deps []core.AssetID) (*Asset, error) {
	const op = "asset.ReloadOneFile"
	if r == nil || r.mgr == nil {
		return nil, core.InvalidArg(op, set+"/"+string(id))
	}
	if err := checkSetName(set); err != nil {
		return placeholder(id, StateEmpty), err
	}
	if err := checkID(id); err != nil {
		bl := BrokenLink{Set: set, ID: id, Code: core.CodeInvalidArg, Cause: err}
		r.appendBroken(bl)
		return placeholder(id, StateEmpty), &bl
	}
	if path == "" {
		cause := core.InvalidArg(op, set+"/"+string(id))
		bl := BrokenLink{Set: set, ID: id, Code: core.CodeInvalidArg, Cause: cause}
		r.appendBroken(bl)
		r.appendLog(ReloadLog{Set: set, ID: id, Code: core.CodeInvalidArg})
		return placeholder(id, StateFailed), &bl
	}
	if !kind.Valid() {
		cause := core.InvalidArg(op, set+"/"+string(id))
		bl := BrokenLink{Set: set, ID: id, Code: core.CodeInvalidArg, Cause: cause}
		r.appendBroken(bl)
		r.appendLog(ReloadLog{Set: set, ID: id, Code: core.CodeInvalidArg})
		return placeholder(id, StateFailed), &bl
	}
	r.mu.Lock()
	members, ok := r.sets[set]
	if !ok {
		r.mu.Unlock()
		cause := core.NotFound(op, set+"/"+string(id))
		bl := BrokenLink{Set: set, ID: id, Code: core.CodeNotFound, Cause: cause}
		r.appendBroken(bl)
		r.appendLog(ReloadLog{Set: set, ID: id, Code: core.CodeNotFound})
		return placeholder(id, StateMissing), &bl
	}
	want, ok := members[id]
	if !ok {
		r.mu.Unlock()
		cause := core.NotFound(op, set+"/"+string(id))
		bl := BrokenLink{Set: set, ID: id, Code: core.CodeNotFound, Cause: cause}
		r.appendBroken(bl)
		r.appendLog(ReloadLog{Set: set, ID: id, Code: core.CodeNotFound})
		return placeholder(id, StateMissing), &bl
	}
	live := r.live
	r.mu.Unlock()
	if live {
		if changed, reason := logicReason(want, kind, ver, deps); changed {
			cause := core.InvalidArg(op, set+"/"+string(id), errors.New("live: logic swap forbidden, use full package update: "+reason))
			bl := BrokenLink{Set: set, ID: id, Code: core.CodeInvalidArg, Cause: cause}
			r.appendBroken(bl)
			r.appendLog(ReloadLog{Set: set, ID: id, Code: core.CodeInvalidArg})
			return r.currentSnapshot(id), &bl
		}
	}
	var good []byte
	var goodDeps []core.AssetID
	var goodKind Kind
	var goodVer core.Version
	hadGood := false
	if cur := r.currentSnapshot(id); cur.State() == StateReady {
		good, goodDeps, goodKind, goodVer = cur.Bytes(), cur.Deps(), cur.Kind(), cur.Version()
		hadGood = len(good) > 0
	}
	start := time.Now()
	got, err := r.mgr.LoadFile(id, path, kind, ver, deps)
	elapsed := time.Since(start)
	if err != nil {
		code := core.CodeOf(err)
		if hadGood {
			if _, rerr := r.mgr.Load(id, goodKind, goodVer, good, goodDeps); rerr == nil {
				_ = r.mgr.Unload(id)
				if restored, werr := r.mgr.Wait(id); werr == nil {
					got = restored
				}
			}
		}
		bl := BrokenLink{Set: set, ID: id, Code: code, Cause: err}
		r.appendBroken(bl)
		r.appendLog(ReloadLog{Set: set, ID: id, Code: code, ElapsedNs: elapsed.Nanoseconds()})
		return got, &bl
	}
	_ = r.mgr.Unload(id)
	r.appendLog(ReloadLog{Set: set, ID: id, Bytes: got.Size(), OK: true, ElapsedNs: elapsed.Nanoseconds()})
	return got, nil
}

func (r *SetReloader) currentSnapshot(id core.AssetID) *Asset {
	if r == nil || r.mgr == nil {
		return placeholder(id, StateEmpty)
	}
	if got, err := r.mgr.Wait(id); err == nil {
		return got
	} else if got != nil {
		return got
	}
	return placeholder(id, StateEmpty)
}

// VerifyLinks checks every member of set plus its deps. Broken chains
// join the report slot with set, image, and the need that broke.
func (r *SetReloader) VerifyLinks(set string) []BrokenLink {
	const op = "asset.VerifyLinks"
	if r == nil || r.mgr == nil {
		return nil
	}
	r.mu.Lock()
	members, ok := r.sets[set]
	if !ok {
		r.mu.Unlock()
		return nil
	}
	ids := make([]core.AssetID, 0, len(members))
	for id := range members {
		ids = append(ids, id)
	}
	r.mu.Unlock()
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	var found []BrokenLink
	for _, id := range ids {
		if r.mgr.StateOf(id) != StateReady {
			_, perr := r.mgr.Poll(id)
			code := core.CodeOf(perr)
			if code == core.CodeUnknown {
				code = core.CodeNotFound
				perr = core.NotFound(op, set+"/"+string(id))
			}
			found = append(found, BrokenLink{Set: set, ID: id, Code: code, Cause: perr})
			continue
		}
		deps, has := r.mgr.Dependencies(id)
		if !has {
			continue
		}
		for _, need := range deps {
			if r.mgr.StateOf(need) != StateReady {
				cause := core.NotFound(op, set+"/"+string(id)+"->"+string(need))
				found = append(found, BrokenLink{Set: set, ID: id, Need: need, Code: core.CodeNotFound, Cause: cause})
			}
		}
	}
	for _, b := range found {
		r.appendBroken(b)
	}
	return found
}

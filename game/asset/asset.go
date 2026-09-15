package asset

import (
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/energye/gpui/game/core"
	"github.com/energye/gpui/game/tex"
)

// Frozen budgets. Beyond budget is OutOfMemory, never a guess.
const (
	// MaxAssets caps cached ids in one manager.
	MaxAssets = 1024
	// MaxBytes caps retained payload bytes in one manager.
	MaxBytes = 64 << 20
	// MaxAssetBytes caps one payload.
	MaxAssetBytes = 8 << 20
	// MaxDeps caps dependencies of one asset.
	MaxDeps = 16
	// MaxIDLen caps the id text length in bytes.
	MaxIDLen = 128
)

// CurrentVersion is the engine manifest version every Load writes.
// Bump Minor for additive fields, Major for a breaking shape.
var CurrentVersion = core.Version{Major: 1, Minor: 0}

// Kind names the frozen payload carrier.
type Kind int

const (
	// KindUnknown is the zero value: no carrier recognized.
	KindUnknown Kind = iota
	// KindRaw is opaque bytes without interior parsing.
	KindRaw
	// KindTextureKTX2 is KTX2 plus BC1 decoded through game/tex.
	KindTextureKTX2
)

var kindNames = []string{"unknown", "raw", "ktx2"}

// String returns the stable log name of k.
func (k Kind) String() string {
	if k >= KindUnknown && int(k) < len(kindNames) {
		return kindNames[int(k)]
	}
	return "unknown"
}

// Valid reports whether k is a loadable carrier.
func (k Kind) Valid() bool { return k == KindRaw || k == KindTextureKTX2 }

// ParseKind looks name up by frozen name. Empty is InvalidArg;
// unfrozen names (basis, spine, tmx, atlas, ...) are Unsupported.
func ParseKind(name string) (Kind, error) {
	const op = "asset.ParseKind"
	switch name {
	case "raw":
		return KindRaw, nil
	case "ktx2":
		return KindTextureKTX2, nil
	case "":
		return KindUnknown, core.InvalidArg(op, "kind")
	default:
		return KindUnknown, core.Unsupported(op, name)
	}
}

// State names the lifecycle of one id.
type State int

const (
	// StateEmpty means no record for the id.
	StateEmpty State = iota
	// StateLoading means a background Request is in flight.
	StateLoading
	// StateReady means payload bytes are cached.
	StateReady
	// StateMissing is the placeholder for a NotFound id or file.
	StateMissing
	// StateFailed is the placeholder for BadData/Unsupported/OutOfMemory/VersionMismatch.
	StateFailed
)

var stateNames = []string{"empty", "loading", "ready", "missing", "failed"}

// String returns the stable log name of s.
func (s State) String() string {
	if s >= StateEmpty && int(s) < len(stateNames) {
		return stateNames[int(s)]
	}
	return "empty"
}

// Stats reports manager counters. Count is Ready entries;
// TotalBytes sums their payload bytes.
type Stats struct {
	Count      int
	TotalBytes int64
	Loads      int64
	Unloads    int64
	Requests   int64
}

// Asset is one immutable snapshot. Methods on a nil asset report
// zero values, never panic.
type Asset struct {
	id      core.AssetID
	kind    Kind
	version core.Version
	hash    uint64
	deps    []core.AssetID
	state   State
	upload  int
	pixels  int
	data    []byte
}

// ID returns the asset id.
func (a *Asset) ID() core.AssetID {
	if a == nil {
		return ""
	}
	return a.id
}

// Kind returns the carrier.
func (a *Asset) Kind() Kind {
	if a == nil {
		return KindUnknown
	}
	return a.kind
}

// Version returns the stored version.
func (a *Asset) Version() core.Version {
	if a == nil {
		return core.Version{}
	}
	return a.version
}

// Hash returns the FNV-1a hash of the stored bytes.
func (a *Asset) Hash() uint64 {
	if a == nil {
		return 0
	}
	return a.hash
}

// Size returns the stored payload bytes.
func (a *Asset) Size() int {
	if a == nil {
		return 0
	}
	return len(a.data)
}

// Deps returns a copy of the dependencies.
func (a *Asset) Deps() []core.AssetID {
	if a == nil || len(a.deps) == 0 {
		return nil
	}
	return cloneIDs(a.deps)
}

// State returns the lifecycle state.
func (a *Asset) State() State {
	if a == nil {
		return StateEmpty
	}
	return a.state
}

// Bytes returns a copy of the payload bytes.
func (a *Asset) Bytes() []byte {
	if a == nil || len(a.data) == 0 {
		return nil
	}
	return cloneBytes(a.data)
}

// UploadSize returns the GPU upload bytes for KTX2, else 0.
func (a *Asset) UploadSize() int {
	if a == nil {
		return 0
	}
	return a.upload
}

// PixelSize returns the decoded RGBA8 bytes for KTX2, else 0.
func (a *Asset) PixelSize() int {
	if a == nil {
		return 0
	}
	return a.pixels
}

// Equal reports whether o holds bitwise the same snapshot.
func (a *Asset) Equal(o *Asset) bool {
	if a == nil || o == nil {
		return a == o
	}
	if a.id != o.id || a.kind != o.kind || a.version != o.version ||
		a.hash != o.hash || a.state != o.state ||
		a.upload != o.upload || a.pixels != o.pixels {
		return false
	}
	if len(a.data) != len(o.data) || len(a.deps) != len(o.deps) {
		return false
	}
	for i := range a.data {
		if a.data[i] != o.data[i] {
			return false
		}
	}
	for i := range a.deps {
		if a.deps[i] != o.deps[i] {
			return false
		}
	}
	return true
}

// record is the cached entry plus its failure cause.
type record struct {
	kind    Kind
	version core.Version
	hash    uint64
	data    []byte
	deps    []core.AssetID
	state   State
	failure error
	upload  int
	pixels  int
}

// Manager owns one asset ledger: cached bytes plus the shared
// core.Manager refcount ledger. A nil Manager never panics.
type Manager struct {
	mu         sync.Mutex
	ledger     *core.Manager
	recs       map[core.AssetID]*record
	stacks     map[core.AssetID][]*core.Handle
	totalBytes int64
	loads      int64
	unloads    int64
	requests   int64
}

// NewManager builds an empty ledger.
func NewManager() *Manager {
	return &Manager{
		ledger: core.NewManager(),
		recs:   map[core.AssetID]*record{},
		stacks: map[core.AssetID][]*core.Handle{},
	}
}

// HashBytes returns the FNV-1a 64 hash of data.
func HashBytes(data []byte) uint64 {
	h := fnv.New64a()
	_, _ = h.Write(data)
	return h.Sum64()
}

func cloneBytes(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

func cloneIDs(in []core.AssetID) []core.AssetID {
	if len(in) == 0 {
		return nil
	}
	out := make([]core.AssetID, len(in))
	copy(out, in)
	return out
}

func checkID(id core.AssetID) error {
	if id.Empty() {
		return core.InvalidArg("asset", "")
	}
	if len(string(id)) > MaxIDLen {
		return core.InvalidArg("asset", string(id))
	}
	return nil
}

func checkDeps(deps []core.AssetID, self core.AssetID) error {
	if len(deps) > MaxDeps {
		return core.OutOfMemory("asset", string(self))
	}
	for _, d := range deps {
		if err := checkID(d); err != nil {
			return err
		}
		if d == self {
			return core.InvalidArg("asset", string(self))
		}
	}
	return nil
}

func snapshot(id core.AssetID, r *record) *Asset {
	if r == nil {
		return &Asset{id: id, state: StateEmpty}
	}
	return &Asset{
		id:      id,
		kind:    r.kind,
		version: r.version,
		hash:    r.hash,
		deps:    cloneIDs(r.deps),
		state:   r.state,
		upload:  r.upload,
		pixels:  r.pixels,
		data:    cloneBytes(r.data),
	}
}

func placeholder(id core.AssetID, st State) *Asset {
	return &Asset{id: id, state: st}
}

// decodeLocked validates data for kind without touching the ledger.
// Callers hold no lock; tex decode runs outside the manager lock.
func decodePayload(kind Kind, data []byte) (upload, pixels int, err error) {
	if kind != KindTextureKTX2 {
		return 0, 0, nil
	}
	im, perr := tex.ParseKTX2(data)
	if perr != nil {
		return 0, 0, perr
	}
	return im.UploadSize(), im.PixelSize(), nil
}

// storeReady caches a validated payload and acquires one claim.
// Caller holds m.mu.
func (m *Manager) storeReady(id core.AssetID, kind Kind, ver core.Version, data []byte, deps []core.AssetID, upload, pixels int) *Asset {
	r, ok := m.recs[id]
	if !ok {
		if len(m.recs) >= MaxAssets {
			return nil
		}
		r = &record{}
		m.recs[id] = r
	} else if r.state == StateReady {
		m.totalBytes -= int64(len(r.data))
	}
	hash := HashBytes(data)
	r.kind = kind
	r.version = ver
	r.hash = hash
	r.data = cloneBytes(data)
	r.deps = cloneIDs(deps)
	r.state = StateReady
	r.failure = nil
	r.upload = upload
	r.pixels = pixels
	m.totalBytes += int64(len(r.data))
	m.loads++
	h, _ := m.ledger.Acquire(id)
	m.stacks[id] = append(m.stacks[id], h)
	return snapshot(id, r)
}

// storeFailure caches a failure placeholder without bytes.
// Caller holds m.mu.
func (m *Manager) storeFailure(id core.AssetID, kind Kind, st State, err error) {
	r, ok := m.recs[id]
	if !ok {
		if len(m.recs) >= MaxAssets {
			return
		}
		r = &record{}
		m.recs[id] = r
	} else if r.state == StateReady {
		m.totalBytes -= int64(len(r.data))
	}
	r.kind = kind
	r.data = nil
	r.deps = nil
	r.state = st
	r.failure = err
	r.upload = 0
	r.pixels = 0
	r.hash = 0
}

// Load caches data under id and acquires one claim. Empty id/data,
// unknown kind, bad deps, and oversize payloads fail without storing;
// version gaps fail as VersionMismatch; torn KTX2 reports the tex code.
// Missing ids are NotFound only through LoadFile/Poll, not Load.
func (m *Manager) Load(id core.AssetID, kind Kind, ver core.Version, data []byte, deps []core.AssetID) (*Asset, error) {
	const op = "asset.Load"
	if m == nil {
		return nil, core.InvalidArg(op, "manager")
	}
	if err := checkID(id); err != nil {
		return nil, err
	}
	if !kind.Valid() {
		return nil, core.InvalidArg(op, string(id))
	}
	if len(data) == 0 {
		return nil, core.InvalidArg(op, string(id))
	}
	if len(data) > MaxAssetBytes {
		return nil, core.OutOfMemory(op, string(id))
	}
	if err := checkDeps(deps, id); err != nil {
		return nil, err
	}
	if !ver.CompatibleWith(CurrentVersion) {
		m.mu.Lock()
		m.storeFailure(id, kind, StateFailed, core.VersionMismatch(op, string(id)))
		out := snapshot(id, m.recs[id])
		failure := m.recs[id].failure
		m.mu.Unlock()
		return out, failure
	}
	upload, pixels, derr := decodePayload(kind, data)
	if derr != nil {
		m.mu.Lock()
		st := StateFailed
		if core.CodeOf(derr) == core.CodeNotFound {
			st = StateMissing
		}
		m.storeFailure(id, kind, st, derr)
		out := snapshot(id, m.recs[id])
		m.mu.Unlock()
		return out, derr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.recs[id]; !ok && len(m.recs) >= MaxAssets {
		return placeholder(id, StateFailed), core.OutOfMemory(op, string(id))
	}
	if old, ok := m.recs[id]; !ok || old.state != StateReady {
		if m.totalBytes+int64(len(data)) > MaxBytes {
			m.storeFailure(id, kind, StateFailed, core.OutOfMemory(op, string(id)))
			return snapshot(id, m.recs[id]), m.recs[id].failure
		}
	} else if m.totalBytes-int64(len(old.data))+int64(len(data)) > MaxBytes {
		m.storeFailure(id, kind, StateFailed, core.OutOfMemory(op, string(id)))
		return snapshot(id, m.recs[id]), m.recs[id].failure
	}
	return m.storeReady(id, kind, ver, data, deps, upload, pixels), nil
}

// LoadFile reads path and caches it under id. Empty path is InvalidArg;
// OS misses are NotFound with a Missing placeholder.
func (m *Manager) LoadFile(id core.AssetID, path string, kind Kind, ver core.Version, deps []core.AssetID) (*Asset, error) {
	const op = "asset.LoadFile"
	if m == nil {
		return nil, core.InvalidArg(op, "manager")
	}
	if err := checkID(id); err != nil {
		return nil, err
	}
	if path == "" {
		return nil, core.InvalidArg(op, string(id))
	}
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		m.mu.Lock()
		m.storeFailure(id, kind, StateMissing, core.NotFound(op, string(id), err))
		out := snapshot(id, m.recs[id])
		failure := m.recs[id].failure
		m.mu.Unlock()
		return out, failure
	}
	return m.Load(id, kind, ver, raw, deps)
}

// Request starts a background load of data under id. The data is copied
// before the goroutine starts, so the caller may reuse it. Poll reports
// Loading until the worker stores Ready or Failed. Duplicate Requests
// while Loading are no-ops.
func (m *Manager) Request(id core.AssetID, kind Kind, ver core.Version, data []byte, deps []core.AssetID) error {
	const op = "asset.Request"
	if m == nil {
		return core.InvalidArg(op, "manager")
	}
	if err := checkID(id); err != nil {
		return err
	}
	if !kind.Valid() {
		return core.InvalidArg(op, string(id))
	}
	if len(data) == 0 {
		return core.InvalidArg(op, string(id))
	}
	if err := checkDeps(deps, id); err != nil {
		return err
	}
	payload := cloneBytes(data)
	depsCopy := cloneIDs(deps)
	m.mu.Lock()
	if r, ok := m.recs[id]; ok && r.state == StateLoading {
		m.mu.Unlock()
		return nil
	}
	if _, ok := m.recs[id]; !ok && len(m.recs) >= MaxAssets {
		m.mu.Unlock()
		return core.OutOfMemory(op, string(id))
	}
	r, ok := m.recs[id]
	if !ok {
		r = &record{}
		m.recs[id] = r
	} else if r.state == StateReady {
		m.totalBytes -= int64(len(r.data))
		r.data = nil
		r.deps = nil
		r.hash = 0
		r.upload = 0
		r.pixels = 0
	}
	r.kind = kind
	r.version = ver
	r.state = StateLoading
	r.failure = nil
	m.requests++
	m.mu.Unlock()
	go func() {
		if !ver.CompatibleWith(CurrentVersion) {
			m.mu.Lock()
			m.storeFailure(id, kind, StateFailed, core.VersionMismatch(op, string(id)))
			m.mu.Unlock()
			return
		}
		upload, pixels, derr := decodePayload(kind, payload)
		m.mu.Lock()
		defer m.mu.Unlock()
		if derr != nil {
			st := StateFailed
			if core.CodeOf(derr) == core.CodeNotFound {
				st = StateMissing
			}
			// Only overwrite our own flight; a newer Request wins.
			if cur, ok := m.recs[id]; ok && cur.state == StateLoading {
				m.storeFailure(id, kind, st, derr)
			}
			return
		}
		if cur, ok := m.recs[id]; !ok || cur.state != StateLoading {
			return
		}
		// Budget check against currently retained bytes.
		retained := m.totalBytes
		if retained+int64(len(payload)) > MaxBytes {
			m.storeFailure(id, kind, StateFailed, core.OutOfMemory(op, string(id)))
			return
		}
		if len(payload) > MaxAssetBytes {
			m.storeFailure(id, kind, StateFailed, core.OutOfMemory(op, string(id)))
			return
		}
		m.storeReady(id, kind, ver, payload, depsCopy, upload, pixels)
	}()
	return nil
}

// RequestFile starts a background load of path under id.
func (m *Manager) RequestFile(id core.AssetID, path string, kind Kind, ver core.Version, deps []core.AssetID) error {
	const op = "asset.RequestFile"
	if m == nil {
		return core.InvalidArg(op, "manager")
	}
	if err := checkID(id); err != nil {
		return err
	}
	if path == "" {
		return core.InvalidArg(op, string(id))
	}
	if !kind.Valid() {
		return core.InvalidArg(op, string(id))
	}
	if err := checkDeps(deps, id); err != nil {
		return err
	}
	depsCopy := cloneIDs(deps)
	m.mu.Lock()
	if r, ok := m.recs[id]; ok && r.state == StateLoading {
		m.mu.Unlock()
		return nil
	}
	if _, ok := m.recs[id]; !ok && len(m.recs) >= MaxAssets {
		m.mu.Unlock()
		return core.OutOfMemory(op, string(id))
	}
	r, ok := m.recs[id]
	if !ok {
		r = &record{}
		m.recs[id] = r
	} else if r.state == StateReady {
		m.totalBytes -= int64(len(r.data))
		r.data = nil
		r.deps = nil
		r.hash = 0
		r.upload = 0
		r.pixels = 0
	}
	r.kind = kind
	r.version = ver
	r.state = StateLoading
	r.failure = nil
	m.requests++
	m.mu.Unlock()
	go func() {
		raw, ferr := os.ReadFile(filepath.Clean(path))
		if ferr != nil {
			m.mu.Lock()
			if cur, ok := m.recs[id]; ok && cur.state == StateLoading {
				m.storeFailure(id, kind, StateMissing, core.NotFound(op, string(id), ferr))
			}
			m.mu.Unlock()
			return
		}
		if len(raw) == 0 {
			m.mu.Lock()
			if cur, ok := m.recs[id]; ok && cur.state == StateLoading {
				m.storeFailure(id, kind, StateFailed, core.InvalidArg(op, string(id)))
			}
			m.mu.Unlock()
			return
		}
		if len(raw) > MaxAssetBytes {
			m.mu.Lock()
			if cur, ok := m.recs[id]; ok && cur.state == StateLoading {
				m.storeFailure(id, kind, StateFailed, core.OutOfMemory(op, string(id)))
			}
			m.mu.Unlock()
			return
		}
		if !ver.CompatibleWith(CurrentVersion) {
			m.mu.Lock()
			if cur, ok := m.recs[id]; ok && cur.state == StateLoading {
				m.storeFailure(id, kind, StateFailed, core.VersionMismatch(op, string(id)))
			}
			m.mu.Unlock()
			return
		}
		upload, pixels, derr := decodePayload(kind, raw)
		m.mu.Lock()
		defer m.mu.Unlock()
		if cur, ok := m.recs[id]; !ok || cur.state != StateLoading {
			return
		}
		if derr != nil {
			st := StateFailed
			if core.CodeOf(derr) == core.CodeNotFound {
				st = StateMissing
			}
			m.storeFailure(id, kind, st, derr)
			return
		}
		if m.totalBytes+int64(len(raw)) > MaxBytes {
			m.storeFailure(id, kind, StateFailed, core.OutOfMemory(op, string(id)))
			return
		}
		m.storeReady(id, kind, ver, raw, depsCopy, upload, pixels)
	}()
	return nil
}

// Poll reports the lifecycle state of id without blocking. Unknown ids
// are StateEmpty plus NotFound; Loading and Ready report nil;
// Missing and Failed replay the stored cause.
func (m *Manager) Poll(id core.AssetID) (State, error) {
	const op = "asset.Poll"
	if m == nil {
		return StateEmpty, core.InvalidArg(op, "manager")
	}
	if err := checkID(id); err != nil {
		return StateEmpty, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.recs[id]
	if !ok {
		return StateEmpty, core.NotFound(op, string(id))
	}
	switch r.state {
	case StateLoading, StateReady:
		return r.state, nil
	case StateMissing, StateFailed:
		if r.failure != nil {
			return r.state, r.failure
		}
		return r.state, core.NotFound(op, string(id))
	default:
		return StateEmpty, core.NotFound(op, string(id))
	}
}

// Wait blocks until id leaves Loading, then returns its snapshot.
// Unknown ids return a NotFound error; Failed/Missing replay the cause.
// The wait caps at 5 seconds so a stuck worker cannot hang play.
func (m *Manager) Wait(id core.AssetID) (*Asset, error) {
	const op = "asset.Wait"
	if m == nil {
		return nil, core.InvalidArg(op, "manager")
	}
	if err := checkID(id); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		m.mu.Lock()
		r, ok := m.recs[id]
		if !ok {
			m.mu.Unlock()
			return placeholder(id, StateEmpty), core.NotFound(op, string(id))
		}
		switch r.state {
		case StateReady:
			out := snapshot(id, r)
			m.mu.Unlock()
			return out, nil
		case StateMissing, StateFailed:
			out := snapshot(id, r)
			failure := r.failure
			m.mu.Unlock()
			if failure == nil {
				return out, core.NotFound(op, string(id))
			}
			return out, failure
		case StateLoading:
			m.mu.Unlock()
		default:
			m.mu.Unlock()
			return placeholder(id, StateEmpty), core.NotFound(op, string(id))
		}
		if time.Now().After(deadline) {
			return placeholder(id, StateLoading), core.OutOfMemory(op, string(id))
		}
		time.Sleep(time.Millisecond)
	}
}

// Ref adds one claim on a Ready id and returns a fresh snapshot.
// Missing or failed ids return their placeholder plus the cause;
// over-budget manager growth is OutOfMemory.
func (m *Manager) Ref(id core.AssetID) (*Asset, error) {
	const op = "asset.Ref"
	if m == nil {
		return nil, core.InvalidArg(op, "manager")
	}
	if err := checkID(id); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.recs[id]
	if !ok {
		return placeholder(id, StateEmpty), core.NotFound(op, string(id))
	}
	switch r.state {
	case StateReady:
		h, err := m.ledger.Acquire(id)
		if err != nil {
			return snapshot(id, r), err
		}
		m.stacks[id] = append(m.stacks[id], h)
		return snapshot(id, r), nil
	case StateLoading:
		return snapshot(id, r), core.NotFound(op, string(id))
	case StateMissing, StateFailed:
		if r.failure != nil {
			return snapshot(id, r), r.failure
		}
		return snapshot(id, r), core.NotFound(op, string(id))
	default:
		return placeholder(id, StateEmpty), core.NotFound(op, string(id))
	}
}

// Unload drops one claim on id. Unknown ids are NotFound;
// ids with no live claims are InvalidArg and change nothing.
// Cached bytes stay for quick reload; use Evict to free them.
func (m *Manager) Unload(id core.AssetID) error {
	const op = "asset.Unload"
	if m == nil {
		return core.InvalidArg(op, "manager")
	}
	if err := checkID(id); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.recs[id]; !ok {
		return core.NotFound(op, string(id))
	}
	stack := m.stacks[id]
	if len(stack) == 0 {
		return core.InvalidArg(op, string(id))
	}
	top := stack[len(stack)-1]
	m.stacks[id] = stack[:len(stack)-1]
	m.unloads++
	return top.Release()
}

// Evict frees the cached bytes of id when it has no live claims.
// Live ids report InvalidArg; unknown ids report NotFound.
func (m *Manager) Evict(id core.AssetID) error {
	const op = "asset.Evict"
	if m == nil {
		return core.InvalidArg(op, "manager")
	}
	if err := checkID(id); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.recs[id]
	if !ok {
		return core.NotFound(op, string(id))
	}
	if len(m.stacks[id]) > 0 {
		return core.InvalidArg(op, string(id))
	}
	if r.state == StateReady {
		m.totalBytes -= int64(len(r.data))
	}
	delete(m.recs, id)
	delete(m.stacks, id)
	return nil
}

// Placeholder returns a usable empty snapshot for id. Missing assets
// use it so callers never branch on nil. Empty ids return nil.
func (m *Manager) Placeholder(id core.AssetID) *Asset {
	if m == nil || id.Empty() {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.recs[id]; ok {
		return snapshot(id, r)
	}
	return placeholder(id, StateMissing)
}

// Dependencies returns a copy of what id needs. Unknown ids report false.
func (m *Manager) Dependencies(id core.AssetID) ([]core.AssetID, bool) {
	if m == nil || id.Empty() {
		return nil, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.recs[id]
	if !ok || r.state != StateReady {
		return nil, false
	}
	return cloneIDs(r.deps), true
}

// Dependents returns every Ready id that lists id, sorted for replay.
func (m *Manager) Dependents(id core.AssetID) []core.AssetID {
	if m == nil || id.Empty() {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []core.AssetID
	for other, r := range m.recs {
		if r.state != StateReady {
			continue
		}
		for _, d := range r.deps {
			if d == id {
				out = append(out, other)
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// VersionOf returns the stored version of a Ready id.
func (m *Manager) VersionOf(id core.AssetID) (core.Version, bool) {
	if m == nil || id.Empty() {
		return core.Version{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.recs[id]
	if !ok || r.state != StateReady {
		return core.Version{}, false
	}
	return r.version, true
}

// HashOf returns the stored hash of a Ready id.
func (m *Manager) HashOf(id core.AssetID) (uint64, bool) {
	if m == nil || id.Empty() {
		return 0, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.recs[id]
	if !ok || r.state != StateReady {
		return 0, false
	}
	return r.hash, true
}

// StateOf returns the lifecycle state of id.
func (m *Manager) StateOf(id core.AssetID) State {
	if m == nil || id.Empty() {
		return StateEmpty
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.recs[id]; ok {
		return r.state
	}
	return StateEmpty
}

// LiveCount returns the live reference count of id.
func (m *Manager) LiveCount(id core.AssetID) int {
	if m == nil || id.Empty() {
		return 0
	}
	return m.ledger.LiveCount(id)
}

// Loaded reports whether id has live claims.
func (m *Manager) Loaded(id core.AssetID) bool {
	if m == nil || id.Empty() {
		return false
	}
	return m.ledger.Loaded(id)
}

// Ready reports whether id has cached bytes.
func (m *Manager) Ready(id core.AssetID) bool { return m.StateOf(id) == StateReady }

// Stats returns a copy of the manager counters.
func (m *Manager) Stats() Stats {
	if m == nil {
		return Stats{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, r := range m.recs {
		if r.state == StateReady {
			n++
		}
	}
	return Stats{
		Count:      n,
		TotalBytes: m.totalBytes,
		Loads:      m.loads,
		Unloads:    m.unloads,
		Requests:   m.requests,
	}
}

// Count returns the Ready entry count.
func (m *Manager) Count() int {
	if m == nil {
		return 0
	}
	return m.Stats().Count
}

// TotalBytes returns the retained payload bytes.
func (m *Manager) TotalBytes() int64 {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.totalBytes
}

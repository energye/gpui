package core

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// AssetID names a loadable asset (texture, map, skeleton, save, ...).
// Lowercase path style: "tex/hero", "map/level1", "bone/slime".
type AssetID string

// Empty reports whether id carries no name.
func (id AssetID) Empty() bool { return id == "" }

// String returns the raw id text.
func (id AssetID) String() string { return string(id) }

// Validate reports InvalidArg when id is empty.
func (id AssetID) Validate() error {
	if id.Empty() {
		return InvalidArg("core.AssetID", "")
	}
	return nil
}

// Handle is a reference-counted claim on an asset. Ref and Release must
// pair; releasing past zero is reported, never silent.
// Handle.refs counts this handle's claims; Manager.counts sums all handles
// for the same id. Ref/Release/Acquire update both sides together.
type Handle struct {
	m    *Manager
	id   AssetID
	refs int
	live bool
}

// ID returns the asset the handle claims.
func (h *Handle) ID() AssetID { return h.id }

// Refs returns the live reference count of the handle's asset.
func (h *Handle) Refs() int {
	if h == nil || h.m == nil {
		return 0
	}
	return h.m.refs(h.id)
}

// Ref adds one claim. A dead or nil handle returns false.
func (h *Handle) Ref() bool {
	if h == nil || h.m == nil || !h.live {
		return false
	}
	h.refs++
	h.m.add(h.id, 1)
	return true
}

// Release drops one claim. Pair with Ref. Releasing a nil, dead, or
// zero-ref handle reports an error instead of going negative.
func (h *Handle) Release() error {
	if h == nil || h.m == nil || !h.live {
		return InvalidArg("core.Handle.Release", "")
	}
	if h.refs <= 0 {
		return InvalidArg("core.Handle.Release", string(h.id))
	}
	h.refs--
	h.m.add(h.id, -1)
	if h.refs == 0 {
		h.live = false
	}
	return nil
}

// Manager owns the id->count ledger every asset pipeline shares, so
// textures, maps, and saves never each invent their own accounting.
type Manager struct {
	mu     sync.Mutex
	counts map[AssetID]int
}

// NewManager builds an empty ledger.
func NewManager() *Manager { return &Manager{counts: map[AssetID]int{}} }

// Acquire claims id (count 1) and returns the live handle. An empty id is
// rejected with InvalidArg.
func (m *Manager) Acquire(id AssetID) (*Handle, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counts[id]++
	return &Handle{m: m, id: id, refs: 1, live: true}, nil
}

// refs reports the live count of id (0 for unknown).
func (m *Manager) refs(id AssetID) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.counts[id]
}

// add shifts the count of id by delta, clamping at zero.
func (m *Manager) add(id AssetID, delta int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counts[id] += delta
	if m.counts[id] < 0 {
		m.counts[id] = 0
	}
}

// Loaded reports whether id currently has live claims.
func (m *Manager) Loaded(id AssetID) bool { return m.refs(id) > 0 }

// LiveCount reports the live reference count of id.
func (m *Manager) LiveCount(id AssetID) int { return m.refs(id) }

// Version is a major.minor data version carried by saves, maps, bones, and
// atlases. Major bumps break loading; minor bumps stay compatible.
type Version struct {
	Major, Minor int
}

// ParseVersion parses "major.minor" (minor optional). Anything else is a
// BadData error, never a guessed version.
func ParseVersion(s string) (Version, error) {
	parts := strings.Split(strings.TrimSpace(s), ".")
	if len(parts) == 0 || len(parts) > 2 {
		return Version{}, BadData("core.ParseVersion", s)
	}
	var v Version
	var err error
	v.Major, err = strconv.Atoi(parts[0])
	if err != nil || v.Major < 0 {
		return Version{}, BadData("core.ParseVersion", s)
	}
	if len(parts) == 2 {
		v.Minor, err = strconv.Atoi(parts[1])
		if err != nil || v.Minor < 0 {
			return Version{}, BadData("core.ParseVersion", s)
		}
	}
	return v, nil
}

// String renders "major.minor".
func (v Version) String() string { return fmt.Sprintf("%d.%d", v.Major, v.Minor) }

// CompatibleWith reports whether v (data) loads under cur (engine):
// same major, and data minor not newer than the engine's.
func (v Version) CompatibleWith(cur Version) bool {
	return v.Major == cur.Major && v.Minor <= cur.Minor
}

// CheckCompatibility errors with VersionMismatch when data v cannot load
// under engine cur.
func (v Version) CheckCompatibility(cur Version) error {
	if v.CompatibleWith(cur) {
		return nil
	}
	return VersionMismatch("core.Version.Check", v.String(),
		errors.New("requires engine "+cur.String()))
}

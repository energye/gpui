package tex

import (
	"math"
	"sync"

	"github.com/energye/gpui/game/core"
)

// Kind names one axis filter. Nearest keeps texels sharp, Linear smooths.
type Kind int

const (
	// KindUnknown is the zero value: no choice made, Normalized maps it
	// to KindLinear (the historic smooth default).
	KindUnknown Kind = iota
	// KindNearest samples the closest texel (near/crisp pictures).
	KindNearest
	// KindLinear blends neighbours (far/stable pictures).
	KindLinear
)

// String returns the stable log name of k.
func (k Kind) String() string {
	switch k {
	case KindNearest:
		return "nearest"
	case KindLinear:
		return "linear"
	default:
		return "unknown"
	}
}

// ParseKind maps "nearest"/"linear" to a Kind. Anything else is Unknown.
func ParseKind(s string) Kind {
	switch s {
	case "nearest":
		return KindNearest
	case "linear":
		return KindLinear
	default:
		return KindUnknown
	}
}

// MipMode names the between-level select. Nearest picks one level
// (stable, the historic default), Linear blends two (smoother far view).
type MipMode int

const (
	// MipUnknown is the zero value: Normalized maps it to MipNearest.
	MipUnknown MipMode = iota
	// MipNearest selects the closest level only.
	MipNearest
	// MipLinear blends adjacent levels (trilinear).
	MipLinear
)

// String returns the stable log name of m.
func (m MipMode) String() string {
	switch m {
	case MipNearest:
		return "nearest"
	case MipLinear:
		return "linear"
	default:
		return "unknown"
	}
}

// ParseMipMode maps "nearest"/"linear" to a MipMode, else MipUnknown.
func ParseMipMode(s string) MipMode {
	switch s {
	case "nearest":
		return MipNearest
	case "linear":
		return MipLinear
	default:
		return MipUnknown
	}
}

const (
	// AnisoMin disables extra filtering, AnisoMax is the device cap.
	AnisoMin = 1
	AnisoMax = 16
	// DefaultAniso is the historic sampler behaviour (off).
	DefaultAniso = 1
)

// Filter is one picture's sampling switch: Near is the magnify filter,
// Far the minify filter, Mip the between-level select, MaxAniso the
// anisotropy cap (1..16, 1 is off). The zero value is valid:
// Normalized maps it to DefaultFilter.
type Filter struct {
	Near     Kind
	Far      Kind
	Mip      MipMode
	MaxAniso uint16
}

// DefaultFilter matches the historic GPU linear sampler: smooth both
// axes, nearest level, anisotropy off.
func DefaultFilter() Filter {
	return Filter{Near: KindLinear, Far: KindLinear, Mip: MipNearest, MaxAniso: DefaultAniso}
}

// NearestFilter matches the historic GPU nearest sampler: sharp both
// axes. Mip and MaxAniso ride along but SamplerParams masks them,
// exactly like the GPU nearest path does.
func NearestFilter() Filter {
	return Filter{Near: KindNearest, Far: KindNearest, Mip: MipNearest, MaxAniso: DefaultAniso}
}

// validKind reports whether k is a settable axis choice.
func validKind(k Kind) bool { return k == KindNearest || k == KindLinear }

// validMip reports whether m is a settable level choice.
func validMip(m MipMode) bool { return m == MipNearest || m == MipLinear }

// normalizeAniso pins a to [AnisoMin, AnisoMax]; 0 means DefaultAniso.
func normalizeAniso(a uint16) uint16 {
	if a == 0 {
		return DefaultAniso
	}
	if a > AnisoMax {
		return AnisoMax
	}
	return a
}

// Normalized returns f with every Unknown axis mapped to the historic
// default (Linear axes, Nearest level) and aniso pinned to [1, 16].
// It never fails; use SetFilter/SetMipmap for loud validation.
func (f Filter) Normalized() Filter {
	if !validKind(f.Near) {
		f.Near = KindLinear
	}
	if !validKind(f.Far) {
		f.Far = KindLinear
	}
	if !validMip(f.Mip) {
		f.Mip = MipNearest
	}
	f.MaxAniso = normalizeAniso(f.MaxAniso)
	return f
}

// SetFilter switches both axes plus the anisotropy cap of this picture
// (the frozen 3.2 interface: near/far per picture, aniso tunable).
// An Unknown Mip rides along as MipNearest (the historic select), so a
// zero-value Filter becomes valid after one SetFilter call; an already
// set Mip is left untouched so old callers keep their level select.
// Bad input returns InvalidArg and leaves f unchanged.
func (f *Filter) SetFilter(near, far Kind, maxAniso int) error {
	const op = "tex.Filter.SetFilter"
	if f == nil {
		return core.InvalidArg(op, "filter")
	}
	if !validKind(near) || !validKind(far) {
		return core.InvalidArg(op, "kind")
	}
	if maxAniso < AnisoMin || maxAniso > AnisoMax {
		return core.InvalidArg(op, "aniso")
	}
	f.Near = near
	f.Far = far
	if !validMip(f.Mip) {
		f.Mip = MipNearest
	}
	f.MaxAniso = uint16(maxAniso) //nolint:gosec // range checked above
	return nil
}

// SetMipmap switches the between-level select of this picture (new
// branch for the far-stable view). Bad input returns InvalidArg and
// leaves f unchanged.
func (f *Filter) SetMipmap(m MipMode) error {
	const op = "tex.Filter.SetMipmap"
	if f == nil {
		return core.InvalidArg(op, "filter")
	}
	if !validMip(m) {
		return core.InvalidArg(op, "mip")
	}
	f.Mip = m
	return nil
}

// SamplerParams maps f to the effective GPU sampler choice at the render
// boundary: magLinear/minLinear/mipLinear plus the aniso cap. A fully
// nearest picture masks mip/aniso (the GPU nearest sampler ignores
// both), so two nearest pictures always merge even when their carried
// Mip/MaxAniso differ.
func (f Filter) SamplerParams() (magLinear, minLinear, mipLinear bool, aniso uint16) {
	n := f.Normalized()
	if n.Near == KindNearest && n.Far == KindNearest {
		return false, false, false, DefaultAniso
	}
	return n.Near == KindLinear, n.Far == KindLinear, n.Mip == MipLinear, n.MaxAniso
}

// NumLevels returns the mipmap level count for a w-by-h picture:
// 1 + floor(log2(max(w, h))). Non-positive sides hold 0 levels and
// never crash (tiny-picture guard).
func NumLevels(w, h int) int {
	if w <= 0 || h <= 0 {
		return 0
	}
	m := w
	if h > m {
		m = h
	}
	return 1 + int(math.Floor(math.Log2(float64(m))))
}

// LevelForScale returns the level index for scale (displayed/original):
// 1.0 or larger is level 0, each halving steps one deeper, clamped to
// the last level. NaN, infinities, zero, and negatives fall back to 0,
// mirroring the CPU chain clamps so both sides pick the same level.
func LevelForScale(scale float64, w, h int) int {
	n := NumLevels(w, h)
	if n <= 0 {
		return 0
	}
	if math.IsNaN(scale) || math.IsInf(scale, 0) || scale >= 1.0 || scale <= 0 {
		return 0
	}
	level := int(math.Floor(-math.Log2(scale)))
	if level < 0 {
		return 0
	}
	if level >= n {
		return n - 1
	}
	return level
}

// LevelSize returns the dimensions of level (0 is the original, each
// level halves, min 1). Out-of-range input reports ok=false, never a
// panic and never a zero size leaking out as valid.
func LevelSize(w, h, level int) (lw, lh int, ok bool) {
	n := NumLevels(w, h)
	if n <= 0 || level < 0 || level >= n {
		return 0, 0, false
	}
	lw = w >> level
	if lw < 1 {
		lw = 1
	}
	lh = h >> level
	if lh < 1 {
		lh = 1
	}
	return lw, lh, true
}

// Table is the per-picture filter switch: one Filter per AssetID.
// S32 wires it to the draw; R3 only freezes the switch and its math.
type Table struct {
	mu sync.RWMutex
	m  map[core.AssetID]Filter
}

// NewTable builds an empty per-picture switch.
func NewTable() *Table { return &Table{m: map[core.AssetID]Filter{}} }

// validateFilter rejects Unknown axes and out-of-range aniso with InvalidArg.
func validateFilter(op string, f Filter) error {
	if !validKind(f.Near) || !validKind(f.Far) {
		return core.InvalidArg(op, "kind")
	}
	if !validMip(f.Mip) {
		return core.InvalidArg(op, "mip")
	}
	if f.MaxAniso < AnisoMin || f.MaxAniso > AnisoMax {
		return core.InvalidArg(op, "aniso")
	}
	return nil
}

// Set records f for id. Empty ids and invalid filters return InvalidArg
// and store nothing.
func (t *Table) Set(id core.AssetID, f Filter) error {
	const op = "tex.Table.Set"
	if t == nil {
		return core.InvalidArg(op, "table")
	}
	if id.Empty() {
		return core.InvalidArg(op, "id")
	}
	if err := validateFilter(op, f); err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.m[id] = f
	return nil
}

// Get returns the Filter stored for id, or false when id is unknown.
func (t *Table) Get(id core.AssetID) (Filter, bool) {
	if t == nil {
		return Filter{}, false
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	f, ok := t.m[id]
	return f, ok
}

// Remove drops id, reporting whether anything was stored.
func (t *Table) Remove(id core.AssetID) bool {
	if t == nil {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.m[id]; !ok {
		return false
	}
	delete(t.m, id)
	return true
}

// Len returns the stored picture count.
func (t *Table) Len() int {
	if t == nil {
		return 0
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.m)
}

// Clear drops every stored picture.
func (t *Table) Clear() {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.m = map[core.AssetID]Filter{}
}

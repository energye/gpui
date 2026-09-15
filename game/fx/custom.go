package fx

import (
	"math"
	"sync"

	"github.com/energye/gpui/game/core"
)

// Frozen custom-material budgets and names (capability 5.5, P3, S44/W6).
const (
	// MaxCustomNameLen caps a material name in bytes.
	MaxCustomNameLen = 64
	// MaxCustomParams caps entries in Custom.Params.
	MaxCustomParams = 8
	// MaxCustomOutlineWidth caps outline width in pixels.
	MaxCustomOutlineWidth = 8
	// MaxCustomSeed caps the dissolve seed (uint32 range).
	MaxCustomSeed = 4294967295
	// MaxCustomSlots caps live Registry handles.
	MaxCustomSlots = 256
)

// Frozen material names. Unknown names are Unsupported, never a guess.
const (
	CustomIdentity = "identity"
	CustomOutline  = "outline"
	CustomDissolve = "dissolve"
)

// Custom is one artist material hook: a name plus float params.
// Only game-layer numbers are stored; this package draws nothing.
// The caller feeds Apply into the existing render draws.
type Custom struct {
	Name   string
	Params map[string]float64
}

// NewCustom builds a Custom copying params. Empty or overlong names are
// InvalidArg, unknown names Unsupported, bad params InvalidArg.
func NewCustom(name string, params map[string]float64) (Custom, error) {
	const op = "fx.NewCustom"
	c := Custom{Name: name, Params: cloneParams(params)}
	if err := c.Validate(); err != nil {
		_ = op
		return Custom{}, err
	}
	return c, nil
}

// NewIdentity builds the do-nothing hook (no params, never degraded).
func NewIdentity() Custom {
	return Custom{Name: CustomIdentity, Params: map[string]float64{}}
}

// NewOutline builds an outline hook: pixels below threshold whose square
// neighbour within width holds opaque become (r,g,b,1).
func NewOutline(width, threshold, r, g, b float64) (Custom, error) {
	return NewCustom(CustomOutline, map[string]float64{
		"width": width, "threshold": threshold, "r": r, "g": g, "b": b,
	})
}

// NewDissolve builds a dissolve hook: per-pixel hash above amount+edge
// keeps src, the edge band shows (r,g,b,1), below amount is transparent.
func NewDissolve(amount, edge, seed, r, g, b float64) (Custom, error) {
	return NewCustom(CustomDissolve, map[string]float64{
		"amount": amount, "edge": edge, "seed": seed, "r": r, "g": g, "b": b,
	})
}

func cloneParams(p map[string]float64) map[string]float64 {
	if p == nil {
		return nil
	}
	out := make(map[string]float64, len(p))
	for k, v := range p {
		out[k] = v
	}
	return out
}

func validUnit(x float64) bool { return finiteFloat(x) && x >= 0 && x <= 1 }

// Validate reports InvalidArg for bad names or params, Unsupported for an
// unknown material name.
func (c Custom) Validate() error {
	const op = "fx.Custom.Validate"
	if c.Name == "" || len(c.Name) > MaxCustomNameLen {
		return core.InvalidArg(op, "name")
	}
	if c.Params == nil {
		return core.InvalidArg(op, "params")
	}
	if len(c.Params) > MaxCustomParams {
		return core.InvalidArg(op, "params")
	}
	switch c.Name {
	case CustomIdentity:
		if len(c.Params) != 0 {
			return core.InvalidArg(op, "params")
		}
		return nil
	case CustomOutline:
		if len(c.Params) != 5 {
			return core.InvalidArg(op, "params")
		}
		w, ok := c.Params["width"]
		if !ok || !finiteFloat(w) || w < 0 || w > MaxCustomOutlineWidth {
			return core.InvalidArg(op, "width")
		}
		th, ok := c.Params["threshold"]
		if !ok || !validUnit(th) {
			return core.InvalidArg(op, "threshold")
		}
		for _, k := range []string{"r", "g", "b"} {
			v, ok := c.Params[k]
			if !ok || !validUnit(v) {
				return core.InvalidArg(op, k)
			}
		}
		return nil
	case CustomDissolve:
		if len(c.Params) != 6 {
			return core.InvalidArg(op, "params")
		}
		a, ok := c.Params["amount"]
		if !ok || !validUnit(a) {
			return core.InvalidArg(op, "amount")
		}
		e, ok := c.Params["edge"]
		if !ok || !finiteFloat(e) || e < 0 || e > 0.5 {
			return core.InvalidArg(op, "edge")
		}
		s, ok := c.Params["seed"]
		if !ok || !finiteFloat(s) || s < 0 || s > MaxCustomSeed || math.Trunc(s) != s {
			return core.InvalidArg(op, "seed")
		}
		for _, k := range []string{"r", "g", "b"} {
			v, ok := c.Params[k]
			if !ok || !validUnit(v) {
				return core.InvalidArg(op, k)
			}
		}
		return nil
	default:
		return core.Unsupported(op, c.Name)
	}
}

// IsIdentity reports whether c is the do-nothing hook.
func (c Custom) IsIdentity() bool { return c.Name == CustomIdentity }

// Degraded reports the frozen CPU fallback mark: identity is exact
// (false), outline and dissolve are reference CPU paths the GPU replaces
// with a filtered shader (true). Callers gate parity on this bit.
func (c Custom) Degraded() bool { return c.Name != CustomIdentity }

// Param returns one param value, or false for a missing key.
func (c Custom) Param(key string) (float64, bool) {
	if c.Params == nil {
		return 0, false
	}
	v, ok := c.Params[key]
	return v, ok
}

// SetParam swaps one param. Unknown keys or out-of-range values are
// InvalidArg and leave c unchanged.
func (c *Custom) SetParam(key string, v float64) error {
	const op = "fx.Custom.SetParam"
	if c == nil {
		return core.InvalidArg(op, "custom")
	}
	if c.Params == nil {
		return core.InvalidArg(op, "params")
	}
	old, ok := c.Params[key]
	if !ok {
		return core.InvalidArg(op, key)
	}
	c.Params[key] = v
	if err := c.Validate(); err != nil {
		c.Params[key] = old
		return err
	}
	return nil
}

// placeholder copies src for the bad-shader path so the main path keeps
// drawing. Nil or corrupt src yields nil (nothing to hold).
func placeholder(src *Image) *Image {
	if src == nil || !src.valid() {
		return nil
	}
	return src.Clone()
}

// Apply runs c on src and returns a fresh image plus the frozen degraded
// mark. src is never mutated. Nil src is InvalidArg, corrupt src BadData,
// over-budget OutOfMemory, an invalid hook InvalidArg, an unknown hook
// Unsupported; a valid src with a bad hook still returns a placeholder
// copy so the main path never breaks.
func (c Custom) Apply(src *Image) (*Image, bool, error) {
	const op = "fx.Custom.Apply"
	if src == nil {
		return nil, false, core.InvalidArg(op, "image")
	}
	if !src.valid() {
		return nil, false, core.BadData(op, "image")
	}
	if int64(src.W)*int64(src.H) > int64(MaxImagePixels) {
		return nil, false, core.OutOfMemory(op, "pixels")
	}
	if err := c.Validate(); err != nil {
		return placeholder(src), true, err
	}
	switch c.Name {
	case CustomIdentity:
		return src.Clone(), false, nil
	case CustomOutline:
		return applyOutline(src, c), true, nil
	case CustomDissolve:
		return applyDissolve(src, c), true, nil
	default:
		return placeholder(src), true, core.Unsupported(op, c.Name)
	}
}

// applyOutline paints the square-neighbour ring. Width rounds to the
// nearest pixel; width 0 copies src.
func applyOutline(src *Image, c Custom) *Image {
	radius := int(math.Round(c.Params["width"]))
	out := src.Clone()
	if radius <= 0 {
		return out
	}
	th := c.Params["threshold"]
	or, og, ob := c.Params["r"], c.Params["g"], c.Params["b"]
	for y := 0; y < src.H; y++ {
		for x := 0; x < src.W; x++ {
			if src.Pix[y*src.W+x].A >= th {
				continue
			}
			hit := false
			for dy := -radius; dy <= radius && !hit; dy++ {
				yy := y + dy
				if yy < 0 || yy >= src.H {
					continue
				}
				for dx := -radius; dx <= radius; dx++ {
					xx := x + dx
					if xx < 0 || xx >= src.W {
						continue
					}
					if src.Pix[yy*src.W+xx].A >= th {
						hit = true
						break
					}
				}
			}
			if hit {
				out.Pix[y*src.W+x] = core.RGBA(or, og, ob, 1)
			}
		}
	}
	return out
}

// customHash01 maps one pixel to [0,1): splitmix64 over (x,y,seed).
func customHash01(x, y int, seed uint64) float64 {
	h := uint64(x)*0x9E3779B97F4A7C15 ^ uint64(y)*0xBF58476D1CE4E5B9 ^ seed*0x94D049BB133111EB
	h += 0x9E3779B97F4A7C15
	z := h
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	z ^= z >> 31
	return float64(z>>11) / (1 << 53)
}

// applyDissolve keeps, edges, or discards each pixel by its hash.
func applyDissolve(src *Image, c Custom) *Image {
	out := src.Clone()
	amount := c.Params["amount"]
	edge := c.Params["edge"]
	seed := uint64(c.Params["seed"])
	er, eg, eb := c.Params["r"], c.Params["g"], c.Params["b"]
	for y := 0; y < src.H; y++ {
		for x := 0; x < src.W; x++ {
			n := customHash01(x, y, seed)
			switch {
			case n > amount+edge:
				// Keep src.
			case n > amount:
				out.Pix[y*src.W+x] = core.RGBA(er, eg, eb, 1)
			default:
				out.Pix[y*src.W+x] = core.RGBA(0, 0, 0, 0)
			}
		}
	}
	return out
}

// Registry holds attached hooks by handle. Attach copies in, Detach
// deletes, Apply calls by handle. Zero value is unusable: use NewRegistry.
type Registry struct {
	mu    sync.Mutex
	next  int
	slots map[int]Custom
}

// NewRegistry builds an empty hook table.
func NewRegistry() *Registry {
	return &Registry{slots: make(map[int]Custom)}
}

// Len returns the live handle count.
func (r *Registry) Len() int {
	if r == nil || r.slots == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.slots)
}

// Attach validates c and returns a fresh handle. Bad hooks are InvalidArg
// or Unsupported and attach nothing; a full table is OutOfMemory.
func (r *Registry) Attach(c Custom) (int, error) {
	const op = "fx.Registry.Attach"
	if r == nil || r.slots == nil {
		return 0, core.InvalidArg(op, "registry")
	}
	if err := c.Validate(); err != nil {
		return 0, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.slots) >= MaxCustomSlots {
		return 0, core.OutOfMemory(op, "slots")
	}
	r.next++
	id := r.next
	cp := Custom{Name: c.Name, Params: cloneParams(c.Params)}
	r.slots[id] = cp
	return id, nil
}

// Detach removes id. Non-positive ids are InvalidArg, missing ids NotFound.
func (r *Registry) Detach(id int) error {
	const op = "fx.Registry.Detach"
	if r == nil || r.slots == nil {
		return core.InvalidArg(op, "registry")
	}
	if id <= 0 {
		return core.InvalidArg(op, "handle")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.slots[id]; !ok {
		return core.NotFound(op, "handle")
	}
	delete(r.slots, id)
	return nil
}

// Get returns a copy of the hook at id, or false for a missing handle.
func (r *Registry) Get(id int) (Custom, bool) {
	if r == nil || r.slots == nil || id <= 0 {
		return Custom{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.slots[id]
	if !ok {
		return Custom{}, false
	}
	return Custom{Name: c.Name, Params: cloneParams(c.Params)}, true
}

// Apply calls the hook at id on src. A bad src reports the image error,
// a missing handle NotFound with a placeholder copy when src is valid,
// an invalid stored hook its own error; valid src always yields a drawable
// placeholder so the main path never breaks.
func (r *Registry) Apply(id int, src *Image) (*Image, bool, error) {
	const op = "fx.Registry.Apply"
	if src == nil {
		return nil, false, core.InvalidArg(op, "image")
	}
	if !src.valid() {
		return nil, false, core.BadData(op, "image")
	}
	if r == nil || r.slots == nil {
		return placeholder(src), true, core.InvalidArg(op, "registry")
	}
	if id <= 0 {
		return placeholder(src), true, core.InvalidArg(op, "handle")
	}
	r.mu.Lock()
	c, ok := r.slots[id]
	r.mu.Unlock()
	if !ok {
		return placeholder(src), true, core.NotFound(op, "handle")
	}
	return c.Apply(src)
}

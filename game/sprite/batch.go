package sprite

import (
	"math"

	"github.com/energye/gpui/game/core"
)

// Sprite is one sub-rect of a large image: which atlas it comes from,
// the source block in image pixels, the destination box in world units,
// and the opacity. Tag is a debug key for tests and logs only.
type Sprite struct {
	Image   core.AssetID
	Src     core.Rect
	Dst     core.Rect
	Opacity float64
	Tag     string
}

func finiteFloat(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

func finiteRect(r core.Rect) bool {
	return finiteFloat(r.X) && finiteFloat(r.Y) && finiteFloat(r.W) && finiteFloat(r.H)
}

// Skippable reports whether Flush skips s without drawing, matching the
// existing render DrawAtlas skip: empty source or zero destination size.
func Skippable(s Sprite) bool {
	return s.Src.W <= 0 || s.Src.H <= 0 || s.Dst.W == 0 || s.Dst.H == 0
}

// EffectiveOpacity maps o to the value the existing render draw uses:
// <=0 defaults to 1, >1 clamps to 1.
func EffectiveOpacity(o float64) float64 {
	if o <= 0 {
		return 1
	}
	if o > 1 {
		return 1
	}
	return o
}

// NewSprite builds a Sprite. Image must be non-empty, Src/Dst coordinates
// and Opacity must be finite. Zero-size rects are allowed here and skipped
// later by Add/Flush, mirroring the render skip.
func NewSprite(image core.AssetID, src, dst core.Rect, opacity float64) (Sprite, error) {
	const op = "sprite.NewSprite"
	if image.Empty() {
		return Sprite{}, core.InvalidArg(op, "image")
	}
	if !finiteRect(src) {
		return Sprite{}, core.InvalidArg(op, "src")
	}
	if !finiteRect(dst) {
		return Sprite{}, core.InvalidArg(op, "dst")
	}
	if !finiteFloat(opacity) {
		return Sprite{}, core.InvalidArg(op, "opacity")
	}
	return Sprite{Image: image, Src: src, Dst: dst, Opacity: opacity}, nil
}

// Batch accumulates sprites and flushes one draw per image: same atlas
// submits once, cutting draw calls. Pure numbers only; the caller draws
// each flushed group with the existing render DrawAtlas in order.
// Not safe for concurrent use.
type Batch struct {
	items   []Sprite
	skipped int
}

// NewBatch builds an empty batch.
func NewBatch() *Batch { return &Batch{} }

// Add stores s. Empty image or non-finite numbers return core InvalidArg
// and store nothing. Zero-size sprites (see Skippable) are skipped quietly:
// they return added=false with a nil error and are never stored, exactly
// like the render draw skips them.
func (b *Batch) Add(s Sprite) (added bool, err error) {
	const op = "sprite.Batch.Add"
	if b == nil {
		return false, core.InvalidArg(op, "batch")
	}
	if s.Image.Empty() {
		return false, core.InvalidArg(op, "image")
	}
	if !finiteRect(s.Src) {
		return false, core.InvalidArg(op, "src")
	}
	if !finiteRect(s.Dst) {
		return false, core.InvalidArg(op, "dst")
	}
	if !finiteFloat(s.Opacity) {
		return false, core.InvalidArg(op, "opacity")
	}
	if Skippable(s) {
		b.skipped++
		return false, nil
	}
	b.items = append(b.items, s)
	return true, nil
}

// Len returns the stored sprite count (skipped sprites are not stored).
func (b *Batch) Len() int {
	if b == nil {
		return 0
	}
	return len(b.items)
}

// Skipped returns how many zero-size sprites Add has skipped.
func (b *Batch) Skipped() int {
	if b == nil {
		return 0
	}
	return b.skipped
}

// Clear drops every stored sprite and resets the skip count.
// The backing array is retained so a reused Batch does not reallocate
// every frame; Len reports 0 either way.
func (b *Batch) Clear() {
	if b == nil {
		return
	}
	b.items = b.items[:0]
	b.skipped = 0
}

// Flush groups stored sprites by image in first-seen order, keeps the Add
// order inside each group, calls emit once per image, drains the batch,
// and returns the draw call count (distinct images). Empty batches call
// nothing and return 0. A nil emit is a quiet no-op returning 0 with the
// batch preserved. Emit must not call back into the same Batch.
func (b *Batch) Flush(emit func(id core.AssetID, sprites []Sprite)) int {
	if b == nil || len(b.items) == 0 {
		return 0
	}
	if emit == nil {
		return 0
	}
	order := make([]core.AssetID, 0, 4)
	groups := make(map[core.AssetID][]Sprite, 4)
	// Fast path: every stored sprite shares one image (the sprite-window
	// hot loop). One emit, no map, same order and count as the grouped road.
	single := true
	first := b.items[0].Image
	for i := 1; i < len(b.items); i++ {
		if b.items[i].Image != first {
			single = false
			break
		}
	}
	if single {
		cp := make([]Sprite, len(b.items))
		copy(cp, b.items)
		emit(first, cp)
		b.items = b.items[:0]
		b.skipped = 0
		return 1
	}
	for _, s := range b.items {
		if _, ok := groups[s.Image]; !ok {
			order = append(order, s.Image)
		}
		groups[s.Image] = append(groups[s.Image], s)
	}
	for _, id := range order {
		cp := make([]Sprite, len(groups[id]))
		copy(cp, groups[id])
		emit(id, cp)
	}
	n := len(order)
	b.items = b.items[:0]
	b.skipped = 0
	return n
}

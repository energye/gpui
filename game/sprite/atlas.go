package sprite

import (
	"math"

	"github.com/energye/gpui/game/core"
	"github.com/energye/gpui/render"
)

// AtlasFilter selects per-sprite sampling: near stays crisp, far stays
// stable, each picture on its own. Zero means the historic smooth road.
type AtlasFilter int

const (
	// AtlasFilterDefault is the zero value: historic bilinear road.
	AtlasFilterDefault AtlasFilter = 0
	// AtlasFilterNearest keeps texels sharp (near/crisp pictures).
	AtlasFilterNearest AtlasFilter = 1
	// AtlasFilterBilinear blends neighbours (far/stable pictures).
	AtlasFilterBilinear AtlasFilter = 2
	// AtlasFilterBicubic uses a 4x4 neighbourhood (highest quality).
	AtlasFilterBicubic AtlasFilter = 3
)

// String returns the stable log name of f.
func (f AtlasFilter) String() string {
	switch f {
	case AtlasFilterDefault:
		return "default"
	case AtlasFilterNearest:
		return "nearest"
	case AtlasFilterBilinear:
		return "bilinear"
	case AtlasFilterBicubic:
		return "bicubic"
	default:
		return "unknown"
	}
}

// Valid reports whether f is a settable choice.
func (f AtlasFilter) Valid() bool {
	return f == AtlasFilterDefault || f == AtlasFilterNearest ||
		f == AtlasFilterBilinear || f == AtlasFilterBicubic
}

// ParseAtlasFilter maps "", "default", "nearest", "bilinear", "bicubic"
// to a filter. Anything else is a core InvalidArg error, never a guess.
func ParseAtlasFilter(s string) (AtlasFilter, error) {
	switch s {
	case "", "default":
		return AtlasFilterDefault, nil
	case "nearest":
		return AtlasFilterNearest, nil
	case "bilinear":
		return AtlasFilterBilinear, nil
	case "bicubic":
		return AtlasFilterBicubic, nil
	default:
		return AtlasFilterDefault, core.InvalidArg("sprite.ParseAtlasFilter", s)
	}
}

// ToRender maps f to the render sampler at the boundary. Default maps to
// the zero mode (render normalizes it to bilinear, the old road).
func (f AtlasFilter) ToRender() (render.InterpolationMode, error) {
	switch f {
	case AtlasFilterDefault:
		return render.InterpolationMode(0), nil
	case AtlasFilterNearest:
		return render.InterpNearest, nil
	case AtlasFilterBilinear:
		return render.InterpBilinear, nil
	case AtlasFilterBicubic:
		return render.InterpBicubic, nil
	default:
		return render.InterpBilinear, core.InvalidArg("sprite.AtlasFilter.ToRender", f.String())
	}
}

// AtlasSprite is one sub-rect of a large image with rotation, tint, flip,
// pivot, and per-sprite filter: which atlas it comes from, the source
// block in image pixels, the destination box in world units, the opacity,
// the rotation about the pivot, the mirror before rotation, the pivot
// offset from the Dst origin, the multiply tint, and the sampling.
// Tag is a debug key for tests and logs only.
type AtlasSprite struct {
	Image   core.AssetID
	Src     core.Rect
	Dst     core.Rect
	Opacity float64
	Rot     float64
	FlipX   bool
	FlipY   bool
	Pivot   core.Vec2
	Tint    core.Color
	Filter  AtlasFilter
	Tag     string
}

func finiteAtlasFloat(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

func finiteAtlasRect(r core.Rect) bool {
	return finiteAtlasFloat(r.X) && finiteAtlasFloat(r.Y) &&
		finiteAtlasFloat(r.W) && finiteAtlasFloat(r.H)
}

func finiteAtlasColor(c core.Color) bool {
	return finiteAtlasFloat(c.R) && finiteAtlasFloat(c.G) &&
		finiteAtlasFloat(c.B) && finiteAtlasFloat(c.A)
}

// NewAtlasSprite builds an AtlasSprite. Image must be non-empty, Src/Dst
// coordinates, Opacity, Rot, Pivot, and Tint components must be finite,
// Filter must be a settable choice. Zero-size rects are allowed here and
// skipped later by the render draw, mirroring the R4 skip. Tag stays empty;
// the caller sets it for debugging.
func NewAtlasSprite(image core.AssetID, src, dst core.Rect, opacity, rot float64, pivot core.Vec2, tint core.Color, filter AtlasFilter, flipX, flipY bool) (AtlasSprite, error) {
	const op = "sprite.NewAtlasSprite"
	if image.Empty() {
		return AtlasSprite{}, core.InvalidArg(op, "image")
	}
	if !finiteAtlasRect(src) {
		return AtlasSprite{}, core.InvalidArg(op, "src")
	}
	if !finiteAtlasRect(dst) {
		return AtlasSprite{}, core.InvalidArg(op, "dst")
	}
	if !finiteAtlasFloat(opacity) {
		return AtlasSprite{}, core.InvalidArg(op, "opacity")
	}
	if !finiteAtlasFloat(rot) {
		return AtlasSprite{}, core.InvalidArg(op, "rot")
	}
	if !finiteAtlasFloat(pivot.X) || !finiteAtlasFloat(pivot.Y) {
		return AtlasSprite{}, core.InvalidArg(op, "pivot")
	}
	if !finiteAtlasColor(tint) {
		return AtlasSprite{}, core.InvalidArg(op, "tint")
	}
	if !filter.Valid() {
		return AtlasSprite{}, core.InvalidArg(op, "filter")
	}
	return AtlasSprite{Image: image, Src: src, Dst: dst, Opacity: opacity, Rot: rot, Pivot: pivot, Tint: tint, Filter: filter, FlipX: flipX, FlipY: flipY}, nil
}

// Validate reports whether s is drawable (finite numbers, known filter,
// non-empty image). Zero-size rects are valid here; they are skipped by
// Skippable and by the render draw, never an error.
func (s AtlasSprite) Validate() error {
	const op = "sprite.AtlasSprite.Validate"
	if s.Image.Empty() {
		return core.InvalidArg(op, "image")
	}
	if !finiteAtlasRect(s.Src) {
		return core.InvalidArg(op, "src")
	}
	if !finiteAtlasRect(s.Dst) {
		return core.InvalidArg(op, "dst")
	}
	if !finiteAtlasFloat(s.Opacity) {
		return core.InvalidArg(op, "opacity")
	}
	if !finiteAtlasFloat(s.Rot) {
		return core.InvalidArg(op, "rot")
	}
	if !finiteAtlasFloat(s.Pivot.X) || !finiteAtlasFloat(s.Pivot.Y) {
		return core.InvalidArg(op, "pivot")
	}
	if !finiteAtlasColor(s.Tint) {
		return core.InvalidArg(op, "tint")
	}
	if !s.Filter.Valid() {
		return core.InvalidArg(op, "filter")
	}
	return nil
}

// Skippable reports whether the render draw skips s without drawing,
// matching the R4 skip: empty source or zero destination size. Negative
// destination size is kept (mirrored draw at the render edge).
func (s AtlasSprite) Skippable() bool {
	return s.Src.W <= 0 || s.Src.H <= 0 || s.Dst.W == 0 || s.Dst.H == 0
}

// HasTint reports whether s carries an explicit tint. The zero color
// means no tint (white opaque at the render edge); any other value,
// including explicit white, multiplies the texels.
func (s AtlasSprite) HasTint() bool { return s.Tint != (core.Color{}) }

// PivotPoint returns the absolute pivot in world units: Dst origin plus
// the pivot offset. Rotating about the feet keeps this point fixed, so a
// turning body stays glued at the feet.
func (s AtlasSprite) PivotPoint() core.Vec2 {
	return core.V2(s.Dst.X+s.Pivot.X, s.Dst.Y+s.Pivot.Y)
}

// FeetPivot returns the pivot offset that pins the rotation center at the
// feet: bottom-edge center of dst in Dst units. Use it as Pivot so a turn
// never lifts the feet.
func FeetPivot(dst core.Rect) core.Vec2 { return core.V2(dst.W/2, dst.H) }

// ToRender converts s to the render draw at the boundary, field for field.
// Bad sprites return a core InvalidArg error and no guess; skippable
// sprites convert fine and are skipped later by DrawAtlasEx.
func (s AtlasSprite) ToRender() (render.AtlasSprite, error) {
	if err := s.Validate(); err != nil {
		return render.AtlasSprite{}, err
	}
	filt, err := s.Filter.ToRender()
	if err != nil {
		return render.AtlasSprite{}, err
	}
	return render.AtlasSprite{
		SrcX: s.Src.X, SrcY: s.Src.Y, SrcW: s.Src.W, SrcH: s.Src.H,
		DstX: s.Dst.X, DstY: s.Dst.Y, DstW: s.Dst.W, DstH: s.Dst.H,
		Opacity: s.Opacity, Rot: s.Rot, FlipX: s.FlipX, FlipY: s.FlipY,
		PivotX: s.Pivot.X, PivotY: s.Pivot.Y,
		Tint:   s.Tint.ToRender(),
		Filter: filt,
	}, nil
}

// AtlasToRender converts items to the render draws at the boundary,
// preserving order and count (lossless): the caller draws the result with
// the existing DrawAtlasEx, old road untouched. The input slice is never
// mutated; the result is a fresh slice. Empty input returns nil with nil
// error. The first bad sprite aborts with its InvalidArg error and nil
// result, never a partial draw.
func AtlasToRender(items []AtlasSprite) ([]render.AtlasSprite, error) {
	if len(items) == 0 {
		return nil, nil
	}
	out := make([]render.AtlasSprite, len(items))
	for i := range items {
		rs, err := items[i].ToRender()
		if err != nil {
			return nil, err
		}
		out[i] = rs
	}
	return out, nil
}

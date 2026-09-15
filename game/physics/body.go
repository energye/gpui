package physics

import (
	"math"

	"github.com/energye/gpui/game/core"
)

// Shape names which geometry a Body carries.
type Shape int

const (
	// ShapeBox is a center plus half extents.
	ShapeBox Shape = 0
	// ShapeCircle is a center plus a radius.
	ShapeCircle Shape = 1
)

// Body is one collision volume: shape plus center plus layer masks plus
// the trigger flag. Name is only a debug key for tests and logs, never
// an asset id; it may be empty. Half belongs to boxes, Radius to
// circles; the unused side stays zero.
type Body struct {
	Name    string
	Shape   Shape
	Pos     core.Vec2
	Half    core.Vec2
	Radius  float64
	Layer   uint32
	Mask    uint32
	Trigger bool
}

func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

func finiteVec(v core.Vec2) bool { return finite(v.X) && finite(v.Y) }

func checkPos(op string, pos core.Vec2) error {
	if !finiteVec(pos) {
		return core.InvalidArg(op, "pos")
	}
	return nil
}

// NewBox builds a box body centered at pos with half extents half.
// Negative or non-finite half, or a non-finite pos, is a core InvalidArg
// error and stores nothing. Layer/Mask accept any bits (0 hits nothing).
func NewBox(name string, pos, half core.Vec2, layer, mask uint32, trigger bool) (Body, error) {
	if err := checkPos("physics.NewBox", pos); err != nil {
		return Body{}, err
	}
	if !finiteVec(half) || half.X < 0 || half.Y < 0 {
		return Body{}, core.InvalidArg("physics.NewBox", "half")
	}
	return Body{Name: name, Shape: ShapeBox, Pos: pos, Half: half, Layer: layer, Mask: mask, Trigger: trigger}, nil
}

// NewCircle builds a circle body centered at pos with radius.
// Negative or non-finite radius, or a non-finite pos, is a core
// InvalidArg error and stores nothing.
func NewCircle(name string, pos core.Vec2, radius float64, layer, mask uint32, trigger bool) (Body, error) {
	if err := checkPos("physics.NewCircle", pos); err != nil {
		return Body{}, err
	}
	if !finite(radius) || radius < 0 {
		return Body{}, core.InvalidArg("physics.NewCircle", "radius")
	}
	return Body{Name: name, Shape: ShapeCircle, Pos: pos, Radius: radius, Layer: layer, Mask: mask, Trigger: trigger}, nil
}

// Valid reports whether b carries finite numbers and a known shape with
// a legal size. Direct struct writes bypass the constructors, so every
// query checks this first and fails closed.
func (b Body) Valid() bool {
	if !finiteVec(b.Pos) {
		return false
	}
	switch b.Shape {
	case ShapeBox:
		return finiteVec(b.Half) && b.Half.X >= 0 && b.Half.Y >= 0
	case ShapeCircle:
		return finite(b.Radius) && b.Radius >= 0
	default:
		return false
	}
}

// Bounds returns the axis-aligned box holding b, or ok=false for an
// invalid body. Zero-size bodies report their empty rect with ok=true;
// emptiness never means invalid here.
func Bounds(b Body) (core.Rect, bool) {
	if !b.Valid() {
		return core.Rect{}, false
	}
	if b.Shape == ShapeCircle {
		return core.NewRect(b.Pos.X-b.Radius, b.Pos.Y-b.Radius, 2*b.Radius, 2*b.Radius), true
	}
	return core.NewRect(b.Pos.X-b.Half.X, b.Pos.Y-b.Half.Y, 2*b.Half.X, 2*b.Half.Y), true
}

// CanCollide reports whether a scans b and b scans a through the layer
// masks. Pure bit math; validity never matters here. Query combines this
// with Overlaps, so masked-out pairs never meet even when they overlap.
func CanCollide(a, b Body) bool {
	return a.Layer&b.Mask != 0 && b.Layer&a.Mask != 0
}

// Overlaps reports whether a and b touch or interpenetrate, edge touch
// included. Invalid bodies never overlap (false, never a panic).
func Overlaps(a, b Body) bool {
	if !a.Valid() || !b.Valid() {
		return false
	}
	// Box-circle runs box-first: swap a circle-first pair into place.
	if a.Shape == ShapeCircle && b.Shape == ShapeBox {
		a, b = b, a
	}
	switch {
	case a.Shape == ShapeBox && b.Shape == ShapeBox:
		return overlapBoxBox(a, b)
	case a.Shape == ShapeCircle && b.Shape == ShapeCircle:
		return overlapCircleCircle(a, b)
	case a.Shape == ShapeBox && b.Shape == ShapeCircle:
		return overlapBoxCircle(a, b)
	default:
		return false
	}
}

func overlapBoxBox(a, b Body) bool {
	dx := math.Abs(a.Pos.X - b.Pos.X)
	dy := math.Abs(a.Pos.Y - b.Pos.Y)
	return dx <= a.Half.X+b.Half.X && dy <= a.Half.Y+b.Half.Y
}

func overlapCircleCircle(a, b Body) bool {
	dx := a.Pos.X - b.Pos.X
	dy := a.Pos.Y - b.Pos.Y
	sum := a.Radius + b.Radius
	return dx*dx+dy*dy <= sum*sum
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func overlapBoxCircle(box, c Body) bool {
	qx := clamp(c.Pos.X, box.Pos.X-box.Half.X, box.Pos.X+box.Half.X)
	qy := clamp(c.Pos.Y, box.Pos.Y-box.Half.Y, box.Pos.Y+box.Half.Y)
	dx := c.Pos.X - qx
	dy := c.Pos.Y - qy
	return dx*dx+dy*dy <= c.Radius*c.Radius
}

// Contact is one touching pair from Query: input indices plus debug
// names plus the trigger flag. Trigger is true when either side is a
// trigger; solid pairs report false. Indices disambiguate duplicate or
// empty names; names are never parsed.
type Contact struct {
	AI      int
	BI      int
	A       string
	B       string
	Trigger bool
}

// Query returns every touching pair with matching masks: CanCollide and
// Overlaps both true. Pairs run i<j in input order. The input is never
// mutated; the result is fresh (nil when nothing touches). Any invalid
// body is a core InvalidArg error with a nil result.
func Query(bodies []Body) ([]Contact, error) {
	for i := range bodies {
		if !bodies[i].Valid() {
			return nil, core.InvalidArg("physics.Query", "bodies")
		}
	}
	var out []Contact
	for i := 0; i < len(bodies); i++ {
		for j := i + 1; j < len(bodies); j++ {
			a, b := bodies[i], bodies[j]
			if CanCollide(a, b) && Overlaps(a, b) {
				out = append(out, Contact{AI: i, BI: j, A: a.Name, B: b.Name, Trigger: a.Trigger || b.Trigger})
			}
		}
	}
	return out, nil
}

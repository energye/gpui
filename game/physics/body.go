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

// standingSnap is the contact tolerance for IsStandingOn only. Overlaps
// keeps its exact edge rule; standing allows this snap so a rider carried
// by identical float deltas stays grounded across steps.
const standingSnap = 1e-9

// Ray is one jump probe or bullet line: origin plus direction plus range
// plus a one-sided layer mask. Dir need not be unit; length never scales
// the hit distance. MaxDist bounds the hit in world units.
type Ray struct {
	Origin  core.Vec2
	Dir     core.Vec2
	MaxDist float64
	Mask    uint32
}

// NewRay builds a ray. Origin must be finite, Dir finite and non-zero,
// MaxDist finite and >= 0. Mask accepts any bits (0 hits nothing).
func NewRay(origin, dir core.Vec2, maxDist float64, mask uint32) (Ray, error) {
	if !finiteVec(origin) {
		return Ray{}, core.InvalidArg("physics.NewRay", "origin")
	}
	if !finiteVec(dir) || dir.IsZero() {
		return Ray{}, core.InvalidArg("physics.NewRay", "dir")
	}
	if !finite(maxDist) || maxDist < 0 {
		return Ray{}, core.InvalidArg("physics.NewRay", "maxdist")
	}
	return Ray{Origin: origin, Dir: dir, MaxDist: maxDist, Mask: mask}, nil
}

// Valid reports whether r can cast: finite origin, finite non-zero dir,
// finite MaxDist >= 0. Mask never affects validity.
func (r Ray) Valid() bool {
	return finiteVec(r.Origin) && finiteVec(r.Dir) && !r.Dir.IsZero() &&
		finite(r.MaxDist) && r.MaxDist >= 0
}

// Hit is the nearest body a ray meets: input index plus debug name plus
// distance from Origin along the normalized Dir plus hit point plus the
// outward face normal plus the body trigger flag. Dist is in world units.
type Hit struct {
	Index   int
	Name    string
	Dist    float64
	Pos     core.Vec2
	Normal  core.Vec2
	Trigger bool
}

// rayDirNorm returns the unit direction without overflowing huge inputs.
func rayDirNorm(d core.Vec2) core.Vec2 {
	l := math.Hypot(d.X, d.Y)
	if l == 0 || !finite(l) {
		return core.Vec2{}
	}
	return core.V2(d.X/l, d.Y/l)
}

// rayHitBox meets an axis box with the slab test. Edge touch counts;
// origin inside reports t=0 with a zero normal; corner reports t with a
// zero normal; otherwise the normal is the entry face axis.
func rayHitBox(origin, dir core.Vec2, maxDist float64, b Body) (float64, core.Vec2, bool) {
	minX := b.Pos.X - b.Half.X
	maxX := b.Pos.X + b.Half.X
	minY := b.Pos.Y - b.Half.Y
	maxY := b.Pos.Y + b.Half.Y
	var txmin, txmax float64
	var nx core.Vec2
	if dir.X == 0 {
		if origin.X < minX || origin.X > maxX {
			return 0, core.Vec2{}, false
		}
		txmin, txmax = math.Inf(-1), math.Inf(1)
	} else {
		t1 := (minX - origin.X) / dir.X
		t2 := (maxX - origin.X) / dir.X
		if t1 < t2 {
			txmin, txmax = t1, t2
		} else {
			txmin, txmax = t2, t1
		}
		if dir.X > 0 {
			nx = core.V2(-1, 0)
		} else {
			nx = core.V2(1, 0)
		}
		if minX == maxX {
			nx = core.Vec2{}
		}
	}
	var tymin, tymax float64
	var ny core.Vec2
	if dir.Y == 0 {
		if origin.Y < minY || origin.Y > maxY {
			return 0, core.Vec2{}, false
		}
		tymin, tymax = math.Inf(-1), math.Inf(1)
	} else {
		t1 := (minY - origin.Y) / dir.Y
		t2 := (maxY - origin.Y) / dir.Y
		if t1 < t2 {
			tymin, tymax = t1, t2
		} else {
			tymin, tymax = t2, t1
		}
		if dir.Y > 0 {
			ny = core.V2(0, -1)
		} else {
			ny = core.V2(0, 1)
		}
		if minY == maxY {
			ny = core.Vec2{}
		}
	}
	tmin := math.Max(txmin, tymin)
	tmax := math.Min(txmax, tymax)
	if tmax < tmin || tmax < 0 || tmin > maxDist {
		return 0, core.Vec2{}, false
	}
	if tmin < 0 {
		return 0, core.Vec2{}, true
	}
	var n core.Vec2
	switch {
	case txmin > tymin:
		n = nx
	case tymin > txmin:
		n = ny
	default:
		n = core.Vec2{}
	}
	return tmin, n, true
}

// rayHitCircle meets a disc. Boundary start reports t=0 with a zero
// normal; tangent counts; otherwise the normal runs center to hit point.
func rayHitCircle(origin, dir core.Vec2, maxDist float64, b Body) (float64, core.Vec2, bool) {
	lx := b.Pos.X - origin.X
	ly := b.Pos.Y - origin.Y
	l2 := lx*lx + ly*ly
	r2 := b.Radius * b.Radius
	if l2 <= r2 {
		return 0, core.Vec2{}, true
	}
	tca := lx*dir.X + ly*dir.Y
	if tca < 0 {
		return 0, core.Vec2{}, false
	}
	d2 := l2 - tca*tca
	if d2 > r2 {
		return 0, core.Vec2{}, false
	}
	t := tca - math.Sqrt(r2-d2)
	if t < 0 || t > maxDist {
		return 0, core.Vec2{}, false
	}
	hx := origin.X + dir.X*t - b.Pos.X
	hy := origin.Y + dir.Y*t - b.Pos.Y
	hl := math.Hypot(hx, hy)
	if hl == 0 || !finite(hl) {
		return t, core.Vec2{}, true
	}
	return t, core.V2(hx/hl, hy/hl), true
}

// CastRay returns the nearest body the ray meets. Bodies with no layer
// bit in the ray mask are skipped (one-sided, unlike Query which needs
// both sides). Ties keep the smaller input index. Empty input reports no
// hit. An invalid ray or any invalid body is a core InvalidArg error.
func CastRay(bodies []Body, ray Ray) (Hit, bool, error) {
	if !ray.Valid() {
		return Hit{}, false, core.InvalidArg("physics.CastRay", "ray")
	}
	for i := range bodies {
		if !bodies[i].Valid() {
			return Hit{}, false, core.InvalidArg("physics.CastRay", "bodies")
		}
	}
	if len(bodies) == 0 || ray.Mask == 0 {
		return Hit{}, false, nil
	}
	dir := rayDirNorm(ray.Dir)
	if dir.IsZero() {
		return Hit{}, false, core.InvalidArg("physics.CastRay", "ray")
	}
	best := math.Inf(1)
	bestIdx := -1
	var bestN core.Vec2
	for i := range bodies {
		if bodies[i].Layer&ray.Mask == 0 {
			continue
		}
		var t float64
		var n core.Vec2
		var ok bool
		if bodies[i].Shape == ShapeCircle {
			t, n, ok = rayHitCircle(ray.Origin, dir, ray.MaxDist, bodies[i])
		} else {
			t, n, ok = rayHitBox(ray.Origin, dir, ray.MaxDist, bodies[i])
		}
		if !ok || !finite(t) || t < 0 || t > ray.MaxDist {
			continue
		}
		if t < best {
			best, bestIdx, bestN = t, i, n
		}
	}
	if bestIdx < 0 {
		return Hit{}, false, nil
	}
	return Hit{
		Index:   bestIdx,
		Name:    bodies[bestIdx].Name,
		Dist:    best,
		Pos:     core.V2(ray.Origin.X+dir.X*best, ray.Origin.Y+dir.Y*best),
		Normal:  bestN,
		Trigger: bodies[bestIdx].Trigger,
	}, true, nil
}

// IsStandingOn reports whether rider stands on top of platform: both
// valid, rider center at or above the platform center, rider bottom
// within standingSnap of the platform top, horizontal spans overlapping
// edge-included. Circles use their bounds footprint. Invalid inputs
// report false, never a panic.
func IsStandingOn(rider, platform Body) bool {
	if !rider.Valid() || !platform.Valid() {
		return false
	}
	if rider.Pos.Y < platform.Pos.Y {
		return false
	}
	rb, ok := Bounds(rider)
	if !ok {
		return false
	}
	pb, ok := Bounds(platform)
	if !ok {
		return false
	}
	if math.Abs(rb.Y-(pb.Y+pb.H)) > standingSnap {
		return false
	}
	return rb.X <= pb.X+pb.W && pb.X <= rb.X+rb.W
}

// CarryRider moves rider by the platform step delta when rider stands on
// platform before the move. Standing is IsStandingOn on the pre-move
// pair. Off-platform reports (false, nil) with rider untouched. A nil
// rider, invalid bodies, a non-finite delta, or a delta pushing rider
// off finite numbers is a core InvalidArg error with rider untouched.
func CarryRider(rider *Body, platform Body, delta core.Vec2) (bool, error) {
	if rider == nil {
		return false, core.InvalidArg("physics.CarryRider", "rider")
	}
	if !rider.Valid() || !platform.Valid() {
		return false, core.InvalidArg("physics.CarryRider", "body")
	}
	if !finiteVec(delta) {
		return false, core.InvalidArg("physics.CarryRider", "delta")
	}
	if !IsStandingOn(*rider, platform) {
		return false, nil
	}
	next := rider.Pos.Add(delta)
	if !finiteVec(next) {
		return false, core.InvalidArg("physics.CarryRider", "delta")
	}
	rider.Pos = next
	return true, nil
}

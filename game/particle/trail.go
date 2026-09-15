package particle

import (
	"math"

	"github.com/energye/gpui/game/core"
	"github.com/energye/gpui/game/sprite"
)

// JointKind names the corner rule between two trail segments (main
// reference Godot Line2D joints: sharp, cut, or round corners).
type JointKind int

const (
	// JointMiter extends the corner to a sharp point. Past MiterLimit the
	// corner falls back to a bevel cut so spikes never explode.
	JointMiter JointKind = 0
	// JointBevel cuts the corner flat with one vertex pair.
	JointBevel JointKind = 1
	// JointRound fans RoundArcSteps pairs around the corner.
	JointRound JointKind = 2
)

// MiterLimit bounds the miter extension in half-widths. Sharper corners
// degrade to bevel (same rule family as Godot Line2D antialiased joints).
const MiterLimit = 4.0

// RoundArcSteps is the fan size per round corner (interior pairs only,
// endpoints are never duplicated).
const RoundArcSteps = 4

// MaxTrailPointsCap bounds one trail ring buffer (D thousands plus
// headroom, shared with the pool budget below).
const MaxTrailPointsCap = 512

// String returns the stable log name of k.
func (k JointKind) String() string {
	switch k {
	case JointMiter:
		return "miter"
	case JointBevel:
		return "bevel"
	case JointRound:
		return "round"
	default:
		return "unknown"
	}
}

// ParseJoint maps "miter"/"bevel"/"round" to a kind, else JointMiter with
// ok=false (never guessed).
func ParseJoint(s string) (JointKind, bool) {
	switch s {
	case "miter":
		return JointMiter, true
	case "bevel":
		return JointBevel, true
	case "round":
		return JointRound, true
	default:
		return JointMiter, false
	}
}

// TrailConfig is the frozen trail recipe: MaxPoints caps the ring buffer,
// HeadWidth/TailWidth set the knife taper (head is the newest point),
// Start/End set the head-to-tail color ramp, Joint sets the corner rule.
// Only core numbers are used.
type TrailConfig struct {
	MaxPoints int
	HeadWidth float64
	TailWidth float64
	Start     core.Color
	End       core.Color
	Joint     JointKind
}

// NewTrailConfig builds a trail recipe: MaxPoints in [1, MaxTrailPointsCap],
// widths finite and >= 0, colors finite, joint known. Anything else is
// InvalidArg.
func NewTrailConfig(maxPoints int, headWidth, tailWidth float64, start, end core.Color, joint JointKind) (TrailConfig, error) {
	const op = "particle.NewTrailConfig"
	if maxPoints < 1 || maxPoints > MaxTrailPointsCap {
		return TrailConfig{}, core.InvalidArg(op, "max_points")
	}
	if !finiteFloat(headWidth) || headWidth < 0 || !finiteFloat(tailWidth) || tailWidth < 0 {
		return TrailConfig{}, core.InvalidArg(op, "width")
	}
	if !finiteColor(start) || !finiteColor(end) {
		return TrailConfig{}, core.InvalidArg(op, "color")
	}
	switch joint {
	case JointMiter, JointBevel, JointRound:
	default:
		return TrailConfig{}, core.InvalidArg(op, "joint")
	}
	return TrailConfig{MaxPoints: maxPoints, HeadWidth: headWidth, TailWidth: tailWidth, Start: start, End: end, Joint: joint}, nil
}

// Validate reports whether c is a constructor-built recipe.
func (c TrailConfig) Validate() error {
	if _, err := NewTrailConfig(c.MaxPoints, c.HeadWidth, c.TailWidth, c.Start, c.End, c.Joint); err != nil {
		return err
	}
	return nil
}

// Trail is one trajectory ribbon: points run oldest (tail) to newest
// (head). The ring drops the oldest point past MaxPoints. Width lerps
// TailWidth to HeadWidth along the same order, color lerps Start (head)
// to End (tail). Not safe for concurrent use.
type Trail struct {
	cfg TrailConfig
	pts []core.Vec2
}

// NewTrail builds an empty trail from cfg. Bad recipes return InvalidArg
// and nil.
func NewTrail(cfg TrailConfig) (*Trail, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Trail{cfg: cfg}, nil
}

// Config returns a copy of the trail recipe.
func (t *Trail) Config() TrailConfig {
	if t == nil {
		return TrailConfig{}
	}
	return t.cfg
}

// Len returns the stored point count.
func (t *Trail) Len() int {
	if t == nil {
		return 0
	}
	return len(t.pts)
}

// Points returns a fresh copy oldest to newest; writing it cannot alias
// the trail.
func (t *Trail) Points() []core.Vec2 {
	if t == nil || len(t.pts) == 0 {
		return nil
	}
	out := make([]core.Vec2, len(t.pts))
	copy(out, t.pts)
	return out
}

// Clear drops every stored point. Config is kept.
func (t *Trail) Clear() {
	if t == nil {
		return
	}
	t.pts = nil
}

// Push appends p as the newest point, dropping the oldest past MaxPoints.
// Non-finite p returns InvalidArg and stores nothing. Nil is InvalidArg.
func (t *Trail) Push(p core.Vec2) error {
	const op = "particle.Trail.Push"
	if t == nil {
		return core.InvalidArg(op, "trail")
	}
	if !finiteVec(p) {
		return core.InvalidArg(op, "point")
	}
	t.pts = append(t.pts, p)
	if len(t.pts) > t.cfg.MaxPoints {
		copy(t.pts, t.pts[len(t.pts)-t.cfg.MaxPoints:])
		t.pts = t.pts[:t.cfg.MaxPoints]
	}
	return nil
}

// fracOf maps raw index i to the head-based fraction: head (newest) is 0,
// tail (oldest) is 1. A lone point parks at the head.
func (t *Trail) fracOf(i int) float64 {
	n := len(t.pts)
	if n <= 1 {
		return 0
	}
	return 1 - float64(i)/float64(n-1)
}

// WidthAt returns the ribbon width at raw index i (oldest 0). Empty trail
// and bad indexes are InvalidArg.
func (t *Trail) WidthAt(i int) (float64, error) {
	const op = "particle.Trail.WidthAt"
	if t == nil || len(t.pts) == 0 {
		return 0, core.InvalidArg(op, "trail")
	}
	if i < 0 || i >= len(t.pts) {
		return 0, core.InvalidArg(op, "index")
	}
	f := 1 - t.fracOf(i)
	return t.cfg.TailWidth + (t.cfg.HeadWidth-t.cfg.TailWidth)*f, nil
}

// ColorAt returns the ramp color at raw index i: Start at the head, End
// at the tail. Empty trail and bad indexes are InvalidArg.
func (t *Trail) ColorAt(i int) (core.Color, error) {
	const op = "particle.Trail.ColorAt"
	if t == nil || len(t.pts) == 0 {
		return core.Color{}, core.InvalidArg(op, "trail")
	}
	if i < 0 || i >= len(t.pts) {
		return core.Color{}, core.InvalidArg(op, "index")
	}
	return t.cfg.Start.Lerp(t.cfg.End, t.fracOf(i)), nil
}

// TrailVert is one ribbon vertex pair: Center plus the Left/Right edge at
// half width along the joint normal. Round corners fan extra pairs with
// the same Center.
type TrailVert struct {
	Center core.Vec2
	Left   core.Vec2
	Right  core.Vec2
}

// segDir returns the unit direction from a to b, or zero when the points
// coincide (callers fall back to a neighbor direction, then +X).
func segDir(a, b core.Vec2) core.Vec2 {
	d := b.Sub(a)
	if d.IsZero() {
		return core.Vec2{}
	}
	return d.Normalize()
}

// perpOf rotates d 90 degrees counter-clockwise (the ribbon normal).
func perpOf(d core.Vec2) core.Vec2 { return core.V2(-d.Y, d.X) }

// jointPair resolves the edge offset at an interior point: n0/n1 are the
// adjacent unit normals, halfW the half width. It returns the miter
// direction and the extension length in half-widths before clamping.
func jointPair(n0, n1 core.Vec2) (dir core.Vec2, ext float64) {
	m := n0.Add(n1)
	if m.IsZero() {
		// U-turn: normals cancel, fall back to the incoming normal.
		return n0, 1
	}
	m = m.Normalize()
	cos := m.Dot(n0)
	if cos <= 0 {
		return n0, 1
	}
	return m, 1 / cos
}

// Verts expands the ribbon to vertex pairs oldest to newest. Empty trails
// return nil with nil error (skip-friendly: an empty track draws nothing).
// Miter corners extend up to MiterLimit then cut to bevel; bevel corners
// emit one pair; round corners fan RoundArcSteps interior pairs. A lone
// point parks facing +X. Duplicate neighbors reuse the neighbor direction
// so degenerate tracks never produce NaN.
func (t *Trail) Verts() ([]TrailVert, error) {
	if t == nil {
		return nil, core.InvalidArg("particle.Trail.Verts", "trail")
	}
	n := len(t.pts)
	if n == 0 {
		return nil, nil
	}
	width := func(i int) float64 {
		w, _ := t.WidthAt(i)
		return w
	}
	if n == 1 {
		h := width(0) / 2
		c := t.pts[0]
		return []TrailVert{{Center: c, Left: core.V2(c.X, c.Y+h), Right: core.V2(c.X, c.Y-h)}}, nil
	}
	dirs := make([]core.Vec2, n-1)
	for i := range dirs {
		dirs[i] = segDir(t.pts[i], t.pts[i+1])
	}
	dirAt := func(i int) core.Vec2 {
		// Endpoint and interior direction with degenerate fallback.
		var d core.Vec2
		switch {
		case i <= 0:
			d = dirs[0]
		case i >= n-1:
			d = dirs[n-2]
		default:
			d = dirs[i-1].Add(dirs[i])
			if d.IsZero() {
				d = dirs[i]
			} else {
				d = d.Normalize()
			}
		}
		if d.IsZero() {
			for _, c := range dirs {
				if !c.IsZero() {
					return c
				}
			}
			return core.V2(1, 0)
		}
		return d
	}
	var out []TrailVert
	emit := func(center, normal core.Vec2, halfW float64) {
		out = append(out, TrailVert{
			Center: center,
			Left:   core.V2(center.X+normal.X*halfW, center.Y+normal.Y*halfW),
			Right:  core.V2(center.X-normal.X*halfW, center.Y-normal.Y*halfW),
		})
	}
	for i := 0; i < n; i++ {
		c, halfW := t.pts[i], width(i)/2
		interior := i > 0 && i < n-1 && !dirs[i-1].IsZero() && !dirs[i].IsZero() &&
			!dirs[i-1].ApproxEqual(dirs[i], 1e-12)
		if !interior {
			emit(c, perpOf(dirAt(i)), halfW)
			continue
		}
		n0, n1 := perpOf(dirs[i-1]), perpOf(dirs[i])
		m, ext := jointPair(n0, n1)
		switch t.cfg.Joint {
		case JointRound:
			// Fan from the incoming normal to the outgoing one through
			// the miter middle; interior points only, no duplicates.
			a0 := math.Atan2(n0.Y, n0.X)
			a1 := math.Atan2(n1.Y, n1.X)
			for a1 <= a0 {
				a1 += 2 * math.Pi
			}
			if a1-a0 > math.Pi {
				a1 -= 2 * math.Pi
			}
			for k := 0; k <= RoundArcSteps+1; k++ {
				a := a0 + (a1-a0)*float64(k)/float64(RoundArcSteps+1)
				emit(c, core.V2(math.Cos(a), math.Sin(a)), halfW)
			}
		case JointBevel:
			emit(c, m, halfW)
		default: // JointMiter with bevel fallback past the limit.
			if ext > MiterLimit {
				emit(c, m, halfW)
			} else {
				emit(c, m, halfW*ext)
			}
		}
	}
	return out, nil
}

// TrailSegment is one drawable center-line piece for the existing atlas
// draw: P0/P1 centers, Angle in radians, endpoint widths, endpoint
// colors. The window turns each segment into one rotated atlas sprite
// plus one joint square, so no new submit path is needed.
type TrailSegment struct {
	P0    core.Vec2
	P1    core.Vec2
	Angle float64
	W0    float64
	W1    float64
	C0    core.Color
	C1    core.Color
}

// Segments returns the center-line pieces oldest to newest. Empty and
// single-point trails return nil with nil error (nothing to draw).
func (t *Trail) Segments() ([]TrailSegment, error) {
	if t == nil {
		return nil, core.InvalidArg("particle.Trail.Segments", "trail")
	}
	n := len(t.pts)
	if n < 2 {
		return nil, nil
	}
	out := make([]TrailSegment, 0, n-1)
	for i := 0; i+1 < n; i++ {
		w0, _ := t.WidthAt(i)
		w1, _ := t.WidthAt(i + 1)
		c0, _ := t.ColorAt(i)
		c1, _ := t.ColorAt(i + 1)
		d := t.pts[i+1].Sub(t.pts[i])
		out = append(out, TrailSegment{
			P0: t.pts[i], P1: t.pts[i+1],
			Angle: math.Atan2(d.Y, d.X),
			W0:    w0, W1: w1, C0: c0, C1: c1,
		})
	}
	return out, nil
}

// AppendToBatch appends one square sprite per trail point centered at the
// point with the WidthAt size and the ColorAt alpha, mirroring the
// Emitter skip rules: zero-alpha and non-finite entries are skipped, bad
// arguments return InvalidArg and append nothing.
func (t *Trail) AppendToBatch(b *sprite.Batch, image core.AssetID, src core.Rect) (int, error) {
	const op = "particle.Trail.AppendToBatch"
	if t == nil {
		return 0, core.InvalidArg(op, "trail")
	}
	if b == nil {
		return 0, core.InvalidArg(op, "batch")
	}
	if image.Empty() {
		return 0, core.InvalidArg(op, "image")
	}
	if !finiteFloat(src.X) || !finiteFloat(src.Y) || !finiteFloat(src.W) || !finiteFloat(src.H) {
		return 0, core.InvalidArg(op, "src")
	}
	n := 0
	for i := range t.pts {
		p := t.pts[i]
		if !finiteVec(p) {
			continue
		}
		w, err := t.WidthAt(i)
		if err != nil || !finiteFloat(w) || w <= 0 {
			continue
		}
		c, err := t.ColorAt(i)
		if err != nil || c.A <= 0 {
			continue
		}
		dst := core.NewRect(p.X-w/2, p.Y-w/2, w, w)
		s, err := sprite.NewSprite(image, src, dst, c.A)
		if err != nil {
			continue
		}
		if ok, err := b.Add(s); err != nil || !ok {
			continue
		}
		n++
	}
	return n, nil
}

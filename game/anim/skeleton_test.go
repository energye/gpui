package anim

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/game/core"
)

const epsSkeleton = 1e-9

type skeletonBoneWant struct {
	Name  string    `json:"name"`
	World []float64 `json:"world"`
	Mat   []float64 `json:"mat"`
}

type skeletonCases struct {
	SkeletonFile string `json:"skeleton_file"`
	Version      struct {
		Major int `json:"major"`
		Minor int `json:"minor"`
	} `json:"version"`
	Skin      string             `json:"skin"`
	Bones     []skeletonBoneWant `json:"bones"`
	DrawOrder []string           `json:"draw_order"`
	BodyWorld [][]float64        `json:"body_world"`
	LegWorld  [][]float64        `json:"leg_world"`
	LegTris   []int              `json:"leg_tris"`
	LegHull   int                `json:"leg_hull"`
	LegUvs    [][]float64        `json:"leg_uvs"`
	IKUpper   []float64          `json:"ik_upper"`
	IKLower   []float64          `json:"ik_lower"`
	IKTip     []float64          `json:"ik_tip"`
	IKTarget  []float64          `json:"ik_target"`
	IKMaxDist float64            `json:"ik_max_dist"`
	HeadWorld []float64          `json:"head_world"`
	HeadMat   []float64          `json:"head_mat"`
}

func loadSkeletonCases(t *testing.T) skeletonCases {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "spine_cases.json"))
	if err != nil {
		t.Fatalf("read spine_cases.json: %v", err)
	}
	var c skeletonCases
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("decode spine_cases.json: %v", err)
	}
	if len(c.Bones) == 0 || len(c.DrawOrder) == 0 {
		t.Fatal("spine_cases.json has no bones or draw order")
	}
	return c
}

func loadHeroSkeleton(t *testing.T) *Skeleton {
	t.Helper()
	s, err := LoadSkeletonFile(filepath.Join("testdata", "spine_hero.json"))
	if err != nil {
		t.Fatalf("LoadSkeletonFile spine_hero.json: %v", err)
	}
	return s
}

func mustFindBoneWant(t *testing.T, c skeletonCases, name string) skeletonBoneWant {
	t.Helper()
	for _, b := range c.Bones {
		if b.Name == name {
			return b
		}
	}
	t.Fatalf("spine_cases.json has no bone %q", name)
	return skeletonBoneWant{}
}

func sameStrings(a, b []string) bool {
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

func closeVec(got core.Vec2, want []float64) bool {
	if len(want) != 2 {
		return false
	}
	return math.Abs(got.X-want[0]) < epsSkeleton && math.Abs(got.Y-want[1]) < epsSkeleton
}

func closeMat(got core.Mat2D, want []float64) bool {
	if len(want) != 6 {
		return false
	}
	vals := []float64{got.A, got.B, got.C, got.D, got.E, got.F}
	for i := range vals {
		if math.Abs(vals[i]-want[i]) >= epsSkeleton {
			return false
		}
	}
	return true
}

// A: bones, slots, skins, mesh, weights, draw order, IK, transform,
// physics placeholder all land on the frozen goldens.
func TestSkeletonBonesFromCases(t *testing.T) {
	c := loadSkeletonCases(t)
	s := loadHeroSkeleton(t)
	if got := s.Version(); got.Major != c.Version.Major || got.Minor != c.Version.Minor {
		t.Fatalf("version = %v, want %d.%d", got, c.Version.Major, c.Version.Minor)
	}
	if n := s.BoneCount(); n != len(c.Bones) {
		t.Fatalf("bones = %d, want %d", n, len(c.Bones))
	}
	if n := s.SlotCount(); n != len(c.DrawOrder) {
		t.Fatalf("slots = %d, want %d", n, len(c.DrawOrder))
	}
	if names := s.SkinNames(); !sameStrings(names, []string{c.Skin}) {
		t.Fatalf("skins = %q, want [%q]", names, c.Skin)
	}
	if cur := s.CurrentSkin(); cur != c.Skin {
		t.Fatalf("current skin = %q, want %q", cur, c.Skin)
	}
	if got := s.DrawOrder(); !sameStrings(got, c.DrawOrder) {
		t.Fatalf("draw order = %q, want %q", got, c.DrawOrder)
	}
	if names := s.IKNames(); !sameStrings(names, []string{"leg_ik"}) {
		t.Fatalf("ik = %q, want [leg_ik]", names)
	}
	if names := s.TransformNames(); !sameStrings(names, []string{"head_look"}) {
		t.Fatalf("transform = %q, want [head_look]", names)
	}
	p, err := NewPose(s)
	if err != nil {
		t.Fatalf("NewPose: %v", err)
	}
	for _, want := range c.Bones {
		v, err := p.WorldPos(want.Name)
		if err != nil {
			t.Errorf("WorldPos %q: %v", want.Name, err)
			continue
		}
		if !closeVec(v, want.World) {
			t.Errorf("%s pos = (%v,%v), want (%v,%v)", want.Name, v.X, v.Y, want.World[0], want.World[1])
		}
		m, err := p.WorldTransform(want.Name)
		if err != nil {
			t.Errorf("WorldTransform %q: %v", want.Name, err)
			continue
		}
		if !closeMat(m, want.Mat) {
			t.Errorf("%s mat = %+v, want %v", want.Name, m, want.Mat)
		}
	}
	body, err := s.Attachment("body_slot", "body_img")
	if err != nil {
		t.Fatalf("body attachment: %v", err)
	}
	if body.Type != "region" || string(body.Path) != "tex/body" {
		t.Errorf("body attachment = %s/%s, want region/tex/body", body.Type, body.Path)
	}
	if len(body.Verts) != 4 || len(body.Triangles) != 6 || body.Hull != 4 {
		t.Errorf("body mesh shape = %d verts %d tris hull %d, want 4/6/4",
			len(body.Verts), len(body.Triangles), body.Hull)
	}
	leg, err := s.Attachment("leg_slot", "leg_mesh")
	if err != nil {
		t.Fatalf("leg attachment: %v", err)
	}
	if leg.Type != "mesh" || string(leg.Path) != "tex/leg" {
		t.Errorf("leg attachment = %s/%s, want mesh/tex/leg", leg.Type, leg.Path)
	}
	if len(leg.Verts) != 4 || len(leg.Weights) != 4 || leg.Hull != c.LegHull {
		t.Fatalf("leg mesh shape = %d verts %d weights hull %d", len(leg.Verts), len(leg.Weights), leg.Hull)
	}
	for i, row := range leg.Weights {
		if len(row) != 2 {
			t.Errorf("leg vert %d influences = %d, want 2", i, len(row))
			continue
		}
		var sum float64
		for _, inf := range row {
			sum += inf.Weight
		}
		if math.Abs(sum-1) >= 1e-9 {
			t.Errorf("leg vert %d weights sum = %v, want 1", i, sum)
		}
	}
	if len(leg.Triangles) != len(c.LegTris) {
		t.Fatalf("leg tris len = %d, want %d", len(leg.Triangles), len(c.LegTris))
	}
	for i, v := range leg.Triangles {
		if v != c.LegTris[i] {
			t.Errorf("leg tri[%d] = %d, want %d", i, v, c.LegTris[i])
		}
	}
	bw, err := p.SkinVertices("body_slot", "body_img")
	if err != nil {
		t.Fatalf("body skin verts: %v", err)
	}
	if len(bw) != len(c.BodyWorld) {
		t.Fatalf("body world verts = %d, want %d", len(bw), len(c.BodyWorld))
	}
	for i, v := range bw {
		if !closeVec(v, c.BodyWorld[i]) {
			t.Errorf("body vert[%d] = (%v,%v), want (%v,%v)", i, v.X, v.Y, c.BodyWorld[i][0], c.BodyWorld[i][1])
		}
	}
	lw, err := p.SkinVertices("leg_slot", "leg_mesh")
	if err != nil {
		t.Fatalf("leg skin verts: %v", err)
	}
	if len(lw) != len(c.LegWorld) {
		t.Fatalf("leg world verts = %d, want %d", len(lw), len(c.LegWorld))
	}
	for i, v := range lw {
		if !closeVec(v, c.LegWorld[i]) {
			t.Errorf("leg vert[%d] = (%v,%v), want (%v,%v)", i, v.X, v.Y, c.LegWorld[i][0], c.LegWorld[i][1])
		}
	}
	ikPose, err := NewPose(s)
	if err != nil {
		t.Fatalf("NewPose: %v", err)
	}
	if err := ikPose.ApplyIK("leg_ik"); err != nil {
		t.Fatalf("ApplyIK: %v", err)
	}
	up, _ := ikPose.WorldPos("upper")
	if !closeVec(up, c.IKUpper) {
		t.Errorf("ik upper = (%v,%v), want (%v,%v)", up.X, up.Y, c.IKUpper[0], c.IKUpper[1])
	}
	lo, _ := ikPose.WorldPos("lower")
	if !closeVec(lo, c.IKLower) {
		t.Errorf("ik lower = (%v,%v), want (%v,%v)", lo.X, lo.Y, c.IKLower[0], c.IKLower[1])
	}
	lm, _ := ikPose.WorldTransform("lower")
	tip := lm.TransformPoint(core.V2(30, 0))
	tv, _ := ikPose.WorldPos("target")
	if d := tip.Sub(tv).Length(); d > c.IKMaxDist {
		t.Errorf("ik tip dist = %v, want <= %v (tip %+v target %+v)", d, c.IKMaxDist, tip, tv)
	}
	if !closeVec(tip, c.IKTip) {
		t.Errorf("ik tip = (%v,%v), want (%v,%v)", tip.X, tip.Y, c.IKTip[0], c.IKTip[1])
	}
	tfPose, err := NewPose(s)
	if err != nil {
		t.Fatalf("NewPose: %v", err)
	}
	if err := tfPose.ApplyTransform("head_look"); err != nil {
		t.Fatalf("ApplyTransform: %v", err)
	}
	hv, _ := tfPose.WorldPos("head")
	if !closeVec(hv, c.HeadWorld) {
		t.Errorf("head = (%v,%v), want (%v,%v)", hv.X, hv.Y, c.HeadWorld[0], c.HeadWorld[1])
	}
	hm, _ := tfPose.WorldTransform("head")
	if !closeMat(hm, c.HeadMat) {
		t.Errorf("head mat = %+v, want %v", hm, c.HeadMat)
	}
	if err := tfPose.ApplyPhysics("inertia"); core.CodeOf(err) != core.CodeUnsupported {
		t.Errorf("ApplyPhysics err = %v, want unsupported", err)
	}
	rule, err := s.IK("leg_ik")
	if err != nil {
		t.Fatalf("IK leg_ik: %v", err)
	}
	if len(rule.Bones) != 2 || rule.Target != "target" || rule.Mix != 1 || !rule.BendPositive {
		t.Errorf("ik rule = %+v, want 2 bones/target/mix1/bend+", rule)
	}
	// Additive entry for S46: replace lerps, additive adds the delta.
	base, _ := NewPose(s)
	layer, _ := NewPose(s)
	if err := layer.SetBoneLocal("upper", BoneLocal{X: 25, Y: 5, Rotation: 30, ScaleX: 1, ScaleY: 1}); err != nil {
		t.Fatalf("SetBoneLocal: %v", err)
	}
	mixed, _ := NewPose(s)
	if err := mixed.BlendFrom(base, layer, 0.5, BlendReplace); err != nil {
		t.Fatalf("BlendFrom replace: %v", err)
	}
	got, _ := mixed.BoneLocal("upper")
	setup, _ := base.BoneLocal("upper")
	if math.Abs(got.Rotation-(setup.Rotation+15)) >= 1e-9 {
		t.Errorf("replace blend rot = %v, want setup+15", got.Rotation)
	}
	added, _ := NewPose(s)
	if err := added.BlendFrom(base, layer, 1, BlendAdditive); err != nil {
		t.Fatalf("BlendFrom additive: %v", err)
	}
	gotAdd, _ := added.BoneLocal("upper")
	if math.Abs(gotAdd.Rotation-30) >= 1e-9 || math.Abs(gotAdd.X-25) >= 1e-9 {
		t.Errorf("additive blend = %+v, want layer values", gotAdd)
	}
	_ = mustFindBoneWant(t, c, "root")
}

// B: empty, missing, torn, versioned, unfrozen, and unknown inputs
// error with distinct codes and never crash.
func TestSkeletonEdgesNoCrash(t *testing.T) {
	s := loadHeroSkeleton(t)
	if _, err := LoadSkeletonFile(""); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty path err = %v, want invalid-arg", err)
	}
	if _, err := LoadSkeletonFile(filepath.Join("testdata", "spine_missing.json")); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("missing file err = %v, want not-found", err)
	}
	if _, err := ParseSkeleton(nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty data err = %v, want invalid-arg", err)
	}
	if _, err := ParseSkeleton([]byte("{torn")); core.CodeOf(err) != core.CodeBadData {
		t.Errorf("torn json err = %v, want bad-data", err)
	}
	badVer := []byte(`{"skeleton":{"spine":"9.9.00"},"bones":[{"name":"r"}]}`)
	if _, err := ParseSkeleton(badVer); core.CodeOf(err) != core.CodeVersionMismatch {
		t.Errorf("bad version err = %v, want version-mismatch", err)
	}
	withPhysics := []byte(`{"skeleton":{"spine":"4.0.00"},"bones":[{"name":"r"}],"physics":[{"name":"p"}]}`)
	if _, err := ParseSkeleton(withPhysics); core.CodeOf(err) != core.CodeUnsupported {
		t.Errorf("physics err = %v, want unsupported", err)
	}
	withPath := []byte(`{"skeleton":{"spine":"4.0.00"},"bones":[{"name":"r"}],"path":[{"name":"p"}]}`)
	if _, err := ParseSkeleton(withPath); core.CodeOf(err) != core.CodeUnsupported {
		t.Errorf("path err = %v, want unsupported", err)
	}
	linked := []byte(`{"skeleton":{"spine":"4.0.00"},"bones":[{"name":"r"}],"slots":[{"name":"s","bone":"r"}],"skins":[{"name":"d","attachments":{"s":{"a":{"type":"linkedmesh"}}}}]}`)
	if _, err := ParseSkeleton(linked); core.CodeOf(err) != core.CodeUnsupported {
		t.Errorf("linkedmesh err = %v, want unsupported", err)
	}
	dangling := []byte(`{"skeleton":{"spine":"4.0.00"},"bones":[{"name":"a","parent":"ghost"}]}`)
	if _, err := ParseSkeleton(dangling); core.CodeOf(err) != core.CodeBadData {
		t.Errorf("dangling parent err = %v, want bad-data", err)
	}
	dup := []byte(`{"skeleton":{"spine":"4.0.00"},"bones":[{"name":"r"},{"name":"r"}]}`)
	if _, err := ParseSkeleton(dup); core.CodeOf(err) != core.CodeBadData {
		t.Errorf("dup bone err = %v, want bad-data", err)
	}
	p, err := NewPose(s)
	if err != nil {
		t.Fatalf("NewPose: %v", err)
	}
	if _, err := NewPose(nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil skeleton pose err = %v, want invalid-arg", err)
	}
	if _, err := s.Bone("ghost"); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("missing bone err = %v, want not-found", err)
	}
	if _, err := s.Bone(""); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty bone err = %v, want invalid-arg", err)
	}
	if _, err := s.Slot("ghost"); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("missing slot err = %v, want not-found", err)
	}
	if err := s.SetSkin("ghost"); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("missing skin err = %v, want not-found", err)
	}
	if err := s.SetSkin(""); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty skin err = %v, want invalid-arg", err)
	}
	if _, err := s.Attachment("leg_slot", "ghost"); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("missing attachment err = %v, want not-found", err)
	}
	before := s.DrawOrder()
	if err := s.SetDrawOrder([]string{"body_slot"}); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("short draw order err = %v, want invalid-arg", err)
	}
	if err := s.SetDrawOrder([]string{"body_slot", "body_slot"}); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("dup draw order err = %v, want invalid-arg", err)
	}
	if err := s.SetDrawOrder([]string{"body_slot", "ghost"}); core.CodeOf(err) == core.CodeUnknown {
		t.Errorf("ghost draw order err = %v, want not-unknown", err)
	}
	if got := s.DrawOrder(); !sameStrings(got, before) {
		t.Errorf("bad SetDrawOrder moved order to %q", got)
	}
	nanLocal := BoneLocal{X: math.NaN(), Y: 0, Rotation: 0, ScaleX: 1, ScaleY: 1}
	if err := p.SetBoneLocal("upper", nanLocal); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("NaN local err = %v, want invalid-arg", err)
	}
	if err := p.SetBoneLocal("ghost", BoneLocal{ScaleX: 1, ScaleY: 1}); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("missing pose bone err = %v, want not-found", err)
	}
	if err := p.SetBoneLocal("", BoneLocal{}); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty pose bone err = %v, want invalid-arg", err)
	}
	if _, err := p.SkinVertices("ghost", "leg_mesh"); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("missing skin slot err = %v, want not-found", err)
	}
	if _, err := p.SkinVertices("", ""); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty skin names err = %v, want invalid-arg", err)
	}
	if err := p.ApplyIK("ghost"); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("missing ik err = %v, want not-found", err)
	}
	if err := p.ApplyIK(""); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty ik err = %v, want invalid-arg", err)
	}
	if err := p.ApplyTransform("ghost"); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("missing transform err = %v, want not-found", err)
	}
	if err := p.ApplyPhysics(""); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty physics err = %v, want invalid-arg", err)
	}
	if err := p.ApplyPhysics("inertia"); core.CodeOf(err) != core.CodeUnsupported {
		t.Errorf("physics err = %v, want unsupported", err)
	}
	base, _ := NewPose(s)
	layer, _ := NewPose(s)
	other, _ := ParseSkeleton(mustHeroBytes(t))
	otherPose, _ := NewPose(other)
	if err := p.BlendFrom(base, layer, math.NaN(), BlendReplace); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("NaN blend err = %v, want invalid-arg", err)
	}
	if err := p.BlendFrom(base, layer, 0.5, BlendMode(99)); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("bad blend mode err = %v, want invalid-arg", err)
	}
	if err := p.BlendFrom(base, otherPose, 0.5, BlendReplace); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("foreign skeleton blend err = %v, want invalid-arg", err)
	}
	if err := p.BlendFrom(nil, layer, 0.5, BlendReplace); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil blend base err = %v, want invalid-arg", err)
	}
	// Nil receivers never panic.
	var nilSkel *Skeleton
	var nilPose *Pose
	if nilSkel.BoneCount() != 0 || nilSkel.SlotCount() != 0 || nilSkel.CurrentSkin() != "" {
		t.Error("nil skeleton accessors returned live values")
	}
	if len(nilSkel.DrawOrder()) != 0 || len(nilSkel.SkinNames()) != 0 {
		t.Error("nil skeleton lists returned live values")
	}
	if _, err := nilSkel.Bone("root"); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Bone err = %v, want invalid-arg", err)
	}
	if err := nilSkel.SetSkin("d"); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil SetSkin err = %v, want invalid-arg", err)
	}
	if _, err := nilPose.WorldPos("root"); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil WorldPos err = %v, want invalid-arg", err)
	}
	if err := nilPose.SetBoneLocal("root", BoneLocal{}); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil SetBoneLocal err = %v, want invalid-arg", err)
	}
	if err := nilPose.ApplyIK("x"); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil ApplyIK err = %v, want invalid-arg", err)
	}
	if err := nilPose.ApplyTransform("x"); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil ApplyTransform err = %v, want invalid-arg", err)
	}
	if err := nilPose.ApplyPhysics("x"); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil ApplyPhysics err = %v, want invalid-arg", err)
	}
	if err := nilPose.BlendFrom(base, layer, 0.5, BlendReplace); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil BlendFrom err = %v, want invalid-arg", err)
	}
	if _, err := nilPose.SkinVertices("a", "b"); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil SkinVertices err = %v, want invalid-arg", err)
	}
	nilPose.Reset()
	if nilPose.Skeleton() != nil || nilSkel.Version() != (core.Version{}) {
		t.Error("nil version/skeleton mismatch")
	}
}

func mustHeroBytes(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "spine_hero.json"))
	if err != nil {
		t.Fatalf("read spine_hero.json: %v", err)
	}
	return raw
}

// C does not need pixels (pure math, draws nothing): the number path
// must replay bitwise identical and cross the render boundary
// losslessly instead.
func TestSkeletonBoundaryIdentical(t *testing.T) {
	s := loadHeroSkeleton(t)
	replay := func() ([]float64, []string) {
		p, err := NewPose(s)
		if err != nil {
			t.Fatalf("NewPose: %v", err)
		}
		if err := p.SetBoneLocal("upper", BoneLocal{X: 22, Y: 3, Rotation: 12, ScaleX: 1, ScaleY: 1}); err != nil {
			t.Fatalf("SetBoneLocal: %v", err)
		}
		if err := p.ApplyIK("leg_ik"); err != nil {
			t.Fatalf("ApplyIK: %v", err)
		}
		if err := p.ApplyTransform("head_look"); err != nil {
			t.Fatalf("ApplyTransform: %v", err)
		}
		var vals []float64
		for _, n := range []string{"root", "body", "upper", "lower", "foot", "target", "head"} {
			v, _ := p.WorldPos(n)
			m, _ := p.WorldTransform(n)
			vals = append(vals, v.X, v.Y, m.A, m.B, m.C, m.D, m.E, m.F)
		}
		sv, err := p.SkinVertices("leg_slot", "leg_mesh")
		if err != nil {
			t.Fatalf("SkinVertices: %v", err)
		}
		for _, v := range sv {
			vals = append(vals, v.X, v.Y)
		}
		return vals, s.DrawOrder()
	}
	aVals, aOrder := replay()
	bVals, bOrder := replay()
	if len(aVals) != len(bVals) {
		t.Fatalf("replay lens %d vs %d", len(aVals), len(bVals))
	}
	for i := range aVals {
		if aVals[i] != bVals[i] {
			t.Fatalf("replay value diverged at %d: %.17g vs %.17g", i, aVals[i], bVals[i])
		}
	}
	if !sameStrings(aOrder, bOrder) {
		t.Fatalf("replay draw order diverged: %q vs %q", aOrder, bOrder)
	}
	// Copies never alias: mutating a return never moves the skeleton.
	p, _ := NewPose(s)
	att, _ := s.Attachment("leg_slot", "leg_mesh")
	att.Verts[0] = core.V2(999, 999)
	att.Triangles[0] = 999
	fresh, _ := s.Attachment("leg_slot", "leg_mesh")
	if fresh.Verts[0].X == 999 || fresh.Triangles[0] == 999 {
		t.Error("Attachment leaked caller mutation into the skeleton")
	}
	order := s.DrawOrder()
	order[0] = "ghost"
	if again := s.DrawOrder(); again[0] == "ghost" {
		t.Error("DrawOrder leaked caller mutation into the skeleton")
	}
	first, _ := p.SkinVertices("leg_slot", "leg_mesh")
	first[0] = core.V2(999, 999)
	second, _ := p.SkinVertices("leg_slot", "leg_mesh")
	if second[0].X == 999 {
		t.Error("SkinVertices reused its buffer across calls")
	}
	// Boundary crossings are lossless both ways.
	v := core.V2(12.5, -7.25)
	if back := core.Vec2FromRenderPoint(v.ToRenderPoint()); back != v {
		t.Errorf("Vec2 boundary round trip = %+v, want %+v", back, v)
	}
	m, _ := p.WorldTransform("upper")
	if back := core.Mat2DFromRenderMatrix(m.ToRenderMatrix()); back != m {
		t.Errorf("Mat2D boundary round trip = %+v, want %+v", back, m)
	}
	c0 := core.RGBA(0.2, 0.4, 0.6, 0.8)
	if back := core.ColorFromRender(c0.ToRender()); back != c0 {
		t.Errorf("Color boundary round trip = %+v, want %+v", back, c0)
	}
}

// D: one character poses and skins with a measured cost.
func TestSkeletonPerfSingle(t *testing.T) {
	// Synthetic load only (no golden): golden pose stays in
	// spine_cases.json. One hero with two attachments per frame.
	s := loadHeroSkeleton(t)
	p, err := NewPose(s)
	if err != nil {
		t.Fatalf("NewPose: %v", err)
	}
	const rounds = 20000
	var acc float64
	start := time.Now()
	for r := 0; r < rounds; r++ {
		p.Reset()
		if err := p.SetBoneLocal("upper", BoneLocal{
			X: 20 + float64(r%10), Y: float64(r % 5),
			Rotation: float64(r % 30), ScaleX: 1, ScaleY: 1,
		}); err != nil {
			t.Fatal("perf SetBoneLocal")
		}
		if err := p.ApplyIK("leg_ik"); err != nil {
			t.Fatal("perf ApplyIK")
		}
		sv, err := p.SkinVertices("leg_slot", "leg_mesh")
		if err != nil {
			t.Fatal("perf SkinVertices")
		}
		for _, v := range sv {
			acc += v.X + v.Y
		}
		bv, err := p.SkinVertices("body_slot", "body_img")
		if err != nil {
			t.Fatal("perf SkinVertices body")
		}
		for _, v := range bv {
			acc += v.X + v.Y
		}
	}
	el := time.Since(start)
	t.Logf("skeleton-single: %d pose+ik+2 skins in %v (%.1f ns/op)", rounds, el, float64(el.Nanoseconds())/float64(rounds))
	if math.IsNaN(acc) || math.IsInf(acc, 0) {
		t.Fatal("perf accumulation went non-finite, benchmark invalid")
	}
	if acc == 0 {
		t.Error("perf loop folded to zero, benchmark invalid")
	}
}

// E: long runs neither drift nor misalign between replays.
func TestSkeletonLongRunStable(t *testing.T) {
	s := loadHeroSkeleton(t)
	mk := func() *Pose {
		p, err := NewPose(s)
		if err != nil {
			t.Fatalf("NewPose: %v", err)
		}
		return p
	}
	// Same IK plus transform stream replays bitwise identically.
	const steps = 50000
	a, b := mk(), mk()
	for i := 0; i < steps; i++ {
		if err := a.SetBoneLocal("target", BoneLocal{
			X: 80 + float64(i%7), Y: 10 - float64(i%5),
			Rotation: 40, ScaleX: 1, ScaleY: 1,
		}); err != nil {
			t.Fatalf("rep %d SetBoneLocal: %v", i, err)
		}
		if err := b.SetBoneLocal("target", BoneLocal{
			X: 80 + float64(i%7), Y: 10 - float64(i%5),
			Rotation: 40, ScaleX: 1, ScaleY: 1,
		}); err != nil {
			t.Fatalf("rep %d SetBoneLocal: %v", i, err)
		}
		if err := a.ApplyIK("leg_ik"); err != nil {
			t.Fatalf("rep %d ApplyIK: %v", i, err)
		}
		if err := b.ApplyIK("leg_ik"); err != nil {
			t.Fatalf("rep %d ApplyIK: %v", i, err)
		}
		va, _ := a.WorldPos("foot")
		vb, _ := b.WorldPos("foot")
		if va != vb {
			t.Fatalf("rep %d foot diverged: %+v vs %+v", i, va, vb)
		}
		ta, _ := a.WorldPos("target")
		if d := va.Sub(ta).Length(); d > 1e-6 {
			t.Fatalf("rep %d ik drifted: dist %v", i, d)
		}
	}
	// Reset parks both runs back on the frozen setup exactly.
	a.Reset()
	b.Reset()
	for _, n := range []string{"root", "body", "upper", "lower", "foot", "target", "head"} {
		va, _ := a.WorldPos(n)
		vb, _ := b.WorldPos(n)
		if va != vb {
			t.Fatalf("reset %s diverged: %+v vs %+v", n, va, vb)
		}
	}
	c := loadSkeletonCases(t)
	for _, want := range c.Bones {
		v, _ := a.WorldPos(want.Name)
		if !closeVec(v, want.World) {
			t.Fatalf("reset %s = (%v,%v), want frozen (%v,%v)", want.Name, v.X, v.Y, want.World[0], want.World[1])
		}
	}
	if got := s.DrawOrder(); !sameStrings(got, c.DrawOrder) {
		t.Fatalf("long-run draw order moved to %q", got)
	}
}

// F: offscreen golden stands in for the window (pure math until the
// game_anim --case=sk window lands with P2). The frozen pose plus draw
// order in spine_cases.json is the evidence both backends share; shape
// assertions below pin the meaning, not just the numbers.
func TestSkeletonOffscreenGolden(t *testing.T) {
	c := loadSkeletonCases(t)
	s := loadHeroSkeleton(t)
	// Golden pins the anchor: chain runs +X, draw order is file order.
	byName := map[string][]float64{}
	for _, b := range c.Bones {
		byName[b.Name] = b.World
	}
	if !(byName["root"][0] < byName["body"][0] && byName["body"][0] < byName["upper"][0] &&
		byName["upper"][0] < byName["lower"][0] && byName["lower"][0] < byName["foot"][0]) {
		t.Errorf("setup chain not +X ordered: %+v", byName)
	}
	if !sameStrings(c.DrawOrder, []string{"body_slot", "leg_slot"}) {
		t.Errorf("draw order = %q, want file order", c.DrawOrder)
	}
	// Mesh shape: hull 4, two tris, uvs in [0,1], indices in range.
	if c.LegHull != 4 || len(c.LegTris) != 6 {
		t.Errorf("leg mesh shape hull %d tris %d, want 4/6", c.LegHull, len(c.LegTris))
	}
	for _, uv := range c.LegUvs {
		if uv[0] < 0 || uv[0] > 1 || uv[1] < 0 || uv[1] > 1 {
			t.Errorf("leg uv %v outside [0,1]", uv)
		}
	}
	for _, ti := range c.LegTris {
		if ti < 0 || ti >= len(c.LegWorld) {
			t.Errorf("leg tri %d outside verts", ti)
		}
	}
	// Weights always partition one bone unit per vertex.
	p, _ := NewPose(s)
	leg, _ := s.Attachment("leg_slot", "leg_mesh")
	for i, row := range leg.Weights {
		var sum float64
		for _, inf := range row {
			sum += inf.Weight
			if inf.Bone < 0 || inf.Bone >= s.BoneCount() {
				t.Errorf("leg vert %d bone %d outside skeleton", i, inf.Bone)
			}
		}
		if math.Abs(sum-1) >= 1e-9 {
			t.Errorf("leg vert %d weights sum %v, want 1", i, sum)
		}
	}
	// IK earns its keep: setup foot far from target, solved tip on it.
	setupFoot := byName["foot"]
	setupTarget := byName["target"]
	before := math.Hypot(setupFoot[0]-setupTarget[0], setupFoot[1]-setupTarget[1])
	if before < 1 {
		t.Errorf("setup foot-target dist %v too small to prove IK", before)
	}
	ikPose, _ := NewPose(s)
	_ = ikPose.ApplyIK("leg_ik")
	lm, _ := ikPose.WorldTransform("lower")
	tip := lm.TransformPoint(core.V2(30, 0))
	tv, _ := ikPose.WorldPos("target")
	if d := tip.Sub(tv).Length(); d > c.IKMaxDist {
		t.Errorf("solved tip dist %v, want <= %v", d, c.IKMaxDist)
	}
	// Transform earns its keep: head lands halfway to the target.
	tfPose, _ := NewPose(s)
	_ = tfPose.ApplyTransform("head_look")
	hv, _ := tfPose.WorldPos("head")
	midx := (byName["head"][0] + byName["target"][0]) / 2
	midy := (byName["head"][1] + byName["target"][1]) / 2
	if math.Abs(hv.X-midx) >= 1e-9 || math.Abs(hv.Y-midy) >= 1e-9 {
		t.Errorf("head = (%v,%v), want midpoint (%v,%v)", hv.X, hv.Y, midx, midy)
	}
	// Window intent: pure math keeps offscreen goldens; the real window
	// game_anim --case=sk (walk/run/jump on true art) lands with P2.
	// examples/game_anim does not exist yet, so no window is built here.
	if _, err := os.Stat(filepath.Join("..", "..", "examples", "game_anim")); err == nil {
		t.Log("game_anim window exists; P2 should wire --case=sk to this skeleton")
	} else {
		t.Log("game_anim window absent; offscreen spine_cases.json stands in (pure math, P2 intent game_anim --case=sk)")
	}
	_ = p
}

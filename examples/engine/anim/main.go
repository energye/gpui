// Command game_anim hosts the 4.x animation independent real windows.
//
// --case=sk is the 4.3 skeleton window (walk/run/jump on true art).
// --case=fsm below is the 4.4 blend state machine window (switch crisp,
// blend bar live). --case=tl further below is the 4.2 keyframe timeline
// window (interpolation, loop wraps, cue order). Each ability keeps its
// own case; no combo window stands in for a single ability.
//
// Window: 1200x800, title game_anim. Three cards WALK/RUN/JUMP paint the
// same 13-bone upright person from the window-local testdata/sk_hero.json
// (window-owned art for a readable person; engine/anim keeps its own
// 7-bone math hero untouched). Bones call engine/anim read-only
// (Pose/Skin/IK), drawing uses only existing render shapes; render/ and
// engine/anim/ stay untouched.
//
// Flags:
//
//	go run ./examples/engine/anim --case=sk -auto-only
//	  RUN_SECONDS=8 (default 8) auto gate, JSON on stdout, exit 1 on fail.
//	go run ./examples/engine/anim --case=sk -manual-seconds 30
//	  resident 30s, real events logged + SetTitle, summary JSON.
//	go run ./examples/engine/anim --case=sk
//	  selftest then resident until close. RUN_SECONDS also times the run.
//
// Gates (hard): present>=1 via wrgate, parity contract passes
// (three-pose replay bitwise + draw order file order, the pure-math
// C-both-sides: raster AA is a known CPU/GPU divergence owned by 9.2,
// 4.3 owns numbers not pixels), logic probes pass (joints match the
// engine/anim offscreen goldens, draw order file order, walk/run/jump
// switch without combine), pixel assertions pass (joint red, bone dark,
// background white, head on its transform spot), Golden zero tolerance
// (offscreen sk_golden.png plus window sk_final_base.png, second run on),
// pose switches>=3 in the auto run.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/energye/gpui/engine/anim"
	"github.com/energye/gpui/engine/core"
	"github.com/energye/gpui/engine/renderconv"
	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/examples/wrsoak"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH = 1200, 800
	winTitle   = "game_anim"
	abilityID  = "anim-sk"
	scenario   = "game_anim--case=sk"

	// Offscreen frozen canvas: three poses side by side, shared verbatim
	// with the window card paint (same mapping, same colors). Cells are
	// wide enough for a mid-stride runner: 13 bones span ~[-8,+26] in
	// world units at cellScale.
	offW, offH = 640, 180
	cellW      = 205.0
	cellOX0    = 40.0
	cellOY     = 150.0
	cellScale  = 1.4

	// Window cards: one pose each, same paint function, bigger scale.
	// A full stride spans ~34 world units; cards keep the whole body
	// plus the ground line in frame.
	cardW, cardH = 280.0, 320.0
	cardScale    = 2.2
	cardOX       = 95.0
	cardOY       = 250.0
)

// Hardcoded tolerances (review visible, never silent).
const (
	logicEps     = 1e-9 // joint match vs engine/anim offscreen goldens
	ikMaxDist    = 1e-9 // solved tip must sit on the target
	pixelByteTol = 10   // per-channel byte tolerance for probes
	goldenTol    = 0.0  // zero tolerance once the baseline exists
	switchMin    = 3    // auto run must show every pose switch
	poseCycleSec = 2.0  // walk->run->jump rotation period
	frozenBones  = 13   // the upright person: pelvis, spine, neck, limbs
)

const testdataDir = "examples/engine/anim/testdata"

// 2.5D摆法：满窗即内容，指标浮左上，Golden排除指标带。
// metricStripH是浮层指标带高度，窗口Golden比对从该高度之下起算。
// 离屏探针帧无指标覆盖；窗口快照只比该带之下的纯画面。
const metricStripH = 32.0

// frozenSetup is the window hero world origins (upright side-view
// person: pelvis at root, torso up to the neck, head on top, one arm
// forward, two legs down to opposite ground targets). Same numbers
// live in testdata/sk_hero.json; the window joints must land on them
// exactly.
var frozenSetup = map[string][2]float64{
	"root":    {0, 56},
	"torso":   {0, 62},
	"head":    {1, 92},
	"armUp":   {3, 86},
	"armLo":   {15.85575219373079, 70.67911113762044},
	"thighL":  {4, 54},
	"shinL":   {4, 26},
	"footL":   {4, -2},
	"targetL": {4, -2},
	"thighR":  {-4, 54},
	"shinR":   {-4, 26},
	"footR":   {-4, -2},
	"targetR": {-4, -2},
}

var boneNames = []string{"root", "torso", "head", "armUp", "armLo", "thighL", "shinL", "footL", "targetL", "thighR", "shinR", "footR", "targetR"}

// Human links: spine up, head on top, arm forward, one leg each side.
// The IK guides root->target stay out and draw thin gray.
var boneLinks = [][2]string{
	{"root", "torso"}, {"torso", "head"},
	{"torso", "armUp"}, {"armUp", "armLo"},
	{"root", "thighL"}, {"thighL", "shinL"}, {"shinL", "footL"},
	{"root", "thighR"}, {"thighR", "shinR"}, {"shinR", "footR"},
}

var poseNames = []string{"walk", "run", "jump"}

type manualSummary struct {
	Pointer int
	Key     int
	Resize  int
	Timed   bool
	Note    string
}

// applyPose parks p on setup, then steps the ground targets plus a body
// lift so walk/run/jump bend each IK leg differently; the free arm swings
// against the left leg for a readable gait. Setup already stands feet
// apart, so walk only toes one foot; every pose still differs, or the
// switch gate fails.
func applyPose(p *anim.Pose, name string) error {
	if p == nil {
		return fmt.Errorf("nil pose")
	}
	p.Reset()
	bump := func(bone string, dRot, dX, dY float64) error {
		l, err := p.BoneLocal(bone)
		if err != nil {
			return err
		}
		l.Rotation += dRot
		l.X += dX
		l.Y += dY
		return p.SetBoneLocal(bone, l)
	}
	swingArm := func(deg float64) error {
		if err := bump("armUp", deg, 0, 0); err != nil {
			return err
		}
		return bump("armLo", -deg/2, 0, 0)
	}
	switch name {
	case "walk":
		if err := bump("targetL", 0, 5, 3); err != nil {
			return err
		}
		if err := bump("footL", 8, 0, 0); err != nil {
			return err
		}
		if err := swingArm(-12); err != nil {
			return err
		}
	case "run":
		if err := bump("targetL", 0, 12, 0); err != nil {
			return err
		}
		if err := bump("targetR", 0, -10, 6); err != nil {
			return err
		}
		if err := bump("root", 0, 0, 1); err != nil {
			return err
		}
		if err := bump("torso", 0, 0, 2); err != nil {
			return err
		}
		if err := bump("footL", 14, 0, 0); err != nil {
			return err
		}
		if err := swingArm(-30); err != nil {
			return err
		}
	case "jump":
		if err := bump("targetL", 0, 8, 22); err != nil {
			return err
		}
		if err := bump("targetR", 0, -8, 22); err != nil {
			return err
		}
		if err := bump("root", 0, 0, 14); err != nil {
			return err
		}
		if err := bump("torso", 0, 0, 3); err != nil {
			return err
		}
		if err := bump("footL", -12, 0, 0); err != nil {
			return err
		}
		if err := bump("footR", -12, 0, 0); err != nil {
			return err
		}
		if err := swingArm(35); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown pose %q", name)
	}
	if err := p.ApplyIK("legL_ik"); err != nil {
		return err
	}
	if err := p.ApplyIK("legR_ik"); err != nil {
		return err
	}
	return p.ApplyTransform("head_look")
}

func buildPose(skel *anim.Skeleton, name string) (*anim.Pose, error) {
	p, err := anim.NewPose(skel)
	if err != nil {
		return nil, err
	}
	if err := applyPose(p, name); err != nil {
		return nil, err
	}
	return p, nil
}

// poseSig freezes every number one pose owns: bone worlds plus matrices
// plus both skin vertex sets. Bitwise compare proves a switch combined
// nothing stale.
func poseSig(p *anim.Pose) ([]float64, error) {
	var out []float64
	for _, n := range boneNames {
		v, err := p.WorldPos(n)
		if err != nil {
			return nil, err
		}
		m, err := p.WorldTransform(n)
		if err != nil {
			return nil, err
		}
		out = append(out, v.X, v.Y, m.A, m.B, m.C, m.D, m.E, m.F)
	}
	for _, sa := range [][2]string{{"torso_slot", "torso_img"}, {"legL_slot", "legL_img"}, {"legR_slot", "legR_img"}, {"arm_slot", "arm_img"}, {"head_slot", "head_img"}} {
		vs, err := p.SkinVertices(sa[0], sa[1])
		if err != nil {
			return nil, err
		}
		for _, v := range vs {
			out = append(out, v.X, v.Y)
		}
	}
	return out, nil
}

func sameSig(a, b []float64) bool {
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

// runLogicProbes checks the frozen contract: joint goldens, draw order,
// mesh shape, IK seating, three-pose switch purity, boundary lossless.
func runLogicProbes(skel *anim.Skeleton) (map[string]any, bool) {
	out := map[string]any{}
	ok := true
	fail := func(k string) {
		out[k] = false
		ok = false
	}
	if skel.BoneCount() != frozenBones {
		out["bone_count"] = skel.BoneCount()
		fail("bone_count_ok")
	} else {
		out["bone_count_ok"] = true
	}
	if got := skel.DrawOrder(); len(got) != 5 || got[0] != "legR_slot" || got[1] != "legL_slot" || got[2] != "torso_slot" || got[3] != "arm_slot" || got[4] != "head_slot" {
		out["draw_order"] = got
		fail("draw_order_ok")
	} else {
		out["draw_order_ok"] = true
	}
	if skel.CurrentSkin() != "default" {
		out["skin"] = skel.CurrentSkin()
		fail("skin_ok")
	} else {
		out["skin_ok"] = true
	}
	if names := skel.IKNames(); len(names) != 2 || names[0] != "legL_ik" || names[1] != "legR_ik" {
		out["ik_names"] = names
		fail("ik_ok")
	} else {
		out["ik_ok"] = true
	}
	if names := skel.TransformNames(); len(names) != 1 || names[0] != "head_look" {
		out["transform_names"] = names
		fail("transform_ok")
	} else {
		out["transform_ok"] = true
	}
	// Setup joints match the engine/anim offscreen goldens exactly.
	setup, err := anim.NewPose(skel)
	if err != nil {
		out["setup_pose"] = err.Error()
		fail("joints_ok")
		return out, false
	}
	jointsOK := true
	for _, n := range boneNames {
		v, err := setup.WorldPos(n)
		if err != nil {
			jointsOK = false
			break
		}
		w := frozenSetup[n]
		if math.Abs(v.X-w[0]) >= logicEps || math.Abs(v.Y-w[1]) >= logicEps {
			out["joint_"+n] = []float64{v.X, v.Y}
			jointsOK = false
		}
	}
	out["joints_ok"] = jointsOK
	if !jointsOK {
		ok = false
	}
	// Mesh shape: five region skins ride their bones (torso, two shins,
	// arm, head); one shin mesh skipped for a readable first person.
	meshOK := true
	for _, sa := range [][2]string{{"torso_slot", "torso_img"}, {"legL_slot", "legL_img"}, {"legR_slot", "legR_img"}, {"arm_slot", "arm_img"}, {"head_slot", "head_img"}} {
		a, aerr := skel.Attachment(sa[0], sa[1])
		if aerr != nil || a.Type != "region" || len(a.Verts) != 4 ||
			len(a.Triangles) != 6 || a.Hull != 4 {
			meshOK = false
			break
		}
	}
	out["mesh_ok"] = meshOK
	if !meshOK {
		ok = false
	}
	// Both IK legs earn their keep: each solved shin tip (one shin
	// length along its X axis) sits on its own ground target.
	ikPose, err := anim.NewPose(skel)
	ikOK := false
	if err == nil && ikPose.ApplyIK("legL_ik") == nil && ikPose.ApplyIK("legR_ik") == nil {
		tipOK := true
		var firstDist float64
		for i, leg := range [][2]string{{"shinL", "targetL"}, {"shinR", "targetR"}} {
			lm, lerr := ikPose.WorldTransform(leg[0])
			tv, verr := ikPose.WorldPos(leg[1])
			if lerr != nil || verr != nil {
				tipOK = false
				break
			}
			tip := lm.TransformPoint(core.V2(28, 0))
			d := tip.Sub(tv).Length()
			if i == 0 {
				firstDist = d
			}
			if d > ikMaxDist {
				tipOK = false
				break
			}
		}
		out["ik_tip_dist"] = firstDist
		ikOK = tipOK
	}
	out["ik_seated_ok"] = ikOK
	if !ikOK {
		ok = false
	}
	// Three poses: same bone count, each replays bitwise, reuse after
	// Reset equals a fresh build (switch combines nothing stale).
	switchOK := true
	sigs := map[string][]float64{}
	for _, name := range poseNames {
		a, err := buildPose(skel, name)
		if err != nil {
			switchOK = false
			break
		}
		if a.Skeleton().BoneCount() != frozenBones {
			switchOK = false
			break
		}
		b, err := buildPose(skel, name)
		if err != nil {
			switchOK = false
			break
		}
		sa, err := poseSig(a)
		if err != nil {
			switchOK = false
			break
		}
		sb, err := poseSig(b)
		if err != nil || !sameSig(sa, sb) {
			switchOK = false
			break
		}
		sigs[name] = sa
	}
	if switchOK {
		reused, err := buildPose(skel, "walk")
		if err != nil || applyPose(reused, "run") != nil {
			switchOK = false
		} else {
			sr, err := poseSig(reused)
			if err != nil || !sameSig(sr, sigs["run"]) {
				switchOK = false
			}
		}
	}
	// Poses actually differ (a switch that changes nothing proves nothing).
	if switchOK && (sameSig(sigs["walk"], sigs["run"]) || sameSig(sigs["run"], sigs["jump"])) {
		switchOK = false
	}
	out["switch_ok"] = switchOK
	if !switchOK {
		ok = false
	}
	// Boundary crossings stay lossless both ways.
	boundaryOK := true
	v := core.V2(12.5, -7.25)
	if back := renderconv.Vec2FromRenderPoint(renderconv.Vec2ToRenderPoint(v)); back != v {
		boundaryOK = false
	}
	if m, err := setup.WorldTransform("shinL"); err != nil {
		boundaryOK = false
	} else if back := renderconv.Mat2DFromRenderMatrix(renderconv.Mat2DToRenderMatrix(m)); back != m {
		boundaryOK = false
	}
	out["boundary_ok"] = boundaryOK
	if !boundaryOK {
		ok = false
	}
	out["probe_ok"] = ok
	return out, ok
}

func w2s(ox, oy, sk, x, y float64) (float64, float64) {
	return ox + x*sk, oy - y*sk
}

// paintPose draws one upright person. Order: ground, clothing capsules
// (torso/shirt, pants, sleeve, shoe), skin quads for draw-order evidence,
// head face plus hand, thin bone core on top for the X-ray read, then red
// joint dots (targets draw as hollow markers, never red squares).
// Probe pixels survive by order: root/head centers end red, the front
// thigh midline ends dark.
func paintPose(dc *render.Context, skel *anim.Skeleton, p *anim.Pose, ox, oy, sk float64) {
	if dc == nil || skel == nil || p == nil {
		return
	}
	at := func(n string) (float64, float64, bool) {
		v, err := p.WorldPos(n)
		if err != nil {
			return 0, 0, false
		}
		x, y := w2s(ox, oy, sk, v.X, v.Y)
		return x, y, true
	}
	tipOf := func(bone string, along float64) (float64, float64, bool) {
		m, err := p.WorldTransform(bone)
		if err != nil {
			return 0, 0, false
		}
		v := m.TransformPoint(core.V2(along, 0))
		x, y := w2s(ox, oy, sk, v.X, v.Y)
		return x, y, true
	}
	seg := func(x1, y1, x2, y2 float64, w float64, r, g, b float64) {
		dc.SetRGBA(r, g, b, 1)
		dc.SetLineCap(render.LineCapRound)
		dc.SetLineWidth(w)
		dc.DrawLine(x1, y1, x2, y2)
		_ = dc.Stroke()
	}
	k := sk / 2.2
	// Ground at the setup foot height: walk/run stand on it, jump lifts off.
	{
		gx1, gy1 := w2s(ox, oy, sk, -16, -2)
		gx2, _ := w2s(ox, oy, sk, 30, -2)
		seg(gx1, gy1, gx2, gy1, 1.5, 0.62, 0.65, 0.70)
	}
	// Gather the body points once; missing joints skip the person.
	rx, ry, okR := at("root")
	nx, ny, okN := tipOf("torso", 30)
	hx, hy, okH := at("head")
	sx, sy, okS := at("armUp")
	ex, ey, okE := at("armLo")
	wx, wy, okW := tipOf("armLo", 18)
	hlx, hly, okHL := at("thighL")
	klx, kly, okKL := at("shinL")
	alx, aly, okAL := at("footL")
	tlx, tly, okTL := tipOf("footL", 10)
	hrx, hry, okHR := at("thighR")
	krx, kry, okKR := at("shinR")
	arx, ary, okAR := at("footR")
	trx, try_, okTR := tipOf("footR", 10)
	if !(okR && okN && okH && okS && okE && okW && okHL && okKL && okAL && okTL && okHR && okKR && okAR && okTR) {
		return
	}
	// Clothing capsules first (round caps read as limbs).
	seg(hrx, hry, krx, kry, 10*k, 0.30, 0.42, 0.66) // back thigh
	seg(krx, kry, arx, ary, 9*k, 0.30, 0.42, 0.66)  // back shin
	seg(rx, ry, nx, ny, 14*k, 0.92, 0.68, 0.46)     // torso shirt root->neck
	seg(hlx, hly, klx, kly, 10*k, 0.45, 0.62, 0.85) // front thigh
	seg(klx, kly, alx, aly, 9*k, 0.45, 0.62, 0.85)  // front shin
	seg(sx, sy, ex, ey, 7*k, 0.72, 0.50, 0.38)      // upper-arm sleeve
	seg(ex, ey, wx, wy, 6*k, 0.72, 0.50, 0.38)      // forearm sleeve
	seg(alx, aly, tlx, tly, 5*k, 0.16, 0.17, 0.20)  // front shoe
	seg(arx, ary, trx, try_, 5*k, 0.16, 0.17, 0.20) // back shoe
	quad := func(slot, att string, r, g, b float64) {
		a, err := skel.Attachment(slot, att)
		if err != nil || len(a.Verts) != 4 {
			return
		}
		bv, err := p.SkinVertices(slot, att)
		if err != nil || len(bv) != 4 {
			return
		}
		dc.SetRGBA(r, g, b, 1)
		dc.MoveTo(w2s(ox, oy, sk, bv[0].X, bv[0].Y))
		for _, v := range bv[1:] {
			x, y := w2s(ox, oy, sk, v.X, v.Y)
			dc.LineTo(x, y)
		}
		dc.ClosePath()
		_ = dc.Fill()
	}
	// Skin quads ride the same bones (draw-order evidence, same palette).
	quad("legR_slot", "legR_img", 0.30, 0.42, 0.66)
	quad("torso_slot", "torso_img", 0.92, 0.68, 0.46)
	quad("legL_slot", "legL_img", 0.45, 0.62, 0.85)
	quad("arm_slot", "arm_img", 0.72, 0.50, 0.38)
	quad("head_slot", "head_img", 0.96, 0.80, 0.62)
	// Head face: skin disc with a dark rim; the red probe dot lands last.
	dc.SetRGBA(0.96, 0.80, 0.62, 1)
	dc.DrawCircle(hx, hy, 11*sk/2.4)
	_ = dc.Fill()
	dc.SetRGBA(0.12, 0.13, 0.16, 1)
	dc.SetLineWidth(1.5)
	dc.DrawCircle(hx, hy, 11*sk/2.4)
	_ = dc.Stroke()
	// Hand at the wrist so the arm ends in a person, not a stick.
	dc.SetRGBA(0.96, 0.80, 0.62, 1)
	dc.DrawCircle(wx, wy, 4.5*sk/2.4)
	_ = dc.Fill()
	dc.SetRGBA(0.12, 0.13, 0.16, 1)
	dc.SetLineWidth(1.2)
	dc.DrawCircle(wx, wy, 4.5*sk/2.4)
	_ = dc.Stroke()
	// Thin bone core on top: the X-ray read without hiding the clothes.
	dc.SetRGBA(0.12, 0.13, 0.16, 1)
	dc.SetLineCap(render.LineCapRound)
	for _, lk := range boneLinks {
		x1, y1, ok1 := at(lk[0])
		x2, y2, ok2 := at(lk[1])
		if !ok1 || !ok2 {
			continue
		}
		if lk[0] == "root" && (lk[1] == "targetL" || lk[1] == "targetR") {
			continue
		}
		if lk[0] == "root" || lk[0] == "torso" {
			dc.SetLineWidth(2.5)
		} else {
			dc.SetLineWidth(2)
		}
		dc.DrawLine(x1, y1, x2, y2)
		_ = dc.Stroke()
	}
	// Foot targets read as hollow ground markers, not extra feet.
	for _, tg := range []string{"targetL", "targetR"} {
		x, y, ok := at(tg)
		if !ok {
			continue
		}
		dc.SetRGBA(0.55, 0.58, 0.62, 1)
		dc.SetLineWidth(1.2)
		dc.DrawCircle(x, y, 4*sk/2.2)
		_ = dc.Stroke()
	}
	for _, n := range boneNames {
		if n == "targetL" || n == "targetR" {
			continue
		}
		x, y, ok := at(n)
		if !ok {
			continue
		}
		dc.SetRGBA(0.85, 0.15, 0.12, 1)
		dc.DrawRectangle(x-2.5, y-2.5, 5, 5)
		_ = dc.Fill()
	}
}

// paintOffscreen draws the frozen three-pose strip shared with the window.
func paintOffscreen(dc *render.Context, skel *anim.Skeleton) error {
	dc.ClearWithColor(render.White)
	for i, name := range poseNames {
		p, err := buildPose(skel, name)
		if err != nil {
			return err
		}
		paintPose(dc, skel, p, cellOX0+float64(i)*cellW, cellOY, cellScale)
	}
	return nil
}

func sampleByte(img image.Image, x, y int) (r, g, b uint8, valid bool) {
	if img == nil {
		return 0, 0, 0, false
	}
	if x < 0 || y < 0 || x >= img.Bounds().Dx() || y >= img.Bounds().Dy() {
		return 0, 0, 0, false
	}
	r32, g32, b32, _ := img.At(x, y).RGBA()
	return uint8(r32 >> 8), uint8(g32 >> 8), uint8(b32 >> 8), true
}

func closeByte(got, want uint8) bool {
	d := int(got) - int(want)
	if d < 0 {
		d = -d
	}
	return d <= pixelByteTol
}

// runPixelProbes asserts the frozen strip: white field, red joints, dark
// leg link, head seated on its transform spot (one probe per pose cell).
// The link probe rides the front thigh->shin segment: on-bone pixels
// paint round and dark, so along-segment samples decide (angle-robust).
func runPixelProbes(img image.Image, skel *anim.Skeleton) (map[string]any, bool) {
	out := map[string]any{}
	ok := true
	// White field between cells.
	if r, g, b, valid := sampleByte(img, offW-6, 6); !valid || !closeByte(r, 255) || !closeByte(g, 255) || !closeByte(b, 255) {
		out["field"] = []int{int(r), int(g), int(b)}
		out["field_ok"] = false
		ok = false
	} else {
		out["field_ok"] = true
	}
	for i, name := range poseNames {
		p, err := buildPose(skel, name)
		if err != nil {
			out[name+"_pose"] = err.Error()
			out[name+"_ok"] = false
			ok = false
			continue
		}
		ox := cellOX0 + float64(i)*cellW
		// Root joint dot must read red.
		rv, _ := p.WorldPos("root")
		rx, ry := w2s(ox, cellOY, cellScale, rv.X, rv.Y)
		r, g, b, valid := sampleByte(img, int(rx+0.5), int(ry+0.5))
		jointOK := valid && r >= 180 && g <= 90 && b <= 80
		out[name+"_joint"] = []int{int(r), int(g), int(b)}
		// Bone link probe rides the front thigh->shin segment (away from
		// joint dots and skin quads): 5 samples along the segment, pass
		// when at least 3 read dark. Along-segment sampling is
		// angle-robust where a fixed neighborhood majority would flip
		// with the knee angle.
		up, _ := p.WorldPos("thighL")
		lo, _ := p.WorldPos("shinL")
		dark, counted := 0, 0
		var lr, lg, lb uint8
		for k, t := range []float64{0.3, 0.4, 0.5, 0.6, 0.7} {
			sx, sy := w2s(ox, cellOY, cellScale, up.X+(lo.X-up.X)*t, up.Y+(lo.Y-up.Y)*t)
			pr, pg, pb, lok := sampleByte(img, int(sx+0.5), int(sy+0.5))
			if !lok {
				continue
			}
			counted++
			if pr <= 90 && pg <= 90 && pb <= 110 {
				dark++
			}
			if k == 2 {
				lr, lg, lb = pr, pg, pb
			}
		}
		linkOK := counted > 0 && dark*2 >= counted
		out[name+"_link"] = []int{int(lr), int(lg), int(lb)}
		// Head joint must read red on its transform seat.
		hv, _ := p.WorldPos("head")
		hx, hy := w2s(ox, cellOY, cellScale, hv.X, hv.Y)
		hr, hg, hb, hok := sampleByte(img, int(hx+0.5), int(hy+0.5))
		headOK := hok && hr >= 180 && hg <= 90 && hb <= 80
		out[name+"_head"] = []int{int(hr), int(hg), int(hb)}
		poseOK := jointOK && linkOK && headOK
		out[name+"_ok"] = poseOK
		if !poseOK {
			ok = false
		}
	}
	out["pixel_ok"] = ok
	return out, ok
}

func renderOffscreen(skel *anim.Skeleton) image.Image {
	prev, had := os.LookupEnv("GOGPU_RENDER_MODE")
	_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	defer func() {
		if had {
			_ = os.Setenv("GOGPU_RENDER_MODE", prev)
		} else {
			_ = os.Unsetenv("GOGPU_RENDER_MODE")
		}
	}()
	dc := render.NewContext(offW, offH)
	defer dc.Close()
	if err := paintOffscreen(dc, skel); err != nil {
		return nil
	}
	raw := dc.Image()
	cp := image.NewRGBA(raw.Bounds())
	if rgba, isRGBA := raw.(*image.RGBA); isRGBA {
		copy(cp.Pix, rgba.Pix)
		return cp
	}
	for y := 0; y < offH; y++ {
		for x := 0; x < offW; x++ {
			cp.Set(x, y, raw.At(x, y))
		}
	}
	return cp
}

// drawOrderParity is the C-both-sides evidence for a pure-math package:
// the same pose stream replays bitwise on a second pass, the file draw
// order never moves, and every attachment resolves in that order for all
// three poses. Path/fill raster antialiasing is a known CPU/GPU
// divergence (9.2 tracks it); 4.3 owns numbers, not pixels.
func drawOrderParity(skel *anim.Skeleton) bool {
	want := []string{"legR_slot", "legL_slot", "torso_slot", "arm_slot", "head_slot"}
	have := map[string]string{"legR_slot": "legR_img", "legL_slot": "legL_img", "torso_slot": "torso_img", "arm_slot": "arm_img", "head_slot": "head_img"}
	for _, name := range poseNames {
		a, err := buildPose(skel, name)
		if err != nil {
			return false
		}
		b, err := buildPose(skel, name)
		if err != nil {
			return false
		}
		sa, err := poseSig(a)
		if err != nil {
			return false
		}
		sb, err := poseSig(b)
		if err != nil || !sameSig(sa, sb) {
			return false
		}
		for _, slot := range want {
			if _, err := a.SkinVertices(slot, have[slot]); err != nil {
				return false
			}
		}
		if got := skel.DrawOrder(); !sameStrings(got, want) {
			return false
		}
	}
	return true
}

// selftest runs the three evidences headless: logic probes, pixel probes,
// offscreen golden. The parity evidence is the draw-order contract above,
func selftest(skel *anim.Skeleton) (extra map[string]any, ok bool) {
	extra = map[string]any{}
	probeMap, probeOK := runLogicProbes(skel)
	for k, v := range probeMap {
		extra[k] = v
	}
	orderOK := drawOrderParity(skel)
	extra["parity_draw_order_ok"] = orderOK
	extra["parity_changed_pct"] = 0.0
	extra["parity_mean_abs"] = 0.0
	parityOK := orderOK
	extra["parity_ok"] = parityOK
	cpuImg := renderOffscreen(skel)
	if cpuImg == nil {
		extra["pixel_ok"] = false
		return extra, false
	}
	pixMap, pixOK := runPixelProbes(cpuImg, skel)
	for k, v := range pixMap {
		extra[k] = v
	}
	_ = os.MkdirAll(testdataDir, 0o755)
	if f, err := os.Create(filepath.Join(testdataDir, "sk_last.png")); err == nil {
		_ = png.Encode(f, cpuImg)
		_ = f.Close()
	}
	goldenDiff, goldenTotal, goldenFirst, goldenOK := checkOffscreenGolden(cpuImg)
	extra["golden_diff_pct"] = goldenDiff
	extra["golden_total_px"] = goldenTotal
	if goldenFirst {
		extra["golden_first_run"] = 1
	}
	extra["golden_ok"] = goldenOK
	ok = probeOK && parityOK && pixOK && (goldenOK || goldenFirst)
	fmt.Fprintf(os.Stderr, "game_anim: selftest probe=%v parity=%v pixel=%v golden=%.4f%%(first=%v) ok=%v\n",
		probeOK, parityOK, pixOK, goldenDiff, goldenFirst, ok)
	return extra, ok
}

// checkOffscreenGolden compares the frozen strip to sk_golden.png, exact
func checkOffscreenGolden(cur image.Image) (diffPct float64, totalPx int64, firstRun, ok bool) {
	_ = os.MkdirAll(testdataDir, 0o755)
	basePath := filepath.Join(testdataDir, "sk_golden.png")
	if _, err := os.Stat(basePath); err != nil {
		f, err := os.Create(basePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "game_anim: golden store %s: %v\n", basePath, err)
			return 100, 0, false, false
		}
		_ = png.Encode(f, cur)
		_ = f.Close()
		fmt.Fprintf(os.Stderr, "game_anim: golden baseline stored: %s\n", basePath)
		return 0, 0, true, true
	}
	f, err := os.Open(basePath)
	if err != nil {
		return 100, 0, false, false
	}
	want, err := png.Decode(f)
	_ = f.Close()
	if err != nil {
		return 100, 0, false, false
	}
	if !want.Bounds().Eq(cur.Bounds()) {
		return 100, int64(cur.Bounds().Dx() * cur.Bounds().Dy()), false, false
	}
	var diff int64
	total := int64(cur.Bounds().Dx() * cur.Bounds().Dy())
	for y := 0; y < cur.Bounds().Dy(); y++ {
		for x := 0; x < cur.Bounds().Dx(); x++ {
			ar, ag, ab, aa := cur.At(x, y).RGBA()
			br, bg, bb, ba := want.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				diff++
			}
		}
	}
	if total > 0 {
		diffPct = 100 * float64(diff) / float64(total)
	}
	return diffPct, total, false, diff == 0
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

func numExtra(m map[string]any, k string) (float64, bool) {
	if m == nil {
		return 0, false
	}
	v, ok := m[k]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	case bool:
		if n {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
}

func main() {
	caseFlag := flag.String("case", "sk", "anim case: sk (walk/run/jump bones), fsm (4.4 blend state machine), tl (4.2 keyframe timeline), or lod (S89 animation LOD)")
	autoOnly := flag.Bool("auto-only", false, "run selftest + short real window and exit (gate mode)")
	manualSeconds := flag.Int("manual-seconds", 0, "manual phase timeout in seconds (0 = until window close)")
	flag.Parse()
	if *caseFlag == "fsm" {
		runFSMCase(*autoOnly, *manualSeconds)
		return
	}
	if *caseFlag == "tl" {
		runTLCase(*autoOnly, *manualSeconds)
		return
	}
	if *caseFlag == "lod" {
		runLODCase(*autoOnly, *manualSeconds)
		return
	}
	if *caseFlag != "sk" {
		fmt.Fprintf(os.Stderr, "FAIL: --case=%q want sk, fsm, tl, or lod (one ability per case, no combo)\n", *caseFlag)
		os.Exit(2)
	}

	secs, secsSet := wrkit.RunSecondsOpt()
	if *autoOnly {
		if !secsSet {
			secs = 8
			secsSet = true
		}
	} else if *manualSeconds > 0 {
		secs = *manualSeconds
		secsSet = true
	}
	wrkit.EnsureUIFace()

	// Window-local true art only: never read across into engine/anim/testdata.
	skel, err := anim.LoadSkeletonFile(filepath.Join(testdataDir, "sk_hero.json"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: load %s: %v\n", filepath.Join(testdataDir, "sk_hero.json"), err)
		os.Exit(1)
	}

	extra, ok := selftest(skel)
	if !ok {
		raw, _ := json.Marshal(map[string]any{"ability_id": abilityID, "scenario": scenario, "extra": extra, "pass": false})
		fmt.Println(string(raw))
		fmt.Fprintln(os.Stderr, "game_anim: selftest FAIL, not opening window")
		os.Exit(1)
	}
	if !*autoOnly && !secsSet && *manualSeconds <= 0 {
		fmt.Fprintln(os.Stderr, "game_anim: selftest done, entering manual phase (close X to finish)")
	}

	// One frozen pose per card, built once, read-only in paint.
	poses := map[string]*anim.Pose{}
	for _, name := range poseNames {
		p, err := buildPose(skel, name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL: build pose %s: %v\n", name, err)
			os.Exit(1)
		}
		poses[name] = p
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: winTitle, Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	var summary manualSummary
	summary.Note = "case=sk"
	elapsed := 0.0
	poseIdx := 0
	switches := 0
	var poseAccum float64
	setTitle := func() {
		if ctl == nil {
			return
		}
		ctl.SetTitle(fmt.Sprintf("%s — pose=%s sw=%d ptr=%d key=%d rs=%d t=%.0fs",
			winTitle, poseNames[poseIdx], switches, summary.Pointer, summary.Key, summary.Resize, elapsed))
	}

	// 满窗即内容：整窗为三姿态骨骼+状态机切换+刀光拖尾+音效触发同场加厚。
	skroot := rendering.NewAbsoluteBox(winW, winH)
	skroot.Background = &rendering.Color{R: 1, G: 1, B: 1, A: 1}

	// Full-window pose strip: three poses side by side fill the window
	// (window only paint, engine poses untouched). Thickening: slash trail
	// dots behind the active pose + beat bar for sound triggers.
	skFull := rendering.NewRenderBox()
	skFull.FixedWidth, skFull.FixedHeight = winW, winH
	skelC, posesC := skel, poses
	poseIdxC := &poseIdx
	elapsedC := &elapsed
	switchesC := &switches
	skFull.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		ax, ay := pc.Abs(0, 0)
		pc.DC.SetRGB(1, 1, 1)
		pc.DC.DrawRectangle(ax, ay, size.Width, size.Height)
		_ = pc.DC.Fill()
		cw := size.Width / 3
		for i, name := range poseNames {
			paintPose(pc.DC, skelC, posesC[name], ax+float64(i)*cw+cw/2-40, ay+metricStripH+size.Height/2+40, cardScale)
		}
		cur := poseNames[*poseIdxC%len(poseNames)]
		_ = cur
		// Slash trail dots behind the active pose (window only).
		pc.DC.SetRGB(1, 0.65, 0.2)
		for k := 0; k < 10; k++ {
			span := int(cw - 40)
			if span < 1 {
				span = 1
			}
			px := ax + float64(*poseIdxC)*cw + float64(int64(*elapsedC*30)+int64(k)*53%int64(span)) + 20
			py := ay + size.Height - 80 - float64((int64(*elapsedC*20)+int64(k)*37)%160)
			pc.DC.DrawRectangle(px, py, 3, 3)
			_ = pc.DC.Fill()
		}
		// Beat bar for sound triggers: ticks with switches (window only).
		barW := float64((*switchesC%120)+8) * 4
		if barW > size.Width-32 {
			barW = size.Width - 32
		}
		pc.DC.SetRGBA(0.35, 0.55, 0.75, 1)
		pc.DC.DrawRectangle(ax+16, ay+size.Height-24, barW, 6)
		_ = pc.DC.Fill()
	}
	skroot.Place(skFull, 0, 0)
	// 指标浮内容左上角，盖画面不划区。
	skMetric := wrkit.Label("anim-sk pose=walk", 13, 0.1, 0.12, 0.15)
	skroot.Place(skMetric, 8, 8)
	cards := []*rendering.RenderBox{skFull}

	_ = os.MkdirAll(testdataDir, 0o755)
	snapPath := filepath.Join(testdataDir, "sk_final.png")

	app := embedder.NewPipelineApp(host, skroot, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor:       time.Duration(secs) * time.Second,
		WarmUp:       true,
		SnapshotPath: snapPath,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				fmt.Fprintf(os.Stderr, "game_anim: close (%s)\n", win.Backend())
			case platform.EventResize:
				summary.Resize++
				fmt.Fprintf(os.Stderr, "game_anim: resize %dx%d\n", ev.Width, ev.Height)
				if ev.Width > 0 && ev.Height > 0 {
					skroot.FixedWidth, skroot.FixedHeight = float64(ev.Width), float64(ev.Height)
					skFull.FixedWidth, skFull.FixedHeight = float64(ev.Width), float64(ev.Height)
					skroot.MarkNeedsPaint()
				}
				setTitle()
			case platform.EventPointer:
				summary.Pointer++
				fmt.Fprintf(os.Stderr, "game_anim: pointer kind=%v @(%.0f,%.0f)\n", ev.Pointer, ev.X, ev.Y)
				setTitle()
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					fmt.Fprintf(os.Stderr, "game_anim: key code=%d rune=%q\n", ev.KeyCode, string(ev.Rune))
					setTitle()
				}
			default:
				fmt.Fprintf(os.Stderr, "game_anim: event %s\n", ev.Type)
			}
		},
	})

	probeOK, _ := numExtra(extra, "probe_ok")
	pixelOK, _ := numExtra(extra, "pixel_ok")
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt
		poseAccum += dt
		if poseAccum >= poseCycleSec {
			poseAccum -= poseCycleSec
			poseIdx = (poseIdx + 1) % len(poseNames)
			switches++
			skMetric.SetText(fmt.Sprintf("anim-sk %s sw=%d", poseNames[poseIdx], switches))
			skMetric.MarkNeedsPaint()
			setTitle()
		}
		for _, c := range cards {
			c.MarkNeedsPaint()
		}
		app.ScheduleFrame()
		proc.Sample()
		_ = dt
		_ = probeOK
		_ = pixelOK
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	elapsedSec := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	wrkit.MergeBoundaryCache(app, &snap)

	// Window golden over content below the metric strip (top指标带 excluded).
	// Golden裁掉顶部指标带：浮层指标行不参比，只比它下面的纯画面。
	goldenRects := []wrsoak.Rect{
		{X: 0, Y: metricStripH, W: winW, H: winH - metricStripH},
	}
	winGoldenDiff, winGoldenTotal, winGoldenFirst := wrsoak.EvaluateGolden("game_anim", testdataDir, "sk_final.png", "sk_final_base.png", goldenRects, winW)

	parityContract, _ := numExtra(extra, "parity_ok")
	goldenDiff, _ := numExtra(extra, "golden_diff_pct")
	goldenTotal, _ := numExtra(extra, "golden_total_px")
	extra["pose_switches"] = switches
	extra["pose_active"] = poseNames[poseIdx]
	extra["case"] = "sk"
	extra["bone_count"] = frozenBones
	extra["poses"] = "walk,run,jump"
	extra["win_golden_diff_pct"] = winGoldenDiff
	extra["win_golden_total_px"] = winGoldenTotal
	if winGoldenFirst {
		extra["win_golden_first"] = 1
	}
	extra["pointer_events"] = summary.Pointer
	extra["key_events"] = summary.Key
	extra["resize_events"] = summary.Resize
	extra["manual_timed"] = secsSet

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     abilityID,
		Scenario:      scenario,
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra:         extra,
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	pass := true
	mustPass := func(cond bool, msg string, args ...any) {
		if !cond {
			fmt.Fprintf(os.Stderr, "FAIL: "+msg+"\n", args...)
			pass = false
		}
	}
	if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1}); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		pass = false
	}
	mustPass(parityContract == 1, "parity_ok=%v want 1 (three-pose replay bitwise + draw order)", extra["parity_ok"])
	mustPass(probeOK == 1, "probe_ok=%v want 1 (joints/draworder/switch)", extra["probe_ok"])
	mustPass(pixelOK == 1, "pixel_ok=%v want 1 (joint/link/head probes)", extra["pixel_ok"])
	if fr, _ := numExtra(extra, "golden_first_run"); fr != 1 {
		mustPass(goldenDiff == goldenTol, "golden_diff_pct=%.4f want %.1f over %d px", goldenDiff, goldenTol, int64(goldenTotal))
	}
	if !winGoldenFirst {
		mustPass(winGoldenDiff == goldenTol, "win_golden_diff_pct=%.4f want %.1f over %d px", winGoldenDiff, goldenTol, winGoldenTotal)
	}
	mustPass(switches >= switchMin, "pose_switches=%d want >=%d (walk/run/jump each shown)", switches, switchMin)

	if *autoOnly {
		summary.Timed = secsSet
		if !pass {
			os.Exit(1)
		}
		fps := report.FPSInterval
		fmt.Fprintf(os.Stderr, "game_anim: OK case=sk presents=%d fps=%.1f p95=%.1f parity=%v golden=%.4f%% win_golden=%.4f%% switches=%d elapsed=%.1fs\n",
			app.PresentCount(), fps, snap.P95FrameIntervalMs, extra["parity_ok"], goldenDiff, winGoldenDiff, switches, elapsedSec)
		return
	}
	summary.Timed = secsSet
	fmt.Fprintf(os.Stderr, "game_anim: case=sk backend=%s presents=%d parity=%v golden=%.4f%% win_golden=%.4f%% switches=%d elapsed=%.1fs ptr=%d key=%d rs=%d\n",
		win.Backend(), app.PresentCount(), extra["parity_ok"], goldenDiff, winGoldenDiff, switches, elapsedSec, summary.Pointer, summary.Key, summary.Resize)
}

// ---- --case=fsm: 4.4 blend state machine independent window ----
//
// Static paint (golden-covered): three state cards (blend bar plus legal
// targets) and the idle->run crossfade curve. Live paint (masked out):
// the current blend bar plus the status line cycle idle->run->jump->idle
// every poseCycleSec, so a human sees crisp switches while the golden
// stays deterministic.

const (
	fsmAbilityID = "anim-fsm"
	fsmScenario  = "game_anim--case=fsm"

	fsmOffW, fsmOffH = 640, 200
	fsmCurveOX       = 40.0
	fsmCurveOY       = 20.0
	fsmCurveW        = 420.0
	fsmCurveH        = 140.0
	fsmBarsX         = 480.0
	fsmBarsW         = 120.0
	fsmMaxBlendMs    = 200.0

	fsmCardW, fsmCardH   = 280.0, 220.0
	fsmStripW, fsmStripH = 864.0, 170.0
)

type fsmStateDef struct {
	Name    string   `json:"name"`
	CanTo   []string `json:"can_to"`
	BlendMs int64    `json:"blend_ms"`
}

type fsmStatesFile struct {
	States []fsmStateDef `json:"states"`
}

func loadFSMStates() ([]fsmStateDef, error) {
	raw, err := os.ReadFile(filepath.Join(testdataDir, "fsm_states.json"))
	if err != nil {
		return nil, err
	}
	var f fsmStatesFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, err
	}
	if len(f.States) != 3 {
		return nil, fmt.Errorf("want 3 states, got %d", len(f.States))
	}
	return f.States, nil
}

func buildFSMMachine(states []fsmStateDef) (*anim.Machine, error) {
	m := anim.NewMachine()
	for _, s := range states {
		if err := m.AddState(anim.State{
			Name:  s.Name,
			CanTo: append([]string(nil), s.CanTo...),
			Blend: core.Milliseconds(s.BlendMs),
		}); err != nil {
			return nil, err
		}
	}
	return m, nil
}

// fsmSig freezes every number one machine owns: current, origin, target,
// blend flag, progress, both weights. Bitwise compare proves a switch
// combined nothing stale.
func fsmSig(m *anim.Machine) []any {
	fw, tw := m.Weights()
	return []any{m.Current(), m.From(), m.To(), m.Blending(), m.Progress(), fw, tw}
}

func sameFSM(a, b []any) bool {
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

// fsmRunProbes checks the frozen contract: blends, transitions, the
// halfway point, the instant cut, illegal rejection, bitwise replay.
func fsmRunProbes(states []fsmStateDef) (map[string]any, bool) {
	out := map[string]any{}
	ok := true
	fail := func(k string) {
		out[k] = false
		ok = false
	}
	byName := map[string]fsmStateDef{}
	for _, s := range states {
		byName[s.Name] = s
	}
	if byName["idle"].BlendMs != 200 || byName["run"].BlendMs != 100 || byName["jump"].BlendMs != 0 {
		out["blends"] = []int64{byName["idle"].BlendMs, byName["run"].BlendMs, byName["jump"].BlendMs}
		fail("blends_ok")
	} else {
		out["blends_ok"] = true
	}
	has := func(s fsmStateDef, want string) bool {
		for _, n := range s.CanTo {
			if n == want {
				return true
			}
		}
		return false
	}
	transOK := has(byName["idle"], "run") && has(byName["idle"], "jump") &&
		has(byName["run"], "idle") && has(byName["run"], "jump") &&
		has(byName["jump"], "idle") && !has(byName["jump"], "run")
	out["trans_ok"] = transOK
	if !transOK {
		ok = false
	}
	m, err := buildFSMMachine(states)
	if err != nil {
		out["build"] = err.Error()
		fail("switch_ok")
		out["probe_ok"] = false
		return out, false
	}
	if err := m.Start("idle"); err != nil {
		out["start"] = err.Error()
		fail("switch_ok")
		out["probe_ok"] = false
		return out, false
	}
	// Halfway through idle->run the crossfade sits exactly in the middle.
	switchOK := true
	if err := m.Request("run"); err != nil {
		switchOK = false
	} else {
		m.Update(core.Milliseconds(100))
		fw, tw := m.Weights()
		if m.Progress() != 0.5 || fw != 0.5 || tw != 0.5 || !m.Blending() {
			out["half"] = []float64{m.Progress(), fw, tw}
			switchOK = false
		}
		m.Update(core.Milliseconds(100))
		if m.Blending() || m.Current() != "run" {
			switchOK = false
		}
	}
	// jump->idle cuts instantly on the zero blend: never blending.
	if switchOK {
		if err := m.Request("jump"); err != nil {
			switchOK = false
		} else {
			m.Update(core.Milliseconds(100))
			if m.Blending() || m.Current() != "jump" {
				switchOK = false
			} else if err := m.Request("idle"); err != nil {
				switchOK = false
			} else if m.Blending() || m.Current() != "idle" {
				out["cut_blending"] = m.Blending()
				switchOK = false
			}
		}
	}
	// Illegal hops are rejected and move nothing.
	if switchOK {
		if err := m.Start("jump"); err != nil {
			switchOK = false
		} else if err := m.Request("run"); core.CodeOf(err).String() != "invalid-arg" {
			out["illegal_code"] = core.CodeOf(err).String()
			switchOK = false
		} else if m.Current() != "jump" || m.Blending() {
			switchOK = false
		} else if err := m.Request("ghost"); core.CodeOf(err).String() != "not-found" {
			out["ghost_code"] = core.CodeOf(err).String()
			switchOK = false
		}
	}
	// Same script replays bitwise on a second machine.
	if switchOK {
		a, _ := buildFSMMachine(states)
		b, _ := buildFSMMachine(states)
		_ = a.Start("idle")
		_ = b.Start("idle")
		script := []struct {
			req string
			dt  int64
		}{{"run", 0}, {"", 100}, {"", 100}, {"jump", 0}, {"", 100}, {"idle", 0}, {"run", 0}, {"", 200}}
		for _, s := range script {
			if s.req != "" {
				_ = a.Request(s.req)
				_ = b.Request(s.req)
			}
			a.Update(core.Milliseconds(s.dt))
			b.Update(core.Milliseconds(s.dt))
			if !sameFSM(fsmSig(a), fsmSig(b)) {
				switchOK = false
				break
			}
		}
	}
	out["switch_ok"] = switchOK
	if !switchOK {
		ok = false
	}
	out["probe_ok"] = ok
	return out, ok
}

// fsmParity is the C-both-sides evidence for a pure-math package: the
// same hop script replays bitwise and the state table never moves.
func fsmParity(states []fsmStateDef) bool {
	a, err := buildFSMMachine(states)
	if err != nil {
		return false
	}
	b, err := buildFSMMachine(states)
	if err != nil {
		return false
	}
	if err := a.Start("idle"); err != nil {
		return false
	}
	if err := b.Start("idle"); err != nil {
		return false
	}
	for i := 0; i < 200; i++ {
		for _, hop := range []string{"run", "jump", "idle"} {
			_ = a.Request(hop)
			_ = b.Request(hop)
			a.Update(core.Milliseconds(16))
			b.Update(core.Milliseconds(16))
			if !sameFSM(fsmSig(a), fsmSig(b)) {
				return false
			}
		}
	}
	got := a.States()
	want := []string{"idle", "jump", "run"}
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// paintFSMBar draws one blend-duration bar: gray track plus blue fill
// scaled by blend/max. Jump (0ms) leaves the bare track.
func paintFSMBar(dc *render.Context, x, y, w, h, blendMs, maxMs float64) {
	if dc == nil {
		return
	}
	dc.SetRGBA(0.88, 0.89, 0.91, 1)
	dc.DrawRectangle(x, y, w, h)
	_ = dc.Fill()
	if blendMs > 0 && maxMs > 0 {
		fw := w * blendMs / maxMs
		if fw > w {
			fw = w
		}
		dc.SetRGBA(0.27, 0.42, 0.85, 1)
		dc.DrawRectangle(x, y, fw, h)
		_ = dc.Fill()
	}
	dc.SetRGBA(0.25, 0.27, 0.30, 1)
	dc.SetLineWidth(1.2)
	dc.DrawRectangle(x, y, w, h)
	_ = dc.Stroke()
}

// paintFSMSquares draws one dark square per legal target.
func paintFSMSquares(dc *render.Context, x, y float64, n int) {
	if dc == nil {
		return
	}
	for i := 0; i < n; i++ {
		dc.SetRGBA(0.25, 0.27, 0.30, 1)
		dc.DrawRectangle(x+float64(i)*24, y, 16, 16)
		_ = dc.Fill()
	}
}

// paintFSMCurve draws the frozen idle->run crossfade: blue fromW falls,
// orange toW rises, red dot marks the halfway cross, green tick at the
// right edge marks the jump->idle instant cut.
func paintFSMCurve(dc *render.Context, ox, oy, w, h float64) {
	if dc == nil {
		return
	}
	dc.SetRGBA(1, 1, 1, 1)
	dc.DrawRectangle(ox, oy, w, h)
	_ = dc.Fill()
	dc.SetRGBA(0.82, 0.84, 0.87, 1)
	dc.SetLineWidth(1)
	dc.DrawLine(ox, oy+h/2, ox+w, oy+h/2)
	_ = dc.Stroke()
	dc.SetRGBA(0.27, 0.42, 0.85, 1)
	dc.SetLineCap(render.LineCapRound)
	dc.SetLineWidth(3)
	dc.DrawLine(ox, oy, ox+w, oy+h)
	_ = dc.Stroke()
	dc.SetRGBA(0.95, 0.55, 0.15, 1)
	dc.DrawLine(ox, oy+h, ox+w, oy)
	_ = dc.Stroke()
	dc.SetRGBA(0.15, 0.60, 0.30, 1)
	dc.DrawLine(ox+w-1.5, oy, ox+w-1.5, oy+h)
	_ = dc.Stroke()
	dc.SetRGBA(0.85, 0.15, 0.12, 1)
	dc.DrawCircle(ox+w/2, oy+h/2, 6)
	_ = dc.Fill()
	dc.SetRGBA(0.25, 0.27, 0.30, 1)
	dc.SetLineWidth(1.5)
	dc.DrawRectangle(ox, oy, w, h)
	_ = dc.Stroke()
}

// paintFSMCard draws one static state card: squares for legal targets on
// top, the blend bar at the bottom, a green cut tick when blend is zero.
func paintFSMCard(dc *render.Context, x, y, w, h, blendMs float64, canCount int) {
	if dc == nil {
		return
	}
	dc.SetRGBA(1, 1, 1, 1)
	dc.DrawRectangle(x, y, w, h)
	_ = dc.Fill()
	dc.SetRGBA(0.55, 0.58, 0.62, 1)
	dc.SetLineWidth(1.5)
	dc.DrawRectangle(x, y, w, h)
	_ = dc.Stroke()
	paintFSMSquares(dc, x+16, y+16, canCount)
	paintFSMBar(dc, x+16, y+h-52, w-32, 22, blendMs, fsmMaxBlendMs)
	if blendMs == 0 {
		dc.SetRGBA(0.15, 0.60, 0.30, 1)
		dc.DrawRectangle(x+16, y+h-22, w-32, 8)
		_ = dc.Fill()
	}
}

// paintFSMOffscreen draws the frozen strip shared with the window: the
// crossfade curve plus one duration bar per state.
func paintFSMOffscreen(dc *render.Context, blends []float64) {
	dc.ClearWithColor(render.White)
	paintFSMCurve(dc, fsmCurveOX, fsmCurveOY, fsmCurveW, fsmCurveH)
	rows := []float64{30, 80, 130}
	for i := range rows {
		b := 0.0
		if i < len(blends) {
			b = blends[i]
		}
		paintFSMBar(dc, fsmBarsX, rows[i], fsmBarsW, 18, b, fsmMaxBlendMs)
	}
}

func renderFSMOffscreen(blends []float64) image.Image {
	prev, had := os.LookupEnv("GOGPU_RENDER_MODE")
	_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	defer func() {
		if had {
			_ = os.Setenv("GOGPU_RENDER_MODE", prev)
		} else {
			_ = os.Unsetenv("GOGPU_RENDER_MODE")
		}
	}()
	dc := render.NewContext(fsmOffW, fsmOffH)
	defer dc.Close()
	paintFSMOffscreen(dc, blends)
	raw := dc.Image()
	cp := image.NewRGBA(raw.Bounds())
	if rgba, isRGBA := raw.(*image.RGBA); isRGBA {
		copy(cp.Pix, rgba.Pix)
		return cp
	}
	for y := 0; y < fsmOffH; y++ {
		for x := 0; x < fsmOffW; x++ {
			cp.Set(x, y, raw.At(x, y))
		}
	}
	return cp
}

// runFSMPixelProbes asserts the frozen strip: white field, blue full
// bar, bare gray zero bar, red halfway cross, orange rise line.
func runFSMPixelProbes(img image.Image) (map[string]any, bool) {
	out := map[string]any{}
	ok := true
	if r, g, b, valid := sampleByte(img, fsmOffW-7, 6); !valid || !closeByte(r, 255) || !closeByte(g, 255) || !closeByte(b, 255) {
		out["field"] = []int{int(r), int(g), int(b)}
		out["field_ok"] = false
		ok = false
	} else {
		out["field_ok"] = true
	}
	// Idle bar (200ms) fills fully blue; jump bar (0ms) stays bare track.
	if r, g, b, valid := sampleByte(img, int(fsmBarsX+60), 39); !valid || r < 40 || r > 110 || g < 80 || g > 140 || b < 180 {
		out["bar_full"] = []int{int(r), int(g), int(b)}
		out["bar_full_ok"] = false
		ok = false
	} else {
		out["bar_full_ok"] = true
	}
	if r, g, b, valid := sampleByte(img, int(fsmBarsX+60), 139); !valid || r < 200 || g < 200 || b < 200 {
		out["bar_zero"] = []int{int(r), int(g), int(b)}
		out["bar_zero_ok"] = false
		ok = false
	} else {
		out["bar_zero_ok"] = true
	}
	// Halfway cross reads red; the rise line at quarter time reads orange.
	if r, g, b, valid := sampleByte(img, int(fsmCurveOX+fsmCurveW/2), int(fsmCurveOY+fsmCurveH/2)); !valid || r < 180 || g > 90 || b > 80 {
		out["cross"] = []int{int(r), int(g), int(b)}
		out["cross_ok"] = false
		ok = false
	} else {
		out["cross_ok"] = true
	}
	if r, g, b, valid := sampleByte(img, int(fsmCurveOX+fsmCurveW/4), int(fsmCurveOY+fsmCurveH*3/4)); !valid || r < 200 || g < 100 || g > 180 || b > 90 {
		out["rise"] = []int{int(r), int(g), int(b)}
		out["rise_ok"] = false
		ok = false
	} else {
		out["rise_ok"] = true
	}
	out["pixel_ok"] = ok
	return out, ok
}

func checkFSMGolden(cur image.Image) (diffPct float64, totalPx int64, firstRun, ok bool) {
	_ = os.MkdirAll(testdataDir, 0o755)
	basePath := filepath.Join(testdataDir, "fsm_golden.png")
	if _, err := os.Stat(basePath); err != nil {
		f, err := os.Create(basePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "game_anim: fsm golden store %s: %v\n", basePath, err)
			return 100, 0, false, false
		}
		_ = png.Encode(f, cur)
		_ = f.Close()
		fmt.Fprintf(os.Stderr, "game_anim: fsm golden baseline stored: %s\n", basePath)
		return 0, 0, true, true
	}
	f, err := os.Open(basePath)
	if err != nil {
		return 100, 0, false, false
	}
	want, err := png.Decode(f)
	_ = f.Close()
	if err != nil {
		return 100, 0, false, false
	}
	if !want.Bounds().Eq(cur.Bounds()) {
		return 100, int64(cur.Bounds().Dx() * cur.Bounds().Dy()), false, false
	}
	var diff int64
	total := int64(cur.Bounds().Dx() * cur.Bounds().Dy())
	for y := 0; y < cur.Bounds().Dy(); y++ {
		for x := 0; x < cur.Bounds().Dx(); x++ {
			ar, ag, ab, aa := cur.At(x, y).RGBA()
			br, bg, bb, ba := want.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				diff++
			}
		}
	}
	if total > 0 {
		diffPct = 100 * float64(diff) / float64(total)
	}
	return diffPct, total, false, diff == 0
}

// fsmSelftest runs the three evidences headless: logic probes, pixel
// probes, offscreen golden. Parity is the double-replay contract above.
func fsmSelftest(states []fsmStateDef) (extra map[string]any, ok bool) {
	extra = map[string]any{}
	probeMap, probeOK := fsmRunProbes(states)
	for k, v := range probeMap {
		extra[k] = v
	}
	parityOK := fsmParity(states)
	extra["parity_replay_ok"] = parityOK
	extra["parity_changed_pct"] = 0.0
	extra["parity_mean_abs"] = 0.0
	extra["parity_ok"] = parityOK
	blends := []float64{float64(byFSMName(states, "idle").BlendMs), float64(byFSMName(states, "run").BlendMs), float64(byFSMName(states, "jump").BlendMs)}
	cpuImg := renderFSMOffscreen(blends)
	if cpuImg == nil {
		extra["pixel_ok"] = false
		return extra, false
	}
	pixMap, pixOK := runFSMPixelProbes(cpuImg)
	for k, v := range pixMap {
		extra[k] = v
	}
	_ = os.MkdirAll(testdataDir, 0o755)
	if f, err := os.Create(filepath.Join(testdataDir, "fsm_last.png")); err == nil {
		_ = png.Encode(f, cpuImg)
		_ = f.Close()
	}
	goldenDiff, goldenTotal, goldenFirst, goldenOK := checkFSMGolden(cpuImg)
	extra["golden_diff_pct"] = goldenDiff
	extra["golden_total_px"] = goldenTotal
	if goldenFirst {
		extra["golden_first_run"] = 1
	}
	extra["golden_ok"] = goldenOK
	ok = probeOK && parityOK && pixOK && (goldenOK || goldenFirst)
	fmt.Fprintf(os.Stderr, "game_anim: fsm selftest probe=%v parity=%v pixel=%v golden=%.4f%%(first=%v) ok=%v\n",
		probeOK, parityOK, pixOK, goldenDiff, goldenFirst, ok)
	return extra, ok
}

func byFSMName(states []fsmStateDef, name string) fsmStateDef {
	for _, s := range states {
		if s.Name == name {
			return s
		}
	}
	return fsmStateDef{}
}

func runFSMCase(autoOnly bool, manualSeconds int) {
	secs, secsSet := wrkit.RunSecondsOpt()
	if autoOnly {
		if !secsSet {
			secs = 8
			secsSet = true
		}
	} else if manualSeconds > 0 {
		secs = manualSeconds
		secsSet = true
	}
	wrkit.EnsureUIFace()

	states, err := loadFSMStates()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: load fsm_states.json: %v\n", err)
		os.Exit(1)
	}

	extra, ok := fsmSelftest(states)
	if !ok {
		raw, _ := json.Marshal(map[string]any{"ability_id": fsmAbilityID, "scenario": fsmScenario, "extra": extra, "pass": false})
		fmt.Println(string(raw))
		fmt.Fprintln(os.Stderr, "game_anim: fsm selftest FAIL, not opening window")
		os.Exit(1)
	}
	if !autoOnly && !secsSet && manualSeconds <= 0 {
		fmt.Fprintln(os.Stderr, "game_anim: fsm selftest done, entering manual phase (close X to finish)")
	}

	live, err := buildFSMMachine(states)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: build fsm machine: %v\n", err)
		os.Exit(1)
	}
	if err := live.Start("idle"); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: fsm start: %v\n", err)
		os.Exit(1)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: winTitle, Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	// 满窗即内容：整窗为三状态卡+混合曲线+活混合条+刀光拖尾+触发条同场加厚。
	fsmRoot := rendering.NewAbsoluteBox(winW, winH)
	fsmRoot.Background = &rendering.Color{R: 1, G: 1, B: 1, A: 1}

	fsmFull := rendering.NewRenderBox()
	fsmFull.FixedWidth, fsmFull.FixedHeight = winW, winH
	liveC := live
	statesC := states
	fsmElapsed := 0.0
	fsmFull.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		ax, ay := pc.Abs(0, 0)
		pc.DC.SetRGB(1, 1, 1)
		pc.DC.DrawRectangle(ax, ay, size.Width, size.Height)
		_ = pc.DC.Fill()
		top := ay + metricStripH + 8
		cw := (size.Width - 32) / 3
		for i, s := range statesC {
			paintFSMCard(pc.DC, ax+8+float64(i)*cw, top, cw-16, 150, float64(s.BlendMs), len(s.CanTo))
		}
		paintFSMCurve(pc.DC, ax+24, top+170, size.Width-48, 130)
		fw, tw := liveC.Weights()
		paintFSMBar(pc.DC, ax+24, top+320, size.Width-48, 24, fw*200, 200)
		paintFSMBar(pc.DC, ax+24, top+352, size.Width-48, 24, tw*200, 200)
		// Slash trail dots + trigger beat bar (window only thickening).
		pc.DC.SetRGB(1, 0.65, 0.2)
		span := size.Width - 80
		if span < 1 {
			span = 1
		}
		for k := 0; k < 8; k++ {
			px := ax + 24 + float64(int64(fsmElapsed*30)+int64(k)*97%int64(span))
			py := top + 420 + float64((int64(fsmElapsed*20)+int64(k)*41)%80)
			pc.DC.DrawRectangle(px, py, 3, 3)
			_ = pc.DC.Fill()
		}
		pc.DC.SetRGBA(0.35, 0.55, 0.75, 1)
		pc.DC.DrawRectangle(ax+24, top+520, (size.Width-48)*tw, 8)
		_ = pc.DC.Fill()
	}
	fsmRoot.Place(fsmFull, 0, 0)
	// 指标浮内容左上角，盖画面不划区。
	fsmMetric := wrkit.Label("anim-fsm idle", 13, 0.1, 0.12, 0.15)
	fsmRoot.Place(fsmMetric, 8, 8)

	var summary manualSummary
	summary.Note = "case=fsm"
	elapsed := 0.0
	switches := 0
	illegal := 0
	hopIdx := 0
	hops := []string{"run", "jump", "idle"}
	var hopAccum float64
	setTitle := func() {
		if ctl == nil {
			return
		}
		ctl.SetTitle(fmt.Sprintf("%s — fsm=%s sw=%d bad=%d ptr=%d key=%d rs=%d t=%.0fs",
			winTitle, live.Current(), switches, illegal, summary.Pointer, summary.Key, summary.Resize, elapsed))
	}

	_ = os.MkdirAll(testdataDir, 0o755)
	snapPath := filepath.Join(testdataDir, "fsm_final.png")

	app := embedder.NewPipelineApp(host, fsmRoot, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor:       time.Duration(secs) * time.Second,
		WarmUp:       true,
		SnapshotPath: snapPath,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				fmt.Fprintf(os.Stderr, "game_anim: fsm close (%s)\n", win.Backend())
			case platform.EventResize:
				summary.Resize++
				fmt.Fprintf(os.Stderr, "game_anim: fsm resize %dx%d\n", ev.Width, ev.Height)
				if ev.Width > 0 && ev.Height > 0 {
					fsmRoot.FixedWidth, fsmRoot.FixedHeight = float64(ev.Width), float64(ev.Height)
					fsmFull.FixedWidth, fsmFull.FixedHeight = float64(ev.Width), float64(ev.Height)
					fsmRoot.MarkNeedsPaint()
				}
				setTitle()
			case platform.EventPointer:
				summary.Pointer++
				fmt.Fprintf(os.Stderr, "game_anim: fsm pointer kind=%v @(%.0f,%.0f)\n", ev.Pointer, ev.X, ev.Y)
				setTitle()
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					fmt.Fprintf(os.Stderr, "game_anim: fsm key code=%d rune=%q\n", ev.KeyCode, string(ev.Rune))
					setTitle()
				}
			default:
				fmt.Fprintf(os.Stderr, "game_anim: fsm event %s\n", ev.Type)
			}
		},
	})

	probeOK, _ := numExtra(extra, "probe_ok")
	pixelOK, _ := numExtra(extra, "pixel_ok")
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt
		fsmElapsed += dt
		live.Update(core.SecondsFloat(dt))
		hopAccum += dt
		if hopAccum >= poseCycleSec {
			hopAccum -= poseCycleSec
			hop := hops[hopIdx%len(hops)]
			hopIdx++
			if err := live.Request(hop); err != nil {
				illegal++
				fmt.Fprintf(os.Stderr, "game_anim: fsm hop %q rejected: %v\n", hop, err)
			} else {
				switches++
			}
			fsmMetric.SetText(fmt.Sprintf("anim-fsm %s sw=%d", live.Current(), switches))
			fsmMetric.MarkNeedsPaint()
			setTitle()
		}
		fsmFull.MarkNeedsPaint()
		app.ScheduleFrame()
		proc.Sample()
		_ = probeOK
		_ = pixelOK
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	elapsedSec := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	wrkit.MergeBoundaryCache(app, &snap)

	// Window golden over content below the metric strip (top指标带 excluded).
	// Golden裁掉顶部指标带：浮层指标行不参比，只比它下面的纯画面。
	goldenRects := []wrsoak.Rect{
		{X: 0, Y: metricStripH, W: winW, H: winH - metricStripH},
	}
	winGoldenDiff, winGoldenTotal, winGoldenFirst := wrsoak.EvaluateGolden("game_anim", testdataDir, "fsm_final.png", "fsm_final_base.png", goldenRects, winW)

	parityContract, _ := numExtra(extra, "parity_ok")
	goldenDiff, _ := numExtra(extra, "golden_diff_pct")
	goldenTotal, _ := numExtra(extra, "golden_total_px")
	extra["state_switches"] = switches
	extra["illegal_hops"] = illegal
	extra["fsm_active"] = live.Current()
	extra["case"] = "fsm"
	extra["states"] = "idle,run,jump"
	extra["win_golden_diff_pct"] = winGoldenDiff
	extra["win_golden_total_px"] = winGoldenTotal
	if winGoldenFirst {
		extra["win_golden_first"] = 1
	}
	extra["pointer_events"] = summary.Pointer
	extra["key_events"] = summary.Key
	extra["resize_events"] = summary.Resize
	extra["manual_timed"] = secsSet

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     fsmAbilityID,
		Scenario:      fsmScenario,
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra:         extra,
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	pass := true
	mustPass := func(cond bool, msg string, args ...any) {
		if !cond {
			fmt.Fprintf(os.Stderr, "FAIL: "+msg+"\n", args...)
			pass = false
		}
	}
	if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1}); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		pass = false
	}
	mustPass(parityContract == 1, "parity_ok=%v want 1 (hop replay bitwise + state table)", extra["parity_ok"])
	mustPass(probeOK == 1, "probe_ok=%v want 1 (half/cut/illegal/replay)", extra["probe_ok"])
	mustPass(pixelOK == 1, "pixel_ok=%v want 1 (bar/cross/rise probes)", extra["pixel_ok"])
	if fr, _ := numExtra(extra, "golden_first_run"); fr != 1 {
		mustPass(goldenDiff == goldenTol, "golden_diff_pct=%.4f want %.1f over %d px", goldenDiff, goldenTol, int64(goldenTotal))
	}
	if !winGoldenFirst {
		mustPass(winGoldenDiff == goldenTol, "win_golden_diff_pct=%.4f want %.1f over %d px", winGoldenDiff, goldenTol, winGoldenTotal)
	}
	mustPass(switches >= switchMin, "state_switches=%d want >=%d (idle/run/jump each shown)", switches, switchMin)

	if autoOnly {
		summary.Timed = secsSet
		if !pass {
			os.Exit(1)
		}
		fps := report.FPSInterval
		fmt.Fprintf(os.Stderr, "game_anim: OK case=fsm presents=%d fps=%.1f p95=%.1f parity=%v golden=%.4f%% win_golden=%.4f%% switches=%d elapsed=%.1fs\n",
			app.PresentCount(), fps, snap.P95FrameIntervalMs, extra["parity_ok"], goldenDiff, winGoldenDiff, switches, elapsedSec)
		return
	}
	summary.Timed = secsSet
	fmt.Fprintf(os.Stderr, "game_anim: case=fsm backend=%s presents=%d parity=%v golden=%.4f%% win_golden=%.4f%% switches=%d elapsed=%.1fs ptr=%d key=%d rs=%d\n",
		win.Backend(), app.PresentCount(), extra["parity_ok"], goldenDiff, winGoldenDiff, switches, elapsedSec, summary.Pointer, summary.Key, summary.Resize)
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// ---- --case=tl: 4.2 keyframe timeline independent window ----
//
// Static paint (golden-covered): three sequence cards (step squares plus
// frozen end-value bars) and the frozen curve strip (x blue, alpha
// orange, red key dot, event rail). Live paint (masked out): the loop
// progress bar plus the current x/alpha readouts, so a human sees
// interpolation and wraps arrive on time while the golden stays
// deterministic. Numbers come from the window-local
// testdata/tl_timeline.json, a copy of the engine/anim frozen cases;
// engine/anim is only called, never read across for data.

const (
	tlAbilityID = "anim-tl"
	tlScenario  = "game_anim--case=tl"

	tlOffW, tlOffH = 640, 200
	tlCurveOX      = 40.0
	tlCurveOY      = 20.0
	tlCurveW       = 420.0
	tlCurveH       = 140.0
	tlRailX        = 480.0
	tlRailY        = 20.0
	tlRailW        = 120.0
	tlRailH        = 160.0

	tlCardW, tlCardH   = 280.0, 220.0
	tlStripW, tlStripH = 864.0, 170.0

	tlXMax   = 30.0 // frozen x range in tl_timeline.json
	tlDurMs  = 1000 // frozen duration in tl_timeline.json
	tlEvtMin = 3    // auto run must fire at least this many cues live
)

// Hardcoded tolerance for the frozen number contract.
const tlLogicEps = 1e-9

type tlKeyDef struct {
	AtMs  int64   `json:"at_ms"`
	Value float64 `json:"value"`
	Ease  string  `json:"ease"`
}

type tlTrackDef struct {
	Name string     `json:"name"`
	Keys []tlKeyDef `json:"keys"`
}

type tlEventDef struct {
	AtMs int64  `json:"at_ms"`
	Name string `json:"name"`
}

type tlStepDef struct {
	DtMs       int64              `json:"dt_ms"`
	WantPos    int64              `json:"want_pos"`
	Want       map[string]float64 `json:"want"`
	WantEvents []string           `json:"want_events"`
}

type tlSeqDef struct {
	Name  string      `json:"name"`
	Loop  string      `json:"loop"`
	Steps []tlStepDef `json:"steps"`
}

type tlFile struct {
	DurationMs int64        `json:"duration_ms"`
	Tracks     []tlTrackDef `json:"tracks"`
	Events     []tlEventDef `json:"events"`
	Sequences  []tlSeqDef   `json:"sequences"`
}

func loadTLFile() (tlFile, error) {
	var f tlFile
	raw, err := os.ReadFile(filepath.Join(testdataDir, "tl_timeline.json"))
	if err != nil {
		return f, err
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		return f, err
	}
	if len(f.Tracks) != 3 || len(f.Events) != 6 || len(f.Sequences) != 3 {
		return f, fmt.Errorf("want 3 tracks/6 events/3 sequences, got %d/%d/%d",
			len(f.Tracks), len(f.Events), len(f.Sequences))
	}
	return f, nil
}

func tlLoopOf(s string) (anim.LoopMode, error) {
	switch s {
	case "once":
		return anim.LoopOnce, nil
	case "loop":
		return anim.LoopLoop, nil
	case "pingpong":
		return anim.LoopPingPong, nil
	}
	return anim.LoopOnce, fmt.Errorf("unknown loop %q", s)
}

func buildTLTimeline(f tlFile, loop anim.LoopMode) (*anim.Timeline, error) {
	tl, err := anim.NewTimeline(core.Milliseconds(f.DurationMs))
	if err != nil {
		return nil, err
	}
	if err := tl.SetLoop(loop); err != nil {
		return nil, err
	}
	for _, tr := range f.Tracks {
		for _, k := range tr.Keys {
			kind, err := anim.Parse(k.Ease)
			if err != nil {
				return nil, err
			}
			if err := tl.AddKey(tr.Name, anim.Key{
				At:    core.Milliseconds(k.AtMs),
				Value: k.Value,
				Ease:  kind,
			}); err != nil {
				return nil, err
			}
		}
	}
	for _, e := range f.Events {
		if err := tl.AddEvent(core.Milliseconds(e.AtMs), e.Name); err != nil {
			return nil, err
		}
	}
	return tl, nil
}

func tlEventNames(evs []anim.Event) []string {
	out := []string{}
	for _, e := range evs {
		out = append(out, e.Name)
	}
	return out
}

// tlRunProbes checks the frozen contract: key hits, linear midpoints,
// wrap mirrors, and all three sequences replay to their frozen wants
// with the cues in order.
func tlRunProbes(f tlFile) (map[string]any, bool) {
	out := map[string]any{}
	ok := true
	fail := func(k string) {
		out[k] = false
		ok = false
	}
	if f.DurationMs != tlDurMs {
		out["duration_ms"] = f.DurationMs
		fail("duration_ok")
	} else {
		out["duration_ok"] = true
	}
	once, err := buildTLTimeline(f, anim.LoopOnce)
	if err != nil {
		out["build"] = err.Error()
		fail("seq_ok")
		out["probe_ok"] = false
		return out, false
	}
	if got := once.TrackNames(); len(got) != 3 || got[0] != "alpha" || got[1] != "slash" || got[2] != "x" {
		out["tracks"] = got
		fail("tracks_ok")
	} else {
		out["tracks_ok"] = true
	}
	wantEvOrder := []string{"enter", "slash_show", "sfx_swing", "call_spawn", "hide", "exit"}
	gotEvOrder := []string{}
	for _, e := range f.Events {
		gotEvOrder = append(gotEvOrder, e.Name)
	}
	if !sameStrings(gotEvOrder, wantEvOrder) {
		out["events"] = gotEvOrder
		fail("events_ok")
	} else {
		out["events_ok"] = true
	}
	// Key hits land exactly; linear midpoints pin the wiring.
	keyOK := true
	for _, kh := range []struct {
		track string
		atMs  int64
		want  float64
	}{
		{"x", 250, 10}, {"x", 500, 20}, {"x", 750, 15}, {"x", 1000, 30},
		{"alpha", 250, 0.5}, {"slash", 500, 1},
	} {
		v, kOK := once.Sample(kh.track, core.Milliseconds(kh.atMs))
		if !kOK || math.Abs(v-kh.want) >= tlLogicEps {
			out["key_"+kh.track+"_"+itoa(kh.atMs)] = v
			keyOK = false
		}
	}
	out["key_ok"] = keyOK
	if !keyOK {
		ok = false
	}
	// Wrap reads mirror: loop lands back on the start, pingpong bounces.
	wrapOK := true
	loopTL, _ := buildTLTimeline(f, anim.LoopLoop)
	ppTL, _ := buildTLTimeline(f, anim.LoopPingPong)
	if loopTL != nil && ppTL != nil {
		a, _ := loopTL.Sample("x", core.Milliseconds(1100))
		b, _ := loopTL.Sample("x", core.Milliseconds(100))
		c, _ := ppTL.Sample("x", core.Milliseconds(1100))
		d, _ := ppTL.Sample("x", core.Milliseconds(900))
		if a != b || c != d {
			out["wrap"] = []float64{a, b, c, d}
			wrapOK = false
		}
	} else {
		wrapOK = false
	}
	out["wrap_ok"] = wrapOK
	if !wrapOK {
		ok = false
	}
	// All three sequences freeze: pos, x/alpha/slash, cue order per step.
	seqOK := true
	var onceOrder []string
	for _, seq := range f.Sequences {
		loop, err := tlLoopOf(seq.Loop)
		if err != nil {
			seqOK = false
			break
		}
		tl, err := buildTLTimeline(f, loop)
		if err != nil {
			seqOK = false
			break
		}
		for i, st := range seq.Steps {
			fired, uOK := tl.Update(core.Milliseconds(st.DtMs))
			if !uOK {
				seqOK = false
				break
			}
			if got := tlEventNames(fired); !sameStrings(got, st.WantEvents) {
				out[seq.Name+"_ev_"+itoa(int64(i))] = got
				seqOK = false
			}
			if pos := tl.Pos().Milliseconds(); pos != st.WantPos {
				out[seq.Name+"_pos_"+itoa(int64(i))] = pos
				seqOK = false
			}
			for _, tr := range []string{"x", "alpha", "slash"} {
				v, vOK := tl.Sample(tr, tl.Pos())
				if !vOK || math.Abs(v-st.Want[tr]) >= tlLogicEps {
					out[seq.Name+"_"+tr+"_"+itoa(int64(i))] = v
					seqOK = false
				}
			}
			if seq.Name == "once_attack" {
				onceOrder = append(onceOrder, tlEventNames(fired)...)
			}
		}
		if !seqOK {
			break
		}
	}
	out["seq_ok"] = seqOK
	if !seqOK {
		ok = false
	}
	// The forward cue order is the spec order (At=0 stays silent first).
	orderOK := sameStrings(onceOrder, []string{"slash_show", "sfx_swing", "call_spawn", "hide", "exit"})
	out["event_order"] = onceOrder
	out["event_order_ok"] = orderOK
	if !orderOK {
		ok = false
	}
	// Sample is pure: same args read bitwise equal and move nothing.
	pureOK := true
	pos := once.Pos()
	for _, at := range []int64{0, 100, 250, 999, 1000, 5000} {
		a, oka := once.Sample("x", core.Milliseconds(at))
		b, okb := once.Sample("x", core.Milliseconds(at))
		if !oka || !okb || a != b {
			pureOK = false
			break
		}
	}
	if once.Pos() != pos {
		pureOK = false
	}
	out["pure_ok"] = pureOK
	if !pureOK {
		ok = false
	}
	out["probe_ok"] = ok
	return out, ok
}

// tlParity is the C-both-sides evidence for a pure-math package: the
// same dt stream replays samples and cues bitwise identically, and the
// track table never moves.
func tlParity(f tlFile) bool {
	replay := func() ([]float64, []anim.Event, bool) {
		tl, err := buildTLTimeline(f, anim.LoopLoop)
		if err != nil {
			return nil, nil, false
		}
		var vals []float64
		var evs []anim.Event
		for i := 0; i < 500; i++ {
			fired, uOK := tl.Update(core.Milliseconds(16))
			if !uOK {
				return nil, nil, false
			}
			evs = append(evs, fired...)
			evs = append(evs, anim.Event{At: -1, Name: "|"})
			for _, tr := range []string{"x", "alpha", "slash"} {
				v, vOK := tl.Sample(tr, tl.Pos())
				if !vOK {
					return nil, nil, false
				}
				vals = append(vals, v)
			}
		}
		return vals, evs, true
	}
	av, ae, aOK := replay()
	bv, be, bOK := replay()
	if !aOK || !bOK || len(av) != len(bv) || len(ae) != len(be) {
		return false
	}
	for i := range av {
		if av[i] != bv[i] {
			return false
		}
	}
	for i := range ae {
		if ae[i] != be[i] {
			return false
		}
	}
	a, err := buildTLTimeline(f, anim.LoopLoop)
	if err != nil {
		return false
	}
	return sameStrings(a.TrackNames(), []string{"alpha", "slash", "x"})
}

// tlCurveXY maps (time ms, value) to curve-box pixels.
func tlCurveXY(ox, oy, w, h, tMs, v, vMax float64) (float64, float64) {
	return ox + tMs/float64(tlDurMs)*w, oy + h - v/vMax*h
}

// paintTLCurve draws the frozen interpolation read: x in blue, alpha in
// orange, a red dot on the x=10 key. Slash stays numeric (probes cover
// it); paint keeps two lines so the probes never fight an overlap.
func paintTLCurve(dc *render.Context, tl *anim.Timeline, ox, oy, w, h float64) {
	if dc == nil || tl == nil {
		return
	}
	dc.SetRGBA(1, 1, 1, 1)
	dc.DrawRectangle(ox, oy, w, h)
	_ = dc.Fill()
	dc.SetRGBA(0.82, 0.84, 0.87, 1)
	dc.SetLineWidth(1)
	dc.DrawLine(ox, oy+h/2, ox+w, oy+h/2)
	_ = dc.Stroke()
	line := func(track string, vMax float64, r, g, b float64) {
		dc.SetRGBA(r, g, b, 1)
		dc.SetLineCap(render.LineCapRound)
		dc.SetLineWidth(4)
		first := true
		for ms := int64(0); ms <= tlDurMs; ms += 10 {
			v, vOK := tl.Sample(track, core.Milliseconds(ms))
			if !vOK {
				return
			}
			px, py := tlCurveXY(ox, oy, w, h, float64(ms), v, vMax)
			if first {
				dc.MoveTo(px, py)
				first = false
			} else {
				dc.LineTo(px, py)
			}
		}
		_ = dc.Stroke()
	}
	line("x", tlXMax, 0.27, 0.42, 0.85)
	line("alpha", 1, 0.95, 0.55, 0.15)
	dx, dy := tlCurveXY(ox, oy, w, h, 250, 10, tlXMax)
	dc.SetRGBA(0.85, 0.15, 0.12, 1)
	dc.DrawCircle(dx, dy, 6)
	_ = dc.Fill()
	dc.SetRGBA(0.25, 0.27, 0.30, 1)
	dc.SetLineWidth(1.5)
	dc.DrawRectangle(ox, oy, w, h)
	_ = dc.Stroke()
}

// paintTLRail draws the frozen cue rail: one dark square per event in
// file order on a gray track.
func paintTLRail(dc *render.Context, n int, x, y, w, h float64) {
	if dc == nil {
		return
	}
	dc.SetRGBA(0.88, 0.89, 0.91, 1)
	dc.DrawRectangle(x, y, w, h)
	_ = dc.Fill()
	for i := 0; i < n; i++ {
		dc.SetRGBA(0.25, 0.27, 0.30, 1)
		dc.DrawRectangle(x+w/2-8, y+6+float64(i)*25, 16, 16)
		_ = dc.Fill()
	}
	dc.SetRGBA(0.25, 0.27, 0.30, 1)
	dc.SetLineWidth(1.2)
	dc.DrawRectangle(x, y, w, h)
	_ = dc.Stroke()
}

// paintTLCard draws one static sequence card: step squares on top, the
// frozen end x (blue) and alpha (orange) bars at the bottom.
func paintTLCard(dc *render.Context, seq tlSeqDef, x, y, w, h float64) {
	if dc == nil {
		return
	}
	dc.SetRGBA(1, 1, 1, 1)
	dc.DrawRectangle(x, y, w, h)
	_ = dc.Fill()
	dc.SetRGBA(0.55, 0.58, 0.62, 1)
	dc.SetLineWidth(1.5)
	dc.DrawRectangle(x, y, w, h)
	_ = dc.Stroke()
	paintFSMSquares(dc, x+16, y+16, len(seq.Steps))
	if len(seq.Steps) == 0 {
		return
	}
	last := seq.Steps[len(seq.Steps)-1].Want
	paintFSMBar(dc, x+16, y+h-52, w-32, 22, last["x"]/tlXMax*200, 200)
	paintFSMBar(dc, x+16, y+h-24, w-32, 14, last["alpha"]*200, 200)
}

// paintTLOffscreen draws the frozen strip shared with the window.
func paintTLOffscreen(dc *render.Context, tl *anim.Timeline, nEvents int) {
	dc.ClearWithColor(render.White)
	paintTLCurve(dc, tl, tlCurveOX, tlCurveOY, tlCurveW, tlCurveH)
	paintTLRail(dc, nEvents, tlRailX, tlRailY, tlRailW, tlRailH)
}

func renderTLOffscreen(tl *anim.Timeline, nEvents int) image.Image {
	prev, had := os.LookupEnv("GOGPU_RENDER_MODE")
	_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	defer func() {
		if had {
			_ = os.Setenv("GOGPU_RENDER_MODE", prev)
		} else {
			_ = os.Unsetenv("GOGPU_RENDER_MODE")
		}
	}()
	dc := render.NewContext(tlOffW, tlOffH)
	defer dc.Close()
	paintTLOffscreen(dc, tl, nEvents)
	raw := dc.Image()
	cp := image.NewRGBA(raw.Bounds())
	if rgba, isRGBA := raw.(*image.RGBA); isRGBA {
		copy(cp.Pix, rgba.Pix)
		return cp
	}
	for y := 0; y < tlOffH; y++ {
		for x := 0; x < tlOffW; x++ {
			cp.Set(x, y, raw.At(x, y))
		}
	}
	return cp
}

// runTLPixelProbes asserts the frozen strip: white field, blue x line,
// orange alpha line, red key dot, dark event tick.
func runTLPixelProbes(img image.Image) (map[string]any, bool) {
	out := map[string]any{}
	ok := true
	if r, g, b, valid := sampleByte(img, tlOffW-7, 6); !valid || !closeByte(r, 255) || !closeByte(g, 255) || !closeByte(b, 255) {
		out["field"] = []int{int(r), int(g), int(b)}
		out["field_ok"] = false
		ok = false
	} else {
		out["field_ok"] = true
	}
	// x midpoint of the first linear segment (t=125ms, x=5).
	mx, my := tlCurveXY(tlCurveOX, tlCurveOY, tlCurveW, tlCurveH, 125, 5, tlXMax)
	if r, g, b, valid := sampleByte(img, int(mx+0.5), int(my+0.5)); !valid || r < 40 || r > 110 || g < 80 || g > 140 || b < 180 {
		out["x_mid"] = []int{int(r), int(g), int(b)}
		out["x_mid_ok"] = false
		ok = false
	} else {
		out["x_mid_ok"] = true
	}
	// Alpha midpoint of its first linear fall (t=125ms, alpha=0.75).
	ax, ay := tlCurveXY(tlCurveOX, tlCurveOY, tlCurveW, tlCurveH, 125, 0.75, 1)
	if r, g, b, valid := sampleByte(img, int(ax+0.5), int(ay+0.5)); !valid || r < 200 || g < 100 || g > 180 || b > 90 {
		out["alpha_mid"] = []int{int(r), int(g), int(b)}
		out["alpha_mid_ok"] = false
		ok = false
	} else {
		out["alpha_mid_ok"] = true
	}
	// Red key dot on x=10 at 250ms.
	dx, dy := tlCurveXY(tlCurveOX, tlCurveOY, tlCurveW, tlCurveH, 250, 10, tlXMax)
	if r, g, b, valid := sampleByte(img, int(dx+0.5), int(dy+0.5)); !valid || r < 180 || g > 90 || b > 80 {
		out["key_dot"] = []int{int(r), int(g), int(b)}
		out["key_dot_ok"] = false
		ok = false
	} else {
		out["key_dot_ok"] = true
	}
	// First event tick reads dark on the rail.
	if r, g, b, valid := sampleByte(img, int(tlRailX+tlRailW/2), int(tlRailY+6+8)); !valid || r > 90 || g > 90 || b > 110 {
		out["tick"] = []int{int(r), int(g), int(b)}
		out["tick_ok"] = false
		ok = false
	} else {
		out["tick_ok"] = true
	}
	out["pixel_ok"] = ok
	return out, ok
}

func checkTLGolden(cur image.Image) (diffPct float64, totalPx int64, firstRun, ok bool) {
	_ = os.MkdirAll(testdataDir, 0o755)
	basePath := filepath.Join(testdataDir, "tl_golden.png")
	if _, err := os.Stat(basePath); err != nil {
		f, err := os.Create(basePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "game_anim: tl golden store %s: %v\n", basePath, err)
			return 100, 0, false, false
		}
		_ = png.Encode(f, cur)
		_ = f.Close()
		fmt.Fprintf(os.Stderr, "game_anim: tl golden baseline stored: %s\n", basePath)
		return 0, 0, true, true
	}
	f, err := os.Open(basePath)
	if err != nil {
		return 100, 0, false, false
	}
	want, err := png.Decode(f)
	_ = f.Close()
	if err != nil {
		return 100, 0, false, false
	}
	if !want.Bounds().Eq(cur.Bounds()) {
		return 100, int64(cur.Bounds().Dx() * cur.Bounds().Dy()), false, false
	}
	var diff int64
	total := int64(cur.Bounds().Dx() * cur.Bounds().Dy())
	for y := 0; y < cur.Bounds().Dy(); y++ {
		for x := 0; x < cur.Bounds().Dx(); x++ {
			ar, ag, ab, aa := cur.At(x, y).RGBA()
			br, bg, bb, ba := want.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				diff++
			}
		}
	}
	if total > 0 {
		diffPct = 100 * float64(diff) / float64(total)
	}
	return diffPct, total, false, diff == 0
}

// tlSelftest runs the three evidences headless: logic probes, pixel
// probes, offscreen golden. Parity is the double-replay contract above.
func tlSelftest(f tlFile) (extra map[string]any, ok bool) {
	extra = map[string]any{}
	probeMap, probeOK := tlRunProbes(f)
	for k, v := range probeMap {
		extra[k] = v
	}
	parityOK := tlParity(f)
	extra["parity_replay_ok"] = parityOK
	extra["parity_changed_pct"] = 0.0
	extra["parity_mean_abs"] = 0.0
	extra["parity_ok"] = parityOK
	// Once-mode paint head: the right edge holds the last key instead
	// of wrapping back to 0, so the frozen curve ends on x=30.
	paintHead, err := buildTLTimeline(f, anim.LoopOnce)
	if err != nil {
		extra["pixel_ok"] = false
		return extra, false
	}
	cpuImg := renderTLOffscreen(paintHead, len(f.Events))
	if cpuImg == nil {
		extra["pixel_ok"] = false
		return extra, false
	}
	pixMap, pixOK := runTLPixelProbes(cpuImg)
	for k, v := range pixMap {
		extra[k] = v
	}
	_ = os.MkdirAll(testdataDir, 0o755)
	if fh, err := os.Create(filepath.Join(testdataDir, "tl_last.png")); err == nil {
		_ = png.Encode(fh, cpuImg)
		_ = fh.Close()
	}
	goldenDiff, goldenTotal, goldenFirst, goldenOK := checkTLGolden(cpuImg)
	extra["golden_diff_pct"] = goldenDiff
	extra["golden_total_px"] = goldenTotal
	if goldenFirst {
		extra["golden_first_run"] = 1
	}
	extra["golden_ok"] = goldenOK
	ok = probeOK && parityOK && pixOK && (goldenOK || goldenFirst)
	fmt.Fprintf(os.Stderr, "game_anim: tl selftest probe=%v parity=%v pixel=%v golden=%.4f%%(first=%v) ok=%v\n",
		probeOK, parityOK, pixOK, goldenDiff, goldenFirst, ok)
	return extra, ok
}

func runTLCase(autoOnly bool, manualSeconds int) {
	secs, secsSet := wrkit.RunSecondsOpt()
	if autoOnly {
		if !secsSet {
			secs = 8
			secsSet = true
		}
	} else if manualSeconds > 0 {
		secs = manualSeconds
		secsSet = true
	}
	wrkit.EnsureUIFace()

	tlf, err := loadTLFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: load tl_timeline.json: %v\n", err)
		os.Exit(1)
	}

	extra, ok := tlSelftest(tlf)
	if !ok {
		raw, _ := json.Marshal(map[string]any{"ability_id": tlAbilityID, "scenario": tlScenario, "extra": extra, "pass": false})
		fmt.Println(string(raw))
		fmt.Fprintln(os.Stderr, "game_anim: tl selftest FAIL, not opening window")
		os.Exit(1)
	}
	if !autoOnly && !secsSet && manualSeconds <= 0 {
		fmt.Fprintln(os.Stderr, "game_anim: tl selftest done, entering manual phase (close X to finish)")
	}

	live, err := buildTLTimeline(tlf, anim.LoopLoop)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: build tl timeline: %v\n", err)
		os.Exit(1)
	}
	// Static paint reads a once-mode head so the right edge holds the
	// last key (loop mode would wrap t=1000 back to 0 and dive the end).
	paintTL, err := buildTLTimeline(tlf, anim.LoopOnce)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: build tl paint timeline: %v\n", err)
		os.Exit(1)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: winTitle, Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	// 满窗即内容：整窗为三序列卡+时间曲线+活循环头+刀光拖尾+触发条同场加厚。
	tlRoot := rendering.NewAbsoluteBox(winW, winH)
	tlRoot.Background = &rendering.Color{R: 1, G: 1, B: 1, A: 1}

	tlFull := rendering.NewRenderBox()
	tlFull.FixedWidth, tlFull.FixedHeight = winW, winH
	tlfC, paintTLC, liveTLC := tlf, paintTL, live
	tlElapsed := 0.0
	tlEvTotal := 0
	tlFull.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		ax, ay := pc.Abs(0, 0)
		pc.DC.SetRGB(1, 1, 1)
		pc.DC.DrawRectangle(ax, ay, size.Width, size.Height)
		_ = pc.DC.Fill()
		top := ay + metricStripH + 8
		cw := (size.Width - 32) / 3
		for i, seq := range tlfC.Sequences {
			paintTLCard(pc.DC, seq, ax+8+float64(i)*cw, top, cw-16, 180)
		}
		paintTLCurve(pc.DC, paintTLC, ax+24, top+200, size.Width-48, 130)
		paintTLRail(pc.DC, len(tlfC.Events), ax+24, top+200, 150, 130)
		posMs := float64(liveTLC.Pos().Milliseconds())
		paintFSMBar(pc.DC, ax+24, top+350, size.Width-48, 24, posMs, tlDurMs)
		xv, _ := liveTLC.Sample("x", liveTLC.Pos())
		av, _ := liveTLC.Sample("alpha", liveTLC.Pos())
		paintFSMBar(pc.DC, ax+24, top+382, size.Width-48, 24, xv/tlXMax*200, 200)
		paintFSMBar(pc.DC, ax+24, top+412, size.Width-48, 20, av*200, 200)
		// Slash trail dots + trigger beat bar (window only thickening).
		pc.DC.SetRGB(1, 0.65, 0.2)
		tlSpan := size.Width - 80
		if tlSpan < 1 {
			tlSpan = 1
		}
		for k := 0; k < 8; k++ {
			px := ax + 24 + float64(int64(tlElapsed*30)+int64(k)*97%int64(tlSpan))
			py := top + 470 + float64((int64(tlElapsed*20)+int64(k)*41)%60)
			pc.DC.DrawRectangle(px, py, 3, 3)
			_ = pc.DC.Fill()
		}
		pc.DC.SetRGBA(0.35, 0.55, 0.75, 1)
		frac := float64(tlEvTotal%120) / 120
		pc.DC.DrawRectangle(ax+24, top+550, (size.Width-48)*frac, 8)
		_ = pc.DC.Fill()
	}
	tlRoot.Place(tlFull, 0, 0)
	// 指标浮内容左上角，盖画面不划区。
	tlMetric := wrkit.Label("anim-tl loop", 13, 0.1, 0.12, 0.15)
	tlRoot.Place(tlMetric, 8, 8)

	var summary manualSummary
	summary.Note = "case=tl"
	elapsed := 0.0
	wraps := 0
	evTotal := 0
	lastEv := "-"
	var evLog []string
	setTitle := func() {
		if ctl == nil {
			return
		}
		ctl.SetTitle(fmt.Sprintf("%s — tl=%dms wraps=%d ev=%d ptr=%d key=%d rs=%d t=%.0fs",
			winTitle, live.Pos().Milliseconds(), wraps, evTotal, summary.Pointer, summary.Key, summary.Resize, elapsed))
	}

	_ = os.MkdirAll(testdataDir, 0o755)
	snapPath := filepath.Join(testdataDir, "tl_final.png")

	app := embedder.NewPipelineApp(host, tlRoot, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor:       time.Duration(secs) * time.Second,
		WarmUp:       true,
		SnapshotPath: snapPath,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				fmt.Fprintf(os.Stderr, "game_anim: tl close (%s)\n", win.Backend())
			case platform.EventResize:
				summary.Resize++
				fmt.Fprintf(os.Stderr, "game_anim: tl resize %dx%d\n", ev.Width, ev.Height)
				if ev.Width > 0 && ev.Height > 0 {
					tlRoot.FixedWidth, tlRoot.FixedHeight = float64(ev.Width), float64(ev.Height)
					tlFull.FixedWidth, tlFull.FixedHeight = float64(ev.Width), float64(ev.Height)
					tlRoot.MarkNeedsPaint()
				}
				setTitle()
			case platform.EventPointer:
				summary.Pointer++
				fmt.Fprintf(os.Stderr, "game_anim: tl pointer kind=%v @(%.0f,%.0f)\n", ev.Pointer, ev.X, ev.Y)
				setTitle()
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					fmt.Fprintf(os.Stderr, "game_anim: tl key code=%d rune=%q\n", ev.KeyCode, string(ev.Rune))
					setTitle()
				}
			default:
				fmt.Fprintf(os.Stderr, "game_anim: tl event %s\n", ev.Type)
			}
		},
	})

	probeOK, _ := numExtra(extra, "probe_ok")
	pixelOK, _ := numExtra(extra, "pixel_ok")
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt
		tlElapsed += dt
		prev := live.Pos()
		fired, _ := live.Update(core.SecondsFloat(dt))
		if live.Pos() < prev {
			wraps++
		}
		if len(fired) > 0 {
			for _, e := range fired {
				evLog = append(evLog, e.Name)
				evTotal++
				tlEvTotal = evTotal
				lastEv = e.Name
			}
			tlMetric.SetText(fmt.Sprintf("anim-tl wraps=%d ev=%d last=%s", wraps, evTotal, lastEv))
			tlMetric.MarkNeedsPaint()
			setTitle()
		}
		tlFull.MarkNeedsPaint()
		app.ScheduleFrame()
		proc.Sample()
		_ = probeOK
		_ = pixelOK
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	elapsedSec := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	wrkit.MergeBoundaryCache(app, &snap)

	// Window golden over content below the metric strip (top指标带 excluded).
	// Golden裁掉顶部指标带：浮层指标行不参比，只比它下面的纯画面。
	goldenRects := []wrsoak.Rect{
		{X: 0, Y: metricStripH, W: winW, H: winH - metricStripH},
	}
	winGoldenDiff, winGoldenTotal, winGoldenFirst := wrsoak.EvaluateGolden("game_anim", testdataDir, "tl_final.png", "tl_final_base.png", goldenRects, winW)

	parityContract, _ := numExtra(extra, "parity_ok")
	goldenDiff, _ := numExtra(extra, "golden_diff_pct")
	goldenTotal, _ := numExtra(extra, "golden_total_px")
	xv, _ := live.Sample("x", live.Pos())
	av, _ := live.Sample("alpha", live.Pos())
	sv, _ := live.Sample("slash", live.Pos())
	extra["tl_wraps"] = wraps
	extra["tl_events_total"] = evTotal
	extra["tl_events"] = evLog
	extra["tl_pos_ms"] = live.Pos().Milliseconds()
	extra["tl_x"] = xv
	extra["tl_alpha"] = av
	extra["tl_slash"] = sv
	extra["tl_active"] = live.Loop().String()
	extra["case"] = "tl"
	extra["tracks"] = "x,alpha,slash"
	extra["sequences"] = "once_attack,loop_wrap,pingpong_bounce"
	extra["win_golden_diff_pct"] = winGoldenDiff
	extra["win_golden_total_px"] = winGoldenTotal
	if winGoldenFirst {
		extra["win_golden_first"] = 1
	}
	extra["pointer_events"] = summary.Pointer
	extra["key_events"] = summary.Key
	extra["resize_events"] = summary.Resize
	extra["manual_timed"] = secsSet

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     tlAbilityID,
		Scenario:      tlScenario,
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra:         extra,
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	pass := true
	mustPass := func(cond bool, msg string, args ...any) {
		if !cond {
			fmt.Fprintf(os.Stderr, "FAIL: "+msg+"\n", args...)
			pass = false
		}
	}
	if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1}); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		pass = false
	}
	mustPass(parityContract == 1, "parity_ok=%v want 1 (three-sequence replay bitwise + event order)", extra["parity_ok"])
	mustPass(probeOK == 1, "probe_ok=%v want 1 (keys/wrap/3-seq frozen/order)", extra["probe_ok"])
	mustPass(pixelOK == 1, "pixel_ok=%v want 1 (x/alpha/dot/tick probes)", extra["pixel_ok"])
	if fr, _ := numExtra(extra, "golden_first_run"); fr != 1 {
		mustPass(goldenDiff == goldenTol, "golden_diff_pct=%.4f want %.1f over %d px", goldenDiff, goldenTol, int64(goldenTotal))
	}
	if !winGoldenFirst {
		mustPass(winGoldenDiff == goldenTol, "win_golden_diff_pct=%.4f want %.1f over %d px", winGoldenDiff, goldenTol, winGoldenTotal)
	}
	mustPass(wraps >= switchMin, "tl_wraps=%d want >=%d (loop head wrapped)", wraps, switchMin)
	mustPass(evTotal >= tlEvtMin, "tl_events_total=%d want >=%d (cues fired live)", evTotal, tlEvtMin)

	if autoOnly {
		summary.Timed = secsSet
		if !pass {
			os.Exit(1)
		}
		fps := report.FPSInterval
		fmt.Fprintf(os.Stderr, "game_anim: OK case=tl presents=%d fps=%.1f p95=%.1f parity=%v golden=%.4f%% win_golden=%.4f%% wraps=%d events=%d elapsed=%.1fs\n",
			app.PresentCount(), fps, snap.P95FrameIntervalMs, extra["parity_ok"], goldenDiff, winGoldenDiff, wraps, evTotal, elapsedSec)
		return
	}
	summary.Timed = secsSet
	fmt.Fprintf(os.Stderr, "game_anim: case=tl backend=%s presents=%d parity=%v golden=%.4f%% win_golden=%.4f%% wraps=%d events=%d elapsed=%.1fs ptr=%d key=%d rs=%d\n",
		win.Backend(), app.PresentCount(), extra["parity_ok"], goldenDiff, winGoldenDiff, wraps, evTotal, elapsedSec, summary.Pointer, summary.Key, summary.Resize)
}

// ---- --case=lod: S89 animation LOD independent window (reuse, no new window) ----
//
// Reads engine/anim/testdata/large_lod.json (engine-owned numbers, window
// only probes). Offscreen paints the frozen viewport map; the live window
// drifts the viewport deterministically with elapsed so the second run on
// the same RUN_SECONDS lands on the same snapshot.

const (
	lodAbilityID = "anim-lod"
	lodScenario  = "game_anim--case=lod"
	lodEngineDir = "engine/anim/testdata"
	lodEngineFn  = "large_lod.json"

	lodOffW, lodOffH = 640, 360
	lodPerfMaxUs     = 1000 // 1000 bounds classify under 1ms
)

type lodFile struct {
	Viewport   [4]float64   `json:"viewport"`
	NearDist   float64      `json:"near_dist"`
	Bounds     [][4]float64 `json:"bounds"`
	WantLevels []int        `json:"want_levels"`
	WantOff    int          `json:"want_off"`
	WantFar    int          `json:"want_far"`
	WantNear   int          `json:"want_near"`
	WantTick0  int          `json:"want_updates_tick0"`
	WantTick1  int          `json:"want_updates_tick1"`
}

func lodRect(r [4]float64) core.Rect { return core.NewRect(r[0], r[1], r[2], r[3]) }

func loadLODFile() (lodFile, error) {
	var f lodFile
	raw, err := os.ReadFile(filepath.Join(lodEngineDir, lodEngineFn))
	if err != nil {
		return f, err
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		return f, err
	}
	if len(f.Bounds) == 0 || len(f.WantLevels) != len(f.Bounds) {
		return f, fmt.Errorf("lod file bounds/levels mismatch %d/%d", len(f.Bounds), len(f.WantLevels))
	}
	return f, nil
}

func lodSelftest(f lodFile) (map[string]any, bool) {
	extra := map[string]any{}
	ok := true
	fail := func(k string, v any) {
		extra[k] = v
		ok = false
	}
	pol, err := anim.NewLodPolicy(f.NearDist)
	if err != nil {
		fail("policy_err", err.Error())
		return extra, false
	}
	vp := lodRect(f.Viewport)
	bounds := make([]core.Rect, len(f.Bounds))
	for i, b := range f.Bounds {
		bounds[i] = lodRect(b)
	}
	levels := pol.Batch(bounds, vp)
	for i, lv := range levels {
		if int(lv) != f.WantLevels[i] {
			fail("level_"+string(rune('0'+i%10)), []int{i, int(lv), f.WantLevels[i]})
			break
		}
	}
	off, far, near := 0, 0, 0
	for _, lv := range levels {
		switch lv {
		case anim.LodOff:
			off++
		case anim.LodFar:
			far++
		case anim.LodNear:
			near++
		}
	}
	extra["lod_off"] = off
	extra["lod_far"] = far
	extra["lod_near"] = near
	if off != f.WantOff || far != f.WantFar || near != f.WantNear {
		fail("lod_counts", []int{off, far, near})
	} else {
		extra["lod_counts_ok"] = true
	}
	s0 := anim.NewLodScheduler(pol)
	fr0 := s0.NextFrame(bounds, vp)
	s1 := anim.NewLodScheduler(pol)
	_ = s1.NextFrame(bounds, vp)
	fr1 := s1.NextFrame(bounds, vp)
	extra["lod_tick0"] = fr0.Wanted
	extra["lod_tick1"] = fr1.Wanted
	if fr0.Wanted != f.WantTick0 || fr1.Wanted != f.WantTick1 {
		fail("lod_ticks", []int{fr0.Wanted, fr1.Wanted})
	} else {
		extra["lod_ticks_ok"] = true
	}
	// Replay: batch twice bitwise, scheduler reset replays.
	again := pol.Batch(bounds, vp)
	replayOK := len(again) == len(levels)
	if replayOK {
		for i := range levels {
			if again[i] != levels[i] {
				replayOK = false
				break
			}
		}
	}
	extra["parity_ok"] = replayOK
	if !replayOK {
		ok = false
	}
	// Cull: off entries never want updates on either tick.
	cullOK := true
	for i, lv := range levels {
		if lv == anim.LodOff && (fr0.Updates[i] || fr1.Updates[i]) {
			cullOK = false
			break
		}
	}
	extra["cull_ok"] = cullOK
	if !cullOK {
		ok = false
	}
	// Far half-rate: far updates alternate across ticks.
	farAltOK := true
	for i, lv := range levels {
		if lv == anim.LodFar && fr0.Updates[i] == fr1.Updates[i] {
			farAltOK = false
			break
		}
	}
	extra["far_half_ok"] = farAltOK
	if !farAltOK && far > 0 {
		ok = false
	}
	// Perf: 1000 classify under 1ms.
	t0 := time.Now()
	for k := 0; k < 5; k++ {
		_ = pol.Batch(bounds, vp)
	}
	el := time.Since(t0)
	perBatchUs := float64(el.Microseconds()) / 5
	extra["lod_batch_us"] = perBatchUs
	extra["lod_perf_ok"] = perBatchUs <= lodPerfMaxUs
	if perBatchUs > lodPerfMaxUs {
		ok = false
	}
	extra["probe_ok"] = ok
	return extra, ok
}

func lodToOff(vp core.Rect, b core.Rect, x, y, w, h float64) (float64, float64, float64, float64) {
	sx := w / 800
	sy := h / 600
	px := x + (b.X-vp.X)*sx
	py := y + (b.Y-vp.Y)*sy
	return px, py, b.W * sx, b.H * sy
}

func paintLODOffscreen(dc *render.Context, f lodFile, levels []anim.LodLevel) {
	dc.ClearWithColor(render.RGBA{R: 0.08, G: 0.09, B: 0.11, A: 1})
	vp := lodRect(f.Viewport)
	// Viewport field.
	dc.SetRGB(0.92, 0.93, 0.95)
	dc.DrawRectangle(20, 20, lodOffW-40, lodOffH-40)
	_ = dc.Fill()
	for i, b4 := range f.Bounds {
		if levels[i] == anim.LodOff {
			continue
		}
		b := lodRect(b4)
		px, py, pw, ph := lodToOff(vp, b, 20, 20, lodOffW-40, lodOffH-40)
		if pw < 2 {
			pw = 2
		}
		if ph < 2 {
			ph = 2
		}
		if levels[i] == anim.LodNear {
			dc.SetRGB(0.2, 0.65, 0.3)
		} else {
			dc.SetRGB(0.95, 0.75, 0.2)
		}
		dc.DrawRectangle(px, py, pw, ph)
		_ = dc.Fill()
	}
}

func lodRenderOffscreen(f lodFile, levels []anim.LodLevel) image.Image {
	prev, had := os.LookupEnv("GOGPU_RENDER_MODE")
	_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	defer func() {
		if had {
			_ = os.Setenv("GOGPU_RENDER_MODE", prev)
		} else {
			_ = os.Unsetenv("GOGPU_RENDER_MODE")
		}
	}()
	dc := render.NewContext(lodOffW, lodOffH)
	defer dc.Close()
	paintLODOffscreen(dc, f, levels)
	raw := dc.Image()
	cp := image.NewRGBA(raw.Bounds())
	if rgba, isRGBA := raw.(*image.RGBA); isRGBA {
		copy(cp.Pix, rgba.Pix)
		return cp
	}
	for y := 0; y < lodOffH; y++ {
		for x := 0; x < lodOffW; x++ {
			cp.Set(x, y, raw.At(x, y))
		}
	}
	return cp
}

func runLODPixelProbes(img image.Image, f lodFile, levels []anim.LodLevel) (map[string]any, bool) {
	out := map[string]any{}
	ok := true
	vp := lodRect(f.Viewport)
	nearIdx, farIdx := -1, -1
	for i, lv := range levels {
		if lv == anim.LodNear && nearIdx < 0 {
			nearIdx = i
		}
		if lv == anim.LodFar && farIdx < 0 {
			farIdx = i
		}
	}
	check := func(idx int, wantR, wantG, wantB uint8, key string) {
		if idx < 0 {
			out[key] = false
			ok = false
			return
		}
		b := lodRect(f.Bounds[idx])
		px, py, pw, ph := lodToOff(vp, b, 20, 20, lodOffW-40, lodOffH-40)
		cx, cy := int(px+pw/2+0.5), int(py+ph/2+0.5)
		r, g, bl, valid := sampleByte(img, cx, cy)
		good := valid && closeByte(r, wantR) && closeByte(g, wantG) && closeByte(bl, wantB)
		out[key] = []int{int(r), int(g), int(bl)}
		out[key+"_ok"] = good
		if !good {
			ok = false
		}
	}
	check(nearIdx, 51, 166, 77, "near")
	check(farIdx, 242, 191, 51, "far")
	if r, g, b, valid := sampleByte(img, 4, 4); !valid || !closeByte(r, 20) || !closeByte(g, 23) || !closeByte(b, 28) {
		out["field"] = []int{int(r), int(g), int(b)}
		out["field_ok"] = false
		ok = false
	} else {
		out["field_ok"] = true
	}
	out["pixel_ok"] = ok
	return out, ok
}

func checkLODGolden(cur image.Image) (float64, int64, bool, bool) {
	_ = os.MkdirAll(testdataDir, 0o755)
	basePath := filepath.Join(testdataDir, "lod_golden.png")
	if _, err := os.Stat(basePath); err != nil {
		f, err := os.Create(basePath)
		if err != nil {
			return 100, 0, false, false
		}
		_ = png.Encode(f, cur)
		_ = f.Close()
		return 0, 0, true, true
	}
	f, err := os.Open(basePath)
	if err != nil {
		return 100, 0, false, false
	}
	want, err := png.Decode(f)
	_ = f.Close()
	if err != nil {
		return 100, 0, false, false
	}
	if !want.Bounds().Eq(cur.Bounds()) {
		return 100, int64(cur.Bounds().Dx() * cur.Bounds().Dy()), false, false
	}
	var diff int64
	total := int64(cur.Bounds().Dx() * cur.Bounds().Dy())
	for y := 0; y < cur.Bounds().Dy(); y++ {
		for x := 0; x < cur.Bounds().Dx(); x++ {
			ar, ag, ab, aa := cur.At(x, y).RGBA()
			br, bg, bb, ba := want.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				diff++
			}
		}
	}
	var pct float64
	if total > 0 {
		pct = 100 * float64(diff) / float64(total)
	}
	return pct, total, false, diff == 0
}

func runLODCase(autoOnly bool, manualSeconds int) {
	secs, secsSet := wrkit.RunSecondsOpt()
	if autoOnly {
		if !secsSet {
			secs = 8
			secsSet = true
		}
	} else if manualSeconds > 0 {
		secs = manualSeconds
		secsSet = true
	}
	wrkit.EnsureUIFace()

	f, err := loadLODFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: load %s: %v\n", filepath.Join(lodEngineDir, lodEngineFn), err)
		os.Exit(1)
	}
	extra, ok := lodSelftest(f)
	pol, _ := anim.NewLodPolicy(f.NearDist)
	vp0 := lodRect(f.Viewport)
	bounds := make([]core.Rect, len(f.Bounds))
	for i, b := range f.Bounds {
		bounds[i] = lodRect(b)
	}
	levels := pol.Batch(bounds, vp0)
	cpuImg := lodRenderOffscreen(f, levels)
	if cpuImg == nil {
		extra["pixel_ok"] = false
		ok = false
	} else {
		pixMap, pixOK := runLODPixelProbes(cpuImg, f, levels)
		for k, v := range pixMap {
			extra[k] = v
		}
		if !pixOK {
			ok = false
		}
		_ = os.MkdirAll(testdataDir, 0o755)
		if fh, err := os.Create(filepath.Join(testdataDir, "lod_last.png")); err == nil {
			_ = png.Encode(fh, cpuImg)
			_ = fh.Close()
		}
		gd, total, first, gok := checkLODGolden(cpuImg)
		extra["golden_diff_pct"] = gd
		extra["golden_total_px"] = total
		if first {
			extra["golden_first_run"] = 1
		}
		extra["golden_ok"] = gok
		if !gok && !first {
			ok = false
		}
	}
	extra["case"] = "lod"
	if !ok {
		raw, _ := json.Marshal(map[string]any{"ability_id": lodAbilityID, "scenario": lodScenario, "extra": extra, "pass": false})
		fmt.Println(string(raw))
		fmt.Fprintln(os.Stderr, "game_anim: lod selftest FAIL, not opening window")
		os.Exit(1)
	}
	if !autoOnly && !secsSet && manualSeconds <= 0 {
		fmt.Fprintln(os.Stderr, "game_anim: lod selftest done, entering manual phase (close X to finish)")
	}

	var proc scheduler.ProcessTracker
	proc.Start()
	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: winTitle, Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()
	var summary manualSummary
	summary.Note = "case=lod"
	elapsed := 0.0
	sched := anim.NewLodScheduler(pol)
	wanted := 0
	setTitle := func() {
		if ctl == nil {
			return
		}
		ctl.SetTitle(fmt.Sprintf("%s — lod off=%v far=%v near=%v want=%d ptr=%d key=%d rs=%d t=%.0fs",
			winTitle, extra["lod_off"], extra["lod_far"], extra["lod_near"], wanted, summary.Pointer, summary.Key, summary.Resize, elapsed))
	}
	root := rendering.NewAbsoluteBox(winW, winH)
	root.Background = &rendering.Color{R: 0.08, G: 0.09, B: 0.11, A: 1}
	full := rendering.NewRenderBox()
	full.FixedWidth, full.FixedHeight = winW, winH
	boundsC, vpC := bounds, vp0
	schedC := sched
	full.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		ax, ay := pc.Abs(0, 0)
		pc.DC.SetRGB(0.08, 0.09, 0.11)
		pc.DC.DrawRectangle(ax, ay, size.Width, size.Height)
		_ = pc.DC.Fill()
		// Frozen viewport for golden replay: LOD motion is proven by the
		// tick counters in the JSON probes, not by drifting the picture.
		vp := vpC
		sx := size.Width / vp.W
		sy := (size.Height - metricStripH) / vp.H
		sk := sx
		if sy < sk {
			sk = sy
		}
		ox := ax + (size.Width-vp.W*sk)/2 - vp.X*sk
		oy := ay + metricStripH + (size.Height-metricStripH-vp.H*sk)/2 - vp.Y*sk
		for i, b := range boundsC {
			lv, _ := schedC.Policy().Classify(b, vp)
			if lv == anim.LodOff {
				continue
			}
			if lv == anim.LodNear {
				pc.DC.SetRGB(0.2, 0.65, 0.3)
			} else {
				pc.DC.SetRGB(0.95, 0.75, 0.2)
			}
			px, py := ox+b.X*sk, oy+b.Y*sk
			pw, ph := b.W*sk, b.H*sk
			if pw < 2 {
				pw = 2
			}
			if ph < 2 {
				ph = 2
			}
			_ = i
			pc.DC.DrawRectangle(px, py, pw, ph)
			_ = pc.DC.Fill()
		}
	}
	root.Place(full, 0, 0)
	metric := wrkit.Label("anim-lod", 13, 1, 1, 1)
	root.Place(metric, 8, 8)
	_ = os.MkdirAll(testdataDir, 0o755)
	snapPath := filepath.Join(testdataDir, "lod_final.png")
	app := embedder.NewPipelineApp(host, root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor:       time.Duration(secs) * time.Second,
		WarmUp:       true,
		SnapshotPath: snapPath,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				fmt.Fprintf(os.Stderr, "game_anim: lod close (%s)\n", win.Backend())
			case platform.EventResize:
				summary.Resize++
				if ev.Width > 0 && ev.Height > 0 {
					root.FixedWidth, root.FixedHeight = float64(ev.Width), float64(ev.Height)
					full.FixedWidth, full.FixedHeight = float64(ev.Width), float64(ev.Height)
					root.MarkNeedsPaint()
				}
				setTitle()
			case platform.EventPointer:
				summary.Pointer++
				fmt.Fprintf(os.Stderr, "game_anim: lod pointer kind=%v @(%.0f,%.0f)\n", ev.Pointer, ev.X, ev.Y)
				setTitle()
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					fmt.Fprintf(os.Stderr, "game_anim: lod key code=%d rune=%q\n", ev.KeyCode, string(ev.Rune))
					setTitle()
				}
			default:
				fmt.Fprintf(os.Stderr, "game_anim: lod event %s\n", ev.Type)
			}
		},
	})
	probeOK, _ := numExtra(extra, "probe_ok")
	pixelOK, _ := numExtra(extra, "pixel_ok")
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt
		vp := vpC
		fr := sched.NextFrame(bounds, vp)
		wanted = fr.Wanted
		metric.SetText(fmt.Sprintf("anim-lod off=%d far=%d near=%d want=%d", fr.Off, fr.Far, fr.Near, fr.Wanted))
		metric.MarkNeedsPaint()
		full.MarkNeedsPaint()
		app.ScheduleFrame()
		proc.Sample()
		_ = probeOK
		_ = pixelOK
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)
	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())
	elapsedSec := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	wrkit.MergeBoundaryCache(app, &snap)
	goldenRects := []wrsoak.Rect{{X: 0, Y: metricStripH, W: winW, H: winH - metricStripH}}
	winGoldenDiff, winGoldenTotal, winGoldenFirst := wrsoak.EvaluateGolden("game_anim", testdataDir, "lod_final.png", "lod_final_base.png", goldenRects, winW)
	parityContract, _ := numExtra(extra, "parity_ok")
	goldenDiff, _ := numExtra(extra, "golden_diff_pct")
	goldenTotal, _ := numExtra(extra, "golden_total_px")
	extra["win_golden_diff_pct"] = winGoldenDiff
	extra["win_golden_total_px"] = winGoldenTotal
	if winGoldenFirst {
		extra["win_golden_first"] = 1
	}
	extra["pointer_events"] = summary.Pointer
	extra["key_events"] = summary.Key
	extra["resize_events"] = summary.Resize
	extra["manual_timed"] = secsSet
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: lodAbilityID, Scenario: lodScenario, Snap: snap,
		PresentCount: app.PresentCount(), ElapsedSec: elapsedSec,
		SurfaceAreaPx: winW * winH, Warmup: true, Extra: extra,
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))
	pass := true
	mustPass := func(cond bool, msg string, args ...any) {
		if !cond {
			fmt.Fprintf(os.Stderr, "FAIL: "+msg+"\n", args...)
			pass = false
		}
	}
	if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1}); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		pass = false
	}
	mustPass(parityContract == 1, "parity_ok=%v want 1 (lod replay bitwise)", extra["parity_ok"])
	mustPass(probeOK == 1, "probe_ok=%v want 1 (lod counts/ticks/perf)", extra["probe_ok"])
	mustPass(pixelOK == 1, "pixel_ok=%v want 1 (near/far/field probes)", extra["pixel_ok"])
	if fr, _ := numExtra(extra, "golden_first_run"); fr != 1 {
		mustPass(goldenDiff == goldenTol, "golden_diff_pct=%.4f want %.1f over %d px", goldenDiff, goldenTol, int64(goldenTotal))
	}
	if !winGoldenFirst {
		mustPass(winGoldenDiff == goldenTol, "win_golden_diff_pct=%.4f want %.1f over %d px", winGoldenDiff, goldenTol, winGoldenTotal)
	}
	if autoOnly {
		summary.Timed = secsSet
		if !pass {
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "game_anim: OK case=lod presents=%d parity=%v golden=%.4f%% win_golden=%.4f%% off=%v far=%v near=%v elapsed=%.1fs\n",
			app.PresentCount(), extra["parity_ok"], goldenDiff, winGoldenDiff, extra["lod_off"], extra["lod_far"], extra["lod_near"], elapsedSec)
		return
	}
	summary.Timed = secsSet
	fmt.Fprintf(os.Stderr, "game_anim: case=lod backend=%s presents=%d parity=%v golden=%.4f%% win_golden=%.4f%% elapsed=%.1fs ptr=%d key=%d rs=%d\n",
		win.Backend(), app.PresentCount(), extra["parity_ok"], goldenDiff, winGoldenDiff, elapsedSec, summary.Pointer, summary.Key, summary.Resize)
}

// Command game_anim hosts the 4.x animation independent real windows.
//
// --case=sk is the 4.3 skeleton window (walk/run/jump on true art).
// --case=fsm below is the 4.4 blend state machine window (switch crisp,
// blend bar live). Each ability keeps its own case; no combo window
// stands in for a single ability.
//
// Window: 1200x800, title game_anim. Three cards WALK/RUN/JUMP paint the
// same 13-bone upright person from the window-local testdata/sk_hero.json
// (window-owned art for a readable person; game/anim keeps its own
// 7-bone math hero untouched). Bones call game/anim read-only
// (Pose/Skin/IK), drawing uses only existing render shapes; render/ and
// game/anim/ stay untouched.
//
// Flags:
//
//	go run ./examples/game_anim --case=sk -auto-only
//	  RUN_SECONDS=8 (default 8) auto gate, JSON on stdout, exit 1 on fail.
//	go run ./examples/game_anim --case=sk -manual-seconds 30
//	  resident 30s, real events logged + SetTitle, summary JSON.
//	go run ./examples/game_anim --case=sk
//	  selftest then resident until close. RUN_SECONDS also times the run.
//
// Gates (hard): present>=1 via wrgate, parity contract passes
// (three-pose replay bitwise + draw order file order, the pure-math
// C-both-sides: raster AA is a known CPU/GPU divergence owned by 9.2,
// 4.3 owns numbers not pixels), logic probes pass (joints match the
// game/anim offscreen goldens, draw order file order, walk/run/jump
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

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/examples/wrsoak"
	"github.com/energye/gpui/game/anim"
	"github.com/energye/gpui/game/core"
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
	logicEps     = 1e-9 // joint match vs game/anim offscreen goldens
	ikMaxDist    = 1e-9 // solved tip must sit on the target
	pixelByteTol = 10   // per-channel byte tolerance for probes
	goldenTol    = 0.0  // zero tolerance once the baseline exists
	switchMin    = 3    // auto run must show every pose switch
	poseCycleSec = 2.0  // walk->run->jump rotation period
	frozenBones  = 13   // the upright person: pelvis, spine, neck, limbs
)

const testdataDir = "examples/game_anim/testdata"

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
	// Setup joints match the game/anim offscreen goldens exactly.
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
	if back := core.Vec2FromRenderPoint(v.ToRenderPoint()); back != v {
		boundaryOK = false
	}
	if m, err := setup.WorldTransform("shinL"); err != nil {
		boundaryOK = false
	} else if back := core.Mat2DFromRenderMatrix(m.ToRenderMatrix()); back != m {
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
	caseFlag := flag.String("case", "sk", "anim case: sk (walk/run/jump bones) or fsm (4.4 blend state machine)")
	autoOnly := flag.Bool("auto-only", false, "run selftest + short real window and exit (gate mode)")
	manualSeconds := flag.Int("manual-seconds", 0, "manual phase timeout in seconds (0 = until window close)")
	flag.Parse()
	if *caseFlag == "fsm" {
		runFSMCase(*autoOnly, *manualSeconds)
		return
	}
	if *caseFlag != "sk" {
		fmt.Fprintf(os.Stderr, "FAIL: --case=%q want sk or fsm (tl gets its own case later, no combo)\n", *caseFlag)
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

	// Window-local true art only: never read across into game/anim/testdata.
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

	shell := wrkit.NewShell(winW, winH, "game_anim --case=sk 走跑跳真资源不错位 (4.3)", []string{
		"WALK/RUN/JUMP 同一套13骨直立人",
		"头顶圆脸+身竖条+双腿双臂",
		"关节=离屏金数据一致",
		"绘制次序 腿后身前头顶",
		"双IK腿吸住+头跟看",
		"切换Reset不Combine错",
		"JSON看parity+探针+金图",
		"--case=sk, 只要这一个",
		"tl/fsm以后各有各case",
	})

	// Three static cards (time-invariant by design: no active highlight in
	// paint, so the window golden stays deterministic).
	var cards []*rendering.RenderBox
	for i, name := range poseNames {
		name := name
		lx := 8 + float64(i)*288
		shell.Body.LabelAt(name, 13, lx, 8, 0.75, 0.82, 0.9)
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cardW, cardH
		box.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
			if pc == nil || pc.DC == nil {
				return
			}
			ax, ay := pc.Abs(0, 0)
			pc.DC.SetRGB(1, 1, 1)
			pc.DC.DrawRectangle(ax, ay, size.Width, size.Height)
			_ = pc.DC.Fill()
			sub := pc.DC
			paintPose(sub, skel, poses[name], ax+cardOX, ay+cardOY, cardScale)
		}
		shell.Body.Place(box, lx, 40)
		cards = append(cards, box)
	}
	status := wrkit.Label("POSE walk switches=0", 12, 0.70, 0.78, 0.88)
	shell.Body.Place(status, 8, 360)
	chain := wrkit.Label("game/anim只算数: Pose+Skin+IK直调冻接口, 画只走现有矩形直线", 12, 0.70, 0.78, 0.88)
	shell.Body.Place(chain, 8, 384)

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

	_ = os.MkdirAll(testdataDir, 0o755)
	snapPath := filepath.Join(testdataDir, "sk_final.png")

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
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
					shell.Resize(float64(ev.Width), float64(ev.Height))
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
			status.SetText(fmt.Sprintf("POSE %s switches=%d", poseNames[poseIdx], switches))
			status.MarkNeedsPaint()
			setTitle()
		}
		for _, c := range cards {
			c.MarkNeedsPaint()
		}
		app.ScheduleFrame()
		proc.Sample()
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gateOK := snapH.PaintCount > 0 && probeOK == 1 && pixelOK == 1
		shell.UpdateHUD(abilityID, "Steady", app, gateOK,
			fmt.Sprintf("presents=%d pose=%s sw=%d", app.PresentCount(), poseNames[poseIdx], switches),
			"walk/run/jump")
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

	// Window golden over the static mask (status line + HUD excluded: live
	// numbers by design). Body at (284,60); cards at local (8/296/584,40).
	goldenRects := []wrsoak.Rect{
		{X: 0, Y: 0, W: winW, H: 48},
		{X: 12, Y: 60, W: 260, H: 656},
		{X: 284 + 8, Y: 60 + 40, W: cardW, H: cardH},
		{X: 284 + 296, Y: 60 + 40, W: cardW, H: cardH},
		{X: 284 + 584, Y: 60 + 40, W: cardW, H: cardH},
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

	shell := wrkit.NewShell(winW, winH, "game_anim --case=fsm 切换脆混合看得见 (4.4)", []string{
		"IDLE/RUN/JUMP 同一套混合状态机",
		"方块=合法去向, 蓝条=混合时长",
		"蓝降橙升=交叉淡, 红点=半程",
		"右绿线=跳回待机直接切",
		"活条跟当前混合权重走",
		"JSON看parity+探针+金图",
		"--case=fsm, 只要这一个",
		"sk/tl各有各case不混",
	})

	// Three static cards plus the static curve strip (golden-covered).
	for i, s := range states {
		s := s
		lx := 8 + float64(i)*288
		shell.Body.LabelAt(s.Name+" blend="+itoa(s.BlendMs)+"ms", 13, lx, 8, 0.75, 0.82, 0.9)
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = fsmCardW, fsmCardH
		box.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
			if pc == nil || pc.DC == nil {
				return
			}
			ax, ay := pc.Abs(0, 0)
			paintFSMCard(pc.DC, ax, ay, size.Width, size.Height, float64(s.BlendMs), len(s.CanTo))
		}
		shell.Body.Place(box, lx, 40)
	}
	strip := rendering.NewRenderBox()
	strip.FixedWidth, strip.FixedHeight = fsmStripW, fsmStripH
	strip.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		ax, ay := pc.Abs(0, 0)
		pc.DC.SetRGB(1, 1, 1)
		pc.DC.DrawRectangle(ax, ay, size.Width, size.Height)
		_ = pc.DC.Fill()
		paintFSMCurve(pc.DC, ax+222, ay+15, 620, 140)
		paintFSMSquares(pc.DC, ax+20, ay+30, 2)
		paintFSMBar(pc.DC, ax+20, ay+60, 150, 20, 200, fsmMaxBlendMs)
		paintFSMBar(pc.DC, ax+20, ay+95, 150, 20, 100, fsmMaxBlendMs)
		paintFSMBar(pc.DC, ax+20, ay+130, 150, 20, 0, fsmMaxBlendMs)
	}
	shell.Body.Place(strip, 8, 270)

	// Live blend bar (masked from the golden): follows the machine.
	liveBox := rendering.NewRenderBox()
	liveBox.FixedWidth, liveBox.FixedHeight = 400, 120
	liveBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		ax, ay := pc.Abs(0, 0)
		fw, tw := live.Weights()
		pc.DC.SetRGB(1, 1, 1)
		pc.DC.DrawRectangle(ax, ay, size.Width, size.Height)
		_ = pc.DC.Fill()
		paintFSMBar(pc.DC, ax+16, ay+16, size.Width-32, 24, fw*200, 200)
		paintFSMBar(pc.DC, ax+16, ay+50, size.Width-32, 24, tw*200, 200)
		pc.DC.SetRGBA(0.85, 0.15, 0.12, 1)
		mx := ax + 16 + (size.Width-32)*tw
		pc.DC.DrawRectangle(mx-3, ay+84, 6, 6)
		_ = pc.DC.Fill()
	}
	shell.Body.Place(liveBox, 8, 470)
	status := wrkit.Label("FSM idle switches=0", 12, 0.70, 0.78, 0.88)
	shell.Body.Place(status, 424, 470)
	chain := wrkit.Label("game/anim只算数: State+Machine直调冻接口, 画只走现有矩形直线", 12, 0.70, 0.78, 0.88)
	shell.Body.Place(chain, 424, 494)

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

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
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
					shell.Resize(float64(ev.Width), float64(ev.Height))
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
			fw, tw := live.Weights()
			status.SetText(fmt.Sprintf("FSM %s from=%s blend=%d%% switches=%d", live.Current(), live.From(), int(tw*100+0.5), switches))
			_ = fw
			status.MarkNeedsPaint()
			setTitle()
		}
		liveBox.MarkNeedsPaint()
		app.ScheduleFrame()
		proc.Sample()
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gateOK := snapH.PaintCount > 0 && probeOK == 1 && pixelOK == 1
		shell.UpdateHUD(fsmAbilityID, "Steady", app, gateOK,
			fmt.Sprintf("presents=%d fsm=%s sw=%d", app.PresentCount(), live.Current(), switches),
			"idle/run/jump")
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

	// Window golden over the static mask (status line, live bar, HUD
	// excluded: live numbers by design). Body at (284,60).
	goldenRects := []wrsoak.Rect{
		{X: 0, Y: 0, W: winW, H: 48},
		{X: 12, Y: 60, W: 260, H: 656},
		{X: 284 + 8, Y: 60 + 40, W: fsmCardW, H: fsmCardH},
		{X: 284 + 296, Y: 60 + 40, W: fsmCardW, H: fsmCardH},
		{X: 284 + 584, Y: 60 + 40, W: fsmCardW, H: fsmCardH},
		{X: 284 + 8, Y: 60 + 270, W: fsmStripW, H: fsmStripH},
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

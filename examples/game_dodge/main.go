// Command game_dodge is the S78 dodge-the-creeps gate: a 400px/s player
// dodges edge-spawned mobs (150-250px/s, perpendicular +-45deg) with a
// per-second score, all through the real engine/physics bodies.
//
// Modes:
//
//	RUN_SECONDS=8 go run ./examples/game_dodge -auto-only
//	  probes + ~8s window (JSON on stdout, exit 1 on fail).
//	go run ./examples/game_dodge -manual-seconds 30
//	  manual 30s (WASD/arrows move, hit dies once, events logged), then summary.
//	go run ./examples/game_dodge
//	  probes, then resident until close (RUN_SECONDS sets a timed run).
//
// Window: 1200x800, title game_dodge. First run writes the golden baseline
// into testdata/; later runs compare it with zero tolerance.
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
	"strconv"
	"time"

	"github.com/energye/gpui/engine/audio"
	"github.com/energye/gpui/engine/core"
	"github.com/energye/gpui/engine/physics"
	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH = 1200, 800
	abilityID  = "dodge-creeps"
	scenario   = "game_dodge"

	goldenPath = "examples/game_dodge/testdata/dodge_golden.png"
	lastPath   = "examples/game_dodge/testdata/dodge_last.png"

	probePixelTol = 8
)

// Demo numbers (Godot dodge_the_creeps: player.gd/main.gd/mob.gd).
const (
	playerSpeed = 400.0
	playerR     = 16.0
	mobR        = 14.0
	mobMin      = 150.0
	mobMax      = 250.0
	mobEvery    = 0.5
	scoreEvery  = 1.0
	mobMargin   = 48.0

	arenaW = 560.0
	arenaH = 400.0

	offW, offH = 480, 270
)

const (
	bgR, bgG, bgB             = 0.08, 0.09, 0.11
	heroR, heroG, heroB       = 0.90, 0.20, 0.15
	creepR, creepG, creepB    = 0.20, 0.65, 0.30
	creep2R, creep2G, creep2B = 0.75, 0.55, 0.20
	scoreR, scoreG, scoreB    = 0.95, 0.85, 0.25
)

const (
	arenaX = 16.0
	arenaY = 44.0
	countX = 600.0
	countY = 44.0
	noteY  = 470.0
)

type mob struct {
	pos   core.Vec2
	vel   core.Vec2
	speed float64
	kind  int
	edge  int
}

type probeResult struct {
	LogicOK       bool
	LogicDetail   string
	PixOK         bool
	PixDetail     string
	GoldenOK      bool
	GoldenChanged int
	GoldenWrote   bool
	OK            bool
}

func runSeconds(def int) int {
	if v := os.Getenv("RUN_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func failJSON(probe probeResult) {
	b, _ := json.Marshal(map[string]any{
		"ability_id": abilityID,
		"scenario":   scenario,
		"probe_ok":   0,
		"pass":       false,
		"pixels":     probe.PixDetail,
		"golden":     probe.GoldenChanged,
	})
	fmt.Fprintln(os.Stdout, string(b))
}

func clampPlayer(p core.Vec2) core.Vec2 {
	if p.X < playerR {
		p.X = playerR
	}
	if p.X > arenaW-playerR {
		p.X = arenaW - playerR
	}
	if p.Y < playerR {
		p.Y = playerR
	}
	if p.Y > arenaH-playerR {
		p.Y = arenaH - playerR
	}
	return p
}

func playerStep(pos core.Vec2, ix, iy, dt float64) core.Vec2 {
	if dt < 0 {
		dt = 0
	}
	if ix == 0 && iy == 0 {
		return clampPlayer(pos)
	}
	l := math.Hypot(ix, iy)
	nx, ny := ix/l, iy/l
	pos.X += nx * playerSpeed * dt
	pos.Y += ny * playerSpeed * dt
	return clampPlayer(pos)
}

func spawnMob(rng *core.Rand) mob {
	edge := int(rng.Int63n(4))
	var px, py, bx, by float64
	switch edge {
	case 0:
		px, py, bx, by = rng.RangeFloat(0, arenaW), 0, 0, 1
	case 1:
		px, py, bx, by = rng.RangeFloat(0, arenaW), arenaH, 0, -1
	case 2:
		px, py, bx, by = 0, rng.RangeFloat(0, arenaH), 1, 0
	default:
		px, py, bx, by = arenaW, rng.RangeFloat(0, arenaH), -1, 0
	}
	ang := rng.RangeFloat(-math.Pi/4, math.Pi/4)
	ca, sa := math.Cos(ang), math.Sin(ang)
	dx := bx*ca - by*sa
	dy := bx*sa + by*ca
	sp := rng.RangeFloat(mobMin, mobMax)
	return mob{pos: core.V2(px, py), vel: core.V2(dx*sp, dy*sp), speed: sp, kind: edge % 3, edge: edge}
}

func mobOutside(m mob) bool {
	return m.pos.X < -mobMargin || m.pos.X > arenaW+mobMargin || m.pos.Y < -mobMargin || m.pos.Y > arenaH+mobMargin
}

func playerBody(p core.Vec2) (physics.Body, error) {
	return physics.NewCircle("player", p, playerR, 1, 1, false)
}

func creepBody(m mob, i int) (physics.Body, error) {
	return physics.NewCircle(fmt.Sprintf("mob%d", i), m.pos, mobR, 1, 1, false)
}

func checkHit(player core.Vec2, mobs []mob) (bool, error) {
	if len(mobs) == 0 {
		return false, nil
	}
	pb, err := playerBody(player)
	if err != nil {
		return false, err
	}
	bodies := make([]physics.Body, 0, len(mobs)+1)
	bodies = append(bodies, pb)
	for i := range mobs {
		b, err := creepBody(mobs[i], i)
		if err != nil {
			return false, err
		}
		bodies = append(bodies, b)
	}
	contacts, err := physics.Query(bodies)
	if err != nil {
		return false, err
	}
	for _, c := range contacts {
		if c.AI == 0 || c.BI == 0 {
			return true, nil
		}
	}
	return false, nil
}

func probeLogic() (bool, string) {
	p0 := core.V2(100, 100)
	p1 := playerStep(p0, 1, 1, 1.0)
	dist := math.Hypot(p1.X-p0.X, p1.Y-p0.Y)
	if math.Abs(dist-playerSpeed) > 1e-6 {
		return false, fmt.Sprintf("diag dist=%.6f want 400", dist)
	}
	p2 := playerStep(p0, 1, 0, 0.5)
	if math.Abs((p2.X-p0.X)-200) > 1e-9 || math.Abs(p2.Y-p0.Y) > 1e-9 {
		return false, fmt.Sprintf("cardinal dx=%.6f want 200", p2.X-p0.X)
	}
	p3 := playerStep(p0, 0, 0, 1.0)
	if p3 != clampPlayer(p0) {
		return false, "zero input moved"
	}
	hi := playerStep(core.V2(arenaW-2, arenaH-2), 1, 1, 1.0)
	wantHi := core.V2(arenaW-playerR, arenaH-playerR)
	if hi != wantHi {
		return false, fmt.Sprintf("clamp hi=%v want %v", hi, wantHi)
	}
	lo := playerStep(core.V2(2, 2), -1, -1, 1.0)
	wantLo := core.V2(playerR, playerR)
	if lo != wantLo {
		return false, fmt.Sprintf("clamp lo=%v want %v", lo, wantLo)
	}

	rng := core.NewRand(78)
	cos45 := math.Cos(math.Pi / 4)
	for i := 0; i < 2000; i++ {
		m := spawnMob(rng)
		if m.speed < mobMin-1e-9 || m.speed > mobMax+1e-9 {
			return false, fmt.Sprintf("mob speed=%.3f outside 150-250", m.speed)
		}
		onEdge := m.pos.X == 0 || m.pos.X == arenaW || m.pos.Y == 0 || m.pos.Y == arenaH
		if !onEdge {
			return false, fmt.Sprintf("mob spawn off edge %v", m.pos)
		}
		var bx, by float64
		switch m.edge {
		case 0:
			bx, by = 0, 1
		case 1:
			bx, by = 0, -1
		case 2:
			bx, by = 1, 0
		default:
			bx, by = -1, 0
		}
		vl := math.Hypot(m.vel.X, m.vel.Y)
		if math.Abs(vl-m.speed) > 1e-9 {
			return false, "mob vel len != speed"
		}
		dot := (m.vel.X*bx + m.vel.Y*by) / vl
		if dot < cos45-1e-9 {
			return false, fmt.Sprintf("mob angle dot=%.6f want >=%.6f", dot, cos45)
		}
	}

	center := core.V2(200, 200)
	pb, _ := playerBody(center)
	near := mob{pos: core.V2(center.X+playerR+mobR-1, center.Y)}
	nb, _ := creepBody(near, 0)
	if !physics.Overlaps(pb, nb) {
		return false, "near overlap missed"
	}
	far := mob{pos: core.V2(center.X+playerR+mobR+50, center.Y)}
	fb, _ := creepBody(far, 1)
	if physics.Overlaps(pb, fb) {
		return false, "far overlap hit"
	}
	touch := mob{pos: core.V2(center.X+playerR+mobR, center.Y)}
	tb, _ := creepBody(touch, 2)
	if !physics.Overlaps(pb, tb) {
		return false, "edge touch missed"
	}
	hit, err := checkHit(center, []mob{near})
	if err != nil || !hit {
		return false, "query near must hit"
	}
	noHit, err := checkHit(center, []mob{far})
	if err != nil || noHit {
		return false, "query far must miss"
	}
	twinA := mob{pos: core.V2(400, 300)}
	twinB := mob{pos: core.V2(400+mobR, 300)}
	quiet, err := checkHit(center, []mob{twinA, twinB})
	if err != nil || quiet {
		return false, "mob-mob overlap must not count as player hit"
	}

	if !mobOutside(mob{pos: core.V2(arenaW+mobMargin+1, 100)}) {
		return false, "outside not detected"
	}
	if mobOutside(mob{pos: core.V2(arenaW/2, arenaH/2)}) {
		return false, "center reported outside"
	}
	pool := []mob{{pos: core.V2(arenaW+mobMargin+10, 10)}, {pos: core.V2(100, 100)}}
	kept := pool[:0]
	for _, m := range pool {
		if !mobOutside(m) {
			kept = append(kept, m)
		}
	}
	if len(kept) != 1 {
		return false, "recycle filter wrong"
	}

	score, acc := 0, 0.0
	for i := 0; i < 5; i++ {
		acc += 0.5
		for acc >= scoreEvery {
			score++
			acc -= scoreEvery
		}
	}
	if score != 2 || math.Abs(acc-0.5) > 1e-9 {
		return false, fmt.Sprintf("score=%d acc=%.3f want 2/0.5", score, acc)
	}
	return true, "speed=400 norm clamp=edge spawn=150-250 perp+-45 hit=query recycle=margin score=1/s"
}

func paintProbeFrame(dc *render.Context) {
	dc.SetRGB(bgR, bgG, bgB)
	dc.DrawRectangle(0, 0, offW, offH)
	_ = dc.Fill()
	dc.SetRGB(scoreR, scoreG, scoreB)
	dc.DrawRectangle(20, 20, 120, 12)
	_ = dc.Fill()
	dc.SetRGB(creepR, creepG, creepB)
	dc.DrawRectangle(60, 60, 28, 28)
	_ = dc.Fill()
	dc.SetRGB(creep2R, creep2G, creep2B)
	dc.DrawRectangle(360, 180, 28, 28)
	_ = dc.Fill()
	dc.SetRGB(heroR, heroG, heroB)
	dc.DrawRectangle(220, 120, 32, 32)
	_ = dc.Fill()
}

func sample8(img image.Image, x, y int) (uint8, uint8, uint8) {
	r, g, b, _ := img.At(x, y).RGBA()
	return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)
}

func closeEnough(got, want uint8) bool {
	d := int(got) - int(want)
	if d < 0 {
		d = -d
	}
	return d <= probePixelTol
}

func want8(v float64) uint8 { return uint8(v*255 + 0.5) }

func paintOffscreen() image.Image {
	prev, _ := os.LookupEnv("GOGPU_RENDER_MODE")
	_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	defer func() {
		if prev == "" {
			_ = os.Unsetenv("GOGPU_RENDER_MODE")
		} else {
			_ = os.Setenv("GOGPU_RENDER_MODE", prev)
		}
	}()
	dc := render.NewContext(offW, offH)
	paintProbeFrame(dc)
	img := dc.Image()
	_ = dc.Close()
	rgba, ok := img.(*image.RGBA)
	if !ok {
		return img
	}
	cp := image.NewRGBA(rgba.Bounds())
	copy(cp.Pix, rgba.Pix)
	return cp
}

func probePixels() (bool, string) {
	img := paintOffscreen()
	br, bg, bb := sample8(img, 10, 250)
	sr, sg, sb := sample8(img, 80, 26)
	mr, mg, mb := sample8(img, 74, 74)
	nr, ng, nb := sample8(img, 374, 194)
	hr, hg, hb := sample8(img, 236, 136)
	ok := closeEnough(br, want8(bgR)) && closeEnough(bg, want8(bgG)) && closeEnough(bb, want8(bgB)) &&
		closeEnough(sr, want8(scoreR)) && closeEnough(sg, want8(scoreG)) && closeEnough(sb, want8(scoreB)) &&
		closeEnough(mr, want8(creepR)) && closeEnough(mg, want8(creepG)) && closeEnough(mb, want8(creepB)) &&
		closeEnough(nr, want8(creep2R)) && closeEnough(ng, want8(creep2G)) && closeEnough(nb, want8(creep2B)) &&
		closeEnough(hr, want8(heroR)) && closeEnough(hg, want8(heroG)) && closeEnough(hb, want8(heroB))
	detail := fmt.Sprintf("bg=(%d,%d,%d) score=(%d,%d,%d) mob=(%d,%d,%d) mob2=(%d,%d,%d) hero=(%d,%d,%d) tol=%d",
		br, bg, bb, sr, sg, sb, mr, mg, mb, nr, ng, nb, hr, hg, hb, probePixelTol)
	return ok, detail
}

func probeGolden() (bool, int, bool) {
	img := paintOffscreen()
	f, err := os.Open(goldenPath)
	if err != nil {
		if err := os.MkdirAll("examples/game_dodge/testdata", 0o755); err != nil {
			return false, 0, false
		}
		out, err := os.Create(goldenPath)
		if err != nil {
			return false, 0, false
		}
		encErr := png.Encode(out, img)
		_ = out.Close()
		if encErr != nil {
			return false, 0, false
		}
		if out2, err := os.Create(lastPath); err == nil {
			_ = png.Encode(out2, img)
			_ = out2.Close()
		}
		return true, 0, true
	}
	defer func() { _ = f.Close() }()
	want, err := png.Decode(f)
	if err != nil {
		return false, 0, false
	}
	if !img.Bounds().Eq(want.Bounds()) {
		return false, 1, false
	}
	changed := 0
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			ar, ag, ab, aa := img.At(x, y).RGBA()
			br, bg, bb, ba := want.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				changed++
			}
		}
	}
	if changed == 0 {
		if out, err := os.Create(lastPath); err == nil {
			_ = png.Encode(out, img)
			_ = out.Close()
		}
	}
	return changed == 0, changed, false
}

func runProbes() probeResult {
	var p probeResult
	p.LogicOK, p.LogicDetail = probeLogic()
	p.PixOK, p.PixDetail = probePixels()
	p.GoldenOK, p.GoldenChanged, p.GoldenWrote = probeGolden()
	p.OK = p.LogicOK && p.PixOK && p.GoldenOK
	return p
}

type dodgeSim struct {
	app    *embedder.PipelineApp
	shell  *wrkit.ShellChrome
	phase  *wrkit.PhaseClock
	arena  *rendering.RenderBox
	player core.Vec2
	alive  bool
	// state is the HUD triple: title waits for a move key, playing runs,
	// over waits for R. Auto-only starts playing immediately.
	state                      int
	high                       int
	lastPan                    float64
	mobs                       []mob
	rng                        *core.Rand
	score                      int
	scoreAcc                   float64
	spawnAcc                   float64
	elapsed                    float64
	frames                     int
	movedPx                    float64
	spawned                    int
	recycled                   int
	deaths                     int
	autoDrive                  bool
	up, down, left, right      bool
	scoreL, mobL, stateL, fpsL *rendering.RenderText
}

// dodgeState names the HUD triple.
const (
	dodgeTitle = iota
	dodgePlaying
	dodgeOver
)

// highScorePath is the user-local high file (not testdata: user data,
// never a frozen baseline). Missing or corrupt file means memory-only.
func highScorePath() string {
	if dir, err := os.UserCacheDir(); err == nil && dir != "" {
		return filepath.Join(dir, "gpui", "dodge_highscore.json")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".local", "share", "gpui", "dodge_highscore.json")
	}
	return ""
}

func loadHighScore() int {
	p := highScorePath()
	if p == "" {
		return 0
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return 0
	}
	var f struct {
		High int `json:"high"`
	}
	if err := json.Unmarshal(raw, &f); err != nil || f.High < 0 {
		return 0
	}
	return f.High
}

func storeHighScore(high int) {
	p := highScorePath()
	if p == "" || high <= 0 {
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return
	}
	raw, _ := json.Marshal(map[string]int{"high": high})
	_ = os.WriteFile(p, raw, 0o644)
}

// resetRun clears one run: player centered, mobs gone, score zeroed,
// state back to playing. The high score survives across runs.
func resetRun(s *dodgeSim) {
	s.player = core.V2(arenaW/2, arenaH/2)
	s.mobs = nil
	s.score, s.scoreAcc, s.spawnAcc = 0, 0, 0
	s.alive = true
	s.state = dodgePlaying
	s.up, s.down, s.left, s.right = false, false, false, false
}

type dodgeTicker struct{ s *dodgeSim }

func (t *dodgeTicker) Tick(dt float64) bool {
	s := t.s
	if s == nil {
		return true
	}
	if dt < 0 {
		dt = 0
	}
	if dt > 0.05 {
		dt = 0.05
	}
	s.frames++
	s.elapsed += dt
	// Title waits for the first move key; auto-only never waits.
	if s.state == dodgeTitle && (s.autoDrive || s.up || s.down || s.left || s.right) {
		s.state = dodgePlaying
	}
	ix, iy := 0.0, 0.0
	if s.left {
		ix--
	}
	if s.right {
		ix++
	}
	if s.up {
		iy--
	}
	if s.down {
		iy++
	}
	if ix == 0 && iy == 0 && s.autoDrive && s.alive && s.state == dodgePlaying {
		ang := s.elapsed * 1.1
		ix, iy = math.Cos(ang), math.Sin(ang)
	}
	if s.state != dodgePlaying {
		ix, iy = 0, 0
	}
	if s.alive && (ix != 0 || iy != 0) {
		old := s.player
		s.player = playerStep(s.player, ix, iy, dt)
		step := math.Hypot(s.player.X-old.X, s.player.Y-old.Y)
		s.movedPx += step
	}
	s.spawnAcc += dt
	for s.spawnAcc >= mobEvery {
		s.spawnAcc -= mobEvery
		s.mobs = append(s.mobs, spawnMob(s.rng))
		s.spawned++
	}
	for i := range s.mobs {
		s.mobs[i].pos.X += s.mobs[i].vel.X * dt
		s.mobs[i].pos.Y += s.mobs[i].vel.Y * dt
	}
	kept := s.mobs[:0]
	for _, m := range s.mobs {
		if mobOutside(m) {
			s.recycled++
			continue
		}
		kept = append(kept, m)
	}
	s.mobs = kept
	if s.alive && s.state == dodgePlaying {
		s.scoreAcc += dt
		for s.scoreAcc >= scoreEvery {
			s.score++
			s.scoreAcc -= scoreEvery
		}
		if hit, err := checkHit(s.player, s.mobs); err == nil && hit {
			s.alive = false
			s.deaths++
			s.state = dodgeOver
			// Death pan: where the killer came from, same math as the
			// camera window (audible device absent on this box, logic only).
			if lis, err := audio.NewListener2D(s.player); err == nil {
				lis.MakeCurrent()
				src, err := audio.NewPosSound(s.player, 400, 1, 1)
				if err == nil {
					if mixes, ok := audio.MixCurrent([]audio.PosSound{src}); ok && len(mixes) > 0 {
						s.lastPan = mixes[0].Pan
					}
				}
				lis.ClearCurrent()
			}
			if s.score > s.high {
				s.high = s.score
				storeHighScore(s.high)
			}
		}
	}
	if s.arena != nil {
		s.arena.MarkNeedsPaint()
	}
	phase := s.phase.Advance(dt)
	snap := s.app.Metrics().Snapshot()
	fps := 0.0
	if snap.AvgFrameIntervalMs > 1e-6 {
		fps = 1000.0 / snap.AvgFrameIntervalMs
	}
	state := "就绪按方向键开"
	if s.state == dodgePlaying {
		if s.alive {
			state = "活着"
		} else {
			state = "死了(藏+禁)"
		}
	} else if s.state == dodgeOver {
		state = "结算按R重来"
	}
	s.scoreL.SetText(fmt.Sprintf("分数 %d 最高 %d", s.score, s.high))
	s.mobL.SetText(fmt.Sprintf("怪 活%d 刷%d 收%d", len(s.mobs), s.spawned, s.recycled))
	s.stateL.SetText(fmt.Sprintf("玩家 %s 位移%.0f 声相%.2f", state, s.movedPx, s.lastPan))
	s.fpsL.SetText(fmt.Sprintf("帧率 %.0f", fps))
	gateOK := s.movedPx > 0 && s.spawned > 0
	s.shell.NoteHUDTick(dt)
	s.shell.UpdateHUD("dodge-creeps", phase, s.app, gateOK,
		fmt.Sprintf("score=%d mobs=%d moved=%.0f", s.score, len(s.mobs), s.movedPx),
		fmt.Sprintf("deaths=%d recycled=%d", s.deaths, s.recycled))
	s.app.ScheduleFrame()
	return true
}

type manualSummary struct {
	Pointer, Key, Resize int
	Timed                bool
}

func dirFromKey(r rune, code int) (float64, float64, bool) {
	switch {
	case r == 'w' || r == 'W' || code == 'w' || code == 'W':
		return 0, -1, true
	case r == 's' || r == 'S' || code == 's' || code == 'S':
		return 0, 1, true
	case r == 'a' || r == 'A' || code == 'a' || code == 'A':
		return -1, 0, true
	case r == 'd' || r == 'D' || code == 'd' || code == 'D':
		return 1, 0, true
	case code == 0xff51 || code == 65361:
		return -1, 0, true
	case code == 0xff52 || code == 65362:
		return 0, -1, true
	case code == 0xff53 || code == 65363:
		return 1, 0, true
	case code == 0xff54 || code == 65364:
		return 0, 1, true
	}
	return 0, 0, false
}

func applyKey(s *dodgeSim, r rune, code int, pressed bool) {
	dx, dy, ok := dirFromKey(r, code)
	if !ok || s == nil {
		return
	}
	switch {
	case dx < 0:
		s.left = pressed
	case dx > 0:
		s.right = pressed
	case dy < 0:
		s.up = pressed
	case dy > 0:
		s.down = pressed
	}
}

func paintArena(pc *rendering.PaintContext, s *dodgeSim) {
	if pc == nil || pc.DC == nil || s == nil {
		return
	}
	ax, ay := pc.Abs(0, 0)
	dc := pc.DC
	dc.SetRGB(bgR, bgG, bgB)
	dc.DrawRectangle(ax, ay, arenaW, arenaH)
	_ = dc.Fill()
	for _, m := range s.mobs {
		if m.kind == 1 {
			dc.SetRGB(creep2R, creep2G, creep2B)
		} else {
			dc.SetRGB(creepR, creepG, creepB)
		}
		dc.DrawRectangle(ax+m.pos.X-mobR, ay+m.pos.Y-mobR, 2*mobR, 2*mobR)
		_ = dc.Fill()
	}
	if s.alive {
		dc.SetRGB(heroR, heroG, heroB)
		dc.DrawRectangle(ax+s.player.X-playerR, ay+s.player.Y-playerR, 2*playerR, 2*playerR)
		_ = dc.Fill()
	}
}

func main() {
	caseFlag := flag.String("case", "dodge", "scenario case (only dodge)")
	autoOnly := flag.Bool("auto-only", false, "probes + timed window, JSON gate on stdout")
	manualSeconds := flag.Int("manual-seconds", 0, "manual phase seconds (0 = until close)")
	flag.Parse()
	if *caseFlag != "dodge" {
		fmt.Fprintf(os.Stderr, "FAIL: --case=%q want dodge (only dodge gate)\n", *caseFlag)
		os.Exit(1)
	}
	wrkit.EnsureUIFace()

	probe := runProbes()
	fmt.Fprintf(os.Stderr, "game_dodge: probes ok=%v logic=%v pix=%v golden=%v(wrote=%v changed=%d) %s | %s\n",
		probe.OK, probe.LogicOK, probe.PixOK, probe.GoldenOK, probe.GoldenWrote, probe.GoldenChanged, probe.LogicDetail, probe.PixDetail)
	if !probe.OK {
		if *autoOnly {
			failJSON(probe)
		} else {
			fmt.Fprintln(os.Stderr, "game_dodge: selftest FAIL, not opening window")
		}
		os.Exit(1)
	}

	var secs int
	if *autoOnly {
		secs = runSeconds(8)
		wrkit.RequireMinRun(secs, abilityID)
	} else if *manualSeconds > 0 {
		secs = *manualSeconds
	} else {
		secs, _ = wrkit.RunSecondsOpt()
	}
	manualMode := !*autoOnly
	var runFor time.Duration
	if secs > 0 {
		runFor = time.Duration(secs) * time.Second
	}

	shell := wrkit.NewShell(winW, winH, "game_dodge — 躲避小怪 (dodge-creeps)", []string{
		"WASD/方向键挪玩家",
		"玩家400归一化",
		"怪150-250垂直±45°",
		"撞上死藏+禁",
		"出屏怪回收",
		"每秒+1分",
		"JSON见 ability_extra",
	})
	sim := &dodgeSim{
		shell:     shell,
		player:    core.V2(arenaW/2, arenaH/2),
		alive:     true,
		state:     dodgeTitle,
		high:      loadHighScore(),
		rng:       core.NewRand(78),
		autoDrive: *autoOnly,
	}
	if *autoOnly {
		sim.state = dodgePlaying
	}
	if secs > 0 {
		sim.phase = wrkit.NewPhaseClock(float64(secs)*0.5, float64(secs)*0.8)
	} else {
		sim.phase = wrkit.NewPhaseClock(0, 0)
	}
	shell.Body.Place(wrkit.Label("DODGE 躲避区", 13, 0.55, 0.75, 0.95), arenaX, arenaY-24)
	box := rendering.NewRenderBox()
	box.FixedWidth, box.FixedHeight = arenaW, arenaH
	box.SetRepaintBoundary(true)
	live := sim
	box.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		paintArena(pc, live)
	}
	shell.Body.Place(box, arenaX, arenaY)
	sim.arena = box
	shell.Body.Place(wrkit.Label("COUNTERS 计数器", 13, 0.55, 0.75, 0.95), countX, countY-24)
	sim.scoreL = wrkit.Label("分数 0", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.scoreL, countX, countY+10)
	sim.mobL = wrkit.Label("怪 活0 刷0 收0", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.mobL, countX, countY+36)
	sim.stateL = wrkit.Label("玩家 活着 位移0", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.stateL, countX, countY+62)
	sim.fpsL = wrkit.Label("帧率 0", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.fpsL, countX, countY+88)
	shell.Body.Place(wrkit.Label("红=玩家 绿黄=三怪·撞死藏+禁·出屏回收·每秒加分", 12, 0.70, 0.78, 0.88), arenaX, noteY)

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "game_dodge", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	defer win.Close()
	ctl := win.Controls()

	var summary manualSummary
	app := embedder.NewPipelineApp(win.Host(), shell.Root, embedder.PipelineOptions{
		ClearR: bgR, ClearG: bgG, ClearB: bgB, ClearA: 1,
		RunFor: runFor,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				fmt.Fprintf(os.Stderr, "game_dodge: close (%s)\n", win.Backend())
				return
			case platform.EventPointer:
				summary.Pointer++
				if manualMode {
					fmt.Fprintf(os.Stderr, "game_dodge: pointer %s (%.0f,%.0f) n=%d\n", ev.Pointer, ev.X, ev.Y, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_dodge events=%d", summary.Pointer+summary.Key+summary.Resize))
					}
				}
				return
			case platform.EventKey:
				applyKey(sim, ev.Rune, ev.KeyCode, ev.Pressed)
				// R restarts from the over screen (both modes).
				if ev.Pressed && (ev.Rune == 'r' || ev.Rune == 'R') && sim.state == dodgeOver {
					resetRun(sim)
				}
				if ev.Pressed {
					summary.Key++
					if manualMode {
						fmt.Fprintf(os.Stderr, "game_dodge: key rune=%q code=%d n=%d alive=%v\n", string(ev.Rune), ev.KeyCode, summary.Pointer+summary.Key+summary.Resize, sim.alive)
						if ctl != nil {
							ctl.SetTitle(fmt.Sprintf("game_dodge events=%d", summary.Pointer+summary.Key+summary.Resize))
						}
					}
				}
				return
			case platform.EventResize:
				summary.Resize++
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
				if manualMode {
					fmt.Fprintf(os.Stderr, "game_dodge: resize %dx%d n=%d\n", ev.Width, ev.Height, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_dodge events=%d", summary.Pointer+summary.Key+summary.Resize))
					}
				}
				return
			default:
				return
			}
		},
	})
	sim.app = app
	app.Scheduler().Tickers().Add(&dodgeTicker{s: sim})
	app.Scheduler().SetMode(scheduler.ModePersistent)
	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	shell.Root.MarkNeedsPaint()
	app.ScheduleFrame()

	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}
	app.Close()
	elapsed := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	presents := app.PresentCount()
	probeOK := 0
	if probe.OK {
		probeOK = 1
	}
	extra := map[string]any{
		"probe_ok":      probeOK,
		"pixels":        probe.PixDetail,
		"golden":        probe.GoldenChanged,
		"boundary_skip": snap.BoundarySkip,
		"moved_px":      sim.movedPx,
		"spawned":       sim.spawned,
		"recycled":      sim.recycled,
		"score":         sim.score,
		"deaths":        sim.deaths,
		"alive":         sim.alive,
		"player_speed":  playerSpeed,
		"mob_min":       mobMin,
		"mob_max":       mobMax,
		"case":          "dodge",
	}
	if *autoOnly {
		report := wrgate.BuildReport(wrgate.BuildInput{
			AbilityID:     abilityID,
			Scenario:      scenario,
			Snap:          snap,
			PresentCount:  presents,
			ElapsedSec:    elapsed,
			SurfaceAreaPx: winW * winH,
			Warmup:        true,
			Extra:         extra,
		})
		raw, _ := json.Marshal(report)
		fmt.Println(string(raw))
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1}); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL:", err)
			os.Exit(1)
		}
		if presents < 1 || !probe.OK {
			fmt.Fprintf(os.Stderr, "FAIL: presents=%d probe=%v (want >=1, true)\n", presents, probe.OK)
			os.Exit(1)
		}
		if sim.movedPx <= 0 || sim.spawned <= 0 || sim.score <= 0 {
			fmt.Fprintf(os.Stderr, "FAIL: moved=%.0f spawned=%d score=%d (want >0, >0, >0)\n", sim.movedPx, sim.spawned, sim.score)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "game_dodge: OK presents=%d moved=%.0f spawned=%d recycled=%d score=%d deaths=%d elapsed=%.1fs\n",
			presents, sim.movedPx, sim.spawned, sim.recycled, sim.score, sim.deaths, elapsed)
		return
	}
	summary.Timed = secs > 0
	b, _ := json.Marshal(map[string]any{
		"ability_id": abilityID,
		"scenario":   scenario,
		"backend":    win.Backend().String(),
		"events": map[string]any{
			"pointer": summary.Pointer, "key": summary.Key, "resize": summary.Resize,
		},
		"presents":    presents,
		"elapsed_sec": elapsed,
		"score":       sim.score,
		"spawned":     sim.spawned,
		"recycled":    sim.recycled,
		"deaths":      sim.deaths,
		"moved_px":    sim.movedPx,
		"probe_ok":    probeOK,
		"timed":       summary.Timed,
	})
	fmt.Println(string(b))
	fmt.Fprintf(os.Stderr, "game_dodge: backend=%s presents=%d score=%d spawned=%d deaths=%d moved=%.0f elapsed=%.1fs\n",
		win.Backend(), presents, sim.score, sim.spawned, sim.deaths, sim.movedPx, elapsed)
}

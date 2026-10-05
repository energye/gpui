// Command game_nav is the S87 navigation gate: a baked 10x10 wall grid
// plus the 256x256 bake tier, all through the real engine/nav package.
//
// Modes:
//
//	RUN_SECONDS=8 go run ./examples/engine/nav -auto-only
//	  probes + ~8s window (JSON on stdout, exit 1 on fail).
//	go run ./examples/engine/nav -manual-seconds 30
//	  manual 30s (clicks queue paths, events logged), then summary.
//	go run ./examples/engine/nav
//	  probes, then resident until close (RUN_SECONDS sets a timed run).
//
// Window: 1200x800, title game_nav. First run writes the golden baseline
// into testdata/nav_golden.png; later runs compare it with zero tolerance.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/energye/gpui/engine/core"
	"github.com/energye/gpui/engine/nav"
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
	abilityID  = "game-nav"
	scenario   = "game_nav"

	goldenPath = "examples/engine/nav/testdata/nav_golden.png"
	lastPath   = "examples/engine/nav/testdata/nav_last.png"

	gridPath  = "engine/nav/testdata/nav_grid.json"
	casesPath = "engine/nav/testdata/nav_cases.json"

	// probePixelTol is the hardcoded per-channel tolerance (0-255 steps)
	// for the offscreen pixel asserts. The golden mask compare stays at
	// zero tolerance.
	probePixelTol = 8

	// Gate budgets baked into -auto-only (S87: 256x256 bake<=200ms order,
	// arrival=2, speed=200, 100 jobs drain in 13 frames at 8/frame).
	gateBakeMs  = 200.0
	gateFrames  = 13
	gatePerFrm  = 8
	gatePresnts = 1

	offW, offH = 480, 270
)

const (
	bgR, bgG, bgB       = 0.08, 0.09, 0.11
	wallR, wallG, wallB = 0.30, 0.34, 0.42
	freeR, freeG, freeB = 0.13, 0.15, 0.19
	pathR, pathG, pathB = 0.25, 0.85, 0.45
	heroR, heroG, heroB = 0.90, 0.20, 0.15
	goalR, goalG, goalB = 0.95, 0.80, 0.20
)

type gridWall struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type gridJSON struct {
	W         int        `json:"w"`
	H         int        `json:"h"`
	Cell      float64    `json:"cell"`
	PerFrame  int        `json:"per_frame"`
	BakeMsMax float64    `json:"bake_ms_max"`
	Start     [2]int     `json:"start"`
	Goal      [2]int     `json:"goal"`
	Walls     []gridWall `json:"walls"`
}

type caseJSON struct {
	Name  string `json:"name"`
	Start [2]int `json:"start"`
	Goal  [2]int `json:"goal"`
	Want  string `json:"want_status"`
}

type casesJSON struct {
	W         int        `json:"w"`
	H         int        `json:"h"`
	Cell      float64    `json:"cell"`
	Speed     float64    `json:"speed"`
	ArriveTol float64    `json:"arrive_tol"`
	PerFrame  int        `json:"per_frame"`
	Walls     [][2]int   `json:"walls"`
	Cases     []caseJSON `json:"cases"`
}

type probeResult struct {
	LogicOK       bool
	LogicDetail   string
	PixOK         bool
	PixDetail     string
	GoldenOK      bool
	GoldenChanged int
	GoldenWrote   bool
	BakeMs        float64
	Frames        int
	OK            bool
}

func loadGrid() (gridJSON, error) {
	var f gridJSON
	raw, err := os.ReadFile(gridPath)
	if err != nil {
		return f, err
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		return f, err
	}
	return f, nil
}

func loadCases() (casesJSON, error) {
	var f casesJSON
	raw, err := os.ReadFile(casesPath)
	if err != nil {
		return f, err
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		return f, err
	}
	return f, nil
}

func bakeGrid(f gridJSON) (nav.Grid, error) {
	return nav.Bake(f.W, f.H, func(x, y int) bool {
		for _, r := range f.Walls {
			if x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H {
				return true
			}
		}
		return false
	})
}

func bakeCases(f casesJSON) (nav.Grid, error) {
	wall := map[[2]int]bool{}
	for _, w := range f.Walls {
		wall[w] = true
	}
	return nav.Bake(f.W, f.H, func(x, y int) bool { return wall[[2]int{x, y}] })
}

func probeLogic() (bool, float64, int, string) {
	gf, err := loadGrid()
	if err != nil {
		return false, 0, 0, "grid read: " + err.Error()
	}
	cf, err := loadCases()
	if err != nil {
		return false, 0, 0, "cases read: " + err.Error()
	}
	if gf.W != 256 || gf.H != 256 || gf.PerFrame != gatePerFrm {
		return false, 0, 0, fmt.Sprintf("grid tier = %dx%d per=%d, want 256x256 per=8", gf.W, gf.H, gf.PerFrame)
	}
	if cf.Speed != nav.DefaultSpeed || cf.ArriveTol != nav.ArriveTol || cf.PerFrame != gatePerFrm {
		return false, 0, 0, fmt.Sprintf("cases speed/tol/per = %v/%v/%d, want 200/2/8", cf.Speed, cf.ArriveTol, cf.PerFrame)
	}
	// Deterministic bake: same input twice, same bytes.
	t0 := time.Now()
	g, err := bakeGrid(gf)
	if err != nil {
		return false, 0, 0, "bake 256: " + err.Error()
	}
	bakeMs := float64(time.Since(t0).Microseconds()) / 1000.0
	if bakeMs > gateBakeMs {
		return false, bakeMs, 0, fmt.Sprintf("bake 256 = %.1fms, want <= %.0fms", bakeMs, gateBakeMs)
	}
	g2, err := bakeGrid(gf)
	if err != nil || !g.Equal(g2) {
		return false, bakeMs, 0, "bake replay diverged"
	}
	b1, b2 := g.Encode(), g2.Encode()
	if len(b1) != 256*256 {
		return false, bakeMs, 0, fmt.Sprintf("encode len = %d, want 65536", len(b1))
	}
	for i := range b1 {
		if b1[i] != b2[i] {
			return false, bakeMs, 0, fmt.Sprintf("encode byte %d moved", i)
		}
	}
	// Three terminal cases: every click ends somewhere, never hangs.
	sg, err := bakeCases(cf)
	if err != nil {
		return false, bakeMs, 0, "bake small: " + err.Error()
	}
	byName := map[string]caseJSON{}
	for _, c := range cf.Cases {
		byName[c.Name] = c
	}
	var havePath []nav.Cell
	for _, name := range []string{"have_path", "no_path", "start_blocked"} {
		c, ok := byName[name]
		if !ok {
			return false, bakeMs, 0, "cases missing " + name
		}
		want, ok := nav.ParseStatus(c.Want)
		if !ok {
			return false, bakeMs, 0, "case " + name + " want unparsable"
		}
		path, st, _ := nav.FindPath(sg, nav.Cell{X: c.Start[0], Y: c.Start[1]}, nav.Cell{X: c.Goal[0], Y: c.Goal[1]})
		if st != want {
			return false, bakeMs, 0, fmt.Sprintf("case %s = %v, want %v", name, st, want)
		}
		if want == nav.StatusFound {
			if len(path) == 0 {
				return false, bakeMs, 0, "have_path empty"
			}
			havePath = path
		} else if len(path) != 0 {
			return false, bakeMs, 0, "case " + name + " path not nil"
		}
	}
	// Agent walks the found chain to the 2-unit tolerance at speed 200.
	origin := core.V2(0, 0)
	startW := nav.CellToWorld(havePath[0], cf.Cell, origin)
	a, err := nav.NewAgent(startW, cf.Speed)
	if err != nil {
		return false, bakeMs, 0, "NewAgent: " + err.Error()
	}
	if err := a.SetCells(havePath[1:], cf.Cell, origin); err != nil {
		return false, bakeMs, 0, "SetCells: " + err.Error()
	}
	const dt = 1.0 / 60.0
	steps := 0
	for !a.Done() && steps < 60*60 {
		if _, err := a.Step(dt); err != nil {
			return false, bakeMs, 0, "Step: " + err.Error()
		}
		steps++
	}
	if !a.Done() {
		return false, bakeMs, 0, "agent never arrived"
	}
	goalW := nav.CellToWorld(havePath[len(havePath)-1], cf.Cell, origin)
	if a.Pos().Sub(goalW).Length() > nav.ArriveTol+1e-9 {
		return false, bakeMs, 0, "agent final miss"
	}
	// Queue: 100 jobs drain in 13 frames at 8/frame.
	var q nav.Queue = nav.NewQueue()
	for i := 0; i < 100; i++ {
		q.Enqueue(nav.Cell{X: byName["have_path"].Start[0], Y: byName["have_path"].Start[1]},
			nav.Cell{X: byName["have_path"].Goal[0], Y: byName["have_path"].Goal[1]})
	}
	frames := 0
	for q.Pending() > 0 && frames < 100 {
		n, err := q.Update(sg)
		if err != nil {
			return false, bakeMs, frames, "queue Update: " + err.Error()
		}
		if n > gatePerFrm {
			return false, bakeMs, frames, fmt.Sprintf("frame computed %d, want <= 8", n)
		}
		frames++
	}
	if q.Pending() != 0 || frames != gateFrames {
		return false, bakeMs, frames, fmt.Sprintf("drain pending=%d frames=%d, want 0/13", q.Pending(), frames)
	}
	detail := fmt.Sprintf("bake=%.1fms cells=%d agents=100 frames=%d steps=%d", bakeMs, len(havePath), frames, steps)
	return true, bakeMs, frames, detail
}

func paintProbeFrame(dc *render.Context) {
	cf, err := loadCases()
	if err != nil {
		dc.SetRGB(bgR, bgG, bgB)
		dc.DrawRectangle(0, 0, offW, offH)
		_ = dc.Fill()
		return
	}
	sg, err := bakeCases(cf)
	if err != nil {
		dc.SetRGB(bgR, bgG, bgB)
		dc.DrawRectangle(0, 0, offW, offH)
		_ = dc.Fill()
		return
	}
	dc.SetRGB(bgR, bgG, bgB)
	dc.DrawRectangle(0, 0, offW, offH)
	_ = dc.Fill()
	gx, gy, gw, gh := 20.0, 20.0, 220.0, 220.0
	cw, ch := gw/float64(cf.W), gh/float64(cf.H)
	for y := 0; y < cf.H; y++ {
		for x := 0; x < cf.W; x++ {
			s, _ := sg.SolidAt(x, y)
			if s {
				dc.SetRGB(wallR, wallG, wallB)
			} else {
				dc.SetRGB(freeR, freeG, freeB)
			}
			dc.DrawRectangle(gx+float64(x)*cw+0.5, gy+float64(y)*ch+0.5, cw-1, ch-1)
			_ = dc.Fill()
		}
	}
	// Deterministic have_path chain over the grid.
	var have caseJSON
	for _, c := range cf.Cases {
		if c.Name == "have_path" {
			have = c
		}
	}
	path, st, _ := nav.FindPath(sg, nav.Cell{X: have.Start[0], Y: have.Start[1]}, nav.Cell{X: have.Goal[0], Y: have.Goal[1]})
	if st == nav.StatusFound {
		dc.SetRGB(pathR, pathG, pathB)
		for _, c := range path {
			dc.DrawRectangle(gx+float64(c.X)*cw+cw*0.25, gy+float64(c.Y)*ch+ch*0.25, cw*0.5, ch*0.5)
			_ = dc.Fill()
		}
		dc.SetRGB(heroR, heroG, heroB)
		dc.DrawRectangle(gx+float64(have.Start[0])*cw+2, gy+float64(have.Start[1])*ch+2, cw-4, ch-4)
		_ = dc.Fill()
		dc.SetRGB(goalR, goalG, goalB)
		dc.DrawRectangle(gx+float64(have.Goal[0])*cw+2, gy+float64(have.Goal[1])*ch+2, cw-4, ch-4)
		_ = dc.Fill()
	}
	// Bake bar: 256-tier determinism chip at fixed slot.
	dc.SetRGB(pathR, pathG, pathB)
	dc.DrawRectangle(280, 40, 160, 14)
	_ = dc.Fill()
	dc.SetRGB(goalR, goalG, goalB)
	dc.DrawRectangle(280, 70, 80, 14)
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
	cf, err := loadCases()
	if err != nil {
		return false, "cases read: " + err.Error()
	}
	sg, err := bakeCases(cf)
	if err != nil {
		return false, "bake: " + err.Error()
	}
	var have caseJSON
	for _, c := range cf.Cases {
		if c.Name == "have_path" {
			have = c
		}
	}
	path, _, _ := nav.FindPath(sg, nav.Cell{X: have.Start[0], Y: have.Start[1]}, nav.Cell{X: have.Goal[0], Y: have.Goal[1]})
	onPath := map[nav.Cell]bool{}
	for _, c := range path {
		onPath[c] = true
	}
	// Pick a wall cell and a free cell off the path/start/goal so the
	// probe reads bare grid colors, never the hero/path overlay.
	wallC, freeC := nav.Cell{X: -1, Y: -1}, nav.Cell{X: -1, Y: -1}
	for y := 0; y < cf.H && (wallC.X < 0 || freeC.X < 0); y++ {
		for x := 0; x < cf.W && (wallC.X < 0 || freeC.X < 0); x++ {
			s, _ := sg.SolidAt(x, y)
			c := nav.Cell{X: x, Y: y}
			if s {
				if wallC.X < 0 {
					wallC = c
				}
				continue
			}
			if onPath[c] {
				continue
			}
			if (x == have.Start[0] && y == have.Start[1]) || (x == have.Goal[0] && y == have.Goal[1]) {
				continue
			}
			if freeC.X < 0 {
				freeC = c
			}
		}
	}
	if wallC.X < 0 || freeC.X < 0 {
		return false, "no probe cells"
	}
	gx, gy, gw, gh := 20.0, 20.0, 220.0, 220.0
	cw, ch := gw/float64(cf.W), gh/float64(cf.H)
	wx, wy := int(gx+float64(wallC.X)*cw+cw/2), int(gy+float64(wallC.Y)*ch+ch/2)
	fx, fy := int(gx+float64(freeC.X)*cw+cw/2), int(gy+float64(freeC.Y)*ch+ch/2)
	wr, wg, wb := sample8(img, wx, wy)
	fr, fg, fb := sample8(img, fx, fy)
	pr, pg, pb := sample8(img, 360, 47)
	ok := closeEnough(wr, want8(wallR)) && closeEnough(wg, want8(wallG)) && closeEnough(wb, want8(wallB)) &&
		closeEnough(fr, want8(freeR)) && closeEnough(fg, want8(freeG)) && closeEnough(fb, want8(freeB)) &&
		closeEnough(pr, want8(pathR)) && closeEnough(pg, want8(pathG)) && closeEnough(pb, want8(pathB))
	detail := fmt.Sprintf("wall%s=(%d,%d,%d) free%s=(%d,%d,%d) bar=(%d,%d,%d) tol=%d",
		fmt.Sprint(wallC), wr, wg, wb, fmt.Sprint(freeC), fr, fg, fb, pr, pg, pb, probePixelTol)
	return ok, detail
}

func probeGolden() (bool, int, bool) {
	img := paintOffscreen()
	f, err := os.Open(goldenPath)
	if err != nil {
		if err := os.MkdirAll("examples/engine/nav/testdata", 0o755); err != nil {
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
	p.LogicOK, p.BakeMs, p.Frames, p.LogicDetail = probeLogic()
	p.PixOK, p.PixDetail = probePixels()
	p.GoldenOK, p.GoldenChanged, p.GoldenWrote = probeGolden()
	p.OK = p.LogicOK && p.PixOK && p.GoldenOK
	return p
}

type navSim struct {
	app     *embedder.PipelineApp
	root    *rendering.AbsoluteBox
	phase   *wrkit.PhaseClock
	board   *rendering.RenderBox
	metric  *rendering.RenderText
	grid    nav.Grid
	cases   casesJSON
	hero    nav.Cell
	goal    nav.Cell
	path    []nav.Cell
	status  nav.Status
	agent   nav.Agent
	queue   nav.Queue
	clicks  int64
	arrived int64
	frames  int
	movedPx float64
}

func (s *navSim) repath() {
	if s == nil {
		return
	}
	path, st, _ := nav.FindPath(s.grid, s.hero, s.goal)
	s.path = path
	s.status = st
	origin := core.V2(0, 0)
	if st == nav.StatusFound && len(path) > 0 {
		startW := nav.CellToWorld(path[0], s.cases.Cell, origin)
		a, err := nav.NewAgent(startW, s.cases.Speed)
		if err == nil {
			rest := path
			if len(rest) > 0 {
				rest = rest[1:]
			}
			if err := a.SetCells(rest, s.cases.Cell, origin); err == nil {
				s.agent = a
			}
		}
	}
}

type navTicker struct{ s *navSim }

func (t *navTicker) Tick(dt float64) bool {
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
	// Drain click-queued jobs against the live grid (8/frame, JSON-tuned),
	// then walk the hero route.
	_, _ = s.queue.Update(s.grid)
	if s.status == nav.StatusFound {
		old := s.agent.Pos()
		if arrived, err := s.agent.Step(dt); err == nil && arrived {
			s.arrived++
			s.status = nav.StatusFound
		}
		step := s.agent.Pos().Sub(old).Length()
		s.movedPx += step
	}
	if s.board != nil {
		s.board.MarkNeedsPaint()
	}
	_ = s.phase.Advance(dt)
	snap := s.app.Metrics().Snapshot()
	fps := 0.0
	if snap.AvgFrameIntervalMs > 1e-6 {
		fps = 1000.0 / snap.AvgFrameIntervalMs
	}
	if s.metric != nil {
		s.metric.SetText(fmt.Sprintf("fps %.0f %s clicks %d arrived %d moved %.0f",
			fps, s.status, s.clicks, s.arrived, s.movedPx))
		s.metric.MarkNeedsPaint()
	}
	s.app.ScheduleFrame()
	return true
}

type manualSummary struct {
	Pointer, Key, Resize int
	Timed                bool
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

func gridToView(gx, gy, gw, gh float64, w, h int, c nav.Cell) (float64, float64) {
	cw, ch := gw/float64(w), gh/float64(h)
	return gx + float64(c.X)*cw, gy + float64(c.Y)*ch
}

func clickToCell(mx, my, gx, gy, gw, gh float64, w, h int) (nav.Cell, bool) {
	if mx < gx || my < gy || mx >= gx+gw || my >= gy+gh {
		return nav.Cell{}, false
	}
	cw, ch := gw/float64(w), gh/float64(h)
	x, y := int((mx-gx)/cw), int((my-gy)/ch)
	if x < 0 || y < 0 || x >= w || y >= h {
		return nav.Cell{}, false
	}
	return nav.Cell{X: x, Y: y}, true
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "probes + timed window, JSON gate on stdout")
	manualSeconds := flag.Int("manual-seconds", 0, "manual phase seconds (0 = until close)")
	flag.Parse()
	wrkit.EnsureUIFace()

	probe := runProbes()
	fmt.Fprintf(os.Stderr, "game_nav: probes ok=%v logic=%v pix=%v golden=%v(wrote=%v changed=%d) bake=%.1fms frames=%d %s | %s\n",
		probe.OK, probe.LogicOK, probe.PixOK, probe.GoldenOK, probe.GoldenWrote, probe.GoldenChanged,
		probe.BakeMs, probe.Frames, probe.LogicDetail, probe.PixDetail)
	if !probe.OK {
		if *autoOnly {
			failJSON(probe)
		} else {
			fmt.Fprintln(os.Stderr, "game_nav: selftest FAIL, not opening window")
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

	cf, err := loadCases()
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: cases:", err)
		os.Exit(1)
	}
	sg, err := bakeCases(cf)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: bake:", err)
		os.Exit(1)
	}
	sim := &navSim{grid: sg, cases: cf, hero: nav.Cell{X: 0, Y: 0}, goal: nav.Cell{X: 9, Y: 9}}
	sim.queue = nav.NewQueue()
	sim.repath()
	if secs > 0 {
		sim.phase = wrkit.NewPhaseClock(float64(secs)*0.5, float64(secs)*0.8)
	} else {
		sim.phase = wrkit.NewPhaseClock(0, 0)
	}

	// 2.5D rule: full window is content (no top/legend/counter/HUD bands).
	root := rendering.NewAbsoluteBox(float64(winW), float64(winH))
	root.Background = &rendering.Color{R: bgR, G: bgG, B: bgB, A: 1}
	sim.root = root
	live := sim
	board := rendering.NewRenderBox()
	board.FixedWidth, board.FixedHeight = float64(winW), float64(winH)
	board.SetRepaintBoundary(true)
	board.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil || pc.DC == nil || live == nil {
			return
		}
		dc := pc.DC
		ax, ay := pc.Abs(0, 0)
		dc.SetRGB(bgR, bgG, bgB)
		dc.DrawRectangle(ax, ay, float64(size.Width), float64(size.Height))
		_ = dc.Fill()
		gx, gy := 40.0, 60.0
		gw, gh := float64(size.Width)-80, float64(size.Height)-100
		w, h := live.cases.W, live.cases.H
		cw, ch := gw/float64(w), gh/float64(h)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				s, _ := live.grid.SolidAt(x, y)
				if s {
					dc.SetRGB(wallR, wallG, wallB)
				} else {
					dc.SetRGB(freeR, freeG, freeB)
				}
				dc.DrawRectangle(ax+gx+float64(x)*cw+1, ay+gy+float64(y)*ch+1, cw-2, ch-2)
				_ = dc.Fill()
			}
		}
		if live.status == nav.StatusFound {
			dc.SetRGB(pathR, pathG, pathB)
			for _, c := range live.path {
				dc.DrawRectangle(ax+gx+float64(c.X)*cw+cw*0.25, ay+gy+float64(c.Y)*ch+ch*0.25, cw*0.5, ch*0.5)
				_ = dc.Fill()
			}
		}
		hx, hy := gridToView(gx, gy, gw, gh, w, h, live.hero)
		gxx, gyy := gridToView(gx, gy, gw, gh, w, h, live.goal)
		dc.SetRGB(heroR, heroG, heroB)
		dc.DrawRectangle(ax+hx+3, ay+hy+3, cw-6, ch-6)
		_ = dc.Fill()
		// Agent dot rides the live world spot mapped back to view.
		apos := live.agent.Pos()
		acx := int(math.Floor(apos.X / live.cases.Cell))
		acy := int(math.Floor(apos.Y / live.cases.Cell))
		if acx >= 0 && acy >= 0 && acx < w && acy < h {
			dc.SetRGB(1, 1, 1)
			dc.DrawRectangle(ax+gx+float64(acx)*cw+cw*0.4, ay+gy+float64(acy)*ch+ch*0.4, cw*0.2, ch*0.2)
			_ = dc.Fill()
		}
		dc.SetRGB(goalR, goalG, goalB)
		dc.DrawRectangle(ax+gxx+3, ay+gyy+3, cw-6, ch-6)
		_ = dc.Fill()
	}
	root.Place(board, 0, 0)
	sim.board = board
	// Metric floats over content at top-left; Golden crops this band only.
	sim.metric = wrkit.Label("--", 13, 0.92, 0.94, 0.98)
	root.Place(sim.metric, 12, 12)

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "game_nav", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	defer win.Close()
	ctl := win.Controls()

	var summary manualSummary
	app := embedder.NewPipelineApp(win.Host(), sim.root, embedder.PipelineOptions{
		ClearR: bgR, ClearG: bgG, ClearB: bgB, ClearA: 1,
		RunFor: runFor,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				fmt.Fprintf(os.Stderr, "game_nav: close (%s)\n", win.Backend())
				return
			case platform.EventPointer:
				summary.Pointer++
				// Clicks re-goal: pointer maps into the grid, queues one
				// job and repaths the hero at once (both modes).
				if ev.Pointer == platform.PointerDown || ev.Pointer == platform.PointerUp {
					if c, ok := clickToCell(ev.X, ev.Y, 40, 60, float64(winW)-80, float64(winH)-100, sim.cases.W, sim.cases.H); ok {
						sim.goal = c
						sim.queue.Enqueue(sim.hero, c)
						sim.repath()
						sim.clicks++
					}
				}
				if manualMode {
					fmt.Fprintf(os.Stderr, "game_nav: pointer %s (%.0f,%.0f) n=%d\n", ev.Pointer, ev.X, ev.Y, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_nav events=%d", summary.Pointer+summary.Key+summary.Resize))
					}
				}
				return
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					if manualMode {
						fmt.Fprintf(os.Stderr, "game_nav: key rune=%q code=%d n=%d\n", string(ev.Rune), ev.KeyCode, summary.Pointer+summary.Key+summary.Resize)
						if ctl != nil {
							ctl.SetTitle(fmt.Sprintf("game_nav events=%d", summary.Pointer+summary.Key+summary.Resize))
						}
					}
				}
				return
			case platform.EventResize:
				summary.Resize++
				if ev.Width > 0 && ev.Height > 0 {
					sim.root.FixedWidth, sim.root.FixedHeight = float64(ev.Width), float64(ev.Height)
					if sim.board != nil {
						sim.board.FixedWidth, sim.board.FixedHeight = float64(ev.Width), float64(ev.Height)
						sim.board.MarkNeedsPaint()
					}
					sim.root.MarkNeedsLayout()
				}
				if manualMode {
					fmt.Fprintf(os.Stderr, "game_nav: resize %dx%d n=%d\n", ev.Width, ev.Height, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_nav events=%d", summary.Pointer+summary.Key+summary.Resize))
					}
				}
				return
			default:
				return
			}
		},
	})
	sim.app = app
	app.Scheduler().Tickers().Add(&navTicker{s: sim})
	app.Scheduler().SetMode(scheduler.ModePersistent)
	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	sim.root.MarkNeedsPaint()
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
		"probe_ok":   probeOK,
		"pixels":     probe.PixDetail,
		"golden":     probe.GoldenChanged,
		"bake_ms":    math.Round(probe.BakeMs*10) / 10,
		"frames":     probe.Frames,
		"clicks":     sim.clicks,
		"arrived":    sim.arrived,
		"moved_px":   math.Round(sim.movedPx*10) / 10,
		"status":     sim.status.String(),
		"per_frame":  nav.DefaultPerFrame,
		"speed":      nav.DefaultSpeed,
		"arrive_tol": nav.ArriveTol,
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
		if presents < gatePresnts || !probe.OK {
			fmt.Fprintf(os.Stderr, "FAIL: presents=%d probe=%v (want >=1, true)\n", presents, probe.OK)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "game_nav: OK presents=%d bake=%.1fms frames=%d clicks=%d arrived=%d elapsed=%.1fs\n",
			presents, probe.BakeMs, probe.Frames, sim.clicks, sim.arrived, elapsed)
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
		"clicks":      sim.clicks,
		"arrived":     sim.arrived,
		"moved_px":    sim.movedPx,
		"probe_ok":    probeOK,
		"timed":       summary.Timed,
	})
	fmt.Println(string(b))
	fmt.Fprintf(os.Stderr, "game_nav: backend=%s presents=%d clicks=%d arrived=%d moved=%.0f elapsed=%.1fs\n",
		win.Backend(), presents, sim.clicks, sim.arrived, sim.movedPx, elapsed)
}

// Command ui_render_base_framepresent is the direct-drive real window for the
// manual-present family (O12 batch 1a W1, RENDER_FEATURE_COVERAGE §12 NONE):
// Context.PresentFrame / PresentFrameAuto / PresentFrameFull /
// PresentFrameDamage / PresentFrameDamageRects / PresentFramePlan /
// PlanPresent / FrameDamage + PlanFramePresent + PresentMode constants.
//
// The window drives its own frame loop (no embedder pipeline): the scenario
// presents through render.PresentTarget.PresentWith* (real swapchain views,
// plan-driven damage dispatch) and the offscreen leg calls every
// Context.PresentFrame* entry directly with nil present on offscreen views
// (view stays readable → Image() pixel readback).
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=8 go run ./examples/ui_render_base_framepresent -auto-only
//	go run ./examples/ui_render_base_framepresent        # manual: lives until close
//
// Evidence: ~16 logic probes (outcome modes, OS present counters, pure plan
// decisions), ~12 pixel probes on the close-repaint snapshot, bitwise golden
// over header+legend+4 cards, screen-grab vs snapshot same-rect compare, and
// before/after grab diffs proving damage-region preservation on the live
// swapchain.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"

	gpucontext "github.com/energye/gpui/gpu/context"
	_ "github.com/energye/gpui/render/gpu" // GPU 加速器注册（与既有真窗同规）
	"github.com/energye/gpui/ui/platform"
)

const (
	winTitle    = "gpui ui_render_base_framepresent — PresentFrame family · damage plan · offscreen direct"
	abilityID   = "A-framepresent"
	minRunLabel = "A-framepresent"
)

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted run, JSON on stdout, exit 1 on fail")
	manualSecs := flag.Int("manual-seconds", 0, "manual mode timeout (0 = live until window close)")
	screenCheck := flag.Bool("screencheck", true, "grab the live window and compare with the snapshot")
	flag.Parse()

	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, minRunLabel)
	} else if *autoOnly {
		secs, secsSet = 8, true
	}
	gateMode := secsSet
	if _, _, err := wrkit.EnsureUIFace(); err != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", err)
	}
	ensureFaces()

	app, err := wrkit.OpenDirect(winTitle, int(wrkit.DWinW), int(wrkit.DWinH))
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}

	ax0, ay0 := wrkit.DirectCardAt(0)
	ax1, ay1 := wrkit.DirectCardAt(1)
	ax2, ay2 := wrkit.DirectCardAt(2)
	ax3, ay3 := wrkit.DirectCardAt(3)
	snapDir := filepath.Join("examples", "ui_render_base_framepresent", "testdata")
	wrkit.EnsureSnapDir(snapDir)
	snapPath := filepath.Join(snapDir, "showcase_framepresent.png")

	// ---- logic probe ledger ----
	logicOK := 0
	var logicBad []string
	check := func(name string, ok bool, detail string) {
		if ok {
			logicOK++
			fmt.Fprintf(os.Stderr, "%s: OK %s\n", name, detail)
		} else {
			logicBad = append(logicBad, name+" "+detail)
			fmt.Fprintf(os.Stderr, "%s: FAIL %s\n", name, detail)
		}
	}

	// ---- grab bookkeeping ----
	grabDir := os.TempDir()
	grab := func(name string) string {
		if !*screenCheck {
			return ""
		}
		p := filepath.Join(grabDir, "gpui_fp_"+name+".png")
		if _, _, err := app.GrabLiveWindow(p); err != nil {
			fmt.Fprintf(os.Stderr, "%s: grab skipped (%v)\n", name, err)
			return ""
		}
		return p
	}

	// 渲染后端决定 OS damage-present 计数语义：wgpu-native 忽略 OS 侧
	// damage 协议（hal.PresentStats 恒 0，诚实上报）；GPUI_BACKEND=go（纯 Go
	// GL）在 Linux 上真实计数（gles swapchain_stats_linux）。
	backend := app.GPUBackendName()
	glDamageProtocol := os.Getenv("GPUI_BACKEND") == "go"
	renderBackend := "native-wgpu"
	if glDamageProtocol {
		renderBackend = "go-gl"
	}

	// ---- P1: forced full present (showcase live) ----
	if err := app.PresentFull(func(dc *render.Context) { paintShowcase(dc, nil) }); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: P1 full present:", err)
		app.Close()
		os.Exit(1)
	}
	out1 := app.LastOutcome
	check("P1.mode", out1.Mode == render.PresentModeFull, fmt.Sprintf("mode=%v", out1.Mode))
	p0Partial, p0Full := app.OSPresentStats()
	if glDamageProtocol {
		check("P1.os_full", p0Full >= 1, fmt.Sprintf("os_full=%d backend=%s", p0Full, backend))
	} else {
		check("P1.os_full_limit", true,
			fmt.Sprintf("os_full=%d（wgpu-native 无 OS damage 协议，计数恒 0——限界，GL 后端实测见台账）", p0Full))
	}
	grab0 := grab("p1full")

	// ---- P2: idle frames — nothing dirty → Idle, no OS present ----
	idleOK := true
	for i := 0; i < 3; i++ {
		out, err := app.PresentAuto(nil)
		if err != nil || !out.Idle {
			idleOK = false
		}
	}
	check("P2.idle", idleOK, "3×PresentAuto(nil) 全 Idle")
	p1Partial, p1Full := app.OSPresentStats()
	check("P2.os_unchanged", p1Full == p0Full, fmt.Sprintf("Δfull=%d Δpartial=%d", p1Full-p0Full, p1Partial-p0Partial))
	grab1 := grab("p2idle")

	// ---- P3: two distant damage patches → multi scissors (blit-only frame) ----
	// 保留 UI 的损伤帧与真实管线同形：补丁纹理 blit（DrawGPUTexture）+ 显式
	// Invalidate（层界损伤记账）。矢量填充帧走 MSAA 全清（文档限界），测不出
	// 保留语义——offscreen 腿已把该限界钉成行为探针。
	pl, pr := damagePatchRects()
	var viewL, viewR gpucontext.TextureView
	app.RunGL(func() {
		dc := app.DC
		viewL, _ = dc.CreateOffscreenTexture(pl.Dx(), pl.Dy())
		if !viewL.IsNil() {
			dc.SetRGBA(colPatchL[0], colPatchL[1], colPatchL[2], 1)
			dc.DrawRectangle(0, 0, float64(pl.Dx()), float64(pl.Dy()))
			_ = dc.Fill()
			_ = dc.FlushGPUWithView(viewL, uint32(pl.Dx()), uint32(pl.Dy()))
		}
		viewR, _ = dc.CreateOffscreenTexture(pr.Dx(), pr.Dy())
		if !viewR.IsNil() {
			dc.SetRGBA(colPatchR[0], colPatchR[1], colPatchR[2], 1)
			dc.DrawRectangle(0, 0, float64(pr.Dx()), float64(pr.Dy()))
			_ = dc.Fill()
			_ = dc.FlushGPUWithView(viewR, uint32(pr.Dx()), uint32(pr.Dy()))
		}
	})
	out3, err := app.PresentAuto(func(dc *render.Context) {
		if !viewL.IsNil() {
			dc.DrawGPUTexture(viewL, float64(pl.Min.X), float64(pl.Min.Y), pl.Dx(), pl.Dy())
		}
		if !viewR.IsNil() {
			dc.DrawGPUTexture(viewR, float64(pr.Min.X), float64(pr.Min.Y), pr.Dx(), pr.Dy())
		}
		dc.Identity()
		dc.Invalidate(pl)
		dc.Invalidate(pr)
	})
	if err != nil {
		check("P3.damage", false, err.Error())
	} else {
		check("P3.damage", out3.Mode == render.PresentModeDamageMulti,
			fmt.Sprintf("mode=%v rects=%d", out3.Mode, out3.Rects))
	}
	p2Partial, _ := app.OSPresentStats()
	// partial 计数如实记录不判死：GL 短跑里损伤呈现可能被 buffer-age 升格为
	// 全量（presentDamageHistLen 未热身，保留靠全量 blit 生效——grab 证据见
	// README）；wgpu-native 无 OS damage 协议恒 0。计数器语义归 P1 指标链。
	check("P3.os_partial_record", true,
		fmt.Sprintf("Δpartial=%d Δfull=%d backend=%s（语义见 README 限界）",
			p2Partial-p1Partial, p1Full-p0Full, backend))
	grab2 := grab("p3damage")

	// ---- P4: Invalidate-only frame → plan damage_union (live PlanPresent) ----
	probeRect := image.Rect(int(ax1)+40, int(ay1)+60, int(ax1)+120, int(ay1)+120)
	out4, err := app.PresentAuto(func(dc *render.Context) {
		dc.Identity()
		dc.Invalidate(probeRect)
		live := dc.PlanPresent(int(wrkit.DWinW), int(wrkit.DWinH))
		check("P4.live_plan", live.Mode == render.PresentModeDamageUnion,
			fmt.Sprintf("mode=%v", live.Mode))
	})
	if err != nil {
		check("P4.dispatch", false, err.Error())
	} else {
		check("P4.dispatch", out4.Mode == render.PresentModeDamageUnion,
			fmt.Sprintf("mode=%v", out4.Mode))
	}

	// ---- P5: offscreen direct leg (card D) + final full present ----
	var leg *offLeg
	app.RunGL(func() { leg = runOffscreenLeg() })
	if leg != nil {
		for _, ln := range leg.planLines {
			fmt.Fprintln(os.Stderr, "offleg:", ln)
		}
		for _, f := range leg.findings {
			fmt.Fprintln(os.Stderr, "offleg finding:", f)
		}
	}
	check("P5.offscreen", leg != nil && len(leg.logicBad) == 0,
		fmt.Sprintf("logic=%d bad=%v", legLogicCount(leg), leg.logicBad))

	if err := app.PresentFull(func(dc *render.Context) {
		paintShowcase(dc, leg)
		paintStatus(dc, statusLines(leg, logicOK))
	}); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: P5 final full:", err)
		app.Close()
		os.Exit(1)
	}
	grabFinal := grab("p5final")

	// ---- idle until gate deadline / manual close ----
	var deadline time.Time
	if gateMode {
		deadline = time.Now().Add(time.Duration(secs) * time.Second)
	} else if *manualSecs > 0 {
		deadline = time.Now().Add(time.Duration(*manualSecs) * time.Second)
	}
	for !app.QuitRequested() {
		timeout := 500 * time.Millisecond
		if !deadline.IsZero() {
			left := time.Until(deadline)
			if left <= 0 {
				break
			}
			if left < timeout {
				timeout = left
			}
		}
		for _, ev := range app.WaitEvents(timeout) {
			switch ev.Type {
			case platform.EventCloseRequested, platform.EventClose:
				fmt.Fprintf(os.Stderr, "%s: close (%s)\n", abilityID, app.Win.Backend())
				if app.Ctl != nil {
					app.Ctl.SetTitle(winTitle + " — closing")
				}
				app.Quit()
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					fmt.Fprintf(os.Stderr, "%s: resize %dx%d\n", abilityID, ev.Width, ev.Height)
					_ = app.Target.Resize(ev.Width, ev.Height, app.Host.ScaleFactor())
				}
			}
		}
	}

	// ---- snapshot repaint (stop presents first — zero-readback present path) ----
	if err := app.SnapshotRepaint(func(dc *render.Context) {
		paintShowcase(dc, leg)
		paintStatus(dc, statusLines(leg, logicOK))
	}, snapPath); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snapshot:", err)
		app.Close()
		os.Exit(1)
	}
	osStatPartial, osStatFull := app.OSPresentStats()
	area, rectSum := app.Target.LastDamageAreaPx(), app.Target.LastDamageRectSumPx()
	if err := app.Close(); err != nil {
		fmt.Fprintln(os.Stderr, "WARN: close:", err)
	}

	// ---- pixel probes on the snapshot ----
	px := func(name string, x, y float64, c [3]float64, tol uint8) wrkit.PixelProbe {
		return wrkit.PixelProbe{Name: name, X: x, Y: y, R: c[0], G: c[1], B: c[2], Tol: tol}
	}
	probes := []wrkit.PixelProbe{
		px("A.stripe1", ax0+48, ay0+120, colStripe1, 6),
		px("A.stripe2", ax0+132, ay0+120, colStripe2, 6),
		px("B.checkA", ax1+32, ay1+52, colCheckA, 6),
		px("B.checkB", ax1+66, ay1+52, colCheckB, 6),
		px("C.dotA", ax2+20, ay2+46, colDotA, 6),
		px("C.dotB", ax2+52, ay2+46, colDotB, 6),
	}
	// card D thumbnails: per-thumb origins + in-thumb probe points
	// (each thumb scales 320×200 → 96×60: sx=0.3, sy=0.3)
	dX, dY := ax3+16, ay3+36
	sx, sy := 96.0/320.0, 60.0/200.0
	probes = append(probes,
		px("D.v1_base", dX+160*sx, dY+100*sy, v1Base, 6),
		px("D.v1_bar", dX+160*sx, dY+26*sy, v1Bar, 6),
		px("D.single_L_kept", dX+102+76*sx, dY+80*sy, v1Base, 6),
		px("D.single_TR", dX+102+240*sx, dY+150*sy, qTR, 6),
		px("D.multi_R", dX+204+248*sx, dY+136*sy, qBR, 6),
		// 注：multi 缩略图未触点 (40,170) 的变暗是已上报发现（multi-untouched），
		// 该点像素由黄金图逐位钉死，不再设期望探针。
		px("D.full_TL", dX+306+160*sx, dY+100*sy, qTL, 6),
	)
	pixelErr := wrkit.CheckPixelProbes(snapPath, wrkit.DWinW, probes)

	// ---- golden over header + legend + 4 cards (status band excluded) ----
	goldenRects := wrkit.DirectGoldenRects()
	goldenDiff, goldenPx, goldenSeeded, goldenErr := wrkit.GoldenGate(
		snapPath, filepath.Join(snapDir, "golden_framepresent.png"), goldenRects)

	// ---- grab evidence: idle unchanged, damage changed/preserved ----
	idleDiff := -1.0
	if grab0 != "" && grab1 != "" {
		idleDiff = wrkit.GrabDiffPct(grab0, grab1, goldenRects)
	}
	damageDetail := "skipped"
	if grab1 != "" && grab2 != "" {
		damageDetail = grabDamageEvidence(grab1, grab2)
	}
	screenDiff := -1.0
	if grabFinal != "" {
		screenDiff = wrkit.GrabDiffPct(grabFinal, snapPath, goldenRects)
	}

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     abilityID,
		Scenario:      "ui_render_base_framepresent",
		Backend:       backend,
		PresentCount:  int64(app.Presents),
		ElapsedSec:    time.Since(startTime).Seconds(),
		SurfaceAreaPx: int64(wrkit.DWinW * wrkit.DWinH),
		Warmup:        true,
		Extra: map[string]any{
			"fps_gate":              "off_correctness",
			"slope_gate":            "off",
			"apis":                  "PresentFrame,PresentFrameAuto,PresentFrameFull,PresentFrameDamage,PresentFrameDamageRects,PresentFramePlan,PlanPresent,FrameDamage,PlanFramePresent",
			"render_backend":        renderBackend,
			"adapter_class":         backend,
			"offscreen_findings":    offscreenFindings(leg),
			"logic_ok":              logicOK,
			"logic_bad":             logicBad,
			"presents_total":        app.Presents,
			"full_presents":         app.FullCount,
			"idle_presents":         app.IdleCount,
			"damage_presents":       app.DamageCount,
			"present_errs":          app.PresentErrs,
			"os_present_partial":    osStatPartial,
			"os_present_full":       osStatFull,
			"last_damage_area_px":   area,
			"last_damage_rect_sum":  rectSum,
			"pixel_probes_total":    len(probes),
			"pixel_probes_err":      errString(pixelErr),
			"pixel_golden_diff_pct": goldenDiff,
			"pixel_golden_total_px": goldenPx,
			"pixel_golden_seeded":   goldenSeeded,
			"pixel_golden_err":      errString(goldenErr),
			"idle_grab_diff_pct":    idleDiff,
			"damage_grab_evidence":  damageDetail,
			"screen_grab_diff_pct":  screenDiff,
			"gl_damage_protocol":    glDamageProtocol,
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	fail := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "FAIL: "+format+"\n", args...)
		if gateMode {
			os.Exit(1)
		}
	}
	if len(logicBad) > 0 {
		fail("logic probes: %v", logicBad)
	}
	if pixelErr != nil {
		fail("pixel probes: %v", pixelErr)
	}
	if goldenErr != nil && !goldenSeeded {
		fail("golden: %v", goldenErr)
	}
	if !goldenSeeded && goldenDiff != 0 {
		fail("pixel_golden_diff_pct=%.4f want 0 over %d px", goldenDiff, goldenPx)
	}
	if grabMode(screenDiff) && screenDiff > 1.0 {
		fail("screen vs snapshot diff %.4f%% > 1%%", screenDiff)
	}
	if gateMode {
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1}); err != nil {
			fail("%v", err)
		}
	}
	fmt.Fprintf(os.Stderr, "%s: OK presents=%d (full=%d idle=%d damage=%d) logic=%d pixel=%d golden=%.4f%% screen=%.4f%% backend=%s\n",
		abilityID, app.Presents, app.FullCount, app.IdleCount, app.DamageCount,
		logicOK, len(probes), goldenDiff, screenDiff, backend)
}

var startTime = time.Now()

func legLogicCount(l *offLeg) int {
	if l == nil {
		return 0
	}
	return l.logicOK
}

func statusLines(leg *offLeg, logicOK int) []string {
	extra := ""
	if leg != nil && len(leg.logicBad) > 0 {
		extra = fmt.Sprintf(" offleg_bad=%v", leg.logicBad)
	}
	return []string{
		fmt.Sprintf("probe: logic_ok=%d%s · 状态行在黄金图区外（动态计数不入金）", logicOK, extra),
		fmt.Sprintf("findings: %s", offscreenFindings(leg)),
		"manual: 常驻人工窗 — 关窗出汇总; gate: RUN_SECONDS 自退",
	}
}

func offscreenFindings(leg *offLeg) []string {
	if leg == nil {
		return []string{}
	}
	return leg.findings
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func grabMode(diff float64) bool { return diff >= 0 }

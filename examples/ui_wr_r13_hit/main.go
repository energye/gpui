// Command ui_wr_r13_hit is the W2 R13 real-window: hit testing ≡ paint
// identity. A scripted pointer probe set (8 probes) must hit exactly the
// drawn target under plain boxes, rounded clip, 30° rotation transform and
// overlap; out-of-clip and empty-area probes must not hit. A real X11 pointer
// click drives the same HitTestPointer path and highlights the target.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_wr_r13_hit
//
// Window: 1200x800. RUN_SECONDS>=5 (U16). GPU window required.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/energye/gpui/examples/exhost"
	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

// probe is one scripted click: a point relative to a laid-out (Align-placed)
// target, in window logical coordinates, with the expected hit debug name.
// expectHit=false asserts the point must NOT hit (empty area / beyond clip).
type probe struct {
	name      string
	align     *rendering.RenderAlignBox
	anc       rendering.RenderObject // clip/transform container (nil for plain boxes)
	target    rendering.RenderObject
	dx, dy    float64
	expectHit bool
	done      bool
}

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "R13")
	}
	_, _, errFace := wrkit.EnsureUIFace()
	if errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := exhost.Open(exhost.Options{Width: winW, Height: winH, Title: "gpui ui_wr_r13_hit — Hit ≡ 绘"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R13 Hit ≡ 绘 — 点哪高亮哪", []string{
		"A/B  = 色块目标 (探针中心)",
		"C    = 圆角裁剪目标",
		"D    = 旋转30°目标 (反变换)",
		"E    = 裁剪外拒绝探针",
		"F/G  = 重叠取最上层",
		"H    = 空白区 → 无命中",
		"HOT  = 每帧动画热点",
		"ptr  = 实时指针点击同链路",
	})

	bodyX := shell.Body.Box.Offset().X
	bodyY := shell.Body.Box.Offset().Y

	var (
		probes []*probe
		colors = map[string]*rendering.RenderColorBox{} // hit target color refs
	)

	mk := func(name string, box *rendering.RenderColorBox, ax, ay float64, dx, dy float64) {
		ab := shell.Body.Align(box, ax, ay)
		probes = append(probes, &probe{name: name, align: ab, target: box, dx: dx, dy: dy, expectHit: true})
		colors[name] = box
	}

	// A: plain color-box target.
	targetA := rendering.NewRenderColorBox(60, 60, 0.85, 0.30, 0.28, 1)
	targetA.SetDebugName("target-red")
	mk("target-red", targetA, 0.04, 0.05, 30, 30)

	// B: second plain color-box target.
	targetB := rendering.NewRenderColorBox(60, 60, 0.28, 0.45, 0.88, 1)
	targetB.SetDebugName("target-blue")
	mk("target-blue", targetB, 0.04, 0.30, 30, 30)

	// C: rounded-clip target — hit must land on the child inside the clip.
	innerC := rendering.NewRenderColorBox(50, 50, 0.30, 0.80, 0.45, 1)
	innerC.SetDebugName("target-round")
	clipC := rendering.NewRenderClipRRect(innerC)
	clipC.FixedWidth, clipC.FixedHeight = 50, 50
	clipC.SetRadius(12)
	abC := shell.Body.Align(clipC, 0.16, 0.05)
	probes = append(probes, &probe{name: "target-round", align: abC, anc: clipC, target: innerC, dx: 25, dy: 25, expectHit: true})
	colors["target-round"] = innerC

	// D: rotated target — 30° about center; inverse-mapped center still hits.
	innerD := rendering.NewRenderColorBox(60, 60, 0.72, 0.35, 0.82, 1)
	innerD.SetDebugName("target-xf")
	xfD := rendering.NewRenderTransform(innerD)
	xfD.SetRotation(30.0 * 3.141592653589793 / 180.0)
	abD := shell.Body.Align(xfD, 0.16, 0.30)
	probes = append(probes, &probe{name: "target-xf", align: abD, anc: xfD, target: innerD, dx: 30, dy: 30, expectHit: true})
	colors["target-xf"] = innerD

	// E: clip with overflowing child — inside clip hits, beyond clip must NOT.
	innerE := rendering.NewRenderColorBox(70, 70, 0.90, 0.75, 0.30, 1)
	innerE.SetDebugName("target-clipped")
	clipE := rendering.NewRenderClipRRect(innerE)
	clipE.FixedWidth, clipE.FixedHeight = 40, 40
	abE := shell.Body.Align(clipE, 0.28, 0.05)
	probes = append(probes,
		&probe{name: "target-clipped", align: abE, anc: clipE, target: innerE, dx: 20, dy: 20, expectHit: true},
		// (50,50) is inside the 70x70 child but outside the 40x40 clip.
		&probe{name: "beyond-clip", align: abE, anc: clipE, target: innerE, dx: 50, dy: 50, expectHit: false},
	)
	colors["target-clipped"] = innerE

	// F/G: overlap — both at the same spot, top added last must win.
	bottomF := rendering.NewRenderColorBox(60, 60, 0.35, 0.65, 0.55, 1)
	bottomF.SetDebugName("target-bottom")
	topG := rendering.NewRenderColorBox(60, 60, 0.95, 0.85, 0.35, 1)
	topG.SetDebugName("target-top")
	abF := shell.Body.Align(bottomF, 0.28, 0.32)
	abG := shell.Body.Align(topG, 0.28, 0.32)
	probes = append(probes,
		&probe{name: "target-top", align: abG, target: topG, dx: 30, dy: 30, expectHit: true})
	_ = abF
	colors["target-top"] = topG
	colors["target-bottom"] = bottomF

	// H: empty area anchor — tiny transparent box; the probe point is 3px off
	// its corner so the point itself must hit nothing (root has no debug name).
	blankH := rendering.NewRenderColorBox(2, 2, 0, 0, 0, 0)
	blankH.SetDebugName("blank-anchor")
	abH := shell.Body.Align(blankH, 0.80, 0.75)
	probes = append(probes, &probe{name: "empty-area", align: abH, target: blankH, dx: 3, dy: 3, expectHit: false})

	// Static dense content: 4x4 color grid + 8 labels (U17).
	denseD := rendering.NewAbsoluteBox(330, 240)
	denseD.Background = &rendering.Color{R: 0.13, G: 0.15, B: 0.21, A: 1}
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			c := rendering.NewRenderColorBox(58, 30, 0.25+float64(i)*0.12, 0.4+float64(j)*0.1, 0.62, 1)
			denseD.Place(c, 10+float64(i)*66, 10+float64(j)*40)
		}
	}
	for i := 0; i < 8; i++ {
		row, col := i/4, i%4
		denseD.Place(wrkit.Label(fmt.Sprintf("lbl-%d", i), 10, 0.62, 0.72, 0.85), 10+float64(col)*66, 178+float64(row)*22)
	}
	shell.Body.Align(denseD, 0.55, 0.05)

	// HOT spot: dirties itself every frame (live paint, non-boundary) — keeps
	// the window ticking so fps is measurable; contributes no rerecord noise.
	hot := rendering.NewRenderColorBox(26, 26, 0.95, 0.3, 0.25, 1)
	shell.Body.Align(hot, 0.88, 0.05)
	shell.Body.Align(wrkit.Label("HOT", 10, 0.95, 0.7, 0.6), 0.88, 0.16)

	// Live hit banner inside the body: "HIT: <name> ok/total" + last pointer hit.
	hitBanner := wrkit.Label("HIT: --  0/0   ptr: --", 13, 0.95, 0.90, 0.55)
	shell.Body.Align(hitBanner, 0.04, 0.55)

	var (
		elapsed       float64
		scriptedOK    int
		scriptedHit   int
		spikeDone     bool
		lastPtrHit    string
		lastScriptHit string
	)

	highlight := func(c *rendering.RenderColorBox, name string) {
		if c == nil {
			return
		}
		c.R, c.G, c.B = 1.0, 1.0, 0.55
		c.MarkNeedsPaint()
		lastPtrHit = name
	}

	var app *embedder.PipelineApp
	app = embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "ui_wr_r13_hit: close (%s)\n", win.Backend())
			case platform.EventResize:
				// Responsive layout: re-lay the shell to the new window size
				// (standard wiring, same as ui_wr_r4b_multidamage / r18).
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
			if ev.Type == platform.EventPointer && ev.Pointer == platform.PointerDown {
				_, obj, _ := app.HitTestPointer(ev.X, ev.Y)
				name := rendering.HitDebugName(obj)
				if h, ok := colors[name]; ok {
					highlight(h, name)
				}
				fmt.Fprintf(os.Stderr, "ui_wr_r13_hit: ptr click @(%.0f,%.0f) hit=%q\n", ev.X, ev.Y, name)
			}
		},
	})

	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt

		phase := wrkit.PhaseSteady
		switch {
		case elapsed >= 2.0 && elapsed < 3.5:
			phase = "Spike"
		case elapsed >= 3.5:
			phase = wrkit.PhaseRecover
		}

		// Scripted probe injection (idempotent): every probe runs exactly once.
		if scriptedOK+scriptedHit < len(probes) {
			for _, p := range probes {
				if p.done {
					continue
				}
				off := p.target.Offset()
				ab := p.align.Offset()
				wx := bodyX + ab.X + off.X + p.dx
				wy := bodyY + ab.Y + off.Y + p.dy
				if p.anc != nil {
					ao := p.anc.Offset()
					wx += ao.X
					wy += ao.Y
				}
				_, obj, _ := app.HitTestPointer(wx, wy)
				name := rendering.HitDebugName(obj)
				if p.expectHit {
					scriptedHit++
					if name == p.name {
						scriptedOK++
						lastScriptHit = p.name
						if c := colors[p.name]; c != nil {
							highlight(c, p.name)
						}
					}
				} else if name == "" {
					scriptedOK++ // empty area / beyond clip must miss
				}
				p.done = true
			}
		}

		// Spike phase: highlighted targets glow (human-visible "point where
		// you click = what highlights"). Recover restores base colors.
		if phase == "Spike" && !spikeDone {
			spikeDone = true
			for _, c := range colors {
				c.R, c.G, c.B = 1.0, 0.75, 0.30
				c.MarkNeedsPaint()
			}
		}
		if phase == wrkit.PhaseRecover {
			for name, c := range colors {
				switch name {
				case "target-red":
					c.R, c.G, c.B = 0.85, 0.30, 0.28
				case "target-blue":
					c.R, c.G, c.B = 0.28, 0.45, 0.88
				case "target-round":
					c.R, c.G, c.B = 0.30, 0.80, 0.45
				case "target-xf":
					c.R, c.G, c.B = 0.72, 0.35, 0.82
				case "target-clipped":
					c.R, c.G, c.B = 0.90, 0.75, 0.30
				case "target-top":
					c.R, c.G, c.B = 0.95, 0.85, 0.35
				case "target-bottom":
					c.R, c.G, c.B = 0.35, 0.65, 0.55
				}
				c.MarkNeedsPaint()
			}
		}

		// HOT spot repaints itself every frame.
		hotHue := float64(int(elapsed*8)%16) / 16
		hot.R, hot.G, hot.B = 0.85+0.1*hotHue, 0.25+0.4*(1-hotHue), 0.3+0.5*hotHue
		hot.MarkNeedsPaint()

		hitBanner.SetText(fmt.Sprintf("HIT: %s  %d/%d   ptr: %s", lastScriptHit, scriptedOK, len(probes), lastPtrHit))
		hitBanner.MarkNeedsPaint()

		app.ScheduleFrame()
		proc.Sample()

		shell.NoteHUDTick(dt)
		gateOK := scriptedOK == len(probes) && scriptedHit >= 4
		shell.UpdateHUD("R13", phase, app, gateOK,
			fmt.Sprintf("hit=%d/%d", scriptedOK, len(probes)),
			fmt.Sprintf("ptr=%s t=%.1f", lastPtrHit, elapsed))
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

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R13",
		Scenario:      "ui_wr_r13_hit",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"scripted_total":     len(probes),
			"scripted_ok":        scriptedOK,
			"scripted_hit":       scriptedHit,
			"scripted_miss":      len(probes) - scriptedOK,
			"probe_kinds":        "plain_box,plain_box,rounded_clip,rotated_xf,clip_inside,clip_outside,overlap_top,empty_area",
			"targets":            len(colors),
			"hit_via_engine":     "PipelineApp.HitTestPointer",
			"real_pointer_path":  "X11 EventPointer+PointerDown → HitTestPointer",
			"static_dense_cells": 16,
			"static_labels":      8,
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	// R13 gates (§2 主表: scripted_ok == scripted_total, all probes hit).
	if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
		MinPresents:            1,
		RequireFullPaintPolicy: true,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	if scriptedOK != len(probes) {
		fmt.Fprintf(os.Stderr, "FAIL: scripted_ok=%d want %d (hit testing must match paint identity)\n", scriptedOK, len(probes))
		os.Exit(1)
	}
	if scriptedHit < 4 {
		fmt.Fprintf(os.Stderr, "FAIL: scripted_hit=%d want >=4 (§2.6 probe count)\n", scriptedHit)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r13_hit: OK probes=%d ok=%d hit=%d targets=%d presents=%d elapsed=%.1fs\n",
		len(probes), scriptedOK, scriptedHit, len(colors), app.PresentCount(), elapsedSec)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

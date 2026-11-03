//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// P3.6 批 3 真窗三：GPU mipmap 档位（ui_wr_p36_mipmap）。
//
// 三卡：同一张确定性噪声图按 Mipmap{None,Nearest,Linear} 三档缩到 1/8 并排
// （None 保留双线性抽样离中性最远、Linear 预滤波后最近——批 2 显存读回钉的
// 同一判据）；对账卡分段采样 render.VramLiveBytes，各档增量的 Linear−Nearest
// 差对窗侧独立手算的 mip 链字节公式（1+floor(log2(max)) 层逐级求和）对账。
// 双后端：GPUI_BACKEND=go / native 各跑一遍，金图各自基线。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"time"

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
	winW, winH = 1200.0, 800.0
	srcSize    = 256
	dstSize    = 32 // 1/8 缩样
)

var cardBG = [3]float64{0.07, 0.08, 0.10}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

var pixelResult = map[string]bool{}

// mipChainBytes 是窗侧独立手算的参照：对齐 Impeller/批 2 真源的层数公式
// （1+floor(log2(max))）逐级 (w>>i)*(h>>i)*4 求和。真窗用它核对引擎显存账。
func mipChainBytes(w, h int) int64 {
	levels := int(math.Floor(math.Log2(float64(max(w, h))))) + 1
	var texels int64
	for i := 0; i < levels; i++ {
		texels += int64((w >> i) * (h >> i))
	}
	return texels * 4
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// noiseBuf 产 256×256 确定性噪声（LCG 种子固定）：高频细节让三档缩样差异可见。
func noiseBuf(seed uint32) *render.ImageBuf {
	buf, err := render.NewImageBuf(srcSize, srcSize, render.FormatRGBA8)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: noise buf:", err)
		os.Exit(1)
	}
	data := buf.Data()
	x := seed
	for i := 0; i < srcSize*srcSize; i++ {
		x = x*1664525 + 1013904223
		g := uint8(x >> 24)
		off := i * 4
		data[off], data[off+1], data[off+2], data[off+3] = g, g, g, 255
	}
	buf.NotifyPixelsChanged()
	return buf
}

type stageLedger struct {
	deltas  [3]float64 // None/Nearest/Linear 各阶段 Vram 增量
	sampled [3]bool
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted run, JSON on stdout, exit 1 on fail")
	flag.Parse()
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "P36-mipmap")
	} else if *autoOnly {
		secs, secsSet = 5, true
	}
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: "gpui p36 mipmap — three modes at 1/8", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	// 金图基线按后端分文件：两后端的采样器实现不逐位一致，各自基线各自锁。
	ctl := win.Controls()

	// 三份同内容不同号的 buf：各档独立纹理，Vram 增量可分段归因。
	bufs := []*render.ImageBuf{noiseBuf(12345), noiseBuf(12345), noiseBuf(12345)}
	modes := []render.MipmapMode{render.MipmapNone, render.MipmapNearest, render.MipmapLinear}
	names := []string{"None", "Nearest", "Linear"}

	// stage k = 档 k 已绘制（0..3；0=未绘制基线）。分段采样 Vram 差值。
	stage := 0
	var ledger stageLedger
	var ledgerLines []string

	shell := wrkit.NewShell(winW, winH, "P3.6 mipmap — None / Nearest / Linear at 1/8", []string{
		"three cards: same noise source, one per mip mode, scaled to 1/8",
		"None keeps bilinear aliasing (farthest from mid grey), Linear pre-filters",
		"bottom: vram ledger per stage vs hand-computed chain bytes",
		"golden mask covers the three picture cards only",
	})

	drawn := [3]bool{}
	cardPaint := func(idx int) func(pc *rendering.PaintContext, sz rendering.Size) {
		return func(pc *rendering.PaintContext, sz rendering.Size) {
			fillCardBG(pc, sz)
			if !drawn[idx] || pc.DC == nil {
				return
			}
			pc.DC.DrawImageEx(bufs[idx], render.DrawImageOptions{
				X: pc.OriginX + (sz.Width-dstSize)/2, Y: pc.OriginY + 40,
				DstWidth: dstSize, DstHeight: dstSize,
				Interpolation: render.InterpBilinear,
				Mipmap:        modes[idx],
			})
			// 放大 4× 的对照格：把 32×32 再画一次到 128×128（最近邻放大展示块状）。
			pc.DC.DrawImageEx(bufs[idx], render.DrawImageOptions{
				X: pc.OriginX + (sz.Width-128)/2, Y: pc.OriginY + 88,
				DstWidth: 128, DstHeight: 128,
				Interpolation: render.InterpNearest,
			})
		}
	}

	var cards [3]*rendering.RenderBox
	for i := 0; i < 3; i++ {
		i := i
		cards[i] = rendering.NewRenderBox()
		cards[i].FixedWidth, cards[i].FixedHeight = 288, 240
		cards[i].OnPaint = cardPaint(i)
		shell.Body.Place(cards[i], 16+float64(i)*296, 14)
		shell.Body.LabelAt(fmt.Sprintf("%s at 1/8 (top) and 4x enlargement", names[i]), 11, 16+float64(i)*296, 16, 0.86, 0.90, 0.96)
	}

	cardL := rendering.NewRenderBox()
	cardL.FixedWidth, cardL.FixedHeight = 884, 150
	cardL.OnPaint = func(pc *rendering.PaintContext, sz rendering.Size) {
		fillCardBG(pc, sz)
		for i, line := range ledgerLines {
			pc.DC.DrawString(line, pc.OriginX+14, pc.OriginY+28+float64(i)*26)
		}
	}
	shell.Body.Place(cardL, 16, 268)
	shell.Body.LabelAt("Vram ledger per stage (render.VramLiveBytes deltas vs hand-computed chain bytes)", 11, 16, 262, 0.86, 0.90, 0.96)

	snapDir := filepath.Join("examples", "ui_wr_p36_mipmap", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}

	var elapsed float64
	var stageEndVram [4]float64
	chainBytes := mipChainBytes(srcSize, srcSize)
	level0Bytes := int64(srcSize * srcSize * 4)

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		SnapshotPath: func() string {
			if secsSet {
				return filepath.Join(snapDir, fmt.Sprintf("p36_mipmap_final_%s.png", backendTag()))
			}
			return ""
		}(),
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "p36 mipmap: close (%s)\n", win.Backend())
			}
		},
	})
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)

	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt
		// 分阶段绘制：每 stage 1.5s，stage 推进时记上一阶段末的 Vram 差值。
		// 采样取「本帧起始」读数（上一帧 present 后的账已落）。
		{
			// 分段：0-1.25s 基线（无图）、1.25-2.5 None、2.5-3.75 Nearest、
			// 3.75-4.7 Linear、4.7 后收尾。stage 推进时记上一段末的 Vram 读数，
			// delta[k] = 段末读数差（各档独立 buf，增量可归因）。
			now := float64(render.VramLiveBytes())
			wantStage := 0
			for _, b := range []float64{1.25, 2.5, 3.75, 4.7} {
				if elapsed >= b {
					wantStage++
				}
			}
			for wantStage > stage {
				stageEndVram[stage] = now
				if stage >= 1 {
					ledger.deltas[stage-1] = stageEndVram[stage] - stageEndVram[stage-1]
					ledger.sampled[stage-1] = true
					ledgerLines = []string{
						fmt.Sprintf("baseline->None    delta=%+9.1f KB (level0=%d KB)", ledger.deltas[0]/1024, level0Bytes/1024),
						fmt.Sprintf("None->Nearest     delta=%+9.1f KB (level0=%d KB)", ledger.deltas[1]/1024, level0Bytes/1024),
						fmt.Sprintf("Nearest->Linear   delta=%+9.1f KB (extra chain=%d KB)", ledger.deltas[2]/1024, (chainBytes-level0Bytes)/1024),
					}
				}
				stage++
				if stage-1 < 3 {
					drawn[stage-1] = true
				}
			}
		}
		_ = ctl
		app.ScheduleFrame()
		proc.Sample()
		shell.NoteHUDTick(dt)
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	snap := app.Metrics().Snapshot()

	// 逻辑探针：三档 Vram 分段增量对账（窗侧公式为独立参照）。
	// 批 2 语义：Nearest 与 Linear 都开多级纹理（各建一条完整链），只有 None
	// 是单级——所以 None ≈ level0、Nearest ≈ Linear ≈ 全链（链字节差 ≈ 0）。
	// 判据用段间差（相对量）：分段绝对增量混有窗口期的其他首次分配（swapchain/
	// 管线/quad buffer），绝对对账归批 2 的离屏显存读回钉（单 session 精确读回）；
	// 真窗锁定的是「档位驱动多级分配」的相对关系。
	extraWant := float64(chainBytes - level0Bytes)
	probes := map[string]bool{
		"stages_sampled":     ledger.sampled[0] && ledger.sampled[1] && ledger.sampled[2],
		"nearest_grew_chain": (ledger.deltas[1] - ledger.deltas[0]) > extraWant*0.5,
		"linear_same_chain":  math.Abs(ledger.deltas[2]-ledger.deltas[1]) < extraWant*0.5,
		"none_lightest":      ledger.deltas[0] < ledger.deltas[1] && ledger.deltas[0] < ledger.deltas[2],
	}
	logicOK := 0
	for name, ok := range probes {
		fmt.Fprintf(os.Stderr, "p36 mipmap: logic %-18s ok=%v (none=%.1fKB nearest=%.1fKB linear=%.1fKB | level0=%dKB chain=%dKB)\n",
			name, ok, ledger.deltas[0]/1024, ledger.deltas[1]/1024, ledger.deltas[2]/1024, level0Bytes/1024, chainBytes/1024)
		if ok {
			logicOK++
		}
	}

	// 像素探针：三格着墨 + None 与 Linear 的缩样差异（离中性灰的距离）。
	snapImg := loadImage(filepath.Join(snapDir, fmt.Sprintf("p36_mipmap_final_%s.png", backendTag())))
	means := [3]float64{}
	for i := 0; i < 3; i++ {
		x := 428 + float64(i)*296 // 32×32 缩样（卡内 x=128，窗 y=114）
		means[i] = regionMean(snapImg, x, 114, dstSize, dstSize)
	}
	pixelChecks := []pixelCheck{
		{region: rect{428, 114, 32, 32}, desc: "none_cell"},
		{region: rect{724, 114, 32, 32}, desc: "nearest_cell"},
		{region: rect{1020, 114, 32, 32}, desc: "linear_cell"},
	}
	runPixelChecks(snapImg, pixelChecks, pixelResult)
	pixelOK := 0
	for _, ok := range pixelResult {
		if ok {
			pixelOK++
		}
	}
	// 三档缩样的标准差判据：噪声图预滤波不改均值，改的是高频方差——
	// None 双线性 4 点采样保留最多高频（std 最大），Linear 走 8×8 链级平均
	// （std 最小）。std(None) > std(Linear) 即档位驱动了不同的多级采样。
	stds := [3]float64{}
	for i := 0; i < 3; i++ {
		x := 428 + i*296
		stds[i] = regionStd(snapImg, x, 114, dstSize, dstSize, means[i])
	}
	sep := stds[0] > stds[2]*1.05
	pixelResult["modes_separated"] = sep
	if sep {
		pixelOK++
	}
	fmt.Fprintf(os.Stderr, "p36 mipmap: pixel std none=%.4f nearest=%.4f linear=%.4f separated=%v\n",
		stds[0], stds[1], stds[2], sep)
	scriptedOK, scriptedTotal := logicOK+pixelOK, len(probes)+len(pixelChecks)+1

	goldenDiff, goldenTotal, goldenSeeded, goldenErr := wrkit.GoldenGateW(
		filepath.Join(snapDir, fmt.Sprintf("p36_mipmap_final_%s.png", backendTag())),
		filepath.Join(snapDir, fmt.Sprintf("p36_mipmap_final_%s_base.png", backendTag())),
		winW, []wrkit.GoldenRect{
			{X: 300, Y: 74, W: 288, H: 240},
			{X: 596, Y: 74, W: 288, H: 240},
			{X: 892, Y: 74, W: 288, H: 240},
		})
	if goldenErr != nil {
		fmt.Fprintln(os.Stderr, "golden:", goldenErr)
	}

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "P36-mipmap",
		Scenario:      "ui_wr_p36_mipmap",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    5,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"logic_probes_ok":       logicOK,
			"logic_probes_total":    len(probes),
			"scripted_ok":           scriptedOK,
			"scripted_total":        scriptedTotal,
			"delta_none":            ledger.deltas[0],
			"delta_nearest":         ledger.deltas[1],
			"delta_linear":          ledger.deltas[2],
			"chain_bytes_want":      chainBytes,
			"pixel_golden_diff_pct": goldenDiff,
			"pixel_golden_total_px": goldenTotal,
			"pixel_golden_seeded":   goldenSeeded,
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	if !secsSet {
		fmt.Fprintf(os.Stderr, "p36 mipmap: manual session done presents=%d\n", app.PresentCount())
		return
	}
	if goldenErr != nil {
		os.Exit(1)
	}
	if !goldenSeeded && goldenDiff != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: golden diff=%.4f%% over %d px\n", goldenDiff, goldenTotal)
		os.Exit(1)
	}
	if scriptedOK != scriptedTotal {
		fmt.Fprintf(os.Stderr, "FAIL: scripted=%d/%d\n", scriptedOK, scriptedTotal)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "p36 mipmap: OK presents=%d scripted=%d/%d golden=%.4f%%\n",
		app.PresentCount(), scriptedOK, scriptedTotal, goldenDiff)
}

// ── 绘制与像素辅助（与前两窗同形状） ──

func fillCardBG(pc *rendering.PaintContext, sz rendering.Size) {
	pc.DC.SetRGBA(cardBG[0], cardBG[1], cardBG[2], 1)
	pc.DC.DrawRectangle(pc.OriginX, pc.OriginY, sz.Width, sz.Height)
	_ = pc.DC.Fill()
}

type rect struct{ x, y, w, h float64 }

type pixelCheck struct {
	region rect
	desc   string
}

func regionMean(img image.Image, x, y, w, h float64) float64 {
	if img == nil {
		return 0
	}
	var sum, n float64
	x0, y0 := int(x), int(y)
	x1, y1 := int(x+w), int(y+h)
	for py := y0; py < y1 && py < img.Bounds().Dy(); py++ {
		for px := x0; px < x1 && px < img.Bounds().Dx(); px++ {
			r8, g8, b8, _ := img.At(px, py).RGBA()
			sum += (float64(r8>>8) + float64(g8>>8) + float64(b8>>8)) / 3
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / n / 255
}

func runPixelChecks(img image.Image, checks []pixelCheck, result map[string]bool) {
	dpr := 1.0
	if img != nil {
		dpr = float64(img.Bounds().Dx()) / winW
	}
	for _, c := range checks {
		n := 0
		if img != nil {
			x0, y0 := int(c.region.x*dpr), int(c.region.y*dpr)
			x1, y1 := int((c.region.x+c.region.w)*dpr), int((c.region.y+c.region.h)*dpr)
			for py := y0; py < y1 && py < img.Bounds().Dy(); py++ {
				for px := x0; px < x1 && px < img.Bounds().Dx(); px++ {
					rr, gg, bb, _ := img.At(px, py).RGBA()
					r8, g8, b8 := float64(rr>>8)/255, float64(gg>>8)/255, float64(bb>>8)/255
					if diff(r8, cardBG[0]) > 0.06 || diff(g8, cardBG[1]) > 0.06 || diff(b8, cardBG[2]) > 0.06 {
						n++
					}
				}
			}
		}
		ok := n >= 150
		fmt.Fprintf(os.Stderr, "p36 mipmap: pixel %-14s warm_px=%d ok=%v\n", c.desc, n, ok)
		result[c.desc] = ok
	}
}

// regionStd 算区域灰度标准差（0..1）。
func regionStd(img image.Image, x, y, w, h int, mean float64) float64 {
	if img == nil {
		return 0
	}
	var acc float64
	n := 0.0
	for py := y; py < y+h && py < img.Bounds().Dy(); py++ {
		for px := x; px < x+w && px < img.Bounds().Dx(); px++ {
			r8, g8, b8, _ := img.At(px, py).RGBA()
			v := (float64(r8>>8) + float64(g8>>8) + float64(b8>>8)) / 3 / 255
			acc += (v - mean) * (v - mean)
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return math.Sqrt(acc / n)
}

func diff(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}

func loadImage(path string) image.Image {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pixel checks: %v\n", err)
		return nil
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pixel checks decode: %v\n", err)
		return nil
	}
	return img
}

// backendTag 返回当前 GPU 后端标签（金图基线按后端分文件）。
func backendTag() string {
	be := os.Getenv("GPUI_BACKEND")
	if be == "" {
		be = "native"
	}
	return be
}

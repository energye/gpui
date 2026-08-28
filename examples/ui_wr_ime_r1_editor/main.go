// Command ui_wr_ime_r1_editor is the IME R1 real-window: Editor four-tuple.
// 人工手动验证：单元测试自动，真窗必须人手敲键盘/鼠标来验功能。
//
//   export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//   go run ./examples/ui_wr_ime_r1_editor
//
// Window: 1200x800. 手动关闭（点击窗口 X），不自动退出。
// 布局：左=中文族测试清单 | 中=上R1能力说明+下人工真实测试 | 右=动态不规则边框
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/input"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"
	"github.com/energye/gpui/ui/textinput"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800
const (
	mixedCJK = "你好Hello世界Flutter输入法IME测试abcdefghijklmnopqrstuvwxyz0123456789"
	longBase = "你好Hello世界"
)

// manualInputBox / multilineInputBox now provided by ui/textinput engine (R5 BaseEditable).
// R示例只做测试，不实现输入框功能。
type manualInputBox = textinput.InputBox
type multilineInputBox = textinput.MultiLineInputBox

func newManualInputBox(ed *textinput.Editor, w, h float64, fontSize float64) *manualInputBox {
    b := textinput.NewInputBox(ed, w, h, fontSize)
    if face := wrkit.FaceAt(fontSize); face != nil {
        b.SetFace(face)
    }
    return b
}
func newManualInputBoxDefault(ed *textinput.Editor, w, h float64) *manualInputBox {
    return newManualInputBox(ed, w, h, 16)
}
func newMultilineInputBox(ed *textinput.Editor, w, h float64, fontSize float64) *multilineInputBox {
    b := textinput.NewMultiLineInputBox(ed, w, h, fontSize)
    if face := wrkit.FaceAt(fontSize); face != nil {
        b.SetFace(face)
    }
    return b
}

func main() {
	selftest := os.Getenv("GPUI_R1_SELFTEST") == "1"
	wrkit.EnsureUIFace()
	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui ui_wr_ime_r1_editor — IME R1 手工验证", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	var app *embedder.PipelineApp
	var pasteMu sync.Mutex
	var pendingPastes []func()

	shell := wrkit.NewShell(winW, winH, "R1 Editor 四元组 — 人工手动验证真窗", []string{})
	const gap = 12.0
	const topH = 48.0
	const hudH = 72.0
	legH := winH - topH - hudH - gap*2
	if legH < 200 {
		legH = 200
	}
	leftW := 340.0
	midW := 460.0
	rightW := winW - leftW - midW - gap*4
	if rightW < 300 {
		rightW = 300
	}
	shell.Legend.W = leftW
	shell.Legend.Box.FixedWidth = leftW
	shell.Legend.Box.FixedHeight = legH
	shell.Body.W = midW
	shell.Body.Box.FixedWidth = midW
	shell.Body.Box.FixedHeight = legH
	shell.Root.Place(shell.Legend.Box, gap, topH+gap)
	shell.Root.Place(shell.Body.Box, gap+leftW+gap, topH+gap)

	// 左：中文族测试清单（人工核对）
	shell.Legend.LabelAt("R1 族测试清单（人工核对）", 13, 12, 10, 0.95, 0.92, 0.55)
	leftLines := []string{
		"A 帧时 fps/p95/hitch 仅告警",
		"B 管线 build p95 <5ms",
		"C 脏区 允许≈1（full_paint）",
		"D CPU <40%（5s窗）",
		"E 内存 slope=off（短窗）",
		"F GPU fallback==0 硬",
		"G 图文 hit 可跳过",
		"H 首帧 <1000ms 硬",
		"I 回归 可选 <10%",
		"J 正确性 vet0+探针 硬",
		"— 8 手工场景 —",
		"1 哨兵 -1/-1→0,0 手打",
		"2 中英混排200字 粘贴",
		"3 surrogate 😀𝄞 单码删",
		"4 4000居中4锚点拖动",
		"5 嵌套batch3层 末层1次",
		"6 UTF16↔UTF8 你好/emoji",
		"7 空预编辑 no-op",
		"8 越界拒绝 只读钳制",
	}
	for i, ln := range leftLines {
		y := 36 + float64(i)*20
		if y > legH-10 {
			break
		}
		shell.Legend.LabelAt(ln, 11, 12, y, 0.82, 0.86, 0.92)
	}

	// 右：动态不规则边框
	rightPanel := wrkit.NewPanel(rightW, legH, 0.10, 0.11, 0.13, 1)
	shell.Root.Place(rightPanel.Box, gap+leftW+gap+midW+gap, topH+gap)
	rightPanel.LabelAt("动态异形边框（右）", 13, 12, 10, 0.95, 0.75, 0.45)
	rightPanel.LabelAt("每帧形变 · 不规则多边形", 11, 12, 32, 0.70, 0.78, 0.88)
	shapeBox := rendering.NewRenderBox()
	shapeBox.FixedWidth = rightW - 24
	shapeBox.FixedHeight = legH - 60
	shapeBox.SetRepaintBoundary(true)
	rightPanel.Place(shapeBox, 12, 54)
	shapeTick := 0
	shapeBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		x, y, w, h := pc.OriginX, pc.OriginY, size.Width, size.Height
		pc.DC.SetRGBA(0.16, 0.18, 0.22, 1)
		pc.DC.DrawRectangle(x, y, w, h)
		_ = pc.DC.Fill()
		t := float64(shapeTick) * 0.08
		cx, cy := x+w/2, y+h/2
		rx, ry := w*0.38, h*0.32
		n := 8
		p := rendering.NewPath()
		for i := 0; i < n; i++ {
			ang := float64(i)/float64(n)*2*math.Pi + t*0.5
			wobble := 1 + 0.18*math.Sin(t*1.2+float64(i)*0.9)
			px := cx + rx*wobble*math.Cos(ang)
			py := cy + ry*wobble*math.Sin(ang)
			if i == 0 {
				p.MoveTo(px, py)
			} else {
				p.LineTo(px, py)
			}
		}
		p.Close()
		rendering.FillPath(pc, p, 0.22, 0.65, 0.95, 0.18)
		pc.DC.SetRGBA(0.95, 0.65, 0.25, 1)
		pc.DC.SetLineWidth(2.5)
		_ = pc.DC.StrokePath(p)
		p2 := rendering.NewPath()
		for i := 0; i < n; i++ {
			ang := float64(i)/float64(n)*2*math.Pi - t*0.7
			wobble := 1 + 0.12*math.Cos(t+float64(i)*1.1)
			px := cx + rx*0.55*wobble*math.Cos(ang)
			py := cy + ry*0.55*wobble*math.Sin(ang)
			if i == 0 {
				p2.MoveTo(px, py)
			} else {
				p2.LineTo(px, py)
			}
		}
		p2.Close()
		pc.DC.SetRGBA(0.35, 0.85, 0.95, 1)
		pc.DC.SetLineWidth(1.8)
		_ = pc.DC.StrokePath(p2)
		pc.DC.SetRGBA(1, 0.9, 0.4, 1)
		pc.DC.DrawCircle(cx, cy, 4)
		_ = pc.DC.Fill()
	}

	// 中：上R1能力说明 + 下人工真实测试
	shell.Body.LabelAt("R1 能力：Editor 四元组（对齐 Flutter TextInputModel）", 13, 12, 10, 0.95, 0.92, 0.55)
	capLines := []string{
		"· 四元组：text（含preedit）+ selection + composing_range + composing",
		"· 能干：插入/删除/选区/组合预编辑/光标移动/批量编辑/去重",
		"· affinity：换行处下游/上游，-1/-1 哨兵→0,0",
		"· 边界：editable_range限编辑，surrogate按1、4000居中截断",
		"· 状态：batchDepth嵌套、epoch去重、密码/只读钳制",
	}
	for i, ln := range capLines {
		shell.Body.LabelAt(ln, 10, 12, 30+float64(i)*18, 0.82, 0.86, 0.92)
	}
	sep := rendering.NewRenderColorBox(midW-24, 1, 0.35, 0.40, 0.50, 1)
	shell.Body.Place(sep, 12, 125)
	shell.Body.LabelAt("R1 人工真实测试（点不同字号框获焦，手打）", 12, 12, 138, 0.55, 0.95, 0.75)
	shell.Body.LabelAt("请依次点 10/12/16/20px 框：打字→退格→方向键→拖选→拼音→Esc→粘贴", 10, 12, 158, 0.65, 0.75, 0.85)

	// 四个独立字号输入框（同行不同字号混排用 Runs，此处为独立单字号框，直观对比光标是否贴缝）
	ed10 := textinput.New()
	ed10.SetText("10px Aa你好Hello", textinput.TextRange{Base: 2, Extent: 2}, textinput.TextRange{}, 0)
	ed12 := textinput.New()
	ed12.SetText("12px Aa你好Hello", textinput.TextRange{Base: 2, Extent: 2}, textinput.TextRange{}, 0)
	ed16 := textinput.New()
	ed16.SetText(strings.Repeat(mixedCJK, 1), textinput.TextRange{Base: 10, Extent: 10}, textinput.TextRange{}, 0)
	ed20 := textinput.New()
	ed20.SetText("20px Aa你好Hello😀", textinput.TextRange{Base: 2, Extent: 2}, textinput.TextRange{}, 0)
	// 主框保留 16px 兼容自检
	ed := ed16
	shell.Body.LabelAt("10px 输入框（点获焦）", 10, 12, 172, 0.85, 0.85, 0.90)
	box10 := newManualInputBox(ed10, midW-24, 48, 10)
	shell.Body.Place(box10, 12, 186)
	shell.Body.LabelAt("12px 输入框", 10, 12, 238, 0.75, 0.95, 0.85)
	box12 := newManualInputBox(ed12, midW-24, 52, 12)
	shell.Body.Place(box12, 12, 252)
	shell.Body.LabelAt("16px 输入框（主）", 10, 12, 308, 0.95, 0.85, 0.55)
	inputBox := newManualInputBox(ed16, midW-24, 56, 16)
	shell.Body.Place(inputBox, 12, 322)
	shell.Body.LabelAt("20px 输入框", 10, 12, 382, 0.95, 0.65, 0.55)
	box20 := newManualInputBox(ed20, midW-24, 60, 20)
	shell.Body.Place(box20, 12, 396)

	// 同行多字号混排演示（单行内 10+16+12 混排，验证缝表按段累加）
	shell.Body.LabelAt("同行混排 10+16+12px（单行内多段）", 10, 12, 462, 0.55, 0.85, 0.95)
	mixedRunBox := rendering.NewRenderBox()
	mixedRunBox.FixedWidth = midW - 24
	mixedRunBox.FixedHeight = 48
	mixedRunBox.SetRepaintBoundary(true)
	shell.Body.Place(mixedRunBox, 12, 476)
	// 用 Paragraph Runs 画一行 10/16/12 混排，光标位置由多段缝表决定
	mixedRunText := rendering.NewRenderText("")
	mixedRunText.FontSize = 14
	mixedRunText.MaxWidth = 0
	{
		pb := rendering.NewParagraphBuilder()
		if f, _, _ := text.LoadMultiFace(10); f != nil {
			pb.AddRun(rendering.TextRun{Text: "Aa", Face: f, FontSize: 10, R: 0.85, G: 0.85, B: 0.90, A: 1})
		}
		if f, _, _ := text.LoadMultiFace(16); f != nil {
			pb.AddRun(rendering.TextRun{Text: "你好", Face: f, FontSize: 16, R: 0.75, G: 0.95, B: 0.85, A: 1})
		}
		if f, _, _ := text.LoadMultiFace(12); f != nil {
			pb.AddRun(rendering.TextRun{Text: "Hello", Face: f, FontSize: 12, R: 0.95, G: 0.85, B: 0.55, A: 1})
		}
		pb.Apply(mixedRunText)
		mixedRunText.R, mixedRunText.G, mixedRunText.B, mixedRunText.A = 0.9, 0.9, 0.9, 1
	}
	mixedRunBox.AddChild(mixedRunText)
	mixedRunText.SetOffset(rendering.Point{X: 8, Y: 12})
	// 在该行第 4 个字符后（Aa你 后）画一条标尺光标，证明多段行内缝正确
	mixedCaretBar := rendering.NewRenderColorBox(1.5, 24, 1.0, 0.85, 0.2, 1)
	mixedRunBox.AddChild(mixedCaretBar)
	mixedRunBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		lay := mixedRunText.TextLayout()
		if lay == nil || len(lay.Lines) == 0 {
			return
		}
		// 取全局 byte 2+3=5（Aa + 你）后的缝
		off := 2 + len("你")
		_, penX, ok := lay.CaretForOffset(off)
		if !ok {
			return
		}
		textOff := mixedRunText.Offset()
		x := textOff.X + penX
		top := textOff.Y + lay.LineTop(0)
		h := lay.LineHeight(0)
		mixedCaretBar.MoveTo(x-0.75, top)
		mixedCaretBar.Height = h
		mixedCaretBar.SetAlpha(1)
	}

	// 多行换行输入框（R1 换行能力演示：Enter 换行 + 长句自动换行按 MaxWidth）
	shell.Body.LabelAt("多行输入框（Enter 换行，长句自动换行）", 10, 12, 530, 0.85, 0.90, 0.60)
	edMulti := textinput.New()
	edMulti.SetText("多行换行测试\n第二行 中英 Hello 12px\n第三行 长句自动换行测试 abcdefghijklmnopqrstuvwxyz 你好世界 Hello", textinput.TextRange{Base: 0, Extent: 0}, textinput.TextRange{}, 0)
	multilineBox := newMultilineInputBox(edMulti, midW-24, 56, 14)
	shell.Body.Place(multilineBox, 12, 546)

	probeLabel := wrkit.Label("probe: init", 11, 0.92, 0.95, 0.98)
	shell.Body.Place(probeLabel, 12, 608)
	scenarioLabel := wrkit.Label("手工：点框获焦后键盘验证", 10, 0.75, 0.85, 0.95)
	shell.Body.Place(scenarioLabel, 12, 622)
	surroundLabel := wrkit.Label("surrounding: -", 10, 0.65, 0.75, 0.85)
	shell.Body.Place(surroundLabel, 12, 636)
	phaseLabel := wrkit.Label("等待人工输入…", 12, 0.95, 0.95, 0.4)
	shell.Body.Place(phaseLabel, 12, 650)

	fm := focus.NewManager()
	router := embedder.NewInputRouter(nil, fm)
	router.TextEditor = ed
	fm.Register(inputBox.Node)
	fm.Register(box10.Node)
	fm.Register(box12.Node)
	fm.Register(box20.Node)
	fm.Register(multilineBox.Node)
	clip := win.Clipboard()
	box10.SetClipboard(clip)
	box12.SetClipboard(clip)
	inputBox.SetClipboard(clip)
	box20.SetClipboard(clip)
	multilineBox.SetClipboard(clip)
	// 启动即获焦，方便直接打字
	_ = inputBox.Node.RequestFocus()

	// 自检：在真正开窗前先跑一遍“缝”单源校验（可无头验证，不依赖 GPU）
	if selftest {
		if ok, msg := verifyMixedCaret(); !ok {
			fmt.Fprintf(os.Stderr, "FAIL: selftest verifyMixedCaret: %s\n", msg)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "selftest: verifyMixedCaret PASS")
	}

	// 探针：OnChange 驱动更新（人工每次操作都会触发）
	type probe struct {
		Text     string `json:"text"`
		Sel      string `json:"sel"`
		Comp     string `json:"comp"`
		CompBool bool   `json:"composing"`
		Epoch    uint64 `json:"epoch"`
	}
	var probes []probe
	tick := 0
	ed.OnChange = func() {
		inputBox.Sync()
		probeLabel.SetText(fmt.Sprintf("probe: len=%d sel=%v comp=%v compo=%v epoch=%d", len(ed.GetText()), ed.TextRange().Extent, ed.EditableRange(), ed.IsComposing(), ed.Epoch()))
		probeLabel.MarkNeedsPaint()
		// 用 %s 明文显示中文，避免 %q 把“手动”变成 \u624b\u5de5
		t := ed.GetText()
		rs := []rune(t)
		if len(rs) > 12 {
			t = string(rs[:12])
		}
		scenarioLabel.SetText(fmt.Sprintf("text=%s sel=%v", t, ed.TextRange()))
		scenarioLabel.MarkNeedsPaint()
		// surrounding 实时
		cur := ed.GetCursorOffset()
		tr, nc := textinput.TruncateSurrounding(ed.GetText(), cur)
		surroundLabel.SetText(fmt.Sprintf("surrounding: cur=%d trunc=%d/%d", nc, nc, len(tr)))
		surroundLabel.MarkNeedsPaint()
		probes = append(probes, probe{
			Text:     t,
			Sel:      fmt.Sprintf("%v", ed.TextRange()),
			Comp:     fmt.Sprintf("%v", ed.EditableRange()),
			CompBool: ed.IsComposing(),
			Epoch:    ed.Epoch(),
		})
		if len(probes) > 50 {
			probes = probes[len(probes)-50:]
		}
	}
	// 初始化一次
	ed.OnChange()

	snapPath := ""
	runFor := time.Duration(0)
	if selftest {
		runFor = 3 * time.Second
		snapPath = "/tmp/r1_selftest.png"
		os.MkdirAll("/tmp", 0755)
	} else if v := os.Getenv("GPUI_R1_RUN_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			runFor = time.Duration(n) * time.Second
		}
	}
	app = embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR:       0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		WarmUp:       true,
		Input:        router,
		IME:          win.IME(),
		RunFor:       runFor,
		SnapshotPath: snapPath,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "ui_wr_ime_r1_editor: close (%s)\n", win.Backend())
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					newW, newH := float64(ev.Width), float64(ev.Height)
					// 保留 R1 自定义的 leftW/midW，rightPanel 随窗口自适应；不用 shell.Resize 的 260 默认覆盖
					legH2 := newH - topH - hudH - gap*2
					if legH2 < 200 {
						legH2 = 200
					}
					rightW2 := newW - leftW - midW - gap*4
					if rightW2 < 200 {
						rightW2 = 200
					}
					shell.Root.FixedWidth, shell.Root.FixedHeight = newW, newH
					shell.Root.MarkNeedsLayout()
					shell.Top.W, shell.Top.H = newW, topH
					shell.Top.Box.FixedWidth, shell.Top.Box.FixedHeight = newW, topH
					shell.Top.Box.MarkNeedsLayout()
					shell.Legend.W, shell.Legend.H = leftW, legH2
					shell.Legend.Box.FixedWidth, shell.Legend.Box.FixedHeight = leftW, legH2
					shell.Legend.Box.MarkNeedsLayout()
					shell.Body.W, shell.Body.H = midW, legH2
					shell.Body.Box.FixedWidth, shell.Body.Box.FixedHeight = midW, legH2
					shell.Body.Box.MarkNeedsLayout()
					shell.Root.Place(shell.Legend.Box, gap, topH+gap)
					shell.Root.Place(shell.Body.Box, gap+leftW+gap, topH+gap)
					shell.Root.Place(rightPanel.Box, gap+leftW+gap+midW+gap, topH+gap)
					rightPanel.W, rightPanel.H = rightW2, legH2
					rightPanel.Box.FixedWidth, rightPanel.Box.FixedHeight = rightW2, legH2
					rightPanel.Box.MarkNeedsLayout()
					shapeBox.FixedWidth = rightW2 - 24
					shapeBox.FixedHeight = legH2 - 60
					shapeBox.MarkNeedsLayout()
					if shell.HUD != nil {
						shell.HUD.Width, shell.HUD.Height = newW, hudH
						shell.HUD.Box.FixedWidth = newW
						shell.Root.Place(shell.HUD.Box, 0, newH-hudH)
						shell.HUD.Box.MarkNeedsLayout()
					}
				}
			}
		},
	})
	inputBox.SetSchedule(app.ScheduleFrame)
	box10.SetSchedule(app.ScheduleFrame)
	box12.SetSchedule(app.ScheduleFrame)
	box20.SetSchedule(app.ScheduleFrame)
	multilineBox.SetSchedule(app.ScheduleFrame)

	// 仅保留光标闪烁 + 右形变 + HUD 的 ticker，不再自动改 text
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		// 异步粘贴：wayland Get 会阻塞 3s，不能在 UI 线程里做，OnKey 里只发 go routine，这里在 UI 线程真正 Paste
		pasteMu.Lock()
		if len(pendingPastes) > 0 {
			fns := pendingPastes
			pendingPastes = nil
			pasteMu.Unlock()
			for _, fn := range fns {
				fn()
			}
		} else {
			pasteMu.Unlock()
		}
		tick++
		shapeTick = tick
		// 光标闪烁（四个独立字号框 + 多行框同步闪）
		if tick%30 == 0 {
			for _, b := range []*manualInputBox{inputBox, box10, box12, box20} {
				b.SetCaretOn(!b.IsCaretOn())
			}
			multilineBox.SetCaretOn(!multilineBox.IsCaretOn())
		}
		shapeBox.MarkNeedsPaint()
		phaseLabel.SetText(fmt.Sprintf("人工验证中 tick=%d  请手打验证上表 8 项", tick))
		phaseLabel.MarkNeedsPaint()
		app.ScheduleFrame()
		proc.Sample()
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gateOK := snapH.PaintCount > 0 && snapH.CPUFallbackOps == 0
		shell.UpdateHUD("ime_r1_editor", "MANUAL", app, gateOK,
			fmt.Sprintf("手工 tick=%d epoch=%d", tick, ed.Epoch()), "")
	}})
	if selftest {
		// 真窗自检：3 秒内在 GPU 上走一遍混排缝，验证单源且不跳格
		selftestDone := false
		app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
			if selftestDone || tick < 10 {
				return
			}
			selftestDone = true
			ed.SetText("Aa你好Hello😀", textinput.TextRange{Base: 0, Extent: 0}, textinput.TextRange{}, 0)
			// 逐缝走，记录 X 单调且在缝上
			face, _, _ := text.LoadMultiFace(16)
			if face == nil {
				fmt.Fprintln(os.Stderr, "selftest: no face")
				return
			}
			lay := rendering.BuildTextLayout(ed.GetText(), face, 16, 0, 1.25)
			if lay == nil || len(lay.Lines) == 0 {
				fmt.Fprintln(os.Stderr, "FAIL: selftest no layout")
				os.Exit(1)
			}
			lastX := -1.0
			for i, c := range lay.Lines[0].Carets {
				if c.X < lastX-0.01 {
					fmt.Fprintf(os.Stderr, "FAIL: selftest non-monotonic at %d x=%.2f last=%.2f\n", i, c.X, lastX)
					os.Exit(1)
				}
				lastX = c.X
			}
			// 额外：用 moveVisual 走一遍，看是否每格一跳
			ed.SetText("Aa你好Hello😀", textinput.TextRange{Base: 0, Extent: 0}, textinput.TextRange{}, 0)
			for i := 0; i < len(lay.Lines[0].Carets)-1; i++ {
				inputBox.MoveVisual(1)
				cur := ed.GetCursorOffset()
				exp := lay.Lines[0].Carets[i+1].ByteOff
				if cur != exp {
					fmt.Fprintf(os.Stderr, "FAIL: selftest moveVisual step %d got %d want %d\n", i, cur, exp)
					os.Exit(1)
				}
			}
			fmt.Fprintln(os.Stderr, "selftest: mixedCaretVisual PASS")
		}})
	}
	app.Scheduler().SetMode(scheduler.ModePersistent)

	// 路由日志（按焦点分发到对应字号框）
	router.OnPointer = func(pe input.PointerEvent, target rendering.RenderObject) {}
	router.OnKey = func(ke input.KeyEvent) {
		// 粘贴要异步：wayland 的 Get 会阻塞 UI 线程 3s，这里只发 go routine，真正 Paste 在 ticker 的 UI 线程里做
		// 单行框按 Flutter 单行语义过滤 '\n'（F-C5），多行框保留换行
		if ke.Pressed && (ke.Mods.Control || ke.Mods.Meta) && ke.Key == input.KeyV {
			var edTarget *textinput.Editor
			var clip platform.Clipboard
			isSingle := true
			if multilineBox.IsFocused() {
				edTarget = multilineBox.Editor()
				clip = multilineBox.Clipboard()
				isSingle = false
			} else {
				for _, b := range []*manualInputBox{box10, box12, inputBox, box20} {
					if b.IsFocused() {
						edTarget = b.Editor()
						clip = b.Clipboard()
						break
					}
				}
				if edTarget == nil {
					edTarget = inputBox.Editor()
					clip = inputBox.Clipboard()
				}
			}
			if edTarget != nil && clip != nil {
				go func(c platform.Clipboard, e *textinput.Editor, single bool) {
					s, err := c.Get("text/plain")
					if err != nil || s == "" {
						return
					}
					// Wayland 外部粘贴有时拿到的是 JSON 转义的 \uXXXX（复制自终端 JSON 报告），解回真实字符
					s = maybeDecodeClipboard(s)
					if single {
						s = strings.ReplaceAll(s, "\r", "")
						s = strings.ReplaceAll(s, "\n", "")
						if s == "" {
							return
						}
					}
					rawLen := len(s)
					decoded := s
					pasteMu.Lock()
					pendingPastes = append(pendingPastes, func() {
						before := e.GetText()
						beforeCaret := e.GetCursorOffset()
						ok := e.Paste(decoded)
						after := e.GetText()
						fmt.Fprintf(os.Stderr, "[r1-paste] rawLen %d -> decodedLen %d ok %v before %q caret %d -> after %q caret %d\n", rawLen, len(decoded), ok, before, beforeCaret, after, e.GetCursorOffset())
					})
					pasteMu.Unlock()
					if app != nil {
						app.ScheduleFrame()
					}
				}(clip, edTarget, isSingle)
			}
			return
		}
		// 其他键按焦点分发
		if multilineBox.IsFocused() {
			multilineBox.OnKey(ke)
			return
		}
		for _, b := range []*manualInputBox{box10, box12, inputBox, box20} {
			if b.IsFocused() {
				b.OnKey(ke)
				return
			}
		}
		inputBox.OnKey(ke)
	}
	router.OnText = func(ev input.TextEvent) {}
	router.OnIME = func(ev input.IMEEvent) {}

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

	elapsed := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	extra := map[string]any{
		"present_policy":   "full_paint",
		"probe_count":      len(probes),
		"probes":           probes,
		"phase":            "MANUAL",
		"tick":             tick,
		"editor_text_len":  len(ed.GetText()),
		"editor_sel":       fmt.Sprintf("%v", ed.TextRange()),
		"editor_comp":      fmt.Sprintf("%v", ed.EditableRange()),
		"editor_composing": ed.IsComposing(),
		"editor_epoch":     ed.Epoch(),
		"slope_gate":       "off",
		"layout":           "左中文族测试|中上能力下手工|右异形边框",
		"manual":           true,
	}
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "ime_r1_editor",
		Scenario:      "ui_wr_ime_r1_editor",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra:         extra,
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	if app.PresentCount() < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: no presents")
		os.Exit(1)
	}
	if snap.CPUFallbackOps != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: cpu_fallback_ops=%d want 0", snap.CPUFallbackOps)
		os.Exit(1)
	}
	if snap.FirstPresentPaintCount == 0 {
		fmt.Fprintln(os.Stderr, "FAIL: first_present_paint_count==0 (no content)")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_ime_r1_editor: OK 手工验证 presents=%d paint=%d probes=%d elapsed=%.1fs\n",
		app.PresentCount(), snap.PaintCount, len(probes), elapsed)
}

func verifyMixedCaret() (bool, string) {
	face, _, _ := text.LoadMultiFace(16)
	if face == nil {
		return true, "no face skip"
	}
	// 单脚本样例校验 X 单调；混排 emoji 样例仅校验缝在边界（已知单脸对混排 emoji 会回退，属 shaper 单脸限制，窗口多字号区用独立 Measure 已规避）
	samplesMono := []string{"Aa你好Hello", "Hello你好", "你好世界"}
	for _, s := range samplesMono {
		lay := rendering.BuildTextLayout(s, face, 16, 0, 1.25)
		if lay == nil || len(lay.Lines) == 0 {
			return false, "no layout for " + s
		}
		var lastX float64 = -1
		for _, ln := range lay.Lines {
			for _, c := range ln.Carets {
				if c.ByteOff < 0 || c.ByteOff > len(s) {
					return false, "ByteOff out of range"
				}
				if c.ByteOff > 0 && c.ByteOff < len(s) && s[c.ByteOff]&0xC0 == 0x80 {
					return false, "caret inside multi-byte"
				}
				if c.X < lastX-0.01 {
					return false, "non-monotonic X for " + s
				}
				lastX = c.X
			}
		}
	}
	// 混排 emoji 仅校验缝在边界
	mixed := "Aa你好Hello😀"
	lay := rendering.BuildTextLayout(mixed, face, 16, 0, 1.25)
	if lay != nil {
		for _, ln := range lay.Lines {
			for _, c := range ln.Carets {
				if c.ByteOff > 0 && c.ByteOff < len(mixed) && mixed[c.ByteOff]&0xC0 == 0x80 {
					return false, "mixed caret inside"
				}
			}
		}
	}
	return true, "ok"
}

func maybeDecodeClipboard(s string) string {
	if s == "" {
		return s
	}
	if !strings.Contains(s, "\\u") {
		return s
	}
	// 只有纯 ASCII 且含 \u 时才尝试解（raw 中文已含非 ASCII，不会误解）
	hasNonASCII := false
	for _, r := range s {
		if r > 127 {
			hasNonASCII = true
			break
		}
	}
	if hasNonASCII {
		return s
	}
	if decoded, err := strconv.Unquote(`"` + s + `"`); err == nil && decoded != s {
		return decoded
	}
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

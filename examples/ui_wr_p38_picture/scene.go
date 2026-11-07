package main

import (
	"fmt"
	"image/color"
	"sync"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/recording"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
)

// Cards: 2x2 grid (1460×800, same geometry as the other P3.8 windows).
// Cards 1-2 replay recordings through CanvasBackend each paint (the
// DrawPicture command path); card 3 replays a self-referencing picture and
// shows the guard; card 4 renders mixed-script text whose cluster indices
// travel the scene encoding (K5).
const (
	cardW, cardH = 560.0, 300.0
	innerW       = 528.0
	innerH       = 252.0
	innerX       = (cardW - innerW) / 2
	innerY       = 40.0
)

var cardAt = [4][2]float64{
	{12, 8}, {592, 8},
	{12, 316}, {592, 316},
}

var (
	colCardBG = render.RGBA{R: 0.10, G: 0.11, B: 0.13, A: 1}
	colInkA   = render.RGBA{R: 0.89, G: 0.29, B: 0.29, A: 1} // red square
	colInkB   = render.RGBA{R: 0.24, G: 0.51, B: 0.89, A: 1} // blue square
	colInkC   = render.RGBA{R: 0.24, G: 0.81, B: 0.48, A: 1} // green square
	colTextFG = render.RGBA{R: 0.9, G: 0.92, B: 0.95, A: 1}
)

type baked struct {
	inner       *recording.Recorder
	mid         *recording.Recorder
	pic         *recording.Recorder
	self        *recording.Recorder
	selfRefused bool
	face        text.Face
	err         string
	closed      bool
}

var (
	bakeOnce sync.Once
	bakedRes baked
)

var cardTitles = [4]string{
	"1 折叠复用：一 picture 三处摆位（引用命令）",
	"2 嵌套三层 picture（绿芯/蓝环/红环）",
	"3 环拒绝牌：自引用 → ErrPictureCycle",
	"4 K5 穿链：中英混排 Cluster 编码往返",
}

func ensureBaked() *baked {
	bakeOnce.Do(func() {
		b := &bakedRes
		// 内层：绿色小方块。
		b.inner = recording.NewRecorder(60, 60)
		b.inner.SetRGBA(colInkC.R, colInkC.G, colInkC.B, colInkC.A)
		b.inner.DrawRectangle(14, 14, 32, 32)
		_ = b.inner.Fill()
		// 中层：蓝框 + 画内层 picture（嵌套一层）。
		b.mid = recording.NewRecorder(60, 60)
		b.mid.SetRGBA(colInkB.R, colInkB.G, colInkB.B, colInkB.A)
		b.mid.DrawRectangle(6, 6, 48, 48)
		_ = b.mid.Fill()
		b.mid.DrawPicture(b.inner)
		// 外层：红框 + 画中层 picture（嵌套两层到内层，共三层——三个色环）。
		b.pic = recording.NewRecorder(60, 60)
		b.pic.SetRGBA(colInkA.R, colInkA.G, colInkA.B, colInkA.A)
		b.pic.DrawRectangle(0, 0, 60, 60)
		_ = b.pic.Fill()
		b.pic.DrawPicture(b.mid)
		// 卡 3：自引用 picture（回放期 ErrPictureCycle 护栏）。护栏结果在
		// bake 期用离屏 Context 预演一次——窗内 DC 可能是任意 Canvas 实现
		// （录制/代理层），拒绝计数口只在裸 *Context 上可用，静态牌面不
		// 依赖运行期 DC 类型。
		b.self = recording.NewRecorder(100, 100)
		b.self.DrawPicture(b.self)
		probeDC := render.NewContext(100, 100, render.WithCPUOnly())
		before := probeDC.PictureRefusals()
		_ = b.self.FinishRecording().Playback(recording.NewCanvasBackend(probeDC))
		b.selfRefused = probeDC.PictureRefusals() > before
		// 卡 4：混排文本 face。
		src, err := text.NewFontSourceFromFile("render/text/testdata/wqy-microhei.ttf")
		if err != nil {
			b.err = "wqy-microhei: " + err.Error()
			return
		}
		b.face = src.Face(24)
	})
	return &bakedRes
}

func closeBaked() {
	b := &bakedRes
	if b.closed {
		return
	}
	b.closed = true
}

func cardBG(dc render.Canvas, ox, oy, w, h float64) {
	dc.SetRGBA(colCardBG.R, colCardBG.G, colCardBG.B, colCardBG.A)
	dc.DrawRectangle(ox, oy, w, h)
	_ = dc.Fill()
}

// replayInto replays a recording onto the card canvas through the standard
// backend path (Begin/End bracket the state).
func replayInto(r *recording.Recorder, dc render.Canvas) error {
	return r.FinishRecording().Playback(recording.NewCanvasBackend(dc))
}

func buildPictureScene(shell *wrkit.ShellChrome) {
	card := func(i int, title string) {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cardW, cardH
		box.OnPaint = func(pc *rendering.PaintContext, sz rendering.Size) {
			if pc == nil || pc.DC == nil {
				return
			}
			dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
			cardBG(dc, ox, oy, sz.Width, sz.Height)
			b := ensureBaked()
			if b.err != "" {
				dc.SetRGBA(0.35, 0.06, 0.08, 1)
				dc.DrawRectangle(ox+innerX, oy+innerY, innerW, innerH)
				_ = dc.Fill()
				return
			}
			switch i {
			case 0:
				// 一份 picture 三处摆位：每处都是引用命令回放，同内容。
				for k, at := range [][2]float64{{20, 40}, {220, 40}, {120, 140}} {
					dc.Push()
					dc.Translate(ox+innerX+at[0], oy+innerY+at[1])
					if k == 1 {
						dc.Scale(1.5, 1.5) // 摆位间 CTM 组合
					}
					_ = replayInto(b.pic, dc)
					dc.Pop()
				}
			case 1:
				// 三层嵌套：外(内(绿)) — DrawPicture 链。
				dc.Push()
				dc.Translate(ox+innerX+40, oy+innerY+60)
				_ = replayInto(b.pic, dc)
				dc.Pop()
			case 2:
				// 自引用拒绝牌：护栏结果 bake 期已验证（离屏 Context 计数），
				// 这里画静态牌面——不依赖运行期 DC 的具体类型。
				dc.SetRGBA(colTextFG.R, colTextFG.G, colTextFG.B, colTextFG.A)
				dc.DrawString("self-reference replay →", ox+innerX+20, oy+innerY+60)
				if b.selfRefused {
					dc.SetRGBA(colInkC.R, colInkC.G, colInkC.B, colInkC.A)
					dc.DrawString("REFUSED: ErrPictureCycle (fail-closed, counted)",
						ox+innerX+20, oy+innerY+100)
				} else {
					dc.SetRGBA(0.89, 0.29, 0.29, 1)
					dc.DrawString("NOT refused (BAD)", ox+innerX+20, oy+innerY+100)
				}
			case 3:
				// 中英混排：cluster 穿链由 logic 探针钉值，这里渲染显示。
				if b.face != nil {
					dc.SetFont(b.face)
					dc.SetRGBA(colTextFG.R, colTextFG.G, colTextFG.B, colTextFG.A)
					dc.DrawString("中AB文 ok", ox+innerX+20, oy+innerY+80)
				}
			}
		}
		shell.Body.Place(box, cardAt[i][0], cardAt[i][1])
		shell.Body.Place(wrkit.Label(title, 12, 0.80, 0.86, 0.94),
			cardAt[i][0]+12, cardAt[i][1]+10)
	}

	for i := range cardAt {
		card(i, cardTitles[i])
	}
	_ = fmt.Sprintf
	_ = color.RGBA{}
}

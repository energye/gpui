package merge

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// CompositeOptions controls side-by-side standard vs actual panels.
type CompositeOptions struct {
	Title          string
	Description    string
	LeftLabel      string
	RightLabel     string
	MinPanelWidth  int
	MinPanelHeight int
	FontPath       string
	FontSize       float64
}

// ComposeSideBySide builds:
//
//	[ 标准图 | 待测图 ]   （上方标签，中间内容互不遮挡）
//	[ 底部预期效果描述 ] （单独页脚区域，不盖住图）
//
// Small images are scaled up so panels and caption remain readable.
func ComposeSideBySide(standard, actual image.Image, opt CompositeOptions) (*image.RGBA, error) {
	if standard == nil || actual == nil {
		return nil, fmt.Errorf("nil image")
	}
	if opt.LeftLabel == "" {
		opt.LeftLabel = "标准 (CanvasKit)"
	}
	if opt.RightLabel == "" {
		opt.RightLabel = "待测 (gpui)"
	}
	if opt.MinPanelWidth <= 0 {
		opt.MinPanelWidth = 240
	}
	if opt.MinPanelHeight <= 0 {
		opt.MinPanelHeight = 160
	}
	if opt.FontSize <= 0 {
		opt.FontSize = 15
	}
	if strings.TrimSpace(opt.Description) == "" {
		opt.Description = "（无预期效果描述）"
	}

	sb, ab := standard.Bounds(), actual.Bounds()
	sw, sh, aw, ah := sb.Dx(), sb.Dy(), ab.Dx(), ab.Dy()
	if sw <= 0 || sh <= 0 || aw <= 0 || ah <= 0 {
		return nil, fmt.Errorf("empty image bounds")
	}

	scale := 1.0
	minW, minH := sw, sh
	if aw < minW {
		minW = aw
	}
	if ah < minH {
		minH = ah
	}
	if minW < opt.MinPanelWidth {
		if s := float64(opt.MinPanelWidth) / float64(minW); s > scale {
			scale = s
		}
	}
	if minH < opt.MinPanelHeight {
		if s := float64(opt.MinPanelHeight) / float64(minH); s > scale {
			scale = s
		}
	}
	if scale > 4 {
		scale = 4
	}

	left := scaleImageNearest(standard, scale)
	right := scaleImageNearest(actual, scale)
	lw, lh := left.Bounds().Dx(), left.Bounds().Dy()
	rw, rh := right.Bounds().Dx(), right.Bounds().Dy()
	panelW, panelH := lw, lh
	if rw > panelW {
		panelW = rw
	}
	if rh > panelH {
		panelH = rh
	}

	const gap, pad, labelH, footerPad = 10, 12, 28, 10

	face, err := loadCompositeFace(opt.FontPath, opt.FontSize)
	if err != nil {
		return nil, err
	}
	defer face.Close()
	labelFace, err := loadCompositeFace(opt.FontPath, opt.FontSize+1)
	if err != nil {
		labelFace = face
	} else {
		defer labelFace.Close()
	}

	contentW := panelW*2 + gap
	totalW := contentW + pad*2
	if totalW < 520 {
		totalW = 520
		contentW = totalW - pad*2
	}

	metrics := face.Metrics()
	lineH := metrics.Height.Ceil()
	if lineH < int(opt.FontSize)+4 {
		lineH = int(opt.FontSize) + 4
	}

	body := opt.Description
	bodyLines := wrapText(face, body, contentW-footerPad*2)
	if opt.Title != "" {
		bodyLines = append([]string{"用例: " + opt.Title}, bodyLines...)
	}
	// +1 line for "预期效果" heading
	footerH := footerPad*2 + lineH*(len(bodyLines)+1)
	if footerH < 56 {
		footerH = 56
	}

	totalH := pad + labelH + panelH + pad + footerH + pad
	dst := image.NewRGBA(image.Rect(0, 0, totalW, totalH))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.RGBA{R: 245, G: 246, B: 248, A: 255}}, image.Point{}, draw.Src)

	leftX := pad + (contentW-panelW*2-gap)/2
	if leftX < pad {
		leftX = pad
	}
	rightX := leftX + panelW + gap
	topY := pad

	drawString(dst, labelFace, opt.LeftLabel, leftX, topY+labelH-8, color.RGBA{R: 20, G: 90, B: 40, A: 255})
	drawString(dst, labelFace, opt.RightLabel, rightX, topY+labelH-8, color.RGBA{R: 30, G: 60, B: 140, A: 255})

	panelTop := topY + labelH
	drawPanel(dst, left, leftX, panelTop, panelW, panelH)
	drawPanel(dst, right, rightX, panelTop, panelW, panelH)
	sep := image.Rect(leftX+panelW+gap/2, panelTop, leftX+panelW+gap/2+1, panelTop+panelH)
	draw.Draw(dst, sep, &image.Uniform{C: color.RGBA{R: 180, G: 180, B: 185, A: 255}}, image.Point{}, draw.Src)

	footerTop := panelTop + panelH + pad
	footerRect := image.Rect(pad, footerTop, totalW-pad, footerTop+footerH)
	draw.Draw(dst, footerRect, &image.Uniform{C: color.RGBA{R: 255, G: 255, B: 255, A: 255}}, image.Point{}, draw.Src)
	border(dst, footerRect, color.RGBA{R: 210, G: 212, B: 218, A: 255})

	drawString(dst, labelFace, "预期效果", pad+footerPad, footerTop+lineH, color.RGBA{R: 80, G: 80, B: 90, A: 255})
	textY := footerTop + footerPad + lineH
	for i, line := range bodyLines {
		drawString(dst, face, line, pad+footerPad, textY+(i+1)*lineH, color.RGBA{R: 25, G: 25, B: 30, A: 255})
	}
	return dst, nil
}

// WriteCompositePNG encodes the composed image.
func WriteCompositePNG(path string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// ComposeFiles loads standard/actual PNGs and writes a side-by-side composite.
func ComposeFiles(standardPath, actualPath, outPath string, opt CompositeOptions) error {
	stdImg, err := DecodePNG(standardPath)
	if err != nil {
		return fmt.Errorf("standard: %w", err)
	}
	actImg, err := DecodePNG(actualPath)
	if err != nil {
		return fmt.Errorf("actual: %w", err)
	}
	out, err := ComposeSideBySide(stdImg, actImg, opt)
	if err != nil {
		return err
	}
	return WriteCompositePNG(outPath, out)
}

func drawPanel(dst *image.RGBA, src image.Image, x, y, panelW, panelH int) {
	r := image.Rect(x, y, x+panelW, y+panelH)
	draw.Draw(dst, r, &image.Uniform{C: color.RGBA{R: 255, G: 255, B: 255, A: 255}}, image.Point{}, draw.Src)
	border(dst, r, color.RGBA{R: 160, G: 165, B: 175, A: 255})
	sb := src.Bounds()
	ox := x + (panelW-sb.Dx())/2
	oy := y + (panelH-sb.Dy())/2
	draw.Draw(dst, image.Rect(ox, oy, ox+sb.Dx(), oy+sb.Dy()), src, sb.Min, draw.Over)
}

func border(dst *image.RGBA, r image.Rectangle, c color.RGBA) {
	u := &image.Uniform{C: c}
	draw.Draw(dst, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+1), u, image.Point{}, draw.Src)
	draw.Draw(dst, image.Rect(r.Min.X, r.Max.Y-1, r.Max.X, r.Max.Y), u, image.Point{}, draw.Src)
	draw.Draw(dst, image.Rect(r.Min.X, r.Min.Y, r.Min.X+1, r.Max.Y), u, image.Point{}, draw.Src)
	draw.Draw(dst, image.Rect(r.Max.X-1, r.Min.Y, r.Max.X, r.Max.Y), u, image.Point{}, draw.Src)
}

func scaleImageNearest(src image.Image, scale float64) *image.RGBA {
	b := src.Bounds()
	if scale <= 1.001 {
		out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		draw.Draw(out, out.Bounds(), src, b.Min, draw.Src)
		return out
	}
	nw := int(float64(b.Dx())*scale + 0.5)
	nh := int(float64(b.Dy())*scale + 0.5)
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	out := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		sy := b.Min.Y + int(float64(y)/scale)
		if sy >= b.Max.Y {
			sy = b.Max.Y - 1
		}
		for x := 0; x < nw; x++ {
			sx := b.Min.X + int(float64(x)/scale)
			if sx >= b.Max.X {
				sx = b.Max.X - 1
			}
			out.Set(x, y, src.At(sx, sy))
		}
	}
	return out
}

func drawString(dst *image.RGBA, face font.Face, s string, x, y int, col color.Color) {
	if face == nil || s == "" {
		return
	}
	d := &font.Drawer{Dst: dst, Src: image.NewUniform(col), Face: face, Dot: fixed.P(x, y)}
	d.DrawString(s)
}

func wrapText(face font.Face, text string, maxW int) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	var out []string
	for _, para := range strings.Split(text, "\n") {
		para = strings.TrimRight(para, " \t")
		if strings.TrimSpace(para) == "" {
			out = append(out, "")
			continue
		}
		var line []rune
		for _, r := range para {
			try := string(append(append([]rune{}, line...), r))
			if face != nil && font.MeasureString(face, try).Ceil() > maxW && len(line) > 0 {
				out = append(out, string(line))
				line = []rune{r}
			} else {
				line = append(line, r)
			}
		}
		if len(line) > 0 {
			out = append(out, string(line))
		}
	}
	return out
}

var (
	faceMu    sync.Mutex
	fontBytes []byte
	fontPath  string
)

func loadCompositeFace(path string, size float64) (font.Face, error) {
	if size <= 0 {
		size = 15
	}
	b, used, err := loadFontBytes(path)
	if err != nil {
		return nil, err
	}
	if col, err := opentype.ParseCollection(b); err == nil && col.NumFonts() > 0 {
		f, err := col.Font(0)
		if err != nil {
			return nil, err
		}
		return opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	}
	f, err := opentype.Parse(b)
	if err != nil {
		return nil, fmt.Errorf("parse font %s: %w", used, err)
	}
	return opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
}

func loadFontBytes(explicit string) ([]byte, string, error) {
	faceMu.Lock()
	defer faceMu.Unlock()
	if explicit != "" {
		b, err := os.ReadFile(explicit)
		return b, explicit, err
	}
	if fontBytes != nil {
		return fontBytes, fontPath, nil
	}
	candidates := []string{
		"standardtest/fonts/NotoSansSC-Regular.otf",
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/truetype/arphic/uming.ttc",
		"/usr/share/fonts/truetype/arphic/ukai.ttc",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"standardtest/fonts/DejaVuSans.ttf",
	}
	var last error
	for _, p := range candidates {
		b, err := os.ReadFile(p)
		if err != nil {
			last = err
			continue
		}
		fontBytes, fontPath = b, p
		return fontBytes, fontPath, nil
	}
	if last == nil {
		last = fmt.Errorf("no font found")
	}
	return nil, "", last
}

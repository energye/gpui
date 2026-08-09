package text

import "errors"

// 本文件是 FreeType 2.11.1 `src/smooth/ftgrays.c` 的逐行 Go 移植
// （x86_64 / FT_INT64 配置）：光栅语义与 FT 灰度光栅器完全一致，
// 用于 glyph mask 生产路径，保证位图与 FT_Render_Glyph 逐字节一致。
//
// 移植对照（ftgrays.c 行号）：
//   - gray_set_cell         : 572
//   - gray_render_line      : 869（#else / FT_INT64 分支）
//   - gray_render_conic     : 1045（BEZIER_USE_DDA）
//   - gray_split_cubic      : 1335
//   - gray_render_cubic     : 1358
//   - FT_Outline_Decompose  : 1647
//   - gray_sweep            : 1475
//   - gray_raster_render    : 2040
//   - gray_convert_glyph    : 1942
//   - ft_glyphslot_preset_bitmap GRAY 分支 : ftobjs.c 359
//
// 坐标约定（与 FT 一致）：
//   - 输入轮廓点：26.6 定点，Y-up（字形像素空间，baseline=0，向上为正）。
//   - 输出位图：256 级灰度，行序从上到下（buffer[0] = 顶部行，
//     bitmap_top = 顶部行相对 baseline 的像素距离）。

const (
	ftGraysPixelBits = 8
	ftGraysOnePixel  = 1 << ftGraysPixelBits
	ftGraysMaxPool   = 2048 // FT_MAX_GRAY_POOL
	ftGraysMaxCells  = 1 << 20
)

// ftVec26 是 26.6 定点轮廓点（FT_Vector 同构）。
type ftVec26 struct {
	x, y int64
}

// ftGraysCell 是光栅 cell（TCell 同构）。链表用 slice 索引模拟指针。
type ftGraysCell struct {
	x     int32
	cover int32
	area  int32
	next  int32
}

const ftCellNull = -1

// ftGraysWorker 是 gray_TWorker 同构。
type ftGraysWorker struct {
	minEx, maxEx int32
	minEy, maxEy int32
	countEy      int32

	cell     int32
	cellFree int32
	cellNull int32

	ycells []int32 // 每行 cell 链表头（索引）

	x, y int64 // 当前点（26.6 定点）

	outlineEvenOdd bool

	// 输出
	width, height int
	line          []byte // 输出行缓存（行序从上到下）

	cells []ftGraysCell
}

// ftGraysSetCell 移植 gray_set_cell（ftgrays.c:572）。
func (w *ftGraysWorker) setCell(ex, ey int32) {
	eyIndex := ey - w.minEy
	if eyIndex < 0 || eyIndex >= w.countEy || ex >= w.maxEx {
		w.cell = w.cellNull
		return
	}
	pcell := &w.ycells[eyIndex]
	ex = maxInt32(ex, w.minEx-1)
	for {
		cellIdx := *pcell
		if cellIdx == w.cellNull {
			// 链表尾：新建
			break
		}
		cell := &w.cells[cellIdx]
		if cell.x > ex {
			break
		}
		if cell.x == ex {
			w.cell = cellIdx
			return
		}
		pcell = &cell.next
	}
	// 插入新 cell
	idx := w.cellFree
	if idx >= int32(len(w.cells)) {
		idx = -1 // 池耗尽（band 重试由调用方处理）
		w.cell = w.cellNull
		return
	}
	w.cellFree++
	w.cells[idx] = ftGraysCell{x: ex, next: *pcell}
	*pcell = idx
	w.cell = idx
}

func maxInt32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

// ftGraysIntegrate 移植 FT_INTEGRATE（32 位环绕加法）。
func (w *ftGraysWorker) integrate(a, b int32) {
	if w.cell == w.cellNull {
		return
	}
	cell := &w.cells[w.cell]
	cell.cover += a
	cell.area += a * b
}

// ftGraysUDIVPrep / ftGraysUDIV 移植 FT_UDIVPREP / FT_UDIV（ftgrays.c:392-395）。
// 注意：FT 用「乘法移位近似除法」而非精确除法：
//
//	b_r = c ? 0xFFFFFFFF / b : 0（有符号向零除法）
//	UDIV(a) = (uint64(a) * uint64(b_r)) >> 32
//
// 该近似的舍入行为必须逐位复现，否则位图会差几个灰度级。
func ftGraysUDIVPrep(c bool, b int64) int64 {
	if !c {
		return 0
	}
	return int64(0xFFFFFFFF) / b
}

func ftGraysUDIV(a int64, bR int64) int32 {
	return int32((uint64(a) * uint64(bR)) >> 32)
}

// ftGraysFRACT 移植 FRACT：低 PIXEL_BITS 位（对负数 = 补码低 8 位，与 FT 一致）。
func ftGraysFRACT(x int64) int32 {
	return int32(x & (ftGraysOnePixel - 1))
}

// ftGraysTRUNC 移植 TRUNC：算术右移（向负无穷，与 C 一致）。
func ftGraysTRUNC(x int64) int32 {
	return int32(x >> ftGraysPixelBits)
}

// renderLine 移植 gray_render_line（ftgrays.c:869，#else / FT_INT64 分支）。
func (w *ftGraysWorker) renderLine(toX, toY int64) {
	ey1 := ftGraysTRUNC(w.y)
	ey2 := ftGraysTRUNC(toY)
	dx := toX - w.x
	dy := toY - w.y
	ex1 := ftGraysTRUNC(w.x)
	ex2 := ftGraysTRUNC(toX)
	fx1 := ftGraysFRACT(w.x)
	fy1 := ftGraysFRACT(w.y)
	var fx2, fy2 int32

	// 垂直裁剪
	if (ey1 >= w.maxEy && ey2 >= w.maxEy) || (ey1 < w.minEy && ey2 < w.minEy) {
		goto End
	}

	if ex1 == ex2 && ey1 == ey2 { // 单 cell 内
	} else if dy == 0 { // ex1 != ex2：水平线
		w.setCell(ex2, ey2)
		goto End
	} else if dx == 0 { // 竖线
		if dy > 0 {
			for {
				fy2 = int32(ftGraysOnePixel)
				w.integrate(fy2-fy1, fx1*2)
				fy1 = 0
				ey1++
				w.setCell(ex1, ey1)
				if ey1 == ey2 {
					break
				}
			}
		} else {
			for {
				fy2 = 0
				w.integrate(fy2-fy1, fx1*2)
				fy1 = int32(ftGraysOnePixel)
				ey1--
				w.setCell(ex1, ey1)
				if ey1 == ey2 {
					break
				}
			}
		}
	} else {
		// 任意方向线（ftgrays.c:932）：prod 判定四方向 + UDIV 近似出口
		one := int64(ftGraysOnePixel)
		prod := dx*int64(fy1) - dy*int64(fx1)
		bRDx := ftGraysUDIVPrep(ex1 != ex2, dx)
		bRDy := ftGraysUDIVPrep(ey1 != ey2, dy)

		for {
			switch {
			case prod-dx*one > 0 && prod <= 0: // left
				fx2 = 0
				fy2 = ftGraysUDIV(-prod, -bRDx)
				prod -= dy * one
				w.integrate(fy2-fy1, fx1+fx2)
				fx1 = int32(ftGraysOnePixel)
				fy1 = fy2
				ex1--
			case prod-dx*one+dy*one > 0 &&
				prod-dx*one <= 0: // up
				prod -= dx * one
				fx2 = ftGraysUDIV(-prod, bRDy)
				fy2 = int32(ftGraysOnePixel)
				w.integrate(fy2-fy1, fx1+fx2)
				fx1 = fx2
				fy1 = 0
				ey1++
			case prod+dy*one >= 0 &&
				prod-dx*one+dy*one <= 0: // right
				prod += dy * one
				fx2 = int32(ftGraysOnePixel)
				fy2 = ftGraysUDIV(prod, bRDx)
				w.integrate(fy2-fy1, fx1+fx2)
				fx1 = 0
				fy1 = fy2
				ex1++
			default: // down
				fx2 = ftGraysUDIV(prod, -bRDy)
				fy2 = 0
				prod += dx * one
				w.integrate(fy2-fy1, fx1+fx2)
				fx1 = fx2
				fy1 = int32(ftGraysOnePixel)
				ey1--
			}
			w.setCell(ex1, ey1)
			if ex1 == ex2 && ey1 == ey2 {
				break
			}
		}
	}

	fx2 = ftGraysFRACT(toX)
	fy2 = ftGraysFRACT(toY)
	w.integrate(fy2-fy1, fx1+fx2)

End:
	w.x = toX
	w.y = toY
}

// renderConicDDA 移植 gray_render_conic（ftgrays.c:1045，BEZIER_USE_DDA 分支）。
// 输入 control/to 为 26.6，内部 UPSCALE 到 8.8（与 gray_render_conic 一致）。
func (w *ftGraysWorker) renderConicDDA(control, to ftVec26) {
	p0x, p0y := w.x, w.y
	p1x, p1y := control.x*ftGraysOnePixel>>6, control.y*ftGraysOnePixel>>6
	p2x, p2y := to.x*ftGraysOnePixel>>6, to.y*ftGraysOnePixel>>6

	// band 穿越快捷路径
	if (ftGraysTRUNC(p0y) >= w.maxEy && ftGraysTRUNC(p1y) >= w.maxEy && ftGraysTRUNC(p2y) >= w.maxEy) ||
		(ftGraysTRUNC(p0y) < w.minEy && ftGraysTRUNC(p1y) < w.minEy && ftGraysTRUNC(p2y) < w.minEy) {
		w.x = p2x
		w.y = p2y
		return
	}

	bx := p1x - p0x
	by := p1y - p0y
	ax := p2x - p1x - bx
	ay := p2y - p1y - by

	dx := ftAbs64(ax)
	dy := ftAbs64(ay)
	if dx < dy {
		dx = dy
	}
	if dx <= ftGraysOnePixel/4 {
		w.renderLine(p2x, p2y)
		return
	}

	shift := 0
	for {
		dx >>= 2
		shift++
		if dx <= ftGraysOnePixel/4 {
			break
		}
	}

	rx := int64(uint64(ax) << (33 - 2*shift))
	ry := int64(uint64(ay) << (33 - 2*shift))
	qx := int64(uint64(bx)<<(33-shift)) + int64(uint64(ax)<<(32-2*shift))
	qy := int64(uint64(by)<<(33-shift)) + int64(uint64(ay)<<(32-2*shift))
	px := int64(uint64(p0x) << 32)
	py := int64(uint64(p0y) << 32)

	for count := 1 << shift; count > 0; count-- {
		px += qx
		py += qy
		qx += rx
		qy += ry
		w.renderLine(int64(int32(px>>32)), int64(int32(py>>32)))
	}
}

// splitCubic 移植 gray_split_cubic（ftgrays.c:1335）。
func splitCubic(base []ftVec26) {
	base[6] = base[3]
	a := base[0].x + base[1].x
	b := base[1].x + base[2].x
	c := base[2].x + base[3].x
	base[5].x = c >> 1
	c += b
	base[4].x = c >> 2
	base[1].x = a >> 1
	a += b
	base[2].x = a >> 2
	base[3].x = (a + c) >> 3

	a = base[0].y + base[1].y
	b = base[1].y + base[2].y
	c = base[2].y + base[3].y
	base[5].y = c >> 1
	c += b
	base[4].y = c >> 2
	base[1].y = a >> 1
	a += b
	base[2].y = a >> 2
	base[3].y = (a + c) >> 3
}

// renderCubic 移植 gray_render_cubic（ftgrays.c:1358）。
// arc 指针在固定栈数组上前后移动（split: +3，flat: -3），用 base 索引模拟。
// 输入 control1/control2/to 为 26.6，内部 UPSCALE 到 8.8（与 gray_render_cubic 一致）。
func (w *ftGraysWorker) renderCubic(control1, control2, to ftVec26) {
	var bezStack [16*3 + 1]ftVec26
	arcBase := 0 // 当前 4 元组起点（bezStack[arcBase : arcBase+4]）

	bezStack[0] = ftVec26{x: to.x * ftGraysOnePixel >> 6, y: to.y * ftGraysOnePixel >> 6}
	bezStack[1] = ftVec26{x: control2.x * ftGraysOnePixel >> 6, y: control2.y * ftGraysOnePixel >> 6}
	bezStack[2] = ftVec26{x: control1.x * ftGraysOnePixel >> 6, y: control1.y * ftGraysOnePixel >> 6}
	bezStack[3] = ftVec26{x: w.x, y: w.y}

	a0, a1, a2, a3 := &bezStack[0], &bezStack[1], &bezStack[2], &bezStack[3]
	if (ftGraysTRUNC(a0.y) >= w.maxEy &&
		ftGraysTRUNC(a1.y) >= w.maxEy &&
		ftGraysTRUNC(a2.y) >= w.maxEy &&
		ftGraysTRUNC(a3.y) >= w.maxEy) ||
		(ftGraysTRUNC(a0.y) < w.minEy &&
			ftGraysTRUNC(a1.y) < w.minEy &&
			ftGraysTRUNC(a2.y) < w.minEy &&
			ftGraysTRUNC(a3.y) < w.minEy) {
		w.x = a0.x
		w.y = a0.y
		return
	}

	arc := func() [4]*ftVec26 {
		b := bezStack[arcBase : arcBase+4]
		return [4]*ftVec26{&b[0], &b[1], &b[2], &b[3]}
	}

	for {
		cur := arc()
		if ftAbs64(2*cur[0].x-3*cur[1].x+cur[3].x) > ftGraysOnePixel/2 ||
			ftAbs64(2*cur[0].y-3*cur[1].y+cur[3].y) > ftGraysOnePixel/2 ||
			ftAbs64(cur[0].x-3*cur[2].x+2*cur[3].x) > ftGraysOnePixel/2 ||
			ftAbs64(cur[0].y-3*cur[2].y+2*cur[3].y) > ftGraysOnePixel/2 {
			splitCubic(bezStack[arcBase : arcBase+7])
			arcBase += 3
			continue
		}
		w.renderLine(cur[0].x, cur[0].y)
		if arcBase == 0 {
			return
		}
		arcBase -= 3
	}
}

func ftAbs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// ftOutlineTag 是轮廓点标记（FT_CURVE_TAG 同构）。
type ftOutlineTag uint8

const (
	ftTagOn    ftOutlineTag = 1 // FT_CURVE_TAG_ON
	ftTagConic ftOutlineTag = 0 // FT_CURVE_TAG_CONIC
	ftTagCubic ftOutlineTag = 2 // FT_CURVE_TAG_CUBIC
)

// decompose 移植 FT_Outline_Decompose（ftgrays.c:1647），shift=0/delta=0。
func (w *ftGraysWorker) decompose(pts []ftVec26, tags []ftOutlineTag, contours []int32) error {
	first := 0
	for _, lastRaw := range contours {
		last := lastRaw
		if last < 0 {
			return errFTInvalidOutline
		}
		vStart := pts[first]
		vLast := pts[last]
		vControl := vStart
		point := first
		tag := tags[first]

		// 首点不能是 cubic 控制点
		if tag == ftTagCubic {
			return errFTInvalidOutline
		}

		if tag == ftTagConic {
			if tags[last] == ftTagOn {
				// 从末点（on）开始
				vStart = vLast
				last--
			} else {
				// 首末均 conic：起点取中点
				vStart.x = (vStart.x + vLast.x) / 2
				vStart.y = (vStart.y + vLast.y) / 2
				vLast = vStart
			}
			point--
		}

		w.moveTo(vStart)
		limit := int(last)

		for point < limit {
			point++
			tag = tags[point]
			switch tag {
			case ftTagOn:
				w.renderLine(pts[point].x*ftGraysOnePixel>>6, pts[point].y*ftGraysOnePixel>>6)
			case ftTagConic:
				vControl = pts[point]
			DoConic:
				if point < limit {
					point++
					tag = tags[point]
					vec := pts[point]
					if tag == ftTagOn {
						w.renderConicDDA(vControl, vec)
						continue
					}
					if tag != ftTagConic {
						return errFTInvalidOutline
					}
					vMiddle := ftVec26{x: (vControl.x + vec.x) / 2, y: (vControl.y + vec.y) / 2}
					w.renderConicDDA(vControl, vMiddle)
					vControl = vec
					goto DoConic
				}
				w.renderConicDDA(vControl, vStart)
				goto Close
			default: // ftTagCubic
				if point+1 > limit || tags[point+1] != ftTagCubic {
					return errFTInvalidOutline
				}
				point += 2
				vec1 := pts[point-2]
				vec2 := pts[point-1]
				if point <= limit {
					vec := pts[point]
					w.renderCubic(vec1, vec2, vec)
					continue
				}
				w.renderCubic(vec1, vec2, vStart)
				goto Close
			}
		}
		// 闭合线
		w.renderLine(vStart.x*ftGraysOnePixel>>6, vStart.y*ftGraysOnePixel>>6)

	Close:
		first = int(lastRaw) + 1
	}
	return nil
}

var errFTInvalidOutline = errors.New("ftgrays: invalid outline")

// moveTo 移植 gray_move_to（ftgrays.c:1426）。
// moveTo 移植 gray_move_to（ftgrays.c:1428）：26.6 → UPSCALE 后设 cell。
func (w *ftGraysWorker) moveTo(v ftVec26) {
	x := v.x * ftGraysOnePixel >> 6
	y := v.y * ftGraysOnePixel >> 6
	w.setCell(ftGraysTRUNC(x), ftGraysTRUNC(y))
	w.x = x
	w.y = y
}

// sweep 移植 gray_sweep（ftgrays.c:1475）。
func (w *ftGraysWorker) sweep() {
	fill := int32(-1) << 31 // INT_MIN（非零绕）
	if w.outlineEvenOdd {
		fill = 0x100
	}
	for y := w.minEy; y < w.maxEy; y++ {
		cellIdx := w.ycells[y-w.minEy]
		x := w.minEx
		cover := int32(0)
		outRow := (w.height - 1 - int(y)) * w.width // 行序从上到下
		for cellIdx != w.cellNull {
			cell := &w.cells[cellIdx]
			var coverage int32
			if cover != 0 && cell.x > x {
				coverage = ftFillRule(cover, fill)
				for xx := x; xx < cell.x; xx++ {
					w.line[outRow+int(xx)] = byte(coverage)
				}
			}
			cover += cell.cover * (ftGraysOnePixel * 2)
			area := cover - cell.area
			if area != 0 && cell.x >= w.minEx {
				coverage = ftFillRule(area, fill)
				w.line[outRow+int(cell.x)] = byte(coverage)
			}
			x = cell.x + 1
			cellIdx = cell.next
		}
		if cover != 0 { // 仅裁剪
			coverage := ftFillRule(cover, fill)
			for xx := x; xx < w.maxEx; xx++ {
				w.line[outRow+int(xx)] = byte(coverage)
			}
		}
	}
}

// ftFillRule 移植 FT_FILL_RULE（ftgrays.c:403-413）。
func ftFillRule(area, fill int32) int32 {
	coverage := area >> (ftGraysPixelBits*2 + 1 - 8) // area >> 9
	if coverage&fill != 0 {
		coverage = ^coverage
	}
	if coverage > 255 && fill&(int32(-1)<<31) != 0 {
		coverage = 255
	}
	return coverage
}

// ftGraysBBox 是轮廓控制盒（26.6）。
type ftGraysBBox struct {
	xMin, yMin, xMax, yMax int64
}

// outlineCBox 移植 FT_Outline_Get_CBox（ftoutln.c:457）。
func outlineCBox(pts []ftVec26) ftGraysBBox {
	if len(pts) == 0 {
		return ftGraysBBox{}
	}
	bb := ftGraysBBox{xMin: pts[0].x, yMin: pts[0].y, xMax: pts[0].x, yMax: pts[0].y}
	for _, p := range pts[1:] {
		if p.x < bb.xMin {
			bb.xMin = p.x
		}
		if p.x > bb.xMax {
			bb.xMax = p.x
		}
		if p.y < bb.yMin {
			bb.yMin = p.y
		}
		if p.y > bb.yMax {
			bb.yMax = p.y
		}
	}
	return bb
}

// presetBitmap 移植 ft_glyphslot_preset_bitmap 的 GRAY 分支（ftobjs.c:359，
// FT_RENDER_MODE_NORMAL/LIGHT）。返回像素盒 + 位图尺寸。
func presetBitmap(cbox ftGraysBBox) (xLeft, yTop, width, height int32) {
	// rough pixel box（origin 平移为 0）
	pboxXMin := int32(cbox.xMin >> 6)
	pboxYMin := int32(cbox.yMin >> 6)
	pboxXMax := int32(cbox.xMax >> 6)
	pboxYMax := int32(cbox.yMax >> 6)
	// tiny remainder box
	cxMin := cbox.xMin & 63
	cyMin := cbox.yMin & 63
	cxMax := cbox.xMax & 63
	cyMax := cbox.yMax & 63
	// GRAY 分支
	pboxXMin += int32(cxMin >> 6)
	pboxYMin += int32(cyMin >> 6)
	pboxXMax += int32((cxMax + 63) >> 6)
	pboxYMax += int32((cyMax + 63) >> 6)

	xLeft = pboxXMin
	yTop = pboxYMax
	width = pboxXMax - pboxXMin
	height = pboxYMax - pboxYMin
	return
}

// RasterizeFT26 移植 gray_raster_render + gray_convert_glyph 的位图模式
// （ftsmooth.c ft_smooth_render：轮廓平移 → 光栅 → sweep）。
//
// 输入：26.6 定点轮廓点（Y-up）+ 每点 tag + 轮廓末点索引（inclusive）。
// 输出：256 级灰度位图（行序从上到下）+ bitmap_left/bitmap_top（FT 语义）。
func RasterizeFT26(pts []ftVec26, tags []ftOutlineTag, contours []int32, evenOdd bool) (mask []byte, left, top int32, err error) {
	if len(pts) == 0 || len(contours) == 0 {
		return nil, 0, 0, nil
	}
	if len(pts) != len(tags) {
		return nil, 0, 0, errors.New("ftgrays: pts/tags mismatch")
	}
	if int(contours[len(contours)-1]) != len(pts)-1 {
		return nil, 0, 0, errors.New("ftgrays: contours last != n_points-1")
	}

	cbox := outlineCBox(pts)
	xLeft, yTop, width, height := presetBitmap(cbox)
	if width <= 0 || height <= 0 {
		return nil, xLeft, yTop, nil
	}

	// ft_smooth_render：x_shift = 64*-left, y_shift = 64*-top + 64*rows
	// （ftsmooth.c:480，位图模式 y 原点在底部：origin = buffer + (rows-1)*pitch）
	// 注意：此处只平移，不 UPSCALE。UPSCALE（26.6→8.8）发生在回调层
	// （gray_move_to / gray_line_to / gray_conic_to / gray_cubic_to），
	// 与 FT 一致——decompose 里的 vStart/vMiddle 中点计算在 26.6 域进行。
	xShift := int64(64) * -int64(xLeft)
	yShift := int64(64)*-int64(yTop) + int64(64)*int64(height)
	shifted := make([]ftVec26, len(pts))
	for i, p := range pts {
		shifted[i] = ftVec26{x: p.x + xShift, y: p.y + yShift}
	}

	// gray_raster_render 位图模式：min_ex=0, min_ey=0, max_ex=width, max_ey=rows
	w := &ftGraysWorker{
		minEx:          0,
		maxEx:          width,
		minEy:          0,
		maxEy:          height,
		width:          int(width),
		height:         int(height),
		outlineEvenOdd: evenOdd,
		line:           make([]byte, int(width)*int(height)),
		cells:          make([]ftGraysCell, ftGraysMaxCells),
	}
	w.countEy = height
	w.cellNull = ftCellNull
	w.ycells = make([]int32, height)
	for i := range w.ycells {
		w.ycells[i] = w.cellNull
	}

	if err := w.decompose(shifted, tags, contours); err != nil {
		return nil, 0, 0, err
	}
	w.sweep()
	return w.line, xLeft, yTop, nil
}

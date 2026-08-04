package hint

// Type 2 charstring 解释器（M1）。
//
// 语义对齐 freetype-2.14.3 src/psaux/cffdecode.c 的
// cff_decoder_parse_charstrings（宽度判定 / flex 点序列 / 轮廓闭合）。
// 输出字体单位（cs）精确轮廓与 hstem/vstem 对，供 cf2HintMap（M2）使用，
// 消除从 26.6 像素反推的 ±1.3FU 误差（§13.4）。

import "fmt"

// csStem 是 charstring 中的横/竖笔（cs，Y-up 字体单位）。
type csStem struct {
	lo, hi float64
}

// csPt 是解释器输出的轮廓点（cs，Y-up）。
type csPt struct {
	x, y float64
	on   bool
}

// csSeac 记录 endchar 5 参数（seac 复合）信息（顺带支持不验证）。
type csSeac struct {
	adx, ady     float64
	bchar, achar uint32
}

// csOutline 是 Type 2 charstring 的解释结果。
type csOutline struct {
	pts      []csPt   // 各闭合轮廓的点（不重复首点）
	contours []int    // 每轮廓点数
	hstems   []csStem // hstem/hstemhm 对（出现顺序）
	vstems   []csStem // vstem/vstemhm 对
	hints    []csStem // 全部 stem，位序 = hstem 先 vstem 后（cf2 pshints.c:870）
	width    float64  // 字体单位宽度（width 参数 + nominalWidthX）
	hasWidth bool
	seac     *csSeac
	// hintmaskCount 是 charstring 中 hintmask/cntrmask 操作符出现次数。
	// 0 = 单区（全 stem 一直激活，M2 验证线）；>0 = 多区（M3 hintmask 分区）。
	hintmaskCount int
	// maskEvents 记录每个 hintmask/cntrmask 事件（M3 分区依据）：
	// 事件后产出的路径点（pts[atPt:]）用该 mask 的 hintmap。
	maskEvents []csMaskEvent
}

// csMaskEvent 是一次 hintmask/cntrmask 事件（cf2 语义：psintrp.c HINTMASK 分支）。
type csMaskEvent struct {
	cntr     bool   // true = cntrmask（counter 语义，psintrp.c:2609-2650）
	mask     []byte // 原始 mask 字节，位序 = hints[0]（MSB-first，pshints.c:897）
	numHints int    // 事件时的 hint 总数（hstem 先 vstem 后）
	atPt     int    // 事件前已产出的点数
}

// csInterp 是单字形 charstring 解释器。
type csInterp struct {
	data          []byte
	pos           int
	stack         []float64
	subrs         [][]byte // 当前局部 subrs（FD 的）
	gsubrs        [][]byte
	nominalWidthX float64 // FD Private DICT op 21（width = stack[0] + nominalWidthX）
	x, y          float64 // 当前点（cs）
	pathBegun     bool
	out           csOutline
	numHints      int
	curMask       []byte // 最近 hintmask/cntrmask 的位
	depth         int
}

const maxCSDepth = 10 // FT 的 CFF_MAX_OPERANDS 对应递归上限

// csBias 计算 callsubr/callgsubr 的偏置（Type 2 规范）。
func csBias(n int) int {
	switch {
	case n < 1240:
		return 107
	case n < 33900:
		return 1131
	default:
		return 32768
	}
}

// interpretCharstring 解释一条 charstring，返回 cs 轮廓与 stem 表。
// nominalWidthX 来自 FD Private DICT op 21（cf2_getNominalWidthX）。
func interpretCharstring(data []byte, subrs, gsubrs [][]byte, nominalWidthX float64) (*csOutline, error) {
	ip := &csInterp{data: data, subrs: subrs, gsubrs: gsubrs, nominalWidthX: nominalWidthX}
	if err := ip.run(); err != nil {
		return nil, err
	}
	ip.out.hints = append(ip.out.hints, ip.out.hstems...)
	ip.out.hints = append(ip.out.hints, ip.out.vstems...)
	return &ip.out, nil
}

func (ip *csInterp) run() error {
	for ip.pos < len(ip.data) {
		b := ip.data[ip.pos]
		ip.pos++
		var op int
		switch {
		case b >= 32 && b <= 246:
			ip.stack = append(ip.stack, float64(int(b)-139))
			continue
		case b >= 247 && b <= 250:
			if ip.pos >= len(ip.data) {
				return fmt.Errorf("cs: short number truncated")
			}
			ip.stack = append(ip.stack, float64((int(b)-247)*256+int(ip.data[ip.pos])+108))
			ip.pos++
			continue
		case b >= 251 && b <= 254:
			if ip.pos >= len(ip.data) {
				return fmt.Errorf("cs: short number truncated")
			}
			ip.stack = append(ip.stack, float64(-(int(b)-251)*256-int(ip.data[ip.pos])-108))
			ip.pos++
			continue
		case b == 28:
			if ip.pos+2 > len(ip.data) {
				return fmt.Errorf("cs: int16 truncated")
			}
			v := float64(int16(int(ip.data[ip.pos])<<8 | int(ip.data[ip.pos+1])))
			ip.stack = append(ip.stack, v)
			ip.pos += 2
			continue
		case b == 255:
			if ip.pos+4 > len(ip.data) {
				return fmt.Errorf("cs: fixed truncated")
			}
			// FT cffdecode.c:589-600：charstring_type==2 时 shift=0，
			// 255 是原始 int32（非 16.16）。
			ip.stack = append(ip.stack, float64(int32(int(ip.data[ip.pos])<<24|int(ip.data[ip.pos+1])<<16|
				int(ip.data[ip.pos+2])<<8|int(ip.data[ip.pos+3]))))
			ip.pos += 4
			continue
		case b == 12:
			if ip.pos >= len(ip.data) {
				return fmt.Errorf("cs: escaped op truncated")
			}
			op = 1200 + int(ip.data[ip.pos])
			ip.pos++
		case b == 10:
			op = 10
		case b == 11:
			op = 11
		case b == 14:
			op = 14
		case b == 21:
			op = 21
		case b == 22:
			op = 22
		case b == 23:
			op = 23
		case b == 29:
			op = 29
		case b <= 31:
			op = int(b)
		default:
			return fmt.Errorf("cs: bad byte %d at %d", b, ip.pos-1)
		}
		if err := ip.exec(op); err != nil {
			return err
		}
	}
	return nil
}

// exec 执行一个 operator（含 width 判定，语义同 FT cffdecode.c:884-945）。
func (ip *csInterp) exec(op int) error {
	numArgs := len(ip.stack)
	switch op {
	case 1, 3, 18, 23: // hstem/vstem/hstemvm/hstemhm
		if numArgs > 0 && !ip.out.hasWidth && numArgs&1 == 1 {
			ip.takeWidth()
		}
		args := ip.stack
		if len(args)%2 != 0 {
			return fmt.Errorf("cs: stem odd args %d", len(args))
		}
		// cf2_doStems（psintrp.c）语义：stem 坐标为相对前一对顶/右的
		// 累积偏移（position 逐对累加，本次 op 从 0 开始）。
		var pos float64
		for i := 0; i+1 < len(args); i += 2 {
			lo := pos + args[i]
			hi := lo + args[i+1]
			pos = hi
			if hi < lo {
				lo, hi = hi, lo
			}
			s := csStem{lo: lo, hi: hi}
			if op == 1 || op == 18 {
				ip.out.hstems = append(ip.out.hstems, s)
			} else {
				ip.out.vstems = append(ip.out.vstems, s)
			}
		}
		ip.numHints += len(args) / 2
		ip.stack = nil
	case 19, 20: // hintmask / cntrmask
		if numArgs > 0 && !ip.out.hasWidth && numArgs&1 == 1 {
			ip.takeWidth()
		}
		maskLen := (ip.numHints + 7) / 8
		if ip.pos+maskLen > len(ip.data) {
			return fmt.Errorf("cs: hintmask truncated")
		}
		ip.curMask = ip.data[ip.pos : ip.pos+maskLen]
		ip.pos += maskLen
		ip.out.hintmaskCount++
		ip.out.maskEvents = append(ip.out.maskEvents, csMaskEvent{
			cntr:     op == 20,
			mask:     append([]byte(nil), ip.curMask...),
			numHints: ip.numHints,
			atPt:     len(ip.out.pts),
		})
		ip.stack = nil
	case 21: // rmoveto
		if numArgs > 0 && !ip.out.hasWidth && numArgs&1 == 1 {
			ip.takeWidth()
		}
		if len(ip.stack) < 2 {
			return fmt.Errorf("cs: rmoveto underflow")
		}
		ip.closeContour()
		ip.pathBegun = false
		ip.x += ip.stack[len(ip.stack)-2]
		ip.y += ip.stack[len(ip.stack)-1]
		ip.stack = nil
	case 22: // hmoveto
		if numArgs > 0 && !ip.out.hasWidth && numArgs&2 != 0 {
			ip.takeWidth()
		}
		if len(ip.stack) < 1 {
			return fmt.Errorf("cs: hmoveto underflow")
		}
		ip.closeContour()
		ip.pathBegun = false
		ip.x += ip.stack[len(ip.stack)-1]
		ip.stack = nil
	case 4: // vmoveto
		if numArgs > 0 && !ip.out.hasWidth && numArgs&2 != 0 {
			ip.takeWidth()
		}
		if len(ip.stack) < 1 {
			return fmt.Errorf("cs: vmoveto underflow")
		}
		ip.closeContour()
		ip.pathBegun = false
		ip.y += ip.stack[len(ip.stack)-1]
		ip.stack = nil
	case 5, 24, 25: // rlineto / rcurveline / rlinecurve
		if err := ip.startPoint(); err != nil {
			return err
		}
		return ip.mixedPairs(op)
	case 6, 7: // hlineto / vlineto
		if err := ip.startPoint(); err != nil {
			return err
		}
		phase := op == 6
		for _, v := range ip.stack {
			if phase {
				ip.x += v
			} else {
				ip.y += v
			}
			ip.addPt(ip.x, ip.y, true)
			phase = !phase
		}
		ip.stack = nil
	case 8, 26, 27: // rrcurveto / vvcurveto / hhcurveto
		if err := ip.startPoint(); err != nil {
			return err
		}
		return ip.curves(op)
	case 30, 31: // vhcurveto / hvcurveto
		if err := ip.startPoint(); err != nil {
			return err
		}
		return ip.alternating(op)
	case 10, 29: // callsubr / callgsubr
		if len(ip.stack) < 1 {
			return fmt.Errorf("cs: call underflow")
		}
		raw := int(ip.stack[len(ip.stack)-1])
		ip.stack = ip.stack[:len(ip.stack)-1]
		targets := ip.subrs
		bias := csBias(len(ip.subrs))
		if op == 29 {
			targets = ip.gsubrs
			bias = csBias(len(ip.gsubrs))
		}
		idx := raw + bias
		if ip.depth >= maxCSDepth {
			return fmt.Errorf("cs: call depth exceeded")
		}
		if idx < 0 || idx >= len(targets) {
			return fmt.Errorf("cs: subr %d out of range", idx)
		}
		sub := csInterp{
			data:          targets[idx],
			subrs:         ip.subrs,
			gsubrs:        ip.gsubrs,
			nominalWidthX: ip.nominalWidthX,
			stack:         ip.stack,
			x:             ip.x,
			y:             ip.y,
			pathBegun:     ip.pathBegun,
			numHints:      ip.numHints,
			depth:         ip.depth + 1,
			out:           ip.out,
		}
		if err := sub.run(); err != nil && err != errCSReturn {
			return err
		}
		ip.out = sub.out
		ip.x, ip.y = sub.x, sub.y
		ip.pathBegun = sub.pathBegun
		ip.numHints = sub.numHints
		ip.stack = sub.stack
	case 11: // return
		return errCSReturn
	case 14: // endchar
		// FT：num_args==1 或 ==5 时首参数为宽度（cf2_intrp.c endchar 语义）。
		if numArgs == 1 && !ip.out.hasWidth {
			ip.takeWidth()
		}
		if numArgs >= 4 && numArgs != 1 {
			// seac：5 参数（width 未消费时）+ 4 参数
			if numArgs == 5 && !ip.out.hasWidth {
				ip.takeWidth()
				numArgs = len(ip.stack)
			}
			if len(ip.stack) >= 4 {
				ip.out.seac = &csSeac{
					adx:   ip.stack[0],
					ady:   ip.stack[1],
					bchar: uint32(int32(ip.stack[2])),
					achar: uint32(int32(ip.stack[3])),
				}
				ip.stack = nil
				return nil
			}
		}
		ip.closeContour()
		ip.stack = nil
		return nil
	case 1235: // flex
		if err := ip.startPoint(); err != nil {
			return err
		}
		// 6 点：off off on off off on（count==4 || count==1 为 on）
		args := ip.stack
		if len(args) < 12 {
			return fmt.Errorf("cs: flex underflow")
		}
		for count := 6; count > 0; count-- {
			ip.x += args[0]
			ip.y += args[1]
			ip.addPt(ip.x, ip.y, count == 4 || count == 1)
			args = args[2:]
		}
		ip.stack = nil
	case 1234: // hflex（7 参数）
		if err := ip.startPoint(); err != nil {
			return err
		}
		args := ip.stack
		if len(args) < 7 {
			return fmt.Errorf("cs: hflex underflow")
		}
		startY := ip.y
		ip.x += args[0]
		ip.addPt(ip.x, ip.y, false)
		ip.x += args[1]
		ip.y += args[2]
		ip.addPt(ip.x, ip.y, false)
		ip.x += args[3]
		ip.addPt(ip.x, ip.y, true)
		ip.x += args[4]
		ip.addPt(ip.x, ip.y, false)
		ip.x += args[5]
		ip.y = startY
		ip.addPt(ip.x, ip.y, false)
		ip.x += args[6]
		ip.addPt(ip.x, ip.y, true)
		ip.stack = nil
	case 1236: // hflex1（9 参数）
		if err := ip.startPoint(); err != nil {
			return err
		}
		args := ip.stack
		if len(args) < 9 {
			return fmt.Errorf("cs: hflex1 underflow")
		}
		startY := ip.y
		ip.x += args[0]
		ip.y += args[1]
		ip.addPt(ip.x, ip.y, false)
		ip.x += args[2]
		ip.y += args[3]
		ip.addPt(ip.x, ip.y, false)
		ip.x += args[4]
		ip.addPt(ip.x, ip.y, true)
		ip.x += args[5]
		ip.addPt(ip.x, ip.y, false)
		ip.x += args[6]
		ip.y += args[7]
		ip.addPt(ip.x, ip.y, false)
		ip.x += args[8]
		ip.y = startY
		ip.addPt(ip.x, ip.y, true)
		ip.stack = nil
	case 1237: // flex1（11 参数）
		if err := ip.startPoint(); err != nil {
			return err
		}
		args := ip.stack
		if len(args) < 11 {
			return fmt.Errorf("cs: flex1 underflow")
		}
		startX, startY := ip.x, ip.y
		var dx, dy float64
		for i := 0; i < 10; i += 2 {
			dx += args[i]
			dy += args[i+1]
		}
		if dx < 0 {
			dx = -dx
		}
		if dy < 0 {
			dy = -dy
		}
		horizontal := dx > dy
		for count := 5; count > 0; count-- {
			ip.x += args[0]
			ip.y += args[1]
			ip.addPt(ip.x, ip.y, count == 3)
			args = args[2:]
		}
		if horizontal {
			ip.x += args[0]
			ip.y = startY
		} else {
			ip.x = startX
			ip.y += args[0]
		}
		ip.addPt(ip.x, ip.y, true)
		ip.stack = nil
	case 12, 1201: // dotsection / vsindex：忽略
	case 1202: // blend：非可变 CFF 不应出现，忽略
		return fmt.Errorf("cs: blend not supported (variable CFF2)")
	default:
		return fmt.Errorf("cs: unsupported op %d", op)
	}
	return nil
}

var errCSReturn = fmt.Errorf("cs: return")

// takeWidth 消费栈首元素为 width（FT cf2 语义：width = stack[0] + nominalWidthX）。
func (ip *csInterp) takeWidth() {
	ip.out.width = ip.stack[0] + ip.nominalWidthX
	ip.out.hasWidth = true
	ip.stack = ip.stack[1:]
}

// startPoint 若路径未开始，先记录当前点为 on 起点（FT cff_builder_start_point）。
func (ip *csInterp) startPoint() error {
	if ip.pathBegun {
		return nil
	}
	ip.pathBegun = true
	ip.addPt(ip.x, ip.y, true)
	return nil
}

// addPt 追加一个点。
func (ip *csInterp) addPt(x, y float64, on bool) {
	ip.out.pts = append(ip.out.pts, csPt{x: x, y: y, on: on})
}

// closeContour 闭合当前轮廓（FT cff_builder_close_contour：重复首点的 on 尾点删除；
// 单点轮廓丢弃）。
func (ip *csInterp) closeContour() {
	if !ip.pathBegun || len(ip.out.pts) == 0 {
		ip.pathBegun = false
		return
	}
	// 找到当前轮廓起点
	var first int
	if len(ip.out.contours) > 0 {
		for _, n := range ip.out.contours {
			first += n
		}
	}
	// 起点 = 本轮廓第一点（contours 累计 + 当前 pts 段起点）
	n := len(ip.out.pts)
	if n > 1 {
		last := ip.out.pts[n-1]
		fp := ip.out.pts[first]
		if last.x == fp.x && last.y == fp.y && last.on {
			ip.out.pts = ip.out.pts[:n-1]
			n--
		}
	}
	if n-first <= 1 {
		// 单点轮廓：丢弃
		ip.out.pts = ip.out.pts[:first]
	} else {
		ip.out.contours = append(ip.out.contours, n-first)
	}
	ip.pathBegun = false
}

// mixedPairs 处理 rlineto/rcurveline/rlinecurve（FT cffdecode.c 语义）。
func (ip *csInterp) mixedPairs(op int) error {
	args := ip.stack
	switch op {
	case 5: // rlineto：全部直线对
		for i := 0; i+1 < len(args); i += 2 {
			ip.x += args[i]
			ip.y += args[i+1]
			ip.addPt(ip.x, ip.y, true)
		}
	case 24: // rcurveline：n 组曲线 + 最后 1 条线（nargs = len-2 向下取 6 倍数 +2）
		if len(args) < 8 {
			return fmt.Errorf("cs: rcurveline underflow")
		}
		nargs := len(args) - 2
		nargs = nargs - nargs%6 + 2
		lineStart := nargs - 2
		for i := 0; i < lineStart; i += 6 {
			ip.curve6(args[i : i+6])
		}
		ip.x += args[lineStart]
		ip.y += args[lineStart+1]
		ip.addPt(ip.x, ip.y, true)
	case 25: // rlinecurve：n 条线 + 最后 1 组曲线（nargs = len&~1）
		if len(args) < 8 {
			return fmt.Errorf("cs: rlinecurve underflow")
		}
		nargs := len(args) &^ 1
		numLines := (nargs - 6) / 2
		for i := 0; i < numLines*2; i += 2 {
			ip.x += args[i]
			ip.y += args[i+1]
			ip.addPt(ip.x, ip.y, true)
		}
		ip.curve6(args[numLines*2:])
	}
	ip.stack = nil
	return nil
}

// curves 处理 rrcurveto(8) / vvcurveto(26) / hhcurveto(27)（FT 语义）。
func (ip *csInterp) curves(op int) error {
	args := ip.stack
	if op == 8 { // rrcurveto：消费 6n，余数丢弃（cffdecode.c:1135-1163）
		nargs := len(args) - len(args)%6
		for i := 0; i < nargs; i += 6 {
			ip.curve6(args[i : i+6])
		}
		ip.stack = nil
		return nil
	}
	// vv/hh：nargs = len&~2；奇数时先消费单个首轴参数
	nargs := len(args) &^ 2
	i := 0
	if nargs&1 == 1 {
		if op == 26 { // vv：首个参数是 dx1
			ip.x += args[0]
		} else { // hh：首个参数是 dy1
			ip.y += args[0]
		}
		i++
	}
	for i+3 < len(args) && i+3 < nargs {
		if op == 26 { // vv：组 (dy1, dx2, dy2, dy3)
			ip.y += args[i]
			ip.addPt(ip.x, ip.y, false)
			ip.x += args[i+1]
			ip.y += args[i+2]
			ip.addPt(ip.x, ip.y, false)
			ip.y += args[i+3]
			ip.addPt(ip.x, ip.y, true)
		} else { // hh：组 (dx1, dx2, dy2, dx3)
			ip.x += args[i]
			ip.addPt(ip.x, ip.y, false)
			ip.x += args[i+1]
			ip.y += args[i+2]
			ip.addPt(ip.x, ip.y, false)
			ip.x += args[i+3]
			ip.addPt(ip.x, ip.y, true)
		}
		i += 4
	}
	ip.stack = nil
	return nil
}

// alternating 处理 vhcurveto(30) / hvcurveto(31)（FT cffdecode.c 语义：
// 每组 4 参数，phase 交替；nargs==1 时该组多消费 args[i+4]，args 每组合推进 4）。
func (ip *csInterp) alternating(op int) error {
	args := ip.stack
	nargs := len(args) &^ 2
	phase := op == 31 // hvcurveto 以 x 开头
	i := 0
	for nargs >= 4 {
		nargs -= 4
		if phase { // hv
			ip.x += args[i]
			ip.addPt(ip.x, ip.y, false)
			ip.x += args[i+1]
			ip.y += args[i+2]
			ip.addPt(ip.x, ip.y, false)
			ip.y += args[i+3]
			if nargs == 1 {
				ip.x += args[i+4]
				nargs = 0
			}
			ip.addPt(ip.x, ip.y, true)
		} else { // vh
			ip.y += args[i]
			ip.addPt(ip.x, ip.y, false)
			ip.x += args[i+1]
			ip.y += args[i+2]
			ip.addPt(ip.x, ip.y, false)
			ip.x += args[i+3]
			if nargs == 1 {
				ip.y += args[i+4]
				nargs = 0
			}
			ip.addPt(ip.x, ip.y, true)
		}
		i += 4
		phase = !phase
	}
	ip.stack = nil
	return nil
}

// curve6 应用一组 6 个相对参数。
func (ip *csInterp) curve6(a []float64) {
	ip.curveTo(a[0], a[1], a[2], a[3], a[4], a[5])
}

// curveTo 应用一条三次贝塞尔（相对偏移），3 个点：off off on。
func (ip *csInterp) curveTo(dx1, dy1, dx2, dy2, dx3, dy3 float64) {
	ip.x += dx1
	ip.y += dy1
	ip.addPt(ip.x, ip.y, false)
	ip.x += dx2
	ip.y += dy2
	ip.addPt(ip.x, ip.y, false)
	ip.x += dx3
	ip.y += dy3
	ip.addPt(ip.x, ip.y, true)
}

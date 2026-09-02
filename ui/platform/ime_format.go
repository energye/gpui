//go:build linux

package platform

import (
	"strings"

	"github.com/godbus/dbus/v5"
)

// ImeSegment mirrors ui/input.Segment for platform layer (avoid import cycle)
// B9: Segments are currently parsed (DecodeIBusVariant/parseFcitxPreedit) but only
// logged in handleSignal, not forwarded to the Editor/DrawPreedit. Rich preedit
// styling (underline/highlight/selected) awaits a Segment-aware IME event and
// DrawPreedit upgrade; until then this is intentional dead code kept for debug.
type ImeSegment struct {
	Start, End int
	Attr       uint8
}

const (
	ibusAttrUnderline  = 1
	ibusAttrForeground = 2
	ibusAttrBackground = 3

	ibusUnderlineSingle = 1
	ibusUnderlineDouble = 2

	ibusTextName     = "IBusText"
	ibusAttrListName = "IBusAttrList"
	ibusAttrName     = "IBusAttribute"
)

// ImeSegment.Attr 取值（与 IBus 属性类型对应，见 docs §15 B21）。
const (
	ImeAttrUnderline = 1 // 下划线（IBus ATTR_UNDERLINE 单线；前景色属性亦归入此值）
	ImeAttrDouble    = 2 // 双下划线（ATTR_UNDERLINE + UNDERLINE_DOUBLE）
	ImeAttrSelected  = 3 // 背景反显（ATTR_BACKGROUND，候选选中态）
	// ImeAttrHighlight 是无属性时的整段默认样式，与单下划线同值：
	// singleSegment 产出该值，故「属性被丢弃、退化为单段」的检测以此为判据。
	ImeAttrHighlight = ImeAttrUnderline
)

type ibusAttr struct {
	Type  uint32
	Value uint32
	Start uint32
	End   uint32
}

type ibusTextPayload struct {
	Text  string
	Attrs []ibusAttr
}

type ibusFullPayload struct {
	Name  string
	Props map[string]dbus.Variant
	Text  string
	Attrs dbus.Variant
}

func attrToSegment(a ibusAttr, textLen int) (ImeSegment, bool) {
	attr := uint8(1)
	switch a.Type {
	case ibusAttrUnderline:
		if a.Value == ibusUnderlineDouble {
			attr = 2
		}
	case ibusAttrBackground:
		attr = 3
	case ibusAttrForeground:
		attr = 1
	}
	start := int(a.Start)
	end := int(a.End)
	if start < 0 {
		start = 0
	}
	if end > textLen {
		end = textLen
	}
	if start >= end {
		return ImeSegment{}, false
	}
	return ImeSegment{Start: start, End: end, Attr: attr}, true
}

// ibusAttrListWire 是 IBusAttrList 的线上结构 (sa{sv}av)。
// godbus 把 (sa{sv}av) 的 variant 解成具名 struct（非 []interface{}），故按结构体直取。
type ibusAttrListWire struct {
	Name  string
	Props map[string]dbus.Variant
	Attrs []dbus.Variant
}

// ibusAttrWire 是单条 IBusAttribute 的线上结构 (sa{sv}uuuu)。
type ibusAttrWire struct {
	Name  string
	Props map[string]dbus.Variant
	Type  uint32
	Value uint32
	Start uint32
	End   uint32
}

// parseIBusAttrList 解析线上真实格式的 AttrList：
// (sa{sv}av) ["IBusAttrList", {}, [<(sa{sv}uuuu) ["IBusAttribute", {}, type, value, start, end]>, ...]]
// 实测 fcitx5/ibus 均按此结构下发（见 docs/ENGINE_TEXT_X11_IME_REQUIREMENT.md §15 B21）。
func parseIBusAttrList(v dbus.Variant, textLen int) []ImeSegment {
	if v.Signature().String() == "" {
		return nil
	}
	attrs := decodeAttrList(v)
	if len(attrs) == 0 {
		return nil
	}
	segs := make([]ImeSegment, 0, len(attrs))
	for _, ia := range attrs {
		if seg, ok := attrToSegment(ia, textLen); ok {
			segs = append(segs, seg)
		}
	}
	return segs
}

// decodeAttrList 从 AttrList variant 取出属性条目，兼容 struct / []interface{} 两种解包形态。
func decodeAttrList(v dbus.Variant) []ibusAttr {
	// 形态一：godbus 直解为具名 struct（实测路径）
	if w, ok := v.Value().(ibusAttrListWire); ok {
		return wireToAttrs(w)
	}
	// 形态二：按结构体 Store
	var w ibusAttrListWire
	if err := dbus.Store([]interface{}{v}, &w); err == nil {
		return wireToAttrs(w)
	}
	// 形态三：原始元组 []interface{}
	if arr, ok := v.Value().([]interface{}); ok && len(arr) >= 3 {
		if vs, ok := arr[2].([]dbus.Variant); ok {
			attrs := make([]ibusAttr, 0, len(vs))
			for _, av := range vs {
				if ia, ok := decodeIBusAttr(av); ok {
					attrs = append(attrs, ia)
				}
			}
			return attrs
		}
	}
	return nil
}

func wireToAttrs(w ibusAttrListWire) []ibusAttr {
	if w.Name != ibusAttrListName && w.Name != "" {
		return nil
	}
	attrs := make([]ibusAttr, 0, len(w.Attrs))
	for _, av := range w.Attrs {
		ia, ok := decodeIBusAttr(av)
		if !ok {
			continue
		}
		attrs = append(attrs, ia)
	}
	return attrs
}

// decodeIBusAttr 解单条 IBusAttribute：
// (sa{sv}uuuu) [name, props, type, value, start, end]
func decodeIBusAttr(v dbus.Variant) (ibusAttr, bool) {
	// 形态一：godbus 直解为具名 struct（实测路径）
	if w, ok := v.Value().(ibusAttrWire); ok {
		if w.Name == ibusAttrName || w.Name == "" {
			return ibusAttr{Type: w.Type, Value: w.Value, Start: w.Start, End: w.End}, true
		}
	}
	// 形态二：按结构体 Store
	var w ibusAttrWire
	if err := dbus.Store([]interface{}{v}, &w); err == nil {
		if w.Name == ibusAttrName || w.Name == "" {
			return ibusAttr{Type: w.Type, Value: w.Value, Start: w.Start, End: w.End}, true
		}
	}
	// 形态三：原始元组 []interface{}
	arr, ok := v.Value().([]interface{})
	if !ok || len(arr) < 6 {
		return ibusAttr{}, false
	}
	name, _ := arr[0].(string)
	if name != ibusAttrName && name != "" {
		return ibusAttr{}, false
	}
	readU := func(i int) (uint32, bool) {
		switch x := arr[i].(type) {
		case uint32:
			return x, true
		case uint:
			return uint32(x), true
		case int32:
			if x < 0 {
				return 0, false
			}
			return uint32(x), true
		}
		return 0, false
	}
	tp, ok1 := readU(2)
	val, ok2 := readU(3)
	st, ok3 := readU(4)
	en, ok4 := readU(5)
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return ibusAttr{}, false
	}
	return ibusAttr{Type: tp, Value: val, Start: st, End: en}, true
}

func singleSegment(text string) []ImeSegment {
	if text == "" {
		return nil
	}
	return []ImeSegment{{Start: 0, End: len(text), Attr: 1}}
}

// parseIBusText decodes the IBus variant Text (IBusText) into plain text and segments.
// IBusText wire is a GVariant-serialized object: (sa{sv}sv) [IBusText, {}, text, <IBusAttrList>]
// godbus exposes it as dbus.Variant whose Value() is a 4-field tuple, third field holds the text.
func parseIBusText(v dbus.Variant) (string, []ImeSegment) {
	if s, ok := v.Value().(string); ok {
		if s == "" {
			return "", nil
		}
		return s, singleSegment(s)
	}
	var txt ibusTextPayload
	if err := dbus.Store([]interface{}{v}, &txt); err == nil && txt.Text != "" {
		if len(txt.Attrs) == 0 {
			return txt.Text, singleSegment(txt.Text)
		}
		segs := make([]ImeSegment, 0, len(txt.Attrs))
		for _, a := range txt.Attrs {
			if seg, ok := attrToSegment(a, len(txt.Text)); ok {
				segs = append(segs, seg)
			}
		}
		if len(segs) == 0 {
			return txt.Text, singleSegment(txt.Text)
		}
		return txt.Text, segs
	}
	var full ibusFullPayload
	if err := dbus.Store([]interface{}{v}, &full); err == nil && full.Text != "" {
		if full.Name == ibusTextName || full.Name == "" {
			// 线上真实格式走这里：Attrs 是 (sa{sv}av) 的 IBusAttrList variant。
			if segs := parseIBusAttrList(full.Attrs, len(full.Text)); len(segs) > 0 {
				return full.Text, segs
			}
			return full.Text, singleSegment(full.Text)
		}
	}
	if val := v.Value(); val != nil {
		if s, ok := extractIBusTextFromValue(val); ok {
			if s == "" {
				return "", nil
			}
			return s, singleSegment(s)
		}
		// Single String() materialization for fallback parsing.
		sStr := v.String()
		if s, ok := extractIBusTextFromString(sStr); ok {
			if s == "" {
				return "", nil
			}
			return s, singleSegment(s)
		}
		// Escaped GVariant form.
		if strings.Contains(sStr, "\\\"") {
			if s, ok := extractIBusTextFromString(strings.ReplaceAll(sStr, "\\\"", "\"")); ok {
				if s == "" {
					return "", nil
				}
				return s, singleSegment(s)
			}
		}
	}
	s := strings.Trim(v.String(), "\"")
	if s == "" {
		return "", nil
	}
	if extracted, ok := extractIBusTextFromString(s); ok {
		if extracted == "" {
			return "", nil
		}
		return extracted, singleSegment(extracted)
	}
	if idx := strings.Index(s, "["); idx >= 0 {
		if ex, ok := extractIBusTextFromString(s[idx:]); ok {
			if ex == "" {
				return "", nil
			}
			return ex, singleSegment(ex)
		}
	}
	return s, singleSegment(s)
}

func extractIBusTextFromValue(val interface{}) (string, bool) {
	if arr, ok := val.([]interface{}); ok && len(arr) >= 3 {
		if s, ok := arr[2].(string); ok && s != ibusTextName && s != ibusAttrListName {
			return s, true
		}
	}
	return "", false
}

func extractIBusTextFromString(s string) (string, bool) {
	norm := s
	if strings.Contains(s, "\\\"") {
		norm = strings.ReplaceAll(s, "\\\"", "\"")
	}
	idx := strings.Index(norm, ibusTextName)
	if idx < 0 {
		return "", false
	}
	after := idx + len(ibusTextName)
	if after < len(norm) && norm[after] == '"' {
		after++
	}
	rest := norm[after:]
	var quotes []string
	for i := 0; i < len(rest) && len(quotes) < 2; {
		q := strings.Index(rest[i:], "\"")
		if q < 0 {
			break
		}
		start := i + q + 1
		end := start
		for end < len(rest) {
			if rest[end] == '"' && (end == 0 || rest[end-1] != '\\') {
				break
			}
			end++
		}
		if end >= len(rest) {
			break
		}
		quotes = append(quotes, rest[start:end])
		i = end + 1
	}
	if len(quotes) == 0 {
		return "", false
	}
	if quotes[0] != ibusAttrListName {
		return quotes[0], true
	}
	if len(quotes) >= 2 {
		return quotes[1], true
	}
	return "", false
}

func parseFcitxPreedit(text string) (string, []ImeSegment) {
	if text == "" {
		return "", nil
	}
	if !strings.Contains(text, "\x1b") {
		return text, singleSegment(text)
	}
	clean := strings.ReplaceAll(text, "\x1b", "")
	if clean == "" {
		clean = text
	}
	return clean, singleSegment(clean)
}

func DecodeIBusVariant(v dbus.Variant) (string, []ImeSegment) {
	return parseIBusText(v)
}

func IBusAttrsToSegments(text string, attrs []struct {
	Type  uint32
	Value uint32
	Start uint32
	End   uint32
}) []ImeSegment {
	if len(attrs) == 0 {
		return singleSegment(text)
	}
	segs := make([]ImeSegment, 0, len(attrs))
	for _, a := range attrs {
		ia := ibusAttr{Type: a.Type, Value: a.Value, Start: a.Start, End: a.End}
		if seg, ok := attrToSegment(ia, len(text)); ok {
			segs = append(segs, seg)
		}
	}
	if len(segs) == 0 {
		return singleSegment(text)
	}
	return segs
}

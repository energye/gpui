//go:build linux

package platform

import (
	"strings"

	"github.com/godbus/dbus/v5"
)

// ImeSegment mirrors ui/input.Segment for platform layer (avoid import cycle)
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

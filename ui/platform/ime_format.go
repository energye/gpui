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

// ibusAttrType mirrors IBUS_ATTR_TYPE_* (ibustypes.h)
const (
	ibusAttrUnderline  = 1
	ibusAttrForeground = 2
	ibusAttrBackground = 3
)

const (
	ibusUnderlineSingle = 1
	ibusUnderlineDouble = 2
)

// parseIBusText decodes the IBus variant Text (IBusText) into plain text and segments.
// IBusText is variant containing struct { text string, attrs []Attr }
// On failure it returns the variant's string value and a default single underline segment.
func parseIBusText(v dbus.Variant) (string, []ImeSegment) {
	// Try variant as string (fallback)
	if s, ok := v.Value().(string); ok {
		if s == "" {
			return "", nil
		}
		return s, []ImeSegment{{Start: 0, End: len(s), Attr: 1}}
	}
	// Try as struct with Text and Attrs via dbus.Store
	type ibusAttr struct {
		Type  uint32
		Value uint32
		Start uint32
		End   uint32
	}
	type ibusText struct {
		Text  string
		Attrs []ibusAttr
	}
	var txt ibusText
	if err := dbus.Store([]interface{}{v}, &txt); err == nil && txt.Text != "" {
		// Convert attrs to segments
		if len(txt.Attrs) == 0 {
			return txt.Text, []ImeSegment{{Start: 0, End: len(txt.Text), Attr: 1}}
		}
		var segs []ImeSegment
		for _, a := range txt.Attrs {
			attr := uint8(1)
			if a.Type == ibusAttrUnderline {
				if a.Value == ibusUnderlineDouble {
					attr = 2
				} else {
					attr = 1
				}
			} else if a.Type == ibusAttrBackground {
				attr = 3
			}
			start := int(a.Start)
			end := int(a.End)
			if start < 0 {
				start = 0
			}
			if end > len(txt.Text) {
				end = len(txt.Text)
			}
			if start >= end {
				continue
			}
			segs = append(segs, ImeSegment{Start: start, End: end, Attr: attr})
		}
		if len(segs) == 0 {
			segs = []ImeSegment{{Start: 0, End: len(txt.Text), Attr: 1}}
		}
		return txt.Text, segs
	}
	// Last fallback: use variant string representation
	s := v.String()
	s = strings.Trim(s, "\"")
	if s == "" {
		return "", nil
	}
	return s, []ImeSegment{{Start: 0, End: len(s), Attr: 1}}
}

// parseFcitxPreedit parses fcitx5 preedit string which may contain formatting
func parseFcitxPreedit(text string) (string, []ImeSegment) {
	if text == "" {
		return "", nil
	}
	clean := strings.ReplaceAll(text, "\x1b", "")
	if clean == "" {
		clean = text
	}
	return clean, []ImeSegment{{Start: 0, End: len(clean), Attr: 1}}
}

// DecodeIBusVariant is exported for S5 signal handling
func DecodeIBusVariant(v dbus.Variant) (string, []ImeSegment) {
	return parseIBusText(v)
}

// IBusAttrsToSegments converts raw IBus attrs to Segments (exported for test)
func IBusAttrsToSegments(text string, attrs []struct {
	Type  uint32
	Value uint32
	Start uint32
	End   uint32
}) []ImeSegment {
	if len(attrs) == 0 {
		return []ImeSegment{{Start: 0, End: len(text), Attr: 1}}
	}
	var segs []ImeSegment
	for _, a := range attrs {
		attr := uint8(1)
		if a.Type == ibusAttrUnderline {
			if a.Value == ibusUnderlineDouble {
				attr = 2
			} else {
				attr = 1
			}
		} else if a.Type == ibusAttrBackground {
			attr = 3
		}
		start := int(a.Start)
		end := int(a.End)
		if start < 0 {
			start = 0
		}
		if end > len(text) {
			end = len(text)
		}
		if start >= end {
			continue
		}
		segs = append(segs, ImeSegment{Start: start, End: end, Attr: attr})
	}
	if len(segs) == 0 {
		segs = []ImeSegment{{Start: 0, End: len(text), Attr: 1}}
	}
	return segs
}

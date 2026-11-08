//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package svg

// XML document preprocessing that Go's encoding/xml does not provide:
// DOCTYPE internal-subset entity expansion and charset decoding.
//
// Entity semantics follow XML 1.0 internal parsed entities, matching
// roxmltree (usvg's XML layer): `<!ENTITY name "value">` declarations in
// the internal subset are expanded wherever `&name;` appears — in text
// content (where the replacement may itself be markup, e.g. a <rect>)
// and in attribute values. External entities (SYSTEM/PUBLIC) are never
// fetched; parameter entities (%name) are not supported. Documents
// without a DOCTYPE (the overwhelming majority) pass through untouched.

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"golang.org/x/text/encoding/charmap"
)

// expandDTDEntities returns data with DOCTYPE internal-subset entity
// references replaced by their values.
func expandDTDEntities(data []byte) []byte {
	start := bytes.Index(data, []byte("<!DOCTYPE"))
	if start < 0 {
		return data
	}
	end, ok := scanMarkupEnd(data, start)
	if !ok {
		return data // malformed DOCTYPE: leave it; the XML parser reports it
	}
	entities := parseInternalSubset(data[start:end])
	if len(entities) == 0 {
		return data
	}
	// Replace references only after the DOCTYPE: the subset itself must not
	// be rewritten, and the declaration span is already consumed.
	tail := data[end:]
	replaced := false
	for name, value := range entities {
		ref := "&" + name + ";"
		if bytes.Contains(tail, []byte(ref)) {
			tail = bytes.ReplaceAll(tail, []byte(ref), value)
			replaced = true
		}
	}
	if !replaced {
		return data
	}
	out := make([]byte, 0, start+len(tail))
	out = append(out, data[:start]...)
	return append(out, tail...)
}

// scanMarkupEnd returns the index just past the '>' closing the markup
// starting at start, honoring quoted values and skipping the internal
// subset bracket (which may contain '>' inside declarations).
func scanMarkupEnd(data []byte, start int) (int, bool) {
	inStr := byte(0)
	for i := start; i < len(data); i++ {
		c := data[i]
		if inStr != 0 {
			if c == inStr {
				inStr = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			inStr = c
		case '[':
			// Internal subset: skip to the matching ']' (quote-aware), then
			// keep scanning for the DOCTYPE's closing '>'.
			depth := 1
			j := i + 1
			for ; j < len(data) && depth > 0; j++ {
				d := data[j]
				if inStr != 0 {
					if d == inStr {
						inStr = 0
					}
					continue
				}
				switch d {
				case '"', '\'':
					inStr = d
				case '[':
					depth++
				case ']':
					depth--
				}
			}
			if depth != 0 {
				return 0, false
			}
			i = j - 1 // sit on the matching ']'; the loop step moves past it
		case '>':
			return i + 1, true
		}
	}
	return 0, false
}

// parseInternalSubset extracts internal parsed general entities from a
// DOCTYPE declaration (including its internal subset). Values are resolved
// against previously defined entities (XML: definition before use), with a
// depth guard against self-reference.
func parseInternalSubset(doctype []byte) map[string][]byte {
	open := bytes.IndexByte(doctype, '[')
	if open < 0 {
		return nil
	}
	close_ := bytes.LastIndexByte(doctype, ']')
	if close_ < 0 || close_ < open {
		return nil
	}
	subset := doctype[open+1 : close_]
	out := map[string][]byte{}
	for i := 0; i < len(subset); {
		declPos := bytes.Index(subset[i:], []byte("<!ENTITY"))
		if declPos < 0 {
			break
		}
		declPos += i
		declEnd, ok := scanMarkupEnd(subset, declPos)
		if !ok {
			break
		}
		name, value, ok := parseEntityDecl(subset[declPos+len("<!ENTITY") : declEnd-1])
		i = declEnd
		if !ok {
			continue // parameter/external/malformed: not expandable here
		}
		out[name] = resolveEntityValue(value, out)
	}
	return out
}

// resolveEntityValue expands references to already-defined entities inside
// an entity value; unknown references are left for the XML layer to
// diagnose. Depth-bounded against runaway nesting.
func resolveEntityValue(value []byte, defined map[string][]byte) []byte {
	for depth := 0; depth < 8; depth++ {
		refStart := bytes.IndexByte(value, '&')
		if refStart < 0 {
			return value
		}
		refEnd := bytes.IndexByte(value[refStart:], ';')
		if refEnd < 0 {
			return value
		}
		repl, ok := defined[string(value[refStart+1:int(refStart)+refEnd])]
		if !ok {
			return value
		}
		next := make([]byte, 0, len(value)+len(repl))
		next = append(next, value[:refStart]...)
		next = append(next, repl...)
		next = append(next, value[refStart+refEnd+1:]...)
		value = next
	}
	return value
}

// parseEntityDecl parses one `<!ENTITY ...>` declaration body: a general
// internal entity `name "value"` (or 'value'). Parameter entities (%name)
// and external entities (SYSTEM/PUBLIC keywords) are rejected — they
// cannot be expanded textually here.
func parseEntityDecl(decl []byte) (string, []byte, bool) {
	q1 := bytes.IndexAny(decl, "\"'")
	head := decl
	if q1 >= 0 {
		head = decl[:q1]
	}
	fields := strings.Fields(string(head))
	if len(fields) == 0 || fields[0] == "%" || q1 < 0 {
		return "", nil, false
	}
	for _, f := range fields[1:] {
		if f == "SYSTEM" || f == "PUBLIC" {
			return "", nil, false
		}
	}
	quote := decl[q1]
	q2 := bytes.IndexByte(decl[q1+1:], quote)
	if q2 < 0 {
		return "", nil, false
	}
	return fields[0], decl[q1+1 : q1+1+q2], true
}

// charsetReader decodes non-UTF-8 XML documents per their encoding
// declaration. Supported: the single-byte sets that occur in practice;
// everything else fails closed (Parse surfaces the error).
func charsetReader(charset string, input io.Reader) (io.Reader, error) {
	switch strings.ToLower(strings.TrimSpace(charset)) {
	case "", "utf-8", "utf8", "us-ascii", "ascii":
		return input, nil
	case "windows-1251":
		return charmap.Windows1251.NewDecoder().Reader(input), nil
	case "windows-1252":
		return charmap.Windows1252.NewDecoder().Reader(input), nil
	case "iso-8859-1", "latin1", "iso_8859-1":
		return charmap.ISO8859_1.NewDecoder().Reader(input), nil
	default:
		return nil, fmt.Errorf("svg: unsupported encoding %q", charset)
	}
}

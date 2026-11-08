//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package svg

// <image> href resolution, aligned with usvg parser/image.rs (batch 4c-6;
// the earlier data-URI-only stance was the D2 named refusal, lifted with
// user approval):
//
//   - data: URLs: "image/jpg" and "image/jpeg" both decode as JPEG
//     (usvg default_data_resolver); an absent mime or "text/plain" falls
//     back to magic-byte sniffing (PNG/JPEG/GIF), then SVG(Z);
//   - plain hrefs resolve as file paths relative to
//     ParseConfig.ResourcesDir (usvg Options::resources_dir — "relative
//     to the input SVG file or resources_dir"); the format comes from
//     magic bytes (get_image_file_format), never the extension; no
//     network fetching (usvg likewise);
//   - SVG(Z) payloads parse as sub-documents rendered into the image slot
//     (resvg ImageKind::SVG / render_vector), with a depth guard against
//     recursive nesting.

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// maxImageSVGDepth bounds <image> → SVG → <image> → … nesting.
const maxImageSVGDepth = 8

// resolveImageRef resolves one href into element image data. Returns the
// decoded raster (raw+mime) or the parsed sub-document.
func resolveImageRef(pc *parseCtx, href string) ([]byte, string, *Document, bool) {
	href = strings.TrimSpace(href)
	if href == "" {
		return nil, "", nil, false
	}
	if strings.HasPrefix(href, "data:") {
		raw, mime, ok := decodeDataURL(href)
		if !ok {
			pc.warn("image", "href", warnInvalidValue, "malformed data URI")
			return nil, "", nil, false
		}
		raw, mime, doc, ok := imageFromPayload(pc, raw, mime)
		return raw, mime, doc, ok
	}
	// Plain href: file path relative to the resources dir. URL schemes
	// (http://, https://, …) are a named refusal — no network fetching
	// (usvg default_string_resolver opens local files only).
	if strings.Contains(href, "://") {
		pc.warn("image", "href", warnUnsupportedRef, "URL schemes are not supported (local paths only)")
		return nil, "", nil, false
	}
	base := pc.cfg.ResourcesDir
	if base == "" {
		pc.warn("image", "href", warnUnsupportedRef, "external references need ParseWithConfig ResourcesDir (data: URIs work without)")
		return nil, "", nil, false
	}
	path := href
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		pc.warn("image", "href", warnUnresolvedRef, "failed to load "+href+": "+err.Error())
		return nil, "", nil, false
	}
	raw, mime, doc, ok := imageFromPayload(pc, raw, "")
	return raw, mime, doc, ok
}

// imageFromPayload classifies a payload by explicit mime or magic bytes
// (usvg get_image_data_format / get_image_file_format: the extension and
// mime are hints, the magic decides) and parses SVG(Z) sub-documents.
func imageFromPayload(pc *parseCtx, raw []byte, mime string) ([]byte, string, *Document, bool) {
	switch mime {
	case "image/png":
		return raw, "image/png", nil, true
	case "image/jpg", "image/jpeg":
		// "image/jpg" is a common alias usvg accepts alongside "image/jpeg".
		return raw, "image/jpeg", nil, true
	case "image/gif":
		return raw, "image/gif", nil, true
	case "image/svg+xml":
		return raw, "image/svg+xml", parseSubSVG(pc, raw), true
	case "", "text/plain", "application/octet-stream":
		// Sniff: usvg matches magic bytes for text/plain and unknown mimes
		// fall through to the SVG attempt.
		switch sniffImageFormat(raw) {
		case "image/png", "image/jpeg", "image/gif":
			return raw, sniffImageFormat(raw), nil, true
		default:
			if doc := parseSubSVG(pc, raw); doc != nil {
				return raw, "image/svg+xml", doc, true
			}
			pc.warn("image", "href", warnUnsupportedRef, "payload is not a PNG, JPEG, GIF or SVG(Z) image")
			return nil, "", nil, false
		}
	default:
		pc.warn("image", "href", warnUnsupportedRef, "mime "+mime+" is not supported in <image>")
		return nil, "", nil, false
	}
}

// sniffImageFormat returns the image mime by magic bytes, "image/svg+xml"
// for gzip or markup-looking payloads, "" when unrecognized.
func sniffImageFormat(raw []byte) string {
	switch {
	case len(raw) >= 4 && bytes.Equal(raw[:4], []byte{0x89, 'P', 'N', 'G'}):
		return "image/png"
	case len(raw) >= 3 && bytes.Equal(raw[:3], []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg"
	case len(raw) >= 6 && (bytes.Equal(raw[:6], []byte("GIF87a")) || bytes.Equal(raw[:6], []byte("GIF89a"))):
		return "image/gif"
	case len(raw) >= 2 && raw[0] == 0x1F && raw[1] == 0x8B:
		return "image/svg+xml" // gzip: SVGZ (or gzipped raster — rejected at decode)
	default:
		// Markup sniff: XML prolog, doctype, comments or a leading <svg.
		head := bytes.TrimLeft(raw, " \t\r\n")
		for _, prefix := range [][]byte{[]byte("<?xml"), []byte("<!DOCTYPE"), []byte("<!--"), []byte("<svg")} {
			if bytes.HasPrefix(head, prefix) {
				return "image/svg+xml"
			}
		}
		return ""
	}
}

// decodeDataURL parses a data: URL into (payload, mime) — base64 only
// (percent-encoded payloads remain a named refusal).
func decodeDataURL(href string) ([]byte, string, bool) {
	comma := strings.Index(href, ",")
	if comma < 0 {
		return nil, "", false
	}
	head, payload := href[:comma], href[comma+1:]
	if !strings.Contains(head, ";base64") {
		return nil, "", false
	}
	// Base64 payloads wrap across lines in real documents; XML attribute
	// normalization leaves spaces/tabs behind — strip all ASCII whitespace
	// before decoding (usvg's data-URL parsing is whitespace-tolerant too).
	payload = strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\r', '\n':
			return -1
		}
		return r
	}, payload)
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, "", false
	}
	mime := strings.TrimPrefix(head, "data:")
	if i := strings.Index(mime, ";"); i >= 0 {
		mime = mime[:i]
	}
	return raw, mime, true
}

// parseSubSVG parses an SVG(Z) payload as a sub-document (usvg
// load_sub_svg / ImageKind::SVG). Depth-guarded; failures return nil with
// a warning (the image slot renders nothing).
func parseSubSVG(pc *parseCtx, raw []byte) *Document {
	if pc.cfg.svgDepth >= maxImageSVGDepth {
		pc.warn("image", "href", warnUnsupportedRef, "SVG image nesting too deep")
		return nil
	}
	data := raw
	if len(raw) >= 2 && raw[0] == 0x1F && raw[1] == 0x8B {
		zr, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			pc.warn("image", "href", warnInvalidValue, "gzip: "+err.Error())
			return nil
		}
		defer zr.Close()
		plain, err := io.ReadAll(io.LimitReader(zr, 32<<20))
		if err != nil {
			pc.warn("image", "href", warnInvalidValue, "gzip: "+err.Error())
			return nil
		}
		data = plain
	}
	doc, err := ParseWithConfig(data, ParseConfig{
		ResourcesDir: pc.cfg.ResourcesDir,
		svgDepth:     pc.cfg.svgDepth + 1,
	})
	if err != nil {
		pc.warn("image", "href", warnInvalidValue, "sub-svg: "+err.Error())
		return nil
	}
	return doc
}

// subDocSize returns the sub-document's intrinsic size (usvg Tree.size:
// the svg width/height, falling back to the viewBox, then 100×100).
func subDocSize(d *Document) (float64, float64) {
	if d == nil {
		return 0, 0
	}
	if d.Width > 0 && d.Height > 0 {
		return d.Width, d.Height
	}
	if d.hasViewBox && d.ViewBox.Width > 0 && d.ViewBox.Height > 0 {
		return d.ViewBox.Width, d.ViewBox.Height
	}
	return 100, 100
}

// decodeImageInto resolves an href into elem without applying the
// <image> geometry defaults — the feImage path (subregion sizing) uses
// its own geometry semantics.
func decodeImageInto(pc *parseCtx, elem *ImageElement) {
	raw, mime, doc, ok := resolveImageRef(pc, elem.HRef)
	if !ok {
		return
	}
	elem.raw, elem.rawMime, elem.SubDoc = raw, mime, doc
	elem.HasImage = true
	if doc != nil {
		sw, sh := subDocSize(doc)
		elem.ImgW, elem.ImgH = int(sw), int(sh)
		return
	}
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(raw)); err == nil {
		elem.ImgW, elem.ImgH = cfg.Width, cfg.Height
	}
}
